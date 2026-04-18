package scanner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func TestScanner_ScopeNodes_CampaignRootAlwaysEmitted(t *testing.T) {
	root := resolvePath(t, t.TempDir())

	s := scanner.New(root)
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	n := g.Node("folder:.")
	if n == nil {
		t.Fatal("folder:. scope node missing")
	}
	if n.Type != graph.NodeFolder {
		t.Errorf("root scope type: got %v, want %v", n.Type, graph.NodeFolder)
	}
	if n.Metadata[graph.MetaScopeKind] != graph.ScopeKindCampaignRoot {
		t.Errorf("campaign root scope_kind: got %q, want %q",
			n.Metadata[graph.MetaScopeKind], graph.ScopeKindCampaignRoot)
	}
	if n.Metadata[graph.MetaPathDepth] != "0" {
		t.Errorf("campaign root path_depth: got %q, want 0", n.Metadata[graph.MetaPathDepth])
	}
}

func TestScanner_ScopeNodes_KnownBuckets(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "projects/alpha/README.md"), "# alpha\n")
	writeFile(t, filepath.Join(root, ".campaign/intents/inbox/idea.md"), "# idea\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	projects := g.Node("folder:projects")
	if projects == nil {
		t.Fatal("folder:projects scope node missing")
	}
	if projects.Metadata[graph.MetaScopeKind] != graph.ScopeKindCampaignBucket {
		t.Errorf("projects scope_kind: got %q, want %q",
			projects.Metadata[graph.MetaScopeKind], graph.ScopeKindCampaignBucket)
	}

	intents := g.Node("folder:.campaign/intents")
	if intents == nil {
		t.Fatal("folder:.campaign/intents scope node missing")
	}
	if intents.Metadata[graph.MetaScopeKind] != graph.ScopeKindCampaignBucket {
		t.Errorf(".campaign/intents scope_kind: got %q, want %q",
			intents.Metadata[graph.MetaScopeKind], graph.ScopeKindCampaignBucket)
	}
}

func TestScanner_ScopeNodes_ArtifactScopeAncestry(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "projects/alpha/README.md"), "# alpha\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	alpha := g.Node("folder:projects/alpha")
	if alpha == nil {
		t.Fatal("folder:projects/alpha scope node missing")
	}
	if alpha.Metadata[graph.MetaScopeKind] != graph.ScopeKindArtifactScope {
		t.Errorf("projects/alpha scope_kind: got %q, want %q",
			alpha.Metadata[graph.MetaScopeKind], graph.ScopeKindArtifactScope)
	}
	if alpha.Metadata[graph.MetaPathDepth] != "2" {
		t.Errorf("projects/alpha path_depth: got %q, want 2", alpha.Metadata[graph.MetaPathDepth])
	}
}

func TestScanner_ScopeNodes_StructuralContainsEdges(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "projects/alpha/README.md"), "# alpha\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	expected := []struct {
		from string
		to   string
	}{
		{"folder:.", "folder:projects"},
		{"folder:projects", "folder:projects/alpha"},
	}
	have := map[string]bool{}
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeContains {
			continue
		}
		if !strings.HasPrefix(e.FromID, "folder:") || !strings.HasPrefix(e.ToID, "folder:") {
			continue
		}
		have[e.FromID+"->"+e.ToID] = true
	}
	for _, want := range expected {
		key := want.from + "->" + want.to
		if !have[key] {
			t.Errorf("expected scope contains edge %s -> %s, not found", want.from, want.to)
		}
	}
}

func TestScanner_BridgeArtifactsToScopes(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, "projects/alpha"))
	mkdirAll(t, filepath.Join(root, "festivals/active/test-fest-TF0001"))
	writeFile(t, filepath.Join(root, ".campaign/intents/inbox/idea.md"), "# idea\n")
	writeFile(t, filepath.Join(root, "workflow/design/new-design/README.md"), "# design\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// Collect folder-to-artifact contains edges for easy assertion.
	bridgeEdges := map[string]string{} // key: artifactID -> folderID
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeContains {
			continue
		}
		if !strings.HasPrefix(e.FromID, "folder:") {
			continue
		}
		if strings.HasPrefix(e.ToID, "folder:") {
			continue
		}
		bridgeEdges[e.ToID] = e.FromID
	}

	cases := []struct {
		artifactID string
		wantScope  string
	}{
		{"project:alpha", "folder:projects"},
		{"festival:test-fest-TF0001", "folder:festivals/active"},
		{"intent:idea", "folder:.campaign/intents/inbox"},
		{"design_doc:new-design", "folder:workflow/design"},
	}
	for _, c := range cases {
		if got, ok := bridgeEdges[c.artifactID]; !ok {
			t.Errorf("artifact %q has no bridge edge; expected from %q", c.artifactID, c.wantScope)
		} else if got != c.wantScope {
			t.Errorf("artifact %q bridge: got from=%q, want from=%q", c.artifactID, got, c.wantScope)
		}
	}
}

