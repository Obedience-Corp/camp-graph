package render

import (
	"context"
	"io"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Render writes the graph in the specified format to w.
// DOT is written directly; SVG/PNG are rendered via go-graphviz.
func Render(ctx context.Context, w io.Writer, g *graph.Graph, format Format) error {
	if format == FormatDOT {
		return RenderDOT(w, g)
	}
	return RenderGraphviz(ctx, w, g, format)
}
