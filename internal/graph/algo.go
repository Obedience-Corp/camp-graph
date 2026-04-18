package graph

import (
	libgraph "github.com/dominikbraun/graph"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// ToLibGraph converts the Graph into a dominikbraun/graph directed graph
// for algorithm support (topological sort, shortest path, etc.).
func (g *Graph) ToLibGraph() (libgraph.Graph[string, string], error) {
	lg := libgraph.New(libgraph.StringHash, libgraph.Directed())
	for _, n := range g.nodes {
		if err := lg.AddVertex(n.ID); err != nil {
			return lg, graphErrors.Wrapf(err, "add vertex %s", n.ID)
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
		return nil, graphErrors.Wrap(err, "convert to lib graph")
	}
	order, err := libgraph.TopologicalSort(lg)
	if err != nil {
		return nil, graphErrors.Wrap(err, "topological sort")
	}
	return order, nil
}

// ShortestPath returns the shortest path between two nodes as a list of node IDs.
func (g *Graph) ShortestPath(from, to string) ([]string, error) {
	lg, err := g.ToLibGraph()
	if err != nil {
		return nil, graphErrors.Wrap(err, "convert to lib graph")
	}
	path, err := libgraph.ShortestPath(lg, from, to)
	if err != nil {
		return nil, graphErrors.Wrapf(err, "shortest path from %s to %s", from, to)
	}
	return path, nil
}
