package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

// writeFile writes data to path, creating parents as needed.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkdirAll creates a directory and all parents.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// resolvePath resolves symlinks so path comparisons work on macOS
// (where /var -> /private/var).
func resolvePath(t *testing.T, p string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("eval symlinks %q: %v", p, err)
	}
	return out
}

func TestDiscoverBoundaries_RootOnly(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, ".git/HEAD"), "ref: refs/heads/main\n")

	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}
	if inv.Root != root {
		t.Errorf("Root: got %q, want %q", inv.Root, root)
	}
	if len(inv.Boundaries) != 1 {
		t.Fatalf("boundaries: got %d, want 1", len(inv.Boundaries))
	}
	b := inv.Boundaries[0]
	if !b.IsRoot {
		t.Errorf("root boundary IsRoot=false, want true")
	}
	if b.RelPath != "." {
		t.Errorf("root RelPath: got %q, want %q", b.RelPath, ".")
	}
	if b.GitDir == "" {
		t.Errorf("root GitDir empty; expected .git path")
	}
	if b.IsSubmodule {
		t.Errorf("root IsSubmodule=true, want false")
	}
}

func TestDiscoverBoundaries_RootWithoutGit(t *testing.T) {
	root := resolvePath(t, t.TempDir())

	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}
	if len(inv.Boundaries) != 1 {
		t.Fatalf("boundaries: got %d, want 1", len(inv.Boundaries))
	}
	b := inv.Boundaries[0]
	if !b.IsRoot {
		t.Errorf("IsRoot=false, want true")
	}
	if b.GitDir != "" {
		t.Errorf("GitDir=%q, want empty for root without .git", b.GitDir)
	}
}

func TestDiscoverBoundaries_NestedRepo(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	// Plain nested repo (no .gitmodules entry for it).
	nested := filepath.Join(root, "projects", "nested-repo")
	mkdirAll(t, filepath.Join(nested, ".git"))
	writeFile(t, filepath.Join(nested, ".git/HEAD"), "ref: refs/heads/main\n")
	// Content below the nested boundary should NOT produce additional
	// boundary entries because discovery stops at the nested boundary.
	mkdirAll(t, filepath.Join(nested, "subpkg"))
	mkdirAll(t, filepath.Join(nested, "subpkg", ".git")) // interior should be ignored

	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}
	if len(inv.Boundaries) != 2 {
		t.Fatalf("boundaries: got %d, want 2; boundaries=%+v", len(inv.Boundaries), inv.Boundaries)
	}

	var nestedBoundary *scanner.Boundary
	for i := range inv.Boundaries {
		if !inv.Boundaries[i].IsRoot {
			nestedBoundary = &inv.Boundaries[i]
		}
	}
	if nestedBoundary == nil {
		t.Fatal("nested boundary not found")
	}
	if nestedBoundary.AbsPath != nested {
		t.Errorf("nested AbsPath: got %q, want %q", nestedBoundary.AbsPath, nested)
	}
	if nestedBoundary.RelPath != "projects/nested-repo" {
		t.Errorf("nested RelPath: got %q, want %q", nestedBoundary.RelPath, "projects/nested-repo")
	}
	if nestedBoundary.IsSubmodule {
		t.Errorf("nested IsSubmodule=true, want false")
	}
	if nestedBoundary.IsRoot {
		t.Errorf("nested IsRoot=true, want false")
	}
	if nestedBoundary.GitDir == "" {
		t.Errorf("nested GitDir empty")
	}
}

