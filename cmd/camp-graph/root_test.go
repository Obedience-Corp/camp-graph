package main

import (
	"os"
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
