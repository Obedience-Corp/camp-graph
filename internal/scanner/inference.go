package scanner

import (
	"sort"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// CandidateSource identifies which posting list produced a candidate
// pair. Sources are serialized into evidence reasons so later passes
// can weight them and explain why an inferred edge exists.
type CandidateSource string

const (
	// CandidateSameFolder: two nodes share an immediate folder ancestor.
	CandidateSameFolder CandidateSource = "same_folder"
	// CandidateSharedAncestor: two nodes share a non-immediate ancestor.
	CandidateSharedAncestor CandidateSource = "shared_ancestor"
	// CandidateSharedTag: two nodes reference the same tag.
	CandidateSharedTag CandidateSource = "shared_tag"
	// CandidateSharedFrontmatter: two nodes have an identical
	// frontmatter key/value pair (type, status, or normalized alias).
	CandidateSharedFrontmatter CandidateSource = "shared_frontmatter"
	// CandidateSharedRepoRoot: two nodes belong to the same repo root.
	CandidateSharedRepoRoot CandidateSource = "shared_repo_root"
)

// CandidatePair represents one potential inferred relationship sourced
// from a single posting list. Many pairs for the same (a, b) may be
// collected; inference aggregation is responsible for merging them.
type CandidatePair struct {
	FromID string
	ToID   string
	Source CandidateSource
	// Value is the shared value that brought the pair together (folder
	// path, tag name, frontmatter "key=value"). Stored so later
	// evidence serialization can record why the candidate was seen.
	Value string
}

// CandidateBudget caps how much work inference does per run. Exceeding
// a cap cuts off enumeration instead of growing unbounded. Zero values
// mean "use built-in defaults".
type CandidateBudget struct {
	// MaxMembersPerGroup caps the number of nodes considered per
	// posting-list group. Groups with more members are truncated; this
	// keeps very large scopes from producing quadratic candidate
	// pairs.
	MaxMembersPerGroup int
	// MaxPairs caps the overall number of emitted pairs. When reached,
	// GenerateCandidates returns the partial result with a Truncated
	// flag set.
	MaxPairs int
}

// CandidateSet is the result of candidate generation plus a
// bookkeeping flag that reports whether any budget was exceeded.
type CandidateSet struct {
	Pairs     []CandidatePair
	Truncated bool
	// Counts per source for telemetry and tests. Keys use
	// CandidateSource string values.
	CountsBySource map[CandidateSource]int
}

// DefaultCandidateBudget returns a conservative budget used when
// callers pass a zero-valued CandidateBudget. Values were chosen to
// keep inference bounded on large vaults without being so tight that
// small campaigns lose useful signals.
func DefaultCandidateBudget() CandidateBudget {
	return CandidateBudget{
		MaxMembersPerGroup: 128,
		MaxPairs:           5000,
	}
}

// GenerateCandidates builds bounded candidate pairs from posting lists
// over the graph. The returned pairs are keyed by (FromID, ToID) with
// Source describing the posting list that produced them.
//
// Candidate generation intentionally avoids global all-pairs
// comparison: pairs are only emitted inside posting-list groups, and
// each group is further limited by budget.MaxMembersPerGroup so the
// worst-case cost stays proportional to the sum of (group_size^2)
// rather than (total_nodes^2).
func GenerateCandidates(g *graph.Graph, budget CandidateBudget) *CandidateSet {
	if budget.MaxMembersPerGroup <= 0 {
		budget.MaxMembersPerGroup = DefaultCandidateBudget().MaxMembersPerGroup
	}
	if budget.MaxPairs <= 0 {
		budget.MaxPairs = DefaultCandidateBudget().MaxPairs
	}

	out := &CandidateSet{
		CountsBySource: make(map[CandidateSource]int),
	}

	// Emit folder-based candidates by walking contains edges from
	// folder nodes to their immediate note/canvas/attachment children.
	emitFolderCandidates(g, out, budget)

	// Emit shared-ancestor candidates: nodes that share any non-immediate
	// folder ancestor. We reuse the contains-edge walk.
	emitSharedAncestorCandidates(g, out, budget)

	// Emit tag posting-list candidates. Notes that reference the same
	// tag belong to the same posting list.
	emitTagCandidates(g, out, budget)

	// Emit frontmatter posting-list candidates using type, status, and
	// aliases metadata already extracted on note nodes.
	emitFrontmatterCandidates(g, out, budget)

	// Emit repo-root posting-list candidates so content inside the same
	// nested repo has at least a weak affinity signal.
	emitRepoRootCandidates(g, out, budget)

	return out
}

// emitFolderCandidates enumerates pairs within each folder scope using
// the existing folder->child contains edges.
func emitFolderCandidates(g *graph.Graph, out *CandidateSet, budget CandidateBudget) {
	groups := groupNotesByImmediateFolder(g)
	for folder, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		emitPairsFromGroup(ids, CandidateSameFolder, folder, budget, out)
		if out.Truncated {
			return
		}
	}
}

