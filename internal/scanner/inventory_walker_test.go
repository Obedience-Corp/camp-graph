package scanner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

// buildTrackedUntrackedIgnoredFixture creates a campaign root with a single
// boundary (the root itself) that mixes tracked, untracked, and ignored
// files. The classification is supplied via a StaticGitProbe so the test
// does not need the git binary.
func buildTrackedUntrackedIgnoredFixture(t *testing.T) (root string, probe *scanner.StaticGitProbe) {
	t.Helper()
	root = resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))

	// Tracked file
	writeFile(t, filepath.Join(root, "projects/alpha/README.md"), "# alpha\n")
	// Untracked file (authored)
	writeFile(t, filepath.Join(root, "workflow/design/new-note.md"), "# new\n")
	// Ignored file
	writeFile(t, filepath.Join(root, "bin/camp-graph"), "binary\n")
	// Ignored directory (node_modules style) with content inside
	writeFile(t, filepath.Join(root, ".gocache/object/x.bin"), "cached\n")

	cls := &scanner.GitClassification{
		Tracked: map[string]bool{
			"projects/alpha/README.md": true,
		},
		Untracked: map[string]bool{
			"workflow/design/new-note.md": true,
		},
		Ignored: map[string]bool{
			"bin/camp-graph":         true,
			".gocache/object/x.bin":  true,
		},
	}

	probe = &scanner.StaticGitProbe{
		ByRepo: map[string]*scanner.GitClassification{root: cls},
	}
	return root, probe
}

func TestBuildInventory_TrackedAndUntrackedIncluded(t *testing.T) {
	root, probe := buildTrackedUntrackedIgnoredFixture(t)

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe: probe,
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	states := map[string]scanner.GitState{}
	for _, e := range inv.Entries {
		if e.IsDir {
			continue
		}
		states[e.RelPath] = e.GitState
	}

	wantTracked := "projects/alpha/README.md"
	if got, ok := states[wantTracked]; !ok || got != scanner.GitStateTracked {
		t.Errorf("tracked file: got %v (present=%v), want %v", got, ok, scanner.GitStateTracked)
	}

	wantUntracked := "workflow/design/new-note.md"
	if got, ok := states[wantUntracked]; !ok || got != scanner.GitStateUntracked {
		t.Errorf("untracked file: got %v (present=%v), want %v", got, ok, scanner.GitStateUntracked)
	}
}

func TestBuildInventory_IgnoredExcludedByDefault(t *testing.T) {
	root, probe := buildTrackedUntrackedIgnoredFixture(t)

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe: probe,
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	for _, e := range inv.Entries {
		if e.IsIgnored {
			t.Errorf("default walk emitted ignored entry %q (state=%v)", e.RelPath, e.GitState)
		}
		// The .gocache tree should not appear at all because its dir is
		// ignored.
		if e.RelPath == ".gocache" || e.RelPath == ".gocache/object/x.bin" {
			t.Errorf("expected %q to be skipped; got entry in inventory", e.RelPath)
		}
		// The ignored file should not appear.
		if e.RelPath == "bin/camp-graph" {
			t.Errorf("expected bin/camp-graph to be skipped; got entry in inventory")
		}
	}
}

func TestBuildInventory_IgnoredIncludedWhenRequested(t *testing.T) {
	root, probe := buildTrackedUntrackedIgnoredFixture(t)

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe:       probe,
		IncludeIgnored: true,
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	sawBinary := false
	sawCacheContent := false
	for _, e := range inv.Entries {
		switch e.RelPath {
		case "bin/camp-graph":
			sawBinary = true
			if e.GitState != scanner.GitStateIgnored {
				t.Errorf("bin/camp-graph GitState=%v, want ignored", e.GitState)
			}
			if !e.IsIgnored {
				t.Errorf("bin/camp-graph IsIgnored=false, want true")
			}
		case ".gocache/object/x.bin":
			sawCacheContent = true
		}
	}
	if !sawBinary {
		t.Error("expected ignored bin/camp-graph when IncludeIgnored=true")
	}
	if !sawCacheContent {
		t.Error("expected ignored cache content when IncludeIgnored=true")
	}
}

func TestBuildInventory_EntryFields(t *testing.T) {
	root, probe := buildTrackedUntrackedIgnoredFixture(t)

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe: probe,
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	var readme *scanner.InventoryEntry
	for i := range inv.Entries {
		if inv.Entries[i].RelPath == "projects/alpha/README.md" {
			readme = &inv.Entries[i]
		}
	}
	if readme == nil {
		t.Fatal("projects/alpha/README.md not in inventory")
	}
	if readme.RepoRoot != root {
		t.Errorf("RepoRoot: got %q, want %q", readme.RepoRoot, root)
	}
	if readme.RelToRepo != "projects/alpha/README.md" {
		t.Errorf("RelToRepo: got %q, want %q", readme.RelToRepo, "projects/alpha/README.md")
	}
	if readme.Extension != "md" {
		t.Errorf("Extension: got %q, want %q", readme.Extension, "md")
	}
	if readme.PathDepth != 3 {
		t.Errorf("PathDepth: got %d, want 3", readme.PathDepth)
	}
	if readme.IsDir {
		t.Errorf("IsDir: got true, want false")
	}
}

