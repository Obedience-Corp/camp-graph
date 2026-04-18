package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/render"
)

var (
	renderOutput    string
	renderNode      string
	renderHops      int
	renderDB        string
	renderFormat    string
	renderOpen      bool
	renderNoSave    bool
	renderScope     string
	renderMode      string
	renderTracked   bool
	renderUntracked bool
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render graph as DOT, SVG, PNG, JSON, or HTML",
	Long:  "Output the knowledge graph in DOT, SVG, PNG, JSON, or HTML format.\nBy default, output is also saved to .campaign/graphs/ for easy access.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := ctx.Value(configKey{}).(*Config)

		format, err := render.ParseFormat(renderFormat)
		if err != nil {
			return err
		}

		dbPath := renderDB
		if dbPath == "" {
			dbPath = filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return graphErrors.New("graph database not found at " + dbPath + "\nRun 'camp-graph build' first to create it")
		}

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return graphErrors.Wrap(err, "open graph database")
		}
		defer store.Close()

		g, err := graph.LoadGraph(ctx, store)
		if err != nil {
			return graphErrors.Wrap(err, "load graph")
		}

		if renderTracked && renderUntracked {
			return graphErrors.New("--tracked and --untracked are mutually exclusive")
		}

		// --node wins over --scope per contract.
		switch {
		case renderNode != "":
			resolved := renderNode
			if n := resolveNodeDB(ctx, store, g, renderNode); n != nil {
				resolved = n.ID
			}
			g = g.Subgraph(resolved, renderHops)
			if g.NodeCount() == 0 {
				return graphErrors.New("node " + renderNode + " not found in graph")
			}
		case renderScope != "":
			sliced, err := sliceByScope(g, renderScope)
			if err != nil {
				return err
			}
			g = sliced
		}

		// Apply mode filter on edges then tracked filter on nodes.
		if m := renderMode; m != "" && m != "hybrid" {
			g = filterByRelationMode(g, m)
		}
		if renderTracked || renderUntracked {
			state := "tracked"
			if renderUntracked {
				state = "untracked"
			}
			g = filterByTrackedState(g, state)
		}

		// All paths are relative to campaign root for portability.
		// Resolve --output relative to campaign root if not absolute.
		resolvedOutput := renderOutput
		if resolvedOutput != "" && !filepath.IsAbs(resolvedOutput) {
			resolvedOutput = filepath.Join(cfg.CampRoot, resolvedOutput)
		}

		// Build the default .campaign/graphs/ filename.
		relGraphsDir := filepath.Join(".campaign", "graphs")
		baseName := "campaign-graph"
		if renderNode != "" {
			baseName = "campaign-graph-" + sanitizeNodeID(renderNode)
		}
		relDefaultPath := filepath.Join(relGraphsDir, baseName+"."+string(format))
		absDefaultPath := filepath.Join(cfg.CampRoot, relDefaultPath)

		// Render to --output if specified.
		if resolvedOutput != "" {
			f, err := os.Create(resolvedOutput)
			if err != nil {
				return graphErrors.Wrap(err, "create output file")
			}
			defer f.Close()
			if err := render.Render(ctx, f, g, format); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %s\n", renderOutput)
		}

		// For text formats (DOT, JSON) with no --output, also write to stdout so pipelines work.
		if resolvedOutput == "" && (format == render.FormatDOT || format == render.FormatJSON) {
			if err := render.Render(ctx, os.Stdout, g, format); err != nil {
				return err
			}
		}

		// Auto-save to .campaign/graphs/ unless --no-save or --output is set.
		if !renderNoSave && resolvedOutput == "" {
			if err := os.MkdirAll(filepath.Join(cfg.CampRoot, relGraphsDir), 0o755); err != nil {
				return graphErrors.Wrap(err, "create graphs directory")
			}
			f, err := os.Create(absDefaultPath)
			if err != nil {
				return graphErrors.Wrap(err, "create graphs file")
			}
			defer f.Close()
			if err := render.Render(ctx, f, g, format); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved to %s\n", relDefaultPath)
		}

		// For non-text formats with no file destination, there's nowhere to write.
		if resolvedOutput == "" && renderNoSave && format != render.FormatDOT && format != render.FormatJSON {
			return graphErrors.New("format " + string(format) + " requires a file output; use --output or remove --no-save")
		}

		// Open the file if requested.
		if renderOpen {
			target := absDefaultPath
			if resolvedOutput != "" {
				target = resolvedOutput
			}
			if err := render.OpenFile(target); err != nil {
				return graphErrors.Wrap(err, "open file")
			}
		}

		return nil
	},
}

// sanitizeNodeID converts a node ID to a filesystem-safe string.
func sanitizeNodeID(id string) string {
	safe := strings.NewReplacer(":", "-", "/", "-", " ", "-", "..", "-").Replace(id)
	return safe
}
