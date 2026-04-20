package tui

import (
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// buildOpts maps UI state on Model to a search.QueryOptions value.
// Pure: no I/O, no pointer escape beyond the returned struct.
func buildOpts(m Model) search.QueryOptions {
	return search.QueryOptions{
		Term: m.search.Value(),
	}
}
