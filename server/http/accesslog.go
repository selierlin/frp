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

package http

import (
	"strconv"

	"github.com/fatedier/frp/pkg/accesslog"
	httppkg "github.com/fatedier/frp/pkg/util/http"
)

// GET /api/accesslog
// Query params:
//
//	proxyName  - filter by proxy name (optional)
//	remoteIP   - filter by remote IP (optional)
//	startTime  - unix milliseconds lower bound (optional)
//	endTime    - unix milliseconds upper bound (optional)
//	page       - 1-based page number (default 1)
//	pageSize   - records per page, max 200 (default 20)
func (c *Controller) APIAccessLog(ctx *httppkg.Context) (any, error) {
	params := accesslog.QueryParams{
		ProxyName: ctx.Query("proxyName"),
		RemoteIP:  ctx.Query("remoteIP"),
	}

	if s := ctx.Query("startTime"); s != "" {
		params.StartTime, _ = strconv.ParseInt(s, 10, 64)
	}
	if s := ctx.Query("endTime"); s != "" {
		params.EndTime, _ = strconv.ParseInt(s, 10, 64)
	}
	if s := ctx.Query("page"); s != "" {
		params.Page, _ = strconv.Atoi(s)
	}
	if s := ctx.Query("pageSize"); s != "" {
		params.PageSize, _ = strconv.Atoi(s)
	}
	// Enforce maximum page size.
	if params.PageSize > 200 {
		params.PageSize = 200
	}

	result, err := accesslog.Default.Query(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}