func TestDiscoverBoundaries_Submodule(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))

	// Submodule A uses a .git FILE (common form for submodule worktrees).
	subA := filepath.Join(root, "projects", "camp")
	mkdirAll(t, subA)
	writeFile(t, filepath.Join(subA, ".git"), "gitdir: ../../.git/modules/camp\n")

	// Submodule B uses a full .git DIRECTORY (less common but valid).
	subB := filepath.Join(root, "projects", "fest")
	mkdirAll(t, filepath.Join(subB, ".git"))
	writeFile(t, filepath.Join(subB, ".git/HEAD"), "ref: refs/heads/main\n")

	// A nested standalone repo not declared as a submodule.
	standalone := filepath.Join(root, "vendor", "third-party")
	mkdirAll(t, filepath.Join(standalone, ".git"))

	gitmodules := `[submodule "camp"]
	path = projects/camp
	url = git@example.com:Obedience-Corp/camp.git
[submodule "fest"]
	path = projects/fest
	url = git@example.com:Obedience-Corp/fest.git
`
	writeFile(t, filepath.Join(root, ".gitmodules"), gitmodules)

	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}
	// Expect 4 boundaries: root + camp + fest + vendor/third-party.
	if len(inv.Boundaries) != 4 {
		paths := make([]string, 0, len(inv.Boundaries))
		for _, b := range inv.Boundaries {
			paths = append(paths, b.RelPath)
		}
		sort.Strings(paths)
		t.Fatalf("boundaries: got %d, want 4; rels=%v", len(inv.Boundaries), paths)
	}

	byRel := map[string]scanner.Boundary{}
	for _, b := range inv.Boundaries {
		byRel[b.RelPath] = b
	}

	camp, ok := byRel["projects/camp"]
	if !ok {
		t.Fatalf("missing camp boundary; have %+v", byRel)
	}
	if !camp.IsSubmodule {
		t.Errorf("camp IsSubmodule=false, want true (declared in .gitmodules)")
	}

	fest, ok := byRel["projects/fest"]
	if !ok {
		t.Fatalf("missing fest boundary; have %+v", byRel)
	}
	if !fest.IsSubmodule {
		t.Errorf("fest IsSubmodule=false, want true")
	}

	vendor, ok := byRel["vendor/third-party"]
	if !ok {
		t.Fatalf("missing vendor boundary; have %+v", byRel)
	}
	if vendor.IsSubmodule {
		t.Errorf("vendor IsSubmodule=true, want false (not in .gitmodules)")
	}
}

func TestDiscoverBoundaries_CancelledContext(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.DiscoverBoundaries(ctx, root)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestDiscoverBoundaries_NonExistentRoot(t *testing.T) {
	_, err := scanner.DiscoverBoundaries(context.Background(), "/does/not/exist-camp-graph-test")
	if err == nil {
		t.Fatal("expected error for non-existent root")
	}
}

func TestDiscoverBoundaries_RootIsFile(t *testing.T) {
	parent := t.TempDir()
	filePath := filepath.Join(parent, "not-a-dir")
	writeFile(t, filePath, "hello")

	_, err := scanner.DiscoverBoundaries(context.Background(), filePath)
	if err == nil {
		t.Fatal("expected error when root is a regular file")
	}
}

func TestInventory_BoundaryFor(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "projects", "nested-repo")
	mkdirAll(t, filepath.Join(nested, ".git"))

	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantRel   string
		wantNotNil bool
	}{
		{
			name:       "root file resolves to root boundary",
			path:       filepath.Join(root, "README.md"),
			wantRel:    ".",
			wantNotNil: true,
		},
		{
			name:       "nested file resolves to nested boundary",
			path:       filepath.Join(nested, "cmd", "main.go"),
			wantRel:    "projects/nested-repo",
			wantNotNil: true,
		},
		{
			name:       "nested root resolves to nested boundary",
			path:       nested,
			wantRel:    "projects/nested-repo",
			wantNotNil: true,
		},
		{
			name:       "outside path returns nil",
			path:       "/completely/unrelated/path",
			wantRel:    "",
			wantNotNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inv.BoundaryFor(tt.path)
			if (got != nil) != tt.wantNotNil {
				t.Fatalf("BoundaryFor(%q) nil=%v; want not-nil=%v", tt.path, got == nil, tt.wantNotNil)
			}
			if got != nil && got.RelPath != tt.wantRel {
				t.Errorf("BoundaryFor(%q) RelPath=%q, want %q", tt.path, got.RelPath, tt.wantRel)
			}
		})
	}
}

func TestInventory_RootBoundary(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	inv, err := scanner.DiscoverBoundaries(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverBoundaries error: %v", err)
	}
	rb := inv.RootBoundary()
	if rb == nil {
		t.Fatal("RootBoundary() returned nil")
	}
	if rb.AbsPath != root {
		t.Errorf("RootBoundary AbsPath: got %q, want %q", rb.AbsPath, root)
	}
	if !rb.IsRoot {
		t.Error("RootBoundary IsRoot=false, want true")
	}
}

func TestScanner_InventoryThreading(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "projects", "alpha")
	mkdirAll(t, filepath.Join(nested, ".git"))

	s := scanner.New(root)
	if s.Inventory() != nil {
		t.Error("Inventory() before Scan should be nil")
	}
	if _, err := s.Scan(context.Background()); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	inv := s.Inventory()
	if inv == nil {
		t.Fatal("Inventory() after Scan is nil")
	}
	if len(inv.Boundaries) != 2 {
		t.Errorf("threaded inventory boundaries: got %d, want 2", len(inv.Boundaries))
	}
}
