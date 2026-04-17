package scanner

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Link and tag regular expressions. They are intentionally narrow and
// anchored to observed formats rather than ambitious heuristics so the
// parsers stay explicit.
var (
	// markdownLinkRe matches an inline markdown link [text](target).
	// Text may be empty. Target may not contain whitespace or ")".
	// Images of the form ![alt](target) are captured separately.
	markdownLinkRe = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)\s]+)\)`)
	// wikiLinkRe matches [[target]] and [[target|alias]] forms. Target
	// may contain slashes, dots, and spaces. Aliases and section anchors
	// ("|alias", "#heading") are discarded from the target portion.
	wikiLinkRe = regexp.MustCompile(`\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|[^\]]+)?\]\]`)
	// tagRe matches inline #tag tokens at word boundaries. It excludes
	// colors (#abcdef), headings (# title), and URL anchors by requiring
	// the character preceding # to be non-word and the tag to have at
	// least one alphabetic character.
	tagRe = regexp.MustCompile(`(?:^|[\s(,;])#([A-Za-z][A-Za-z0-9_\-/]*)`)
)

// extractExplicitLinks walks every note node in the graph and parses
// its content for markdown links, wiki links, image/attachment embeds,
// and tags. Discovered links produce explicit edges; discovered tags
// produce NodeTag nodes with references edges.
//
// The pass consumes the nodes emitted by scanNotes rather than walking
// the filesystem a second time.
func (s *Scanner) extractExplicitLinks(ctx context.Context, g *graph.Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.inventory == nil {
		return nil
	}
	root := s.inventory.Root

	for _, n := range g.Nodes() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if n.Type != graph.NodeNote {
			continue
		}
		if err := s.extractLinksFromNote(g, root, n); err != nil {
			return err
		}
	}

	// After notes are processed, look for canvas files among the
	// inventory entries and parse them.
	for _, e := range s.inventory.Entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e.IsDir {
			continue
		}
		if !isCanvasExtension(e.Extension) {
			continue
		}
		if err := s.extractCanvasLinks(g, &e); err != nil {
			return err
		}
	}
	return nil
}

// extractLinksFromNote parses one note file and emits link, embed, and
// tag edges into g.
func (s *Scanner) extractLinksFromNote(g *graph.Graph, root string, note *graph.Node) error {
	data, err := os.ReadFile(note.Path)
	if err != nil {
		// Missing or unreadable notes should not halt the whole scan;
		// skip silently because the note node still exists.
		return nil
	}
	// Trim leading frontmatter so tag regex does not match tags inside
	// YAML (those are captured via Node.Metadata already).
	body := stripLeadingFrontmatter(string(data))

	s.emitMarkdownLinks(g, root, note, body)
	s.emitWikiLinks(g, note, body)
	s.emitTags(g, note, body)
	return nil
}

// emitMarkdownLinks finds [text](target) and ![alt](target) occurrences
// and routes them to either explicit link edges (markdown target in
// worktree) or attachment edges (non-markdown resource).
func (s *Scanner) emitMarkdownLinks(g *graph.Graph, root string, note *graph.Node, body string) {
	matches := markdownLinkRe.FindAllStringSubmatchIndex(body, -1)
	for _, m := range matches {
		full := body[m[0]:m[1]]
		isEmbed := strings.HasPrefix(full, "!")
		target := body[m[4]:m[5]]
		target = strings.TrimSpace(target)
		if target == "" || isExternalURL(target) {
			continue
		}
		// Strip section anchors and query strings.
		if idx := strings.IndexAny(target, "#?"); idx >= 0 {
			target = target[:idx]
		}
		if target == "" {
			continue
		}
		// Decode URL-encoded targets (e.g. spaces -> %20).
		if decoded, err := url.PathUnescape(target); err == nil {
			target = decoded
		}
		resolvedRel, ok := resolveRelativeToRoot(root, note.Path, target)
		if !ok {
			continue
		}
		subtype := "markdown_link"
		edgeType := graph.EdgeLinksTo
		if isEmbed {
			subtype = "embed"
		}
		toID, _ := resolveTargetID(g, resolvedRel, isEmbed)
		if toID == "" {
			// Create a synthetic attachment node when the target is a
			// non-markdown resource we have not seen as an artifact or
			// note. This keeps explicit edges meaningful.
			if !isMarkdownExtension(extensionOf(resolvedRel)) {
				attNode := newAttachmentNode(resolvedRel, filepath.Join(root, filepath.FromSlash(resolvedRel)))
				g.AddNode(attNode)
				toID = attNode.ID
				edgeType = graph.EdgeLinksTo
				subtype = "attachment"
			} else {
				continue
			}
		}
		edge := graph.NewEdge(note.ID, toID, edgeType, 1.0, graph.SourceExplicit)
		edge.Subtype = subtype
		g.AddEdge(edge)
	}
}

// emitWikiLinks finds [[target]] occurrences and emits explicit
// wiki_link edges if a note with the matching basename exists.
func (s *Scanner) emitWikiLinks(g *graph.Graph, note *graph.Node, body string) {
	matches := wikiLinkRe.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		target := strings.TrimSpace(m[1])
		if target == "" {
			continue
		}
		toID := resolveWikiTargetID(g, target)
		if toID == "" {
			continue
		}
		edge := graph.NewEdge(note.ID, toID, graph.EdgeLinksTo, 1.0, graph.SourceExplicit)
		edge.Subtype = "wiki_link"
		g.AddEdge(edge)
	}
}

