// Package scanner walks the campaign filesystem and builds graph nodes and edges
// from the directory structure and metadata files.
package scanner

import (
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

// Scan walks the campaign filesystem and returns a populated graph.
func (s *Scanner) Scan() (*graph.Graph, error) {
	g := graph.New()
	// Phase 1: structural scan will be implemented here.
	_ = g
	return g, nil
}
