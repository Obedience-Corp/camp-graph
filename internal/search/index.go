package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// Indexer writes Document rows into search_docs and mirrors them into
// search_docs_fts so FTS5 queries can return snippets and ranked
// results in the same transaction.
type Indexer struct {
	db *sql.DB
}

// NewIndexer returns an Indexer that writes to the given database.
// The database must already have search_docs and search_docs_fts
// present (createTablesSQL in internal/graph/store.go).
func NewIndexer(db *sql.DB) *Indexer {
	return &Indexer{db: db}
}

// UpsertDocument writes a Document into search_docs (inserting or
// replacing by node_id) and keeps search_docs_fts in sync.
//
// The FTS mirror uses the external-content pattern: when a row is
// upserted, we delete any existing FTS row for that rowid and insert
// the replacement. This matches the pattern recommended for
// "content='...' content_rowid='rowid'" virtual tables.
func (idx *Indexer) UpsertDocument(ctx context.Context, doc Document) error {
	if doc.NodeID == "" {
		return graphErrors.New("search: document missing node_id")
	}
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return graphErrors.Wrap(err, "begin upsert tx")
	}
	defer tx.Rollback()

	aliases, err := encodeStringArray(doc.Aliases)
	if err != nil {
		return graphErrors.Wrap(err, "encode aliases")
	}
	tags, err := encodeStringArray(doc.Tags)
	if err != nil {
		return graphErrors.Wrap(err, "encode tags")
	}
	updatedAt := doc.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	// Find any prior rowid so we can delete the corresponding FTS row.
	var priorRowID sql.NullInt64
	var priorTitle, priorRelPath, priorScope, priorBody, priorSummary, priorAliases, priorTags sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT rowid, title, rel_path, scope, body, summary, aliases, tags FROM search_docs WHERE node_id = ?`,
		doc.NodeID,
	).Scan(&priorRowID, &priorTitle, &priorRelPath, &priorScope,
		&priorBody, &priorSummary, &priorAliases, &priorTags); err != nil && err != sql.ErrNoRows {
		return graphErrors.Wrap(err, "lookup prior search_doc rowid")
	}

	// Upsert the logical row.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO search_docs (node_id, title, rel_path, scope, body, summary, aliases, tags, tracked_state, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(node_id) DO UPDATE SET
		   title=excluded.title,
		   rel_path=excluded.rel_path,
		   scope=excluded.scope,
		   body=excluded.body,
		   summary=excluded.summary,
		   aliases=excluded.aliases,
		   tags=excluded.tags,
		   tracked_state=excluded.tracked_state,
		   updated_at=excluded.updated_at`,
		doc.NodeID, doc.Title, doc.RelPath, doc.Scope, doc.Body, doc.Summary,
		aliases, tags, doc.TrackedState, updatedAt,
	); err != nil {
		return graphErrors.Wrapf(err, "upsert search_doc %s", doc.NodeID)
	}

	// Fetch the canonical rowid post-upsert.
	var rowID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT rowid FROM search_docs WHERE node_id = ?`, doc.NodeID,
	).Scan(&rowID); err != nil {
		return graphErrors.Wrap(err, "lookup upserted rowid")
	}

	// Delete any stale FTS row before inserting the fresh one. FTS5
	// external-content deletes require the original indexed values so
	// the term dictionary is adjusted correctly; passing placeholder
	// strings would leave stale postings behind.
	if priorRowID.Valid {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_docs_fts(search_docs_fts, rowid, title, rel_path, scope, body, summary, aliases, tags)
			 VALUES('delete', ?, ?, ?, ?, ?, ?, ?, ?)`,
			priorRowID.Int64,
			priorTitle.String, priorRelPath.String, priorScope.String,
			priorBody.String, priorSummary.String, priorAliases.String, priorTags.String,
		); err != nil {
			return graphErrors.Wrapf(err, "fts delete rowid %d", priorRowID.Int64)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO search_docs_fts(rowid, title, rel_path, scope, body, summary, aliases, tags)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		rowID, doc.Title, doc.RelPath, doc.Scope, doc.Body, doc.Summary, aliases, tags,
	); err != nil {
		return graphErrors.Wrapf(err, "fts insert rowid %d", rowID)
	}

	if err := tx.Commit(); err != nil {
		return graphErrors.Wrap(err, "commit upsert tx")
	}
	return nil
}

