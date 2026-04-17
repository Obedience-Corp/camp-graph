package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/Obedience-Corp/camp-graph/internal/version"
	"github.com/spf13/cobra"
)

var (
	refreshDB   string
	refreshJSON bool
)

func newRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the graph database incrementally (falls back to rebuild)",
		RunE:  runRefresh,
	}
	cmd.Flags().StringVar(&refreshDB, "db", "", "override graph database path")
	cmd.Flags().BoolVar(&refreshJSON, "json", false, "emit graph-refresh/v1alpha1 JSON")
	return cmd
}

func runRefresh(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := ctx.Value(configKey{}).(*Config)

	dbPath := refreshDB
	if dbPath == "" {
		dbPath = filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dbPath, err)
	}

	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	buildMetaFn := func(mode runtime.RefreshMode, now time.Time, searchAvailable bool) graph.BuildMeta {
		return graph.BuildMeta{
			GraphSchemaVersion: search.GraphSchemaVersion,
			PluginVersion:      version.Version,
			CampaignRoot:       cfg.CampRoot,
			BuiltAt:            now.UTC(),
			LastRefreshAt:      now.UTC(),
			LastRefreshMode:    string(mode),
			SearchAvailable:    searchAvailable,
		}
	}

	req := runtime.RefreshRequest{
		CampaignRoot: cfg.CampRoot,
		DBPath:       dbPath,
		Store:        store,
		BuildDocs:    buildSearchDocs,
		BuildMetaFn:  buildMetaFn,
	}

	report, err := runtime.Refresh(ctx, req)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	if refreshJSON {
		payload := search.RefreshResponse{
			SchemaVersion:     search.RefreshResultSchemaVersion,
			CampaignRoot:      cfg.CampRoot,
			DBPath:            dbPath,
			Mode:              string(report.Mode),
			ReindexedFiles:    report.ReindexedFiles,
			DeletedFiles:      report.DeletedFiles,
			NodesWritten:      report.NodesWritten,
			EdgesWritten:      report.EdgesWritten,
			SearchDocsWritten: report.SearchDocsWritten,
			DurationMs:        report.DurationMs,
			StaleBefore:       report.StaleBefore,
			StaleAfter:        report.StaleAfter,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(os.Stdout, "mode=%s reindexed=%d deleted=%d nodes=%d edges=%d docs=%d duration=%dms\n",
		report.Mode, report.ReindexedFiles, report.DeletedFiles,
		report.NodesWritten, report.EdgesWritten, report.SearchDocsWritten,
		report.DurationMs)
	return nil
}
