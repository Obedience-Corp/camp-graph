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
	fetcher := m.previewFetcher
	if fetcher == nil {
		fetcher = defaultPreviewFetcher{store: m.store, g: m.graph}
	}
	return runPreviewCmd(ctx, fetcher, id)
}

// renderPreviewEmpty returns the hint shown when no row is focused
// yet (zero-value Model, empty result list, or after the user clears
// all filters without selecting a row).
func renderPreviewEmpty(width, height int) string {
	_ = width
	_ = height
	return "move the cursor to preview a node\n\ntab to focus this pane, tab again to return"
}

// renderPreviewLoading is shown while a preview fetch Cmd is in
// flight for the currently focused row id.
func renderPreviewLoading(width, height int) string {
	_ = width
	_ = height
	return "loading preview..."
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
			return renderPreviewLoading(width, height)
		}
		return renderPreviewEmpty(width, height)
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

// previewFetcher is the narrow contract runPreviewCmd depends on.
// The production implementation wraps *graph.Store and *graph.Graph;
// tests substitute a stub to exercise cancellation without opening a
// real database.
type previewFetcher interface {
	Fetch(ctx context.Context, id string) (*graph.Node, previewEdges, []search.RelatedItem, error)
}

// defaultPreviewFetcher is the production implementation backed by
// the in-memory Graph (for edges and node lookup) plus the FTS-backed
// search.Related query (for related rows).
type defaultPreviewFetcher struct {
	store *graph.Store
	g     *graph.Graph
}

func (f defaultPreviewFetcher) Fetch(ctx context.Context, id string) (*graph.Node, previewEdges, []search.RelatedItem, error) {
	node := f.g.Node(id)
	edges := previewEdges{Out: f.g.EdgesFrom(id), In: f.g.EdgesTo(id)}
	var related []search.RelatedItem
	var err error
	if node != nil && node.Path != "" {
		related, err = search.Related(ctx, f.store.DB(), search.RelatedOptions{
			Path:  node.Path,
			Limit: 3,
		})
	}
	return node, edges, related, err
}

// runPreviewCmd fetches node, outgoing and incoming edges, and related
// rows for id and delivers them via a previewMsg. The caller owns
// ctx; when a newer focus target supersedes this fetch, the caller
// cancels ctx and relies on the stale-drop in Update (id mismatch)
// to discard the eventual message.
func runPreviewCmd(ctx context.Context, f previewFetcher, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return previewMsg{}
		}
		node, edges, related, err := f.Fetch(ctx, id)
		return previewMsg{
			id:      id,
			node:    node,
			edges:   edges,
			related: related,
			err:     err,
		}
	}
}
