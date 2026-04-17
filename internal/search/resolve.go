package search

import (
	"context"
	"database/sql"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// Resolve looks up a node ID using the retrieval-backed order defined
// in IMPLEMENTATION_CONTRACTS.md:
//
//  1. exact node ID match,
//  2. exact relative-path match (any node path column),
//  3. top lexical hit from FTS-backed search.
//
// Returns the resolved node ID plus a reason describing which stage
// produced it ("exact_id", "exact_rel_path", "top_hit"). When nothing
// matches the returned ID is empty and error is nil; callers should
// surface their own "not found" message.
func Resolve(ctx context.Context, db *sql.DB, query string) (nodeID, reason string, err error) {
	if query == "" {
		return "", "", nil
	}
	// 1. Exact node id.
	if id, ok, ferr := exactNodeID(ctx, db, query); ferr != nil {
		return "", "", graphErrors.Wrap(ferr, "resolve exact id")
	} else if ok {
		return id, "exact_id", nil
	}
	// 2. Exact relative-path match (search_docs.rel_path and nodes.path).
	if id, ok, ferr := exactRelPath(ctx, db, query); ferr != nil {
		return "", "", graphErrors.Wrap(ferr, "resolve exact rel path")
	} else if ok {
		return id, "exact_rel_path", nil
	}
	// 3. Top lexical hit. Only meaningful when FTS is available.
	if !FTSAvailable(ctx, db) {
		return "", "", nil
	}
	q := NewQuerier(db)
	results, rerr := q.Search(ctx, QueryOptions{Term: query, Limit: 1})
	if rerr != nil {
		return "", "", graphErrors.Wrap(rerr, "resolve top hit")
	}
	if len(results) > 0 {
		return results[0].NodeID, "top_hit", nil
	}
	return "", "", nil
}

func exactNodeID(ctx context.Context, db *sql.DB, query string) (string, bool, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM nodes WHERE id = ?`, query).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// exactRelPath tries to match query against nodes.path OR search_docs.rel_path.
// Matching on nodes.path keeps artifact nodes (project, festival, etc.)
// addressable by their on-disk path.
func exactRelPath(ctx context.Context, db *sql.DB, query string) (string, bool, error) {
	// Search_docs.rel_path first because notes dominate that column.
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT node_id FROM search_docs WHERE rel_path = ?`, query,
	).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	// Fallback: match against nodes.path as-is or with forward slashes.
	queries := []string{query, strings.TrimPrefix(query, "./")}
	for _, q := range queries {
		err = db.QueryRowContext(ctx, `SELECT id FROM nodes WHERE path = ?`, q).Scan(&id)
		if err == nil {
			return id, true, nil
		}
		if err != sql.ErrNoRows {
			return "", false, err
		}
	}
	return "", false, nil
}
