package tui

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func TestNewStartsOnScopeAnchors(t *testing.T) {
	g := newTestGraph()
	store := newTestStore(t)

	m := New(context.Background(), store, g)
	if m == nil {
		t.Fatal("New returned nil model")
	}
	if !m.showingAnchors {
		t.Fatal("expected model to start in scope-anchor mode")
	}
	if m.relationMode != RelationHybrid {
		t.Fatalf("expected relation mode %q, got %q", RelationHybrid, m.relationMode)
	}
	if got, want := len(m.filtered), 3; got != want {
		t.Fatalf("expected %d initial anchors, got %d", want, got)
	}

	gotNames := []string{m.filtered[0].Name, m.filtered[1].Name, m.filtered[2].Name}
	wantNames := []string{".", "projects/camp-graph", "workflow"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("anchor %d: got %q, want %q", i, gotNames[i], wantNames[i])
		}
	}
}

func TestSearchEscClearsQueryAndRestoresAllNodes(t *testing.T) {
	model := *New(context.Background(), newTestStore(t), newTestGraph())

	model = updateModel(t, model, keyRunes("/"))
	if !model.searching {
		t.Fatal("expected search mode to be active after /")
	}

	model = updateModel(t, model, keyRunes("camp"))
	if got := model.search.Value(); got != "camp" {
		t.Fatalf("expected search query %q, got %q", "camp", got)
	}
	if len(model.filtered) == 0 {
		t.Fatal("expected filtered results for query")
	}

	model = updateModel(t, model, keyNamed(tea.KeyEsc))
	if model.searching {
		t.Fatal("expected search mode to be inactive after esc")
	}
	if got := model.search.Value(); got != "" {
		t.Fatalf("expected cleared search query, got %q", got)
	}
	if got, want := len(model.filtered), len(model.nodes); got != want {
		t.Fatalf("expected esc to restore all nodes (%d), got %d", want, got)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor reset to 0 after esc, got %d", model.cursor)
	}
}

func TestEnterOpensMicrographAndEscReturnsToList(t *testing.T) {
	model := *New(context.Background(), newTestStore(t), newTestGraph())

	model = updateModel(t, model, keyNamed(tea.KeyEnter))
	if model.mode != modeMicrograph {
		t.Fatalf("expected micrograph mode after enter, got %v", model.mode)
	}
	if model.focusNode == nil || model.focusNode.Name != "." {
		t.Fatalf("expected focus node %q, got %#v", ".", model.focusNode)
	}
	if len(model.neighbors) != 2 {
		t.Fatalf("expected 2 neighbors for campaign root, got %d", len(model.neighbors))
	}

	model = updateModel(t, model, keyNamed(tea.KeyEsc))
	if model.mode != modeList {
		t.Fatalf("expected list mode after esc, got %v", model.mode)
	}
	if model.focusNode == nil || model.focusNode.Name != "." {
		t.Fatalf("expected focused node to remain %q after exiting micrograph, got %#v", ".", model.focusNode)
	}
}

func updateModel(t *testing.T, model Model, msg tea.KeyMsg) Model {
	t.Helper()

	next, _ := model.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", next)
	}
	return updated
}

func keyNamed(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

func keyRunes(input string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(input)}
}

func newTestStore(t *testing.T) *graph.Store {
	t.Helper()
	store, err := graph.OpenStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newTestGraph() *graph.Graph {
	g := graph.New()

	root := graph.NewNode("folder:.", graph.NodeFolder, ".", "/campaign")
	root.Metadata[graph.MetaScopeKind] = graph.ScopeKindCampaignRoot
	root.Metadata[graph.MetaPathDepth] = "0"

	repo := graph.NewNode("folder:projects/camp-graph", graph.NodeFolder, "projects/camp-graph", "/campaign/projects/camp-graph")
	repo.Metadata[graph.MetaScopeKind] = graph.ScopeKindRepoRoot
	repo.Metadata[graph.MetaPathDepth] = "1"

	workflow := graph.NewNode("folder:workflow", graph.NodeFolder, "workflow", "/campaign/workflow")
	workflow.Metadata[graph.MetaScopeKind] = graph.ScopeKindCampaignBucket
	workflow.Metadata[graph.MetaPathDepth] = "1"

	file := graph.NewNode("file:projects/camp-graph/internal/tui/model.go", graph.NodeFile, "projects/camp-graph/internal/tui/model.go", "/campaign/projects/camp-graph/internal/tui/model.go")

	g.AddNode(root)
	g.AddNode(repo)
	g.AddNode(workflow)
	g.AddNode(file)

	g.AddEdge(graph.NewEdge(root.ID, repo.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(root.ID, workflow.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	g.AddEdge(graph.NewEdge(repo.ID, file.ID, graph.EdgeContains, 1.0, graph.SourceStructural))

	return g
}
