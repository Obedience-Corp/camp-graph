package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/runtime"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/Obedience-Corp/camp-graph/internal/version"
)

var outputPath string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build knowledge graph from campaign filesystem",
	Long:  "Scan the campaign directory and build a knowledge graph of all artifacts and their relationships.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := ctx.Value(configKey{}).(*Config)
		root := cfg.CampRoot

		if _, err := os.Stat(filepath.Join(root, "projects")); os.IsNotExist(err) {
			return graphErrors.New(root + " does not appear to be a campaign (no projects/ directory)")
		}

		fmt.Printf("Building graph from: %s\n\n", root)

		fmt.Println("Scanning...")
		sc := scanner.New(root)
		g, err := sc.Scan(ctx)
		if err != nil {
			return graphErrors.Wrap(err, "scan failed")
		}
		printScanSummary(g)

		dbPath := outputPath
		if dbPath == "" {
			dbPath = filepath.Join(root, ".campaign", "graph.db")
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return graphErrors.Wrapf(err, "create directory for %s", dbPath)
		}

		store, err := graph.OpenStore(ctx, dbPath)
		if err != nil {
			return graphErrors.Wrap(err, "open store")
		}
		defer store.Close()

		docs, err := buildSearchDocs(g)
		if err != nil {
			return graphErrors.Wrap(err, "build search docs")
		}
		now := time.Now().UTC()
		meta := graph.BuildMeta{
			GraphSchemaVersion: search.GraphSchemaVersion,
			PluginVersion:      version.Version,
			CampaignRoot:       root,
			BuiltAt:            now,
			LastRefreshAt:      now,
			LastRefreshMode:    "rebuild",
			SearchAvailable:    search.FTSAvailable(ctx, store.DB()),
		}
		indexed := runtime.BuildIndexedFileRecords(sc.Inventory(), g, now)
		if err := graph.SaveFullBuildWithIndex(ctx, store, g, docs, indexed, meta); err != nil {
			return graphErrors.Wrap(err, "save full build")
		}

		fmt.Printf("\nSaved to: %s (%d nodes, %d edges, %d search docs, %d indexed files)\n",
			dbPath, g.NodeCount(), g.EdgeCount(), len(docs), len(indexed))
		return nil
	},
}

// buildSearchDocs converts notes (and other indexable nodes) to
// DocumentRecord values consumed by graph.SaveFullBuild. For the first
// iteration this indexes note nodes only. Artifact indexing can extend
// the function without touching the save pipeline.
//
// A note that vanished between scan and index (permissions changed,
// file deleted, broken symlink) is skipped with a warning emitted to
// stderr; other read errors halt the build so operators see real
// corruption instead of silently empty search documents.
func buildSearchDocs(g *graph.Graph) ([]graph.DocumentRecord, error) {
	var docs []graph.DocumentRecord
	for _, n := range g.Nodes() {
		if n.Type != graph.NodeNote {
			continue
		}
		body, err := os.ReadFile(n.Path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", n.ID, err)
				continue
			}
			return nil, graphErrors.Wrapf(err, "read note body for %s (%s)", n.ID, n.Path)
		}
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
	return docs, nil
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
