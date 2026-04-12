package main

import "testing"

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
