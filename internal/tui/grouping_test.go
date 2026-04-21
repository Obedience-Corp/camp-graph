package tui

import (
	"reflect"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// TestGroupByTypeInterleavedPriority exercises groupByType against an
// interleaved fixture covering multiple known NodeType values plus an
// unknown type. Known types must sort by typePriority ascending;
// unknown types must appear after all known types and preserve
// first-appearance order. Within each group, BM25 order (input order)
// must be preserved.
func TestGroupByTypeInterleavedPriority(t *testing.T) {
	in := []search.QueryResult{
		{NodeID: "t1", NodeType: "task", Score: 0.9},
		{NodeID: "p1", NodeType: "project", Score: 0.85},
		{NodeID: "t2", NodeType: "task", Score: 0.8},
		{NodeID: "s1", NodeType: "sequence", Score: 0.7},
		{NodeID: "u1", NodeType: "unknown-x", Score: 0.6},
		{NodeID: "t3", NodeType: "task", Score: 0.5},
	}

	got := groupByType(in)

	wantGroups := []struct {
		Type     string
		Expanded bool
		Rows     []string
	}{
		{"project", true, []string{"p1"}},
		{"sequence", true, []string{"s1"}},
		{"task", true, []string{"t1", "t2", "t3"}},
		{"unknown-x", true, []string{"u1"}},
	}

	if len(got) != len(wantGroups) {
		t.Fatalf("got %d groups, want %d: %+v", len(got), len(wantGroups), got)
	}
	for i, want := range wantGroups {
		if got[i].Type != want.Type {
			t.Errorf("group[%d].Type=%s want %s", i, got[i].Type, want.Type)
		}
		if got[i].Expanded != want.Expanded {
			t.Errorf("group[%d].Expanded=%v want %v", i, got[i].Expanded, want.Expanded)
		}
		var gotIDs []string
		for _, r := range got[i].Rows {
			gotIDs = append(gotIDs, r.NodeID)
		}
		if !reflect.DeepEqual(gotIDs, want.Rows) {
			t.Errorf("group[%d] row IDs=%v want %v", i, gotIDs, want.Rows)
		}
	}
}

// TestGroupByTypeUnknownsPreserveOrder verifies that when every type
// is unknown, first-appearance ordering is preserved and alphabetical
// tie-break applies only when priorities are equal.
func TestGroupByTypeUnknownsPreserveOrder(t *testing.T) {
	in := []search.QueryResult{
		{NodeID: "z1", NodeType: "zebra"},
		{NodeID: "a1", NodeType: "alpha"},
		{NodeID: "m1", NodeType: "mango"},
	}
	got := groupByType(in)

	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("got %d groups want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Type != w {
			t.Errorf("group[%d].Type=%s want %s", i, got[i].Type, w)
		}
	}
}
