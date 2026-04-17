package search_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func seedTwoNotes(t *testing.T) (*graph.Store, func()) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "query.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	notes := []*graph.Node{
		graph.NewNode("note:Work/JobSearch/plan.md", graph.NodeNote, "Work/JobSearch/plan.md", "/tmp/plan.md"),
		graph.NewNode("note:Work/JobSearch/kickoff.md", graph.NodeNote, "Work/JobSearch/kickoff.md", "/tmp/kickoff.md"),
		graph.NewNode("note:Business/ShinySwap/notes.md", graph.NodeNote, "Business/ShinySwap/notes.md", "/tmp/shiny.md"),
	}
	for _, n := range notes {
		if err := store.InsertNode(ctx, n); err != nil {
			t.Fatalf("insert node %s: %v", n.ID, err)
		}
	}
	idx := search.NewIndexer(store.DB())
	docs := []search.Document{
		{
			NodeID:       "note:Work/JobSearch/plan.md",
			Title:        "Job Search Plan",
			RelPath:      "Work/JobSearch/plan.md",
			Scope:        "Work/JobSearch",
			Body:         "Roadmap for my job search and planning.",
			TrackedState: "tracked",
			Aliases:      []string{"plan"},
			Tags:         []string{"planning"},
			UpdatedAt:    time.Now(),
		},
		{
			NodeID:       "note:Work/JobSearch/kickoff.md",
			Title:        "Kickoff",
			RelPath:      "Work/JobSearch/kickoff.md",
			Scope:        "Work/JobSearch",
			Body:         "First-week job search checklist.",
			TrackedState: "tracked",
			UpdatedAt:    time.Now(),
		},
		{
			NodeID:       "note:Business/ShinySwap/notes.md",
			Title:        "Shiny Notes",
			RelPath:      "Business/ShinySwap/notes.md",
			Scope:        "Business/ShinySwap",
			Body:         "Nothing to do with job search.",
			TrackedState: "untracked",
			UpdatedAt:    time.Now(),
		},
	}
	for _, d := range docs {
		if err := idx.UpsertDocument(ctx, d); err != nil {
			t.Fatalf("upsert %s: %v", d.NodeID, err)
		}
	}
	return store, func() { store.Close() }
}

func TestQuerier_Search_ReturnsLexicalMatches(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()
	q := search.NewQuerier(store.DB())

	results, err := q.Search(context.Background(), search.QueryOptions{
		Term:  "job search",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results; got %d", len(results))
	}
	// All results should match either Work/JobSearch or the Shiny note
	// (body mentions "job search"). Ranking details are SQLite's job.
	for _, r := range results {
		if r.Snippet == "" {
			t.Errorf("result missing snippet: %+v", r)
		}
		if len(r.Reasons) == 0 {
			t.Errorf("result missing reasons: %+v", r)
		}
	}
}

func TestQuerier_Search_ScopeFilter(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()
	q := search.NewQuerier(store.DB())

	results, err := q.Search(context.Background(), search.QueryOptions{
		Term:  "search",
		Scope: "Work/JobSearch",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, r := range results {
		if r.Scope != "Work/JobSearch" {
			t.Errorf("scope filter leaked; got result with scope=%q", r.Scope)
		}
	}
}

func TestQuerier_Search_PathPrefix(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()
	q := search.NewQuerier(store.DB())

	results, err := q.Search(context.Background(), search.QueryOptions{
		Term:       "search",
		PathPrefix: "Business",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, r := range results {
		if len(r.RelativePath) < len("Business") ||
			r.RelativePath[:len("Business")] != "Business" {
			t.Errorf("path-prefix filter leaked; got %q", r.RelativePath)
		}
	}
}

func TestQuerier_Search_TrackedFilter(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()
	q := search.NewQuerier(store.DB())

	results, err := q.Search(context.Background(), search.QueryOptions{
		Term:    "search",
		Tracked: true,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, r := range results {
		if r.TrackedState != "tracked" {
			t.Errorf("tracked filter leaked; got %q", r.TrackedState)
		}
	}
}

func TestQuerier_Search_TypeFilter(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()
	q := search.NewQuerier(store.DB())

	results, err := q.Search(context.Background(), search.QueryOptions{
		Term:  "search",
		Type:  "note",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, r := range results {
		if r.NodeType != "note" {
			t.Errorf("type filter leaked; got %q", r.NodeType)
		}
	}
}

func TestParseMode_DefaultsToHybrid(t *testing.T) {
	if m := search.ParseMode(""); m != search.QueryModeHybrid {
		t.Errorf("empty default: got %v, want hybrid", m)
	}
	if m := search.ParseMode("invalid"); m != search.QueryModeHybrid {
		t.Errorf("invalid default: got %v, want hybrid", m)
	}
	if m := search.ParseMode("structural"); m != search.QueryModeStructural {
		t.Errorf("structural: got %v", m)
	}
}
