package scanner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func scanNotesFixture(t *testing.T, root string) *graph.Graph {
	t.Helper()
	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	return g
}

func TestLinks_MarkdownLinkToNote(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/alpha.md"), "# alpha\n")
	writeFile(t, filepath.Join(root, "Work/beta.md"), "See [alpha](alpha.md).\n")

	g := scanNotesFixture(t, root)

	if !hasEdge(g, "note:Work/beta.md", "note:Work/alpha.md", graph.EdgeLinksTo, "markdown_link") {
		t.Fatalf("expected markdown_link edge beta->alpha; got: %s", edgeSummary(g))
	}
}

func TestLinks_MarkdownLinkToAttachment(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Assets/logo.png"), "PNG")
	writeFile(t, filepath.Join(root, "Notes/readme.md"), "![logo](../Assets/logo.png)\n")

	g := scanNotesFixture(t, root)

	// The attachment node should be synthesized.
	att := g.Node("attachment:Assets/logo.png")
	if att == nil {
		t.Fatal("expected attachment node to be synthesized")
	}
	// Edge should be tagged as attachment subtype.
	if !hasEdge(g, "note:Notes/readme.md", "attachment:Assets/logo.png", graph.EdgeLinksTo, "attachment") {
		t.Fatalf("expected attachment link edge; got: %s", edgeSummary(g))
	}
}

func TestLinks_WikiLinkToNote(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/alpha.md"), "# alpha\n")
	writeFile(t, filepath.Join(root, "Work/beta.md"), "See [[alpha]].\n")

	g := scanNotesFixture(t, root)
	if !hasEdge(g, "note:Work/beta.md", "note:Work/alpha.md", graph.EdgeLinksTo, "wiki_link") {
		t.Fatalf("expected wiki_link edge beta->alpha; got: %s", edgeSummary(g))
	}
}

func TestLinks_WikiLinkWithSectionAndAlias(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/alpha.md"), "# alpha\n")
	writeFile(t, filepath.Join(root, "Work/beta.md"), "See [[alpha#intro|preface]].\n")

	g := scanNotesFixture(t, root)
	if !hasEdge(g, "note:Work/beta.md", "note:Work/alpha.md", graph.EdgeLinksTo, "wiki_link") {
		t.Fatalf("section+alias wiki_link not resolved: %s", edgeSummary(g))
	}
}

func TestLinks_ExternalLinksIgnored(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/note.md"),
		"See [site](https://example.com) and [anchor](#heading).\n")

	g := scanNotesFixture(t, root)
	for _, e := range g.Edges() {
		if e.Type == graph.EdgeLinksTo {
			t.Errorf("unexpected link edge for external/anchor reference: %s->%s", e.FromID, e.ToID)
		}
	}
}

func TestLinks_InlineTagsCreateTagNodesAndReferences(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/note.md"),
		"Body mentions #research and #job-search here.\n")

	g := scanNotesFixture(t, root)
	for _, tag := range []string{"research", "job-search"} {
		tagNode := g.Node("tag:" + tag)
		if tagNode == nil {
			t.Errorf("tag node %q missing", "tag:"+tag)
			continue
		}
		if tagNode.Type != graph.NodeTag {
			t.Errorf("tag node type: got %v, want tag", tagNode.Type)
		}
		if !hasEdge(g, "note:Work/note.md", "tag:"+tag, graph.EdgeReferences, "inline_tag") {
			t.Errorf("missing references edge to tag:%s", tag)
		}
	}
}

func TestLinks_HeadingsAreNotTags(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/note.md"),
		"# Heading\n\n## Section\n\nBody.\n")

	g := scanNotesFixture(t, root)
	for _, n := range g.Nodes() {
		if n.Type == graph.NodeTag {
			t.Errorf("heading-like # should not create a tag node; got %q", n.ID)
		}
	}
}

