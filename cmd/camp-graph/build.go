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

		docs, err := buildSearchDocs(root, g)
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

// indexableArtifactTypes is the set of non-note node types that are
// documented as queryable via `camp-graph query --type <t>`. Each gets
// a search_docs row synthesised from its name, metadata, and — when
// backed by a concrete file on disk — its contents.
//
// Kept as a map (rather than a slice) so membership checks are O(1)
// inside the hot build loop.
var indexableArtifactTypes = map[graph.NodeType]bool{
	graph.NodeProject:    true,
	graph.NodeFestival:   true,
	graph.NodeChain:      true,
	graph.NodePhase:      true,
	graph.NodeSequence:   true,
	graph.NodeTask:       true,
	graph.NodeIntent:     true,
	graph.NodeDesignDoc:  true,
	graph.NodeExploreDoc: true,
	// Code-slice nodes emitted by extractCodeSlices inside nested repos.
	// Indexing them lets `query --type file|package` return matches that
	// the CLI docs already advertise.
	graph.NodeFile:    true,
	graph.NodePackage: true,
}

// buildSearchDocs converts every indexable node in g into a
// DocumentRecord consumed by graph.SaveFullBuild. Both notes and
// artifact nodes (project, festival, intent, phase, sequence, task,
// design_doc, explore_doc, chain) are emitted so the CLI's
// `query --type <kind>` surface actually returns matches for each kind
// it advertises.
//
// campRoot is required to derive stable relative paths for nodes whose
// n.Path is an absolute filesystem path rather than a campaign-root
// relative one.
//
// A document whose source file vanished between scan and index
// (permissions changed, file deleted, broken symlink) is skipped with
// a warning on stderr; other read errors halt the build so operators
// see real corruption instead of silently empty search rows.
func buildSearchDocs(campRoot string, g *graph.Graph) ([]graph.DocumentRecord, error) {
	var docs []graph.DocumentRecord
	for _, n := range g.Nodes() {
		switch {
		case n.Type == graph.NodeNote:
			doc, skip, err := buildNoteDoc(n)
			if err != nil {
				return nil, err
			}
			if !skip {
				docs = append(docs, doc)
			}
		case indexableArtifactTypes[n.Type]:
			doc, skip, err := buildArtifactDoc(campRoot, n)
			if err != nil {
				return nil, err
			}
			if !skip {
				docs = append(docs, doc)
			}
		}
	}
	return docs, nil
}

// buildNoteDoc emits the search_docs row for a note node. A skip=true
// return means the caller should drop the node (vanished file); a
// non-nil error aborts the whole build.
func buildNoteDoc(n *graph.Node) (graph.DocumentRecord, bool, error) {
	body, err := os.ReadFile(n.Path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", n.ID, err)
			return graph.DocumentRecord{}, true, nil
		}
		return graph.DocumentRecord{}, false, graphErrors.Wrapf(err, "read note body for %s (%s)", n.ID, n.Path)
	}
	return graph.DocumentRecord{
		NodeID:       n.ID,
		Title:        firstNonEmpty(n.Metadata[graph.MetaNoteTitle], n.Name),
		RelPath:      n.Name,
		Scope:        inferScopeFromRel(n.Name),
		Body:         string(body),
		Aliases:      splitCommaList(n.Metadata[graph.MetaNoteAliases]),
		Tags:         splitCommaList(n.Metadata[graph.MetaNoteTags]),
		TrackedState: firstNonEmpty(n.Metadata[graph.MetaGitState], "unknown"),
		UpdatedAt:    n.UpdatedAt,
	}, false, nil
}

// buildArtifactDoc emits the search_docs row for an artifact node.
// File-backed artifacts (intents, tasks, design_docs, explore_docs
// whose Path points at a .md file) get their on-disk content folded
// into the body; directory-backed artifacts (project, festival, phase,
// sequence, chain) synthesise a body from name + metadata so FTS still
// matches on e.g. the project name or status.
func buildArtifactDoc(campRoot string, n *graph.Node) (graph.DocumentRecord, bool, error) {
	relPath := artifactRelPath(campRoot, n)
	body := artifactBaseBody(n)
	if content, ok, err := readIfFile(n.Path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", n.ID, err)
			return graph.DocumentRecord{}, true, nil
		}
		return graph.DocumentRecord{}, false, graphErrors.Wrapf(err, "read artifact body for %s (%s)", n.ID, n.Path)
	} else if ok {
		body = body + "\n" + string(content)
	}
	title := firstNonEmpty(n.Metadata[graph.MetaNoteTitle], n.Name)
	scope := inferScopeFromRel(relPath)
	if relPath == "" {
		scope = "."
	}
	return graph.DocumentRecord{
		NodeID:       n.ID,
		Title:        title,
		RelPath:      relPath,
		Scope:        scope,
		Body:         body,
		Aliases:      splitCommaList(n.Metadata[graph.MetaNoteAliases]),
		Tags:         splitCommaList(n.Metadata[graph.MetaNoteTags]),
		TrackedState: firstNonEmpty(n.Metadata[graph.MetaGitState], "tracked"),
		UpdatedAt:    n.UpdatedAt,
	}, false, nil
}

// artifactRelPath returns the campaign-relative path for an artifact
// node. Most artifact nodes store n.Path as an absolute filesystem
// path; we prefer Rel(campRoot, Path) so the result matches what other
// tables (indexed_files, note rel paths) already use. When the input
// is unsuitable the function falls back to n.Name so the search row
// still has a non-empty rel_path.
func artifactRelPath(campRoot string, n *graph.Node) string {
	if campRoot != "" && n.Path != "" && filepath.IsAbs(n.Path) {
		if rel, err := filepath.Rel(campRoot, n.Path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return n.Name
}

// artifactBaseBody synthesises a search body from the artifact's
// identity fields so the node is discoverable even when it has no
// backing file. Keeping each field on its own line encourages FTS5's
// tokenizer to index them as separate terms rather than one long run.
func artifactBaseBody(n *graph.Node) string {
	parts := []string{n.Name}
	if n.Status != "" {
		parts = append(parts, "status: "+n.Status)
	}
	if t := n.Metadata[graph.MetaNoteType]; t != "" {
		parts = append(parts, "type: "+t)
	}
	return strings.Join(parts, "\n")
}

// readIfFile returns the contents of p when p refers to a regular
// file, along with ok=true. Directories and non-existent paths return
// ok=false with a nil error so the caller can fall back to a synthetic
// body without treating the absence as a failure.
func readIfFile(p string) ([]byte, bool, error) {
	if p == "" {
		return nil, false, nil
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, err
		}
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
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
