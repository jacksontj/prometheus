// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promqltest

import (
	"time"

	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/testutil"
)

// Test is the exported handle for parsed PromQL tests. It carries a parsed
// command list, a backing storage, and a query engine. This pre-3.x shape
// is preserved for fork consumers (notably promxy) that need to attach a
// NodeReplacer to the engine before running, or to swap the storage for
// a wrapping layer between parsing and execution.
type Test struct {
	t      *test
	engine *promql.Engine
	closed bool
}

// NewTest parses the given test input and returns a Test ready to Run.
func NewTest(t testutil.T, input string) (*Test, error) {
	inner, err := newTest(t, input, false, newTestStorage)
	if err != nil {
		return nil, err
	}
	engine := promql.NewEngine(promql.EngineOpts{
		Logger:                   nil,
		Reg:                      nil,
		MaxSamples:               10000,
		Timeout:                  100 * time.Second,
		NoStepSubqueryIntervalFn: func(int64) int64 { return durationMilliseconds(1 * time.Minute) },
		EnableAtModifier:         true,
		EnableNegativeOffset:     true,
		EnableDelayedNameRemoval: true,
	})
	return &Test{t: inner, engine: engine}, nil
}

// Storage returns the storage backing the Test.
func (t *Test) Storage() storage.Storage { return t.t.storage }

// Queryable returns the storage as a storage.Queryable.
func (t *Test) Queryable() storage.Queryable { return t.t.storage }

// QueryEngine returns the query engine the Test will use when Run is called.
func (t *Test) QueryEngine() *promql.Engine { return t.engine }

// SetStorage overrides the storage used during Run. The previous storage is
// closed.
func (t *Test) SetStorage(s storage.Storage) { t.t.SetStorage(s) }

// Run executes all parsed commands against the Test's engine and storage.
func (t *Test) Run() error {
	for _, cmd := range t.t.cmds {
		if err := t.t.exec(cmd, t.engine); err != nil {
			return err
		}
	}
	return nil
}

// Close releases all resources held by the Test. Safe to call multiple times.
//
// Storage close errors are swallowed: callers may have wrapped t.Storage()
// in a layered storage that itself owns the underlying handle, in which
// case calling Close again here would double-close.
func (t *Test) Close() {
	if t.t == nil || t.closed {
		return
	}
	t.closed = true
	if t.t.storage != nil {
		// The storage may have been wrapped via SetStorage and shares its
		// underlying handle with code paths that already closed it; recover
		// to keep Close idempotent for those cases.
		func() {
			defer func() { _ = recover() }()
			t.t.storage.Close()
		}()
	}
	if t.t.cancelCtx != nil {
		t.t.cancelCtx()
	}
}
