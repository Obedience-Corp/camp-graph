package scanner

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// DiscoverBoundaries walks the campaign root and identifies every repository
// boundary using explicit .git markers, not path-name heuristics. The
// campaign root is always included as a boundary even when it has no .git
// marker so that later passes can anchor file inventory against a single
// canonical root.
//
// A nested boundary is any directory that contains a .git marker (either a
// directory, which indicates a regular nested repository, or a regular file,
// which indicates a submodule worktree or git worktree). Submodule status is
// cross-referenced against the campaign root's .gitmodules file so the
// inventory can distinguish submodules from standalone nested repos.
//
// Discovery stops descending at each nested boundary. Callers that need the
// interior of a nested repo should issue a second DiscoverBoundaries call
// rooted at that nested path.
func DiscoverBoundaries(ctx context.Context, root string) (*Inventory, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, graphErrors.Wrapf(err, "resolve root %q", root)
	}
	// Resolve symlinks so the root matches the canonical form
	// filepath.WalkDir produces for descendants. On macOS `/var` resolves
	// to `/private/var`; without this, the root boundary stores one form
	// while the walker emits the other, breaking every downstream
	// rel-path derivation and boundary classification.
	if resolved, rerr := filepath.EvalSymlinks(absRoot); rerr == nil {
		absRoot = resolved
	} else if !os.IsNotExist(rerr) {
		return nil, graphErrors.Wrapf(rerr, "resolve symlinks for %q", absRoot)
	}
	if st, err := os.Stat(absRoot); err != nil {
		return nil, graphErrors.Wrapf(err, "stat root %q", absRoot)
	} else if !st.IsDir() {
		return nil, graphErrors.New("root " + absRoot + " is not a directory")
	}

	rootBoundary := Boundary{
		AbsPath: absRoot,
		RelPath: ".",
		IsRoot:  true,
	}
	if gitMarker, ok := probeGitMarker(absRoot); ok {
		rootBoundary.GitDir = gitMarker
	}

	submodules, err := readSubmodulePaths(absRoot)
	if err != nil {
		return nil, graphErrors.Wrap(err, "read submodule paths")
	}

	inv := &Inventory{
		Root:       absRoot,
		Boundaries: []Boundary{rootBoundary},
	}

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, werr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if werr != nil {
			if os.IsPermission(werr) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return werr
		}
		if path == absRoot {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Never descend into a .git directory - its contents are not
		// part of the workspace inventory.
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		gitMarker, ok := probeGitMarker(path)
		if !ok {
			return nil
		}
		rel, rerr := filepath.Rel(absRoot, path)
		if rerr != nil {
			return graphErrors.Wrapf(rerr, "relativize %q", path)
		}
		rel = filepath.ToSlash(rel)
		b := Boundary{
			AbsPath:     path,
			RelPath:     rel,
			GitDir:      gitMarker,
			IsSubmodule: submodules[filepath.ToSlash(filepath.Clean(rel))],
		}
		inv.Boundaries = append(inv.Boundaries, b)
		// Stop descending; nested interior is out of this pass's scope.
		return filepath.SkipDir
	})
	if walkErr != nil {
		return nil, graphErrors.Wrapf(walkErr, "walk %q", absRoot)
	}

	sort.Slice(inv.Boundaries, func(i, j int) bool {
		return inv.Boundaries[i].AbsPath < inv.Boundaries[j].AbsPath
	})
	return inv, nil
}

// probeGitMarker returns the absolute path to a .git marker inside dir
// if one exists AND the marker looks like a real git boundary. The
// marker may be:
//   - a directory (regular repo)
//   - a regular file whose first line begins with "gitdir:" (submodule
//     worktree or linked worktree pointer)
//   - a symlink resolving to one of the above
//
// Arbitrary files named .git, zero-byte files, and broken symlinks are
// rejected so boundary discovery does not register noise that would
// later cause `git ls-files` to fail inside the directory.
func probeGitMarker(dir string) (string, bool) {
	marker := filepath.Join(dir, ".git")
	info, err := os.Stat(marker) // Stat follows symlinks.
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return marker, true
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", false
	}
	f, err := os.Open(marker)
	if err != nil {
		return "", false
	}
	defer f.Close()
	// The gitdir pointer file is always tiny; peek at the first line.
	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	head := strings.TrimSpace(string(buf[:n]))
	if !strings.HasPrefix(head, "gitdir:") {
		return "", false
	}
	return marker, true
}

// readSubmodulePaths parses the campaign root's .gitmodules file and
// returns a set of submodule worktree paths (relative to the root, forward
// slashes). Missing .gitmodules is treated as an empty set.
func readSubmodulePaths(absRoot string) (map[string]bool, error) {
	out := make(map[string]bool)
	path := filepath.Join(absRoot, ".gitmodules")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, graphErrors.Wrapf(err, "open %q", path)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "path") {
			continue
		}
		// Expected form: `path = relative/path/to/submodule`.
		_, rhs, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		raw := strings.TrimSpace(rhs)
		if raw == "" {
			continue
		}
		normalized := filepath.ToSlash(filepath.Clean(raw))
		out[normalized] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, graphErrors.Wrapf(err, "read %q", path)
	}
	return out, nil
}
