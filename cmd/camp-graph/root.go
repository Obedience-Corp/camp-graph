package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
	"github.com/Obedience-Corp/camp-graph/internal/tui"
	"github.com/Obedience-Corp/camp-graph/internal/version"
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
		root, err := getCampRoot()
		if err != nil {
			return fmt.Errorf("determining campaign root: %w", err)
		}
		cfg.CampRoot = root
		cmd.SetContext(context.WithValue(cmd.Context(), configKey{}, cfg))
		return nil
	}

	buildCmd.Flags().StringVar(&outputPath, "output", "", "override database output path")
	queryCmd.Flags().StringVar(&queryType, "type", "", "filter by node type (project, festival, intent, etc.)")
	contextCmd.Flags().IntVar(&contextHops, "hops", 1, "number of hops from center node")

	browseCmd.Flags().StringVar(&browsePath, "db", "", "path to graph database")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(browseCmd)
}

var (
	outputPath  string
	queryType   string
	contextHops int
	browsePath  string
)

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func getCampRoot() (string, error) {
	if root := os.Getenv("CAMP_ROOT"); root != "" {
		return root, nil
	}
	return os.Getwd()
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

		if err := graph.SaveGraph(ctx, store, g); err != nil {
			return fmt.Errorf("save graph: %w", err)
		}

		fmt.Printf("\nSaved to: %s (%d nodes, %d edges)\n", dbPath, g.NodeCount(), g.EdgeCount())
		return nil
	},
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

var queryCmd = &cobra.Command{
	Use:   "query <term>",
	Short: "Search across all graph nodes",
	Args:  cobra.MinimumNArgs(1),
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

		term := strings.ToLower(args[0])
		var matches []*graph.Node
		for _, n := range g.Nodes() {
			if queryType != "" && string(n.Type) != queryType {
				continue
			}
			if strings.Contains(strings.ToLower(n.Name), term) ||
				strings.Contains(strings.ToLower(n.ID), term) {
				matches = append(matches, n)
			}
		}

		if len(matches) == 0 {
			fmt.Println("No matches found.")
			return nil
		}

		for _, n := range matches {
			tag := strings.ToUpper(string(n.Type)[:3])
			status := ""
			if n.Status != "" {
				status = fmt.Sprintf("  (%s)", n.Status)
			}
			fmt.Printf("  [%s] %-40s %s%s\n", tag, n.ID, n.Path, status)
		}
		fmt.Printf("\n%d result(s)\n", len(matches))
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:   "context <id>",
	Short: "Show relationships for an artifact (micrograph view)",
	Long:  "Display the neighborhood of a node — all directly related artifacts grouped by relationship type.",
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

		nodeID := args[0]
		center := g.Node(nodeID)
		if center == nil {
			return fmt.Errorf("node %q not found (use 'camp-graph query' to search)", nodeID)
		}

		tag := strings.ToUpper(string(center.Type)[:3])
		fmt.Printf("\n  [%s] %s - %s", tag, center.ID, center.Name)
		if center.Status != "" {
			fmt.Printf(" (%s)", center.Status)
		}
		fmt.Printf("\n  Path: %s\n", center.Path)

		sub := g.Subgraph(nodeID, contextHops)

		fmt.Println("\n  Relationships:")
		outgoing := g.EdgesFrom(nodeID)
		edgeGroups := make(map[graph.EdgeType][]*graph.Edge)
		for _, e := range outgoing {
			edgeGroups[e.Type] = append(edgeGroups[e.Type], e)
		}
		for edgeType, edges := range edgeGroups {
			fmt.Printf("    %s ──►\n", edgeType)
			for _, e := range edges {
				if n := sub.Node(e.ToID); n != nil {
					ntag := strings.ToUpper(string(n.Type)[:3])
					fmt.Printf("      [%s] %s\n", ntag, n.ID)
				}
			}
		}

		incoming := g.EdgesTo(nodeID)
		inGroups := make(map[graph.EdgeType][]*graph.Edge)
		for _, e := range incoming {
			inGroups[e.Type] = append(inGroups[e.Type], e)
		}
		for edgeType, edges := range inGroups {
			fmt.Printf("    %s ◄──\n", edgeType)
			for _, e := range edges {
				if n := sub.Node(e.FromID); n != nil {
					ntag := strings.ToUpper(string(n.Type)[:3])
					fmt.Printf("      [%s] %s\n", ntag, n.ID)
				}
			}
		}

		fmt.Println()
		return nil
	},
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
