package render

import (
	"bytes"
	"context"
	"fmt"
	"io"

	graphviz "github.com/goccy/go-graphviz"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// RenderGraphviz renders the graph to SVG or PNG using go-graphviz (WASM).
// It generates DOT via RenderDOT, then parses and renders to the target format.
func RenderGraphviz(ctx context.Context, w io.Writer, g *graph.Graph, format Format) error {
	var dotBuf bytes.Buffer
	if err := RenderDOT(&dotBuf, g); err != nil {
		return fmt.Errorf("generate DOT: %w", err)
	}

	gv, err := graphviz.New(ctx)
	if err != nil {
		return fmt.Errorf("init graphviz: %w", err)
	}
	defer gv.Close()

	parsed, err := graphviz.ParseBytes(dotBuf.Bytes())
	if err != nil {
		return fmt.Errorf("parse DOT: %w", err)
	}
	defer parsed.Close()

	var gvFormat graphviz.Format
	switch format {
	case FormatSVG:
		gvFormat = graphviz.SVG
	case FormatPNG:
		gvFormat = graphviz.PNG
	default:
		return fmt.Errorf("graphviz render does not handle format %q", format)
	}

	if err := gv.Render(ctx, parsed, gvFormat, w); err != nil {
		return fmt.Errorf("render %s: %w", format, err)
	}
	return nil
}
