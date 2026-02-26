// Package scanner walks the campaign filesystem and builds graph nodes and edges
// from the directory structure and metadata files.
package scanner

import (
	"context"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Scanner walks a campaign directory and produces graph nodes and edges.
type Scanner struct {
	root string
}

// New creates a scanner rooted at the given campaign directory.
func New(root string) *Scanner {
	return &Scanner{root: root}
}

// Scan walks the campaign filesystem rooted at s.root and returns a populated graph.
// The context must be checked before each directory walk to support cancellation
// (e.g., user pressing Ctrl+C or a TUI timeout).
func (s *Scanner) Scan(ctx context.Context) (*graph.Graph, error) {
	g := graph.New()
	// Phase 1: structural scan will be implemented here.
	// Check ctx.Err() before each directory walk:
	//   if err := ctx.Err(); err != nil { return nil, err }
	return g, nil
}
