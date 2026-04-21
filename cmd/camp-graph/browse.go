package main

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/tui"
)

var browsePath string

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

		model := tui.New(ctx, store, g)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}
