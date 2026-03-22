//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

// setupCampaignLayout creates a minimal campaign directory structure in the container.
func setupCampaignLayout(t *testing.T, tc *TestContainer) {
	t.Helper()
	root := "/campaign"

	dirs := []string{
		root + "/projects/alpha",
		root + "/projects/beta",
		root + "/festivals/active/test-fest-TF0001/001_BUILD/01_core",
		root + "/festivals/planning/plan-fest-PF0001",
		root + "/festivals/dungeon/completed/done-fest-DF0001",
		root + "/.campaign/intents/inbox",
		root + "/.campaign/intents/active",
		root + "/.campaign/intents/ready",
		root + "/.campaign/intents/dungeon/archived",
		root + "/.campaign/intents/dungeon/done",
		root + "/workflow/design/my-design",
		root + "/workflow/explore/my-explore",
	}

	for _, d := range dirs {
		if err := tc.MkdirAll(d); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Festival task files
	for _, name := range []string{"01_task_one.md", "02_task_two.md", "SEQUENCE_GOAL.md"} {
		if err := tc.WriteFile(root+"/festivals/active/test-fest-TF0001/001_BUILD/01_core/"+name, "# "+name); err != nil {
			t.Fatalf("failed to write task file: %v", err)
		}
	}

	// fest.yaml for metadata extraction
	if err := tc.WriteFile(root+"/festivals/active/test-fest-TF0001/fest.yaml", "project_path: projects/alpha"); err != nil {
		t.Fatalf("failed to write fest.yaml: %v", err)
	}

	// Intent files across lifecycle directories
	if err := tc.WriteFile(root+"/.campaign/intents/inbox/my-idea.md", "---\ntitle: My Idea\nstatus: inbox\n---\n# My Idea\n"); err != nil {
		t.Fatalf("failed to write inbox intent: %v", err)
	}
	if err := tc.WriteFile(root+"/.campaign/intents/active/in-progress.md", "---\ntitle: In Progress\nstatus: active\n---\n# In Progress\n"); err != nil {
		t.Fatalf("failed to write active intent: %v", err)
	}
	if err := tc.WriteFile(root+"/.campaign/intents/ready/ready-to-go.md", "---\ntitle: Ready To Go\nstatus: ready\n---\n# Ready To Go\n"); err != nil {
		t.Fatalf("failed to write ready intent: %v", err)
	}
	if err := tc.WriteFile(root+"/.campaign/intents/dungeon/archived/old-idea.md", "---\ntitle: Old Idea\nstatus: archived\n---\n# Old Idea\n"); err != nil {
		t.Fatalf("failed to write archived intent: %v", err)
	}
	if err := tc.WriteFile(root+"/.campaign/intents/dungeon/done/finished.md", "---\ntitle: Finished\nstatus: done\n---\n# Finished\n"); err != nil {
		t.Fatalf("failed to write done intent: %v", err)
	}
}

func TestBuildGraph(t *testing.T) {
	tc := GetSharedContainer(t)
	setupCampaignLayout(t, tc)

	// Build the graph database
	output, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build command failed: %v\noutput: %s", err, output)
	}

	// Verify the database was created
	exists, err := tc.CheckFileExists("/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("failed to check graph.db: %v", err)
	}
	if !exists {
		t.Fatal("graph.db was not created")
	}
}

func TestQueryIntents(t *testing.T) {
	tc := GetSharedContainer(t)
	setupCampaignLayout(t, tc)

	// Build first
	_, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Query for intents
	output, err := tc.RunGraphInDir("/campaign", "query", "my-idea", "--type", "intent")
	if err != nil {
		// Query might return non-zero if no results - check output
		t.Logf("query output: %s", output)
	}

	// Verify intents are discoverable
	if !strings.Contains(output, "my-idea") {
		t.Errorf("expected query to find 'my-idea' intent, got: %s", output)
	}
}

