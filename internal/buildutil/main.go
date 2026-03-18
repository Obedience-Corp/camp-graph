package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Obedience-Corp/obey-shared/buildutil"
)

func main() {
	buildutil.Run(os.Args[1:], buildutil.BuildConfig{
		BinaryName:  "camp-graph",
		MainPath:    "./cmd/camp-graph",
		SectionName: "Camp Graph",
		LDFlags:     ldflags,
		IntegrationBuildEnv: func() []string {
			// Pure Go SQLite (modernc.org/sqlite) — no CGO or zig needed
			return []string{
				"CGO_ENABLED=0",
				"GOOS=linux",
				"GOARCH=" + runtime.GOARCH,
			}
		},
	})
}

func ldflags() string {
	versionPkg := "github.com/Obedience-Corp/camp-graph/internal/version"
	version := envOrDefault("VERSION", "dev")
	commit := cmdOutput("git", "rev-parse", "--short", "HEAD")
	date := cmdOutput("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")

	parts := []string{
		fmt.Sprintf("-X %s.Version=%s", versionPkg, version),
		fmt.Sprintf("-X %s.Commit=%s", versionPkg, commit),
		fmt.Sprintf("-X %s.BuildDate=%s", versionPkg, date),
	}
	return strings.Join(parts, " ")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func cmdOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
