package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/version"
	"github.com/Obedience-Corp/obey-shared/camputil"
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
			return graphErrors.Wrap(err, "determining campaign root")
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
