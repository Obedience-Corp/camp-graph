package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// BuildInventory discovers boundaries and walks the live worktree inside
// each boundary, producing an Inventory with file-level entries.
//
// Boundary discovery uses DiscoverBoundaries. The walk respects
// boundaries: when walking the campaign root, descent stops at each
// nested boundary and that nested boundary is walked separately so that
// git classification is applied per-repo.
//
// By default, ignored entries are excluded and ignored directories are
// not descended into. Set InventoryOptions.IncludeIgnored to include
// them.
func BuildInventory(ctx context.Context, root string, opts InventoryOptions) (*Inventory, error) {
	inv, err := DiscoverBoundaries(ctx, root)
	if err != nil {
		return nil, err
	}
	if err := walkEntries(ctx, inv, opts); err != nil {
		return nil, err
	}
	return inv, nil
}

// walkEntries populates inv.Entries for every boundary in inv.
func walkEntries(ctx context.Context, inv *Inventory, opts InventoryOptions) error {
	probe := opts.GitProbe
	if probe == nil {
		probe = NewCmdGitProbe()
	}

	// Classify each boundary once so the walker can annotate entries
	// without repeated git invocations.
	classifications := make(map[string]*GitClassification, len(inv.Boundaries))
	for _, b := range inv.Boundaries {
		if b.GitDir == "" {
			classifications[b.AbsPath] = newGitClassification()
			continue
		}
		c, err := probe.Classify(ctx, b.AbsPath)
		if err != nil {
			return graphErrors.Wrapf(err, "git classify %q", b.AbsPath)
		}
		classifications[b.AbsPath] = c
	}

	// Precompute nested boundary lookup for stopping descent at nested
	// repo roots when walking an outer boundary.
	nestedChildren := groupNestedChildren(inv)

	var entries []InventoryEntry
	for i := range inv.Boundaries {
		b := &inv.Boundaries[i]
		cls := classifications[b.AbsPath]
		boundaryEntries, err := walkBoundary(ctx, inv, b, cls, nestedChildren[b.AbsPath], opts)
		if err != nil {
			return err
		}
		entries = append(entries, boundaryEntries...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})
	inv.Entries = entries
	return nil
}

// groupNestedChildren returns, for each boundary, the absolute paths of
// any other boundaries that are its direct descendants (the deepest
// ancestor relationship). Used to stop descent at nested boundaries when
// walking an outer one.
func groupNestedChildren(inv *Inventory) map[string]map[string]bool {
	children := make(map[string]map[string]bool, len(inv.Boundaries))
	for i := range inv.Boundaries {
		children[inv.Boundaries[i].AbsPath] = map[string]bool{}
	}
	for i := range inv.Boundaries {
		child := inv.Boundaries[i].AbsPath
		// Find the deepest boundary that strictly contains this one.
		var parent string
		for j := range inv.Boundaries {
			cand := inv.Boundaries[j].AbsPath
			if cand == child {
				continue
			}
			if !pathContains(cand, child) {
				continue
			}
			if len(cand) > len(parent) {
				parent = cand
			}
		}
		if parent != "" {
			children[parent][child] = true
		}
	}
	return children
}

// walkBoundary walks a single boundary's filesystem and emits entries.
// Descent is suppressed at nested boundary roots and .git directories.
// Ignored paths are excluded unless opts.IncludeIgnored is true.
func walkBoundary(
	ctx context.Context,
	inv *Inventory,
	b *Boundary,
	cls *GitClassification,
	nestedChildren map[string]bool,
	opts InventoryOptions,
) ([]InventoryEntry, error) {
	var out []InventoryEntry
	hasGit := b.GitDir != ""

	err := filepath.WalkDir(b.AbsPath, func(path string, d fs.DirEntry, werr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if werr != nil {
			if os.IsPermission(werr) && d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return werr
		}
		// Skip the boundary's own .git marker.
		if d.Name() == ".git" && path != b.AbsPath {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Never descend into another boundary's subtree when walking an
		// outer boundary; that boundary walks itself.
		if d.IsDir() && path != b.AbsPath && nestedChildren[path] {
			return filepath.SkipDir
		}
		// Skip the boundary root record itself; we only emit entries for
		// content strictly inside a boundary.
		if path == b.AbsPath {
			return nil
		}

		relToRepo, err := filepath.Rel(b.AbsPath, path)
		if err != nil {
			return graphErrors.Wrapf(err, "rel to repo %q", path)
		}
		relToRepo = filepath.ToSlash(relToRepo)

		relToRoot, err := filepath.Rel(inv.Root, path)
		if err != nil {
			return graphErrors.Wrapf(err, "rel to root %q", path)
		}
		relToRoot = filepath.ToSlash(relToRoot)

		var state GitState
		isIgnored := false
		if hasGit {
			state = cls.Classify(relToRepo)
			if state == GitStateIgnored {
				isIgnored = true
			}
		} else {
			state = GitStateUnknown
		}

		// For directories, git ls-files does not list directories per se.
		// Treat a directory as ignored when every classification set misses
		// it but any descendant path under it is classified ignored - this
		// keeps ignored-dir descent-skipping cheap without a separate git
		// call. We detect this by checking whether any ignored entry
		// starts with the directory's relative path.
		if d.IsDir() && hasGit && state == GitStateUnknown {
			if directoryIsIgnored(relToRepo, cls) {
				state = GitStateIgnored
				isIgnored = true
			}
		}

		if isIgnored && !opts.IncludeIgnored {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := InventoryEntry{
			AbsPath:   path,
			RelPath:   relToRoot,
			RepoRoot:  b.AbsPath,
			RelToRepo: relToRepo,
			GitState:  state,
			IsIgnored: isIgnored,
			IsDir:     d.IsDir(),
			Extension: fileExtension(path, d.IsDir()),
			PathDepth: pathDepth(relToRoot),
		}
		out = append(out, entry)
		return nil
	})
	if err != nil {
		return nil, graphErrors.Wrapf(err, "walk boundary %q", b.AbsPath)
	}
	return out, nil
}

// directoryIsIgnored reports whether any ignored file sits under relDir.
// relDir is relative to the repo root with forward slashes.
func directoryIsIgnored(relDir string, cls *GitClassification) bool {
	prefix := relDir
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	// Cheap heuristic: any ignored path with this prefix implies the dir
	// is at least partially ignored. We only flip to ignored if EVERY
	// classified path under it is ignored, because a partially-ignored
	// dir still has authored content we want to see.
	var anyUnder bool
	for p := range cls.Tracked {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	for p := range cls.Untracked {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	for p := range cls.Ignored {
		if strings.HasPrefix(p, prefix) {
			anyUnder = true
			break
		}
	}
	return anyUnder
}

// fileExtension returns the lower-cased extension without the leading dot.
func fileExtension(path string, isDir bool) string {
	if isDir {
		return ""
	}
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// pathDepth returns the number of path segments in a forward-slashed
// relative path. "." has depth 0.
func pathDepth(rel string) int {
	if rel == "" || rel == "." {
		return 0
	}
	return strings.Count(rel, "/") + 1
}
