package render

import (
	"context"
	"io"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Render writes the graph in the specified format to w.
// DOT and JSON are written directly; HTML wraps an SVG render;
// SVG/PNG are rendered via go-graphviz.
func Render(ctx context.Context, w io.Writer, g *graph.Graph, format Format) error {
	switch format {
	case FormatDOT:
		return RenderDOT(w, g)
	case FormatJSON:
		return RenderJSON(w, g)
	case FormatHTML:
		return RenderHTML(ctx, w, g)
	default:
		return RenderGraphviz(ctx, w, g, format)
	}
}
