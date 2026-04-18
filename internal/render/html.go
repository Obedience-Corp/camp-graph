package render

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

//go:embed templates/graph.html.tmpl
var htmlTemplates embed.FS

var htmlTmpl = template.Must(template.ParseFS(htmlTemplates, "templates/graph.html.tmpl"))

type htmlLegendEntry struct {
	Label     string
	Color     string
	FillColor string
}

type htmlData struct {
	Title   string
	Summary string
	Legend  []htmlLegendEntry
	SVG     template.HTML
}

// RenderHTML writes a self-contained HTML document embedding the graph as inline SVG,
// with a legend of node types and a summary of node/edge counts. No JS, no CDN.
func RenderHTML(ctx context.Context, w io.Writer, g *graph.Graph) error {
	var svgBuf bytes.Buffer
	if err := RenderGraphviz(ctx, &svgBuf, g, FormatSVG); err != nil {
		return graphErrors.Wrap(err, "render SVG for HTML embed")
	}

	data := htmlData{
		Title:   "Campaign Graph",
		Summary: fmt.Sprintf("%s, %s", pluralize(g.NodeCount(), "node"), pluralize(g.EdgeCount(), "edge")),
		Legend:  buildLegend(g),
		SVG:     template.HTML(stripSVGProlog(svgBuf.String())), //nolint:gosec // go-graphviz output, not user-controlled HTML
	}

	return htmlTmpl.Execute(w, data)
}

// buildLegend returns legend entries for each distinct node type present in the graph,
// ordered by node type string for stable output.
func buildLegend(g *graph.Graph) []htmlLegendEntry {
	seen := make(map[graph.NodeType]struct{})
	for _, n := range g.Nodes() {
		seen[n.Type] = struct{}{}
	}
	types := make([]graph.NodeType, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return string(types[i]) < string(types[j]) })

	entries := make([]htmlLegendEntry, 0, len(types))
	for _, t := range types {
		style := StyleForNode(t)
		entries = append(entries, htmlLegendEntry{
			Label:     t.String(),
			Color:     style.Color,
			FillColor: style.FillColor,
		})
	}
	return entries
}

// stripSVGProlog removes the XML prolog and DOCTYPE declarations from a
// standalone SVG document so it can be safely inlined in HTML.
func stripSVGProlog(svg string) string {
	if idx := strings.Index(svg, "<svg"); idx > 0 {
		return svg[idx:]
	}
	return svg
}

func pluralize(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}
