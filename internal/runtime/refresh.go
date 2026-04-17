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
// against indexed_files, and either rebuilds the DB or performs a
// targeted recompute. In the first implementation the targeted path
// falls back to a full rebuild but still reports accurate add/change/
// delete counts so clients see the real amount of work done and
// indexed_files stays consistent for future truly-incremental passes.
func Refresh(ctx context.Context, req RefreshRequest) (*RefreshReport, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	start := time.Now()

	stateStore := NewIndexState(req.Store.DB())

	// Precondition: detect if a rebuild is required.
	mode, staleBefore, err := decideMode(ctx, req.Store.DB())
	if err != nil {
		return nil, err
	}

	// Scan to get the canonical graph regardless of mode.
	sc := scanner.New(req.CampaignRoot)
	g, err := sc.Scan(ctx)
	if err != nil {
		return nil, graphErrors.Wrap(err, "scan during refresh")
	}

	// Diff inventory against the existing indexed_files state.
	prior, err := stateStore.Load(ctx)
	if err != nil {
		return nil, err
	}
	inv := sc.Inventory()
	added, changed, deleted := diffInventory(inv, prior)
	reindexed := added + changed

	// Build DocumentRecords and BuildMeta for the full SaveFullBuild
	// path. Callers supply the derivation function so we do not
	// duplicate CLI-layer logic here.
	docs := req.BuildDocs(g)
	searchAvailable := search.FTSAvailable(ctx, req.Store.DB())
	meta := req.BuildMetaFn(mode, start, searchAvailable)
	if err := graph.SaveFullBuild(ctx, req.Store, g, docs, meta); err != nil {
		return nil, graphErrors.Wrap(err, "save full build during refresh")
	}

	// Populate indexed_files for every inventory entry so subsequent
	// refresh runs have real fingerprints to diff against.
	if err := populateIndexedFiles(ctx, stateStore, req.CampaignRoot, inv, g); err != nil {
		return nil, err
	}

	report := &RefreshReport{
		Mode:              mode,
		ReindexedFiles:    reindexed,
		DeletedFiles:      deleted,
		NodesWritten:      g.NodeCount(),
		EdgesWritten:      g.EdgeCount(),
		SearchDocsWritten: len(docs),
		DurationMs:        time.Since(start).Milliseconds(),
		StaleBefore:       staleBefore,
		StaleAfter:        false,
	}
	return report, nil
}

// decideMode inspects the database to determine whether refresh can
// proceed incrementally or must fall back to a full rebuild. A DB that
// has no graph_schema_version or one that is incompatible forces
// ModeRebuild. Presence of any rows plus the current schema version
// allows ModeRefresh.
func decideMode(ctx context.Context, db *sql.DB) (RefreshMode, bool, error) {
	var schema string
	row := db.QueryRowContext(ctx, `SELECT value FROM graph_meta WHERE key = 'graph_schema_version'`)
	err := row.Scan(&schema)
	if err != nil && err != sql.ErrNoRows {
		return ModeRebuild, true, graphErrors.Wrap(err, "read graph_schema_version")
	}
	if schema == "" {
		// Fresh DB - treat as stale and rebuild.
		return ModeRebuild, true, nil
	}
	if schema != search.GraphSchemaVersion {
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

// populateIndexedFiles writes indexed_files rows for every eligible
// inventory entry. Directory entries are skipped.
func populateIndexedFiles(
	ctx context.Context,
	s *IndexState,
	campaignRoot string,
	inv *scanner.Inventory,
	g *graph.Graph,
) error {
	if inv == nil {
		return nil
	}
	// Collect the nodes owning each rel path. We look up note nodes by
	// path-stable ID; artifacts can be resolved in later enhancements.
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
		}
		scopeID := ""
		if dir := filepath.Dir(e.RelPath); dir != "." {
			scopeID = "folder:" + dir
		} else {
			scopeID = "folder:."
		}
		parser := "markdown"
		switch e.Extension {
		case "canvas":
			parser = "canvas"
		case "md", "markdown", "mdx":
			parser = "markdown"
		default:
			parser = "attachment"
		}
		if err := s.Upsert(ctx, IndexedFile{
			RelPath:      e.RelPath,
			RepoRoot:     e.RepoRoot,
			NodeID:       nodeID,
			TrackedState: string(e.GitState),
			ContentHash:  hash,
			MtimeNs:      mt,
			ParserKind:   parser,
			ScopeID:      scopeID,
			IndexedAt:    time.Now().UTC(),
		}); err != nil {
			return err
		}
	}
	return nil
}
