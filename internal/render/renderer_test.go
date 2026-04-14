package render

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func testGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(graph.NewNode("project:test", graph.NodeProject, "test-project", "/test"))
	g.AddNode(graph.NewNode("festival:test", graph.NodeFestival, "test-festival", "/fest"))
	g.AddEdge(graph.NewEdge("festival:test", "project:test", graph.EdgeLinksTo, 1.0, graph.SourceExplicit))
	return g
}

func TestRenderDOTFormat(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	g := testGraph()

	if err := Render(ctx, &buf, g, FormatDOT); err != nil {
		t.Fatalf("Render DOT: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "digraph campaign") {
		t.Error("DOT output missing digraph header")
	}
	if !strings.Contains(out, "test-project") {
		t.Error("DOT output missing test-project node")
	}
}

func TestRenderSVGFormat(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	g := testGraph()

	if err := Render(ctx, &buf, g, FormatSVG); err != nil {
		t.Fatalf("Render SVG: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "<svg") {
		t.Error("SVG output missing <svg tag")
	}
	if !strings.Contains(out, "</svg>") {
		t.Error("SVG output missing closing </svg> tag")
	}
}

func TestRenderJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	g := testGraph()

	if err := Render(ctx, &buf, g, FormatJSON); err != nil {
		t.Fatalf("Render JSON: %v", err)
	}

	var envelope struct {
		Version string           `json:"version"`
		Nodes   []map[string]any `json:"nodes"`
		Edges   []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if envelope.Version == "" {
		t.Error("JSON envelope missing version field")
	}
	if len(envelope.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(envelope.Nodes))
	}
	if len(envelope.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(envelope.Edges))
	}

	// Deterministic output: sorted by ID means project:test precedes festival:test alphabetically.
	foundProject := false
	for _, n := range envelope.Nodes {
		if n["id"] == "project:test" && n["name"] == "test-project" {
			foundProject = true
			break
		}
	}
	if !foundProject {
		t.Error("JSON output missing project:test node with round-tripped fields")
	}

	// Re-render and compare byte-for-byte for determinism.
	var buf2 bytes.Buffer
	if err := Render(ctx, &buf2, g, FormatJSON); err != nil {
		t.Fatalf("Render JSON (second pass): %v", err)
	}
	if !bytes.Equal(buf.Bytes(), buf2.Bytes()) {
		t.Error("JSON output is not deterministic across renders")
	}
}

func TestRenderPNGFormat(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	g := testGraph()

	if err := Render(ctx, &buf, g, FormatPNG); err != nil {
		t.Fatalf("Render PNG: %v", err)
	}

	data := buf.Bytes()
	// PNG files start with the magic bytes: 137 80 78 71 13 10 26 10
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < 8 {
		t.Fatal("PNG output too short")
	}
	for i, b := range pngHeader {
		if data[i] != b {
			t.Fatalf("PNG header byte %d: got %x, want %x", i, data[i], b)
		}
	}
}

func TestRenderHTMLFormat(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	g := testGraph()

	if err := Render(ctx, &buf, g, FormatHTML); err != nil {
		t.Fatalf("Render HTML: %v", err)
	}

	out := buf.String()
	requires := []string{
		"<!DOCTYPE html>",
		"<html",
		"</html>",
		"<svg",
		"</svg>",
		"project:test",   // node ID appears unescaped in SVG <title>
		"festival:test",  // second node also present
		"2 node",         // summary reflects node count
		"1 edge",         // summary reflects edge count
		"Campaign Graph", // page title
	}
	for _, want := range requires {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}

	// Must not pull in external scripts or stylesheets.
	forbidden := []string{
		"<script src=",
		"cdn.",
		`href="http`,
	}
	for _, bad := range forbidden {
		if strings.Contains(out, bad) {
			t.Errorf("HTML output contains external dep marker %q", bad)
		}
	}
}
