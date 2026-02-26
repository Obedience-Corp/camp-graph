package main

import (
	"context"
	"fmt"
	"os"

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

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(queryCmd)
}

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
		cfg := cmd.Context().Value(configKey{}).(*Config)
		fmt.Printf("Building graph from: %s\n", cfg.CampRoot)
		if cfg.Verbose {
			fmt.Println("verbose output enabled")
		}
		fmt.Println("(not yet implemented)")
		return nil
	},
}

var queryCmd = &cobra.Command{
	Use:   "query <term>",
	Short: "Search across all graph nodes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Querying graph for: %s\n", args[0])
		fmt.Println("(not yet implemented)")
		return nil
	},
}
