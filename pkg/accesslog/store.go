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

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS access_logs (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	proxy_name   TEXT    NOT NULL,
	proxy_type   TEXT    NOT NULL,
	proxy_user   TEXT    NOT NULL DEFAULT '',
	remote_ip    TEXT    NOT NULL,
	remote_port  INTEGER NOT NULL,
	connected_at INTEGER NOT NULL,
	duration     INTEGER NOT NULL,
	traffic_in   INTEGER NOT NULL,
	traffic_out  INTEGER NOT NULL,
	user_agent   TEXT    NOT NULL DEFAULT '',
	host         TEXT    NOT NULL DEFAULT '',
	url          TEXT    NOT NULL DEFAULT '',
	status_code  INTEGER NOT NULL DEFAULT 0,
	blocked      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_proxy_time ON access_logs(proxy_name, connected_at DESC);
CREATE INDEX IF NOT EXISTS idx_ip_time    ON access_logs(remote_ip,  connected_at DESC);
CREATE INDEX IF NOT EXISTS idx_time       ON access_logs(connected_at);
`

type store struct {
	db *sql.DB
}

func newStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer goroutine is used, but set max open conns for reads.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA temp_store=MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &store{db: db}, nil
}

func (s *store) batchInsert(records []*Record) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO access_logs
		(proxy_name, proxy_type, proxy_user, remote_ip, remote_port,
		 connected_at, duration, traffic_in, traffic_out,
		 user_agent, host, url, status_code, blocked)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, r := range records {
		blocked := 0
		if r.Blocked {
			blocked = 1
		}
		if _, err := stmt.Exec(
			r.ProxyName, r.ProxyType, r.ProxyUser, r.RemoteIP, r.RemotePort,
			r.ConnectedAt, r.Duration, r.TrafficIn, r.TrafficOut,
			r.UserAgent, r.Host, r.URL, r.StatusCode, blocked,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *store) query(p QueryParams) (*QueryResult, error) {
	pageSize := p.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	page := p.Page
	if page <= 0 {
		page = 1
	}

	where, args := buildWhere(p)

	countSQL := "SELECT COUNT(*) FROM access_logs" + where
	var total int64
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf(
		"SELECT id,proxy_name,proxy_type,proxy_user,remote_ip,remote_port,connected_at,duration,traffic_in,traffic_out,user_agent,host,url,status_code,blocked FROM access_logs%s ORDER BY connected_at DESC LIMIT ? OFFSET ?",
		where,
	)
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*Record, 0, pageSize)
	for rows.Next() {
		r := &Record{}
		var blocked int
		if err := rows.Scan(
			&r.ID, &r.ProxyName, &r.ProxyType, &r.ProxyUser,
			&r.RemoteIP, &r.RemotePort, &r.ConnectedAt, &r.Duration,
			&r.TrafficIn, &r.TrafficOut, &r.UserAgent, &r.Host, &r.URL,
			&r.StatusCode, &blocked,
		); err != nil {
			return nil, err
		}
		r.Blocked = blocked != 0
		records = append(records, r)
	}
	return &QueryResult{Total: total, Records: records}, rows.Err()
}

func (s *store) deleteOlderThan(before time.Time) error {
	_, err := s.db.Exec("DELETE FROM access_logs WHERE connected_at < ?", before.UnixMilli())
	return err
}

func (s *store) close() error {
	return s.db.Close()
}

func buildWhere(p QueryParams) (string, []any) {
	var clauses []string
	var args []any

	if p.ProxyName != "" {
		clauses = append(clauses, "proxy_name = ?")
		args = append(args, p.ProxyName)
	}
	if p.RemoteIP != "" {
		clauses = append(clauses, "remote_ip = ?")
		args = append(args, p.RemoteIP)
	}
	if p.StartTime > 0 {
		clauses = append(clauses, "connected_at >= ?")
		args = append(args, p.StartTime)
	}
	if p.EndTime > 0 {
		clauses = append(clauses, "connected_at <= ?")
		args = append(args, p.EndTime)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
