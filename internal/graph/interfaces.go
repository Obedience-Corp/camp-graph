package graph

import "context"

// Persister saves and loads graphs from persistent storage.
type Persister interface {
	Save(ctx context.Context, g *Graph) error
	Load(ctx context.Context) (*Graph, error)
	Close() error
}

// Querier retrieves nodes and edges from storage.
type Querier interface {
	GetNode(ctx context.Context, id string) (*Node, error)
	GetNodesByType(ctx context.Context, t NodeType) ([]*Node, error)
	GetAllNodes(ctx context.Context) ([]*Node, error)
	GetAllEdges(ctx context.Context) ([]*Edge, error)
}
