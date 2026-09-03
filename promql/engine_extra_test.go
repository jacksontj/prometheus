// Copyright 2023 The Prometheus Authors
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

package promql

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

type StubNode struct {
	start, end int
}

func (s StubNode) String() string {
	return fmt.Sprintf("[%d,%d]", s.start, s.end)
}

func (s StubNode) Pretty(int) string { return s.String() }

func (s StubNode) PositionRange() posrange.PositionRange {
	return posrange.PositionRange{
		Start: posrange.Pos(s.start),
		End:   posrange.Pos(s.end),
	}
}

func TestFindPathRange(t *testing.T) {
	var (
		root  = StubNode{0, 10}
		left  = StubNode{0, 4}
		inner = StubNode{1, 3}
		right = StubNode{6, 10}
	)

	tests := []struct {
		path    []parser.Node
		eRanges []evalRange
		out     time.Duration
	}{
		// Test a case where the evalRange is longer than the path
		{
			path: []parser.Node{root},
			eRanges: []evalRange{
				{Prefix: []parser.Node{root, left}, Range: time.Minute},
			},
		},
		// An ancestor of the path: its range applies
		{
			path: []parser.Node{root, left, inner},
			eRanges: []evalRange{
				{Prefix: []parser.Node{root, left}, Range: time.Minute},
			},
			out: time.Minute,
		},
		// The deepest matching prefix wins
		{
			path: []parser.Node{root, left, inner},
			eRanges: []evalRange{
				{Prefix: []parser.Node{root}, Range: time.Hour},
				{Prefix: []parser.Node{root, left}, Range: time.Minute},
			},
			out: time.Minute,
		},
		// A range recorded under a sibling subtree must not leak across
		{
			path: []parser.Node{root, right},
			eRanges: []evalRange{
				{Prefix: []parser.Node{root, left}, Range: time.Minute},
			},
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			out := findPathRange(test.path, test.eRanges)
			if !reflect.DeepEqual(out, test.out) {
				t.Fatalf("Mismatch in test output expected=%#v actual=%#v", test.out, out)
			}
		})
	}
}
