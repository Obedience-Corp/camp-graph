package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func TestExtractChainMetadata(t *testing.T) {
	root := t.TempDir()

	// Create a chain YAML file
	chainFile := filepath.Join(root, "my-chain.yaml")
	chainYAMLContent := []byte(`festivals:
  - fest-a
  - fest-b
edges:
  - from: fest-a
    to: fest-b
    type: hard
  - from: fest-b
    to: fest-a
    type: soft
`)
	if err := os.WriteFile(chainFile, chainYAMLContent, 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.New()
	// Add the chain and festival nodes so edges can be created
	g.AddNode(graph.NewNode("chain:my-chain", graph.NodeChain, "my-chain", root))
	g.AddNode(graph.NewNode("festival:fest-a", graph.NodeFestival, "fest-a", "/a"))
	g.AddNode(graph.NewNode("festival:fest-b", graph.NodeFestival, "fest-b", "/b"))

	extractChainMetadata(context.Background(), g, "chain:my-chain", chainFile)

	// Verify chain_member edges
	memberCount := 0
	for _, e := range g.Edges() {
		if e.Type == graph.EdgeChainMember {
			memberCount++
		}
	}
	if memberCount != 2 {
		t.Errorf("chain_member edges: got %d, want 2", memberCount)
	}

	// Verify depends_on edges with correct confidence and subtype
	depCount := 0
	for _, e := range g.Edges() {
		if e.Type == graph.EdgeDependsOn {
			depCount++
			if e.FromID == "festival:fest-a" && e.ToID == "festival:fest-b" {
				if e.Confidence != 1.0 {
					t.Errorf("hard dep confidence: got %f, want 1.0", e.Confidence)
				}
				if e.Subtype != "hard" {
					t.Errorf("hard dep subtype: got %q, want %q", e.Subtype, "hard")
				}
			}
			if e.FromID == "festival:fest-b" && e.ToID == "festival:fest-a" {
				if e.Confidence != 0.8 {
					t.Errorf("soft dep confidence: got %f, want 0.8", e.Confidence)
				}
				if e.Subtype != "soft" {
					t.Errorf("soft dep subtype: got %q, want %q", e.Subtype, "soft")
				}
			}
		}
	}
	if depCount != 2 {
		t.Errorf("depends_on edges: got %d, want 2", depCount)
	}
}

func TestExtractChainMetadataMissingFile(t *testing.T) {
	g := graph.New()
	// Should not panic or error — just logs a warning
	extractChainMetadata(context.Background(), g, "chain:x", "/nonexistent/file.yaml")
	if g.EdgeCount() != 0 {
		t.Errorf("edges: got %d, want 0", g.EdgeCount())
	}
}

func TestExtractChainMetadataMalformed(t *testing.T) {
	root := t.TempDir()
	chainFile := filepath.Join(root, "bad-chain.yaml")
	if err := os.WriteFile(chainFile, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.New()
	extractChainMetadata(context.Background(), g, "chain:bad", chainFile)
	if g.EdgeCount() != 0 {
		t.Errorf("edges: got %d, want 0", g.EdgeCount())
	}
}

func TestParseYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid frontmatter",
			input:   "---\nconcept: test\n---\n# Body",
			wantErr: false,
		},
		{
			name:    "no opening delimiter",
			input:   "no frontmatter here",
			wantErr: true,
		},
		{
			name:    "no closing delimiter",
			input:   "---\nconcept: test\nno closing",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseYAMLFrontmatter([]byte(tc.input))
			if (err != nil) != tc.wantErr {
				t.Errorf("parseYAMLFrontmatter() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestIsPhaseDirEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"001_BUILD", true},
		{"999_X", true},
		{"00_short", false}, // only 2 digits
		{"abc_invalid", false},
		{"12", false},   // too short
		{"123X", false}, // 4th char not underscore
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPhaseDir(tc.name); got != tc.want {
				t.Errorf("isPhaseDir(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
