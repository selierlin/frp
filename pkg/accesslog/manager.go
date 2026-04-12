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
	"context"
	"sync"
	"time"

	"github.com/fatedier/frp/pkg/util/log"
)

// Writer is the interface for writing access log records.
type Writer interface {
	Write(r *Record)
	Query(p QueryParams) (*QueryResult, error)
	Close() error
}

// Default is the global Writer. It is a noop by default and never nil,
// so callers do not need nil checks.
var Default Writer = noopWriter{}

var registerOnce sync.Once

// Register sets the global Writer. It can only be called once.
func Register(w Writer) {
	registerOnce.Do(func() {
		Default = w
	})
}

// noopWriter silently discards all records.
type noopWriter struct{}

func (noopWriter) Write(*Record)                           {}
func (noopWriter) Query(QueryParams) (*QueryResult, error) { return &QueryResult{}, nil }
func (noopWriter) Close() error                            { return nil }

// Manager is the real Writer backed by SQLite.
// It uses a single background goroutine for all writes (batch insert),
// which avoids SQLite concurrent-write issues and keeps the hot path non-blocking.
type Manager struct {
	st          *store
	ch          chan *Record
	reserveDays int
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewManager opens the SQLite database and returns a Manager.
// Call Run(ctx) to start the background goroutine.
func NewManager(storagePath string, reserveDays, queueSize int) (*Manager, error) {
	st, err := newStore(storagePath)
	if err != nil {
		return nil, err
	}
	return &Manager{
		st:          st,
		ch:          make(chan *Record, queueSize),
		reserveDays: reserveDays,
		stopCh:      make(chan struct{}),
	}, nil
}

// Write enqueues a record for async persistence.
// If the queue is full the record is dropped and a warning is logged.
func (m *Manager) Write(r *Record) {
	select {
	case m.ch <- r:
	default:
		log.Warnf("accesslog: queue full, record dropped (proxy=%s remote=%s:%d)", r.ProxyName, r.RemoteIP, r.RemotePort)
	}
}

// Query executes a filtered, paginated query against the SQLite database.
func (m *Manager) Query(p QueryParams) (*QueryResult, error) {
	return m.st.query(p)
}

// Close stops the background goroutine and waits for all queued records to be
// flushed before closing the database.
func (m *Manager) Close() error {
	close(m.stopCh)
	m.wg.Wait()
	return m.st.close()
}

// Run starts the write goroutine and the daily cleanup goroutine.
// It blocks until ctx is cancelled; call it in a separate goroutine.
func (m *Manager) Run(ctx context.Context) {
	m.wg.Add(1)
	go m.writeLoop()
	m.cleanWorker(ctx)
}

// writeLoop is the single writer goroutine.
// It batches up to 100 records or flushes every 500 ms, whichever comes first.
func (m *Manager) writeLoop() {
	defer m.wg.Done()

	batch := make([]*Record, 0, 100)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := m.st.batchInsert(batch); err != nil {
			log.Warnf("accesslog: batch insert error: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case r := <-m.ch:
			batch = append(batch, r)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-m.stopCh:
			// Drain remaining records before exiting.
			for {
				select {
				case r := <-m.ch:
					batch = append(batch, r)
				default:
					flush()
					return
				}
			}
		}
	}
}

// cleanWorker runs a daily cleanup that removes records older than reserveDays.
// It exits when ctx is cancelled.
func (m *Manager) cleanWorker(ctx context.Context) {
	if m.reserveDays <= 0 {
		// Keep forever; nothing to clean.
		<-ctx.Done()
		return
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	doClean := func() {
		before := time.Now().AddDate(0, 0, -m.reserveDays)
		if err := m.st.deleteOlderThan(before); err != nil {
			log.Warnf("accesslog: cleanup error: %v", err)
		}
	}

	// Run once immediately on startup.
	doClean()

	for {
		select {
		case <-ticker.C:
			doClean()
		case <-ctx.Done():
			return
		}
	}
}
