// Copyright 2025 The frp Authors
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

package accesslog

// Record represents a single third-party access event on a proxy.
type Record struct {
	ID          int64  `json:"id"`
	ProxyName   string `json:"proxyName"`
	ProxyType   string `json:"proxyType"`
	ProxyUser   string `json:"proxyUser"`
	RemoteIP    string `json:"remoteIP"`
	RemotePort  int    `json:"remotePort"`
	ConnectedAt int64  `json:"connectedAt"` // unix milliseconds
	Duration    int64  `json:"duration"`    // milliseconds
	TrafficIn   int64  `json:"trafficIn"`   // bytes
	TrafficOut  int64  `json:"trafficOut"`  // bytes
	// HTTP/HTTPS only
	UserAgent  string `json:"userAgent,omitempty"`
	Host       string `json:"host,omitempty"`
	URL        string `json:"url,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	// Blocked is true when the connection was rejected by access control.
	Blocked bool `json:"blocked,omitempty"`
}

// QueryParams holds filter and pagination parameters for querying access logs.
type QueryParams struct {
	ProxyName string
	RemoteIP  string
	StartTime int64 // unix milliseconds, 0 means no lower bound
	EndTime   int64 // unix milliseconds, 0 means no upper bound
	Page      int   // 1-based
	PageSize  int   // max 200
}

// QueryResult is the paginated result of a log query.
type QueryResult struct {
	Total   int64     `json:"total"`
	Records []*Record `json:"records"`
}
