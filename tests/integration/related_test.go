//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

type relatedEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	CampaignRoot  string `json:"campaign_root"`
	QueryPath     string `json:"query_path"`
	Mode          string `json:"mode"`
	Stale         bool   `json:"stale"`
	Items         []struct {
		NodeID       string  `json:"node_id"`
		NodeType     string  `json:"node_type"`
		Title        string  `json:"title"`
		RelativePath string  `json:"relative_path"`
		Scope        string  `json:"scope"`
		Reason       string  `json:"reason"`
		Score        float64 `json:"score"`
	} `json:"items"`
}

// TestRelatedJSON_ScopeAndLinkEnrichment proves that `camp-graph
// related` returns same-scope and explicit-link related items when
// both are present. The envelope must match graph-related/v1alpha1.
func TestRelatedJSON_ScopeAndLinkEnrichment(t *testing.T) {
	tc := GetSharedContainer(t)
	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":     "",
				"Work/JobSearch/Action Plan.md":   "# Action Plan\n\nSee [Kickoff](Kickoff.md) for details.\n",
				"Work/JobSearch/Kickoff.md":       "# Kickoff\n\nFirst week notes.\n",
				"Work/JobSearch/Remote.md":        "# Remote Listings\n\nBody.\n",
				"Business/ShinySwap/readme.md":    "# Shiny\n\nUnrelated.\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}

	out, err := tc.RunGraphInDir("/campaign", "related",
		"--path", "Work/JobSearch/Action Plan.md",
		"--json", "--limit", "5",
	)
	if err != nil {
		t.Fatalf("related: %v\n%s", err, out)
	}
	var env relatedEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, out)
	}

	if env.SchemaVersion != "graph-related/v1alpha1" {
		t.Errorf("schema_version: got %q, want graph-related/v1alpha1", env.SchemaVersion)
	}
	if env.QueryPath != "Work/JobSearch/Action Plan.md" {
		t.Errorf("query_path: got %q", env.QueryPath)
	}
	if len(env.Items) == 0 {
		t.Fatal("expected at least one related item")
	}

	sawSameScope := false
	sawExplicit := false
	for _, it := range env.Items {
		if it.Reason == "same_scope" {
			sawSameScope = true
			if it.Scope != "Work/JobSearch" {
				t.Errorf("same_scope item scope: got %q", it.Scope)
			}
		}
		if it.Reason == "explicit_edge" {
			sawExplicit = true
			if !strings.Contains(it.RelativePath, "Kickoff") {
				t.Errorf("explicit_edge item should be Kickoff; got %+v", it)
			}
		}
		if strings.Contains(it.RelativePath, "Business/ShinySwap") {
			t.Errorf("leaked cross-scope item: %+v", it)
		}
	}
	if !sawSameScope {
		t.Error("expected at least one same_scope item")
	}
	if !sawExplicit {
		t.Error("expected explicit_edge item for linked kickoff")
	}
}

// TestRelatedJSON_UnknownPathEmptyItems asserts that an unknown path
// returns a well-formed envelope with an empty items list. This is the
// degraded state the workitem integration relies on to ignore
// enrichment cleanly when the target has not been indexed yet.
func TestRelatedJSON_UnknownPathEmptyItems(t *testing.T) {
	tc := GetSharedContainer(t)
	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep": "",
				"Work/note.md":                "# note\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)
	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := tc.RunGraphInDir("/campaign", "related",
		"--path", "Missing/ghost.md", "--json", "--limit", "5",
	)
	if err != nil {
		t.Fatalf("related: %v\n%s", err, out)
	}
	var env relatedEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, out)
	}
	if env.SchemaVersion != "graph-related/v1alpha1" {
		t.Errorf("schema: got %q", env.SchemaVersion)
	}
	if len(env.Items) != 0 {
		t.Errorf("expected 0 items for missing path; got %d", len(env.Items))
	}
}

// TestStatusJSON_PayloadShape verifies that status emits the
// graph-status/v1alpha1 envelope with the expected fields populated
// after a build.
func TestStatusJSON_PayloadShape(t *testing.T) {
	tc := GetSharedContainer(t)
	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep": "",
				"Work/note.md":                "# note\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)
	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := tc.RunGraphInDir("/campaign", "status", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	var env struct {
		SchemaVersion      string `json:"schema_version"`
		GraphSchemaVersion string `json:"graph_schema_version"`
		SearchAvailable    bool   `json:"search_available"`
		Nodes              int    `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, out)
	}
	if env.SchemaVersion != "graph-status/v1alpha1" {
		t.Errorf("schema: got %q", env.SchemaVersion)
	}
	if env.GraphSchemaVersion != "graphdb/v2alpha1" {
		t.Errorf("graph_schema_version: got %q", env.GraphSchemaVersion)
	}
	if !env.SearchAvailable {
		t.Error("search_available should be true after build")
	}
	if env.Nodes == 0 {
		t.Error("status reports zero nodes after build")
	}
}
