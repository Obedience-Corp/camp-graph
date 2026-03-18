package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

// createMinimalCampaign builds a synthetic campaign directory tree for testing.
func createMinimalCampaign(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"projects/alpha",
		"projects/beta",
		"festivals/active/test-fest-TF0001/001_BUILD/01_core",
		"festivals/planning/plan-fest-PF0001",
		"festivals/dungeon/completed/done-fest-DF0001",
		".campaign/intents/inbox",
		"workflow/design/my-design",
		"workflow/explore/my-explore",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	seqDir := filepath.Join(root, "festivals/active/test-fest-TF0001/001_BUILD/01_core")
	for _, name := range []string{"01_task_one.md", "02_task_two.md", "SEQUENCE_GOAL.md"} {
		if err := os.WriteFile(filepath.Join(seqDir, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create an intent file
	if err := os.WriteFile(filepath.Join(root, ".campaign/intents/inbox/my-intent.md"), []byte("# My Intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestScanMinimalCampaign(t *testing.T) {
	root := createMinimalCampaign(t)
	ctx := context.Background()

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	counts := map[graph.NodeType]int{}
	for _, n := range g.Nodes() {
		counts[n.Type]++
	}

	if counts[graph.NodeProject] != 2 {
		t.Errorf("projects: got %d, want 2", counts[graph.NodeProject])
	}
	if counts[graph.NodeFestival] != 3 {
		t.Errorf("festivals: got %d, want 3", counts[graph.NodeFestival])
	}
	if counts[graph.NodePhase] != 1 {
		t.Errorf("phases: got %d, want 1", counts[graph.NodePhase])
	}
	if counts[graph.NodeSequence] != 1 {
		t.Errorf("sequences: got %d, want 1", counts[graph.NodeSequence])
	}
	if counts[graph.NodeTask] != 2 {
		t.Errorf("tasks: got %d, want 2 (SEQUENCE_GOAL.md excluded)", counts[graph.NodeTask])
	}
	if counts[graph.NodeIntent] != 1 {
		t.Errorf("intents: got %d, want 1", counts[graph.NodeIntent])
	}
	if counts[graph.NodeDesignDoc] != 1 {
		t.Errorf("design_docs: got %d, want 1", counts[graph.NodeDesignDoc])
	}
	if counts[graph.NodeExploreDoc] != 1 {
		t.Errorf("explore_docs: got %d, want 1", counts[graph.NodeExploreDoc])
	}

	// Verify structural edges: festival->phase(1) + phase->seq(1) + seq->task(2) = 4
	containsCount := 0
	for _, e := range g.Edges() {
		if e.Type == graph.EdgeContains {
			containsCount++
		}
	}
	if containsCount != 4 {
		t.Errorf("contains edges: got %d, want 4", containsCount)
	}

	// Verify festival status
	festNode := g.Node("festival:test-fest-TF0001")
	if festNode == nil {
		t.Fatal("festival node test-fest-TF0001 not found")
	}
	if festNode.Status != "active" {
		t.Errorf("festival status: got %q, want %q", festNode.Status, "active")
	}

	doneNode := g.Node("festival:done-fest-DF0001")
	if doneNode == nil {
		t.Fatal("festival node done-fest-DF0001 not found")
	}
	if doneNode.Status != "completed" {
		t.Errorf("festival status: got %q, want %q", doneNode.Status, "completed")
	}
}

func TestScanEmptyCampaign(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	s := scanner.New(root)
	g, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan() error on empty dir: %v", err)
	}
	if len(g.Nodes()) != 0 {
		t.Errorf("nodes: got %d, want 0", len(g.Nodes()))
	}
}

func TestScanCancelledContext(t *testing.T) {
	root := createMinimalCampaign(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := scanner.New(root)
	_, err := s.Scan(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestScanFilesInProjectsDirSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file (not directory) in projects/
	if err := os.WriteFile(filepath.Join(root, "projects/README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create an actual project dir
	if err := os.MkdirAll(filepath.Join(root, "projects/real-proj"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if g.NodeCount() != 1 {
		t.Errorf("nodes: got %d, want 1 (only real-proj)", g.NodeCount())
	}
}

func TestScanMultipleLifecycleDirs(t *testing.T) {
	root := t.TempDir()
	dirs := []struct {
		path   string
		status string
	}{
		{"festivals/active/fest-a", "active"},
		{"festivals/planning/fest-b", "planning"},
		{"festivals/ready/fest-c", "ready"},
		{"festivals/ritual/fest-d", "ritual"},
		{"festivals/dungeon/completed/fest-e", "completed"},
		{"festivals/dungeon/archived/fest-f", "archived"},
		{"festivals/dungeon/someday/fest-g", "someday"},
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d.path), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := scanner.New(root)
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	counts := map[graph.NodeType]int{}
	for _, n := range g.Nodes() {
		counts[n.Type]++
	}
	if counts[graph.NodeFestival] != 7 {
		t.Errorf("festivals: got %d, want 7", counts[graph.NodeFestival])
	}

	// Verify each status
	for _, d := range dirs {
		name := filepath.Base(d.path)
		n := g.Node("festival:" + name)
		if n == nil {
			t.Errorf("missing festival node: %s", name)
			continue
		}
		if n.Status != d.status {
			t.Errorf("festival %s status: got %q, want %q", name, n.Status, d.status)
		}
	}
}

func TestScanPhaseDirPattern(t *testing.T) {
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals/active/pattern-test")
	// Valid phase dirs
	for _, d := range []string{"001_BUILD", "002_TEST", "999_DEPLOY"} {
		if err := os.MkdirAll(filepath.Join(festDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Invalid phase dirs (should be skipped)
	for _, d := range []string{"AB_INVALID", "no_number", "01_short"} {
		if err := os.MkdirAll(filepath.Join(festDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Also a file (should be skipped)
	if err := os.WriteFile(filepath.Join(festDir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New(root)
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	counts := map[graph.NodeType]int{}
	for _, n := range g.Nodes() {
		counts[n.Type]++
	}
	if counts[graph.NodePhase] != 3 {
		t.Errorf("phases: got %d, want 3 (only NNN_ pattern)", counts[graph.NodePhase])
	}
}

func TestScanHiddenDirsSkipped(t *testing.T) {
	root := t.TempDir()
	dirs := []string{
		"projects/.hidden-proj",
		"projects/visible-proj",
		"festivals/active/.hidden-fest",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := scanner.New(root)
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	counts := map[graph.NodeType]int{}
	for _, n := range g.Nodes() {
		counts[n.Type]++
	}
	if counts[graph.NodeProject] != 1 {
		t.Errorf("projects: got %d, want 1 (hidden dir skipped)", counts[graph.NodeProject])
	}
	if counts[graph.NodeFestival] != 0 {
		t.Errorf("festivals: got %d, want 0 (hidden dir skipped)", counts[graph.NodeFestival])
	}
}