// emitTags creates tag nodes and references edges from the note to the
// tag for every inline #tag found in the body.
func (s *Scanner) emitTags(g *graph.Graph, note *graph.Node, body string) {
	seen := map[string]bool{}
	matches := tagRe.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		tag := strings.ToLower(strings.TrimSpace(m[1]))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tagID := "tag:" + tag
		if g.Node(tagID) == nil {
			g.AddNode(newTagNode(tag))
		}
		edge := graph.NewEdge(note.ID, tagID, graph.EdgeReferences, 1.0, graph.SourceExplicit)
		edge.Subtype = "inline_tag"
		g.AddEdge(edge)
	}
}

// extractCanvasLinks parses an Obsidian-style canvas JSON file and
// emits canvas_link edges from the canvas node to every referenced
// note or file.
//
// The file format has a stable shape:
//
//	{
//	  "nodes": [
//	    {"id":"...", "type":"file", "file":"Notes/Plan.md"},
//	    {"id":"...", "type":"text", "text":"..."}
//	  ],
//	  "edges": [...]
//	}
//
// Only "file" nodes produce canvas_link edges; text/group nodes are
// ignored because they do not reference other workspace content.
func (s *Scanner) extractCanvasLinks(g *graph.Graph, e *InventoryEntry) error {
	data, err := os.ReadFile(e.AbsPath)
	if err != nil {
		return graphErrors.Wrapf(err, "read canvas %q", e.AbsPath)
	}
	var parsed canvasFile
	if jerr := json.Unmarshal(data, &parsed); jerr != nil {
		// Malformed canvas should not fail the scan; fall through and
		// still emit a canvas node without link edges.
		return nil
	}
	canvasNode := newCanvasNode(e.RelPath, e.AbsPath)
	g.AddNode(canvasNode)

	// Attach to folder scope so browse can navigate to it.
	parentID := findExistingFolderAncestor(g, parentFolderRel(e.RelPath))
	if parentID != "" {
		g.AddEdge(graph.NewEdge(parentID, canvasNode.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	}

	for _, cn := range parsed.Nodes {
		if cn.Type != "file" || cn.File == "" {
			continue
		}
		targetRel := filepath.ToSlash(filepath.Clean(cn.File))
		toID, ok := resolveTargetID(g, targetRel, false)
		if !ok || toID == "" {
			continue
		}
		edge := graph.NewEdge(canvasNode.ID, toID, graph.EdgeLinksTo, 1.0, graph.SourceExplicit)
		edge.Subtype = "canvas_link"
		g.AddEdge(edge)
	}
	return nil
}

// canvasFile reflects the subset of the Obsidian canvas JSON schema we
// rely on. Unknown fields are ignored.
type canvasFile struct {
	Nodes []canvasNode `json:"nodes"`
}

type canvasNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	File string `json:"file"`
}

// isExternalURL reports whether target has a scheme that makes it a
// remote reference rather than a workspace path.
func isExternalURL(target string) bool {
	for _, prefix := range []string{"http://", "https://", "mailto:", "tel:", "ftp://"} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

// resolveRelativeToRoot resolves a link target that lives in the same
// repository as fromNotePath. The target is joined against the note's
// directory, cleaned, and converted to a forward-slashed path relative
// to root. Absolute-style links (starting with "/") are anchored at the
// campaign root directly.
func resolveRelativeToRoot(root, fromNotePath, target string) (string, bool) {
	var joined string
	if strings.HasPrefix(target, "/") {
		joined = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(target, "/")))
	} else {
		joined = filepath.Join(filepath.Dir(fromNotePath), filepath.FromSlash(target))
	}
	rel, err := filepath.Rel(root, filepath.Clean(joined))
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return "", false
	}
	return rel, true
}

// resolveTargetID returns the graph node ID that matches the given
// relative path. It prefers an explicit node match (note: or canvas:)
// before inferring that the target is an attachment.
func resolveTargetID(g *graph.Graph, rel string, _ bool) (string, bool) {
	noteID := "note:" + rel
	if g.Node(noteID) != nil {
		return noteID, true
	}
	canvasID := "canvas:" + rel
	if g.Node(canvasID) != nil {
		return canvasID, true
	}
	attID := "attachment:" + rel
	if g.Node(attID) != nil {
		return attID, true
	}
	return "", false
}

// resolveWikiTargetID tries to match a wiki-link target (which may lack
// directory context or extension) to a note node. Lookup order:
//  1. exact rel path match
//  2. basename + .md match against every note node
//  3. basename match ignoring extension
func resolveWikiTargetID(g *graph.Graph, target string) string {
	candidates := []string{
		"note:" + target,
		"note:" + target + ".md",
	}
	for _, id := range candidates {
		if g.Node(id) != nil {
			return id
		}
	}
	// Fallback: basename scan.
	targetBase := strings.TrimSuffix(path.Base(target), path.Ext(target))
	for _, n := range g.Nodes() {
		if n.Type != graph.NodeNote {
			continue
		}
		baseName := strings.TrimSuffix(path.Base(n.Path), path.Ext(n.Path))
		if strings.EqualFold(baseName, targetBase) {
			return n.ID
		}
	}
	return ""
}

// stripLeadingFrontmatter removes a YAML frontmatter block at the
// beginning of content so link and tag scanners do not re-consume it.
func stripLeadingFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	return rest[idx+4:]
}

// extensionOf returns the lower-cased extension of rel without the dot.
func extensionOf(rel string) string {
	ext := filepath.Ext(rel)
	if ext == "" {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// isCanvasExtension reports whether ext indicates an Obsidian-style
// canvas file.
func isCanvasExtension(ext string) bool {
	return strings.EqualFold(ext, "canvas")
}
