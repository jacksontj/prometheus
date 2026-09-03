package promql

import (
	"time"

	"github.com/prometheus/prometheus/promql/parser"
)

func findPathRange(path []parser.Node, eRanges []evalRange) time.Duration {
	var (
		evalRange time.Duration
		depth     = -1
	)
	for _, r := range eRanges {
		// If the prefix is longer then it can't be the parent of `child`
		if len(r.Prefix) > len(path) {
			continue
		}

		// Check if we are a child
		child := true
		for i, p := range r.Prefix {
			if p != path[i] {
				child = false
				break
			}
		}
		if child && len(r.Prefix) > depth {
			evalRange = r.Range
			depth = len(r.Prefix)
		}
	}

	return evalRange
}

// evalRange summarizes a defined evalRange (from a MatrixSelector) within the ast
//
// Prefix identifies the ancestor chain by node pointer rather than by
// PositionRange: parser.Walk fans siblings out when a NodeReplacer is
// installed, and PositionRange() recurses into a node's children, so reading
// it here would race with the SetChild calls a sibling subtree performs.
type evalRange struct {
	Prefix []parser.Node
	Range  time.Duration
}
