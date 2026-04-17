package main

import (
	"fmt"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// sliceByScope returns a new graph containing only nodes whose
// relative path lives inside the given scope, plus every edge that
// connects two nodes inside the slice. If the scope is unknown the
// function returns an error so the CLI can surface it clearly.
func sliceByScope(g *graph.Graph, scope string) (*graph.Graph, error) {
	scope = strings.TrimSuffix(scope, "/")
	if scope == "" {
		return g, nil
	}
	// Verify the scope exists as a folder node.
	if g.Node("folder:"+scope) == nil {
		return nil, fmt.Errorf("scope %q not found in graph", scope)
	}
	prefix := scope + "/"

	keep := map[string]bool{}
	out := graph.New()
	for _, n := range g.Nodes() {
		if !nodeMatchesScope(n, scope, prefix) {
			continue
		}
		out.AddNode(n)
		keep[n.ID] = true
	}
	for _, e := range g.Edges() {
		if keep[e.FromID] && keep[e.ToID] {
			out.AddEdge(e)
		}
	}
	return out, nil
}

// nodeMatchesScope reports whether a node belongs to scope. Match
// semantics: a node's Name (for path-backed nodes) or Path field must
// equal the scope or live under scope/.
func nodeMatchesScope(n *graph.Node, scope, prefix string) bool {
	candidates := []string{n.Name}
	if n.Path != "" {
		candidates = append(candidates, n.Path)
	}
	for _, c := range candidates {
		c = strings.TrimSuffix(c, "/")
		if c == scope || strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// filterByRelationMode returns a new graph where only edges matching
// the relation mode are kept. Nodes remain even if they lose all
// edges so subgraph rendering remains meaningful.
func filterByRelationMode(g *graph.Graph, mode string) *graph.Graph {
	allowed := relationSourceFilter(mode)
	out := graph.New()
	for _, n := range g.Nodes() {
		out.AddNode(n)
	}
	for _, e := range g.Edges() {
		if allowed == nil || allowed[e.Source] {
			out.AddEdge(e)
		}
	}
	return out
}

func relationSourceFilter(mode string) map[graph.ConfidenceSource]bool {
	switch strings.ToLower(mode) {
	case "structural":
		return map[graph.ConfidenceSource]bool{graph.SourceStructural: true}
	case "explicit":
		return map[graph.ConfidenceSource]bool{graph.SourceExplicit: true}
	case "semantic":
		return map[graph.ConfidenceSource]bool{graph.SourceInferred: true}
	case "hybrid", "":
		return nil
	}
	return nil
}

// filterByTrackedState keeps nodes whose metadata records the given
// git state, plus folder and artifact scopes that lack a git_state
// field entirely (so scope scaffolding remains visible).
func filterByTrackedState(g *graph.Graph, state string) *graph.Graph {
	out := graph.New()
	keep := map[string]bool{}
	for _, n := range g.Nodes() {
		ns := n.Metadata[graph.MetaGitState]
		// Keep scope nodes and artifacts that do not record state.
		if ns == "" || ns == state {
			out.AddNode(n)
			keep[n.ID] = true
		}
	}
	for _, e := range g.Edges() {
		if keep[e.FromID] && keep[e.ToID] {
			out.AddEdge(e)
		}
	}
	return out
}
