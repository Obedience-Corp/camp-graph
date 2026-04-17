//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestReleaseE2E_BuildQueryRefreshStatusRelated runs the full release
// contract against a single campaign fixture: build creates a graph
// DB, query returns content-backed hits, refresh updates changed
// files, status reports fresh state, and related returns scope-local
// items.
func TestReleaseE2E_BuildQueryRefreshStatusRelated(t *testing.T) {
	tc := GetSharedContainer(t)

	specs := []RepoSpec{
		{
			Path: "/campaign",
			TrackedFiles: map[string]string{
				"projects/_placeholder/.keep":                     "",
				".campaign/intents/inbox/idea.md":                 "# idea\n",
				"festivals/active/release-fest/FESTIVAL_GOAL.md":  "# goal\n",
				"Work/JobSearch/Action Plan.md":                   "# Action Plan\n\nRoadmap for my job search.\n",
				"Work/JobSearch/Kickoff.md":                       "# Kickoff\n\nFirst week.\n",
				"Business/ShinySwap/notes.md":                     "# Shiny notes\n",
			},
		},
	}
	tc.SetupRepoFixtures(t, specs)

	// 1) build
	out, err := tc.RunGraphInDir("/campaign", "build")
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// 2) query --json for content-backed match
	qout, err := tc.RunGraphInDir("/campaign", "query", "job search", "--json", "--limit", "5")
	if err != nil {
		t.Fatalf("query: %v\n%s", err, qout)
	}
	if !strings.Contains(qout, "Work/JobSearch/Action Plan.md") {
		t.Errorf("expected Action Plan.md in query results; got: %s", qout)
	}

	// 3) refresh --json
	rout, err := tc.RunGraphInDir("/campaign", "refresh", "--json")
	if err != nil {
		t.Fatalf("refresh: %v\n%s", err, rout)
	}
	var refreshEnv struct {
		SchemaVersion string `json:"schema_version"`
		Mode          string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(rout), &refreshEnv); err != nil {
		t.Fatalf("parse refresh: %v\nraw=%s", err, rout)
	}
	if refreshEnv.SchemaVersion != "graph-refresh/v1alpha1" {
		t.Errorf("refresh schema: got %q", refreshEnv.SchemaVersion)
	}
	if refreshEnv.Mode == "" {
		t.Errorf("refresh mode empty")
	}

	// 4) status --json reports non-stale state
	sout, err := tc.RunGraphInDir("/campaign", "status", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, sout)
	}
	var statusEnv struct {
		SchemaVersion      string `json:"schema_version"`
		GraphSchemaVersion string `json:"graph_schema_version"`
		SearchAvailable    bool   `json:"search_available"`
		Stale              bool   `json:"stale"`
		Nodes              int    `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(sout), &statusEnv); err != nil {
		t.Fatalf("parse status: %v\nraw=%s", err, sout)
	}
	if statusEnv.Stale {
		t.Error("status stale=true after refresh; want false")
	}
	if !statusEnv.SearchAvailable {
		t.Error("search_available=false after build; want true")
	}
	if statusEnv.Nodes == 0 {
		t.Error("status reports zero nodes")
	}

	// 5) related --json for scope-local results
	relout, err := tc.RunGraphInDir("/campaign", "related",
		"--path", "Work/JobSearch/Action Plan.md",
		"--json", "--limit", "5",
	)
	if err != nil {
		t.Fatalf("related: %v\n%s", err, relout)
	}
	var relatedEnv struct {
		SchemaVersion string `json:"schema_version"`
		Items         []struct {
			Scope string `json:"scope"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(relout), &relatedEnv); err != nil {
		t.Fatalf("parse related: %v\nraw=%s", err, relout)
	}
	if relatedEnv.SchemaVersion != "graph-related/v1alpha1" {
		t.Errorf("related schema: got %q", relatedEnv.SchemaVersion)
	}
	if len(relatedEnv.Items) == 0 {
		t.Error("related returned zero items for a populated campaign")
	}
	for _, it := range relatedEnv.Items {
		if strings.HasPrefix(it.Scope, "Business/") {
			t.Errorf("related leaked Business/ scope: %+v", it)
		}
	}
}
