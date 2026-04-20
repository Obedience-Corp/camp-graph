package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func TestBuildOpts(t *testing.T) {
	cases := []struct {
		name string
		term string
		want search.QueryOptions
	}{
		{"empty", "", search.QueryOptions{Term: ""}},
		{"whitespace", "   ", search.QueryOptions{Term: "   "}},
		{"plain", "campaign", search.QueryOptions{Term: "campaign"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{search: newTestInput(tc.term)}
			got := buildOpts(m)
			if got != tc.want {
				t.Fatalf("buildOpts=%+v want %+v", got, tc.want)
			}
		})
	}
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
