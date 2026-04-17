package runtime_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func TestCheckCompatibility_FreshDB(t *testing.T) {
	ctx := context.Background()
	store, err := graph.OpenStore(ctx, filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	v, schema, err := runtime.CheckCompatibility(ctx, store.DB())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if v != runtime.CompatFresh {
		t.Errorf("verdict: got %v, want fresh", v)
	}
	if schema != "" {
		t.Errorf("schema should be empty on fresh DB; got %q", schema)
	}
}

func TestCheckCompatibility_MatchingSchema(t *testing.T) {
	ctx := context.Background()
	store, err := graph.OpenStore(ctx, filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	_ = store.SetMeta(ctx, "graph_schema_version", search.GraphSchemaVersion)
	v, schema, err := runtime.CheckCompatibility(ctx, store.DB())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if v != runtime.CompatMatching {
		t.Errorf("verdict: got %v, want matching", v)
	}
	if schema != search.GraphSchemaVersion {
		t.Errorf("schema: got %q, want %q", schema, search.GraphSchemaVersion)
	}
}

func TestCheckCompatibility_IncompatibleSchema(t *testing.T) {
	ctx := context.Background()
	store, err := graph.OpenStore(ctx, filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	_ = store.SetMeta(ctx, "graph_schema_version", "graphdb/v1alpha1")
	v, schema, err := runtime.CheckCompatibility(ctx, store.DB())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if v != runtime.CompatIncompatible {
		t.Errorf("verdict: got %v, want incompatible", v)
	}
	if schema != "graphdb/v1alpha1" {
		t.Errorf("schema: got %q, want graphdb/v1alpha1", schema)
	}
}

func TestCheckCompatibility_KnownList(t *testing.T) {
	if !runtime.KnownCompatibleSchemas[search.GraphSchemaVersion] {
		t.Errorf("current GraphSchemaVersion %q should be in KnownCompatibleSchemas",
			search.GraphSchemaVersion)
	}
}
