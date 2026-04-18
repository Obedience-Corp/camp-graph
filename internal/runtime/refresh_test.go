package runtime_test

import (
	"time"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
)

// diffInventory is unexported, so the add/change/delete classification
// is exercised end-to-end through the Refresh tests below rather than
// directly.

func TestRefresh_FreshDBForcesRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects", "seed"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan\n"), 0o644); err != nil {
		_ = os.MkdirAll(filepath.Join(root, "Work"), 0o755)
		if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	dbPath := filepath.Join(t.TempDir(), "graph.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	report, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root,
		DBPath:       dbPath,
		Store:        store,
		BuildDocs:    func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn: func(mode runtime.RefreshMode, now time.Time, searchAvailable bool) graph.BuildMeta {
			return graph.BuildMeta{
				GraphSchemaVersion: "graphdb/v2alpha1",
				CampaignRoot:       root,
				SearchAvailable:    searchAvailable,
			}
		},
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if report.Mode != runtime.ModeRebuild {
		t.Errorf("fresh DB mode: got %q, want rebuild", report.Mode)
	}
	if !report.StaleBefore {
		t.Error("expected StaleBefore=true on a fresh DB")
	}
}

func TestRefresh_SecondRunReportsRefreshMode(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Work"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan v1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "graph.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	buildMeta := func(mode runtime.RefreshMode, _ time.Time, searchAvailable bool) graph.BuildMeta {
		return graph.BuildMeta{
			GraphSchemaVersion: "graphdb/v2alpha1",
			CampaignRoot:       root,
			SearchAvailable:    searchAvailable,
		}
	}

	first, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root,
		DBPath:       dbPath,
		Store:        store,
		BuildDocs:    func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn:  buildMeta,
	})
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if first.Mode != runtime.ModeRebuild {
		t.Errorf("first mode: got %q, want rebuild", first.Mode)
	}

	// Mutate content so the second run sees a changed file.
	if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan v2\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	second, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root,
		DBPath:       dbPath,
		Store:        store,
		BuildDocs:    func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn:  buildMeta,
	})
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if second.Mode != runtime.ModeRefresh {
		t.Errorf("second mode: got %q, want refresh", second.Mode)
	}
	if second.ReindexedFiles == 0 {
		t.Error("expected some reindexed files when content changes")
	}
}

func TestRefresh_DeletedFileCounted(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Work"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	plan := filepath.Join(root, "Work", "plan.md")
	if err := os.WriteFile(plan, []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	other := filepath.Join(root, "Work", "other.md")
	if err := os.WriteFile(other, []byte("# other\n"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "graph.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	buildMeta := func(mode runtime.RefreshMode, _ time.Time, searchAvailable bool) graph.BuildMeta {
		return graph.BuildMeta{
			GraphSchemaVersion: "graphdb/v2alpha1",
			CampaignRoot:       root,
			SearchAvailable:    searchAvailable,
		}
	}
	req := runtime.RefreshRequest{
		CampaignRoot: root,
		DBPath:       dbPath,
		Store:        store,
		BuildDocs:    func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn:  buildMeta,
	}
	if _, err := runtime.Refresh(ctx, req); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Delete one file.
	if err := os.Remove(other); err != nil {
		t.Fatalf("remove: %v", err)
	}
	report, err := runtime.Refresh(ctx, req)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if report.DeletedFiles != 1 {
		t.Errorf("deleted count: got %d, want 1", report.DeletedFiles)
	}
}

func TestRefresh_ParityWithClearRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Work"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Work", "kickoff.md"), []byte("# kickoff\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	buildMeta := func(mode runtime.RefreshMode, _ time.Time, searchAvailable bool) graph.BuildMeta {
		return graph.BuildMeta{
			GraphSchemaVersion: "graphdb/v2alpha1",
			CampaignRoot:       root,
			SearchAvailable:    searchAvailable,
		}
	}

	// DB #1: rebuild from scratch.
	dbA := filepath.Join(t.TempDir(), "A.db")
	storeA, err := graph.OpenStore(ctx, dbA)
	if err != nil {
		t.Fatalf("openA: %v", err)
	}
	defer storeA.Close()
	reportA, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root, DBPath: dbA, Store: storeA,
		BuildDocs:   func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn: buildMeta,
	})
	if err != nil {
		t.Fatalf("rebuildA: %v", err)
	}

	// DB #2: rebuild then refresh again without changes.
	dbB := filepath.Join(t.TempDir(), "B.db")
	storeB, err := graph.OpenStore(ctx, dbB)
	if err != nil {
		t.Fatalf("openB: %v", err)
	}
	defer storeB.Close()
	if _, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root, DBPath: dbB, Store: storeB,
		BuildDocs:   func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn: buildMeta,
	}); err != nil {
		t.Fatalf("rebuildB: %v", err)
	}
	reportB, err := runtime.Refresh(ctx, runtime.RefreshRequest{
		CampaignRoot: root, DBPath: dbB, Store: storeB,
		BuildDocs:   func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn: buildMeta,
	})
	if err != nil {
		t.Fatalf("refreshB: %v", err)
	}

	if reportA.NodesWritten != reportB.NodesWritten {
		t.Errorf("nodes parity: rebuild=%d refresh=%d", reportA.NodesWritten, reportB.NodesWritten)
	}
	if reportA.EdgesWritten != reportB.EdgesWritten {
		t.Errorf("edges parity: rebuild=%d refresh=%d", reportA.EdgesWritten, reportB.EdgesWritten)
	}
}

// TestRefresh_NoChangesSkipsFullRebuild proves that a second refresh
// with no filesystem mutations takes the no-op fast path: reindexed
// and deleted stay at zero, the last_refresh_mode is "refresh", and
// the call completes without rewriting node/edge rows.
func TestRefresh_NoChangesSkipsFullRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Work"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Work", "plan.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "graph.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	buildMeta := func(mode runtime.RefreshMode, _ time.Time, searchAvailable bool) graph.BuildMeta {
		return graph.BuildMeta{
			GraphSchemaVersion: "graphdb/v2alpha1",
			CampaignRoot:       root,
			SearchAvailable:    searchAvailable,
		}
	}
	req := runtime.RefreshRequest{
		CampaignRoot: root, DBPath: dbPath, Store: store,
		BuildDocs:   func(g *graph.Graph) ([]graph.DocumentRecord, error) { return nil, nil },
		BuildMetaFn: buildMeta,
	}

	// First run: rebuild.
	if _, err := runtime.Refresh(ctx, req); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	// Second run without mutation: fast path.
	report, err := runtime.Refresh(ctx, req)
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if report.Mode != runtime.ModeRefresh {
		t.Errorf("mode: got %q, want refresh", report.Mode)
	}
	if report.ReindexedFiles != 0 || report.DeletedFiles != 0 {
		t.Errorf("no-op path: reindexed=%d deleted=%d; want 0/0",
			report.ReindexedFiles, report.DeletedFiles)
	}
	mode, err := store.GetMeta(ctx, "last_refresh_mode")
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if mode != "refresh" {
		t.Errorf("last_refresh_mode: got %q, want refresh", mode)
	}
}
