// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vhost

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	libio "github.com/fatedier/golib/io"
	"github.com/fatedier/golib/pool"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	httppkg "github.com/fatedier/frp/pkg/util/http"
	"github.com/fatedier/frp/pkg/util/log"
	utilnet "github.com/fatedier/frp/pkg/util/net"
)

var ErrNoRouteFound = errors.New("no route found")

// statsResponseWriter wraps http.ResponseWriter to capture status code and bytes written.
type statsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytesOut   int64
}

func (w *statsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statsResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesOut += int64(n)
	return n, err
}

// countingReader wraps an io.ReadCloser to count bytes read.
type countingReader struct {
	io.ReadCloser
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.n += int64(n)
	return n, err
}

type HTTPReverseProxyOptions struct {
	ResponseHeaderTimeoutS int64
}

type HTTPReverseProxy struct {
	proxy       http.Handler
	vhostRouter *Routers

	responseHeaderTimeout time.Duration
	globalACL             *GlobalACL
	globalACLMu           sync.RWMutex // protects globalACL
	trustedProxies        []string
	trustedProxiesMu      sync.RWMutex // protects trustedProxies
}

func NewHTTPReverseProxy(option HTTPReverseProxyOptions, vhostRouter *Routers) *HTTPReverseProxy {
	if option.ResponseHeaderTimeoutS <= 0 {
		option.ResponseHeaderTimeoutS = 60
	}
	rp := &HTTPReverseProxy{
		responseHeaderTimeout: time.Duration(option.ResponseHeaderTimeoutS) * time.Second,
		vhostRouter:           vhostRouter,
	}
	proxy := &httputil.ReverseProxy{
		// Modify incoming requests by route policies.
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
			r.SetXForwarded()
			req := r.Out
			req.URL.Scheme = "http"
			reqRouteInfo := req.Context().Value(RouteInfoKey).(*RequestRouteInfo)
			originalHost, _ := httppkg.CanonicalHost(reqRouteInfo.Host)

			rc := req.Context().Value(RouteConfigKey).(*RouteConfig)
			if rc != nil {
				if rc.RewriteHost != "" {
					req.Host = rc.RewriteHost
				}

				var endpoint string
				if rc.ChooseEndpointFn != nil {
					// ignore error here, it will use CreateConnFn instead later
					endpoint, _ = rc.ChooseEndpointFn()
					reqRouteInfo.Endpoint = endpoint
					log.Tracef("choose endpoint name [%s] for http request host [%s] path [%s] httpuser [%s]",
						endpoint, originalHost, reqRouteInfo.URL, reqRouteInfo.HTTPUser)
				}
				// Set {domain}.{location}.{routeByHTTPUser}.{endpoint} as URL host here to let http transport reuse connections.
				req.URL.Host = rc.Domain + "." +
					base64.StdEncoding.EncodeToString([]byte(rc.Location)) + "." +
					base64.StdEncoding.EncodeToString([]byte(rc.RouteByHTTPUser)) + "." +
					base64.StdEncoding.EncodeToString([]byte(endpoint))

				for k, v := range rc.Headers {
					req.Header.Set(k, v)
				}
			} else {
				req.URL.Host = req.Host
			}
		},
		ModifyResponse: func(r *http.Response) error {
			rc := r.Request.Context().Value(RouteConfigKey).(*RouteConfig)
			if rc != nil {
				for k, v := range rc.ResponseHeaders {
					r.Header.Set(k, v)
				}
			}
			return nil
		},
		// Create a connection to one proxy routed by route policy.
		Transport: &http.Transport{
			ResponseHeaderTimeout: rp.responseHeaderTimeout,
			IdleConnTimeout:       60 * time.Second,
			MaxIdleConnsPerHost:   5,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return rp.CreateConnection(ctx.Value(RouteInfoKey).(*RequestRouteInfo), true)
			},
			Proxy: func(req *http.Request) (*url.URL, error) {
				// Use proxy mode if there is host in HTTP first request line.
				// GET http://example.com/ HTTP/1.1
				// Host: example.com
				//
				// Normal:
				// GET / HTTP/1.1
				// Host: example.com
				urlHost := req.Context().Value(RouteInfoKey).(*RequestRouteInfo).URLHost
				if urlHost != "" {
					return req.URL, nil
				}
				return nil, nil
			},
		},
		BufferPool: pool.NewBuffer(32 * 1024),
		ErrorLog:   stdlog.New(log.NewWriteLogger(log.WarnLevel, 2), "", 0),
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			log.Logf(log.WarnLevel, 1, "do http proxy request [host: %s] error: %v", req.Host, err)
			if err != nil {
				if e, ok := err.(net.Error); ok && e.Timeout() {
					rw.WriteHeader(http.StatusGatewayTimeout)
					return
				}
			}
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write(getNotFoundPageContent())
		},
	}
	rp.proxy = h2c.NewHandler(proxy, &http2.Server{})
	return rp
}

