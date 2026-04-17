package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/render"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/Obedience-Corp/camp-graph/internal/tui"
	"github.com/Obedience-Corp/camp-graph/internal/version"
	"github.com/Obedience-Corp/obey-shared/camputil"
	"github.com/spf13/cobra"
)

// configKey is the unexported type used as the context key for Config values.
type configKey struct{}

// Config holds the runtime configuration for camp-graph commands.
// It is populated from flags in init() and stored in the cobra context
// so all subcommands can access it without reading global state.
type Config struct {
	// Verbose enables detailed output when true.
	Verbose bool

	// CampRoot is the resolved campaign root directory for this invocation.
	CampRoot string
}

var rootCmd = &cobra.Command{
	Use:   "camp-graph",
	Short: "Knowledge graph visualization for campaigns",
	Long: `camp-graph builds and visualizes knowledge graphs from campaign artifacts.

It discovers relationships between projects, festivals, intents, design docs,
chains, and code to provide a unified view of your campaign.

When installed on PATH, camp discovers it automatically:
  camp graph build
  camp graph browse
  camp graph query "auth"`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	cfg := &Config{}

	rootCmd.PersistentFlags().BoolVar(&cfg.Verbose, "verbose", false, "enable verbose output")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		root, err := camputil.FindCampaignRoot(cmd.Context(), "")
		if err != nil {
			return fmt.Errorf("determining campaign root: %w", err)
		}
		cfg.CampRoot = root
		cmd.SetContext(context.WithValue(cmd.Context(), configKey{}, cfg))
		return nil
	}

	buildCmd.Flags().StringVar(&outputPath, "output", "", "override database output path")
	contextCmd.Flags().IntVar(&contextHops, "hops", 2, "neighborhood depth")
	contextCmd.Flags().BoolVar(&contextDot, "dot", false, "output micrograph as DOT format")

	browseCmd.Flags().StringVar(&browsePath, "db", "", "path to graph database")
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "", "write output to file instead of stdout")
	renderCmd.Flags().StringVar(&renderNode, "node", "", "render only the neighborhood of this node ID")
	renderCmd.Flags().IntVar(&renderHops, "hops", 2, "neighborhood depth when using --node")
	renderCmd.Flags().StringVar(&renderDB, "db", "", "path to graph database")
	renderCmd.Flags().StringVarP(&renderFormat, "format", "f", "dot", "output format: dot, svg, png, json, html")
	renderCmd.Flags().BoolVar(&renderOpen, "open", false, "open rendered file after writing")
	renderCmd.Flags().BoolVar(&renderNoSave, "no-save", false, "skip auto-save to .campaign/graphs/")
	renderCmd.Flags().StringVar(&renderScope, "scope", "", "render only the nodes inside this scope path")
	renderCmd.Flags().StringVar(&renderMode, "mode", "hybrid", "relation mode: structural|explicit|semantic|hybrid")
	renderCmd.Flags().BoolVar(&renderTracked, "tracked", false, "only tracked files")
	renderCmd.Flags().BoolVar(&renderUntracked, "untracked", false, "only untracked files")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(newQueryCmd())
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(browseCmd)
	rootCmd.AddCommand(renderCmd)
	rootCmd.AddCommand(newRefreshCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newRelatedCmd())
}

var (
	outputPath      string
	queryType       string
	contextHops     int
	contextDot      bool
	browsePath      string
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

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("camp-graph %s (%s) built %s\n",
			version.Version, version.Commit, version.BuildDate)
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build knowledge graph from campaign filesystem",
	Long:  "Scan the campaign directory and build a knowledge graph of all artifacts and their relationships.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := ctx.Value(configKey{}).(*Config)
		root := cfg.CampRoot

		if _, err := os.Stat(filepath.Join(root, "projects")); os.IsNotExist(err) {
			return fmt.Errorf("%s does not appear to be a campaign (no projects/ directory)", root)
		}

		fmt.Printf("Building graph from: %s\n\n", root)

		fmt.Println("Scanning...")
		sc := scanner.New(root)
		g, err := sc.Scan(ctx)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		printScanSummary(g)

		dbPath := outputPath
		if dbPath == "" {
			dbPath = filepath.Join(root, ".campaign", "graph.db")
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", dbPath, err)
		}

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer store.Close()

		docs := buildSearchDocs(g)
		meta := graph.BuildMeta{
			GraphSchemaVersion: search.GraphSchemaVersion,
			PluginVersion:      version.Version,
			CampaignRoot:       root,
			BuiltAt:            time.Now().UTC(),
			LastRefreshAt:      time.Now().UTC(),
			LastRefreshMode:    "rebuild",
			SearchAvailable:    search.FTSAvailable(ctx, store.DB()),
		}
		if err := graph.SaveFullBuild(ctx, store, g, docs, meta); err != nil {
			return fmt.Errorf("save full build: %w", err)
		}

		fmt.Printf("\nSaved to: %s (%d nodes, %d edges, %d search docs)\n",
			dbPath, g.NodeCount(), g.EdgeCount(), len(docs))
		return nil
	},
}

