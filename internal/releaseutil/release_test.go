package main

import "testing"

func TestParseStableTag(t *testing.T) {
	got, ok, err := parseStableTag("v1.2.3")
	if err != nil {
		t.Fatalf("parseStableTag returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected stable tag to parse")
	}

	want := stableVersion{Major: 1, Minor: 2, Patch: 3}
	if got != want {
		t.Fatalf("parseStableTag = %#v, want %#v", got, want)
	}
}

func TestParseStableTagRejectsPrerelease(t *testing.T) {
	_, ok, err := parseStableTag("v1.2.3-rc.1")
	if err != nil {
		t.Fatalf("parseStableTag returned error: %v", err)
	}
	if ok {
		t.Fatal("expected prerelease tag to be ignored")
	}
}

func TestNextStableTagDefaultsToFirstRelease(t *testing.T) {
	got, err := nextStableTag(nil, "patch")
	if err != nil {
		t.Fatalf("nextStableTag returned error: %v", err)
	}
	if got != "v0.1.0" {
		t.Fatalf("nextStableTag = %q, want %q", got, "v0.1.0")
	}
}

func TestNextStableTagBumpsPatch(t *testing.T) {
	got, err := nextStableTag([]stableVersion{{Major: 1, Minor: 4, Patch: 2}}, "patch")
	if err != nil {
		t.Fatalf("nextStableTag returned error: %v", err)
	}
	if got != "v1.4.3" {
		t.Fatalf("nextStableTag = %q, want %q", got, "v1.4.3")
	}
}

func TestNextStableTagBumpsMinor(t *testing.T) {
	got, err := nextStableTag([]stableVersion{{Major: 1, Minor: 4, Patch: 2}}, "minor")
	if err != nil {
		t.Fatalf("nextStableTag returned error: %v", err)
	}
	if got != "v1.5.0" {
		t.Fatalf("nextStableTag = %q, want %q", got, "v1.5.0")
	}
}

func TestNextStableTagBumpsMajor(t *testing.T) {
	got, err := nextStableTag([]stableVersion{{Major: 1, Minor: 4, Patch: 2}}, "major")
	if err != nil {
		t.Fatalf("nextStableTag returned error: %v", err)
	}
	if got != "v2.0.0" {
		t.Fatalf("nextStableTag = %q, want %q", got, "v2.0.0")
	}
}

func TestSortStableVersionsDesc(t *testing.T) {
	versions := []stableVersion{
		{Major: 1, Minor: 2, Patch: 3},
		{Major: 2, Minor: 0, Patch: 0},
		{Major: 1, Minor: 10, Patch: 0},
	}

	sortStableVersionsDesc(versions)

	want := []stableVersion{
		{Major: 2, Minor: 0, Patch: 0},
		{Major: 1, Minor: 10, Patch: 0},
		{Major: 1, Minor: 2, Patch: 3},
	}

	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("versions[%d] = %#v, want %#v", i, versions[i], want[i])
		}
	}
}

func TestValidateExplicitTag(t *testing.T) {
	if err := validateExplicitTag("v1.2.3"); err != nil {
		t.Fatalf("validateExplicitTag returned error: %v", err)
	}

	if err := validateExplicitTag("v1.2.3-rc.1"); err == nil {
		t.Fatal("expected prerelease tag to be rejected")
	}
}
