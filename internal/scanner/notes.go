package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"gopkg.in/yaml.v3"
)

// noteFrontmatter captures the stable frontmatter fields that search,
// browse, and inference consume. Fields absent from the document are
// left as zero values; unknown keys are ignored.
type noteFrontmatter struct {
	Title   string   `yaml:"title"`
	Aliases []string `yaml:"aliases"`
	Tags    []string `yaml:"tags"`
	Type    string   `yaml:"type"`
	Status  string   `yaml:"status"`
}

// scanNotes emits NodeNote entries for every markdown file in the
// inventory that is not already owned by a dedicated artifact scanner
// (intents, workflow docs, and festival task files keep their own
// artifact IDs). Frontmatter is parsed into stable metadata and the
// note is attached to its nearest folder scope via a structural
// contains edge.
//
// The pass consumes s.inventory rather than walking the filesystem so
// the inventory remains the single canonical view of repo state.
func (s *Scanner) scanNotes(ctx context.Context, g *graph.Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	inv := s.inventory
	if inv == nil {
		return nil
	}

	for _, e := range inv.Entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e.IsDir {
			continue
		}
		if !isMarkdownExtension(e.Extension) {
			continue
		}
		if !shouldEmitNoteNode(e.RelPath) {
			continue
		}
		if err := s.addNoteNode(g, &e); err != nil {
			return err
		}
	}
	return nil
}

// addNoteNode constructs and attaches one note node for the given
// inventory entry, populating frontmatter metadata from the file.
func (s *Scanner) addNoteNode(g *graph.Graph, e *InventoryEntry) error {
	node := newNoteNode(e.RelPath, e.AbsPath)
	node.Metadata[graph.MetaPathDepth] = itoa(e.PathDepth)
	node.Metadata[graph.MetaRepoRoot] = e.RepoRoot
	if e.GitState != "" {
		node.Metadata[graph.MetaGitState] = string(e.GitState)
	}

	if err := populateFrontmatterMetadata(node, e.AbsPath); err != nil {
		// Frontmatter parse failures must not abort the scan; treat
		// them as missing frontmatter and continue with a bare note.
		node.Metadata[graph.MetaNoteTitle] = titleFromPath(e.RelPath)
		g.AddNode(node)
		return nil
	}

	g.AddNode(node)
	parentID := findExistingFolderAncestor(g, parentFolderRel(e.RelPath))
	if parentID != "" {
		g.AddEdge(graph.NewEdge(parentID, node.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	}
	return nil
}

// populateFrontmatterMetadata reads the file at absPath, parses any YAML
// frontmatter block, and sets stable metadata keys on node. A missing
// frontmatter block is not an error - the function just sets a fallback
// title derived from the filename.
func populateFrontmatterMetadata(node *graph.Node, absPath string) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return graphErrors.Wrapf(err, "read note %q", absPath)
	}
	fm, ok := extractNoteFrontmatter(data)
	if !ok {
		node.Metadata[graph.MetaNoteTitle] = titleFromPath(node.Path)
		return nil
	}
	if fm.Title != "" {
		node.Metadata[graph.MetaNoteTitle] = fm.Title
	} else {
		node.Metadata[graph.MetaNoteTitle] = titleFromPath(node.Path)
	}
	if len(fm.Aliases) > 0 {
		node.Metadata[graph.MetaNoteAliases] = joinCanonical(fm.Aliases)
	}
	if len(fm.Tags) > 0 {
		node.Metadata[graph.MetaNoteTags] = joinCanonical(fm.Tags)
	}
	if fm.Type != "" {
		node.Metadata[graph.MetaNoteType] = fm.Type
	}
	if fm.Status != "" {
		node.Metadata[graph.MetaNoteStatus] = fm.Status
	}
	return nil
}

// extractNoteFrontmatter parses the YAML frontmatter block at the start
// of a markdown file. It returns ok=false when no frontmatter block is
// present so callers can fall back to filename-derived metadata.
func extractNoteFrontmatter(data []byte) (noteFrontmatter, bool) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return noteFrontmatter{}, false
	}
	// Find the closing delimiter on its own line.
	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return noteFrontmatter{}, false
	}
	var fm noteFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		return noteFrontmatter{}, false
	}
	return fm, true
}

// isMarkdownExtension reports whether ext refers to a markdown variant.
func isMarkdownExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case "md", "markdown", "mdx":
		return true
	}
	return false
}

// shouldEmitNoteNode gates which markdown files become note nodes.
// Artifact markdown (intent docs, design docs, explore docs, festival
// task files, and well-known files such as SEQUENCE_GOAL.md) keep their
// artifact IDs; every other markdown file gets a note node.
func shouldEmitNoteNode(rel string) bool {
	rel = filepath.ToSlash(rel)
	// Intent files already become NodeIntent via scanIntents.
	if strings.HasPrefix(rel, ".campaign/intents/") {
		return false
	}
	// Task files inside festival sequence directories (numeric-prefix
	// phase dirs) are already NodeTask entries.
	if strings.HasPrefix(rel, "festivals/") && looksLikeFestivalTaskFile(rel) {
		return false
	}
	// Design and explore top-level directories are owned by their own
	// artifact constructors, but nested markdown files inside them are
	// reasonable note candidates. The artifact scanners emit NodeDesignDoc
	// and NodeExploreDoc for the directory itself, not for its contents.
	return true
}

// looksLikeFestivalTaskFile reports whether rel resembles a festival
// task markdown path such as
// "festivals/active/<fest>/001_BUILD/01_seq/01_task.md".
func looksLikeFestivalTaskFile(rel string) bool {
	parts := strings.Split(rel, "/")
	// Minimum 5 segments: festivals / lifecycle / festival / phase / file.
	if len(parts) < 5 {
		return false
	}
	phase := parts[3]
	if !isPhaseDir(phase) {
		return false
	}
	return strings.HasSuffix(rel, ".md")
}

// titleFromPath returns a readable title derived from the note's
// filename: the final path segment without the extension, with
// underscores and dashes converted to spaces and trimmed.
func titleFromPath(rel string) string {
	base := filepath.Base(rel)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ReplaceAll(name, "_", " ")
	return strings.TrimSpace(name)
}

// joinCanonical returns a comma-delimited, sorted, trimmed join of
// string values suitable for stable metadata storage.
func joinCanonical(values []string) string {
	cleaned := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		cleaned = append(cleaned, v)
	}
	sort.Strings(cleaned)
	return strings.Join(cleaned, ",")
}

// itoa is a tiny helper that avoids pulling strconv into callers that
// use it once. It is duplicated here for readability.
func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