func TestIntentLifecycleStatuses(t *testing.T) {
	tc := GetSharedContainer(t)
	root := "/campaign"

	// projects/ is required for camp-graph to recognize a campaign
	if err := tc.MkdirAll(root + "/projects/placeholder"); err != nil {
		t.Fatal(err)
	}

	// Create intents in every lifecycle state
	if err := tc.MkdirAll(root + "/.campaign/intents/inbox"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/active"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/ready"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/dungeon/archived"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/dungeon/done"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/dungeon/someday"); err != nil {
		t.Fatal(err)
	}

	intents := []struct {
		path   string
		status string
	}{
		{root + "/.campaign/intents/inbox/new-idea.md", "inbox"},
		{root + "/.campaign/intents/active/wip.md", "active"},
		{root + "/.campaign/intents/ready/promoted.md", "ready"},
		{root + "/.campaign/intents/dungeon/archived/shelved.md", "archived"},
		{root + "/.campaign/intents/dungeon/done/completed.md", "done"},
		{root + "/.campaign/intents/dungeon/someday/maybe.md", "someday"},
	}

	for _, intent := range intents {
		content := "---\ntitle: test\nstatus: " + intent.status + "\n---\n# Test\n"
		if err := tc.WriteFile(intent.path, content); err != nil {
			t.Fatalf("failed to write intent %s: %v", intent.path, err)
		}
	}

	// Build and verify all intents are discovered
	_, err := tc.RunGraphInDir(root, "build", "--output", root+"/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// The build succeeding with 6 intent files across all lifecycle dirs
	// is the primary validation. Query each to confirm presence.
	for _, intent := range intents {
		name := strings.TrimSuffix(strings.Split(intent.path, "/")[len(strings.Split(intent.path, "/"))-1], ".md")
		output, _ := tc.RunGraphInDir(root, "query", name)
		if !strings.Contains(output, name) {
			t.Errorf("intent %q (status=%s) not found in graph query output: %s", name, intent.status, output)
		}
	}
}

func TestIntentMetadataEdges(t *testing.T) {
	tc := GetSharedContainer(t)
	root := "/campaign"

	// Create a project that the intent references
	if err := tc.MkdirAll(root + "/projects/source-proj"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/.campaign/intents/inbox"); err != nil {
		t.Fatal(err)
	}

	// Create intent with frontmatter referencing the project
	intentContent := "---\ngathered_from:\n  - source-proj\nrelated_projects:\n  - source-proj\n---\n# Intent with metadata\n"
	if err := tc.WriteFile(root+"/.campaign/intents/inbox/linked-intent.md", intentContent); err != nil {
		t.Fatal(err)
	}

	// Build graph
	_, err := tc.RunGraphInDir(root, "build", "--output", root+"/.campaign/graph.db")
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Use context command to verify edges exist
	output, _ := tc.RunGraphInDir(root, "context", "intent:linked-intent")
	t.Logf("context output: %s", output)

	// The intent should have edges to source-proj
	if !strings.Contains(output, "source-proj") {
		t.Errorf("expected context to show relationship to source-proj, got: %s", output)
	}
}

func TestMinimalCampaignBuild(t *testing.T) {
	tc := GetSharedContainer(t)

	// Minimal campaign needs .campaign/ marker and projects/ to be recognized
	if err := tc.MkdirAll("/campaign/.campaign"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll("/campaign/projects"); err != nil {
		t.Fatal(err)
	}

	output, err := tc.RunGraphInDir("/campaign", "build", "--output", "/campaign/graph.db")
	if err != nil {
		t.Fatalf("build on minimal campaign failed: %v\noutput: %s", err, output)
	}
}

func TestNoIntentsDirectory(t *testing.T) {
	tc := GetSharedContainer(t)
	root := "/campaign"

	// Create .campaign/ marker and projects, but no .campaign/intents/
	if err := tc.MkdirAll(root + "/.campaign"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll(root + "/projects/my-proj"); err != nil {
		t.Fatal(err)
	}

	_, err := tc.RunGraphInDir(root, "build", "--output", root+"/graph.db")
	if err != nil {
		t.Fatalf("build without intents directory should succeed: %v", err)
	}
}