// SetGlobalACL sets the server-level access control rules.
// Safe to call at any time; updates are immediately effective.
func (rp *HTTPReverseProxy) SetGlobalACL(acl *GlobalACL) {
	rp.globalACLMu.Lock()
	rp.globalACL = acl
	rp.globalACLMu.Unlock()
}

// GetGlobalACL returns the current server-level access control rules.
func (rp *HTTPReverseProxy) GetGlobalACL() *GlobalACL {
	rp.globalACLMu.RLock()
	defer rp.globalACLMu.RUnlock()
	return rp.globalACL
}

// SetTrustedProxies sets the list of trusted proxy IP addresses/CIDR ranges.
// Requests from these IPs will have their X-Forwarded-For header parsed to extract real client IP.
func (rp *HTTPReverseProxy) SetTrustedProxies(proxies []string) {
	rp.trustedProxiesMu.Lock()
	rp.trustedProxies = proxies
	rp.trustedProxiesMu.Unlock()
}

// GetTrustedProxies returns the current list of trusted proxy IP addresses/CIDR ranges.
func (rp *HTTPReverseProxy) GetTrustedProxies() []string {
	rp.trustedProxiesMu.RLock()
	defer rp.trustedProxiesMu.RUnlock()
	return rp.trustedProxies
}

// Register register the route config to reverse proxy
// reverse proxy will use CreateConnFn from routeCfg to create a connection to the remote service
func (rp *HTTPReverseProxy) Register(routeCfg RouteConfig) error {
	err := rp.vhostRouter.Add(routeCfg.Domain, routeCfg.Location, routeCfg.RouteByHTTPUser, &routeCfg)
	if err != nil {
		return err
	}
	return nil
}

// UnRegister unregister route config by domain and location
func (rp *HTTPReverseProxy) UnRegister(routeCfg RouteConfig) {
	rp.vhostRouter.Del(routeCfg.Domain, routeCfg.Location, routeCfg.RouteByHTTPUser)
}

func (rp *HTTPReverseProxy) GetRouteConfig(domain, location, routeByHTTPUser string) *RouteConfig {
	vr, ok := rp.getVhost(domain, location, routeByHTTPUser)
	if ok {
		log.Debugf("get new http request host [%s] path [%s] httpuser [%s]", domain, location, routeByHTTPUser)
		return vr.payload.(*RouteConfig)
	}
	return nil
}

// CreateConnection create a new connection by route config
func (rp *HTTPReverseProxy) CreateConnection(reqRouteInfo *RequestRouteInfo, byEndpoint bool) (net.Conn, error) {
	host, _ := httppkg.CanonicalHost(reqRouteInfo.Host)
	vr, ok := rp.getVhost(host, reqRouteInfo.URL, reqRouteInfo.HTTPUser)
	if ok {
		if byEndpoint {
			fn := vr.payload.(*RouteConfig).CreateConnByEndpointFn
			if fn != nil {
				return fn(reqRouteInfo.Endpoint, reqRouteInfo.RemoteAddr)
			}
		}
		fn := vr.payload.(*RouteConfig).CreateConnFn
		if fn != nil {
			return fn(reqRouteInfo.RemoteAddr)
		}
	}
	return nil, fmt.Errorf("%v: %s %s %s", ErrNoRouteFound, host, reqRouteInfo.URL, reqRouteInfo.HTTPUser)
}

