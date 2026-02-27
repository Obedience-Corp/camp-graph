package graph

import (
	"sort"
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

func TestNodeTypeString(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		want     string
	}{
		{NodeProject, "project"},
		{NodeFestival, "festival"},
		{NodeChain, "chain"},
		{NodePhase, "phase"},
		{NodeSequence, "sequence"},
		{NodeTask, "task"},
		{NodeIntent, "intent"},
		{NodeDesignDoc, "design_doc"},
		{NodeExploreDoc, "explore_doc"},
		{NodeFile, "file"},
		{NodeFunction, "function"},
		{NodeTypeDef, "type_def"},
		{NodePackage, "package"},
		{NodeType("custom_thing"), "custom_thing"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.nodeType.String(); got != tc.want {
				t.Errorf("NodeType(%q).String() = %q, want %q", tc.nodeType, got, tc.want)
			}
		})
	}
}

func TestEdgeTypeString(t *testing.T) {
	tests := []struct {
		edgeType EdgeType
		want     string
	}{
		{EdgeContains, "contains"},
		{EdgeChainMember, "chain_member"},
		{EdgeDependsOn, "depends_on"},
		{EdgeLinksTo, "links_to"},
		{EdgeRelatesTo, "relates_to"},
		{EdgeGatheredFrom, "gathered_from"},
		{EdgeReferences, "references"},
		{EdgeSimilarTo, "similar_to"},
		{EdgeDefines, "defines"},
		{EdgeCalls, "calls"},
		{EdgeImports, "imports"},
		{EdgeModifies, "modifies"},
		{EdgeType("custom_edge"), "custom_edge"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.edgeType.String(); got != tc.want {
				t.Errorf("EdgeType(%q).String() = %q, want %q", tc.edgeType, got, tc.want)
			}
		})
	}
}

func TestNodesByType(t *testing.T) {
	g := New()
	g.AddNode(NewNode("p1", NodeProject, "proj-1", "/p1"))
	g.AddNode(NewNode("p2", NodeProject, "proj-2", "/p2"))
	g.AddNode(NewNode("f1", NodeFestival, "fest-1", "/f1"))
	g.AddNode(NewNode("t1", NodeTask, "task-1", "/t1"))

	projects := g.NodesByType(NodeProject)
	if len(projects) != 2 {
		t.Fatalf("NodesByType(NodeProject): got %d, want 2", len(projects))
	}
	ids := make(map[string]bool)
	for _, n := range projects {
		ids[n.ID] = true
	}
	if !ids["p1"] || !ids["p2"] {
		t.Errorf("expected p1 and p2, got %v", ids)
	}

	// No matches returns nil/empty slice.
	intents := g.NodesByType(NodeIntent)
	if len(intents) != 0 {
		t.Errorf("NodesByType(NodeIntent): got %d, want 0", len(intents))
	}
}

