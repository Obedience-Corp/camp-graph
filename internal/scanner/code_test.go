package scanner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func TestCodeSlices_NestedRepoOnly(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Campaign-root Go file should NOT become a file node (code
	// extraction is scoped to nested repos only).
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, "tooling/main.go"), "package main\n\nfunc main() {}\n")

	// Nested repo with a Go file: should emit NodeFile + NodePackage.
	nested := filepath.Join(root, "projects/nested-tool")
	mkdirAll(t, filepath.Join(nested, ".git"))
	writeFile(t, filepath.Join(nested, "cmd/main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(nested, "internal/util/util.go"), "package util\n\nfunc Helper() {}\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// Files at campaign root should NOT be NodeFile.
	if g.Node("file:tooling/main.go") != nil {
		t.Error("campaign-root main.go should not be promoted to NodeFile")
	}

	// Files inside nested repo should be NodeFile.
	if g.Node("file:projects/nested-tool/cmd/main.go") == nil {
		t.Error("nested cmd/main.go missing as NodeFile")
	}
	if g.Node("file:projects/nested-tool/internal/util/util.go") == nil {
		t.Error("nested util/util.go missing as NodeFile")
	}

	// Package detection.
	foundMain := false
	foundUtil := false
	for _, n := range g.Nodes() {
		if n.Type != graph.NodePackage {
			continue
		}
		if strings.Contains(n.ID, "projects/nested-tool/cmd") && n.Name == "main" {
			foundMain = true
		}
		if strings.Contains(n.ID, "projects/nested-tool/internal/util") && n.Name == "util" {
			foundUtil = true
		}
	}
	if !foundMain {
		t.Error("expected NodePackage main in nested cmd/")
	}
	if !foundUtil {
		t.Error("expected NodePackage util in nested internal/util/")
	}
}

func TestCodeSlices_RecordsRepoRootMetadata(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "projects/subrepo")
	mkdirAll(t, filepath.Join(nested, ".git"))
	writeFile(t, filepath.Join(nested, "main.go"), "package main\nfunc main(){}\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	f := g.Node("file:projects/subrepo/main.go")
	if f == nil {
		t.Fatal("file node missing")
	}
	if f.Metadata[graph.MetaRepoRoot] != nested {
		t.Errorf("repo_root metadata: got %q, want %q",
			f.Metadata[graph.MetaRepoRoot], nested)
	}
	if f.Metadata["language"] != "go" {
		t.Errorf("language metadata: got %q, want go", f.Metadata["language"])
	}
}

func TestCodeSlices_NonCodeFilesSkipped(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	mkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "projects/mixed")
	mkdirAll(t, filepath.Join(nested, ".git"))
	// README.md should not become a NodeFile (it becomes a note).
	writeFile(t, filepath.Join(nested, "README.md"), "# readme\n")
	writeFile(t, filepath.Join(nested, "main.go"), "package main\nfunc main(){}\n")

	s := scanner.New(root)
	s.SetInventoryOptions(scanner.InventoryOptions{GitProbe: &scanner.StaticGitProbe{}})
	g, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if g.Node("file:projects/mixed/README.md") != nil {
		t.Error("README.md should not be a NodeFile")
	}
	if g.Node("note:projects/mixed/README.md") == nil {
		t.Error("README.md should still be a note")
	}
	if g.Node("file:projects/mixed/main.go") == nil {
		t.Error("main.go should be a NodeFile")
	}
}
