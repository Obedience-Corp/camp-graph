//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// TestContentExtraction_ObsidianVaultBuild exercises the full workspace
// content extraction path (notes, frontmatter, links, tags, canvas) on
// a realistic Obsidian-style vault to prove end-to-end build stability.
func TestContentExtraction_ObsidianVaultBuild(t *testing.T) {
	tc := GetSharedContainer(t)

	canvasBody := `{
  "nodes": [
    {"id":"n1", "type":"file", "file":"Notes/Plan.md"},
    {"id":"n2", "type":"file", "file":"Work/OKRs.md"},
    {"id":"n3", "type":"text", "text":"just a comment"}
  ]
}`

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep": "",
				"Notes/Plan.md": "---\ntitle: Master Plan\naliases: [plan, roadmap]\ntags: [planning, ok-2026]\n---\n\n# Plan\n" +
					"Depends on [[OKRs]]. See [background](background.md).\n" +
					"Tagged inline: #strategy.\n",
				"Notes/background.md": "---\ntitle: Background\n---\n\n# bg\nBody.\n",
				"Work/OKRs.md":        "---\ntitle: OKRs\ntags: [ok-2026]\n---\n\n# Q1 OKRs\nSee [[Plan]].\n",
				"Canvas/strategy.canvas": canvasBody,
				"Assets/diagram.png":     "PNG\n",
				"Notes/with-image.md":    "![diagram](../Assets/diagram.png)\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on obsidian vault: %v\noutput: %s", err, out)
	}
	if ok, err := tc.CheckFileExists("/campaign/.campaign/graph.db"); err != nil || !ok {
		t.Fatalf("graph.db missing: ok=%v err=%v", ok, err)
	}

	// Query for a note to ensure the new note layer is queryable.
	planOut, _ := tc.RunGraphInDir("/campaign", "query", "Plan", "--type", "note")
	if !strings.Contains(planOut, "Plan") {
		t.Errorf("expected note query to surface Plan; got: %s", planOut)
	}
}

// TestContentExtraction_MalformedFrontmatterNoBuildFailure asserts that
// corrupt frontmatter in user-authored notes does not crash the build.
func TestContentExtraction_MalformedFrontmatterNoBuildFailure(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep": "",
				"Notes/broken.md":             "---\ntitle: [unterminated\n\n# body\n",
				"Notes/empty-delim.md":        "---\n---\n# body\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	out, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed on malformed frontmatter: %v\noutput: %s", err, out)
	}
}
