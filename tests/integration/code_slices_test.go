//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestCodeSlices_BoundedToNestedRepos proves that build discovers
// NodeFile/NodePackage entries inside a nested repo, while identical
// source files at the campaign root are NOT promoted to code nodes.
func TestCodeSlices_BoundedToNestedRepos(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep": "",
				"tooling/helper.go":           "package tooling\n\nfunc H(){}\n",
			},
		},
		{
			Path: "/campaign/projects/subrepo",
			TrackedFiles: map[string]string{
				"cmd/main.go":            "package main\nfunc main(){}\n",
				"internal/util/util.go":  "package util\nfunc Helper(){}\n",
				"README.md":              "# readme\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Query for a file inside the nested repo; it should be
	// discoverable.
	out, _ := tc.RunGraphInDir("/campaign", "query", "util.go", "--type", "file", "--json", "--limit", "5")
	if !strings.Contains(out, "projects/subrepo/internal/util/util.go") {
		t.Errorf("expected util.go in query output; got: %s", out)
	}

	// Campaign-root Go file (tooling/helper.go) should NOT appear as a
	// file node because code extraction is scoped to nested repos.
	out, _ = tc.RunGraphInDir("/campaign", "query", "tooling", "--type", "file", "--json", "--limit", "5")
	if strings.Contains(out, "file:tooling/helper.go") {
		t.Errorf("campaign-root tooling/helper.go leaked as NodeFile: %s", out)
	}
}
