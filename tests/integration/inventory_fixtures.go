//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
)

// RepoSpec describes a git repository fixture to create inside the container.
// It is used by SetupRepoFixtures to build campaign roots that include
// nested repos, submodules, tracked files, untracked files, and ignored
// content for inventory testing.
type RepoSpec struct {
	// Path is the absolute path inside the container where this repo will
	// be rooted.
	Path string
	// TrackedFiles maps relative paths to content. These files are added
	// and committed after the repo is initialized.
	TrackedFiles map[string]string
	// UntrackedFiles maps relative paths to content. These files are
	// written to the worktree but not staged or committed.
	UntrackedFiles map[string]string
	// IgnoredPatterns are appended to the repo's .gitignore. Files matching
	// these patterns are classified as ignored.
	IgnoredPatterns []string
	// IgnoredFiles maps relative paths to content. These files are written
	// to the worktree and are expected to match IgnoredPatterns.
	IgnoredFiles map[string]string
	// SubmodulePath, when set, registers this repo as a submodule of its
	// parent at the given path (relative to the parent's root). The parent
	// is identified by PathContains at fixture-creation time.
	SubmodulePath string
	// ParentPath, when set, is the absolute path of this repo's parent
	// inside the container. Submodule registration uses this relationship.
	ParentPath string
}

// SetupRepoFixtures creates git repositories inside the shared container
// that together exercise tracked, untracked, ignored, nested-repo, and
// submodule scenarios.
//
// This is the required path for repo-state mutation tests per festival
// rules: host-side t.TempDir() mutation is not permitted for repo-state
// behavior. Authors of later inventory or refresh tests should call this
// helper (or compose RepoSpec manually) to produce fixture state.
func (tc *TestContainer) SetupRepoFixtures(t *testing.T, specs []RepoSpec) {
	t.Helper()
	// Ensure git identity is configured in the container so commits
	// succeed without host credentials.
	tc.ensureGitIdentity(t)

	for _, spec := range specs {
		tc.createRepo(t, spec)
	}

	// Second pass: any repo with SubmodulePath+ParentPath registers as a
	// submodule of its parent. This happens after all repos are created
	// so both sides are fully initialized.
	for _, spec := range specs {
		if spec.SubmodulePath == "" || spec.ParentPath == "" {
			continue
		}
		tc.registerSubmodule(t, spec.ParentPath, spec.Path, spec.SubmodulePath)
	}
}

// ensureGitIdentity configures user.name and user.email so `git commit`
// works inside the container without interactive prompts.
func (tc *TestContainer) ensureGitIdentity(t *testing.T) {
	t.Helper()
	cmds := [][]string{
		{"git", "config", "--global", "user.email", "fixture@camp-graph.test"},
		{"git", "config", "--global", "user.name", "Fixture Bot"},
		{"git", "config", "--global", "init.defaultBranch", "main"},
		// Safe directory disables ownership checks, needed because the
		// tree is created by the test driver inside the container.
		{"git", "config", "--global", "safe.directory", "*"},
	}
	// Alpine images may not ship with git preinstalled; ensure it first.
	if out, code, err := tc.ExecCommand("sh", "-c", "command -v git >/dev/null 2>&1 || apk add --no-cache git"); err != nil || code != 0 {
		t.Fatalf("install git in container: %v (exit=%d): %s", err, code, out)
	}
	for _, cmd := range cmds {
		if out, code, err := tc.ExecCommand(cmd...); err != nil || code != 0 {
			t.Fatalf("git config (%v) failed: %v (exit=%d): %s", cmd, err, code, out)
		}
	}
}

