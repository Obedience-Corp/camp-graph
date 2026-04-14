package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const goreleaserCmd = "github.com/goreleaser/goreleaser/v2@v2.15.2"

var stableTagPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

type stableVersion struct {
	Major int
	Minor int
	Patch int
}

func (v stableVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func runCurrent() error {
	if err := fetchOriginRefs(); err != nil {
		return err
	}

	tags, err := listStableTags()
	if err != nil {
		return err
	}

	if len(tags) == 0 {
		fmt.Println("none")
		return nil
	}

	fmt.Println(tags[0].String())
	return nil
}

func runStable(level string) error {
	if _, err := validateLevel(level); err != nil {
		return err
	}

	if err := ensureCleanTree(); err != nil {
		return err
	}
	if err := ensureBranch("main"); err != nil {
		return err
	}
	if err := fetchOriginRefs(); err != nil {
		return err
	}
	if err := ensureSyncedWithOriginMain(); err != nil {
		return err
	}
	if err := runGoReleaserCheck(); err != nil {
		return err
	}

	tags, err := listStableTags()
	if err != nil {
		return err
	}

	next, err := nextStableTag(tags, level)
	if err != nil {
		return err
	}

	return createAndPushTag(next)
}

func runTag(version string) error {
	if err := validateExplicitTag(version); err != nil {
		return err
	}
	if err := ensureCleanTree(); err != nil {
		return err
	}
	if err := ensureBranch("main"); err != nil {
		return err
	}
	if err := fetchOriginRefs(); err != nil {
		return err
	}
	if err := ensureSyncedWithOriginMain(); err != nil {
		return err
	}
	if err := runGoReleaserCheck(); err != nil {
		return err
	}

	return createAndPushTag(version)
}

func validateLevel(level string) (string, error) {
	switch level {
	case "patch", "minor", "major":
		return level, nil
	default:
		return "", fmt.Errorf("unknown level %q (use patch, minor, or major)", level)
	}
}

func validateExplicitTag(tag string) error {
	if !stableTagPattern.MatchString(tag) {
		return fmt.Errorf("version must match stable semver like v0.1.0")
	}
	return nil
}

func nextStableTag(tags []stableVersion, level string) (string, error) {
	if _, err := validateLevel(level); err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "v0.1.0", nil
	}

	next := tags[0]
	switch level {
	case "major":
		next.Major++
		next.Minor = 0
		next.Patch = 0
	case "minor":
		next.Minor++
		next.Patch = 0
	case "patch":
		next.Patch++
	}

	return next.String(), nil
}

func listStableTags() ([]stableVersion, error) {
	out, err := runGitOutput("tag", "-l", "v*")
	if err != nil {
		return nil, err
	}

	lines := strings.Fields(strings.TrimSpace(out))
	versions := make([]stableVersion, 0, len(lines))
	for _, line := range lines {
		v, ok, err := parseStableTag(line)
		if err != nil {
			return nil, err
		}
		if ok {
			versions = append(versions, v)
		}
	}

	sortStableVersionsDesc(versions)
	return versions, nil
}

func parseStableTag(tag string) (stableVersion, bool, error) {
	matches := stableTagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return stableVersion{}, false, nil
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return stableVersion{}, false, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return stableVersion{}, false, err
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return stableVersion{}, false, err
	}

	return stableVersion{Major: major, Minor: minor, Patch: patch}, true, nil
}

func sortStableVersionsDesc(versions []stableVersion) {
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if compareStableVersion(versions[j], versions[i]) > 0 {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
}

func compareStableVersion(a, b stableVersion) int {
	switch {
	case a.Major != b.Major:
		return compareInts(a.Major, b.Major)
	case a.Minor != b.Minor:
		return compareInts(a.Minor, b.Minor)
	default:
		return compareInts(a.Patch, b.Patch)
	}
}

func compareInts(a, b int) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	default:
		return 0
	}
}

func ensureCleanTree() error {
	out, err := runGitOutput("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("repo has uncommitted changes")
	}
	return nil
}

func ensureBranch(expected string) error {
	branch, err := runGitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	if branch != expected {
		return fmt.Errorf("releases must be cut from %s (current: %s)", expected, branch)
	}
	return nil
}

func fetchOriginRefs() error {
	cmd := exec.Command("git", "fetch", "--prune", "--prune-tags", "origin", "+refs/tags/*:refs/tags/*", "+refs/heads/main:refs/remotes/origin/main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fetching origin refs: %w", err)
	}
	return nil
}

func ensureSyncedWithOriginMain() error {
	head, err := runGitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	originMain, err := runGitOutput("rev-parse", "origin/main")
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) != strings.TrimSpace(originMain) {
		return errors.New("local main is not in sync with origin/main; run `git pull --ff-only origin main`")
	}
	return nil
}

func runGoReleaserCheck() error {
	cmd := exec.Command("go", "run", goreleaserCmd, "check")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goreleaser check failed: %w", err)
	}
	return nil
}

func createAndPushTag(tag string) error {
	if err := ensureTagAbsent(tag); err != nil {
		return err
	}

	fmt.Printf("Creating release tag %s\n", tag)
	if err := runGitStreaming("tag", "-a", tag, "-m", "Release "+tag); err != nil {
		return err
	}

	fmt.Printf("Pushing %s\n", tag)
	if err := runGitStreaming("push", "origin", tag); err != nil {
		return err
	}

	fmt.Printf("Pushed %s\n", tag)
	fmt.Println("Release workflow should now start on GitHub.")
	return nil
}

func ensureTagAbsent(tag string) error {
	out, err := runGitOutput("tag", "-l", tag)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("tag %s already exists locally", tag)
	}

	out, err = runGitOutput("ls-remote", "--tags", "origin", "refs/tags/"+tag)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("tag %s already exists on origin", tag)
	}
	return nil
}

func runGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitStreaming(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
