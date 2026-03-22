package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCampRoot_EnvVar(t *testing.T) {
	root := t.TempDir()
	// Must have .campaign/ for the env var to be accepted.
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAMP_ROOT", root)

	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("command with CAMP_ROOT failed: %v", err)
	}
}

func TestCampRoot_InvalidEnvVar(t *testing.T) {
	t.Setenv("CAMP_ROOT", "/nonexistent/path")

	rootCmd.SetArgs([]string{"build"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid CAMP_ROOT")
	}
}

func TestCampRoot_WalksUp(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "projects", "myapp", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	// Unset CAMP_ROOT, change to nested dir so walk-up kicks in.
	t.Setenv("CAMP_ROOT", "")
	origDir, _ := os.Getwd()
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("walk-up detection failed: %v", err)
	}
}

// setupTestCampaign creates a minimal campaign layout in a temp directory.
func setupTestCampaign(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"projects/alpha",
		"projects/beta",
		"festivals/active/test-fest-TF0001/001_IMPLEMENT/01_seq",
		".campaign/intents/inbox",
		"workflow/design/test-design",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	festYAML := `project_path: projects/alpha`
	if err := os.WriteFile(
		filepath.Join(root, "festivals/active/test-fest-TF0001/fest.yaml"),
		[]byte(festYAML), 0o644,
	); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "festivals/active/test-fest-TF0001/001_IMPLEMENT/01_seq/01_task.md"),
		[]byte("# Task"), 0o644,
	); err != nil {
		t.Fatalf("write task: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(root, ".campaign/intents/inbox/test-intent.md"),
		[]byte("# Test Intent\n"), 0o644,
	); err != nil {
		t.Fatalf("write intent: %v", err)
	}

	return root
}

func TestBuildCommand(t *testing.T) {
	root := setupTestCampaign(t)
	dbPath := filepath.Join(root, ".campaign", "graph.db")

	t.Setenv("CAMP_ROOT", root)

	rootCmd.SetArgs([]string{"build", "--output", dbPath})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("build command: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("graph.db was not created")
	}
}

func TestQueryCommand(t *testing.T) {
	t.Skip("stub: implement after build+query integration is wired")
}

func TestContextCommand(t *testing.T) {
	t.Skip("stub: implement after build+context integration is wired")
}

func TestSanitizeNodeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"project:camp", "project-camp"},
		{"festival:test/sub", "festival-test-sub"},
		{"node with spaces", "node-with-spaces"},
		{"../../etc/passwd", "----etc-passwd"},
		{"simple", "simple"},
		{"a:b/c d..e", "a-b-c-d-e"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeNodeID(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeNodeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		Verbose:  true,
		CampRoot: "/test/campaign",
	}

	if !cfg.Verbose {
		t.Error("Config.Verbose: expected true")
	}
	if cfg.CampRoot != "/test/campaign" {
		t.Errorf("Config.CampRoot: got %q, want %q", cfg.CampRoot, "/test/campaign")
	}
}