func (rp *HTTPReverseProxy) CheckAuth(domain, location, routeByHTTPUser, user, passwd string) bool {
	vr, ok := rp.getVhost(domain, location, routeByHTTPUser)
	if ok {
		checkUser := vr.payload.(*RouteConfig).Username
		checkPasswd := vr.payload.(*RouteConfig).Password
		if (checkUser != "" || checkPasswd != "") && (checkUser != user || checkPasswd != passwd) {
			return false
		}
	}
	return true
}

// RouteConfig is also extended with AllowIPs/DenyIPs for HTTP-level access control.
// checkHTTPAccess returns true if the request should be allowed through.
// Logic:
//  1. If source IP is in DenyIPs → reject
//  2. If UA matches DenyUserAgents → reject
//  3. If AllowIPs and AllowUserAgents are both empty → allow
//  4. If source IP matches AllowIPs OR UA matches AllowUserAgents → allow
//  5. Otherwise → reject
func checkHTTPAccess(remoteAddr, userAgent string, allowIPs, denyIPs, allowUserAgents, denyUserAgents []string) bool {
	host, _, err := parseHost(remoteAddr)
	if err != nil {
		// can't parse addr, deny for safety
		return false
	}

	// 1. DenyIPs always wins
	for _, cidr := range denyIPs {
		if matchIPStr(host, cidr) {
			return false
		}
	}

	// 2. DenyUserAgents always wins
	for _, pattern := range denyUserAgents {
		if matchUserAgent(userAgent, pattern) {
			return false
		}
	}

	// 3. If no allow rules at all, pass through
	if len(allowIPs) == 0 && len(allowUserAgents) == 0 {
		return true
	}

	// 4. OR: IP match OR UA match
	for _, cidr := range allowIPs {
		if matchIPStr(host, cidr) {
			return true
		}
	}
	for _, pattern := range allowUserAgents {
		if matchUserAgent(userAgent, pattern) {
			return true
		}
	}
	return false
}

func parseHost(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// If addr doesn't have a port (e.g., "192.168.1.100"), treat it as a pure IP
		// net.SplitHostPort returns error for addresses without port
		return addr, "", nil
	}
	return host, port, err
}

// matchIPStr checks whether the host string matches a CIDR range or exact IP.
func matchIPStr(host, cidrOrIP string) bool {
	return utilnet.MatchIPStr(host, cidrOrIP)
}

// matchUserAgent checks whether ua matches a single glob pattern.
// Only '*' is supported as a wildcard; it matches any sequence of characters,
// including '/' (unlike path.Match which treats '/' as a separator).
// Example patterns: "Mozilla/*", "*Chrome*", "curl*".
func matchUserAgent(ua, pattern string) bool {
	// No wildcard: require exact match.
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return ua == pattern
	}
	// The first segment must be a prefix of ua.
	if !strings.HasPrefix(ua, parts[0]) {
		return false
	}
	// Advance past the matched prefix, then locate each subsequent segment
	// in order within the remaining string.
	remaining := ua[len(parts[0]):]
	for i, p := range parts[1:] {
		// The last segment must be a suffix of the remaining string (tail anchor).
		if i == len(parts)-2 {
			return strings.HasSuffix(remaining, p)
		}
		idx := strings.Index(remaining, p)
		if idx < 0 {
			return false
		}
		remaining = remaining[idx+len(p):]
	}
	return true
}

