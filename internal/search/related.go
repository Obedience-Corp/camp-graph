package search

import (
	"context"
	"database/sql"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// RelatedOptions captures flags for the related command.
type RelatedOptions struct {
	// Path is campaign-relative and identifies the source document.
	Path string
	// Mode controls which edge sources and signals contribute to the
	// related set. Defaults to hybrid.
	Mode QueryMode
	// Limit caps the number of items returned.
	Limit int
}

// Related returns workspace nodes related to opts.Path. Candidates
// come from three sources, ordered by reliability:
//
//  1. explicit outgoing and incoming edges in the graph,
//  2. scope-local neighbors (same folder, same artifact subtree),
//  3. top lexical hits ranked against the source document's title and
//     scope.
//
// Duplicate node IDs are collapsed, preserving the first occurrence's
// reason. The function never crosses campaign boundaries.
func Related(ctx context.Context, db *sql.DB, opts RelatedOptions) ([]RelatedItem, error) {
	if opts.Path == "" {
		return nil, graphErrors.New("related: --path is required")
	}
	if opts.Mode == "" {
		opts.Mode = QueryModeHybrid
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	sourceID, _, err := Resolve(ctx, db, opts.Path)
	if err != nil {
		return nil, graphErrors.Wrap(err, "resolve source path")
	}
	if sourceID == "" {
		return nil, nil
	}

	var items []RelatedItem
	seen := map[string]bool{sourceID: true}

	// Ranking order (per IMPLEMENTATION_CONTRACTS "same scope, explicit
	// link, semantic"): scope neighbors first because they are the
	// strongest locality signal, then explicit links, then semantic
	// (lexical) top-up.
	if opts.Mode == QueryModeHybrid || opts.Mode == QueryModeStructural {
		scopeItems, err := scopeNeighbors(ctx, db, sourceID, opts.Path)
		if err != nil {
			return nil, err
		}
		items = appendUnique(items, scopeItems, seen, limit)
		if len(items) >= limit {
			return items[:limit], nil
		}
	}

	if opts.Mode == QueryModeHybrid || opts.Mode == QueryModeExplicit {
		edgeItems, err := explicitNeighbors(ctx, db, sourceID)
		if err != nil {
			return nil, err
		}
		items = appendUnique(items, edgeItems, seen, limit)
		if len(items) >= limit {
			return items[:limit], nil
		}
	}

	if opts.Mode == QueryModeHybrid || opts.Mode == QueryModeSemantic {
		lexItems, err := lexicalNeighbors(ctx, db, sourceID, opts.Path, limit-len(items))
		if err != nil {
			return nil, err
		}
		items = appendUnique(items, lexItems, seen, limit)
	}

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func explicitNeighbors(ctx context.Context, db *sql.DB, sourceID string) ([]RelatedItem, error) {
	rows, err := db.QueryContext(ctx, `
SELECT n.id, n.type, COALESCE(sd.title, n.name), COALESCE(sd.rel_path, n.path), COALESCE(sd.scope, ''), e.confidence
FROM edges AS e
JOIN nodes AS n ON n.id = e.to_id
LEFT JOIN search_docs AS sd ON sd.node_id = n.id
WHERE e.from_id = ? AND e.source = 'explicit'
UNION ALL
SELECT n.id, n.type, COALESCE(sd.title, n.name), COALESCE(sd.rel_path, n.path), COALESCE(sd.scope, ''), e.confidence
FROM edges AS e
JOIN nodes AS n ON n.id = e.from_id
LEFT JOIN search_docs AS sd ON sd.node_id = n.id
WHERE e.to_id = ? AND e.source = 'explicit'
`, sourceID, sourceID)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query explicit neighbors")
	}
	defer rows.Close()

	var out []RelatedItem
	for rows.Next() {
		var it RelatedItem
		if err := rows.Scan(&it.NodeID, &it.NodeType, &it.Title, &it.RelativePath, &it.Scope, &it.Score); err != nil {
			return nil, graphErrors.Wrap(err, "scan explicit neighbor")
		}
		it.Reason = "explicit_edge"
		out = append(out, it)
	}
	return out, rows.Err()
}

func scopeNeighbors(ctx context.Context, db *sql.DB, sourceID, path string) ([]RelatedItem, error) {
	scope := parentDir(path)
	if scope == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
SELECT sd.node_id, n.type, sd.title, sd.rel_path, sd.scope, 0.7
FROM search_docs AS sd
JOIN nodes AS n ON n.id = sd.node_id
WHERE sd.scope = ? AND sd.node_id != ?
`, scope, sourceID)
	if err != nil {
		return nil, graphErrors.Wrap(err, "query scope neighbors")
	}
	defer rows.Close()

	var out []RelatedItem
	for rows.Next() {
		var it RelatedItem
		if err := rows.Scan(&it.NodeID, &it.NodeType, &it.Title, &it.RelativePath, &it.Scope, &it.Score); err != nil {
			return nil, graphErrors.Wrap(err, "scan scope neighbor")
		}
		it.Reason = "same_scope"
		out = append(out, it)
	}
	return out, rows.Err()
}

func lexicalNeighbors(ctx context.Context, db *sql.DB, sourceID, path string, budget int) ([]RelatedItem, error) {
	if budget <= 0 || !FTSAvailable(ctx, db) {
		return nil, nil
	}
	// Use the filename stem as the lexical probe to find notes that
	// share tokens with the source document.
	base := baseStem(path)
	if base == "" {
		return nil, nil
	}
	q := NewQuerier(db)
	results, err := q.Search(ctx, QueryOptions{
		Term:  base,
		Limit: budget + 1, // +1 to filter self
	})
	if err != nil {
		return nil, err
	}
	var out []RelatedItem
	for _, r := range results {
		if r.NodeID == sourceID {
			continue
		}
		out = append(out, RelatedItem{
			NodeID:       r.NodeID,
			NodeType:     r.NodeType,
			Title:        r.Title,
			RelativePath: r.RelativePath,
			Scope:        r.Scope,
			Reason:       "lexical_match",
			Score:        r.Score,
		})
	}
	return out, nil
}

func appendUnique(out, in []RelatedItem, seen map[string]bool, limit int) []RelatedItem {
	for _, it := range in {
		if len(out) >= limit {
			break
		}
		if seen[it.NodeID] {
			continue
		}
		seen[it.NodeID] = true
		out = append(out, it)
	}
	return out
}

func parentDir(rel string) string {
	idx := strings.LastIndex(rel, "/")
	if idx < 0 {
		return "."
	}
	return rel[:idx]
}

func baseStem(rel string) string {
	idx := strings.LastIndex(rel, "/")
	if idx >= 0 {
		rel = rel[idx+1:]
	}
	dot := strings.LastIndex(rel, ".")
	if dot > 0 {
		rel = rel[:dot]
	}
	return rel
}
