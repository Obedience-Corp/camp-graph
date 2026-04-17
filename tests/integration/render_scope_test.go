//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestRender_ScopeSlice proves that render --scope emits only the
// nodes inside the requested scope subtree. We render JSON and assert
// that cross-scope notes are absent from the output.
func TestRender_ScopeSlice(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":   "",
				"Work/JobSearch/plan.md":        "# plan\n",
				"Work/JobSearch/kickoff.md":     "# kickoff\n",
				"Business/ShinySwap/readme.md":  "# readme\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}

	out, err := tc.RunGraphInDir("/campaign", "render",
		"--scope", "Work/JobSearch",
		"--format", "json",
		"--no-save",
	)
	if err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Work/JobSearch") {
		t.Errorf("expected scope output to mention Work/JobSearch; got: %s", out)
	}
	if strings.Contains(out, "Business/ShinySwap") {
		t.Errorf("scope filter leaked Business/ShinySwap content: %s", out)
	}
}