// getRealClientIP extracts the real client IP from X-Forwarded-For header
// if the request comes from a trusted proxy. Otherwise returns req.RemoteAddr.
//
// X-Forwarded-For format: "client1, proxy1, proxy2"
// The first IP is the original client, subsequent IPs are proxies.
// Only extract if direct connection source is in trustedProxies.
//
// The returned value preserves the original port from req.RemoteAddr so that
// downstream code (e.g. net.ResolveTCPAddr for proxy protocol) continues to work.
// Format: "realIP:originalPort"  or just "realIP" when the original port is unavailable.
func getRealClientIP(req *http.Request, trustedProxies []string) string {
	remoteHost, remotePort, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}

	// Check if source IP is in trusted proxies
	trusted := false
	for _, cidr := range trustedProxies {
		if matchIPStr(remoteHost, cidr) {
			trusted = true
			break
		}
	}

	if !trusted {
		return req.RemoteAddr
	}

	// helper: rebuild addr keeping the original port
	withPort := func(ip string) string {
		if remotePort == "" {
			return ip
		}
		return net.JoinHostPort(ip, remotePort)
	}

	// Extract first IP from X-Forwarded-For
	xff := req.Header.Get("X-Forwarded-For")
	if xff == "" {
		// Try X-Real-IP as fallback
		xri := strings.TrimSpace(req.Header.Get("X-Real-IP"))
		if xri != "" {
			return withPort(xri)
		}
		return req.RemoteAddr
	}

	// X-Forwarded-For: client, proxy1, proxy2
	// Get the first (leftmost) IP which is the original client
	ips := strings.Split(xff, ",")
	if len(ips) > 0 {
		clientIP := strings.TrimSpace(ips[0])
		if clientIP != "" {
			return withPort(clientIP)
		}
	}

	return req.RemoteAddr
}

// getVhost tries to get vhost router by route policy.
func (rp *HTTPReverseProxy) getVhost(domain, location, routeByHTTPUser string) (*Router, bool) {
	findRouter := func(inDomain, inLocation, inRouteByHTTPUser string) (*Router, bool) {
		vr, ok := rp.vhostRouter.Get(inDomain, inLocation, inRouteByHTTPUser)
		if ok {
			return vr, ok
		}
		// Try to check if there is one proxy that doesn't specify routerByHTTPUser, it means match all.
		vr, ok = rp.vhostRouter.Get(inDomain, inLocation, "")
		if ok {
			return vr, ok
		}
		return nil, false
	}

	// First we check the full hostname
	// if not exist, then check the wildcard_domain such as *.example.com
	vr, ok := findRouter(domain, location, routeByHTTPUser)
	if ok {
		return vr, ok
	}

	// e.g. domain = test.example.com, try to match wildcard domains.
	// *.example.com
	// *.com
	domainSplit := strings.Split(domain, ".")
	for len(domainSplit) >= 3 {
		domainSplit[0] = "*"
		domain = strings.Join(domainSplit, ".")
		vr, ok = findRouter(domain, location, routeByHTTPUser)
		if ok {
			return vr, true
		}
		domainSplit = domainSplit[1:]
	}

	// Finally, try to check if there is one proxy that domain is "*" means match all domains.
	vr, ok = findRouter("*", location, routeByHTTPUser)
	if ok {
		return vr, true
	}
	return nil, false
}

