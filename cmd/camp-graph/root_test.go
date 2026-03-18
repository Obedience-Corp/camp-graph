package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCampRoot_EnvVar(t *testing.T) {
	want := "/home/user/my-campaign"
	t.Setenv("CAMP_ROOT", want)

	got, err := getCampRoot()
	if err != nil {
		t.Fatalf("getCampRoot() returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("getCampRoot() = %q, want %q", got, want)
	}
}

func TestGetCampRoot_CwdFallback(t *testing.T) {
	t.Setenv("CAMP_ROOT", "")

	expectedCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed in test setup: %v", err)
	}

	got, err := getCampRoot()
	if err != nil {
		t.Fatalf("getCampRoot() returned unexpected error: %v", err)
	}
	if got == "" {
		t.Error("getCampRoot() returned empty string on cwd fallback")
	}
	if got != expectedCwd {
		t.Errorf("getCampRoot() = %q, want cwd %q", got, expectedCwd)
	}
}

func TestGetCampRoot_Precedence(t *testing.T) {
	tests := []struct {
		name       string
		campRoot   string
		wantEnvVal bool
	}{
		{
			name:       "CAMP_ROOT set takes priority",
			campRoot:   "/explicit/campaign/root",
			wantEnvVal: true,
		},
		{
			name:       "CAMP_ROOT empty falls back to cwd",
			campRoot:   "",
			wantEnvVal: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CAMP_ROOT", tc.campRoot)

			got, err := getCampRoot()
			if err != nil {
				t.Fatalf("getCampRoot() error: %v", err)
			}

			if tc.wantEnvVal {
				if got != tc.campRoot {
					t.Errorf("getCampRoot() = %q, want env value %q", got, tc.campRoot)
				}
			} else {
				if got == "" {
					t.Error("getCampRoot() returned empty string on cwd fallback")
				}
				if got == tc.campRoot {
					t.Error("getCampRoot() returned empty CAMP_ROOT instead of cwd")
				}
			}
		})
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