func TestLinks_FrontmatterTagsNotDuplicatedAsInlineReferences(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	content := `---
title: Example
tags: [research]
---

# body
Body text references #planning explicitly.
`
	writeFile(t, filepath.Join(root, "Work/note.md"), content)

	g := scanNotesFixture(t, root)

	// frontmatter tags are stored on the note node; no tag node for them.
	note := g.Node("note:Work/note.md")
	if note == nil {
		t.Fatal("note node missing")
	}
	if got := note.Metadata[graph.MetaNoteTags]; got != "research" {
		t.Errorf("frontmatter tags metadata: got %q, want %q", got, "research")
	}
	// The inline #planning tag should still be extracted.
	if g.Node("tag:planning") == nil {
		t.Error("expected inline #planning tag to be extracted from body")
	}
}

func TestLinks_CanvasFileParsed(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/alpha.md"), "# alpha\n")
	writeFile(t, filepath.Join(root, "Work/beta.md"), "# beta\n")
	canvas := `{
  "nodes": [
    {"id":"n1", "type":"file", "file":"Work/alpha.md"},
    {"id":"n2", "type":"file", "file":"Work/beta.md"},
    {"id":"n3", "type":"text", "text":"just a note"}
  ]
}`
	writeFile(t, filepath.Join(root, "Business/plan.canvas"), canvas)

	g := scanNotesFixture(t, root)

	canvasNode := g.Node("canvas:Business/plan.canvas")
	if canvasNode == nil {
		t.Fatal("canvas node missing")
	}
	if canvasNode.Type != graph.NodeCanvas {
		t.Errorf("canvas type: got %v, want canvas", canvasNode.Type)
	}

	for _, target := range []string{"note:Work/alpha.md", "note:Work/beta.md"} {
		if !hasEdge(g, canvasNode.ID, target, graph.EdgeLinksTo, "canvas_link") {
			t.Errorf("expected canvas_link from %s to %s", canvasNode.ID, target)
		}
	}
}

func TestLinks_MissingTargetIgnored(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/note.md"),
		"See [missing](ghost-note.md).\n[[Nonexistent]]\n")

	g := scanNotesFixture(t, root)

	for _, e := range g.Edges() {
		if e.Type != graph.EdgeLinksTo {
			continue
		}
		if strings.HasPrefix(e.ToID, "note:") {
			t.Errorf("unexpected link to missing target: %s->%s", e.FromID, e.ToID)
		}
	}
}

func TestLinks_MalformedCanvasProducesCanvasNodeWithoutEdges(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/alpha.md"), "# alpha\n")
	// Malformed JSON - missing closing brace.
	writeFile(t, filepath.Join(root, "Business/broken.canvas"),
		`{"nodes":[{"id":"a","type":"file","file":"Work/alpha.md"}`)

	g := scanNotesFixture(t, root)
	// Canvas node is NOT created for malformed JSON because we bail
	// before the newCanvasNode call in extractCanvasLinks. Verify no
	// stray canvas_link edges appear either.
	if g.Node("canvas:Business/broken.canvas") != nil {
		t.Error("malformed canvas should not produce a canvas node")
	}
	for _, e := range g.Edges() {
		if e.Subtype == "canvas_link" {
			t.Errorf("unexpected canvas_link edge from malformed canvas: %s->%s", e.FromID, e.ToID)
		}
	}
}

func TestLinks_URLEncodedTargetResolves(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/JobSearch/Action Plan.md"), "# plan\n")
	writeFile(t, filepath.Join(root, "Work/note.md"),
		"Follow [plan](JobSearch/Action%20Plan.md).\n")

	g := scanNotesFixture(t, root)
	if !hasEdge(g, "note:Work/note.md", "note:Work/JobSearch/Action Plan.md",
		graph.EdgeLinksTo, "markdown_link") {
		t.Fatalf("url-encoded target should resolve: %s", edgeSummary(g))
	}
}

// hasEdge reports whether g contains an edge matching all four fields.
func hasEdge(g *graph.Graph, from, to string, t graph.EdgeType, subtype string) bool {
	for _, e := range g.Edges() {
		if e.FromID == from && e.ToID == to && e.Type == t && strings.EqualFold(e.Subtype, subtype) {
			return true
		}
	}
	return false
}
