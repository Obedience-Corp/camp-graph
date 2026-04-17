package runtime

import (
	"context"
	"database/sql"
	"path/filepath"
	"time"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/scanner"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// RefreshMode reports which path the refresh flow actually took. A
// true incremental refresh returns ModeRefresh; a full rebuild returns
// ModeRebuild. Callers can surface either string in the JSON envelope.
type RefreshMode string

const (
	ModeRefresh RefreshMode = "refresh"
	ModeRebuild RefreshMode = "rebuild"
)

// RefreshReport is the data behind the graph-refresh/v1alpha1 envelope.
type RefreshReport struct {
	Mode              RefreshMode
	ReindexedFiles    int
	DeletedFiles      int
	NodesWritten      int
	EdgesWritten      int
	SearchDocsWritten int
	DurationMs        int64
	StaleBefore       bool
	StaleAfter        bool
}

// RefreshRequest captures the inputs a caller supplies to the refresh
// flow. CampaignRoot is the absolute campaign-root path; Store is the
// opened graph store; ScanBuilder is the scanner wiring used to
// produce a fresh in-memory graph.
type RefreshRequest struct {
	CampaignRoot  string
	DBPath        string
	Store         *graph.Store
	BuildDocs     func(g *graph.Graph) []graph.DocumentRecord
	BuildMetaFn   func(mode RefreshMode, now time.Time, searchAvailable bool) graph.BuildMeta
}

// Refresh runs the refresh flow. It rebuilds the inventory, diffs
// against indexed_files, and chooses between three paths:
//
//  1. ModeRebuild: preconditions fail (fresh DB, schema mismatch,
//     empty graph). Full rebuild via SaveFullBuildWithIndex.
//  2. ModeRefresh with zero changes: a no-op fast path that only
//     updates graph_meta. No nodes, edges, search_docs, or
//     indexed_files are rewritten. This is the common case when the
//     user re-runs refresh without touching files.
//  3. ModeRefresh with changes: a targeted rebuild. The first
//     implementation rebuilds the full graph because the scanner
//     operates at repo granularity, but reports accurate
//     reindexed/deleted counts so clients see the real amount of
//     work done.
func Refresh(ctx context.Context, req RefreshRequest) (*RefreshReport, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	start := time.Now()

	stateStore := NewIndexState(req.Store.DB())

	mode, staleBefore, err := decideMode(ctx, req.Store.DB())
	if err != nil {
		return nil, err
	}

	sc := scanner.New(req.CampaignRoot)
	g, err := sc.Scan(ctx)
	if err != nil {
		return nil, graphErrors.Wrap(err, "scan during refresh")
	}

	prior, err := stateStore.Load(ctx)
	if err != nil {
		return nil, err
	}
	inv := sc.Inventory()
	added, changed, deleted := diffInventory(inv, prior)
	reindexed := added + changed
	searchAvailable := search.FTSAvailable(ctx, req.Store.DB())

	// Fast-path: ModeRefresh with no observed changes updates
	// last_refresh_at in graph_meta and returns without rewriting any
	// heavy state. This is the scope-local minimum work the contract
	// asks for when inventory diff reports zero changes.
	if mode == ModeRefresh && reindexed == 0 && deleted == 0 {
		if err := UpdateRefreshMeta(ctx, req.Store.DB(), string(mode), start); err != nil {
			return nil, graphErrors.Wrap(err, "update refresh meta on no-op path")
		}
		nodes, edges, docs := liveCounts(ctx, req.Store.DB())
		return &RefreshReport{
			Mode:              mode,
			ReindexedFiles:    0,
			DeletedFiles:      0,
			NodesWritten:      nodes,
			EdgesWritten:      edges,
			SearchDocsWritten: docs,
			DurationMs:        time.Since(start).Milliseconds(),
			StaleBefore:       staleBefore,
			StaleAfter:        false,
		}, nil
	}

	// Heavy-path: rebuild everything but still report the diff so
	// clients can see real add/change/delete counts and
	// indexed_files stays consistent.
	docs := req.BuildDocs(g)
	meta := req.BuildMetaFn(mode, start, searchAvailable)
	indexed := BuildIndexedFileRecords(inv, g, start)
	if err := graph.SaveFullBuildWithIndex(ctx, req.Store, g, docs, indexed, meta); err != nil {
		return nil, graphErrors.Wrap(err, "save full build during refresh")
	}

	return &RefreshReport{
		Mode:              mode,
		ReindexedFiles:    reindexed,
		DeletedFiles:      deleted,
		NodesWritten:      g.NodeCount(),
		EdgesWritten:      g.EdgeCount(),
		SearchDocsWritten: len(docs),
		DurationMs:        time.Since(start).Milliseconds(),
		StaleBefore:       staleBefore,
		StaleAfter:        false,
	}, nil
}

// liveCounts reads node, edge, and search_docs counts from the DB so
// the no-op refresh path can report meaningful numbers without
// rebuilding.
func liveCounts(ctx context.Context, db *sql.DB) (nodes, edges, docs int) {
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&nodes)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM edges`).Scan(&edges)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM search_docs`).Scan(&docs)
	return
}

