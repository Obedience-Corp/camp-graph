//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestInventoryHarness_TrackedUntrackedIgnored verifies that the fixture
// harness produces a git repo with the expected tracked, untracked, and
// ignored classifications.
func TestInventoryHarness_TrackedUntrackedIgnored(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/alpha/README.md": "# alpha\n",
			},
			UntrackedFiles: map[string]string{
				"workflow/design/new-note.md": "# new\n",
			},
			IgnoredPatterns: []string{"bin/", "*.log"},
			IgnoredFiles: map[string]string{
				"bin/camp-graph": "binary\n",
				"debug.log":      "oops\n",
			},
		},
	}

	tc.SetupRepoFixtures(t, specs)

	summary := tc.RepoFixtureSummary(t, "/campaign")

	checks := []struct {
		label  string
		needle string
		want   bool
	}{
		{"tracked README", "projects/alpha/README.md", true},
		{"untracked note", "workflow/design/new-note.md", true},
		{"ignored binary", "bin/camp-graph", true},
		{"ignored log", "debug.log", true},
	}
	for _, c := range checks {
		if strings.Contains(summary, c.needle) != c.want {
			t.Errorf("%s: expected presence=%v in summary\n%s", c.label, c.want, summary)
		}
	}

	// Verify the section placement: tracked appears before untracked,
	// untracked before ignored.
	trackedIdx := strings.Index(summary, "tracked:")
	untrackedIdx := strings.Index(summary, "untracked:")
	ignoredIdx := strings.Index(summary, "ignored:")
	if !(trackedIdx < untrackedIdx && untrackedIdx < ignoredIdx) {
		t.Fatalf("summary section ordering wrong; summary=%s", summary)
	}
	// README.md should be in the tracked section.
	trackedSection := summary[trackedIdx:untrackedIdx]
	if !strings.Contains(trackedSection, "projects/alpha/README.md") {
		t.Errorf("README.md missing from tracked section: %q", trackedSection)
	}
	// new-note.md should be in the untracked section.
	untrackedSection := summary[untrackedIdx:ignoredIdx]
	if !strings.Contains(untrackedSection, "workflow/design/new-note.md") {
		t.Errorf("new-note.md missing from untracked section: %q", untrackedSection)
	}
	// bin/camp-graph should be in the ignored section.
	ignoredSection := summary[ignoredIdx:]
	if !strings.Contains(ignoredSection, "bin/camp-graph") {
		t.Errorf("bin/camp-graph missing from ignored section: %q", ignoredSection)
	}
}

// TestInventoryHarness_NestedRepo verifies the harness produces a
// campaign root with a nested non-submodule repo.
func TestInventoryHarness_NestedRepo(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"workflow/root.md": "# root\n",
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

	// Campaign root has its own .git.
	if ok, err := tc.CheckDirExists("/campaign/.git"); err != nil || !ok {
		t.Fatalf("campaign .git missing: ok=%v err=%v", ok, err)
	}
	// Nested repo has its own .git.
	if ok, err := tc.CheckDirExists("/campaign/vendor/third-party/.git"); err != nil || !ok {
		t.Fatalf("nested .git missing: ok=%v err=%v", ok, err)
	}
	// Campaign has no .gitmodules because nested is not registered.
	if ok, _ := tc.CheckFileExists("/campaign/.gitmodules"); ok {
		t.Error("campaign has .gitmodules but nested repo should be standalone, not a submodule")
	}
}

// TestInventoryHarness_Submodule verifies the harness produces a
// campaign root with a registered submodule (entry in .gitmodules).
func TestInventoryHarness_Submodule(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"README.md": "# root\n",
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

	// .gitmodules must exist and reference projects/camp.
	ok, err := tc.CheckFileExists("/campaign/.gitmodules")
	if err != nil {
		t.Fatalf("check .gitmodules: %v", err)
	}
	if !ok {
		t.Fatal(".gitmodules missing")
	}
	out, code, err := tc.ExecCommand("cat", "/campaign/.gitmodules")
	if err != nil || code != 0 {
		t.Fatalf("cat .gitmodules: %v (exit=%d)", err, code)
	}
	if !strings.Contains(out, "path = projects/camp") {
		t.Errorf(".gitmodules missing submodule entry; got:\n%s", out)
	}
}
