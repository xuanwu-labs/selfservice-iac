// Package dependency builds and analyzes the stack dependency DAG.
//
// A catalog item's dependencies (stack_dependencies rows, loaded via
// StackDependencyRepo) form a directed graph where an edge From→To means
// "From depends on To", i.e. To must be applied first. The codegen pipeline
// needs the nodes in execution order (dependencies before dependents) and a
// way to reject cyclic catalogs up front.
//
// This package is pure: it takes a slice of Edge values and returns the sorted
// node list or a cycle error. No DB access, no I/O — the caller (W2 codegen)
// is responsible for converting stack_dependencies rows into []Edge.
package dependency

import (
	"fmt"
	"sort"
	"strings"
)

// Edge is a single dependency: From depends on To. In execution order, To
// comes before From. Both endpoints are opaque node identifiers (typically
// stack_id strings or component slugs — whatever the caller normalized to).
type Edge struct {
	From string
	To   string
}

// TopoSort returns nodes in execution order: every edge's To appears before
// its From (dependencies first). The sort is implemented with Kahn's algorithm
// (in-degree counting + BFS over zero-in-degree nodes).
//
// Determinism: when multiple nodes are simultaneously ready (in-degree 0),
// they are emitted in lexicographic order. This makes the output stable for a
// given input regardless of map iteration order — required by D19 codegen
// determinism so that two runs over the same catalog produce identical files.
//
// On cycle: returns nil and an error whose message names the nodes involved in
// the remaining subgraph (see DetectCycle for the precise format). Callers that
// only need a yes/no cycle answer should call DetectCycle first.
func TopoSort(edges []Edge) ([]string, error) {
	if len(edges) == 0 {
		return []string{}, nil
	}

	// Collect the node set (preserving insertion order via a slice) and compute
	// in-degrees + adjacency lists. Self-loops (From==To) are cycles and are
	// detected below by the standard algorithm.
	nodes := make([]string, 0, len(edges)*2)
	seen := make(map[string]struct{}, len(edges)*2)
	inDegree := make(map[string]int, len(edges)*2)
	adj := make(map[string][]string, len(edges)*2) // To → [From...] (forward edges)

	addNode := func(n string) {
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		nodes = append(nodes, n)
		inDegree[n] = 0
		adj[n] = nil
	}

	for _, e := range edges {
		addNode(e.From)
		addNode(e.To)
		adj[e.To] = append(adj[e.To], e.From)
		inDegree[e.From]++
	}

	// Seed the ready set with all zero-in-degree nodes, then drain it in
	// lexicographic order to keep output deterministic.
	ready := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if inDegree[n] == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)

	out := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		// Pop the lexicographically smallest ready node.
		cur := ready[0]
		ready = ready[1:]
		out = append(out, cur)

		// Decrement in-degree of its forward neighbors. Any that hit 0 are
		// merged into ready, then we re-sort to preserve lexicographic order.
		merged := false
		for _, nb := range adj[cur] {
			inDegree[nb]--
			if inDegree[nb] == 0 {
				ready = append(ready, nb)
				merged = true
			}
		}
		if merged {
			sort.Strings(ready)
		}
	}

	if len(out) < len(nodes) {
		// Cycle: the nodes that never reached in-degree 0 are part of (or
		// downstream of) a cycle.
		leftover := make([]string, 0, len(nodes)-len(out))
		for _, n := range nodes {
			if inDegree[n] > 0 {
				leftover = append(leftover, n)
			}
		}
		return nil, fmt.Errorf("dependency: cycle detected involving nodes: %s", strings.Join(leftover, ", "))
	}
	return out, nil
}

// DetectCycle reports whether the edge set contains a cycle. Returns nil if
// the graph is acyclic; otherwise returns an error whose message lists the
// nodes that participate in (or depend on) the cycle, mirroring TopoSort's
// format. It is a thin wrapper around TopoSort so the two never disagree.
func DetectCycle(edges []Edge) error {
	_, err := TopoSort(edges)
	return err
}
