package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// previewEdges carries the outgoing and incoming edges for the node
// currently rendered in the preview pane.
type previewEdges struct {
	Out []*graph.Edge
	In  []*graph.Edge
}

// previewMsg is the async payload delivered by the preview Cmd. id
// tags the request so stale responses can be dropped when the list
// cursor has moved on by the time the msg arrives.
type previewMsg struct {
	id      string
	node    *graph.Node
	edges   previewEdges
	related []search.RelatedItem
	err     error
}

// issuePreview cancels any in-flight preview fetch, derives a fresh
// child context from m.ctx, and returns a runPreviewCmd targeting the
// currently focused row. Returns nil when no row is focused so the
// caller can skip batching a nil Cmd.
func (m *Model) issuePreview() tea.Cmd {
	id := m.focusedRowID()
	if id == "" {
		return nil
	}
	if m.previewCancel != nil {
		m.previewCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.previewCancel = cancel
	m.previewFocusID = id
	return runPreviewCmd(ctx, m.store, m.graph, id)
}

// runPreviewCmd fetches node, outgoing and incoming edges, and related
// rows for id and delivers them via a previewMsg. Edges are read from
// the in-memory Graph; related rows come from the FTS-backed
// search.Related query. ctx is owned by the caller and will be
// cancelled when the Cmd is superseded by a newer focus target.
func runPreviewCmd(ctx context.Context, store *graph.Store, g *graph.Graph, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return previewMsg{}
		}
		node := g.Node(id)
		out := g.EdgesFrom(id)
		in := g.EdgesTo(id)
		var related []search.RelatedItem
		var err error
		if node != nil && node.Path != "" {
			related, err = search.Related(ctx, store.DB(), search.RelatedOptions{
				Path:  node.Path,
				Limit: 3,
			})
		}
		return previewMsg{
			id:      id,
			node:    node,
			edges:   previewEdges{Out: out, In: in},
			related: related,
			err:     err,
		}
	}
}
