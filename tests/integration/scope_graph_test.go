//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestScopeGraph_CustomizedLayoutBuild exercises scope-graph creation on a
// campaign whose authored content lives partly outside the strict artifact
// buckets. The build must still succeed and artifact discovery must remain
// intact.
func TestScopeGraph_CustomizedLayoutBuild(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/alpha/README.md":                           "# alpha\n",
				".campaign/intents/inbox/idea.md":                    "# idea\n",
				"Work/JobSearch/Action Plan.md":                      "# plan\n",
				"Business/ShinySwap/notes.md":                        "# notes\n",
				"festivals/active/test-fest-TF0001/FESTIVAL_GOAL.md": "# goal\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on customized layout: %v\noutput: %s", err, out)
	}
	if ok, err := tc.CheckFileExists("/campaign/.campaign/graph.db"); err != nil || !ok {
		t.Fatalf("graph.db missing after build: ok=%v err=%v", ok, err)
	}

	// Baseline artifact discovery remains intact.
	alphaOut, _ := tc.RunGraphInDir("/campaign", "query", "alpha", "--type", "project")
	if !strings.Contains(alphaOut, "alpha") {
		t.Errorf("alpha project not discoverable after scope-graph integration: %s", alphaOut)
	}
	ideaOut, _ := tc.RunGraphInDir("/campaign", "query", "idea", "--type", "intent")
	if !strings.Contains(ideaOut, "idea") {
		t.Errorf("inbox intent not discoverable after scope-graph integration: %s", ideaOut)
	}
}

// TestScopeGraph_ObsidianStyleLayout models an Obsidian-vault layout where
// most authored content lives under user-authored folders rather than
// under projects/ or festivals/. The campaign still needs to build and
// artifacts should still be discoverable.
func TestScopeGraph_ObsidianStyleLayout(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				".campaign/intents/inbox/anchor.md": "# anchor\n",
				"projects/_placeholder/README.md":   "# placeholder\n",
				"Notes/Daily/2026-04-17.md":         "# today\n",
				"Notes/Topics/ai-stack.md":          "# ai stack\n",
				"Vault/Research/deep-dive.md":       "# research\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on obsidian-style layout: %v\noutput: %s", err, out)
	}
	if ok, err := tc.CheckFileExists("/campaign/.campaign/graph.db"); err != nil || !ok {
		t.Fatalf("graph.db missing after build: ok=%v err=%v", ok, err)
	}
}
