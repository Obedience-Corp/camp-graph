package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// groupedModel builds a Model whose m.groups contains two node types
// with two rows each, all expanded, mirroring the shape produced by
// groupByType for a live FTS result set. The in-memory graph knows
// every referenced NodeID so enter can resolve to a *graph.Node.
func groupedModel(t *testing.T) Model {
	t.Helper()

	g := graph.New()

	n1 := graph.NewNode("file:a/one.go", graph.NodeFile, "a/one.go", "/campaign/a/one.go")
	n2 := graph.NewNode("file:a/two.go", graph.NodeFile, "a/two.go", "/campaign/a/two.go")
	n3 := graph.NewNode("task:task-1", graph.NodeTask, "task-1", "/campaign/festivals/active/task-1")
	n4 := graph.NewNode("task:task-2", graph.NodeTask, "task-2", "/campaign/festivals/active/task-2")
	for _, n := range []*graph.Node{n1, n2, n3, n4} {
		g.AddNode(n)
	}

	m := *New(context.Background(), newTestStore(t), g)
	m.results = []search.QueryResult{
		{NodeID: n3.ID, NodeType: string(graph.NodeTask), Title: "task-1"},
		{NodeID: n4.ID, NodeType: string(graph.NodeTask), Title: "task-2"},
		{NodeID: n1.ID, NodeType: string(graph.NodeFile), Title: "one"},
		{NodeID: n2.ID, NodeType: string(graph.NodeFile), Title: "two"},
	}
	m.groups = groupByType(m.results)
	m.showingAnchors = false
	m.filteredAnchors = nil
	// Simulate a live search so isExplorerFallback returns false.
	m.search.SetValue("query")
	return m
}

func TestFocusedRowIDWalksHeadersWhenGroupsExpanded(t *testing.T) {
	m := groupedModel(t)

	if want := groupVisibleCount(m.groups); want != 6 {
		t.Fatalf("test setup: groupVisibleCount = %d, want 6 (2 headers + 4 rows)", want)
	}

	cases := []struct {
		cursor int
		want   string
	}{
		{0, ""},             // header: task
		{1, "task:task-1"},  // first task row
		{2, "task:task-2"},  // second task row
		{3, ""},             // header: file
		{4, "file:a/one.go"},
		{5, "file:a/two.go"},
	}

	for _, tc := range cases {
		m.cursor = tc.cursor
		got := m.focusedRowID()
		if got != tc.want {
			t.Errorf("cursor=%d: focusedRowID = %q, want %q", tc.cursor, got, tc.want)
		}
	}
}

func TestFocusedRowIDSkipsCollapsedGroupBody(t *testing.T) {
	m := groupedModel(t)
	m.groups[0].Expanded = false // collapse the first group

	// Visible layout: [header0, header1, row1-0, row1-1] (4 entries).
	if want := groupVisibleCount(m.groups); want != 4 {
		t.Fatalf("test setup: groupVisibleCount = %d, want 4", want)
	}

	cases := []struct {
		cursor int
		want   string
	}{
		{0, ""},              // collapsed header0
		{1, ""},              // header1
		{2, "file:a/one.go"}, // first file row
		{3, "file:a/two.go"}, // second file row
	}

	for _, tc := range cases {
		m.cursor = tc.cursor
		got := m.focusedRowID()
		if got != tc.want {
			t.Errorf("cursor=%d: focusedRowID = %q, want %q", tc.cursor, got, tc.want)
		}
	}
}

func TestEnterOnGroupHeaderTogglesExpanded(t *testing.T) {
	m := groupedModel(t)
	m.cursor = 0 // first group header

	m = updateModel(t, m, keyNamed(tea.KeyEnter))
	if m.groups[0].Expanded {
		t.Fatal("expected first group to be collapsed after enter on header")
	}

	m = updateModel(t, m, keyNamed(tea.KeyEnter))
	if !m.groups[0].Expanded {
		t.Fatal("expected first group to be re-expanded after second enter")
	}
	if m.mode == modeMicrograph {
		t.Fatal("expected enter on header to not switch to micrograph mode")
	}
}

func TestEnterOnGroupRowOpensResolvedGraphNode(t *testing.T) {
	m := groupedModel(t)
	m.cursor = 2 // second task row

	m = updateModel(t, m, keyNamed(tea.KeyEnter))

	if m.mode != modeMicrograph {
		t.Fatalf("expected enter on row to switch to micrograph mode, got %v", m.mode)
	}
	if m.focusNode == nil {
		t.Fatal("expected focusNode populated after enter on group row")
	}
	if want := "task:task-2"; m.focusNode.ID != want {
		t.Fatalf("focusNode.ID = %q, want %q", m.focusNode.ID, want)
	}
}

func TestEnterOnEmptyGroupCursorIsNoop(t *testing.T) {
	m := groupedModel(t)
	m.cursor = 99 // out of range

	m = updateModel(t, m, keyNamed(tea.KeyEnter))

	if m.mode == modeMicrograph {
		t.Fatal("expected no mode switch when cursor is out of range over groups")
	}
	if m.focusNode != nil {
		t.Fatalf("expected focusNode to remain nil, got %#v", m.focusNode)
	}
}

func TestQueryResultMsgClampsCursor(t *testing.T) {
	m := groupedModel(t)
	m.queryGen = 1
	m.cursor = 5 // last visible entry before shrink

	shrunk := queryResultMsg{
		gen:     1,
		results: []search.QueryResult{m.results[0]}, // single row -> 1 header + 1 row
	}
	next, _ := m.Update(shrunk)
	mm := next.(Model)

	if want := 1; mm.cursor != want {
		t.Fatalf("cursor = %d, want %d (clamped to groupVisibleCount-1)", mm.cursor, want)
	}
	if got := groupVisibleCount(mm.groups); got != 2 {
		t.Fatalf("groupVisibleCount = %d, want 2", got)
	}
}

func TestQueryResultMsgResetsCursorWhenResultsEmpty(t *testing.T) {
	m := groupedModel(t)
	m.queryGen = 1
	m.cursor = 4

	// Emulate the state the search-esc path sets up: no FTS results,
	// filteredAnchors populated for the empty-query fallback.
	empty := queryResultMsg{gen: 1, results: nil}
	next, _ := m.Update(empty)
	mm := next.(Model)

	if len(mm.groups) != 0 {
		t.Fatalf("expected groups cleared, got %d groups", len(mm.groups))
	}
	if mm.cursor < 0 {
		t.Fatalf("expected cursor clamped >= 0, got %d", mm.cursor)
	}
}