func TestEdgesFrom(t *testing.T) {
	g := New()
	g.AddNode(NewNode("a", NodeProject, "a", "/a"))
	g.AddNode(NewNode("b", NodeFestival, "b", "/b"))
	g.AddNode(NewNode("c", NodeTask, "c", "/c"))
	g.AddEdge(NewEdge("a", "b", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("a", "c", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("b", "c", EdgeDependsOn, 0.9, SourceInferred))

	from := g.EdgesFrom("a")
	if len(from) != 2 {
		t.Fatalf("EdgesFrom(a): got %d, want 2", len(from))
	}

	fromB := g.EdgesFrom("b")
	if len(fromB) != 1 {
		t.Fatalf("EdgesFrom(b): got %d, want 1", len(fromB))
	}

	fromC := g.EdgesFrom("c")
	if len(fromC) != 0 {
		t.Errorf("EdgesFrom(c): got %d, want 0", len(fromC))
	}
}

func TestEdgesTo(t *testing.T) {
	g := New()
	g.AddNode(NewNode("a", NodeProject, "a", "/a"))
	g.AddNode(NewNode("b", NodeFestival, "b", "/b"))
	g.AddNode(NewNode("c", NodeTask, "c", "/c"))
	g.AddEdge(NewEdge("a", "b", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("a", "c", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("b", "c", EdgeDependsOn, 0.9, SourceInferred))

	toC := g.EdgesTo("c")
	if len(toC) != 2 {
		t.Fatalf("EdgesTo(c): got %d, want 2", len(toC))
	}

	toA := g.EdgesTo("a")
	if len(toA) != 0 {
		t.Errorf("EdgesTo(a): got %d, want 0", len(toA))
	}
}

func TestSubgraph(t *testing.T) {
	// Build chain: A -> B -> C -> D
	g := New()
	g.AddNode(NewNode("A", NodeProject, "A", "/A"))
	g.AddNode(NewNode("B", NodeFestival, "B", "/B"))
	g.AddNode(NewNode("C", NodeTask, "C", "/C"))
	g.AddNode(NewNode("D", NodeFile, "D", "/D"))
	g.AddEdge(NewEdge("A", "B", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("B", "C", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("C", "D", EdgeContains, 1.0, SourceStructural))

	sub := g.Subgraph("B", 1)
	if sub.NodeCount() != 3 {
		t.Errorf("Subgraph(B,1) nodes: got %d, want 3", sub.NodeCount())
	}
	for _, id := range []string{"A", "B", "C"} {
		if sub.Node(id) == nil {
			t.Errorf("Subgraph(B,1): expected node %s", id)
		}
	}
	if sub.Node("D") != nil {
		t.Error("Subgraph(B,1): D should not be included")
	}
	if sub.EdgeCount() != 2 {
		t.Errorf("Subgraph(B,1) edges: got %d, want 2", sub.EdgeCount())
	}

	// Non-existent node returns empty graph.
	empty := g.Subgraph("Z", 1)
	if empty.NodeCount() != 0 {
		t.Errorf("Subgraph(Z,1): got %d nodes, want 0", empty.NodeCount())
	}

	// Zero hops returns only the center node.
	center := g.Subgraph("B", 0)
	if center.NodeCount() != 1 {
		t.Errorf("Subgraph(B,0): got %d nodes, want 1", center.NodeCount())
	}
	if center.Node("B") == nil {
		t.Error("Subgraph(B,0): expected node B")
	}
}

func TestTopologicalSort(t *testing.T) {
	g := New()
	g.AddNode(NewNode("A", NodeProject, "A", "/A"))
	g.AddNode(NewNode("B", NodeFestival, "B", "/B"))
	g.AddNode(NewNode("C", NodeTask, "C", "/C"))
	g.AddNode(NewNode("D", NodeFile, "D", "/D"))
	g.AddEdge(NewEdge("A", "B", EdgeDependsOn, 1.0, SourceStructural))
	g.AddEdge(NewEdge("B", "C", EdgeDependsOn, 1.0, SourceStructural))
	g.AddEdge(NewEdge("C", "D", EdgeDependsOn, 1.0, SourceStructural))

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("TopologicalSort: got %d nodes, want 4", len(order))
	}

	// Verify ordering: A must come before B, B before C, C before D.
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	if pos["A"] > pos["B"] {
		t.Errorf("A (pos %d) should come before B (pos %d)", pos["A"], pos["B"])
	}
	if pos["B"] > pos["C"] {
		t.Errorf("B (pos %d) should come before C (pos %d)", pos["B"], pos["C"])
	}
	if pos["C"] > pos["D"] {
		t.Errorf("C (pos %d) should come before D (pos %d)", pos["C"], pos["D"])
	}
}

func TestShortestPath(t *testing.T) {
	g := New()
	g.AddNode(NewNode("A", NodeProject, "A", "/A"))
	g.AddNode(NewNode("B", NodeFestival, "B", "/B"))
	g.AddNode(NewNode("C", NodeTask, "C", "/C"))
	g.AddNode(NewNode("D", NodeFile, "D", "/D"))
	g.AddEdge(NewEdge("A", "B", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("B", "C", EdgeContains, 1.0, SourceStructural))
	g.AddEdge(NewEdge("C", "D", EdgeContains, 1.0, SourceStructural))

	path, err := g.ShortestPath("A", "D")
	if err != nil {
		t.Fatalf("ShortestPath(A, D): %v", err)
	}
	want := []string{"A", "B", "C", "D"}
	if len(path) != len(want) {
		t.Fatalf("ShortestPath(A, D): got %v, want %v", path, want)
	}
	for i, id := range want {
		if path[i] != id {
			t.Errorf("ShortestPath[%d]: got %q, want %q", i, path[i], id)
		}
	}
}

func TestShortestPath_NoPath(t *testing.T) {
	g := New()
	g.AddNode(NewNode("A", NodeProject, "A", "/A"))
	g.AddNode(NewNode("B", NodeFestival, "B", "/B"))
	// No edge between A and B.

	_, err := g.ShortestPath("A", "B")
	if err == nil {
		t.Error("ShortestPath with no path should return error")
	}
}

func TestToLibGraph(t *testing.T) {
	g := New()
	g.AddNode(NewNode("A", NodeProject, "A", "/A"))
	g.AddNode(NewNode("B", NodeFestival, "B", "/B"))
	g.AddEdge(NewEdge("A", "B", EdgeContains, 1.0, SourceStructural))

	lg, err := g.ToLibGraph()
	if err != nil {
		t.Fatalf("ToLibGraph: %v", err)
	}

	// Verify vertices exist.
	order, err := lg.Order()
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if order != 2 {
		t.Errorf("Order: got %d, want 2", order)
	}

	size, err := lg.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if size != 1 {
		t.Errorf("Size: got %d, want 1", size)
	}
}

// Unused import guard — sort is used in TestSubgraph_LargerHops.
func TestSubgraph_LargerHops(t *testing.T) {
	// Star topology: center -> a, b, c, d
	g := New()
	g.AddNode(NewNode("center", NodeProject, "center", "/center"))
	for _, id := range []string{"a", "b", "c", "d"} {
		g.AddNode(NewNode(id, NodeTask, id, "/"+id))
		g.AddEdge(NewEdge("center", id, EdgeContains, 1.0, SourceStructural))
	}
	// a -> e (2 hops from center)
	g.AddNode(NewNode("e", NodeFile, "e", "/e"))
	g.AddEdge(NewEdge("a", "e", EdgeContains, 1.0, SourceStructural))

	sub := g.Subgraph("center", 2)
	if sub.NodeCount() != 6 {
		ids := make([]string, 0, sub.NodeCount())
		for _, n := range sub.Nodes() {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		t.Errorf("Subgraph(center,2) nodes: got %d %v, want 6", sub.NodeCount(), ids)
	}
}

func TestEmptyGraphOperations(t *testing.T) {
	g := New()

	if g.NodeCount() != 0 {
		t.Errorf("empty graph NodeCount: got %d, want 0", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("empty graph EdgeCount: got %d, want 0", g.EdgeCount())
	}
	if n := g.Node("missing"); n != nil {
		t.Errorf("empty graph Node(missing): got %v, want nil", n)
	}
	if nodes := g.NodesByType(NodeProject); len(nodes) != 0 {
		t.Errorf("empty graph NodesByType: got %d, want 0", len(nodes))
	}
	if edges := g.EdgesFrom("missing"); len(edges) != 0 {
		t.Errorf("empty graph EdgesFrom: got %d, want 0", len(edges))
	}
	if edges := g.EdgesTo("missing"); len(edges) != 0 {
		t.Errorf("empty graph EdgesTo: got %d, want 0", len(edges))
	}
	if neighbors := g.Neighbors("missing"); len(neighbors) != 0 {
		t.Errorf("empty graph Neighbors: got %d, want 0", len(neighbors))
	}
	sub := g.Subgraph("missing", 3)
	if sub.NodeCount() != 0 {
		t.Errorf("empty graph Subgraph: got %d nodes, want 0", sub.NodeCount())
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
