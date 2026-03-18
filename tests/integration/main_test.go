//go:build integration
// +build integration

package integration

import (
	"os"
	"testing"
)

// sharedContainer is the package-level container reused across all tests.
// It is initialized once in TestMain and cleaned up after all tests complete.
var sharedContainer *TestContainer

// TestMain sets up a shared container for all integration tests.
func TestMain(m *testing.M) {
	var err error
	sharedContainer, err = NewSharedContainer()
	if err != nil {
		os.Stderr.WriteString("Failed to create shared container: " + err.Error() + "\n")
		os.Exit(1)
	}

	code := m.Run()

	sharedContainer.Cleanup()
	os.Exit(code)
}

// GetSharedContainer returns the shared container, resetting state first.
func GetSharedContainer(t *testing.T) *TestContainer {
	t.Helper()
	if sharedContainer == nil {
		t.Fatal("shared container not initialized - TestMain not called?")
	}

	if err := sharedContainer.Reset(); err != nil {
		t.Fatalf("failed to reset container: %v", err)
	}

	return &TestContainer{
		container: sharedContainer.container,
		ctx:       sharedContainer.ctx,
		t:         t,
	}
}
