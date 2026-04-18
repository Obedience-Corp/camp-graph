package graph

import (
	"context"
	"database/sql"
	"encoding/json"

	_ "modernc.org/sqlite"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// Store provides SQLite-backed persistence for graph nodes and edges.
type Store struct {
	db *sql.DB
}

const createTablesSQL = `
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT,
    status TEXT,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);

CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id TEXT NOT NULL REFERENCES nodes(id),
    to_id TEXT NOT NULL REFERENCES nodes(id),
    type TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    source TEXT,
    subtype TEXT,
    note TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(from_id, to_id, type)
);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);

CREATE TABLE IF NOT EXISTS graph_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS indexed_files (
    rel_path TEXT PRIMARY KEY,
    repo_root TEXT NOT NULL,
    node_id TEXT,
    tracked_state TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    mtime_ns INTEGER NOT NULL,
    parser_kind TEXT NOT NULL,
    scope_id TEXT,
    indexed_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_indexed_files_repo_root ON indexed_files(repo_root);
CREATE INDEX IF NOT EXISTS idx_indexed_files_scope_id ON indexed_files(scope_id);

CREATE TABLE IF NOT EXISTS search_docs (
    rowid INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL UNIQUE REFERENCES nodes(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    rel_path TEXT NOT NULL,
    scope TEXT NOT NULL,
    body TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    aliases TEXT NOT NULL DEFAULT '[]',
    tags TEXT NOT NULL DEFAULT '[]',
    tracked_state TEXT NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_search_docs_rel_path ON search_docs(rel_path);

CREATE VIRTUAL TABLE IF NOT EXISTS search_docs_fts USING fts5(
    title,
    rel_path,
    scope,
    body,
    summary,
    aliases,
    tags,
    content='search_docs',
    content_rowid='rowid'
);
`

// currentSchemaVersion is the graph_meta.graph_schema_version value
// the current binary writes when it creates the tables in
// createTablesSQL. Any DB whose stored version differs from this is
// considered incompatible and its tables are dropped and recreated
// before they are used.
//
// The graph cache is derived state — a full `camp-graph build`
// regenerates it from the campaign filesystem — so destroying it on
// schema drift is strictly preferable to silently running against a
// stale column layout.
const currentSchemaVersion = "graphdb/v2alpha1"

// allManagedTables lists every table and virtual table defined in
// createTablesSQL. DROP order matters: indexes and virtual tables are
// dropped implicitly with their parent table, so we only list real
// tables. FTS5 external-content virtual tables must be dropped before
// their content-table is gone, so search_docs_fts is listed first.
var allManagedTables = []string{
	"search_docs_fts",
	"search_docs",
	"indexed_files",
	"graph_meta",
	"edges",
	"nodes",
}

// OpenStore opens or creates a SQLite database at path and ensures tables exist.
func OpenStore(ctx context.Context, path string) (*Store, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, graphErrors.NewStore("open", path, err)
	}
	// Enable foreign-key enforcement so CASCADE declarations actually
	// fire. modernc/sqlite defaults this to OFF, which silently breaks
	// the search_docs.node_id -> nodes(id) ON DELETE CASCADE contract.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, graphErrors.NewStore("pragma foreign_keys", path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, graphErrors.NewStore("pragma journal_mode", path, err)
	}
	if err := migrateSchemaIfNeeded(ctx, db); err != nil {
		db.Close()
		return nil, graphErrors.NewStore("migrate schema", path, err)
	}
	if _, err := db.ExecContext(ctx, createTablesSQL); err != nil {
		db.Close()
		return nil, graphErrors.NewStore("create tables", path, err)
	}
	return &Store{db: db}, nil
}

