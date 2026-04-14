package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func TestRenderDOT_BasicOutput(t *testing.T) {
	g := graph.New()
	g.AddNode(graph.NewNode("project:camp", graph.NodeProject, "camp", "projects/camp"))
	g.AddNode(graph.NewNode("festival:test", graph.NodeFestival, "test-fest", "festivals/active/test"))
	g.AddEdge(graph.NewEdge("festival:test", "project:camp", graph.EdgeLinksTo, 1.0, graph.SourceExplicit))

	var buf bytes.Buffer
	err := RenderDOT(&buf, g)
	if err != nil {
		t.Fatalf("RenderDOT: %v", err)
	}

	out := buf.String()

	if !strings.HasPrefix(out, "digraph campaign {") {
		t.Error("missing digraph header")
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "}") {
		t.Error("missing closing brace")
	}
	if !strings.Contains(out, `"project:camp"`) {
		t.Error("missing project node")
	}
	if !strings.Contains(out, `"festival:test"`) {
		t.Error("missing festival node")
	}
	if !strings.Contains(out, "->") {
		t.Error("missing edge")
	}
	if !strings.Contains(out, "links_to") {
		t.Error("missing edge label")
	}
}

func TestRenderDOT_EmptyGraph(t *testing.T) {
	g := graph.New()
	var buf bytes.Buffer
	if err := RenderDOT(&buf, g); err != nil {
		t.Fatalf("RenderDOT empty: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "digraph campaign") {
		t.Error("empty graph should still produce valid DOT header")
	}
}

func TestRenderDOT_AllNodeTypes(t *testing.T) {
	g := graph.New()
	types := []struct {
		id    string
		ntype graph.NodeType
	}{
		{"p:1", graph.NodeProject},
		{"f:1", graph.NodeFestival},
		{"c:1", graph.NodeChain},
		{"ph:1", graph.NodePhase},
		{"s:1", graph.NodeSequence},
		{"t:1", graph.NodeTask},
		{"i:1", graph.NodeIntent},
		{"d:1", graph.NodeDesignDoc},
		{"e:1", graph.NodeExploreDoc},
		{"fi:1", graph.NodeFile},
		{"fn:1", graph.NodeFunction},
		{"td:1", graph.NodeTypeDef},
		{"pk:1", graph.NodePackage},
	}

	for _, tt := range types {
		g.AddNode(graph.NewNode(tt.id, tt.ntype, "test-"+tt.id, "/"+tt.id))
	}

	var buf bytes.Buffer
	if err := RenderDOT(&buf, g); err != nil {
		t.Fatalf("RenderDOT: %v", err)
	}

	out := buf.String()
	for _, tt := range types {
		if !strings.Contains(out, `"`+tt.id+`"`) {
			t.Errorf("missing node %s in DOT output", tt.id)
		}
	}
}

func TestEscDOT(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{`say "hi"`, `say \"hi\"`},
		{`path\to\file`, `path\\to\\file`},
	}
	for _, tt := range tests {
		got := escDOT(tt.input)
		if got != tt.want {
			t.Errorf("escDOT(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStyleForNode_UnknownType(t *testing.T) {
	style := StyleForNode("unknown_type")
	if style.Shape != defaultStyle.Shape {
		t.Errorf("unknown type shape = %q, want %q", style.Shape, defaultStyle.Shape)
	}
}
