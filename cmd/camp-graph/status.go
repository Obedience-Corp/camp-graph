package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/spf13/cobra"
)

var (
	statusDB   string
	statusJSON bool
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show graph database status",
		RunE:  runStatus,
	}
	cmd.Flags().StringVar(&statusDB, "db", "", "override graph database path")
	cmd.Flags().BoolVar(&statusJSON, "json", false, "emit graph-status/v1alpha1 JSON")
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := ctx.Value(configKey{}).(*Config)

	dbPath := statusDB
	if dbPath == "" {
		dbPath = filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
	}

	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		return graphErrors.Wrap(err, "open store")
	}
	defer store.Close()

	status, err := runtime.LoadStatus(ctx, store.DB(), cfg.CampRoot, dbPath)
	if err != nil {
		return graphErrors.Wrap(err, "load status")
	}
	// Treat FTS as unavailable whenever the virtual table fails so
	// search_available is always a live observation.
	status.SearchAvailable = search.FTSAvailable(ctx, store.DB())

	if statusJSON {
		payload := search.StatusResponse{
			SchemaVersion:      search.StatusResultSchemaVersion,
			CampaignRoot:       status.CampaignRoot,
			DBPath:             status.DBPath,
			GraphSchemaVersion: status.GraphSchemaVersion,
			PluginVersion:      status.PluginVersion,
			BuiltAt:            status.BuiltAt,
			LastRefreshAt:      status.LastRefreshAt,
			LastRefreshMode:    status.LastRefreshMode,
			SearchAvailable:    status.SearchAvailable,
			Stale:              status.Stale,
			IndexedFiles:       status.IndexedFiles,
			Nodes:              status.Nodes,
			Edges:              status.Edges,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(os.Stdout, "campaign=%s db=%s schema=%s plugin=%s nodes=%d edges=%d indexed=%d stale=%v search=%v\n",
		status.CampaignRoot, status.DBPath,
		status.GraphSchemaVersion, status.PluginVersion,
		status.Nodes, status.Edges, status.IndexedFiles,
		status.Stale, status.SearchAvailable,
	)
	return nil
}
