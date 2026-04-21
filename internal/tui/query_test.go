package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/Obedience-Corp/camp-graph/internal/tui/chips"
)

type stubQuerier struct {
	mu      sync.Mutex
	calls   []*stubCall
	release chan struct{}
}

type stubCall struct {
	term string
	ctx  context.Context
}

func (s *stubQuerier) Search(ctx context.Context, opts search.QueryOptions) ([]search.QueryResult, error) {
	s.mu.Lock()
	call := &stubCall{term: opts.Term, ctx: ctx}
	s.calls = append(s.calls, call)
	s.mu.Unlock()
	select {
	case <-s.release:
		return []search.QueryResult{{NodeID: opts.Term, NodeType: "task", Title: opts.Term}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *stubQuerier) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func waitFor(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestQueryCancellation(t *testing.T) {
	stub := &stubQuerier{release: make(chan struct{})}

	terms := []string{"a", "ab", "abc"}
	cancelsByTerm := make(map[string]context.CancelFunc, len(terms))
	msgCh := make(chan tea.Msg, len(terms))
	for i, term := range terms {
		ctx, cancel := context.WithCancel(context.Background())
		cancelsByTerm[term] = cancel
		cmd := runQueryCmd(ctx, stub, search.QueryOptions{Term: term}, uint64(i+1))
		go func() { msgCh <- cmd() }()
	}

	waitFor(t, func() bool { return stub.callCount() == 3 })

	cancelsByTerm["a"]()
	cancelsByTerm["ab"]()
	defer cancelsByTerm["abc"]()

	callCtxByTerm := func(term string) context.Context {
		stub.mu.Lock()
		defer stub.mu.Unlock()
		for _, c := range stub.calls {
			if c.term == term {
				return c.ctx
			}
		}
		t.Fatalf("no call observed for term %q", term)
		return nil
	}
	for _, term := range []string{"a", "ab"} {
		select {
		case <-callCtxByTerm(term).Done():
		case <-time.After(time.Second):
			t.Fatalf("context for term %q not cancelled", term)
		}
	}

	close(stub.release)

	received := make([]queryResultMsg, 0, 3)
	timeout := time.After(2 * time.Second)
	for len(received) < 3 {
		select {
		case msg := <-msgCh:
			qrm, ok := msg.(queryResultMsg)
			if !ok {
				t.Fatalf("unexpected msg type %T", msg)
			}
			received = append(received, qrm)
		case <-timeout:
			t.Fatalf("only received %d/3 msgs", len(received))
		}
	}

	m := Model{queryGen: 3}
	for _, msg := range received {
		next, _ := m.Update(msg)
		m = next.(Model)
	}

	if len(m.results) != 1 || m.results[0].NodeID != "abc" {
		t.Fatalf("expected only gen-3 result accepted, got %+v", m.results)
	}
	if m.queryCancel != nil {
		t.Fatal("expected queryCancel nil after accepting gen-3 msg")
	}
}

func TestBuildOpts(t *testing.T) {
	cases := []struct {
		name       string
		term       string
		typeVal    string
		trackedVal string
		modeVal    string
		want       search.QueryOptions
	}{
		{"empty all defaults", "", "All", "All", "hybrid", search.QueryOptions{Term: "", Mode: search.QueryModeHybrid}},
		{"term only", "foo", "All", "All", "hybrid", search.QueryOptions{Term: "foo", Mode: search.QueryModeHybrid}},
		{"type set", "foo", "task", "All", "hybrid", search.QueryOptions{Term: "foo", Type: "task", Mode: search.QueryModeHybrid}},
		{"tracked only", "foo", "All", "Tracked only", "hybrid", search.QueryOptions{Term: "foo", Tracked: true, Mode: search.QueryModeHybrid}},
		{"untracked only", "foo", "All", "Untracked only", "hybrid", search.QueryOptions{Term: "foo", Untracked: true, Mode: search.QueryModeHybrid}},
		{"mode structural", "foo", "All", "All", "structural", search.QueryOptions{Term: "foo", Mode: search.QueryModeStructural}},
		{"mode explicit", "foo", "All", "All", "explicit", search.QueryOptions{Term: "foo", Mode: search.QueryModeExplicit}},
		{"mode semantic", "foo", "All", "All", "semantic", search.QueryOptions{Term: "foo", Mode: search.QueryModeSemantic}},
		{"all three set", "foo", "intent", "Untracked only", "semantic", search.QueryOptions{Term: "foo", Type: "intent", Untracked: true, Mode: search.QueryModeSemantic}},
		{"whitespace term", "   ", "All", "All", "hybrid", search.QueryOptions{Term: "   ", Mode: search.QueryModeHybrid}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				search: newTestInput(tc.term),
				chips: chipBar{
					Type:    newTestChip("Type", []string{"All", "project", "festival", "task", "intent"}, tc.typeVal),
					Tracked: newTestChip("Tracked", []string{"All", "Tracked only", "Untracked only"}, tc.trackedVal),
					Mode:    newTestChip("Mode", []string{"hybrid", "structural", "explicit", "semantic"}, tc.modeVal),
				},
			}
			got := buildOpts(m)
			if got != tc.want {
				t.Fatalf("buildOpts=%+v want %+v", got, tc.want)
			}
		})
	}
}

func newTestChip(label string, options []string, selected string) chips.Chip {
	c := chips.NewChip(label, options)
	for i, opt := range options {
		if opt == selected {
			c.SetSelected(i)
			break
		}
	}
	return c
}

func TestGroupByType(t *testing.T) {
	in := []search.QueryResult{
		{NodeID: "t1", NodeType: "task", Title: "Task One"},
		{NodeID: "p1", NodeType: "project", Title: "Proj One"},
		{NodeID: "t2", NodeType: "task", Title: "Task Two"},
		{NodeID: "f1", NodeType: "festival", Title: "Fest One"},
	}
	got := groupByType(in)
	wantOrder := []string{"project", "festival", "task"}
	if len(got) != len(wantOrder) {
		t.Fatalf("group count=%d want %d", len(got), len(wantOrder))
	}
	for i, w := range wantOrder {
		if got[i].Type != w {
			t.Fatalf("group[%d].Type=%s want %s", i, got[i].Type, w)
		}
		if !got[i].Expanded {
			t.Fatalf("group[%d] not expanded by default", i)
		}
	}
	if got[2].Rows[0].NodeID != "t1" || got[2].Rows[1].NodeID != "t2" {
		t.Fatalf("task group row order mangled: %+v", got[2].Rows)
	}
}

func TestGroupByTypeSingleType(t *testing.T) {
	in := []search.QueryResult{
		{NodeID: "f1", NodeType: "file", Title: "A"},
		{NodeID: "f2", NodeType: "file", Title: "B"},
	}
	got := groupByType(in)
	if len(got) != 1 {
		t.Fatalf("group count=%d want 1", len(got))
	}
	if got[0].Type != "file" {
		t.Fatalf("group[0].Type=%s want file", got[0].Type)
	}
	if len(got[0].Rows) != 2 || got[0].Rows[0].NodeID != "f1" || got[0].Rows[1].NodeID != "f2" {
		t.Fatalf("row order mangled: %+v", got[0].Rows)
	}
}

func TestGroupByTypeEmpty(t *testing.T) {
	if got := groupByType(nil); got != nil {
		t.Fatalf("groupByType(nil)=%v want nil", got)
	}
}

func TestBuildOptsEmptyTermYieldsZeroOpts(t *testing.T) {
	m := Model{search: newTestInput("")}
	got := buildOpts(m)
	want := search.QueryOptions{}
	if got != want {
		t.Fatalf("buildOpts(empty)=%+v want zero %+v", got, want)
	}
}

func TestFilterAnchors(t *testing.T) {
	anchors := []*graph.Node{
		{ID: "root", Type: graph.NodeFolder, Name: ".", Path: "."},
		{ID: "repo", Type: graph.NodeFolder, Name: "projects/camp", Path: "projects/camp"},
		{ID: "workflow", Type: graph.NodeFolder, Name: "workflow", Path: "workflow"},
		{ID: "file", Type: graph.NodeFile, Name: "a.go", Path: "projects/camp/a.go", Metadata: map[string]string{"tracked_state": "tracked"}},
		{ID: "untracked", Type: graph.NodeFile, Name: "b.go", Path: "projects/other/b.go", Metadata: map[string]string{"tracked_state": "untracked"}},
	}

	t.Run("all defaults returns input", func(t *testing.T) {
		got := filterAnchors(anchors, "", "", "")
		if len(got) != len(anchors) {
			t.Fatalf("got %d want %d", len(got), len(anchors))
		}
	})

	t.Run("type chip narrows", func(t *testing.T) {
		got := filterAnchors(anchors, string(graph.NodeFolder), "", "")
		if len(got) != 3 {
			t.Fatalf("got %d want 3 folders", len(got))
		}
	})

	t.Run("tracked only chip narrows", func(t *testing.T) {
		got := filterAnchors(anchors, "", "Tracked only", "")
		if len(got) != 1 || got[0].ID != "file" {
			t.Fatalf("got %+v want [file]", got)
		}
	})

	t.Run("untracked only chip narrows", func(t *testing.T) {
		got := filterAnchors(anchors, "", "Untracked only", "")
		if len(got) != 1 || got[0].ID != "untracked" {
			t.Fatalf("got %+v want [untracked]", got)
		}
	})

	t.Run("scope prefix narrows", func(t *testing.T) {
		got := filterAnchors(anchors, "", "", "projects/camp")
		if len(got) != 2 {
			t.Fatalf("got %d want 2", len(got))
		}
		for _, n := range got {
			if n.Path != "projects/camp" && n.Path != "projects/camp/a.go" {
				t.Fatalf("unexpected node in scope result: %s", n.Path)
			}
		}
	})
}

func newTestInput(term string) textinput.Model {
	ti := textinput.New()
	ti.SetValue(term)
	return ti
}
