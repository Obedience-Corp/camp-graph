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
	relatedDB    string
	relatedMode  string
	relatedLimit int
	relatedJSON  bool
	relatedPath  string
)

func newRelatedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "related",
		Short: "Find items related to a campaign-relative path",
		Long: `Return graph-related/v1alpha1 items relevant to --path.

The --path argument is campaign-relative. When called by camp workitem
integrations, callers should pass primary_doc when non-empty and fall
back to relative_path otherwise.`,
		RunE: runRelated,
	}
	cmd.Flags().StringVar(&relatedPath, "path", "", "campaign-relative path to the source document")
	cmd.Flags().StringVar(&relatedMode, "mode", "hybrid", "relation mode: structural|explicit|semantic|hybrid")
	cmd.Flags().IntVar(&relatedLimit, "limit", 10, "maximum number of items to return")
	cmd.Flags().StringVar(&relatedDB, "db", "", "override graph database path")
	cmd.Flags().BoolVar(&relatedJSON, "json", false, "emit graph-related/v1alpha1 JSON")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}

func runRelated(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := ctx.Value(configKey{}).(*Config)

	dbPath := relatedDB
	if dbPath == "" {
		dbPath = filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
	}
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		return graphErrors.Wrap(err, "open store")
	}
	defer store.Close()

	items, err := search.Related(ctx, store.DB(), search.RelatedOptions{
		Path:  relatedPath,
		Mode:  search.ParseMode(relatedMode),
		Limit: relatedLimit,
	})
	if err != nil {
		return graphErrors.Wrap(err, "related")
	}

	status, _ := runtime.LoadStatus(ctx, store.DB(), cfg.CampRoot, dbPath)
	stale := status != nil && status.Stale

	if relatedJSON {
		payload := search.RelatedResponse{
			SchemaVersion: search.RelatedResultSchemaVersion,
			CampaignRoot:  cfg.CampRoot,
			QueryPath:     relatedPath,
			Mode:          relatedMode,
			Stale:         stale,
			Items:         nonNilRelated(items),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stdout, "No related items.")
		return nil
	}
	for _, it := range items {
		fmt.Fprintf(os.Stdout, "  [%s] %-50s reason=%s score=%.2f\n",
			it.NodeType, it.NodeID, it.Reason, it.Score)
	}
	return nil
}

func nonNilRelated(items []search.RelatedItem) []search.RelatedItem {
	if items == nil {
		return []search.RelatedItem{}
	}
	return items
}