func TestBuildInventory_NestedRepoClassifiedSeparately(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "projects", "nested-repo")
	mkdirAll(t, filepath.Join(nested, ".git"))

	// File inside the nested repo. The root's classification should
	// not report this path; only the nested repo's classification does.
	writeFile(t, filepath.Join(nested, "src/main.go"), "package main\n")
	// File inside the root boundary.
	writeFile(t, filepath.Join(root, "workflow/README.md"), "# root\n")

	probe := &scanner.StaticGitProbe{
		ByRepo: map[string]*scanner.GitClassification{
			root: {
				Tracked:   map[string]bool{"workflow/README.md": true},
				Untracked: map[string]bool{},
				Ignored:   map[string]bool{},
			},
			nested: {
				// Path is relative to the NESTED repo.
				Tracked:   map[string]bool{"src/main.go": true},
				Untracked: map[string]bool{},
				Ignored:   map[string]bool{},
			},
		},
	}

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe: probe,
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	byRel := map[string]scanner.InventoryEntry{}
	for _, e := range inv.Entries {
		byRel[e.RelPath] = e
	}

	rootEntry, ok := byRel["workflow/README.md"]
	if !ok {
		t.Fatalf("root file missing; entries=%v", entryPaths(inv.Entries))
	}
	if rootEntry.RepoRoot != root {
		t.Errorf("root file RepoRoot: got %q, want %q", rootEntry.RepoRoot, root)
	}
	if rootEntry.GitState != scanner.GitStateTracked {
		t.Errorf("root file GitState: got %v, want tracked", rootEntry.GitState)
	}

	nestedEntry, ok := byRel["projects/nested-repo/src/main.go"]
	if !ok {
		t.Fatalf("nested file missing; entries=%v", entryPaths(inv.Entries))
	}
	if nestedEntry.RepoRoot != nested {
		t.Errorf("nested file RepoRoot: got %q, want %q", nestedEntry.RepoRoot, nested)
	}
	if nestedEntry.RelToRepo != "src/main.go" {
		t.Errorf("nested file RelToRepo: got %q, want %q", nestedEntry.RelToRepo, "src/main.go")
	}
	if nestedEntry.GitState != scanner.GitStateTracked {
		t.Errorf("nested file GitState: got %v, want tracked", nestedEntry.GitState)
	}
}

func TestBuildInventory_NoGitMarkerMeansUnknown(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "note.md"), "# note\n")

	inv, err := scanner.BuildInventory(context.Background(), root, scanner.InventoryOptions{
		GitProbe: &scanner.StaticGitProbe{},
	})
	if err != nil {
		t.Fatalf("BuildInventory error: %v", err)
	}

	found := false
	for _, e := range inv.Entries {
		if e.RelPath == "note.md" {
			found = true
			if e.GitState != scanner.GitStateUnknown {
				t.Errorf("GitState: got %v, want unknown (no .git at root)", e.GitState)
			}
			if e.IsIgnored {
				t.Errorf("IsIgnored=true; should be false when no git state is known")
			}
		}
	}
	if !found {
		t.Error("note.md missing from inventory")
	}
}

func TestBuildInventory_CancelledContext(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "note.md"), "# note\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.BuildInventory(ctx, root, scanner.InventoryOptions{
		GitProbe: &scanner.StaticGitProbe{},
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestScanner_InventoryEntriesThreaded(t *testing.T) {
	root, probe := buildTrackedUntrackedIgnoredFixture(t)

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: probe})
	if _, err := s.Scan(context.Background()); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	inv := s.Inventory()
	if inv == nil {
		t.Fatal("Inventory nil after Scan")
	}
	if len(inv.Entries) == 0 {
		t.Fatal("Inventory.Entries is empty; expected live worktree entries")
	}
	hasTracked := false
	hasUntracked := false
	for _, e := range inv.Entries {
		if e.GitState == scanner.GitStateTracked {
			hasTracked = true
		}
		if e.GitState == scanner.GitStateUntracked {
			hasUntracked = true
		}
	}
	if !hasTracked {
		t.Error("threaded inventory has no tracked entries")
	}
	if !hasUntracked {
		t.Error("threaded inventory has no untracked entries")
	}
}

func entryPaths(entries []scanner.InventoryEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.RelPath
	}
	return out
}
