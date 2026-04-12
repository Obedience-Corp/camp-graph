//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"
)

func TestCampRootEnvVar(t *testing.T) {
	tc := GetSharedContainer(t)

	if err := tc.MkdirAll("/env-root/.campaign"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll("/test/work"); err != nil {
		t.Fatal(err)
	}

	output, err := tc.RunGraphInDirWithEnv("/test/work", map[string]string{
		"CAMP_ROOT": "/env-root",
	}, "version")
	if err != nil {
		t.Fatalf("command with CAMP_ROOT failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "camp-graph") {
		t.Fatalf("expected version output, got: %s", output)
	}
}

func TestCampRootInvalidEnvVarOutsideCampaign(t *testing.T) {
	tc := GetSharedContainer(t)

	if err := tc.MkdirAll("/test/work"); err != nil {
		t.Fatal(err)
	}

	output, err := tc.RunGraphInDirWithEnv("/test/work", map[string]string{
		"CAMP_ROOT": "/nonexistent/path",
	}, "build")
	if err == nil {
		t.Fatalf("expected error for invalid CAMP_ROOT, got output: %s", output)
	}
	if !strings.Contains(output, "determining campaign root") {
		t.Fatalf("expected campaign root error, got: %s", output)
	}
}

func TestCampRootWalksUp(t *testing.T) {
	tc := GetSharedContainer(t)

	if err := tc.MkdirAll("/campaign/.campaign"); err != nil {
		t.Fatal(err)
	}
	if err := tc.MkdirAll("/campaign/projects/myapp/src"); err != nil {
		t.Fatal(err)
	}

	output, err := tc.RunGraphInDirWithEnv("/campaign/projects/myapp/src", nil, "version")
	if err != nil {
		t.Fatalf("walk-up detection failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "camp-graph") {
		t.Fatalf("expected version output, got: %s", output)
	}
}
