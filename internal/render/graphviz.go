package render

import (
	"bytes"
	"context"
	"io"

	graphviz "github.com/goccy/go-graphviz"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// RenderGraphviz renders the graph to SVG or PNG using go-graphviz (WASM).
// It generates DOT via RenderDOT, then parses and renders to the target format.
func RenderGraphviz(ctx context.Context, w io.Writer, g *graph.Graph, format Format) error {
	var dotBuf bytes.Buffer
	if err := RenderDOT(&dotBuf, g); err != nil {
		return graphErrors.Wrap(err, "generate DOT")
	}

	gv, err := graphviz.New(ctx)
	if err != nil {
		return graphErrors.Wrap(err, "init graphviz")
	}
	defer gv.Close()

	parsed, err := graphviz.ParseBytes(dotBuf.Bytes())
	if err != nil {
		return graphErrors.Wrap(err, "parse DOT")
	}
	defer parsed.Close()

	var gvFormat graphviz.Format
	switch format {
	case FormatSVG:
		gvFormat = graphviz.SVG
	case FormatPNG:
		gvFormat = graphviz.PNG
	default:
		return graphErrors.New("graphviz render does not handle format " + string(format))
	}

	if err := gv.Render(ctx, parsed, gvFormat, w); err != nil {
		return graphErrors.Wrapf(err, "render %s", format)
	}
	return nil
}
