package tui

import (
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// TestBuildOptsCrossPermutations locks in the full QueryOptions
// mapping per TUI_CONTRACT.md at the extremes of chip + scope +
// term state. Independent from TestBuildOpts in query_test.go to
// guard against regressions when the per-field mapping rules change.
func TestBuildOptsCrossPermutations(t *testing.T) {
	cases := []struct {
		name   string
		model  Model
		expect search.QueryOptions
	}{
		{
			name: "everything set",
			model: Model{
				search: newTestInput("foo"),
				scope:  "projects/camp",
				chips: chipBar{
					Type:    newTestChip("Type", []string{"All", "project", "task"}, "task"),
					Tracked: newTestChip("Tracked", []string{"All", "Tracked only", "Untracked only"}, "Tracked only"),
					Mode:    newTestChip("Mode", []string{"hybrid", "structural", "explicit", "semantic"}, "explicit"),
				},
			},
			expect: search.QueryOptions{
				Term:    "foo",
				Type:    "task",
				Tracked: true,
				Mode:    search.QueryModeExplicit,
				Scope:   "projects/camp",
			},
		},
		{
			name: "untracked + semantic + scope no term",
			model: Model{
				search: newTestInput(""),
				scope:  "workflow",
				chips: chipBar{
					Type:    newTestChip("Type", []string{"All", "note"}, "All"),
					Tracked: newTestChip("Tracked", []string{"All", "Tracked only", "Untracked only"}, "Untracked only"),
					Mode:    newTestChip("Mode", []string{"hybrid", "semantic"}, "semantic"),
				},
			},
			expect: search.QueryOptions{
				Untracked: true,
				Mode:      search.QueryModeSemantic,
				Scope:     "workflow",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildOpts(tc.model)
			if got != tc.expect {
				t.Fatalf("buildOpts=%+v want %+v", got, tc.expect)
			}
		})
	}
}
