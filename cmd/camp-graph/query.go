package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/spf13/cobra"
)

var (
	queryType       string
	queryScope      string
	queryPathPrefix string
	queryMode       string
	queryTracked    bool
	queryUntracked  bool
	queryLimit      int
	queryJSON       bool
)

// newQueryCmd constructs the `camp-graph query` cobra command. Its
// surface is defined by IMPLEMENTATION_CONTRACTS.md.
func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <term>",
		Short: "Search across all workspace content (FTS5-backed)",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runQuery,
	}
	cmd.Flags().StringVar(&queryType, "type", "", "filter by node type (project, festival, note, intent, etc.)")
	cmd.Flags().StringVar(&queryScope, "scope", "", "limit results to a scope path")
	cmd.Flags().StringVar(&queryPathPrefix, "path-prefix", "", "limit results to a path prefix")
	cmd.Flags().StringVar(&queryMode, "mode", "hybrid", "relation mode: structural|explicit|semantic|hybrid")
	cmd.Flags().BoolVar(&queryTracked, "tracked", false, "only tracked files")
	cmd.Flags().BoolVar(&queryUntracked, "untracked", false, "only untracked files")
	cmd.Flags().IntVar(&queryLimit, "limit", 20, "maximum number of results")
	cmd.Flags().BoolVar(&queryJSON, "json", false, "emit graph-query/v1alpha1 JSON")
	return cmd
}

func runQuery(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctx.Value(configKey{}).(*Config)

	if queryTracked && queryUntracked {
		return graphErrors.New("--tracked and --untracked are mutually exclusive")
	}

	dbPath := filepath.Join(cfg.CampRoot, ".campaign", "graph.db")
	store, err := graph.OpenStore(ctx, dbPath)
	if err != nil {
		return graphErrors.Wrap(err, "open store (run 'camp-graph build' first)")
	}
	defer store.Close()

	if !search.FTSAvailable(ctx, store.DB()) {
		return graphErrors.New("search (FTS5) is unavailable on this database; run 'camp-graph build' to regenerate")
	}

	opts := search.QueryOptions{
		Term:       args[0],
		Type:       queryType,
		Scope:      queryScope,
		PathPrefix: queryPathPrefix,
		Mode:       search.ParseMode(queryMode),
		Tracked:    queryTracked,
		Untracked:  queryUntracked,
		Limit:      queryLimit,
	}
	querier := search.NewQuerier(store.DB())
	results, err := querier.Search(ctx, opts)
	if err != nil {
		return graphErrors.Wrap(err, "query")
	}

	if queryJSON {
		return writeQueryJSON(ctx, os.Stdout, cfg.CampRoot, opts, results)
	}
	return writeQueryHuman(os.Stdout, opts, results)
}

func writeQueryJSON(_ context.Context, w *os.File, campaignRoot string, opts search.QueryOptions, results []search.QueryResult) error {
	payload := search.QueryResponse{
		SchemaVersion: search.QueryResultSchemaVersion,
		CampaignRoot:  campaignRoot,
		Query:         opts.Term,
		Mode:          string(opts.Mode),
		Limit:         opts.Limit,
		Results:       nonNilResults(results),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func writeQueryHuman(w *os.File, opts search.QueryOptions, results []search.QueryResult) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No matches found.")
		return nil
	}
	for _, r := range results {
		fmt.Fprintf(w, "  [%s] %-50s  score=%.2f\n", r.NodeType, r.NodeID, r.Score)
		if r.Title != "" {
			fmt.Fprintf(w, "        %s\n", r.Title)
		}
		if r.Snippet != "" {
			fmt.Fprintf(w, "        %s\n", r.Snippet)
		}
	}
	fmt.Fprintf(w, "\n%d result(s)\n", len(results))
	return nil
}

func nonNilResults(in []search.QueryResult) []search.QueryResult {
	if in == nil {
		return []search.QueryResult{}
	}
	return in
}
