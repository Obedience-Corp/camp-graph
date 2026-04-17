package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DocumentRecord is a minimal shape shared with internal/search so that
// SaveFullBuild can insert search_docs rows without importing the
// search package (which would create an import cycle). The search
// package exposes the richer search.Document type; callers convert
// using DocumentRecordFromSearch when invoking SaveFullBuild.
type DocumentRecord struct {
	NodeID       string
	Title        string
	RelPath      string
	Scope        string
	Body         string
	Summary      string
	Aliases      []string
	Tags         []string
	TrackedState string
	UpdatedAt    time.Time
}

// IndexedFileRecord mirrors the indexed_files row shape so
// SaveFullBuild can persist fingerprint state alongside the graph and
// search tables in one transaction. The runtime package exposes a
// richer runtime.IndexedFile type; callers convert before invoking
// SaveFullBuild.
type IndexedFileRecord struct {
	RelPath      string
	RepoRoot     string
	NodeID       string
	TrackedState string
	ContentHash  string
	MtimeNs      int64
	ParserKind   string
	ScopeID      string
	IndexedAt    time.Time
}

// BuildMeta captures the graph_meta keys populated on a full build. It
// mirrors the keys enumerated in the implementation contract.
type BuildMeta struct {
	GraphSchemaVersion string
	PluginVersion      string
	CampaignRoot       string
	BuiltAt            time.Time
	LastRefreshAt      time.Time
	LastRefreshMode    string
	SearchAvailable    bool
}

// SaveFullBuild writes the graph, graph_meta, search_docs (plus
// their FTS mirror), and indexed_files in a single transaction so a
// build leaves the DB in a coherent state even under concurrent
// readers. Callers that have indexed_file state to persist pass it in
// via SaveFullBuildWithIndex; SaveFullBuild keeps the prior
// zero-index signature for backward compatibility.
func SaveFullBuild(ctx context.Context, store *Store, g *Graph, docs []DocumentRecord, meta BuildMeta) error {
	return SaveFullBuildWithIndex(ctx, store, g, docs, nil, meta)
}

// SaveFullBuildWithIndex is SaveFullBuild plus indexed_files rows.
// The indexed slice may be nil when a caller has not yet computed
// fingerprints; refresh and build both pass a populated slice in the
// normal path so status --json reports accurate indexed_files counts
// immediately after a rebuild.
func SaveFullBuildWithIndex(
	ctx context.Context,
	store *Store,
	g *Graph,
	docs []DocumentRecord,
	indexed []IndexedFileRecord,
	meta BuildMeta,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin full-build tx: %w", err)
	}
	defer tx.Rollback()

	if err := wipeBuildState(ctx, tx); err != nil {
		return err
	}
	if err := writeNodesTx(ctx, tx, g); err != nil {
		return err
	}
	if err := writeEdgesTx(ctx, tx, g); err != nil {
		return err
	}
	if err := writeSearchDocsTx(ctx, tx, docs); err != nil {
		return err
	}
	if err := writeIndexedFilesTx(ctx, tx, indexed); err != nil {
		return err
	}
	if err := writeGraphMetaTx(ctx, tx, meta); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit full-build tx: %w", err)
	}
	return nil
}

// wipeBuildState clears tables that full builds replace wholesale.
func wipeBuildState(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`DELETE FROM search_docs_fts`,
		`DELETE FROM search_docs`,
		`DELETE FROM indexed_files`,
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`DELETE FROM graph_meta`,
	}
	for _, s := range statements {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("wipe %q: %w", s, err)
		}
	}
	return nil
}

func writeNodesTx(ctx context.Context, tx *sql.Tx, g *Graph) error {
	for _, n := range g.Nodes() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		metaJSON, _ := json.Marshal(n.Metadata)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO nodes (id, type, name, path, status, metadata, created_at, updated_at)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, string(n.Type), n.Name, n.Path, n.Status, string(metaJSON), n.CreatedAt, n.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}
	return nil
}

func writeEdgesTx(ctx context.Context, tx *sql.Tx, g *Graph) error {
	for _, e := range g.Edges() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO edges (from_id, to_id, type, confidence, source, subtype, note, created_at)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.FromID, e.ToID, string(e.Type), e.Confidence, string(e.Source), e.Subtype, e.Note, e.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert edge %s->%s: %w", e.FromID, e.ToID, err)
		}
	}
	return nil
}

