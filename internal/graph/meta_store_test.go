package graph

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_MetaSetGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "meta.db")
	store, err := OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.SetMeta(ctx, "graph_schema_version", "graphdb/v2alpha1"); err != nil {
		t.Fatalf("set meta: %v", err)
	}
	if err := store.SetMeta(ctx, "search_available", "true"); err != nil {
		t.Fatalf("set search_available: %v", err)
	}
	// Overwrite to exercise upsert path.
	if err := store.SetMeta(ctx, "graph_schema_version", "graphdb/v2alpha1"); err != nil {
		t.Fatalf("re-set meta: %v", err)
	}

	val, err := store.GetMeta(ctx, "graph_schema_version")
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if val != "graphdb/v2alpha1" {
		t.Errorf("graph_schema_version: got %q, want %q", val, "graphdb/v2alpha1")
	}

	missing, err := store.GetMeta(ctx, "does_not_exist")
	if err != nil {
		t.Fatalf("get missing meta: %v", err)
	}
	if missing != "" {
		t.Errorf("expected empty string for missing key; got %q", missing)
	}

	all, err := store.AllMeta(ctx)
	if err != nil {
		t.Fatalf("all meta: %v", err)
	}
	if all["search_available"] != "true" {
		t.Errorf("AllMeta search_available: got %q, want %q", all["search_available"], "true")
	}
	if all["graph_schema_version"] != "graphdb/v2alpha1" {
		t.Errorf("AllMeta graph_schema_version: got %q, want %q",
			all["graph_schema_version"], "graphdb/v2alpha1")
	}
}

func TestStore_SearchDocsTableReady(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "search.db")
	store, err := OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Sanity-check that the schema created the table. A simple count
	// query is enough to validate the column list.
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM search_docs`).Scan(&count); err != nil {
		t.Fatalf("count search_docs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected empty search_docs table; got %d rows", count)
	}

	// Same for search_docs_fts and indexed_files.
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM indexed_files`).Scan(&count); err != nil {
		t.Fatalf("count indexed_files: %v", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM search_docs_fts`).Scan(&count); err != nil {
		t.Fatalf("count search_docs_fts: %v", err)
	}
}
