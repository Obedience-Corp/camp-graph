package graph

// Graph holds the in-memory knowledge graph of campaign artifacts.
// A Graph is not safe for concurrent use. Callers that share a Graph
// across goroutines (e.g., a background scanner and a TUI renderer)
// must synchronize access with a mutex or channel-based ownership.
type Graph struct {
	nodes map[string]*Node
	edges []*Edge
}

// New creates an empty knowledge graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
	}
}

// AddNode inserts a node into the graph.
// Returns true if a node with the same ID already existed and was replaced.
// The new node is always stored regardless — callers may log or handle
// duplicate IDs as appropriate for their use case.
func (g *Graph) AddNode(n *Node) bool {
	_, existed := g.nodes[n.ID]
	g.nodes[n.ID] = n
	return existed
}

// AddEdge inserts a directed edge between two nodes.
func (g *Graph) AddEdge(e *Edge) {
	g.edges = append(g.edges, e)
}

// Node returns a node by ID, or nil if not found.
func (g *Graph) Node(id string) *Node {
	return g.nodes[id]
}

// Nodes returns all nodes in the graph.
func (g *Graph) Nodes() []*Node {
	out := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	return out
}

// Edges returns all edges in the graph.
func (g *Graph) Edges() []*Edge {
	out := make([]*Edge, len(g.edges))
	copy(out, g.edges)
	return out
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	return len(g.edges)
}

// Neighbors returns all nodes directly connected to the given node ID.
func (g *Graph) Neighbors(id string) []*Node {
	seen := make(map[string]bool)
	var result []*Node

	for _, e := range g.edges {
		var neighborID string
		if e.FromID == id {
			neighborID = e.ToID
		} else if e.ToID == id {
			neighborID = e.FromID
		} else {
			continue
		}

		if !seen[neighborID] {
			seen[neighborID] = true
			if n := g.nodes[neighborID]; n != nil {
				result = append(result, n)
			}
		}
	}
	return result
}
