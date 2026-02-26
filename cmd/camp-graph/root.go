package main

import (
	"fmt"
	"os"

	"github.com/Obedience-Corp/camp-graph/internal/version"
	"github.com/spf13/cobra"
)

var (
	verbose bool

	rootCmd = &cobra.Command{
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
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(queryCmd)
}

// Execute runs the root command.
func Execute() error {
	// Read campaign context from environment (set by camp plugin discovery).
	campRoot := os.Getenv("CAMP_ROOT")
	if campRoot == "" {
		// If not invoked via camp, try to detect campaign root.
		cwd, err := os.Getwd()
		if err == nil {
			campRoot = cwd
		}
	}
	_ = campRoot // used by subcommands via getCampRoot()

	return rootCmd.Execute()
}

func getCampRoot() string {
	if root := os.Getenv("CAMP_ROOT"); root != "" {
		return root
	}
	cwd, _ := os.Getwd()
	return cwd
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
		root := getCampRoot()
		fmt.Printf("Building graph from: %s\n", root)
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