// buildSearchDocs converts notes (and other indexable nodes) to
// DocumentRecord values consumed by graph.SaveFullBuild. For the first
// iteration this indexes note nodes only. Artifact indexing can extend
// the function without touching the save pipeline.
func buildSearchDocs(g *graph.Graph) []graph.DocumentRecord {
	var docs []graph.DocumentRecord
	for _, n := range g.Nodes() {
		if n.Type != graph.NodeNote {
			continue
		}
		body, _ := os.ReadFile(n.Path)
		doc := graph.DocumentRecord{
			NodeID:       n.ID,
			Title:        firstNonEmpty(n.Metadata[graph.MetaNoteTitle], n.Name),
			RelPath:      n.Name,
			Scope:        inferScopeFromRel(n.Name),
			Body:         string(body),
			Aliases:      splitCommaList(n.Metadata[graph.MetaNoteAliases]),
			Tags:         splitCommaList(n.Metadata[graph.MetaNoteTags]),
			TrackedState: firstNonEmpty(n.Metadata[graph.MetaGitState], "unknown"),
			UpdatedAt:    n.UpdatedAt,
		}
		docs = append(docs, doc)
	}
	return docs
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func splitCommaList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// inferScopeFromRel returns the immediate parent folder path as the
// scope label shown in results. The campaign root maps to "." so the
// scope column is never empty.
func inferScopeFromRel(rel string) string {
	rel = filepath.ToSlash(rel)
	idx := strings.LastIndex(rel, "/")
	if idx < 0 {
		return "."
	}
	return rel[:idx]
}

// printScanSummary displays node counts by type.
func printScanSummary(g *graph.Graph) {
	types := []graph.NodeType{
		graph.NodeProject, graph.NodeFestival, graph.NodeChain,
		graph.NodePhase, graph.NodeSequence, graph.NodeTask,
		graph.NodeIntent, graph.NodeDesignDoc, graph.NodeExploreDoc,
	}
	for _, t := range types {
		count := len(g.NodesByType(t))
		if count > 0 {
			fmt.Printf("  %-14s %d\n", t.String()+":", count)
		}
	}
	fmt.Printf("\n  Edges: %d\n", g.EdgeCount())
}


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
			return fmt.Errorf("open store (run 'camp-graph build' first): %w", err)
		}
		defer store.Close()

		g, err := graph.LoadGraph(ctx, store)
		if err != nil {
			return fmt.Errorf("load graph: %w", err)
		}

		target := resolveNodeDB(ctx, store, g, args[0])
		if target == nil {
			return fmt.Errorf("node %q not found\nTry: camp-graph query %s", args[0], args[0])
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

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Interactive graph browser (TUI)",
	Long:  "Launch an interactive terminal browser to explore the knowledge graph.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := ctx.Value(configKey{}).(*Config)

		dbPath := browsePath
		if dbPath == "" {
			dbPath = filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("graph database not found at %s\nRun 'camp-graph build' first to create it", dbPath)
		}

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("open graph database: %w", err)
		}
		defer store.Close()

		g, err := graph.LoadGraph(ctx, store)
		if err != nil {
			return fmt.Errorf("load graph: %w", err)
		}

		model := tui.New(g)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

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
			return fmt.Errorf("graph database not found at %s\nRun 'camp-graph build' first to create it", dbPath)
		}

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("open graph database: %w", err)
		}
		defer store.Close()

		g, err := graph.LoadGraph(ctx, store)
		if err != nil {
			return fmt.Errorf("load graph: %w", err)
		}

		if renderTracked && renderUntracked {
			return fmt.Errorf("--tracked and --untracked are mutually exclusive")
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
				return fmt.Errorf("node %q not found in graph", renderNode)
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
				return fmt.Errorf("create output file: %w", err)
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
				return fmt.Errorf("create graphs directory: %w", err)
			}
			f, err := os.Create(absDefaultPath)
			if err != nil {
				return fmt.Errorf("create graphs file: %w", err)
			}
			defer f.Close()
			if err := render.Render(ctx, f, g, format); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved to %s\n", relDefaultPath)
		}

		// For non-text formats with no file destination, there's nowhere to write.
		if resolvedOutput == "" && renderNoSave && format != render.FormatDOT && format != render.FormatJSON {
			return fmt.Errorf("format %q requires a file output; use --output or remove --no-save", format)
		}

		// Open the file if requested.
		if renderOpen {
			target := absDefaultPath
			if resolvedOutput != "" {
				target = resolvedOutput
			}
			if err := render.OpenFile(target); err != nil {
				return fmt.Errorf("open file: %w", err)
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