// migrateSchemaIfNeeded reads the persisted graph_schema_version and,
// when it diverges from currentSchemaVersion, drops every managed
// table so the subsequent CREATE TABLE IF NOT EXISTS statements
// install the current schema cleanly.
//
// This is why the constant lives in graph rather than runtime: the
// graph cache is derived state, and OpenStore is the single choke
// point through which every writer and reader must pass. Doing the
// drift check here guarantees the schema on disk matches the schema
// the code expects, regardless of which entry point opened the DB.
func migrateSchemaIfNeeded(ctx context.Context, db *sql.DB) error {
	// Detect whether the graph_meta table already exists. If it does
	// not, the DB is either fresh or was created by a pre-migration
	// build and we can proceed to CREATE TABLE IF NOT EXISTS directly.
	var metaExists int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='graph_meta'`,
	).Scan(&metaExists); err != nil {
		return graphErrors.Wrap(err, "probe graph_meta table")
	}
	if metaExists == 0 {
		return nil
	}
	var version string
	err := db.QueryRowContext(ctx,
		`SELECT value FROM graph_meta WHERE key = 'graph_schema_version'`,
	).Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return graphErrors.Wrap(err, "read graph_schema_version")
	}
	if version == "" || version == currentSchemaVersion {
		return nil
	}
	// Schema drift: drop every managed table so CREATE TABLE IF NOT
	// EXISTS below installs the current layout. The drop is wrapped
	// in a transaction so a failure mid-way leaves no partial state.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return graphErrors.Wrap(err, "begin schema-drop tx")
	}
	defer tx.Rollback()
	for _, name := range allManagedTables {
		if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
			return graphErrors.Wrapf(err, "drop %s", name)
		}
	}
	if err := tx.Commit(); err != nil {
		return graphErrors.Wrap(err, "commit schema-drop tx")
	}
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// InsertNode stores a node in the database. Metadata is marshalled to JSON.
func (s *Store) InsertNode(ctx context.Context, n *Node) error {
	metaJSON, err := json.Marshal(n.Metadata)
	if err != nil {
		return graphErrors.Wrapf(err, "marshal metadata for %s", n.ID)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO nodes (id, type, name, path, status, metadata, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, string(n.Type), n.Name, n.Path, n.Status, string(metaJSON), n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return graphErrors.Wrapf(err, "insert node %s", n.ID)
	}
	return nil
}

// InsertEdge stores an edge in the database.
func (s *Store) InsertEdge(ctx context.Context, e *Edge) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO edges (from_id, to_id, type, confidence, source, subtype, note, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.FromID, e.ToID, string(e.Type), e.Confidence, string(e.Source), e.Subtype, e.Note, e.CreatedAt,
	)
	if err != nil {
		return graphErrors.Wrapf(err, "insert edge %s->%s", e.FromID, e.ToID)
	}
	return nil
}

// GetNode retrieves a single node by ID. Returns nil if not found.
func (s *Store) GetNode(ctx context.Context, id string) (*Node, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, type, name, path, status, metadata, created_at, updated_at FROM nodes WHERE id = ?`, id)
	n := &Node{}
	var metaJSON string
	err := row.Scan(&n.ID, &n.Type, &n.Name, &n.Path, &n.Status, &metaJSON, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, graphErrors.Wrapf(err, "scan node %s", id)
	}
	if metaJSON != "" {
		if err := json.Unmarshal([]byte(metaJSON), &n.Metadata); err != nil {
			return nil, graphErrors.Wrapf(err, "unmarshal metadata for %s", id)
		}
	}
	return n, nil
}

// GetNodesByType returns all nodes of the given type.
func (s *Store) GetNodesByType(ctx context.Context, t NodeType) ([]*Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, name, path, status, metadata, created_at, updated_at FROM nodes WHERE type = ?`, string(t))
	if err != nil {
		return nil, graphErrors.Wrapf(err, "query nodes by type %s", t)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetAllNodes returns every node in the database.
func (s *Store) GetAllNodes(ctx context.Context) ([]*Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, name, path, status, metadata, created_at, updated_at FROM nodes`)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query all nodes")
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetAllEdges returns every edge in the database.
func (s *Store) GetAllEdges(ctx context.Context) ([]*Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT from_id, to_id, type, confidence, source, subtype, note, created_at FROM edges`)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query all edges")
	}
	defer rows.Close()
	var edges []*Edge
	for rows.Next() {
		e := &Edge{}
		if err := rows.Scan(&e.FromID, &e.ToID, &e.Type, &e.Confidence, &e.Source, &e.Subtype, &e.Note, &e.CreatedAt); err != nil {
			return nil, graphErrors.Wrap(err, "scan edge row")
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// DeleteAll removes all nodes and edges from the database.
func (s *Store) DeleteAll(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM edges"); err != nil {
		return graphErrors.Wrap(err, "delete edges")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM nodes"); err != nil {
		return graphErrors.Wrap(err, "delete nodes")
	}
	return nil
}

// DB returns the underlying *sql.DB for packages that need to issue
// their own queries (for example, the search package). Callers must not
// close the returned handle; Store.Close remains the owner.
func (s *Store) DB() *sql.DB {
	return s.db
}

// SetMeta inserts or replaces a graph_meta key/value pair.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO graph_meta (key, value) VALUES (?, ?)`,
		key, value,
	)
	if err != nil {
		return graphErrors.Wrapf(err, "set meta %s", key)
	}
	return nil
}

// GetMeta returns the value for key, or "" if the key is absent.
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT value FROM graph_meta WHERE key = ?`, key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", graphErrors.Wrapf(err, "get meta %s", key)
	}
	return value, nil
}

// AllMeta returns every graph_meta row as a map.
func (s *Store) AllMeta(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM graph_meta`)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query graph_meta")
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, graphErrors.Wrap(err, "scan graph_meta")
		}
		out[k] = v
	}
	return out, rows.Err()
}

// scanNodes is a helper that scans rows into Node slices.
func scanNodes(rows *sql.Rows) ([]*Node, error) {
	var nodes []*Node
	for rows.Next() {
		n := &Node{}
		var metaJSON string
		if err := rows.Scan(&n.ID, &n.Type, &n.Name, &n.Path, &n.Status, &metaJSON, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, graphErrors.Wrap(err, "scan node row")
		}
		if metaJSON != "" {
			if err := json.Unmarshal([]byte(metaJSON), &n.Metadata); err != nil {
				return nil, graphErrors.Wrapf(err, "unmarshal metadata for %s", n.ID)
			}
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
