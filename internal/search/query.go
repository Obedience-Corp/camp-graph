package search

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// QueryMode enumerates the relation modes supported by the query
// command. Structural and explicit modes boost scope and explicit-link
// reasons respectively; semantic leans on inferred signals; hybrid
// combines everything at moderate weight.
type QueryMode string

const (
	QueryModeStructural QueryMode = "structural"
	QueryModeExplicit   QueryMode = "explicit"
	QueryModeSemantic   QueryMode = "semantic"
	QueryModeHybrid     QueryMode = "hybrid"
)

// QueryOptions captures the flags accepted by `camp-graph query`.
type QueryOptions struct {
	Term       string
	Type       string
	Scope      string
	PathPrefix string
	Mode       QueryMode
	Tracked    bool
	Untracked  bool
	Limit      int
}

// Querier performs FTS5-backed lexical retrieval against the
// search_docs tables. It is safe to reuse across calls.
type Querier struct {
	db *sql.DB
}

// NewQuerier returns a Querier bound to the given database.
func NewQuerier(db *sql.DB) *Querier {
	return &Querier{db: db}
}

// Search runs a lexical query and returns ranked QueryResult items.
// It never falls back to substring scanning of nodes; if FTS is
// unavailable the caller should surface search_available=false.
func (q *Querier) Search(ctx context.Context, opts QueryOptions) ([]QueryResult, error) {
	if opts.Term == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := buildFTSQuery(opts.Term)
	args := []any{ftsQuery}

	// Build the FTS-backed lexical query with optional scope and
	// path-prefix filters. Tracked/untracked filter on search_docs.tracked_state.
	var filters []string
	if opts.Scope != "" {
		filters = append(filters, "sd.scope = ?")
		args = append(args, opts.Scope)
	}
	if opts.PathPrefix != "" {
		filters = append(filters, "(sd.rel_path = ? OR sd.rel_path LIKE ?)")
		args = append(args, opts.PathPrefix, opts.PathPrefix+"/%")
	}
	if opts.Tracked && !opts.Untracked {
		filters = append(filters, "sd.tracked_state = 'tracked'")
	}
	if opts.Untracked && !opts.Tracked {
		filters = append(filters, "sd.tracked_state = 'untracked'")
	}
	if opts.Type != "" {
		filters = append(filters, "n.type = ?")
		args = append(args, opts.Type)
	}

	where := ""
	if len(filters) > 0 {
		where = " AND " + strings.Join(filters, " AND ")
	}

	// Negative ranks are lower is better; we flip to positive so higher
	// scores mean better matches in the result payload.
	query := `
SELECT
    sd.node_id,
    n.type,
    sd.title,
    sd.rel_path,
    sd.scope,
    sd.tracked_state,
    -bm25(search_docs_fts) AS score,
    snippet(search_docs_fts, 3, '…', '…', '…', 12) AS body_snippet
FROM search_docs_fts
JOIN search_docs AS sd ON sd.rowid = search_docs_fts.rowid
JOIN nodes AS n ON n.id = sd.node_id
WHERE search_docs_fts MATCH ?` + where + `
ORDER BY score DESC
LIMIT ?`
	args = append(args, limit)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, graphErrors.Wrapf(err, "query fts for %q", opts.Term)
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var r QueryResult
		var snippet sql.NullString
		if err := rows.Scan(
			&r.NodeID, &r.NodeType, &r.Title, &r.RelativePath, &r.Scope,
			&r.TrackedState, &r.Score, &snippet,
		); err != nil {
			return nil, graphErrors.Wrap(err, "scan query row")
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		r.Reasons = deriveReasons(opts, r)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, graphErrors.Wrap(err, "iterate query rows")
	}
	return results, nil
}

// buildFTSQuery turns a free-text term into a safe FTS5 MATCH
// expression. It tokenizes on whitespace, quotes each token to avoid
// syntax interpretation, and joins with OR so single-word and
// multi-word inputs behave predictably. Empty tokens are dropped.
func buildFTSQuery(term string) string {
	tokens := strings.Fields(term)
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		cleaned := strings.ReplaceAll(tok, "\"", "")
		if cleaned == "" {
			continue
		}
		parts = append(parts, strconv.Quote(cleaned))
	}
	if len(parts) == 0 {
		// Fallback: quote the whole term as-is.
		return strconv.Quote(term)
	}
	return strings.Join(parts, " OR ")
}

// deriveReasons records why a result was returned. The first version
// returns a narrow set of provenance tags based on filters and the
// match itself; richer graph-aware reasons arrive in later sequences
// that combine structural/explicit boosts.
func deriveReasons(opts QueryOptions, r QueryResult) []string {
	reasons := []string{"fts_match"}
	if opts.Term != "" {
		lowerTerm := strings.ToLower(opts.Term)
		if strings.Contains(strings.ToLower(r.RelativePath), lowerTerm) {
			reasons = append(reasons, "exact_path_token")
		}
		if strings.Contains(strings.ToLower(r.Title), lowerTerm) {
			reasons = append(reasons, "title_match")
		}
	}
	if opts.Scope != "" && opts.Scope == r.Scope {
		reasons = append(reasons, "same_scope")
	}
	return reasons
}

// ParseMode converts a CLI string into a QueryMode. Empty string and
// invalid values default to QueryModeHybrid per the contract.
func ParseMode(s string) QueryMode {
	switch QueryMode(strings.ToLower(s)) {
	case QueryModeStructural, QueryModeExplicit, QueryModeSemantic, QueryModeHybrid:
		return QueryMode(strings.ToLower(s))
	}
	return QueryModeHybrid
}