func writeSearchDocsTx(ctx context.Context, tx *sql.Tx, docs []DocumentRecord) error {
	for _, d := range docs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		aliases, err := json.Marshal(nonNilStringSlice(d.Aliases))
		if err != nil {
			return fmt.Errorf("marshal aliases for %s: %w", d.NodeID, err)
		}
		tags, err := json.Marshal(nonNilStringSlice(d.Tags))
		if err != nil {
			return fmt.Errorf("marshal tags for %s: %w", d.NodeID, err)
		}
		updatedAt := d.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		}
		// Use RETURNING rowid to avoid an extra SELECT round-trip; the
		// rowid is needed to keep the external-content FTS5 mirror in
		// sync with the logical row.
		var rowID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO search_docs (node_id, title, rel_path, scope, body, summary, aliases, tags, tracked_state, updated_at)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	         RETURNING rowid`,
			d.NodeID, d.Title, d.RelPath, d.Scope, d.Body, d.Summary,
			string(aliases), string(tags), d.TrackedState, updatedAt,
		).Scan(&rowID); err != nil {
			return fmt.Errorf("insert search_doc %s: %w", d.NodeID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_docs_fts(rowid, title, rel_path, scope, body, summary, aliases, tags)
	         VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			rowID, d.Title, d.RelPath, d.Scope, d.Body, d.Summary, string(aliases), string(tags),
		); err != nil {
			return fmt.Errorf("fts insert %s: %w", d.NodeID, err)
		}
	}
	return nil
}

func writeIndexedFilesTx(ctx context.Context, tx *sql.Tx, indexed []IndexedFileRecord) error {
	for _, f := range indexed {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		indexedAt := f.IndexedAt
		if indexedAt.IsZero() {
			indexedAt = time.Now().UTC()
		}
		nodeID := any(nil)
		if f.NodeID != "" {
			nodeID = f.NodeID
		}
		scopeID := any(nil)
		if f.ScopeID != "" {
			scopeID = f.ScopeID
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO indexed_files
             (rel_path, repo_root, node_id, tracked_state, content_hash, mtime_ns, parser_kind, scope_id, indexed_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.RelPath, f.RepoRoot, nodeID, f.TrackedState,
			f.ContentHash, f.MtimeNs, f.ParserKind, scopeID, indexedAt,
		); err != nil {
			return fmt.Errorf("insert indexed_file %s: %w", f.RelPath, err)
		}
	}
	return nil
}

func writeGraphMetaTx(ctx context.Context, tx *sql.Tx, m BuildMeta) error {
	entries := map[string]string{
		"graph_schema_version": m.GraphSchemaVersion,
		"plugin_version":       m.PluginVersion,
		"campaign_root":        m.CampaignRoot,
		"built_at":             formatTime(m.BuiltAt),
		"last_refresh_at":      formatTime(m.LastRefreshAt),
		"last_refresh_mode":    m.LastRefreshMode,
		"search_available":     boolString(m.SearchAvailable),
	}
	for k, v := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO graph_meta (key, value) VALUES (?, ?)`, k, v,
		); err != nil {
			return fmt.Errorf("set graph_meta %s: %w", k, err)
		}
	}
	return nil
}

func nonNilStringSlice(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// SaveGraph writes the entire graph to the store, replacing existing data.
// The operation is wrapped in a transaction for atomicity.
func SaveGraph(ctx context.Context, store *Store, g *Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM edges"); err != nil {
		return fmt.Errorf("delete edges: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM nodes"); err != nil {
		return fmt.Errorf("delete nodes: %w", err)
	}

	for _, n := range g.Nodes() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		metaJSON, _ := json.Marshal(n.Metadata)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO nodes (id, type, name, path, status, metadata, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, string(n.Type), n.Name, n.Path, n.Status, string(metaJSON), n.CreatedAt, n.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}

	for _, e := range g.Edges() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO edges (from_id, to_id, type, confidence, source, subtype, note, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.FromID, e.ToID, string(e.Type), e.Confidence, string(e.Source), e.Subtype, e.Note, e.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert edge %s->%s: %w", e.FromID, e.ToID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// LoadGraph reads all nodes and edges from the store into a new in-memory Graph.
func LoadGraph(ctx context.Context, store *Store) (*Graph, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	g := New()

	nodes, err := store.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("load nodes: %w", err)
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	edges, err := store.GetAllEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("load edges: %w", err)
	}
	for _, e := range edges {
		g.AddEdge(e)
	}
	return g, nil
}
