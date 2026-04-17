package scanner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func TestScanner_Notes_PathStableID(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/JobSearch/Action Plan.md"), "# plan\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	n := g.Node("note:Work/JobSearch/Action Plan.md")
	if n == nil {
		t.Fatal("note node missing; want stable path-based ID")
	}
	if n.Type != graph.NodeNote {
		t.Errorf("type: got %v, want note", n.Type)
	}
	if n.Metadata[graph.MetaPathDepth] != "3" {
		t.Errorf("path_depth: got %q, want 3", n.Metadata[graph.MetaPathDepth])
	}
}

func TestScanner_Notes_FrontmatterFields(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	content := `---
title: My Note
aliases:
  - my-alt
  - another
tags: [research, jobs]
type: reference
status: active
---

# body
`
	writeFile(t, filepath.Join(root, "Work/JobSearch/plan.md"), content)

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	n := g.Node("note:Work/JobSearch/plan.md")
	if n == nil {
		t.Fatal("note node missing")
	}
	want := map[string]string{
		graph.MetaNoteTitle:   "My Note",
		graph.MetaNoteAliases: "another,my-alt",
		graph.MetaNoteTags:    "jobs,research",
		graph.MetaNoteType:    "reference",
		graph.MetaNoteStatus:  "active",
	}
	for k, v := range want {
		if got := n.Metadata[k]; got != v {
			t.Errorf("metadata[%s]: got %q, want %q", k, got, v)
		}
	}
}

func TestScanner_Notes_FilenameTitleFallback(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/JobSearch/action_plan.md"), "# body\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	n := g.Node("note:Work/JobSearch/action_plan.md")
	if n == nil {
		t.Fatal("note node missing")
	}
	if got := n.Metadata[graph.MetaNoteTitle]; got != "action plan" {
		t.Errorf("title fallback: got %q, want %q", got, "action plan")
	}
}

func TestScanner_Notes_ContainedByFolderScope(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/JobSearch/plan.md"), "# plan\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	var foundEdge bool
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeContains {
			continue
		}
		if e.FromID == "folder:Work/JobSearch" && e.ToID == "note:Work/JobSearch/plan.md" {
			foundEdge = true
			if e.Source != graph.SourceStructural {
				t.Errorf("note contains edge source=%q, want structural", e.Source)
			}
			break
		}
	}
	if !foundEdge {
		t.Fatalf("expected folder -> note contains edge; edges=%v", edgeSummary(g))
	}
}

func TestScanner_Notes_ArtifactMarkdownDoesNotDuplicate(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, ".campaign/intents/inbox/idea.md"),
		"---\ntitle: Idea\n---\n# idea\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// The intent artifact node must exist.
	if g.Node("intent:idea") == nil {
		t.Fatal("intent artifact node missing")
	}
	// The markdown file must NOT also be promoted to a note node, because
	// intents already own that path.
	if g.Node("note:.campaign/intents/inbox/idea.md") != nil {
		t.Error("intent markdown was duplicated as a note node")
	}
}

func TestScanner_Notes_BadFrontmatterDoesNotBreakNode(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Malformed frontmatter - missing closing delimiter.
	writeFile(t, filepath.Join(root, "Work/messed.md"), "---\ntitle: Broken\n\n# body\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	n := g.Node("note:Work/messed.md")
	if n == nil {
		t.Fatal("note node missing when frontmatter is malformed")
	}
	// Title falls back to filename.
	if got := n.Metadata[graph.MetaNoteTitle]; !strings.Contains(got, "messed") {
		t.Errorf("title fallback: got %q, want contains 'messed'", got)
	}
}

// edgeSummary returns a short string listing edges for diagnostics.
func edgeSummary(g *graph.Graph) string {
	var parts []string
	for _, e := range g.Edges() {
		parts = append(parts, e.FromID+"-"+string(e.Type)+"->"+e.ToID)
	}
	return strings.Join(parts, " ; ")
}
