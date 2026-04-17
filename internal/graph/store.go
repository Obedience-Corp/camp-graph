package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
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

// OpenStore opens or creates a SQLite database at path and ensures tables exist.
func OpenStore(ctx context.Context, path string) (*Store, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, createTablesSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// InsertNode stores a node in the database. Metadata is marshalled to JSON.
func (s *Store) InsertNode(ctx context.Context, n *Node) error {
	metaJSON, err := json.Marshal(n.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata for %s: %w", n.ID, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO nodes (id, type, name, path, status, metadata, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, string(n.Type), n.Name, n.Path, n.Status, string(metaJSON), n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert node %s: %w", n.ID, err)
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
		return fmt.Errorf("insert edge %s->%s: %w", e.FromID, e.ToID, err)
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
		return nil, fmt.Errorf("scan node %s: %w", id, err)
	}
	if metaJSON != "" {
		if err := json.Unmarshal([]byte(metaJSON), &n.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata for %s: %w", id, err)
		}
	}
	return n, nil
}

// GetNodesByType returns all nodes of the given type.
func (s *Store) GetNodesByType(ctx context.Context, t NodeType) ([]*Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, name, path, status, metadata, created_at, updated_at FROM nodes WHERE type = ?`, string(t))
	if err != nil {
		return nil, fmt.Errorf("query nodes by type %s: %w", t, err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetAllNodes returns every node in the database.
func (s *Store) GetAllNodes(ctx context.Context) ([]*Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, name, path, status, metadata, created_at, updated_at FROM nodes`)
	if err != nil {
		return nil, fmt.Errorf("query all nodes: %w", err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetAllEdges returns every edge in the database.
func (s *Store) GetAllEdges(ctx context.Context) ([]*Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT from_id, to_id, type, confidence, source, subtype, note, created_at FROM edges`)
	if err != nil {
		return nil, fmt.Errorf("query all edges: %w", err)
	}
	defer rows.Close()
	var edges []*Edge
	for rows.Next() {
		e := &Edge{}
		if err := rows.Scan(&e.FromID, &e.ToID, &e.Type, &e.Confidence, &e.Source, &e.Subtype, &e.Note, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan edge row: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// DeleteAll removes all nodes and edges from the database.
func (s *Store) DeleteAll(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM edges"); err != nil {
		return fmt.Errorf("delete edges: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM nodes"); err != nil {
		return fmt.Errorf("delete nodes: %w", err)
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
		return fmt.Errorf("set meta %s: %w", key, err)
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
		return "", fmt.Errorf("get meta %s: %w", key, err)
	}
	return value, nil
}

// AllMeta returns every graph_meta row as a map.
func (s *Store) AllMeta(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM graph_meta`)
	if err != nil {
		return nil, fmt.Errorf("query graph_meta: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan graph_meta: %w", err)
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
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &n.Metadata)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