// emitSharedAncestorCandidates enumerates pairs for every non-immediate
// ancestor scope. Pairs already emitted as same-folder are deduplicated
// by the caller (CandidateSet tolerates duplicates per source).
func emitSharedAncestorCandidates(g *graph.Graph, out *CandidateSet, budget CandidateBudget) {
	byAncestor := groupNotesByAncestor(g)
	for ancestor, ids := range byAncestor {
		if len(ids) < 2 {
			continue
		}
		emitPairsFromGroup(ids, CandidateSharedAncestor, ancestor, budget, out)
		if out.Truncated {
			return
		}
	}
}

// emitTagCandidates enumerates pairs for notes that reference the same
// tag via inline references or frontmatter tags.
func emitTagCandidates(g *graph.Graph, out *CandidateSet, budget CandidateBudget) {
	groups := groupNotesByTag(g)
	for tag, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		emitPairsFromGroup(ids, CandidateSharedTag, tag, budget, out)
		if out.Truncated {
			return
		}
	}
}

// emitFrontmatterCandidates groups notes by frontmatter type/status and
// alias tokens to produce candidate pairs.
func emitFrontmatterCandidates(g *graph.Graph, out *CandidateSet, budget CandidateBudget) {
	groupings := groupNotesByFrontmatter(g)
	for value, ids := range groupings {
		if len(ids) < 2 {
			continue
		}
		emitPairsFromGroup(ids, CandidateSharedFrontmatter, value, budget, out)
		if out.Truncated {
			return
		}
	}
}

// emitRepoRootCandidates emits a weaker affinity signal that couples
// nodes belonging to the same repo root. Membership uses the metadata
// recorded on folder and note nodes.
func emitRepoRootCandidates(g *graph.Graph, out *CandidateSet, budget CandidateBudget) {
	groups := groupNotesByRepoRoot(g)
	for root, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		emitPairsFromGroup(ids, CandidateSharedRepoRoot, root, budget, out)
		if out.Truncated {
			return
		}
	}
}

// emitPairsFromGroup enumerates all unordered pairs inside ids and
// appends them to out, respecting both MaxMembersPerGroup and MaxPairs.
func emitPairsFromGroup(ids []string, source CandidateSource, value string, budget CandidateBudget, out *CandidateSet) {
	if len(ids) > budget.MaxMembersPerGroup {
		ids = ids[:budget.MaxMembersPerGroup]
		out.Truncated = true
	}
	sort.Strings(ids)
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if len(out.Pairs) >= budget.MaxPairs {
				out.Truncated = true
				return
			}
			out.Pairs = append(out.Pairs, CandidatePair{
				FromID: ids[i],
				ToID:   ids[j],
				Source: source,
				Value:  value,
			})
			out.CountsBySource[source]++
		}
	}
}

// inferenceCandidateTypes lists the node types that participate in
// candidate generation. Artifact nodes (project, festival, etc.) are
// excluded because their relationships are already explicit.
func inferenceCandidateTypes() map[graph.NodeType]bool {
	return map[graph.NodeType]bool{
		graph.NodeNote:       true,
		graph.NodeCanvas:     true,
		graph.NodeAttachment: true,
	}
}

