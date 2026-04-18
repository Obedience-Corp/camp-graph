//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestInventoryRegression_NestedRepoBuild asserts that the build command
// succeeds on a campaign that contains a nested (non-submodule) repository.
// The new inventory layer must not regress the baseline artifact-only
// discovery path.
func TestInventoryRegression_NestedRepoBuild(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/alpha/README.md":                          "# alpha\n",
				"festivals/active/test-fest-TF0001/FESTIVAL_GOAL.md": "# goal\n",
				".campaign/intents/inbox/idea.md":                    "# idea\n",
			},
		},
		{
			Path: "/campaign/vendor/third-party",
			TrackedFiles: map[string]string{
				"LICENSE": "MIT\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	// Build using the default campaign root so the inventory pass runs
	// against the live worktree state the harness produced.
	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on nested-repo campaign: %v\noutput: %s", err, out)
	}
	exists, err := tc.CheckFileExists("/campaign/.campaign/graph.db")
	if err != nil || !exists {
		t.Fatalf("graph.db not created: exists=%v err=%v", exists, err)
	}

	// Baseline artifact discovery must still work: alpha project should
	// appear when querying for projects.
	qout, err := tc.RunGraphInDir("/campaign", "query", "alpha", "--type", "project")
	if err != nil {
		t.Logf("query exit non-zero (acceptable for empty result); out=%s", qout)
	}
	if !strings.Contains(qout, "alpha") {
		t.Errorf("expected query to find alpha project after inventory layer; got: %s", qout)
	}
}

// TestInventoryRegression_SubmoduleBuild asserts that build succeeds when
// a campaign includes a registered submodule. The inventory pass should
// detect the submodule as a boundary and leave artifact discovery intact.
func TestInventoryRegression_SubmoduleBuild(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/placeholder/.keep":       "",
				".campaign/intents/inbox/seed.md":  "# seed\n",
			},
		},
		{
			Path: "/campaign/projects/camp",
			TrackedFiles: map[string]string{
				"main.go": "package main\n",
			},
			SubmodulePath: "projects/camp",
			ParentPath:    "/campaign",
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on submodule campaign: %v\noutput: %s", err, out)
	}
	if ok, err := tc.CheckFileExists("/campaign/.campaign/graph.db"); err != nil || !ok {
		t.Fatalf("graph.db not created: ok=%v err=%v", ok, err)
	}
}

// TestInventoryRegression_IgnoredContentExcluded asserts that build
// succeeds when the campaign's worktree contains ignored directories
// (common for local caches). The inventory walker must not descend into
// them and produce artifact-mode regressions.
func TestInventoryRegression_IgnoredContentExcluded(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/alpha/README.md":        "# alpha\n",
				".campaign/intents/inbox/idea.md": "# idea\n",
			},
			IgnoredPatterns: []string{"bin/", ".gocache/", "out/"},
			IgnoredFiles: map[string]string{
				"bin/large-binary":    strings.Repeat("x", 4096),
				".gocache/object/foo": "cached\n",
				"out/report.txt":      "stale\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed with ignored content: %v\noutput: %s", err, out)
	}
	if ok, err := tc.CheckFileExists("/campaign/.campaign/graph.db"); err != nil || !ok {
		t.Fatalf("graph.db not created: ok=%v err=%v", ok, err)
	}
	// Sanity-check that artifact discovery continues to work; the alpha
	// project should still be queryable.
	qout, _ := tc.RunGraphInDir("/campaign", "query", "alpha", "--type", "project")
	if !strings.Contains(qout, "alpha") {
		t.Errorf("alpha project not discoverable after ignored-content run: %s", qout)
	}
}

// TestInventoryRegression_UntrackedAuthoredContent asserts that build
// succeeds and discovers artifacts even when meaningful campaign content
// is untracked (still in the live worktree, not yet committed).
// This locks down the design goal of preserving authored untracked files.
func TestInventoryRegression_UntrackedAuthoredContent(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/alpha/README.md": "# alpha\n",
			},
			UntrackedFiles: map[string]string{
				"festivals/active/new-fest-NF0001/FESTIVAL_GOAL.md": "# new festival\n",
				".campaign/intents/inbox/fresh.md":                  "# fresh\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed: %v\noutput: %s", err, out)
	}

	// Untracked artifacts must still be discoverable because the inventory
	// walker keeps authored untracked content by default.
	qout, _ := tc.RunGraphInDir("/campaign", "query", "new-fest", "--type", "festival")
	if !strings.Contains(qout, "new-fest") {
		t.Errorf("expected untracked festival to be discoverable; got: %s", qout)
	}
}
