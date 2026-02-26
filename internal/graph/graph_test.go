package graph

import (
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	before := time.Now()
	n := NewNode("proj-1", NodeProject, "camp-graph", "/projects/camp-graph")
	after := time.Now()

	if n.ID != "proj-1" {
		t.Errorf("ID: got %q, want %q", n.ID, "proj-1")
	}
	if n.Type != NodeProject {
		t.Errorf("Type: got %q, want %q", n.Type, NodeProject)
	}
	if n.Name != "camp-graph" {
		t.Errorf("Name: got %q, want %q", n.Name, "camp-graph")
	}
	if n.Path != "/projects/camp-graph" {
		t.Errorf("Path: got %q, want %q", n.Path, "/projects/camp-graph")
	}
	if n.Metadata == nil {
		t.Fatal("Metadata must not be nil — writing to nil map panics")
	}
	// Verify Metadata is writable (would panic if nil).
	n.Metadata["test-key"] = "test-val"

	if n.CreatedAt.IsZero() {
		t.Error("CreatedAt must not be zero")
	}
	if n.UpdatedAt.IsZero() {
		t.Error("UpdatedAt must not be zero")
	}
	if n.CreatedAt.Before(before) || n.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v is outside expected range [%v, %v]", n.CreatedAt, before, after)
	}
	if n.UpdatedAt.Before(before) || n.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt %v is outside expected range [%v, %v]", n.UpdatedAt, before, after)
	}
}

func TestNewEdge(t *testing.T) {
	before := time.Now()
	e := NewEdge("proj-1", "fest-1", EdgeContains, 1.0, SourceStructural)
	after := time.Now()

	if e.FromID != "proj-1" {
		t.Errorf("FromID: got %q, want %q", e.FromID, "proj-1")
	}
	if e.ToID != "fest-1" {
		t.Errorf("ToID: got %q, want %q", e.ToID, "fest-1")
	}
	if e.Type != EdgeContains {
		t.Errorf("Type: got %q, want %q", e.Type, EdgeContains)
	}
	if e.Confidence != 1.0 {
		t.Errorf("Confidence: got %v, want 1.0", e.Confidence)
	}
	if e.Source != SourceStructural {
		t.Errorf("Source: got %q, want %q", e.Source, SourceStructural)
	}
	if e.CreatedAt.IsZero() {
		t.Error("CreatedAt must not be zero")
	}
	if e.CreatedAt.Before(before) || e.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v outside expected range [%v, %v]", e.CreatedAt, before, after)
	}
}

func TestAddNode_ReturnValue(t *testing.T) {
	tests := []struct {
		name         string
		insertFirst  bool
		wantReplaced bool
	}{
		{
			name:         "new node returns false",
			insertFirst:  false,
			wantReplaced: false,
		},
		{
			name:         "duplicate node returns true",
			insertFirst:  true,
			wantReplaced: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := New()
			n := NewNode("node-1", NodeProject, "camp-graph", "/projects/camp-graph")

			if tc.insertFirst {
				g.AddNode(n)
			}

			replaced := g.AddNode(n)
			if replaced != tc.wantReplaced {
				t.Errorf("AddNode() returned %v, want %v", replaced, tc.wantReplaced)
			}
		})
	}
}

func TestEdges_DefensiveCopy(t *testing.T) {
	g := New()
	e1 := NewEdge("a", "b", EdgeContains, 1.0, SourceStructural)
	e2 := NewEdge("b", "c", EdgeDependsOn, 0.9, SourceInferred)
	g.AddEdge(e1)
	g.AddEdge(e2)

	edges := g.Edges()
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	// Append a spurious edge to the returned slice.
	spurious := NewEdge("x", "y", EdgeRelatesTo, 0.5, SourceInferred)
	_ = append(edges, spurious)

	if g.EdgeCount() != 2 {
		t.Errorf("EdgeCount after mutating returned slice: got %d, want 2 — Edges() is not returning a copy", g.EdgeCount())
	}
}

func TestNodes_DefensiveCopy(t *testing.T) {
	g := New()
	n1 := NewNode("a", NodeProject, "proj-a", "/a")
	n2 := NewNode("b", NodeFestival, "fest-b", "/b")
	g.AddNode(n1)
	g.AddNode(n2)

	nodes := g.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Append a spurious node to the returned slice.
	spurious := NewNode("z", NodeTask, "task-z", "/z")
	_ = append(nodes, spurious)

	if g.NodeCount() != 2 {
		t.Errorf("NodeCount after mutating returned slice: got %d, want 2 — Nodes() is not returning a copy", g.NodeCount())
	}
}

func TestNeighbors(t *testing.T) {
	tests := []struct {
		name            string
		setupGraph      func(g *Graph)
		queryID         string
		wantNeighborIDs []string
	}{
		{
			name: "outgoing edge found",
			setupGraph: func(g *Graph) {
				g.AddNode(NewNode("a", NodeProject, "a", "/a"))
				g.AddNode(NewNode("b", NodeFestival, "b", "/b"))
				g.AddEdge(NewEdge("a", "b", EdgeContains, 1.0, SourceStructural))
			},
			queryID:         "a",
			wantNeighborIDs: []string{"b"},
		},
		{
			name: "incoming edge found",
			setupGraph: func(g *Graph) {
				g.AddNode(NewNode("a", NodeProject, "a", "/a"))
				g.AddNode(NewNode("b", NodeFestival, "b", "/b"))
				g.AddEdge(NewEdge("a", "b", EdgeContains, 1.0, SourceStructural))
			},
			queryID:         "b",
			wantNeighborIDs: []string{"a"},
		},
		{
			name: "bidirectional edge deduplicated",
			setupGraph: func(g *Graph) {
				g.AddNode(NewNode("a", NodeProject, "a", "/a"))
				g.AddNode(NewNode("b", NodeFestival, "b", "/b"))
				g.AddEdge(NewEdge("a", "b", EdgeContains, 1.0, SourceStructural))
				g.AddEdge(NewEdge("b", "a", EdgeDependsOn, 0.8, SourceInferred))
			},
			queryID:         "a",
			wantNeighborIDs: []string{"b"},
		},
		{
			name: "node with no edges",
			setupGraph: func(g *Graph) {
				g.AddNode(NewNode("a", NodeProject, "a", "/a"))
			},
			queryID:         "a",
			wantNeighborIDs: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := New()
			tc.setupGraph(g)

			neighbors := g.Neighbors(tc.queryID)

			got := make(map[string]bool, len(neighbors))
			for _, n := range neighbors {
				got[n.ID] = true
			}

			if len(got) != len(tc.wantNeighborIDs) {
				t.Errorf("Neighbors(%q) returned %d nodes, want %d",
					tc.queryID, len(got), len(tc.wantNeighborIDs))
				return
			}
			for _, wantID := range tc.wantNeighborIDs {
				if !got[wantID] {
					t.Errorf("Neighbors(%q): expected neighbor %q not found",
						tc.queryID, wantID)
				}
			}
		})
	}
}
