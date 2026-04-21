package tui

import (
	"context"
	"fmt"
	"strings"

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

// previewEdgeCap limits how many edges are rendered per direction in
// the preview pane. Overflow past the cap collapses to a "... +N more"
// footer line so long edge lists do not push related rows off-screen.
const previewEdgeCap = 50

// renderPreview builds the preview pane body: node header (Name +
// [Type]), path, optional status, outgoing edges (capped), incoming
// edges (capped), and up to three related items. When focused, the
// body is sliced by m.previewScroll lines from the top.
func renderPreview(m Model, width, height int) string {
	_ = width
	_ = height
	if m.previewNode == nil {
		if m.previewFocusID != "" {
			return "Loading preview..."
		}
		return "No node selected"
	}
	n := m.previewNode
	var b strings.Builder

	fmt.Fprintf(&b, "%s  [%s]\n", n.Name, n.Type)
	if n.Path != "" {
		fmt.Fprintf(&b, "%s\n", n.Path)
	}
	if n.Status != "" {
		fmt.Fprintf(&b, "status: %s\n", n.Status)
	}
	b.WriteString("\n")

	b.WriteString("outgoing:\n")
	writeEdgeList(&b, m.previewEdges.Out, previewEdgeCap)
	b.WriteString("\n")

	b.WriteString("incoming:\n")
	writeEdgeList(&b, m.previewEdges.In, previewEdgeCap)
	b.WriteString("\n")

	b.WriteString("related:\n")
	rel := m.previewRelated
	if len(rel) > 3 {
		rel = rel[:3]
	}
	if len(rel) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, r := range rel {
		fmt.Fprintf(&b, "  %s  [%s]\n", r.Title, r.NodeType)
	}

	body := b.String()
	if m.previewScroll > 0 {
		lines := strings.Split(body, "\n")
		if m.previewScroll < len(lines) {
			body = strings.Join(lines[m.previewScroll:], "\n")
		} else {
			body = ""
		}
	}
	return body
}

// writeEdgeList writes up to cap edges as "<type> -> <toID>" lines
// with a trailing "... +N more" line when the list exceeds the cap.
func writeEdgeList(b *strings.Builder, edges []*graph.Edge, cap int) {
	if len(edges) == 0 {
		b.WriteString("  (none)\n")
		return
	}
	n := len(edges)
	shown := n
	if shown > cap {
		shown = cap
	}
	for i := 0; i < shown; i++ {
		e := edges[i]
		fmt.Fprintf(b, "  %s -> %s\n", e.Type, e.ToID)
	}
	if n > cap {
		fmt.Fprintf(b, "  ... +%d more\n", n-cap)
	}
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
