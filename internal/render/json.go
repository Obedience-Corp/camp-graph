package render

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// jsonSchemaVersion identifies the shape of the JSON envelope.
// Bump when making backwards-incompatible changes to field names or structure.
const jsonSchemaVersion = "1"

type jsonEnvelope struct {
	Version string        `json:"version"`
	Nodes   []*graph.Node `json:"nodes"`
	Edges   []*graph.Edge `json:"edges"`
}

// RenderJSON writes the graph as an indented JSON document to w.
// Nodes and edges are sorted deterministically so output is diffable and git-friendly.
func RenderJSON(w io.Writer, g *graph.Graph) error {
	nodes := g.Nodes()
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	edges := g.Edges()
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromID != edges[j].FromID {
			return edges[i].FromID < edges[j].FromID
		}
		if edges[i].ToID != edges[j].ToID {
			return edges[i].ToID < edges[j].ToID
		}
		return edges[i].Type < edges[j].Type
	})

	env := jsonEnvelope{
		Version: jsonSchemaVersion,
		Nodes:   nodes,
		Edges:   edges,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
