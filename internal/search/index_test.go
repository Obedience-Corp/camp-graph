package search_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func openTestStore(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "search.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// The search tests need a node row because search_docs.node_id
	// references nodes(id). Inserting a stub node makes the ON DELETE
	// CASCADE behavior observable in real tests too.
	n := graph.NewNode("note:Work/plan.md", graph.NodeNote, "Work/plan.md", "/tmp/plan.md")
	if err := store.InsertNode(ctx, n); err != nil {
		t.Fatalf("insert stub node: %v", err)
	}
	return store.DB(), func() { store.Close() }
}

func TestIndexer_UpsertAndLoad(t *testing.T) {
	ctx := context.Background()
	db, cleanup := openTestStore(t)
	defer cleanup()
	idx := search.NewIndexer(db)

	doc := search.Document{
		NodeID:       "note:Work/plan.md",
		Title:        "Plan",
		RelPath:      "Work/plan.md",
		Scope:        "Work",
		Body:         "Body that mentions job search and planning.",
		Summary:      "Plan summary",
		Aliases:      []string{"planning", "roadmap"},
		Tags:         []string{"strategy", "ok-2026"},
		TrackedState: "tracked",
		UpdatedAt:    time.Now().UTC(),
	}
	if err := idx.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	loaded, err := idx.LoadDocument(ctx, doc.NodeID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("document not found after upsert")
	}
	if loaded.Title != doc.Title || loaded.RelPath != doc.RelPath || loaded.Scope != doc.Scope {
		t.Errorf("scalar round-trip mismatch: got %+v, want title/rel/scope=%s/%s/%s",
			loaded, doc.Title, doc.RelPath, doc.Scope)
	}
	if len(loaded.Aliases) != 2 || loaded.Aliases[0] != "planning" || loaded.Aliases[1] != "roadmap" {
		t.Errorf("aliases round-trip: got %v, want [planning roadmap]", loaded.Aliases)
	}
	if len(loaded.Tags) != 2 || loaded.Tags[0] != "strategy" || loaded.Tags[1] != "ok-2026" {
		t.Errorf("tags round-trip: got %v, want [strategy ok-2026]", loaded.Tags)
	}
	if loaded.Body != doc.Body {
		t.Errorf("body round-trip: got %q, want %q", loaded.Body, doc.Body)
	}
}

func TestIndexer_UpsertOverwrites(t *testing.T) {
	ctx := context.Background()
	db, cleanup := openTestStore(t)
	defer cleanup()
	idx := search.NewIndexer(db)

	doc := search.Document{
		NodeID:       "note:Work/plan.md",
		Title:        "First",
		RelPath:      "Work/plan.md",
		Scope:        "Work",
		Body:         "body v1",
		TrackedState: "tracked",
	}
	if err := idx.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	doc.Title = "Second"
	doc.Body = "body v2"
	if err := idx.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	loaded, err := idx.LoadDocument(ctx, doc.NodeID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Title != "Second" || loaded.Body != "body v2" {
		t.Errorf("upsert did not overwrite; got %+v", loaded)
	}

	// FTS should also reflect the replacement.
	var ftsCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM search_docs_fts WHERE search_docs_fts MATCH 'v2'`,
	).Scan(&ftsCount); err != nil {
		t.Fatalf("fts count: %v", err)
	}
	if ftsCount != 1 {
		t.Errorf("fts v2 match count: got %d, want 1", ftsCount)
	}
}

func TestIndexer_DeleteDocument(t *testing.T) {
	ctx := context.Background()
	db, cleanup := openTestStore(t)
	defer cleanup()
	idx := search.NewIndexer(db)

	doc := search.Document{
		NodeID:       "note:Work/plan.md",
		Title:        "Plan",
		RelPath:      "Work/plan.md",
		Scope:        "Work",
		Body:         "disposable",
		TrackedState: "tracked",
	}
	if err := idx.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := idx.DeleteDocument(ctx, doc.NodeID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	loaded, err := idx.LoadDocument(ctx, doc.NodeID)
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil after delete; got %+v", loaded)
	}
	// FTS mirror must not return the deleted row.
	var ftsCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM search_docs_fts WHERE search_docs_fts MATCH 'disposable'`,
	).Scan(&ftsCount); err != nil {
		t.Fatalf("fts count after delete: %v", err)
	}
	if ftsCount != 0 {
		t.Errorf("fts had stale entries after delete; got %d", ftsCount)
	}
}

func TestIndexer_MissingNodeIDRejected(t *testing.T) {
	ctx := context.Background()
	db, cleanup := openTestStore(t)
	defer cleanup()
	idx := search.NewIndexer(db)

	err := idx.UpsertDocument(ctx, search.Document{})
	if err == nil {
		t.Fatal("expected error for empty node_id")
	}
}

func TestFTSAvailable(t *testing.T) {
	ctx := context.Background()
	db, cleanup := openTestStore(t)
	defer cleanup()
	if !search.FTSAvailable(ctx, db) {
		t.Error("expected FTS5 availability on test db")
	}
}
