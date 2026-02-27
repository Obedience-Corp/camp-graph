package graph

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	store, err := OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	original := New()
	projNode := NewNode("project:camp", NodeProject, "camp", "projects/camp")
	projNode.Metadata["lang"] = "go"
	festNode := NewNode("festival:test-fest", NodeFestival, "test-fest", "festivals/active/test-fest")
	festNode.Status = "active"
	original.AddNode(projNode)
	original.AddNode(festNode)
	original.AddEdge(NewEdge("festival:test-fest", "project:camp", EdgeLinksTo, 1.0, SourceExplicit))

	if err := SaveGraph(ctx, store, original); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	loaded, err := LoadGraph(ctx, store)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	if loaded.NodeCount() != original.NodeCount() {
		t.Errorf("node count: got %d, want %d", loaded.NodeCount(), original.NodeCount())
	}
	if loaded.EdgeCount() != original.EdgeCount() {
		t.Errorf("edge count: got %d, want %d", loaded.EdgeCount(), original.EdgeCount())
	}

	loadedProj := loaded.Node("project:camp")
	if loadedProj == nil {
		t.Fatal("project:camp not found after load")
	}
	if loadedProj.Name != "camp" {
		t.Errorf("project name: got %q, want %q", loadedProj.Name, "camp")
	}
	if loadedProj.Metadata["lang"] != "go" {
		t.Errorf("project metadata[lang]: got %q, want %q", loadedProj.Metadata["lang"], "go")
	}

	loadedFest := loaded.Node("festival:test-fest")
	if loadedFest == nil {
		t.Fatal("festival:test-fest not found after load")
	}
	if loadedFest.Status != "active" {
		t.Errorf("festival status: got %q, want %q", loadedFest.Status, "active")
	}

	edges := loaded.EdgesFrom("festival:test-fest")
	if len(edges) != 1 {
		t.Fatalf("edges from festival: got %d, want 1", len(edges))
	}
	if edges[0].Type != EdgeLinksTo {
		t.Errorf("edge type: got %s, want %s", edges[0].Type, EdgeLinksTo)
	}
}

func TestOpenStoreCreatesFile(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "new.db")
	store, err := OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	n := NewNode("test:1", NodeProject, "test", "/test")
	if err := store.InsertNode(ctx, n); err != nil {
		t.Fatalf("insert into new store: %v", err)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	n, err := store.GetNode(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != nil {
		t.Errorf("expected nil for missing node, got %+v", n)
	}
}

func TestGetNodesByType(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	store.InsertNode(ctx, NewNode("p1", NodeProject, "p1", "/p1"))
	store.InsertNode(ctx, NewNode("p2", NodeProject, "p2", "/p2"))
	store.InsertNode(ctx, NewNode("f1", NodeFestival, "f1", "/f1"))

	projects, err := store.GetNodesByType(ctx, NodeProject)
	if err != nil {
		t.Fatalf("get nodes by type: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("projects: got %d, want 2", len(projects))
	}
}

func TestInsertEdgeUniqueConstraint(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	store.InsertNode(ctx, NewNode("a", NodeProject, "a", "/a"))
	store.InsertNode(ctx, NewNode("b", NodeFestival, "b", "/b"))

	e := NewEdge("a", "b", EdgeLinksTo, 1.0, SourceExplicit)
	if err := store.InsertEdge(ctx, e); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Second insert with same from/to/type should be silently ignored
	if err := store.InsertEdge(ctx, e); err != nil {
		t.Fatalf("duplicate insert should not error: %v", err)
	}

	edges, _ := store.GetAllEdges(ctx)
	if len(edges) != 1 {
		t.Errorf("edges after duplicate insert: got %d, want 1", len(edges))
	}
}

func TestDeleteAll(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	store.InsertNode(ctx, NewNode("a", NodeProject, "a", "/a"))
	store.InsertNode(ctx, NewNode("b", NodeFestival, "b", "/b"))
	store.InsertEdge(ctx, NewEdge("a", "b", EdgeLinksTo, 1.0, SourceExplicit))

	if err := store.DeleteAll(ctx); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	nodes, _ := store.GetAllNodes(ctx)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after delete, got %d", len(nodes))
	}
	edges, _ := store.GetAllEdges(ctx)
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after delete, got %d", len(edges))
	}
}

func TestStoreContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestSaveGraphReplacesExisting(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Save first graph
	g1 := New()
	g1.AddNode(NewNode("a", NodeProject, "a", "/a"))
	g1.AddNode(NewNode("b", NodeFestival, "b", "/b"))
	SaveGraph(ctx, store, g1)

	// Save second graph (should replace)
	g2 := New()
	g2.AddNode(NewNode("c", NodeTask, "c", "/c"))
	SaveGraph(ctx, store, g2)

	loaded, _ := LoadGraph(ctx, store)
	if loaded.NodeCount() != 1 {
		t.Errorf("after replace: got %d nodes, want 1", loaded.NodeCount())
	}
	if loaded.Node("c") == nil {
		t.Error("expected node c from second save")
	}
	if loaded.Node("a") != nil {
		t.Error("node a from first save should be gone")
	}
}
