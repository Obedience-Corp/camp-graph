package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/render"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

var (
	contextHops int
	contextDot  bool
)

var contextCmd = &cobra.Command{
	Use:   "context <id-or-name>",
	Short: "Show artifact context (micrograph neighborhood view)",
	Long:  "Display the knowledge graph neighborhood around a specific artifact, showing all related nodes within hop range.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := ctx.Value(configKey{}).(*Config)
		dbPath := filepath.Join(cfg.CampRoot, ".campaign", "graph.db")

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return graphErrors.Wrap(err, "open store (run 'camp-graph build' first)")
		}
		defer store.Close()

		g, err := graph.LoadGraph(ctx, store)
		if err != nil {
			return graphErrors.Wrap(err, "load graph")
		}

		target := resolveNodeDB(ctx, store, g, args[0])
		if target == nil {
			return graphErrors.New("node " + args[0] + " not found\nTry: camp-graph query " + args[0])
		}

		sub := g.Subgraph(target.ID, contextHops)

		if contextDot {
			return render.RenderDOT(os.Stdout, sub)
		}

		printMicrograph(os.Stdout, g, target, sub, contextHops)
		return nil
	},
}

// resolveNodeDB resolves a user-supplied node reference using the
// retrieval-backed order defined in IMPLEMENTATION_CONTRACTS.md: exact
// ID, then exact relative path, then top lexical hit. It falls back to
// an in-memory exact-name match only when the DB resolver returns
// nothing so callers that pass a bare artifact name (e.g. "alpha")
// still succeed before we go to search.
func resolveNodeDB(ctx context.Context, store *graph.Store, g *graph.Graph, query string) *graph.Node {
	if n := g.Node(query); n != nil {
		return n
	}
	id, _, err := search.Resolve(ctx, store.DB(), query)
	if err == nil && id != "" {
		if n := g.Node(id); n != nil {
			return n
		}
	}
	// Exact name fallback for artifacts whose IDs embed the name
	// (e.g. project:alpha from "alpha"). This keeps legacy CLI UX.
	for _, n := range g.Nodes() {
		if n.Name == query {
			return n
		}
	}
	return nil
}

// printMicrograph outputs a formatted neighborhood view.
func printMicrograph(w io.Writer, full *graph.Graph, target *graph.Node, sub *graph.Graph, hops int) {
	fmt.Fprintf(w, "\n=== %s ===\n", target.Name)
	fmt.Fprintf(w, "Type:   %s\n", target.Type)
	fmt.Fprintf(w, "Path:   %s\n", target.Path)
	if target.Status != "" {
		fmt.Fprintf(w, "Status: %s\n", target.Status)
	}
	fmt.Fprintln(w)

	outgoing := full.EdgesFrom(target.ID)
	if len(outgoing) > 0 {
		fmt.Fprintln(w, "Outgoing:")
		for _, e := range outgoing {
			if n := full.Node(e.ToID); n != nil {
				fmt.Fprintf(w, "  → %s [%s] (%s)\n", n.Name, e.Type, n.Type)
			}
		}
		fmt.Fprintln(w)
	}

	incoming := full.EdgesTo(target.ID)
	if len(incoming) > 0 {
		fmt.Fprintln(w, "Incoming:")
		for _, e := range incoming {
			if n := full.Node(e.FromID); n != nil {
				fmt.Fprintf(w, "  ← %s [%s] (%s)\n", n.Name, e.Type, n.Type)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Neighborhood: %d nodes, %d edges (%d hops)\n\n",
		sub.NodeCount(), sub.EdgeCount(), hops)
}
