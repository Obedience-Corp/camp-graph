package main

import (
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func seedScopeGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	root := graph.NewNode("folder:.", graph.NodeFolder, ".", "/root")
	work := graph.NewNode("folder:Work", graph.NodeFolder, "Work", "/root/Work")
	jobs := graph.NewNode("folder:Work/JobSearch", graph.NodeFolder, "Work/JobSearch", "/root/Work/JobSearch")
	other := graph.NewNode("folder:Other", graph.NodeFolder, "Other", "/root/Other")
	plan := graph.NewNode("note:Work/JobSearch/plan.md", graph.NodeNote, "Work/JobSearch/plan.md", "/root/Work/JobSearch/plan.md")
	plan.Metadata[graph.MetaGitState] = "tracked"
	stray := graph.NewNode("note:Other/random.md", graph.NodeNote, "Other/random.md", "/root/Other/random.md")
	stray.Metadata[graph.MetaGitState] = "untracked"
	for _, n := range []*graph.Node{root, work, jobs, other, plan, stray} {
		g.AddNode(n)
	}
	g.AddEdge(graph.NewEdge(root.ID, work.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(work.ID, jobs.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(jobs.ID, plan.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(root.ID, other.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(other.ID, stray.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	// Inferred cross-scope edge to test mode filtering.
	g.AddEdge(graph.NewEdge(plan.ID, stray.ID, graph.EdgeRelatesTo, 0.5, graph.SourceInferred))
	return g
}

func TestSliceByScope_KeepsOnlyScopeSubtree(t *testing.T) {
	g := seedScopeGraph(t)
	sliced, err := sliceByScope(g, "Work/JobSearch")
	if err != nil {
		t.Fatalf("slice: %v", err)
	}
	for _, n := range sliced.Nodes() {
		if n.ID == "folder:Other" || n.ID == "note:Other/random.md" {
			t.Errorf("leaked node %q outside scope", n.ID)
		}
	}
	if sliced.Node("folder:Work/JobSearch") == nil {
		t.Error("expected folder:Work/JobSearch in slice")
	}
	if sliced.Node("note:Work/JobSearch/plan.md") == nil {
		t.Error("expected note:Work/JobSearch/plan.md in slice")
	}
}

func TestSliceByScope_UnknownScopeErrors(t *testing.T) {
	g := seedScopeGraph(t)
	if _, err := sliceByScope(g, "DoesNotExist"); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestFilterByRelationMode_Structural(t *testing.T) {
	g := seedScopeGraph(t)
	filtered := filterByRelationMode(g, "structural")
	for _, e := range filtered.Edges() {
		if e.Source != graph.SourceStructural {
			t.Errorf("structural filter leaked edge source=%v", e.Source)
		}
	}
}

func TestFilterByRelationMode_Semantic(t *testing.T) {
	g := seedScopeGraph(t)
	filtered := filterByRelationMode(g, "semantic")
	for _, e := range filtered.Edges() {
		if e.Source != graph.SourceInferred {
			t.Errorf("semantic filter leaked edge source=%v", e.Source)
		}
	}
}

func TestFilterByRelationMode_HybridKeepsAll(t *testing.T) {
	g := seedScopeGraph(t)
	filtered := filterByRelationMode(g, "hybrid")
	if filtered.EdgeCount() != g.EdgeCount() {
		t.Errorf("hybrid filter changed edge count: got %d, want %d",
			filtered.EdgeCount(), g.EdgeCount())
	}
}

func TestFilterByTrackedState_Tracked(t *testing.T) {
	g := seedScopeGraph(t)
	filtered := filterByTrackedState(g, "tracked")
	if n := filtered.Node("note:Other/random.md"); n != nil {
		t.Error("tracked filter leaked untracked note")
	}
	if n := filtered.Node("note:Work/JobSearch/plan.md"); n == nil {
		t.Error("tracked filter dropped tracked note")
	}
}

func TestFilterByTrackedState_Untracked(t *testing.T) {
	g := seedScopeGraph(t)
	filtered := filterByTrackedState(g, "untracked")
	if n := filtered.Node("note:Work/JobSearch/plan.md"); n != nil {
		t.Error("untracked filter leaked tracked note")
	}
	if n := filtered.Node("note:Other/random.md"); n == nil {
		t.Error("untracked filter dropped untracked note")
	}
}
