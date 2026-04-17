package scanner

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// knownCampaignBuckets is the set of top-level directories whose
// semantics are stable enough to warrant stronger scope metadata. Folders
// matching these paths (as the first segment of a relative path) are
// tagged ScopeKindCampaignBucket so later passes can treat them as
// well-known anchors.
var knownCampaignBuckets = map[string]bool{
	"projects":          true,
	"festivals":         true,
	"workflow":          true,
	"workflow/design":   true,
	"workflow/explore":  true,
	".campaign":         true,
	".campaign/intents": true,
}

// bridgeArtifactsToScopes emits structural `contains` edges from each
// artifact node's nearest owning folder scope to the artifact itself.
// This threads projects, festivals, intents, and workflow docs into the
// scope hierarchy without rewriting their IDs.
//
// If no folder scope exists for the immediate parent of an artifact's
// path, the nearest existing ancestor folder is used. The campaign-root
// folder ("folder:.") is always available as a last resort.
func (s *Scanner) bridgeArtifactsToScopes(ctx context.Context, g *graph.Graph) error {
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
		if !isBridgeableArtifact(n.Type) {
			continue
		}
		if n.Path == "" {
			continue
		}
		rel, err := filepath.Rel(root, n.Path)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			continue
		}
		parentRel := parentFolderRel(rel)
		parentID := findExistingFolderAncestor(g, parentRel)
		if parentID == "" {
			continue
		}
		g.AddEdge(graph.NewEdge(parentID, n.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
	}
	return nil
}

// findExistingFolderAncestor walks from rel upward to the campaign root
// and returns the ID of the nearest folder scope node present in g.
// It always terminates because "folder:." is emitted for every scan.
func findExistingFolderAncestor(g *graph.Graph, rel string) string {
	cur := rel
	for {
		id := "folder:" + cur
		if g.Node(id) != nil {
			return id
		}
		if cur == "." || cur == "" {
			return ""
		}
		cur = parentFolderRel(cur)
	}
}

// isBridgeableArtifact reports whether a node type represents a
// campaign artifact that should bridge to the workspace scope graph.
// Phases, sequences, and tasks are omitted because their containment is
// already expressed through the festival artifact hierarchy; bridging
// them to a scope folder would be redundant and noisy.
func isBridgeableArtifact(t graph.NodeType) bool {
	switch t {
	case graph.NodeProject,
		graph.NodeFestival,
		graph.NodeIntent,
		graph.NodeDesignDoc,
		graph.NodeExploreDoc,
		graph.NodeChain:
		return true
	}
	return false
}

// scanScopes creates folder nodes for the campaign root, every repo
// boundary, every known campaign bucket, and every ancestor of an
// inventory entry. Structural `contains` edges connect each folder to
// its parent folder so later passes can traverse scope hierarchy.
//
// scanScopes is idempotent on the graph: it never emits duplicate folder
// nodes because the folder set is keyed on relative path, and contains
// edges only link each folder to its immediate parent.
func (s *Scanner) scanScopes(ctx context.Context, g *graph.Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	inv := s.inventory
	if inv == nil {
		return nil
	}

	// Collect relative directory paths that need folder nodes.
	folders := map[string]*folderSpec{}

	// Campaign root is always present as the anchor folder.
	folders["."] = &folderSpec{
		rel:       ".",
		abs:       inv.Root,
		scopeKind: graph.ScopeKindCampaignRoot,
		repoRoot:  inv.Root,
	}

	// Boundaries produce repo or submodule scope folders.
	for _, b := range inv.Boundaries {
		rel := b.RelPath
		if rel == "." {
			continue // campaign root already recorded
		}
		rel = filepath.ToSlash(rel)
		kind := graph.ScopeKindRepoRoot
		if b.IsSubmodule {
			kind = graph.ScopeKindSubmoduleRoot
		}
		folders[rel] = &folderSpec{
			rel:         rel,
			abs:         b.AbsPath,
			scopeKind:   kind,
			repoRoot:    b.AbsPath,
			isSubmodule: b.IsSubmodule,
		}
		// Emit a separate repo:<rel> slice anchor for nested boundaries.
		// The campaign root does not get a repo node; folder:. serves
		// as the root anchor.
		repoNode := newRepoNode(rel, b.AbsPath)
		repoNode.Metadata[graph.MetaRepoRoot] = b.AbsPath
		repoNode.Metadata[graph.MetaScopeKind] = kind
		repoNode.Metadata[graph.MetaPathDepth] = strconv.Itoa(pathDepth(rel))
		if b.IsSubmodule {
			repoNode.Metadata[graph.MetaIsSubmodule] = "true"
		}
		g.AddNode(repoNode)
	}

	// Ancestors of inventory entries. Only directories that actually
	// have authored content beneath them become folder nodes - we do
	// not invent nodes for every path fragment the walker visits.
	for _, e := range inv.Entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Skip entries whose parent chain we do not want to surface.
		// Directories at rel "." have no parent to register.
		rel := filepath.ToSlash(e.RelPath)
		if rel == "." || rel == "" {
			continue
		}
		for _, anc := range ancestorPaths(rel) {
			if _, ok := folders[anc]; ok {
				continue
			}
			kind := classifyFolderKind(anc, inv)
			folders[anc] = &folderSpec{
				rel:       anc,
				abs:       filepath.Join(inv.Root, filepath.FromSlash(anc)),
				scopeKind: kind,
				repoRoot:  ownerRepoRoot(anc, inv),
			}
		}
	}

	// Emit folder nodes with stable metadata.
	for rel, spec := range folders {
		node := newFolderNode(rel, spec.abs, spec.scopeKind)
		node.Metadata[graph.MetaRepoRoot] = spec.repoRoot
		node.Metadata[graph.MetaPathDepth] = strconv.Itoa(pathDepth(rel))
		if spec.isSubmodule {
			node.Metadata[graph.MetaIsSubmodule] = "true"
		}
		if relToRepo, ok := relativeToRepo(spec.abs, spec.repoRoot); ok {
			node.Metadata[graph.MetaBoundaryRel] = relToRepo
		}
		g.AddNode(node)
	}

	// Emit contains edges between each folder and its parent folder.
	for rel := range folders {
		if rel == "." {
			continue
		}
		parent := parentFolderRel(rel)
		if _, ok := folders[parent]; !ok {
			// Ensure all ancestors are present; if not, register them
			// as user scopes (we already added known kinds above).
			continue
		}
		from := "folder:" + parent
		to := "folder:" + rel
		g.AddEdge(graph.NewEdge(from, to, graph.EdgeContains, 1.0, graph.SourceStructural))
	}

	// Bridge each repo slice anchor to its folder counterpart so
	// navigation works from either node and slicing by repo:<rel>
	// still reaches workspace content.
	for _, b := range inv.Boundaries {
		if b.RelPath == "." {
			continue
		}
		rel := filepath.ToSlash(b.RelPath)
		repoID := "repo:" + rel
		folderID := "folder:" + rel
		if g.Node(repoID) != nil && g.Node(folderID) != nil {
			g.AddEdge(graph.NewEdge(repoID, folderID, graph.EdgeContains, 1.0, graph.SourceStructural))
		}
	}

	return nil
}

