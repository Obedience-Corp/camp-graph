package tui

import (
	"context"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// previewEdges carries the outgoing and incoming edges for the node
// currently rendered in the preview pane.
type previewEdges struct {
	Out []graph.Edge
	In  []graph.Edge
}

// previewMsg is the async payload delivered by the preview Cmd. id
// tags the request so stale responses can be dropped when the list
// cursor has moved on by the time the msg arrives.
type previewMsg struct {
	id      string
	node    *graph.Node
	edges   previewEdges
	related []search.QueryResult
	err     error
}

var _ = context.Background
