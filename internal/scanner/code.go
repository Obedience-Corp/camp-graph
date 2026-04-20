package scanner

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// codeExtensions is the set of file extensions that extractCodeSlices
// promotes to NodeFile. Additional extensions can be added here but
// the pass intentionally stays narrow: it extracts nodes, not a
// language-aware syntax tree.
var codeExtensions = map[string]string{
	"go":   "go",
	"ts":   "typescript",
	"tsx":  "typescript",
	"js":   "javascript",
	"jsx":  "javascript",
	"py":   "python",
	"rs":   "rust",
	"rb":   "ruby",
	"java": "java",
	"c":    "c",
	"h":    "c",
	"cc":   "cpp",
	"cpp":  "cpp",
	"hpp":  "cpp",
	"md":   "", // excluded via empty string
}

// goPackageRe extracts the `package foo` declaration from the first
// 32 lines of a Go source file so code slices carry package-level
// grouping without pulling in go/ast.
var goPackageRe = regexp.MustCompile(`^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)`)

// extractCodeSlices emits NodeFile entries (and NodePackage nodes
// for Go source files) for every eligible inventory entry inside a
// nested repo boundary. Campaign-root files are skipped because code
// slices are intentionally scoped to nested repos.
//
// Files are bridged to their owning folder scope via structural
// contains edges and carry MetaRepoRoot so consumers can re-join the
// slice with its anchor.
func (s *Scanner) extractCodeSlices(ctx context.Context, g *graph.Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	inv := s.inventory
	if inv == nil {
		return nil
	}

	// Build a quick lookup of nested-boundary roots so we skip the
	// campaign-root boundary which would otherwise flood the graph
	// with code from every project.
	nestedBoundary := map[string]bool{}
	for _, b := range inv.Boundaries {
		if b.IsRoot {
			continue
		}
		nestedBoundary[b.AbsPath] = true
	}

	goPackages := map[string]string{} // folder rel -> package name

	for _, e := range inv.Entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e.IsDir {
			continue
		}
		if !nestedBoundary[e.RepoRoot] {
			continue
		}
		lang, ok := codeExtensions[e.Extension]
		if !ok || lang == "" {
			continue
		}
		// Emit file node.
		fileID := "file:" + e.RelPath
		fileNode := graph.NewNode(fileID, graph.NodeFile, e.RelPath, e.AbsPath)
		fileNode.Metadata[graph.MetaRepoRoot] = e.RepoRoot
		fileNode.Metadata[graph.MetaPathDepth] = itoa(e.PathDepth)
		fileNode.Metadata[graph.MetaGitState] = string(e.GitState)
		fileNode.Metadata["language"] = lang
		g.AddNode(fileNode)

		parentID := findExistingFolderAncestor(g, parentFolderRel(e.RelPath))
		if parentID != "" {
			g.AddEdge(graph.NewEdge(parentID, fileID, graph.EdgeContains, 1.0, graph.SourceStructural))
		}

		// Go package detection without go/ast so the extractor stays
		// dependency-free. Only read the first 32 lines.
		if lang == "go" {
			if pkg := firstGoPackage(e.AbsPath); pkg != "" {
				pkgKey := parentFolderRel(e.RelPath) + "#" + pkg
				if _, seen := goPackages[pkgKey]; !seen {
					goPackages[pkgKey] = pkg
					pkgID := "package:" + parentFolderRel(e.RelPath) + ":" + pkg
					pkgNode := graph.NewNode(pkgID, graph.NodePackage, pkg, filepath.Dir(e.AbsPath))
					pkgNode.Metadata[graph.MetaRepoRoot] = e.RepoRoot
					pkgNode.Metadata["language"] = lang
					g.AddNode(pkgNode)
					if parentID != "" {
						g.AddEdge(graph.NewEdge(parentID, pkgID, graph.EdgeContains, 1.0, graph.SourceStructural))
					}
					g.AddEdge(graph.NewEdge(pkgID, fileID, graph.EdgeContains, 1.0, graph.SourceStructural))
				} else {
					pkgID := "package:" + parentFolderRel(e.RelPath) + ":" + pkg
					g.AddEdge(graph.NewEdge(pkgID, fileID, graph.EdgeContains, 1.0, graph.SourceStructural))
				}
			}
		}
	}
	return nil
}

// firstGoPackage returns the package name declared at the top of the
// Go file at path, or "" if the declaration cannot be found in the
// first 32 lines. Uses a simple line scan rather than go/parser so
// the scanner stays bounded and dependency-free.
func firstGoPackage(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for i := 0; i < 32 && scan.Scan(); i++ {
		line := scan.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if m := goPackageRe.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}