// createRepo initializes a git repo at spec.Path, writes tracked,
// untracked, and ignored files per the spec, commits tracked content, and
// applies .gitignore patterns.
func (tc *TestContainer) createRepo(t *testing.T, spec RepoSpec) {
	t.Helper()
	if spec.Path == "" {
		t.Fatalf("RepoSpec.Path is required")
	}
	if err := tc.MkdirAll(spec.Path); err != nil {
		t.Fatalf("mkdir %s: %v", spec.Path, err)
	}

	if out, code, err := tc.ExecCommand("git", "-C", spec.Path, "init", "-q"); err != nil || code != 0 {
		t.Fatalf("git init %s failed: %v (exit=%d): %s", spec.Path, err, code, out)
	}

	// Tracked files first, so they land in the initial commit.
	for rel, content := range spec.TrackedFiles {
		full := joinPath(spec.Path, rel)
		if err := tc.WriteFile(full, content); err != nil {
			t.Fatalf("write tracked %s: %v", full, err)
		}
	}

	// .gitignore before ignored files so matching is applied.
	if len(spec.IgnoredPatterns) > 0 {
		gitignore := joinPath(spec.Path, ".gitignore")
		existing := ""
		if ok, _ := tc.CheckFileExists(gitignore); ok {
			out, code, err := tc.ExecCommand("cat", gitignore)
			if err == nil && code == 0 {
				existing = out
				if !strings.HasSuffix(existing, "\n") {
					existing += "\n"
				}
			}
		}
		body := existing + strings.Join(spec.IgnoredPatterns, "\n") + "\n"
		if err := tc.WriteFile(gitignore, body); err != nil {
			t.Fatalf("write .gitignore at %s: %v", gitignore, err)
		}
	}

	// Add and commit everything currently non-ignored in the worktree.
	// This captures TrackedFiles plus .gitignore itself.
	if len(spec.TrackedFiles) > 0 || len(spec.IgnoredPatterns) > 0 {
		if out, code, err := tc.ExecCommand("git", "-C", spec.Path, "add", "."); err != nil || code != 0 {
			t.Fatalf("git add . in %s failed: %v (exit=%d): %s", spec.Path, err, code, out)
		}
		if out, code, err := tc.ExecCommand("git", "-C", spec.Path, "commit", "-q", "-m", "fixture initial commit", "--allow-empty"); err != nil || code != 0 {
			t.Fatalf("git commit in %s failed: %v (exit=%d): %s", spec.Path, err, code, out)
		}
	}

	// Untracked and ignored files go in after the commit so they do not
	// accidentally land in the index.
	for rel, content := range spec.UntrackedFiles {
		full := joinPath(spec.Path, rel)
		if err := tc.WriteFile(full, content); err != nil {
			t.Fatalf("write untracked %s: %v", full, err)
		}
	}
	for rel, content := range spec.IgnoredFiles {
		full := joinPath(spec.Path, rel)
		if err := tc.WriteFile(full, content); err != nil {
			t.Fatalf("write ignored %s: %v", full, err)
		}
	}
}

// registerSubmodule records childPath as a submodule of parentPath at the
// given relPath. It writes a .gitmodules entry at parentPath and ensures
// the child repo's .git directory is preserved (we do not run
// `git submodule add` because that would require a remote URL; fixtures
// use filesystem paths for determinism).
func (tc *TestContainer) registerSubmodule(t *testing.T, parentPath, childPath, relPath string) {
	t.Helper()
	gitmodulesPath := joinPath(parentPath, ".gitmodules")
	existing := ""
	if ok, _ := tc.CheckFileExists(gitmodulesPath); ok {
		if out, code, err := tc.ExecCommand("cat", gitmodulesPath); err == nil && code == 0 {
			existing = out
			if !strings.HasSuffix(existing, "\n") {
				existing += "\n"
			}
		}
	}
	entry := fmt.Sprintf("[submodule %q]\n\tpath = %s\n\turl = file://%s\n", relPath, relPath, childPath)
	body := existing + entry
	if err := tc.WriteFile(gitmodulesPath, body); err != nil {
		t.Fatalf("write .gitmodules at %s: %v", gitmodulesPath, err)
	}
	// Stage and commit the .gitmodules change if the parent is a repo.
	if ok, _ := tc.CheckDirExists(joinPath(parentPath, ".git")); ok {
		if out, code, err := tc.ExecCommand("git", "-C", parentPath, "add", ".gitmodules"); err != nil || code != 0 {
			t.Fatalf("git add .gitmodules in %s failed: %v (exit=%d): %s", parentPath, err, code, out)
		}
		if out, code, err := tc.ExecCommand("git", "-C", parentPath, "commit", "-q", "-m", "fixture: register submodule "+relPath); err != nil || code != 0 {
			t.Fatalf("git commit .gitmodules in %s failed: %v (exit=%d): %s", parentPath, err, code, out)
		}
	}
}

// joinPath joins base and rel with a forward slash, preserving absolute
// base paths. Intended for in-container POSIX paths.
func joinPath(base, rel string) string {
	if base == "" {
		return rel
	}
	if rel == "" {
		return base
	}
	if strings.HasSuffix(base, "/") {
		return base + rel
	}
	return base + "/" + rel
}

// RepoFixtureSummary returns a newline-delimited summary of the repo at
// path: tracked files, untracked files, and ignored files. It shells out
// to git ls-files so tests can assert on the real git classification.
func (tc *TestContainer) RepoFixtureSummary(t *testing.T, path string) string {
	t.Helper()
	tracked, _, _ := tc.ExecCommand("git", "-C", path, "ls-files", "-c")
	untracked, _, _ := tc.ExecCommand("git", "-C", path, "ls-files", "-o", "--exclude-standard")
	ignored, _, _ := tc.ExecCommand("git", "-C", path, "ls-files", "-o", "--ignored", "--exclude-standard")
	var b strings.Builder
	b.WriteString("tracked:\n")
	b.WriteString(tracked)
	b.WriteString("untracked:\n")
	b.WriteString(untracked)
	b.WriteString("ignored:\n")
	b.WriteString(ignored)
	return b.String()
}
