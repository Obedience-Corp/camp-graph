package graph

import (
	"fmt"

	libgraph "github.com/dominikbraun/graph"
)

// ToLibGraph converts the Graph into a dominikbraun/graph directed graph
// for algorithm support (topological sort, shortest path, etc.).
func (g *Graph) ToLibGraph() (libgraph.Graph[string, string], error) {
	lg := libgraph.New(libgraph.StringHash, libgraph.Directed())
	for _, n := range g.nodes {
		if err := lg.AddVertex(n.ID); err != nil {
			return lg, fmt.Errorf("add vertex %s: %w", n.ID, err)
		}
	}
	for _, e := range g.edges {
		if err := lg.AddEdge(e.FromID, e.ToID); err != nil {
			// Skip duplicate edges — dominikbraun/graph doesn't allow them.
			continue
		}
	}
	return lg, nil
}

// TopologicalSort returns node IDs in dependency order.
// Only meaningful when the graph has no cycles in depends_on edges.
func (g *Graph) TopologicalSort() ([]string, error) {
	lg, err := g.ToLibGraph()
	if err != nil {
		return nil, fmt.Errorf("convert to lib graph: %w", err)
	}
	order, err := libgraph.TopologicalSort(lg)
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}
	return order, nil
}

// ShortestPath returns the shortest path between two nodes as a list of node IDs.
func (g *Graph) ShortestPath(from, to string) ([]string, error) {
	lg, err := g.ToLibGraph()
	if err != nil {
		return nil, fmt.Errorf("convert to lib graph: %w", err)
	}
	path, err := libgraph.ShortestPath(lg, from, to)
	if err != nil {
		return nil, fmt.Errorf("shortest path from %s to %s: %w", from, to, err)
	}
	return path, nil
}
