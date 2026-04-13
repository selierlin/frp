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

	"github.com/fatedier/frp/pkg/util/log"
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
	blocked      INTEGER NOT NULL DEFAULT 0,
	event        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_proxy_time ON access_logs(proxy_name, connected_at DESC);
CREATE INDEX IF NOT EXISTS idx_ip_time    ON access_logs(remote_ip,  connected_at DESC);
CREATE INDEX IF NOT EXISTS idx_event      ON access_logs(event, connected_at DESC);
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
	st := &store{db: db}
	// On startup, mark any records still in "connected" state as "disconnected".
	// These are stale entries left by a previous unclean shutdown (e.g. SIGKILL).
	if err := st.closeStaleConnected(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("close stale connected records: %w", err)
	}
	return st, nil
}

func (s *store) insert(r *Record) (int64, error) {
	blocked := 0
	if r.Blocked {
		blocked = 1
	}
	res, err := s.db.Exec(`INSERT INTO access_logs
		(proxy_name, proxy_type, proxy_user, remote_ip, remote_port,
		 connected_at, duration, traffic_in, traffic_out,
		 user_agent, host, url, status_code, blocked, event)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ProxyName, r.ProxyType, r.ProxyUser, r.RemoteIP, r.RemotePort,
		r.ConnectedAt, r.Duration, r.TrafficIn, r.TrafficOut,
		r.UserAgent, r.Host, r.URL, r.StatusCode, blocked, r.Event,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *store) updateOnClose(id, duration, trafficIn, trafficOut int64) error {
	_, err := s.db.Exec(
		`UPDATE access_logs SET duration=?, traffic_in=?, traffic_out=?, event=? WHERE id=?`,
		duration, trafficIn, trafficOut, EventDisconnected, id,
	)
	return err
}

// closeStaleConnected marks all records with event="connected" as "disconnected".
// These records are leftovers from a previous unclean shutdown (e.g. SIGKILL / OOM).
// Called once at startup before any new records are written.
func (s *store) closeStaleConnected() error {
	result, err := s.db.Exec(
		`UPDATE access_logs SET event=? WHERE event=?`,
		EventDisconnected, EventConnected,
	)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n > 0 {
		log.Infof("accesslog: closed %d stale 'connected' records left by previous unclean shutdown", n)
	}
	return nil
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
		 user_agent, host, url, status_code, blocked, event)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
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
			r.UserAgent, r.Host, r.URL, r.StatusCode, blocked, r.Event,
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
		"SELECT id,proxy_name,proxy_type,proxy_user,remote_ip,remote_port,connected_at,duration,traffic_in,traffic_out,user_agent,host,url,status_code,blocked,event FROM access_logs%s ORDER BY connected_at DESC LIMIT ? OFFSET ?",
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
			&r.StatusCode, &blocked, &r.Event,
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
	if p.Event != "" {
		clauses = append(clauses, "event = ?")
		args = append(args, p.Event)
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