func TestScanner_ScopeNodes_UserAuthoredFolder(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// User-authored folder with a note; outside strict artifact buckets.
	writeFile(t, filepath.Join(root, "Work/JobSearch/Action Plan.md"), "# plan\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	work := g.Node("folder:Work")
	if work == nil {
		t.Fatal("folder:Work scope node missing for user-authored folder")
	}
	if work.Metadata[graph.MetaScopeKind] != graph.ScopeKindUserScope {
		t.Errorf("Work scope_kind: got %q, want %q",
			work.Metadata[graph.MetaScopeKind], graph.ScopeKindUserScope)
	}
	jobSearch := g.Node("folder:Work/JobSearch")
	if jobSearch == nil {
		t.Fatal("folder:Work/JobSearch scope node missing for user-authored folder")
	}
	if jobSearch.Metadata[graph.MetaScopeKind] != graph.ScopeKindUserScope {
		t.Errorf("Work/JobSearch scope_kind: got %q, want %q",
			jobSearch.Metadata[graph.MetaScopeKind], graph.ScopeKindUserScope)
	}
	if jobSearch.Metadata[graph.MetaPathDepth] != "2" {
		t.Errorf("Work/JobSearch path_depth: got %q, want 2", jobSearch.Metadata[graph.MetaPathDepth])
	}
}

func TestScanner_StructuralVsInferredEdgeDistinction(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "projects/alpha/README.md"), "# alpha\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	structural := 0
	inferred := 0
	explicit := 0
	for _, e := range g.Edges() {
		switch e.Source {
		case graph.SourceStructural:
			structural++
		case graph.SourceInferred:
			inferred++
		case graph.SourceExplicit:
			explicit++
		}
	}
	if structural == 0 {
		t.Error("expected at least one structural edge from scope graph")
	}
	// The scope-foundation sequence has not yet added inferred edges.
	// Lock that in so later sequences must explicitly introduce inferred
	// edges rather than accidentally promoting structural ones.
	if inferred != 0 {
		t.Errorf("expected no inferred edges in scope-foundation graph; got %d", inferred)
	}
	// Every scope-produced contains edge must be sourced as structural
	// and confidence 1.0 to keep evidence-weighted reasoning clean.
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeContains {
			continue
		}
		if !strings.HasPrefix(e.FromID, "folder:") {
			continue
		}
		if e.Source != graph.SourceStructural {
			t.Errorf("edge %s->%s source=%q, want structural", e.FromID, e.ToID, e.Source)
		}
		if e.Confidence < 0.999 {
			t.Errorf("edge %s->%s confidence=%f, want ~1.0", e.FromID, e.ToID, e.Confidence)
		}
	}
}

func TestScanner_ScopeNodes_RepoAndSubmodule(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	// nested submodule
	sub := filepath.Join(root, "projects/camp")
	mkdirAll(t, filepath.Join(sub, ".git"))
	writeFile(t, filepath.Join(sub, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, ".gitmodules"),
		"[submodule \"camp\"]\n\tpath = projects/camp\n\turl = file://./camp\n")
	// standalone nested repo
	standalone := filepath.Join(root, "vendor/third-party")
	mkdirAll(t, filepath.Join(standalone, ".git"))

	probe := &scanner.StaticGitProbe{}
	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: probe})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	sm := g.Node("folder:projects/camp")
	if sm == nil {
		t.Fatal("folder:projects/camp missing")
	}
	if sm.Metadata[graph.MetaScopeKind] != graph.ScopeKindSubmoduleRoot {
		t.Errorf("submodule scope_kind: got %q, want %q",
			sm.Metadata[graph.MetaScopeKind], graph.ScopeKindSubmoduleRoot)
	}
	if sm.Metadata[graph.MetaIsSubmodule] != "true" {
		t.Errorf("submodule is_submodule metadata: got %q, want true", sm.Metadata[graph.MetaIsSubmodule])
	}

	vendor := g.Node("folder:vendor/third-party")
	if vendor == nil {
		t.Fatal("folder:vendor/third-party missing")
	}
	if vendor.Metadata[graph.MetaScopeKind] != graph.ScopeKindRepoRoot {
		t.Errorf("standalone scope_kind: got %q, want %q",
			vendor.Metadata[graph.MetaScopeKind], graph.ScopeKindRepoRoot)
	}
	if vendor.Metadata[graph.MetaIsSubmodule] == "true" {
		t.Errorf("standalone is_submodule: got true, want absent/false")
	}
}