// folderSpec holds the metadata needed to build one folder node.
type folderSpec struct {
	rel         string
	abs         string
	scopeKind   string
	repoRoot    string
	isSubmodule bool
}

// ancestorPaths returns the ancestor directories of a relative file path,
// from shallowest to deepest, excluding the root "." and excluding the
// entry path itself. For a file "a/b/c/note.md" the ancestors are
// ["a", "a/b", "a/b/c"]. For a directory entry "a/b/c" the ancestors are
// ["a", "a/b"]; the directory itself is not included.
func ancestorPaths(rel string) []string {
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.Trim(rel, "/")
	if rel == "" || rel == "." {
		return nil
	}
	segments := strings.Split(rel, "/")
	// The last segment is the entry itself (file or dir). Emit
	// ancestors for index [0, len-1).
	var out []string
	for i := 1; i < len(segments); i++ {
		out = append(out, strings.Join(segments[:i], "/"))
	}
	return out
}

// parentFolderRel returns the parent folder relative path. Rel "a" has
// parent ".". Rel "a/b/c" has parent "a/b".
func parentFolderRel(rel string) string {
	rel = strings.Trim(rel, "/")
	if rel == "" || rel == "." {
		return "."
	}
	idx := strings.LastIndex(rel, "/")
	if idx < 0 {
		return "."
	}
	return rel[:idx]
}

// classifyFolderKind inspects the path relative to the campaign root and
// returns the most specific ScopeKind that applies. Boundaries are not
// handled here because they are emitted explicitly above.
func classifyFolderKind(rel string, inv *Inventory) string {
	if knownCampaignBuckets[rel] {
		return graph.ScopeKindCampaignBucket
	}
	// A nested segment may also map to a known bucket; e.g. the folder
	// "projects/alpha" is not a bucket itself but lives under one. Leave
	// such folders as user scopes unless they are explicit buckets.
	if isArtifactOwnedPath(rel, inv) {
		return graph.ScopeKindArtifactScope
	}
	return graph.ScopeKindUserScope
}

// isArtifactOwnedPath reports whether rel lives beneath a campaign
// bucket that the artifact scanners already own. For the first
// implementation, treat any descendant of a known bucket as an artifact
// scope.
func isArtifactOwnedPath(rel string, _ *Inventory) bool {
	for bucket := range knownCampaignBuckets {
		if rel == bucket {
			return false
		}
		if strings.HasPrefix(rel, bucket+"/") {
			return true
		}
	}
	return false
}

// ownerRepoRoot returns the absolute repo-root path that owns rel. When
// rel lies under a nested boundary, that boundary's AbsPath is returned;
// otherwise the campaign root is returned.
func ownerRepoRoot(rel string, inv *Inventory) string {
	abs := filepath.Join(inv.Root, filepath.FromSlash(rel))
	if b := inv.BoundaryFor(abs); b != nil {
		return b.AbsPath
	}
	return inv.Root
}

// relativeToRepo returns abs relative to repoRoot in forward-slash form.
// Returns false when abs is not under repoRoot.
func relativeToRepo(abs, repoRoot string) (string, bool) {
	if repoRoot == "" {
		return "", false
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return "", false
	}
	return rel, true
}
