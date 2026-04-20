package tui

import (
	"context"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// filterAnchors returns anchors filtered by UI chip/scope state.
// When all filters are at their default, anchors is returned
// unchanged. NodeType is matched via string(Node.Type); tracked state
// reads Node.Metadata[MetaTrackedState]; scope is a path-prefix match
// against Node.Path.
func filterAnchors(anchors []*graph.Node, typeChip, trackedChip, scope string) []*graph.Node {
	if typeChip == "" && trackedChip == "" && scope == "" {
		return anchors
	}
	out := make([]*graph.Node, 0, len(anchors))
	for _, n := range anchors {
		if typeChip != "" && string(n.Type) != typeChip {
			continue
		}
		if trackedChip == "Tracked only" && n.Metadata["tracked_state"] != "tracked" {
			continue
		}
		if trackedChip == "Untracked only" && n.Metadata["tracked_state"] != "untracked" {
			continue
		}
		if scope != "" && !scopeMatches(n, scope) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func scopeMatches(n *graph.Node, scope string) bool {
	return n.Path == scope || strings.HasPrefix(n.Path, scope+"/")
}

// chipTypeValue returns the currently selected NodeType chip value, or
// "" when no chip is set. Chip UI lands in sequence 03; this shim
// returns "" so sequence 02 can wire filterAnchors without a circular
// dependency on chip state.
func chipTypeValue(Model) string { return "" }

// chipTrackedValue returns the currently selected tracked-state chip
// value, or "" when no chip is set. See chipTypeValue for the
// sequencing note.
func chipTrackedValue(Model) string { return "" }

// resultGroup buckets search results that share a NodeType.
type resultGroup struct {
	Type     string
	Expanded bool
	Rows     []search.QueryResult
}

// typePriority returns the display-order rank for a NodeType. Lower
// rank sorts earlier; unknown types sort to the end. Ties break
// alphabetically.
func typePriority(t string) int {
	switch t {
	case "project":
		return 0
	case "festival":
		return 1
	case "phase":
		return 2
	case "sequence":
		return 3
	case "task":
		return 4
	case "intent":
		return 5
	case "design_doc":
		return 6
	case "explore_doc":
		return 7
	case "note":
		return 8
	case "canvas":
		return 9
	case "attachment":
		return 10
	case "tag":
		return 11
	case "chain":
		return 12
	case "repo":
		return 13
	case "folder":
		return 14
	case "package":
		return 15
	case "type_def":
		return 16
	case "function":
		return 17
	case "file":
		return 18
	}
	return 100
}

// groupByType buckets results by NodeType in deterministic priority
// order. Within each bucket, BM25 order (the input order) is preserved.
// Groups default to Expanded: true.
func groupByType(results []search.QueryResult) []resultGroup {
	if len(results) == 0 {
		return nil
	}
	buckets := map[string][]search.QueryResult{}
	order := []string{}
	for _, r := range results {
		if _, seen := buckets[r.NodeType]; !seen {
			order = append(order, r.NodeType)
		}
		buckets[r.NodeType] = append(buckets[r.NodeType], r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		pi, pj := typePriority(order[i]), typePriority(order[j])
		if pi != pj {
			return pi < pj
		}
		return order[i] < order[j]
	})
	groups := make([]resultGroup, 0, len(order))
	for _, t := range order {
		groups = append(groups, resultGroup{Type: t, Expanded: true, Rows: buckets[t]})
	}
	return groups
}

// querierIface is the subset of *search.Querier that internal/tui
// depends on. Declaring it here keeps the dependency one-directional;
// *search.Querier satisfies it structurally. Tests substitute a stub
// to exercise cancellation and msg-routing without a real database.
type querierIface interface {
	Search(ctx context.Context, opts search.QueryOptions) ([]search.QueryResult, error)
}

// buildOpts maps UI state on Model to a search.QueryOptions value.
// Pure: no I/O, no pointer escape beyond the returned struct.
func buildOpts(m Model) search.QueryOptions {
	return search.QueryOptions{
		Term: m.search.Value(),
	}
}

// queryResultMsg delivers the result of a live query issued via
// runQueryCmd. gen matches the queryGen assigned when the Cmd was
// created; stale results (gen != m.queryGen) are dropped by Update.
type queryResultMsg struct {
	gen     uint64
	results []search.QueryResult
	err     error
}

// runQueryCmd executes a search on the given querier and wraps the
// result in a queryResultMsg tagged with gen. The caller owns ctx and
// must cancel it when the Cmd is superseded.
func runQueryCmd(ctx context.Context, q querierIface, opts search.QueryOptions, gen uint64) tea.Cmd {
	return func() tea.Msg {
		results, err := q.Search(ctx, opts)
		return queryResultMsg{gen: gen, results: results, err: err}
	}
}
