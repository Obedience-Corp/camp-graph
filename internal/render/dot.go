package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// NodeStyle defines DOT visual attributes for a node type.
type NodeStyle struct {
	Shape     string
	Color     string
	FillColor string
}

// styleMap maps node types to their DOT visual styles.
var styleMap = map[graph.NodeType]NodeStyle{
	graph.NodeProject:    {Shape: "box", Color: "#00BCD4", FillColor: "#E0F7FA"},
	graph.NodeFestival:   {Shape: "hexagon", Color: "#E040FB", FillColor: "#F3E5F5"},
	graph.NodeChain:      {Shape: "component", Color: "#FF9800", FillColor: "#FFF3E0"},
	graph.NodePhase:      {Shape: "folder", Color: "#FDD835", FillColor: "#FFFDE7"},
	graph.NodeSequence:   {Shape: "note", Color: "#66BB6A", FillColor: "#E8F5E9"},
	graph.NodeTask:       {Shape: "ellipse", Color: "#78909C", FillColor: "#ECEFF1"},
	graph.NodeIntent:     {Shape: "diamond", Color: "#42A5F5", FillColor: "#E3F2FD"},
	graph.NodeDesignDoc:  {Shape: "tab", Color: "#EF5350", FillColor: "#FFEBEE"},
	graph.NodeExploreDoc: {Shape: "tab", Color: "#AB47BC", FillColor: "#F3E5F5"},
	graph.NodeFile:       {Shape: "note", Color: "#BDBDBD", FillColor: "#F5F5F5"},
	graph.NodeFunction:   {Shape: "cds", Color: "#26A69A", FillColor: "#E0F2F1"},
	graph.NodeTypeDef:    {Shape: "record", Color: "#5C6BC0", FillColor: "#E8EAF6"},
	graph.NodePackage:    {Shape: "box3d", Color: "#7E57C2", FillColor: "#EDE7F6"},
}

var defaultStyle = NodeStyle{Shape: "ellipse", Color: "#9E9E9E", FillColor: "#FAFAFA"}

// RenderDOT writes the graph in Graphviz DOT format to the writer.
func RenderDOT(w io.Writer, g *graph.Graph) error {
	fmt.Fprintln(w, "digraph campaign {")
	fmt.Fprintln(w, "  rankdir=LR;")
	fmt.Fprintln(w, `  node [style=filled, fontname="Helvetica"];`)
	fmt.Fprintln(w, `  edge [fontname="Helvetica", fontsize=10];`)
	fmt.Fprintln(w)

	for _, n := range g.Nodes() {
		style := styleForNode(n.Type)
		label := escDOT(n.Name)
		fmt.Fprintf(w, "  %q [label=%q, shape=%s, color=%q, fillcolor=%q];\n",
			n.ID, label, style.Shape, style.Color, style.FillColor)
	}
	fmt.Fprintln(w)

	for _, e := range g.Edges() {
		label := escDOT(string(e.Type))
		fmt.Fprintf(w, "  %q -> %q [label=%q];\n", e.FromID, e.ToID, label)
	}

	fmt.Fprintln(w, "}")
	return nil
}

func styleForNode(t graph.NodeType) NodeStyle {
	if s, ok := styleMap[t]; ok {
		return s
	}
	return defaultStyle
}

func escDOT(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
