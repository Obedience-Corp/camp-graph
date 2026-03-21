package render

import (
	"bytes"
	"context"
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