// groupNotesByImmediateFolder returns a map from folder rel-path to
// the set of note/canvas/attachment IDs whose immediate parent folder
// matches.
func groupNotesByImmediateFolder(g *graph.Graph) map[string][]string {
	candidates := inferenceCandidateTypes()
	groups := map[string][]string{}
	for _, n := range g.Nodes() {
		if !candidates[n.Type] {
			continue
		}
		parentRel := parentFolderRel(n.Name)
		if parentRel == "" {
			continue
		}
		groups[parentRel] = append(groups[parentRel], n.ID)
	}
	return groups
}

// groupNotesByAncestor returns a map from ancestor rel-path to note
// IDs contained beneath that ancestor (excluding the immediate
// parent, which groupNotesByImmediateFolder already covers).
func groupNotesByAncestor(g *graph.Graph) map[string][]string {
	candidates := inferenceCandidateTypes()
	groups := map[string][]string{}
	for _, n := range g.Nodes() {
		if !candidates[n.Type] {
			continue
		}
		ancestors := ancestorPaths(n.Name)
		// Exclude immediate parent; that group is produced elsewhere.
		if len(ancestors) == 0 {
			continue
		}
		for _, anc := range ancestors[:len(ancestors)-1] {
			groups[anc] = append(groups[anc], n.ID)
		}
	}
	return groups
}

// groupNotesByTag returns a map from normalized tag name to the notes
// that reference the tag. References include both frontmatter tags
// (MetaNoteTags) and inline references via EdgeReferences to
// NodeTag nodes.
func groupNotesByTag(g *graph.Graph) map[string][]string {
	groups := map[string][]string{}
	candidates := inferenceCandidateTypes()
	// Frontmatter tags.
	for _, n := range g.Nodes() {
		if !candidates[n.Type] {
			continue
		}
		if raw := n.Metadata[graph.MetaNoteTags]; raw != "" {
			for _, t := range splitCanonical(raw) {
				groups[strings.ToLower(t)] = append(groups[strings.ToLower(t)], n.ID)
			}
		}
	}
	// Inline references.
	for _, e := range g.Edges() {
		if e.Type != graph.EdgeReferences {
			continue
		}
		if !strings.HasPrefix(e.ToID, "tag:") {
			continue
		}
		tag := strings.TrimPrefix(e.ToID, "tag:")
		// Only count nodes that are valid candidates.
		if n := g.Node(e.FromID); n != nil && candidates[n.Type] {
			groups[tag] = append(groups[tag], e.FromID)
		}
	}
	// Deduplicate IDs within each group.
	for tag, ids := range groups {
		groups[tag] = dedupeStrings(ids)
	}
	return groups
}

// groupNotesByFrontmatter groups notes by their frontmatter type,
// status, and alias values. The group key encodes which field was
// shared so evidence strings remain informative.
func groupNotesByFrontmatter(g *graph.Graph) map[string][]string {
	groups := map[string][]string{}
	candidates := inferenceCandidateTypes()
	for _, n := range g.Nodes() {
		if !candidates[n.Type] {
			continue
		}
		if t := n.Metadata[graph.MetaNoteType]; t != "" {
			k := "type=" + t
			groups[k] = append(groups[k], n.ID)
		}
		if s := n.Metadata[graph.MetaNoteStatus]; s != "" {
			k := "status=" + s
			groups[k] = append(groups[k], n.ID)
		}
		if raw := n.Metadata[graph.MetaNoteAliases]; raw != "" {
			for _, a := range splitCanonical(raw) {
				if a == "" {
					continue
				}
				k := "alias=" + strings.ToLower(a)
				groups[k] = append(groups[k], n.ID)
			}
		}
	}
	return groups
}

// groupNotesByRepoRoot groups candidate nodes by the absolute repo-root
// path recorded in MetaRepoRoot.
func groupNotesByRepoRoot(g *graph.Graph) map[string][]string {
	groups := map[string][]string{}
	candidates := inferenceCandidateTypes()
	for _, n := range g.Nodes() {
		if !candidates[n.Type] {
			continue
		}
		root := n.Metadata[graph.MetaRepoRoot]
		if root == "" {
			continue
		}
		groups[root] = append(groups[root], n.ID)
	}
	return groups
}

// splitCanonical reverses joinCanonical. It accepts a comma-delimited
// metadata string and returns the trimmed components.
func splitCanonical(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// dedupeStrings returns the input slice with duplicate strings removed.
// Order is stable: the first occurrence wins.
func dedupeStrings(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