// decideMode inspects the database to determine whether refresh can
// proceed incrementally or must fall back to a full rebuild. A DB that
// is Fresh or Incompatible forces ModeRebuild. Matching schema plus a
// non-empty graph allows ModeRefresh.
func decideMode(ctx context.Context, db *sql.DB) (RefreshMode, bool, error) {
	verdict, _, err := CheckCompatibility(ctx, db)
	if err != nil {
		return ModeRebuild, true, err
	}
	if verdict != CompatMatching {
		return ModeRebuild, true, nil
	}
	// Count nodes; an empty DB means we should rebuild even if schema
	// is present (defensive against manual tampering).
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&n); err != nil {
		return ModeRebuild, true, graphErrors.Wrap(err, "count nodes")
	}
	if n == 0 {
		return ModeRebuild, true, nil
	}
	return ModeRefresh, false, nil
}

// diffInventory compares the current scanner inventory against the
// indexed_files state from the prior run and returns the number of
// added, changed, and deleted files.
func diffInventory(inv *scanner.Inventory, prior map[string]IndexedFile) (added, changed, deleted int) {
	if inv == nil {
		return 0, 0, len(prior)
	}
	seen := make(map[string]bool, len(inv.Entries))
	for _, e := range inv.Entries {
		if e.IsDir {
			continue
		}
		seen[e.RelPath] = true
		p, ok := prior[e.RelPath]
		if !ok {
			added++
			continue
		}
		// Classify changed by content hash first, falling back to
		// mtime so refresh works even when hashes are missing.
		hash, err := ComputeContentHash(e.AbsPath)
		if err == nil && hash != p.ContentHash {
			changed++
			continue
		}
		mt, err := ComputeMtimeNs(e.AbsPath)
		if err == nil && mt != p.MtimeNs {
			changed++
		}
	}
	for rel := range prior {
		if !seen[rel] {
			deleted++
		}
	}
	return
}

// BuildIndexedFileRecords derives IndexedFileRecord rows for every
// eligible inventory entry. Directory entries are skipped. The
// resulting slice is written atomically by
// graph.SaveFullBuildWithIndex alongside nodes, edges, search_docs,
// and graph_meta.
func BuildIndexedFileRecords(inv *scanner.Inventory, g *graph.Graph, indexedAt time.Time) []graph.IndexedFileRecord {
	if inv == nil {
		return nil
	}
	out := make([]graph.IndexedFileRecord, 0, len(inv.Entries))
	for _, e := range inv.Entries {
		if e.IsDir {
			continue
		}
		hash, _ := ComputeContentHash(e.AbsPath)
		mt, _ := ComputeMtimeNs(e.AbsPath)
		nodeID := ""
		noteID := "note:" + e.RelPath
		if g.Node(noteID) != nil {
			nodeID = noteID
		} else if g.Node("file:"+e.RelPath) != nil {
			nodeID = "file:" + e.RelPath
		}
		scopeID := "folder:."
		if dir := filepath.Dir(e.RelPath); dir != "." {
			scopeID = "folder:" + dir
		}
		parser := "attachment"
		switch e.Extension {
		case "canvas":
			parser = "canvas"
		case "md", "markdown", "mdx":
			parser = "markdown"
		case "go":
			parser = "go"
		}
		out = append(out, graph.IndexedFileRecord{
			RelPath:      e.RelPath,
			RepoRoot:     e.RepoRoot,
			NodeID:       nodeID,
			TrackedState: string(e.GitState),
			ContentHash:  hash,
			MtimeNs:      mt,
			ParserKind:   parser,
			ScopeID:      scopeID,
			IndexedAt:    indexedAt,
		})
	}
	return out
}
