package search_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func seedRelatedGraph(t *testing.T) *graph.Store {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "related.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	notes := []*graph.Node{
		graph.NewNode("note:Work/JobSearch/plan.md", graph.NodeNote, "Work/JobSearch/plan.md", "/tmp/plan.md"),
		graph.NewNode("note:Work/JobSearch/kickoff.md", graph.NodeNote, "Work/JobSearch/kickoff.md", "/tmp/kickoff.md"),
		graph.NewNode("note:Work/JobSearch/summary.md", graph.NodeNote, "Work/JobSearch/summary.md", "/tmp/summary.md"),
		graph.NewNode("note:Business/ShinySwap/readme.md", graph.NodeNote, "Business/ShinySwap/readme.md", "/tmp/readme.md"),
	}
	for _, n := range notes {
		if err := store.InsertNode(ctx, n); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	// Explicit edge from plan -> kickoff.
	store.InsertEdge(ctx, graph.NewEdge("note:Work/JobSearch/plan.md",
		"note:Work/JobSearch/kickoff.md", graph.EdgeLinksTo, 1.0, graph.SourceExplicit))

	idx := search.NewIndexer(store.DB())
	docs := []search.Document{
		{
			NodeID: "note:Work/JobSearch/plan.md", Title: "Plan",
			RelPath: "Work/JobSearch/plan.md", Scope: "Work/JobSearch",
			Body: "Plan body.", TrackedState: "tracked", UpdatedAt: time.Now(),
		},
		{
			NodeID: "note:Work/JobSearch/kickoff.md", Title: "Kickoff",
			RelPath: "Work/JobSearch/kickoff.md", Scope: "Work/JobSearch",
			Body: "Kickoff body.", TrackedState: "tracked", UpdatedAt: time.Now(),
		},
		{
			NodeID: "note:Work/JobSearch/summary.md", Title: "Summary",
			RelPath: "Work/JobSearch/summary.md", Scope: "Work/JobSearch",
			Body: "Summary body.", TrackedState: "tracked", UpdatedAt: time.Now(),
		},
		{
			NodeID: "note:Business/ShinySwap/readme.md", Title: "Shiny",
			RelPath: "Business/ShinySwap/readme.md", Scope: "Business/ShinySwap",
			Body: "Shiny body.", TrackedState: "tracked", UpdatedAt: time.Now(),
		},
	}
	for _, d := range docs {
		if err := idx.UpsertDocument(ctx, d); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	return store
}

func TestRelated_PrefersScopeNeighbors(t *testing.T) {
	store := seedRelatedGraph(t)
	defer store.Close()

	items, err := search.Related(context.Background(), store.DB(), search.RelatedOptions{
		Path:  "Work/JobSearch/plan.md",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("no related items returned")
	}
	// Scope neighbors should dominate the top of the list.
	scopeSeen := 0
	for i, it := range items {
		if it.Reason == "same_scope" && i < 3 {
			scopeSeen++
		}
	}
	if scopeSeen == 0 {
		t.Errorf("expected same_scope reason in top results; got %+v", items)
	}
}

func TestRelated_IncludesExplicitEdges(t *testing.T) {
	store := seedRelatedGraph(t)
	defer store.Close()

	items, err := search.Related(context.Background(), store.DB(), search.RelatedOptions{
		Path:  "Work/JobSearch/plan.md",
		Mode:  search.QueryModeExplicit,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	foundKickoff := false
	for _, it := range items {
		if it.NodeID == "note:Work/JobSearch/kickoff.md" && it.Reason == "explicit_edge" {
			foundKickoff = true
		}
	}
	if !foundKickoff {
		t.Errorf("expected explicit_edge to kickoff; got %+v", items)
	}
}

func TestRelated_RequiresPath(t *testing.T) {
	store := seedRelatedGraph(t)
	defer store.Close()
	_, err := search.Related(context.Background(), store.DB(), search.RelatedOptions{Limit: 5})
	if err == nil {
		t.Fatal("expected error when --path is empty")
	}
}

func TestRelated_UnknownPathReturnsNone(t *testing.T) {
	store := seedRelatedGraph(t)
	defer store.Close()
	items, err := search.Related(context.Background(), store.DB(), search.RelatedOptions{
		Path:  "No/Such/path.md",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for unknown path; got %d", len(items))
	}
}
