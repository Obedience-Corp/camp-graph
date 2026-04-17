// Package runtime holds incremental-refresh state and related
// bookkeeping for the camp-graph plugin. It sits between the scanner
// (which produces Inventory records) and the graph store (which owns
// SQLite persistence) so refresh logic can reason about
// add/change/delete transitions without rewalking the entire graph.
package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"time"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// IndexedFile captures one row in the indexed_files table. It carries
// enough state for the refresh flow to classify a path as added,
// changed, unchanged, or deleted relative to the live worktree.
type IndexedFile struct {
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

// IndexState provides persistence for indexed_files rows. It is safe
// for concurrent use across goroutines because every call operates on
// its own connection.
type IndexState struct {
	db *sql.DB
}

// NewIndexState returns an IndexState bound to the given database.
func NewIndexState(db *sql.DB) *IndexState {
	return &IndexState{db: db}
}

// Upsert inserts or replaces a single indexed_files row.
func (s *IndexState) Upsert(ctx context.Context, f IndexedFile) error {
	indexedAt := f.IndexedAt
	if indexedAt.IsZero() {
		indexedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO indexed_files
         (rel_path, repo_root, node_id, tracked_state, content_hash, mtime_ns, parser_kind, scope_id, indexed_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(rel_path) DO UPDATE SET
           repo_root=excluded.repo_root,
           node_id=excluded.node_id,
           tracked_state=excluded.tracked_state,
           content_hash=excluded.content_hash,
           mtime_ns=excluded.mtime_ns,
           parser_kind=excluded.parser_kind,
           scope_id=excluded.scope_id,
           indexed_at=excluded.indexed_at`,
		f.RelPath, f.RepoRoot, f.NodeID, f.TrackedState,
		f.ContentHash, f.MtimeNs, f.ParserKind, f.ScopeID, indexedAt,
	)
	if err != nil {
		return graphErrors.Wrapf(err, "upsert indexed_file %s", f.RelPath)
	}
	return nil
}

// Delete removes an indexed_files row by rel_path. Missing rows are
// not an error.
func (s *IndexState) Delete(ctx context.Context, relPath string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM indexed_files WHERE rel_path = ?`, relPath)
	if err != nil {
		return graphErrors.Wrapf(err, "delete indexed_file %s", relPath)
	}
	return nil
}

// Load returns every indexed_files row keyed by rel_path. Empty map is
// returned when the table is empty.
func (s *IndexState) Load(ctx context.Context) (map[string]IndexedFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rel_path, repo_root, node_id, tracked_state, content_hash, mtime_ns, parser_kind, scope_id, indexed_at
         FROM indexed_files`)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query indexed_files")
	}
	defer rows.Close()

	out := make(map[string]IndexedFile)
	for rows.Next() {
		var f IndexedFile
		var nodeID, scopeID sql.NullString
		if err := rows.Scan(
			&f.RelPath, &f.RepoRoot, &nodeID, &f.TrackedState,
			&f.ContentHash, &f.MtimeNs, &f.ParserKind, &scopeID, &f.IndexedAt,
		); err != nil {
			return nil, graphErrors.Wrap(err, "scan indexed_file")
		}
		if nodeID.Valid {
			f.NodeID = nodeID.String
		}
		if scopeID.Valid {
			f.ScopeID = scopeID.String
		}
		out[f.RelPath] = f
	}
	return out, rows.Err()
}

// Count returns the number of rows in indexed_files.
func (s *IndexState) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM indexed_files`,
	).Scan(&n); err != nil {
		return 0, graphErrors.Wrap(err, "count indexed_files")
	}
	return n, nil
}

// ComputeContentHash returns the SHA-256 hex digest of the file at
// path. It is used to detect content changes independent of mtime so
// mtime-insensitive file systems do not confuse the refresh flow.
func ComputeContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", graphErrors.Wrapf(err, "open %q", path)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", graphErrors.Wrapf(err, "hash %q", path)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeMtimeNs returns the modification time of path as nanoseconds
// since the Unix epoch.
func ComputeMtimeNs(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, graphErrors.Wrapf(err, "stat %q", path)
	}
	return info.ModTime().UnixNano(), nil
}