// DeleteDocument removes the search_docs row (and its FTS mirror) for
// the given node_id. Missing rows are treated as a no-op.
func (idx *Indexer) DeleteDocument(ctx context.Context, nodeID string) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return graphErrors.Wrap(err, "begin delete tx")
	}
	defer tx.Rollback()

	var rowID sql.NullInt64
	var title, relPath, scope, body, summary, aliases, tags sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT rowid, title, rel_path, scope, body, summary, aliases, tags FROM search_docs WHERE node_id = ?`,
		nodeID,
	).Scan(&rowID, &title, &relPath, &scope, &body, &summary, &aliases, &tags); err != nil {
		if err == sql.ErrNoRows {
			return tx.Commit()
		}
		return graphErrors.Wrap(err, "lookup rowid for delete")
	}
	if rowID.Valid {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_docs_fts(search_docs_fts, rowid, title, rel_path, scope, body, summary, aliases, tags)
			 VALUES('delete', ?, ?, ?, ?, ?, ?, ?, ?)`,
			rowID.Int64,
			title.String, relPath.String, scope.String,
			body.String, summary.String, aliases.String, tags.String,
		); err != nil {
			return graphErrors.Wrapf(err, "fts delete rowid %d", rowID.Int64)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM search_docs WHERE node_id = ?`, nodeID,
	); err != nil {
		return graphErrors.Wrapf(err, "delete search_doc %s", nodeID)
	}
	if err := tx.Commit(); err != nil {
		return graphErrors.Wrap(err, "commit delete tx")
	}
	return nil
}

// LoadDocument retrieves a Document by node_id. Returns nil if absent.
func (idx *Indexer) LoadDocument(ctx context.Context, nodeID string) (*Document, error) {
	row := idx.db.QueryRowContext(ctx,
		`SELECT node_id, title, rel_path, scope, body, summary, aliases, tags, tracked_state, updated_at
		 FROM search_docs WHERE node_id = ?`, nodeID,
	)
	var doc Document
	var aliases, tags string
	if err := row.Scan(
		&doc.NodeID, &doc.Title, &doc.RelPath, &doc.Scope,
		&doc.Body, &doc.Summary, &aliases, &tags,
		&doc.TrackedState, &doc.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, graphErrors.Wrapf(err, "scan search_doc %s", nodeID)
	}
	if err := json.Unmarshal([]byte(aliases), &doc.Aliases); err != nil {
		return nil, graphErrors.Wrap(err, "decode aliases")
	}
	if err := json.Unmarshal([]byte(tags), &doc.Tags); err != nil {
		return nil, graphErrors.Wrap(err, "decode tags")
	}
	return &doc, nil
}

// CountDocuments returns the total number of rows in search_docs.
func (idx *Indexer) CountDocuments(ctx context.Context) (int, error) {
	var n int
	if err := idx.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM search_docs`,
	).Scan(&n); err != nil {
		return 0, graphErrors.Wrap(err, "count search_docs")
	}
	return n, nil
}

// FTSAvailable reports whether search_docs_fts is functional by
// executing a trivial MATCH against an empty corpus. A non-nil error
// here means FTS5 is unusable, which per the implementation contract
// should surface as search_available=false.
//
// We use QueryRowContext and discard sql.ErrNoRows because a fresh
// corpus will legitimately return no rows; the probe is only meant to
// confirm the query plan compiles and the virtual table is resolvable.
func FTSAvailable(ctx context.Context, db *sql.DB) bool {
	var rowid int64
	err := db.QueryRowContext(ctx,
		`SELECT rowid FROM search_docs_fts WHERE search_docs_fts MATCH ? LIMIT 1`,
		"noop",
	).Scan(&rowid)
	if err == nil || err == sql.ErrNoRows {
		return true
	}
	return false
}

// encodeStringArray JSON-encodes a slice of strings for TEXT column
// storage. Nil slices encode to "[]" per the contract default.
func encodeStringArray(values []string) (string, error) {
	if values == nil {
		return "[]", nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
