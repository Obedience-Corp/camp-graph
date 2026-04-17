package scanner

import (
	"path/filepath"
	"strings"
)

// GitState classifies how git sees a path within its owning boundary.
type GitState string

const (
	// GitStateTracked means the file is in git's index.
	GitStateTracked GitState = "tracked"
	// GitStateUntracked means the file exists in the worktree but is not
	// tracked and is not ignored.
	GitStateUntracked GitState = "untracked"
	// GitStateIgnored means the file is matched by a .gitignore rule.
	GitStateIgnored GitState = "ignored"
	// GitStateUnknown means the owning boundary has no git metadata, so
	// no classification is available.
	GitStateUnknown GitState = "unknown"
)

// InventoryEntry describes a single file or directory discovered during the
// inventory walk. Entries carry enough metadata for later scope, extraction,
// inference, and refresh passes to operate without re-walking the tree.
type InventoryEntry struct {
	// AbsPath is the absolute path to the entry.
	AbsPath string
	// RelPath is the path relative to the campaign root, using forward
	// slashes so downstream passes can use it as a stable ID component.
	RelPath string
	// RepoRoot is the absolute path to the owning boundary (campaign root
	// or nested repo). For files at the campaign root this equals
	// Inventory.Root.
	RepoRoot string
	// RelToRepo is the path relative to RepoRoot, forward-slashed. Useful
	// when applying git classification which is always scoped to a repo.
	RelToRepo string
	// GitState is the git classification of the entry.
	GitState GitState
	// IsIgnored is true when the entry is matched by a .gitignore rule.
	// An ignored directory suppresses descent unless the walker was
	// configured to include ignored content.
	IsIgnored bool
	// IsDir reports whether this entry is a directory.
	IsDir bool
	// Extension is the lower-cased file extension with the leading dot
	// stripped. Empty for directories and files without an extension.
	Extension string
	// PathDepth is the number of path segments from the campaign root.
	// The campaign root itself has depth 0; "projects" has depth 1;
	// "projects/alpha/README.md" has depth 3.
	PathDepth int
}

// Boundary represents a single repository boundary discovered within a
// campaign. The campaign root is always present as a boundary; any nested
// regular repos or submodules under the root are also represented.
type Boundary struct {
	// AbsPath is the absolute path to the repository root.
	AbsPath string
	// RelPath is the path relative to the campaign root. The campaign root
	// itself has a RelPath of ".".
	RelPath string
	// IsRoot reports whether this boundary is the campaign root.
	IsRoot bool
	// IsSubmodule reports whether this boundary is a submodule declared in
	// the campaign root's .gitmodules file.
	IsSubmodule bool
	// GitDir is the absolute path to the .git marker (directory or file)
	// for the repository. Empty if the boundary is a bare directory that
	// happens to be the campaign root without a .git marker.
	GitDir string
}

// Inventory carries the workspace-level boundary metadata and per-file
// records that later scanner passes consume.
type Inventory struct {
	// Root is the absolute campaign root path. It is always present even
	// when no .git marker exists at the root.
	Root string
	// Boundaries contains every discovered boundary including the campaign
	// root, sorted by AbsPath in ascending order.
	Boundaries []Boundary
	// Entries are the file and directory records discovered inside the
	// boundaries. Entries respect the InventoryOptions in effect at build
	// time; by default ignored paths are excluded. Entries are sorted by
	// RelPath in ascending order.
	Entries []InventoryEntry
}

// RootBoundary returns the campaign-root boundary.
func (inv *Inventory) RootBoundary() *Boundary {
	for i := range inv.Boundaries {
		if inv.Boundaries[i].IsRoot {
			return &inv.Boundaries[i]
		}
	}
	return nil
}

// BoundaryFor returns the deepest boundary that contains the given absolute
// path. It returns nil if the path is outside every known boundary.
func (inv *Inventory) BoundaryFor(absPath string) *Boundary {
	clean := filepath.Clean(absPath)
	var best *Boundary
	for i := range inv.Boundaries {
		b := &inv.Boundaries[i]
		if !pathContains(b.AbsPath, clean) {
			continue
		}
		if best == nil || len(b.AbsPath) > len(best.AbsPath) {
			best = b
		}
	}
	return best
}

// pathContains reports whether base contains target as an equal path or a
// descendant path. It uses filepath.Separator-aware comparison instead of
// raw string prefix matching so that sibling names like "/a/foo" and
// "/a/foobar" are not confused.
func pathContains(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	if base == target {
		return true
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(base, sep) {
		base += sep
	}
	return strings.HasPrefix(target, base)
}

// InventoryOptions configures BuildInventory behavior.
type InventoryOptions struct {
	// IncludeIgnored, when true, emits entries for paths that match a
	// .gitignore rule. When false, ignored entries are skipped and
	// ignored directories suppress descent.
	IncludeIgnored bool
	// GitProbe classifies paths as tracked, untracked, or ignored. If
	// nil, the walker uses a command-backed probe that shells out to
	// the git binary. Passing a custom probe is intended for tests.
	GitProbe GitProbe
}