func (rp *HTTPReverseProxy) connectHandler(rw http.ResponseWriter, req *http.Request) {
	hj, ok := rw.(http.Hijacker)
	if !ok {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, _, err := hj.Hijack()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rc, _ := req.Context().Value(RouteConfigKey).(*RouteConfig)
	reqRouteInfo := req.Context().Value(RouteInfoKey).(*RequestRouteInfo)
	connectedAt := time.Now()

	remote, err := rp.CreateConnection(reqRouteInfo, false)
	if err != nil {
		_ = NotFoundResponse().Write(client)
		client.Close()
		return
	}
	_ = req.Write(remote)
	inCount, outCount, _ := libio.Join(remote, client)

	if rc != nil && rc.AccessLogFn != nil {
		rc.AccessLogFn(AccessLogEntry{
			RemoteAddr:  reqRouteInfo.RemoteAddr,
			UserAgent:   req.Header.Get("User-Agent"),
			Host:        req.Host,
			URL:         req.URL.Path,
			StatusCode:  http.StatusOK,
			TrafficIn:   inCount,
			TrafficOut:  outCount,
			ConnectedAt: connectedAt.UnixMilli(),
			Duration:    time.Since(connectedAt).Milliseconds(),
		})
	}
}

func (rp *HTTPReverseProxy) injectRequestInfoToCtx(req *http.Request, realIP string) *http.Request {
	user := ""
	// If url host isn't empty, it's a proxy request. Get http user from Proxy-Authorization header.
	if req.URL.Host != "" {
		proxyAuth := req.Header.Get("Proxy-Authorization")
		if proxyAuth != "" {
			user, _, _ = httppkg.ParseBasicAuth(proxyAuth)
		}
	}
	if user == "" {
		user, _, _ = req.BasicAuth()
	}

	reqRouteInfo := &RequestRouteInfo{
		URL:        req.URL.Path,
		Host:       req.Host,
		HTTPUser:   user,
		RemoteAddr: realIP,
		URLHost:    req.URL.Host,
	}

	originalHost, _ := httppkg.CanonicalHost(reqRouteInfo.Host)
	rc := rp.GetRouteConfig(originalHost, reqRouteInfo.URL, reqRouteInfo.HTTPUser)

	newctx := req.Context()
	newctx = context.WithValue(newctx, RouteInfoKey, reqRouteInfo)
	newctx = context.WithValue(newctx, RouteConfigKey, rc)
	return req.Clone(newctx)
}

func (rp *HTTPReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Extract real client IP first (before any checks that use RemoteAddr)
	trustedProxies := rp.GetTrustedProxies()
	realIP := getRealClientIP(req, trustedProxies)

	domain, _ := httppkg.CanonicalHost(req.Host)
	location := req.URL.Path
	user, passwd, _ := req.BasicAuth()
	if !rp.CheckAuth(domain, location, user, user, passwd) {
		rw.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Layer 1: global ACL (server-level, evaluated before per-proxy rules).
	globalACL := rp.GetGlobalACL()
	if globalACL != nil {
		ua := req.Header.Get("User-Agent")
		if !checkHTTPAccess(realIP, ua,
			globalACL.AllowIPs,
			globalACL.DenyIPs,
			globalACL.AllowUserAgents,
			globalACL.DenyUserAgents,
		) {
			log.Debugf("http request from [%s] rejected by global access control, UA: %q", realIP, ua)
			http.Error(rw, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	newreq := rp.injectRequestInfoToCtx(req, realIP)
	// Layer 2: per-proxy IP + UA access control (OR logic: pass if IP matches AllowIPs OR UA matches AllowUserAgents).
	// DenyIPs and DenyUserAgents are always checked first and always reject.
	rc, _ := newreq.Context().Value(RouteConfigKey).(*RouteConfig)
	if rc != nil && (len(rc.AllowIPs) > 0 || len(rc.DenyIPs) > 0 || len(rc.AllowUserAgents) > 0) {
		ua := req.Header.Get("User-Agent")
		if !checkHTTPAccess(realIP, ua, rc.AllowIPs, rc.DenyIPs, rc.AllowUserAgents, nil) {
			log.Debugf("http request from [%s] rejected by access control, UA: %q", realIP, ua)
			if rc.AccessLogFn != nil {
				rc.AccessLogFn(AccessLogEntry{
					RemoteAddr:  realIP,
					UserAgent:   ua,
					Host:        req.Host,
					URL:         req.URL.Path,
					StatusCode:  http.StatusForbidden,
					ConnectedAt: time.Now().UnixMilli(),
					Blocked:     true,
				})
			}
			http.Error(rw, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	if req.Method == http.MethodConnect {
		rp.connectHandler(rw, newreq)
		return
	}

	// Wrap request body and response writer to capture traffic stats.
	connectedAt := time.Now()
	var bodyReader *countingReader
	if req.Body != nil {
		bodyReader = &countingReader{ReadCloser: req.Body}
		newreq.Body = bodyReader
	}
	srw := &statsResponseWriter{ResponseWriter: rw, statusCode: http.StatusOK}

	rp.proxy.ServeHTTP(srw, newreq)

	if rc != nil && rc.AccessLogFn != nil {
		var bytesIn int64
		if bodyReader != nil {
			bytesIn = bodyReader.n
		}
		rc.AccessLogFn(AccessLogEntry{
			RemoteAddr:  realIP,
			UserAgent:   req.Header.Get("User-Agent"),
			Host:        req.Host,
			URL:         req.URL.Path,
			StatusCode:  srw.statusCode,
			TrafficIn:   bytesIn,
			TrafficOut:  srw.bytesOut,
			ConnectedAt: connectedAt.UnixMilli(),
			Duration:    time.Since(connectedAt).Milliseconds(),
		})
	}
}
