//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

type queryEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	CampaignRoot  string `json:"campaign_root"`
	Query         string `json:"query"`
	Mode          string `json:"mode"`
	Limit         int    `json:"limit"`
	Results       []struct {
		NodeID       string   `json:"node_id"`
		NodeType     string   `json:"node_type"`
		Title        string   `json:"title"`
		RelativePath string   `json:"relative_path"`
		Scope        string   `json:"scope"`
		Snippet      string   `json:"snippet"`
		TrackedState string   `json:"tracked_state"`
		Score        float64  `json:"score"`
		Reasons      []string `json:"reasons"`
	} `json:"results"`
}

// TestQueryJSON_FindsBuriedContentBySearchTerm proves that
// camp-graph query emits the graph-query/v1alpha1 envelope and returns
// content-backed matches for a markdown note whose name does NOT
// contain the search term.
func TestQueryJSON_FindsBuriedContentBySearchTerm(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":      "",
				"Work/JobSearch/Action Plan.md":    "# Action Plan\n\nRoadmap for my job search and planning.\n",
				"Work/JobSearch/Remote Listings.md":"# Listings\n\nRemote role ideas.\n",
				"Business/ShinySwap/notes.md":      "# Shiny\n\nBody unrelated content.\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}

	out, err := tc.RunGraphInDir("/campaign", "query", "job search", "--json", "--limit", "5")
	if err != nil {
		t.Fatalf("query: %v\nout=%s", err, out)
	}
	var env queryEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("query json parse: %v\nraw=%s", err, out)
	}
	if env.SchemaVersion != "graph-query/v1alpha1" {
		t.Errorf("schema_version: got %q, want %q", env.SchemaVersion, "graph-query/v1alpha1")
	}
	if env.Mode != "hybrid" {
		t.Errorf("mode: got %q, want hybrid", env.Mode)
	}
	if env.Limit != 5 {
		t.Errorf("limit: got %d, want 5", env.Limit)
	}
	if len(env.Results) == 0 {
		t.Fatalf("expected at least 1 result; got 0\nraw=%s", out)
	}

	found := false
	for _, r := range env.Results {
		if r.RelativePath == "Work/JobSearch/Action Plan.md" {
			found = true
			if r.Scope != "Work/JobSearch" {
				t.Errorf("scope: got %q, want %q", r.Scope, "Work/JobSearch")
			}
			if r.NodeID != "note:Work/JobSearch/Action Plan.md" {
				t.Errorf("node_id: got %q, want %q", r.NodeID, "note:Work/JobSearch/Action Plan.md")
			}
			if r.NodeType != "note" {
				t.Errorf("node_type: got %q, want note", r.NodeType)
			}
			if r.Snippet == "" {
				t.Error("snippet empty; expected FTS snippet to highlight body match")
			}
			if len(r.Reasons) == 0 {
				t.Error("reasons empty; expected at least fts_match")
			}
			if !strings.Contains(strings.Join(r.Reasons, ","), "fts_match") {
				t.Errorf("reasons missing fts_match: %v", r.Reasons)
			}
		}
	}
	if !found {
		t.Errorf("buried Action Plan.md not discovered by body search; results=%+v", env.Results)
	}
}

// TestQueryJSON_ScopeFilterNarrowsResults asserts that --scope
// removes cross-scope matches.
func TestQueryJSON_ScopeFilterNarrowsResults(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":  "",
				"Work/JobSearch/plan.md":       "# plan\nBody plan.\n",
				"Business/ShinySwap/plan.md":   "# plan\nBody plan in shiny.\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)
	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := tc.RunGraphInDir("/campaign", "query", "plan",
		"--scope", "Work/JobSearch", "--json", "--limit", "10")
	if err != nil {
		t.Fatalf("query: %v\n%s", err, out)
	}
	var env queryEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("query json parse: %v\nraw=%s", err, out)
	}
	for _, r := range env.Results {
		if r.Scope != "Work/JobSearch" {
			t.Errorf("scope filter leak: got %q", r.Scope)
		}
	}
}

// TestQueryJSON_TypeFilter ensures --type narrows to a specific node
// type (e.g. intent) without including notes.
func TestQueryJSON_TypeFilter(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":  "",
				".campaign/intents/inbox/idea.md": "# idea note\n",
				"Work/idea-notes.md":              "# idea notes\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)
	if _, err := tc.RunGraphInDir("/campaign", "build"); err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := tc.RunGraphInDir("/campaign", "query", "idea", "--type", "note", "--json", "--limit", "10")
	if err != nil {
		t.Fatalf("query: %v\n%s", err, out)
	}
	var env queryEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("query json parse: %v\nraw=%s", err, out)
	}
	for _, r := range env.Results {
		if r.NodeType != "note" {
			t.Errorf("type filter leak: got %q", r.NodeType)
		}
	}
}
