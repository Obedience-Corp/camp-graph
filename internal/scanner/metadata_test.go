package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func TestMetadataExtraction(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "projects/my-app"), 0o755); err != nil {
		t.Fatal(err)
	}

	festDir := filepath.Join(root, "festivals/active/app-fest-AF0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}

	festYAML := []byte("project_path: projects/my-app\nmetadata:\n  chain: my-chain\n")
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), festYAML, 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	festNode := g.Node("festival:app-fest-AF0001")
	if festNode == nil {
		t.Fatal("festival node not found")
	}

	// Verify links_to edge to the project
	found := false
	for _, e := range g.Edges() {
		if e.FromID == "festival:app-fest-AF0001" && e.ToID == "project:my-app" && e.Type == graph.EdgeLinksTo {
			found = true
			if e.Confidence != 1.0 {
				t.Errorf("links_to confidence: got %f, want 1.0", e.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected links_to edge from festival to project, not found")
	}
}

func TestMetadataMissingFestYAML(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	festDir := filepath.Join(root, "festivals/active/no-yaml-NF0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if g.Node("festival:no-yaml-NF0001") == nil {
		t.Error("festival node should exist even without fest.yaml")
	}

	for _, e := range g.Edges() {
		if e.FromID == "festival:no-yaml-NF0001" {
			t.Errorf("unexpected edge from festival: %+v", e)
		}
	}
}

func TestMetadataMalformedFestYAML(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	festDir := filepath.Join(root, "festivals/active/bad-yaml-BY0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() should not fail on malformed YAML: %v", err)
	}
	// Festival node still created, just no metadata edges
	if g.Node("festival:bad-yaml-BY0001") == nil {
		t.Error("festival node should exist despite malformed fest.yaml")
	}
}

func TestMetadataChainReference(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	// Create festival with chain reference, but no chain node — edge should not be created
	festDir := filepath.Join(root, "festivals/active/chain-ref-CR0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	festYAML := []byte("metadata:\n  chain: nonexistent-chain\n")
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), festYAML, 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	// No chain_member edge since chain node doesn't exist
	for _, e := range g.Edges() {
		if e.Type == graph.EdgeChainMember {
			t.Errorf("unexpected chain_member edge when chain node doesn't exist: %+v", e)
		}
	}
}

func TestIntentMetadataExtraction(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	// Create project that intent references
	if err := os.MkdirAll(filepath.Join(root, "projects/source-proj"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create intent file in .campaign/intents/inbox/
	intentDir := filepath.Join(root, ".campaign/intents/inbox")
	if err := os.MkdirAll(intentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	intentMD := []byte("---\ngathered_from:\n  - source-proj\nrelated_projects:\n  - source-proj\n---\n# Test Intent\n")
	if err := os.WriteFile(filepath.Join(intentDir, "test-intent.md"), intentMD, 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	// Verify gathered_from edge
	foundGathered := false
	foundRelates := false
	for _, e := range g.Edges() {
		if e.FromID == "intent:test-intent" && e.ToID == "project:source-proj" {
			if e.Type == graph.EdgeGatheredFrom {
				foundGathered = true
			}
			if e.Type == graph.EdgeRelatesTo {
				foundRelates = true
				if e.Confidence != 0.8 {
					t.Errorf("relates_to confidence: got %f, want 0.8", e.Confidence)
				}
			}
		}
	}
	if !foundGathered {
		t.Error("expected gathered_from edge from intent to project")
	}
	if !foundRelates {
		t.Error("expected relates_to edge from intent to project")
	}
}
