package runtime_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
)

func openStore(t *testing.T) *graph.Store {
	t.Helper()
	ctx := context.Background()
	store, err := graph.OpenStore(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestIndexState_UpsertLoadDelete(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	s := runtime.NewIndexState(store.DB())
	ctx := context.Background()

	f := runtime.IndexedFile{
		RelPath:      "Work/plan.md",
		RepoRoot:     "/campaign",
		NodeID:       "note:Work/plan.md",
		TrackedState: "tracked",
		ContentHash:  "abc123",
		MtimeNs:      time.Now().UnixNano(),
		ParserKind:   "markdown",
		ScopeID:      "folder:Work",
	}
	if err := s.Upsert(ctx, f); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	files, err := s.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := files["Work/plan.md"]
	if !ok {
		t.Fatal("file not found after upsert")
	}
	if got.NodeID != "note:Work/plan.md" || got.TrackedState != "tracked" {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// Upsert again to exercise conflict path.
	f.ContentHash = "def456"
	if err := s.Upsert(ctx, f); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	files, _ = s.Load(ctx)
	if files["Work/plan.md"].ContentHash != "def456" {
		t.Errorf("upsert did not overwrite content_hash")
	}

	if err := s.Delete(ctx, "Work/plan.md"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	files, _ = s.Load(ctx)
	if _, ok := files["Work/plan.md"]; ok {
		t.Error("row still present after delete")
	}
}

func TestStatus_RoundTripFromMeta(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	ctx := context.Background()
	_ = store.SetMeta(ctx, "graph_schema_version", "graphdb/v2alpha1")
	_ = store.SetMeta(ctx, "plugin_version", "dev")
	_ = store.SetMeta(ctx, "built_at", "2026-04-17T00:00:00Z")
	_ = store.SetMeta(ctx, "last_refresh_at", "2026-04-17T00:05:00Z")
	_ = store.SetMeta(ctx, "last_refresh_mode", "refresh")
	_ = store.SetMeta(ctx, "search_available", "true")

	// Insert one node to exercise the counts path.
	n := graph.NewNode("note:test.md", graph.NodeNote, "test.md", "/tmp/test.md")
	if err := store.InsertNode(ctx, n); err != nil {
		t.Fatalf("insert node: %v", err)
	}

	s, err := runtime.LoadStatus(ctx, store.DB(), "/campaign", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if s.GraphSchemaVersion != "graphdb/v2alpha1" {
		t.Errorf("schema: got %q", s.GraphSchemaVersion)
	}
	if s.LastRefreshMode != "refresh" {
		t.Errorf("mode: got %q", s.LastRefreshMode)
	}
	if !s.SearchAvailable {
		t.Error("search_available should be true")
	}
	if s.Nodes != 1 {
		t.Errorf("nodes: got %d, want 1", s.Nodes)
	}
}

func TestUpdateRefreshMeta(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	ctx := context.Background()
	at := time.Date(2026, 4, 17, 12, 34, 0, 0, time.UTC)
	if err := runtime.UpdateRefreshMeta(ctx, store.DB(), "refresh", at); err != nil {
		t.Fatalf("update refresh meta: %v", err)
	}
	v, _ := store.GetMeta(ctx, "last_refresh_mode")
	if v != "refresh" {
		t.Errorf("mode: got %q, want refresh", v)
	}
	v, _ = store.GetMeta(ctx, "last_refresh_at")
	if v != "2026-04-17T12:34:00Z" {
		t.Errorf("at: got %q", v)
	}
}
