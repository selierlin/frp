package vhost

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		pattern  string
		expected bool
	}{
		// No wildcard - exact match
		{name: "exact_match", ua: "curl/7.68.0", pattern: "curl/7.68.0", expected: true},
		{name: "exact_no_match", ua: "curl/7.68.0", pattern: "curl/7.67.0", expected: false},

		// Prefix wildcard
		{name: "prefix_wildcard_match", ua: "Mozilla/5.0 Chrome/90.0", pattern: "Mozilla/*", expected: true},
		{name: "prefix_wildcard_no_match", ua: "Chrome/90.0", pattern: "Mozilla/*", expected: false},

		// Suffix wildcard
		{name: "suffix_wildcard_match2", ua: "MyApp-curl", pattern: "*curl", expected: true},
		{name: "suffix_wildcard_match3", ua: "Chrome/90.0 Safari", pattern: "*Safari", expected: true},

		// Middle wildcard
		{name: "middle_wildcard_match", ua: "Mozilla/5.0 Chrome/90.0", pattern: "Mozilla/*Chrome*", expected: true},
		{name: "middle_wildcard_no_match", ua: "Mozilla/5.0 Firefox", pattern: "Mozilla/*Chrome*", expected: false},

		// Multiple wildcards
		{name: "multi_wildcard_match", ua: "abc-def-ghi", pattern: "abc*def*ghi", expected: true},
		{name: "multi_wildcard_no_match", ua: "abc-xyz-ghi", pattern: "abc*def*ghi", expected: false},

		// Pure wildcard - matches everything
		{name: "pure_wildcard", ua: "anything", pattern: "*", expected: true},
		{name: "pure_wildcard_empty", ua: "", pattern: "*", expected: true},

		// Edge cases
		{name: "empty_ua", ua: "", pattern: "something", expected: false},
		{name: "empty_pattern", ua: "something", pattern: "", expected: false},
		{name: "both_empty", ua: "", pattern: "", expected: true},
		{name: "wildcard_at_end_empty_suffix", ua: "test", pattern: "test*", expected: true},
		{name: "wildcard_at_start_empty_prefix", ua: "test", pattern: "*test", expected: true},

		// Contains '/' - unlike path.Match, '/' is not a separator
		{name: "slash_in_ua", ua: "Mozilla/5.0", pattern: "Mozilla/*5.0", expected: true},
		{name: "slash_no_separator", ua: "a/b/c", pattern: "a*c", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchUserAgent(tt.ua, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchUserAgent(%q, %q) = %v, expected %v", tt.ua, tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestGlobalACLLogic tests the Global ACL check logic directly.
// This tests checkHTTPAccess function which is the core of Global ACL.
func TestGlobalACLLogic(t *testing.T) {
	tests := []struct {
		name           string
		allowIPs       []string
		denyIPs        []string
		allowUserAgents []string
		denyUserAgents []string
		remoteAddr     string
		userAgent      string
		expectAllow    bool
	}{
		// Deny by IP
		{name: "deny_by_ip", denyIPs: []string{"192.168.1.100"},
			remoteAddr: "192.168.1.100:12345", userAgent: "curl", expectAllow: false},
		{name: "deny_by_ip_not_match", denyIPs: []string{"192.168.1.100"},
			remoteAddr: "192.168.1.101:12345", userAgent: "curl", expectAllow: true},

		// Deny by CIDR
		{name: "deny_by_cidr", denyIPs: []string{"10.0.0.0/8"},
			remoteAddr: "10.1.2.3:12345", userAgent: "curl", expectAllow: false},
		{name: "deny_by_cidr_not_match", denyIPs: []string{"10.0.0.0/8"},
			remoteAddr: "192.168.1.1:12345", userAgent: "curl", expectAllow: true},

		// Deny by UserAgent glob
		{name: "deny_by_ua_exact", denyUserAgents: []string{"curl"},
			remoteAddr: "1.2.3.4:12345", userAgent: "curl", expectAllow: false},
		{name: "deny_by_ua_wildcard", denyUserAgents: []string{"curl*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "curl/7.68.0", expectAllow: false},
		{name: "deny_by_ua_wildcard_middle", denyUserAgents: []string{"*bot*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "Googlebot/2.1", expectAllow: false},
		{name: "deny_by_ua_not_match", denyUserAgents: []string{"curl*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "Mozilla", expectAllow: true},

		// Allow by IP (when allow list is non-empty)
		{name: "allow_by_ip", allowIPs: []string{"192.168.1.100"},
			remoteAddr: "192.168.1.100:12345", userAgent: "curl", expectAllow: true},
		{name: "allow_by_ip_not_in_list", allowIPs: []string{"192.168.1.100"},
			remoteAddr: "192.168.1.101:12345", userAgent: "curl", expectAllow: false},

		// Allow by UserAgent (when allow list is non-empty)
		{name: "allow_by_ua", allowUserAgents: []string{"Mozilla*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "Mozilla/5.0", expectAllow: true},
		{name: "allow_by_ua_not_match", allowUserAgents: []string{"Mozilla*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "curl", expectAllow: false},

		// Both AllowIPs and AllowUserAgents (OR logic)
		{name: "allow_ip_or_ua_ip_match", allowIPs: []string{"192.168.1.100"}, allowUserAgents: []string{"Mozilla*"},
			remoteAddr: "192.168.1.100:12345", userAgent: "curl", expectAllow: true},
		{name: "allow_ip_or_ua_ua_match", allowIPs: []string{"192.168.1.100"}, allowUserAgents: []string{"Mozilla*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "Mozilla/5.0", expectAllow: true},
		{name: "allow_ip_or_ua_both_not_match", allowIPs: []string{"192.168.1.100"}, allowUserAgents: []string{"Mozilla*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "curl", expectAllow: false},

		// Empty allow lists = allow all (no deny rules)
		{name: "empty_lists_allow_all",
			remoteAddr: "1.2.3.4:12345", userAgent: "anything", expectAllow: true},

		// Deny takes precedence over allow
		{name: "deny_ip_precedence", denyIPs: []string{"192.168.1.100"}, allowIPs: []string{"192.168.1.100"},
			remoteAddr: "192.168.1.100:12345", userAgent: "curl", expectAllow: false},
		{name: "deny_ua_precedence", denyUserAgents: []string{"curl"}, allowUserAgents: []string{"curl*"},
			remoteAddr: "1.2.3.4:12345", userAgent: "curl", expectAllow: false},

		// IPv6
		{name: "deny_ipv6", denyIPs: []string{"::1"},
			remoteAddr: "[::1]:12345", userAgent: "curl", expectAllow: false},
		{name: "deny_ipv6_cidr", denyIPs: []string{"2001:db8::/32"},
			remoteAddr: "[2001:db8::1]:12345", userAgent: "curl", expectAllow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkHTTPAccess(tt.remoteAddr, tt.userAgent,
				tt.allowIPs, tt.denyIPs, tt.allowUserAgents, tt.denyUserAgents)
			if result != tt.expectAllow {
				t.Errorf("checkHTTPAccess(%q, %q, ...) = %v, expected %v",
					tt.remoteAddr, tt.userAgent, result, tt.expectAllow)
			}
		})
	}
}

// TestGetRealClientIP tests the real client IP extraction from X-Forwarded-For header.
func TestGetRealClientIP(t *testing.T) {
	tests := []struct {
		name            string
		remoteAddr      string
		trustedProxies  []string
		xForwardedFor   string
		xRealIP         string
		expectedIP      string
	}{
		// No trusted proxies - use RemoteAddr directly
		{name: "no_trusted_proxies", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{},
			xForwardedFor: "192.168.1.100", expectedIP: "127.0.0.1:12345"},
		{name: "empty_trusted_proxies", remoteAddr: "127.0.0.1:12345", trustedProxies: nil,
			xForwardedFor: "192.168.1.100", expectedIP: "127.0.0.1:12345"},

		// Source not in trusted proxies - use RemoteAddr
		{name: "source_not_trusted", remoteAddr: "10.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "192.168.1.100", expectedIP: "10.0.0.1:12345"},
		{name: "source_cidr_not_match", remoteAddr: "192.168.1.1:12345", trustedProxies: []string{"10.0.0.0/8"},
			xForwardedFor: "192.168.1.100", expectedIP: "192.168.1.1:12345"},

		// Source in trusted proxies - extract from X-Forwarded-For (port is preserved from RemoteAddr)
		{name: "extract_from_xff_single", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "192.168.1.100", expectedIP: "192.168.1.100:12345"},
		{name: "extract_from_xff_chain", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "192.168.1.100, 10.0.0.1, 127.0.0.1", expectedIP: "192.168.1.100:12345"},
		{name: "extract_from_xff_with_spaces", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: " 192.168.1.100 , 10.0.0.1 ", expectedIP: "192.168.1.100:12345"},
		{name: "extract_from_xff_cidr_trusted", remoteAddr: "10.0.0.1:12345", trustedProxies: []string{"10.0.0.0/8"},
			xForwardedFor: "203.0.113.50, 10.0.0.2", expectedIP: "203.0.113.50:12345"},

		// No X-Forwarded-For but has X-Real-IP - fallback to X-Real-IP (port preserved)
		{name: "fallback_to_x_real_ip", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "", xRealIP: "192.168.1.100", expectedIP: "192.168.1.100:12345"},
		{name: "x_real_ip_only", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1"},
			xForwardedFor: "", xRealIP: "203.0.113.50", expectedIP: "203.0.113.50:12345"},

		// Trusted but no headers - use RemoteAddr
		{name: "trusted_no_headers", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "", xRealIP: "", expectedIP: "127.0.0.1:12345"},
		{name: "trusted_empty_xff", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "", expectedIP: "127.0.0.1:12345"},

		// IPv6 support (port is preserved from RemoteAddr)
		{name: "ipv6_trusted", remoteAddr: "[::1]:12345", trustedProxies: []string{"::1"},
			xForwardedFor: "2001:db8::1", expectedIP: "[2001:db8::1]:12345"},
		{name: "ipv6_cidr_trusted", remoteAddr: "[2001:db8::1]:12345", trustedProxies: []string{"2001:db8::/32"},
			xForwardedFor: "203.0.113.50", expectedIP: "203.0.113.50:12345"},
		{name: "ipv6_not_trusted", remoteAddr: "[::1]:12345", trustedProxies: []string{"192.168.1.0/24"},
			xForwardedFor: "10.0.0.1", expectedIP: "[::1]:12345"},

		// Edge cases
		{name: "invalid_remoteaddr", remoteAddr: "invalid", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "192.168.1.100", expectedIP: "invalid"},
		{name: "xff_only_spaces", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: "   ", expectedIP: "127.0.0.1:12345"},
		{name: "xff_empty_first", remoteAddr: "127.0.0.1:12345", trustedProxies: []string{"127.0.0.1/8"},
			xForwardedFor: ", 10.0.0.1", expectedIP: "127.0.0.1:12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			result := getRealClientIP(req, tt.trustedProxies)
			if result != tt.expectedIP {
				t.Errorf("getRealClientIP(remoteAddr=%q, trustedProxies=%v, xff=%q) = %q, expected %q",
					tt.remoteAddr, tt.trustedProxies, tt.xForwardedFor, result, tt.expectedIP)
			}
		})
	}
}

// TestTrustedProxiesIntegration tests TrustedProxies in the full HTTPReverseProxy flow.
// Verifies that real IP is extracted from X-Forwarded-For and used for per-proxy ACL checks.
// Note: global IP rules (GlobalAllowIPs/GlobalDenyIPs) are now enforced at the BaseProxy layer
// (server/proxy/proxy.go) and are not tested here.
func TestTrustedProxiesIntegration(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		perProxyDenyIPs []string // per-proxy deny list to verify real IP extraction
		remoteAddr     string   // TCP connection source (proxy IP)
		xForwardedFor  string   // Real client IP chain
		expectStatus   int      // 403 = blocked by per-proxy ACL using real IP, 404 = passed but backend unavailable
	}{
		// Trusted proxy enabled: real IP extracted from XFF and used for per-proxy ACL
		{name: "trusted_proxy_block_real_ip", trustedProxies: []string{"127.0.0.1/8"},
			perProxyDenyIPs: []string{"192.168.1.100"},
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusForbidden},
		{name: "trusted_proxy_allow_real_ip", trustedProxies: []string{"127.0.0.1/8"},
			perProxyDenyIPs: []string{"192.168.1.200"}, // deny a different IP
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusNotFound}, // real IP not in deny list

		// Trusted proxy: proxy IP (127.0.0.1) is NOT used when XFF provides real IP
		{name: "trusted_proxy_proxy_ip_not_used", trustedProxies: []string{"127.0.0.1/8"},
			perProxyDenyIPs: []string{"127.0.0.1"}, // deny proxy IP
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusNotFound}, // real IP is 192.168.1.100, not denied

		// No trusted proxy: raw connection IP used directly
		{name: "no_trusted_proxy_block_proxy_ip", trustedProxies: []string{},
			perProxyDenyIPs: []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusForbidden},
		{name: "no_trusted_proxy_ignore_xff", trustedProxies: []string{},
			perProxyDenyIPs: []string{"192.168.1.100"}, // deny XFF IP, but XFF is ignored
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusNotFound}, // proxy IP not denied

		// Source not trusted: proxy IP used, XFF ignored
		{name: "untrusted_source_block_proxy_ip", trustedProxies: []string{"10.0.0.0/8"},
			perProxyDenyIPs: []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:12345", xForwardedFor: "192.168.1.100", expectStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routers := NewRouters()
			rp := NewHTTPReverseProxy(HTTPReverseProxyOptions{}, routers)

			// Set trusted proxies
			if len(tt.trustedProxies) > 0 {
				rp.SetTrustedProxies(tt.trustedProxies)
			}

			// Register a route with per-proxy DenyIPs to verify real IP extraction
			err := routers.Add("example.com", "/test", "", &RouteConfig{
				Domain:   "example.com",
				Location: "/test",
				DenyIPs:  tt.perProxyDenyIPs,
				CreateConnFn: func(remoteAddr string) (net.Conn, error) {
					return nil, http.ErrNotSupported
				},
			})
			if err != nil {
				t.Fatalf("failed to add route: %v", err)
			}

			// Create test request
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			// Execute request
			rec := httptest.NewRecorder()
			rp.ServeHTTP(rec, req)

			// Verify result
			if rec.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
