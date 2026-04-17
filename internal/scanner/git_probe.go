package scanner

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// GitClassification holds the per-repo classification of worktree paths,
// keyed by path relative to the repo root using forward slashes.
type GitClassification struct {
	Tracked   map[string]bool
	Untracked map[string]bool
	Ignored   map[string]bool
}

// newGitClassification returns an empty classification ready for population.
func newGitClassification() *GitClassification {
	return &GitClassification{
		Tracked:   make(map[string]bool),
		Untracked: make(map[string]bool),
		Ignored:   make(map[string]bool),
	}
}

// Classify returns the GitState for a path relative to the repo root.
// Paths not found in any set default to GitStateUnknown.
func (c *GitClassification) Classify(relToRepo string) GitState {
	rel := filepath.ToSlash(filepath.Clean(relToRepo))
	if c.Ignored[rel] {
		return GitStateIgnored
	}
	if c.Tracked[rel] {
		return GitStateTracked
	}
	if c.Untracked[rel] {
		return GitStateUntracked
	}
	return GitStateUnknown
}

// GitProbe classifies worktree paths as tracked, untracked, or ignored.
type GitProbe interface {
	// Classify returns the classification for the given repo root. The
	// repoRoot is an absolute path to the directory that contains .git.
	Classify(ctx context.Context, repoRoot string) (*GitClassification, error)
}

// CmdGitProbe is a GitProbe that shells out to the git binary. It is safe
// to reuse across repos and goroutines.
type CmdGitProbe struct {
	// Binary overrides the git executable path. Empty uses "git" from PATH.
	Binary string
}

// NewCmdGitProbe returns a CmdGitProbe using the git binary on PATH.
func NewCmdGitProbe() *CmdGitProbe {
	return &CmdGitProbe{}
}

// Classify implements GitProbe using "git ls-files" invocations.
//
// Tracked files come from `git ls-files -c -z`.
// Untracked-not-ignored come from `git ls-files -o --exclude-standard -z`.
// Ignored come from `git ls-files -o --ignored --exclude-standard -z`.
//
// Each list is scoped to the repoRoot working directory.
func (p *CmdGitProbe) Classify(ctx context.Context, repoRoot string) (*GitClassification, error) {
	out := newGitClassification()

	tracked, err := p.lsFiles(ctx, repoRoot, "-c")
	if err != nil {
		return nil, graphErrors.Wrap(err, "list tracked")
	}
	for _, p := range tracked {
		out.Tracked[p] = true
	}

	untracked, err := p.lsFiles(ctx, repoRoot, "-o", "--exclude-standard")
	if err != nil {
		return nil, graphErrors.Wrap(err, "list untracked")
	}
	for _, p := range untracked {
		out.Untracked[p] = true
	}

	ignored, err := p.lsFiles(ctx, repoRoot, "-o", "--ignored", "--exclude-standard")
	if err != nil {
		return nil, graphErrors.Wrap(err, "list ignored")
	}
	for _, p := range ignored {
		out.Ignored[p] = true
	}

	return out, nil
}

// lsFiles runs `git -C repoRoot ls-files -z <flags>` and returns the list
// of NUL-separated paths normalized to forward slashes. If the directory is
// not a valid git repository (e.g. a stub .git marker with no objects),
// lsFiles returns an empty list rather than erroring; callers can still
// produce an Inventory with GitStateUnknown classifications.
func (p *CmdGitProbe) lsFiles(ctx context.Context, repoRoot string, flags ...string) ([]string, error) {
	binary := p.Binary
	if binary == "" {
		binary = "git"
	}
	args := append([]string{"-C", repoRoot, "ls-files", "-z"}, flags...)
	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if strings.Contains(msg, "not a git repository") {
			return nil, nil
		}
		return nil, graphErrors.Wrapf(err, "%s %s: %s", binary, strings.Join(args, " "), msg)
	}
	raw := stdout.Bytes()
	if len(raw) == 0 {
		return nil, nil
	}
	parts := bytes.Split(bytes.TrimRight(raw, "\x00"), []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		out = append(out, filepath.ToSlash(string(p)))
	}
	return out, nil
}

// StaticGitProbe returns preconfigured classifications. Useful for tests.
type StaticGitProbe struct {
	// ByRepo maps a repo root (absolute path) to its classification.
	ByRepo map[string]*GitClassification
	// Err, when non-nil, is returned for all Classify calls.
	Err error
}

// Classify implements GitProbe by looking up the classification in ByRepo.
// Unknown repo roots return an empty classification.
func (s *StaticGitProbe) Classify(ctx context.Context, repoRoot string) (*GitClassification, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if c, ok := s.ByRepo[repoRoot]; ok && c != nil {
		return c, nil
	}
	return newGitClassification(), nil
}
