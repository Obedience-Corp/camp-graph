//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// demuxDockerOutput strips Docker exec multiplexed stream headers from output.
// Docker exec output is multiplexed with 8-byte headers:
// - byte 0: stream type (1=stdout, 2=stderr)
// - bytes 1-3: padding (zeros)
// - bytes 4-7: big-endian uint32 payload size
func demuxDockerOutput(data []byte) []byte {
	var result bytes.Buffer
	offset := 0
	for offset < len(data) {
		if offset+8 > len(data) {
			result.Write(data[offset:])
			break
		}
		payloadSize := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		payloadStart := offset + 8
		payloadEnd := payloadStart + int(payloadSize)
		if payloadEnd > len(data) {
			payloadEnd = len(data)
		}
		result.Write(data[payloadStart:payloadEnd])
		offset = payloadEnd
	}
	return result.Bytes()
}

// TestContainer wraps container operations for testing.
type TestContainer struct {
	container testcontainers.Container
	ctx       context.Context
	t         *testing.T
}

// NewSharedContainer creates a container for reuse across multiple tests.
func NewSharedContainer() (*TestContainer, error) {
	ctx := context.Background()

	// Build camp-graph binary for Linux
	graphBinary, err := buildGraphBinaryShared()
	if err != nil {
		return nil, fmt.Errorf("failed to build camp-graph binary: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Cmd:        []string{"sleep", "3600"},
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(30 * time.Second),
		AutoRemove: true,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Copy binary into container
	if err := container.CopyFileToContainer(ctx, graphBinary, "/camp-graph", 0o755); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to copy camp-graph binary into container: %w", err)
	}

	// Create initial working directories
	exitCode, _, err := container.Exec(ctx, []string{"mkdir", "-p", "/test", "/campaign"})
	if err != nil || exitCode != 0 {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create initial directories: %w", err)
	}

	return &TestContainer{
		container: container,
		ctx:       ctx,
		t:         nil,
	}, nil
}

// buildGraphBinaryShared builds the camp-graph binary for Linux.
func buildGraphBinaryShared() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Navigate to project root (from tests/integration/)
	projectRoot := filepath.Join(cwd, "../..")
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	binDir := filepath.Join(projectRoot, "bin", "linux")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin/linux directory: %w", err)
	}

	binaryPath := filepath.Join(binDir, "camp-graph")

	cmd := fmt.Sprintf("cd %s && GOOS=linux GOARCH=%s go build -o %s ./cmd/camp-graph", projectRoot, runtime.GOARCH, binaryPath)
	if err := runCommand(cmd); err != nil {
		return "", fmt.Errorf("failed to build binary: %w", err)
	}

	return binaryPath, nil
}

func runCommand(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Reset clears container state between tests.
func (tc *TestContainer) Reset() error {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		"rm -rf /test /campaign 2>/dev/null; mkdir -p /test /campaign; sync",
	})
	if err != nil {
		return fmt.Errorf("failed to reset container: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("reset command failed with exit code %d", exitCode)
	}
	return nil
}

// Cleanup terminates the container.
func (tc *TestContainer) Cleanup() {
	if tc.container != nil {
		tc.container.Terminate(tc.ctx)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// RunGraph runs the camp-graph command in the container.
func (tc *TestContainer) RunGraph(args ...string) (string, error) {
	cmd := append([]string{"/camp-graph"}, args...)

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute camp-graph: %w", err)
	}

	rawOutput, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	output := demuxDockerOutput(rawOutput)

	if exitCode != 0 {
		return string(output), fmt.Errorf("camp-graph exited with code %d: %s", exitCode, output)
	}

	return string(output), nil
}

// RunGraphInDir runs camp-graph from a specific directory.
func (tc *TestContainer) RunGraphInDir(dir string, args ...string) (string, error) {
	return tc.RunGraphInDirWithEnv(dir, map[string]string{"CAMP_ROOT": dir}, args...)
}

// RunGraphInDirWithEnv runs camp-graph from a specific directory with explicit env vars.
func (tc *TestContainer) RunGraphInDirWithEnv(dir string, env map[string]string, args ...string) (string, error) {
	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = shellQuote(arg)
	}

	var envParts []string
	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			envParts = append(envParts, fmt.Sprintf("%s=%s", k, shellQuote(env[k])))
		}
	}

	envPrefix := ""
	if len(envParts) > 0 {
		envPrefix = strings.Join(envParts, " ") + " "
	}

	cmdStr := fmt.Sprintf("cd %s && %s/camp-graph %s 2>&1", shellQuote(dir), envPrefix, strings.Join(quotedArgs, " "))
	cmd := []string{"sh", "-c", cmdStr}

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute camp-graph: %w", err)
	}

	rawOutput, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	output := demuxDockerOutput(rawOutput)

	if exitCode != 0 {
		return string(output), fmt.Errorf("camp-graph exited with code %d: %s", exitCode, output)
	}

	return string(output), nil
}

// WriteFile writes content to a file in the container.
func (tc *TestContainer) WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", dir})
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	exitCode, _, err = tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("printf '%%s' '%s' > %s", strings.ReplaceAll(content, "'", "'\\''"), path),
	})
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// MkdirAll creates directories in the container.
func (tc *TestContainer) MkdirAll(path string) error {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", path})
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// CheckFileExists checks if a file exists in the container.
func (tc *TestContainer) CheckFileExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-f", path})
	if err != nil {
		return false, fmt.Errorf("failed to check file: %w", err)
	}
	return exitCode == 0, nil
}

// CheckDirExists checks if a directory exists in the container.
func (tc *TestContainer) CheckDirExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-d", path})
	if err != nil {
		return false, fmt.Errorf("failed to check directory: %w", err)
	}
	return exitCode == 0, nil
}

// ExecCommand executes an arbitrary command in the container.
func (tc *TestContainer) ExecCommand(args ...string) (string, int, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, args)
	if err != nil {
		return "", -1, fmt.Errorf("failed to execute command: %w", err)
	}

	rawOutput, err := io.ReadAll(reader)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to read output: %w", err)
	}

	output := demuxDockerOutput(rawOutput)
	return string(output), exitCode, nil
}
