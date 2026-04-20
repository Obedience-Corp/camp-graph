package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/search"
)

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
func runQueryCmd(ctx context.Context, q *search.Querier, opts search.QueryOptions, gen uint64) tea.Cmd {
	return func() tea.Msg {
		results, err := q.Search(ctx, opts)
		return queryResultMsg{gen: gen, results: results, err: err}
	}
}
