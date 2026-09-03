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

package parser

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var walkTestExprs = []string{
	`foo`,
	`sum(rate(foo[5m])) + max(bar) / baz`,
	`histogram_quantile(0.9, sum by (le) (rate(foo_bucket[5m]))) > on (job) group_left count(up == 1)`,
	`(a + b) * (c + d) - (e + f) / (g + h)`,
	`sum_over_time((foo{a="b"} unless bar)[10m:1m]) or vector(0)`,
}

// depthFirst is a reference traversal: the pre-order, left-to-right walk that
// Walk is expected to perform.
func depthFirst(node Node) []Node {
	visited := []Node{node}
	for _, child := range Children(node) {
		visited = append(visited, depthFirst(child)...)
	}
	return visited
}

// TestWalkVisitsSequentially pins read-only walks (no NodeReplacer) to a
// single-goroutine, depth-first traversal. Those walks do no I/O, so there is
// nothing to overlap, and visitors all over prometheus keep unsynchronised
// per-walk state (see TestWalkVisitorStateNeedsNoSynchronisation).
func TestWalkVisitsSequentially(t *testing.T) {
	for _, exprStr := range walkTestExprs {
		t.Run(exprStr, func(t *testing.T) {
			expr, err := ParseExpr(exprStr)
			require.NoError(t, err)

			// Deliberately unsynchronised: a parallel Walk both reorders
			// and races on this slice.
			var visited []Node
			_, err = Inspect(context.Background(), &EvalStmt{Expr: expr}, func(node Node, _ []Node) error {
				// Walk signals "done with this subtree" with a nil node.
				if node != nil {
					visited = append(visited, node)
				}
				return nil
			}, nil)
			require.NoError(t, err)
			require.Equal(t, depthFirst(expr), visited)
		})
	}
}

// TestWalkVisitorStateNeedsNoSynchronisation reproduces the shape of
// rules.buildDependencyMap, which writes to a plain map from its visitor. A
// concurrent read-only Walk turns that into "fatal error: concurrent map
// writes" (promxy#809); -race flags it too.
func TestWalkVisitorStateNeedsNoSynchronisation(t *testing.T) {
	for _, exprStr := range walkTestExprs {
		t.Run(exprStr, func(t *testing.T) {
			expr, err := ParseExpr(exprStr)
			require.NoError(t, err)

			for i := 0; i < 50; i++ {
				counts := make(map[string]int)
				_, err := Inspect(context.Background(), &EvalStmt{Expr: expr}, func(node Node, _ []Node) error {
					if vs, ok := node.(*VectorSelector); ok {
						counts[vs.Name]++
					}
					return nil
				}, nil)
				require.NoError(t, err)
			}
		})
	}
}

// TestWalkFansOutWithNodeReplacer covers the other half of the contract: when
// a NodeReplacer is installed the walk is doing I/O (promxy replaces
// pushed-down subtrees with the result of a downstream query, and
// populateSeries calls querier.Select per selector), so sibling subtrees must
// be visited concurrently or a fan-out query pays the sum of its round trips.
func TestWalkFansOutWithNodeReplacer(t *testing.T) {
	expr, err := ParseExpr(`sum(a) + sum(b) + sum(c) + sum(d)`)
	require.NoError(t, err)

	var (
		mu             sync.Mutex
		inFlight, peak int
	)
	// A NodeReplacer that never replaces anything still selects the
	// parallel path -- its presence is what marks the walk as I/O bearing.
	nr := func(context.Context, *EvalStmt, Node, []Node) (Node, error) { return nil, nil }

	_, err = Inspect(context.Background(), &EvalStmt{Expr: expr}, func(node Node, _ []Node) error {
		if _, ok := node.(*VectorSelector); !ok {
			return nil
		}
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()

		// Stand in for a downstream round trip.
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		return nil
	}, nr)
	require.NoError(t, err)
	require.Greater(t, peak, 1, "sibling selectors were fetched one at a time")
}

// TestWalkReplacesChildrenWithNodeReplacer checks that the parallel path still
// installs replacements: SetChild runs once every sibling goroutine has
// joined, so the rewritten AST must come back fully substituted.
func TestWalkReplacesChildrenWithNodeReplacer(t *testing.T) {
	expr, err := ParseExpr(`sum(a) + sum(b) + sum(c) + sum(d)`)
	require.NoError(t, err)

	// Replace every VectorSelector foo with foo{replaced="true"}.
	nr := func(_ context.Context, _ *EvalStmt, node Node, _ []Node) (Node, error) {
		vs, ok := node.(*VectorSelector)
		if !ok || vs.Name == "" {
			return nil, nil
		}
		replaced, err := ParseExpr(vs.Name + `{replaced="true"}`)
		if err != nil {
			return nil, err
		}
		return replaced, nil
	}

	stmt := &EvalStmt{Expr: expr}
	n, err := Inspect(context.Background(), stmt, func(Node, []Node) error { return nil }, nr)
	require.NoError(t, err)
	require.Equal(t,
		`sum(a{replaced="true"}) + sum(b{replaced="true"}) + sum(c{replaced="true"}) + sum(d{replaced="true"})`,
		n.String())
}
