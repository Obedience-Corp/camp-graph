package tui

import (
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// collectScopeAnchors selects the scope nodes that form the top-level
// browse list: campaign root, repo/submodule roots, and campaign-bucket
// folders. Ordered by scope kind priority then path length.
func collectScopeAnchors(g *graph.Graph) []*graph.Node {
	priority := map[string]int{
		graph.ScopeKindCampaignRoot:   0,
		graph.ScopeKindRepoRoot:       1,
		graph.ScopeKindSubmoduleRoot:  1,
		graph.ScopeKindCampaignBucket: 2,
		graph.ScopeKindArtifactScope:  3,
		graph.ScopeKindUserScope:      4,
	}
	var anchors []*graph.Node
	for _, n := range g.Nodes() {
		if n.Type != graph.NodeFolder {
			continue
		}
		kind := n.Metadata[graph.MetaScopeKind]
		if _, ok := priority[kind]; !ok {
			continue
		}
		// Only include depth-0 and depth-1 scopes as top-level anchors
		// so the initial view stays compact. Users widen by pressing
		// enter on a scope to descend into the neighborhood.
		depth := n.Metadata[graph.MetaPathDepth]
		if depth == "" {
			continue
		}
		if depth != "0" && depth != "1" && kind != graph.ScopeKindRepoRoot && kind != graph.ScopeKindSubmoduleRoot {
			// Still include sub-bucket anchors like workflow/design.
			if kind != graph.ScopeKindCampaignBucket {
				continue
			}
		}
		anchors = append(anchors, n)
	}
	// Stable ordering: kind priority then path depth then name.
	sortByScopePriority(anchors, priority)
	return anchors
}

func sortByScopePriority(nodes []*graph.Node, priority map[string]int) {
	byPriority := func(a, b *graph.Node) bool {
		ap := priority[a.Metadata[graph.MetaScopeKind]]
		bp := priority[b.Metadata[graph.MetaScopeKind]]
		if ap != bp {
			return ap < bp
		}
		ad := a.Metadata[graph.MetaPathDepth]
		bd := b.Metadata[graph.MetaPathDepth]
		if ad != bd {
			return ad < bd
		}
		return a.Name < b.Name
	}
	// Simple insertion sort to avoid a sort import if not already
	// present.
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && byPriority(nodes[j], nodes[j-1]); j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}
