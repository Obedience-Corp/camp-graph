package scanner_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

type evidencePayload struct {
	Reasons []struct {
		Kind   string  `json:"kind"`
		Value  string  `json:"value,omitempty"`
		Weight float64 `json:"weight"`
	} `json:"reasons"`
	Score float64 `json:"score"`
}

func findInferredEdge(g *graph.Graph, from, to string) *graph.Edge {
	for _, e := range g.Edges() {
		if e.Source != graph.SourceInferred {
			continue
		}
		if (e.FromID == from && e.ToID == to) || (e.FromID == to && e.ToID == from) {
			return e
		}
	}
	return nil
}

func TestInferredEdges_SingleEdgePerPair(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Two notes in same folder with shared frontmatter type produce
	// multiple signals but must collapse to one inferred edge.
	writeFile(t, filepath.Join(root, "Work/plan-alpha.md"),
		"---\ntype: daily\ntags: [planning]\n---\n\n# plan alpha\n")
	writeFile(t, filepath.Join(root, "Work/plan-beta.md"),
		"---\ntype: daily\ntags: [planning]\n---\n\n# plan beta\n")

	g := scanNotesFixture(t, root)

	count := 0
	for _, e := range g.Edges() {
		if e.Source != graph.SourceInferred {
			continue
		}
		if (e.FromID == "note:Work/plan-alpha.md" && e.ToID == "note:Work/plan-beta.md") ||
			(e.FromID == "note:Work/plan-beta.md" && e.ToID == "note:Work/plan-alpha.md") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one inferred edge for pair; got %d", count)
	}
}

func TestInferredEdges_ReasonsAndSubtype(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/plan-alpha.md"),
		"---\ntype: daily\ntags: [planning]\n---\n\n# plan alpha\n")
	writeFile(t, filepath.Join(root, "Work/plan-beta.md"),
		"---\ntype: daily\ntags: [planning]\n---\n\n# plan beta\n")

	g := scanNotesFixture(t, root)

	e := findInferredEdge(g, "note:Work/plan-alpha.md", "note:Work/plan-beta.md")
	if e == nil {
		t.Fatal("expected inferred edge between alpha and beta")
	}
	if e.Subtype == "" {
		t.Errorf("inferred edge missing Subtype")
	}
	if e.Note == "" {
		t.Fatalf("inferred edge missing Note payload")
	}
	var payload evidencePayload
	if err := json.Unmarshal([]byte(e.Note), &payload); err != nil {
		t.Fatalf("Note is not valid JSON: %v; note=%q", err, e.Note)
	}
	if len(payload.Reasons) < 2 {
		t.Errorf("expected >=2 reasons (same_folder + shared signal); got %d", len(payload.Reasons))
	}
	// Reasons must be sorted by weight desc.
	for i := 1; i < len(payload.Reasons); i++ {
		if payload.Reasons[i-1].Weight < payload.Reasons[i].Weight {
			t.Errorf("reasons not weight-sorted: %v", payload.Reasons)
			break
		}
	}
	if payload.Score <= 0 || payload.Score > 1 {
		t.Errorf("payload score out of range: %v", payload.Score)
	}
}

func TestInferredEdges_ArtifactOwnedSignal(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Both notes live under projects/alpha subtree.
	writeFile(t, filepath.Join(root, "projects/alpha/notes/plan.md"), "# plan\n")
	writeFile(t, filepath.Join(root, "projects/alpha/notes/recap.md"), "# recap\n")

	g := scanNotesFixture(t, root)

	e := findInferredEdge(g, "note:projects/alpha/notes/plan.md", "note:projects/alpha/notes/recap.md")
	if e == nil {
		t.Fatal("expected inferred edge under projects/alpha")
	}
	var payload evidencePayload
	if err := json.Unmarshal([]byte(e.Note), &payload); err != nil {
		t.Fatalf("Note is not valid JSON: %v", err)
	}
	found := false
	for _, r := range payload.Reasons {
		if r.Kind == "artifact_owned" && strings.HasPrefix(r.Value, "projects/alpha") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected artifact_owned reason; reasons=%v", payload.Reasons)
	}
}

func TestInferredEdges_WeakSignalsDroppedBelowThreshold(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Two unrelated notes under different folders with no frontmatter,
	// no tags. They share only CampaignRepoRoot (weight 0.10) which is
	// below minInferredConfidence.
	writeFile(t, filepath.Join(root, "A/one.md"), "# one\n")
	writeFile(t, filepath.Join(root, "B/two.md"), "# two\n")

	g := scanNotesFixture(t, root)

	if findInferredEdge(g, "note:A/one.md", "note:B/two.md") != nil {
		t.Error("did not expect inferred edge for weak-signal pair")
	}
}

func TestInferredEdges_SimilarToKindForTokenOverlap(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Shared significant tokens in filenames trigger shared_tokens;
	// inferenceEdgeType should pick EdgeSimilarTo.
	writeFile(t, filepath.Join(root, "Work/job-search-status-alpha.md"), "# update\n")
	writeFile(t, filepath.Join(root, "Work/job-search-status-beta.md"), "# kickoff\n")

	g := scanNotesFixture(t, root)

	e := findInferredEdge(g,
		"note:Work/job-search-status-alpha.md",
		"note:Work/job-search-status-beta.md")
	if e == nil {
		t.Fatal("expected inferred edge for shared-token pair")
	}
	if e.Type != graph.EdgeSimilarTo {
		t.Errorf("edge type: got %v, want similar_to for shared_tokens pair", e.Type)
	}
}
