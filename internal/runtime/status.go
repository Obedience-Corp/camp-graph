package runtime

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// Status is the data payload behind `camp-graph status --json`. It is
// assembled from graph_meta and row counts against the live database.
type Status struct {
	CampaignRoot       string
	DBPath             string
	GraphSchemaVersion string
	PluginVersion      string
	BuiltAt            string
	LastRefreshAt      string
	LastRefreshMode    string
	SearchAvailable    bool
	Stale              bool
	IndexedFiles       int
	Nodes              int
	Edges              int
}

// LoadStatus reads the persisted graph_meta keys and row counts from
// the database and returns a Status ready to serialize. Missing
// optional fields remain empty string rather than placeholder values.
func LoadStatus(ctx context.Context, db *sql.DB, campaignRoot, dbPath string) (*Status, error) {
	s := &Status{CampaignRoot: campaignRoot, DBPath: dbPath}
	meta, err := loadAllMeta(ctx, db)
	if err != nil {
		return nil, err
	}
	s.GraphSchemaVersion = meta["graph_schema_version"]
	s.PluginVersion = meta["plugin_version"]
	s.BuiltAt = meta["built_at"]
	s.LastRefreshAt = meta["last_refresh_at"]
	s.LastRefreshMode = meta["last_refresh_mode"]
	s.SearchAvailable = boolFromMeta(meta["search_available"])

	counts, err := loadCounts(ctx, db)
	if err != nil {
		return nil, err
	}
	s.Nodes = counts.nodes
	s.Edges = counts.edges
	s.IndexedFiles = counts.indexedFiles

	// A rough staleness heuristic: if last_refresh_at is empty but the
	// database has rows, treat as stale; otherwise it's fresh. Refresh
	// will override this with a real computation when it runs.
	if s.LastRefreshAt == "" && s.Nodes > 0 {
		s.Stale = true
	}
	return s, nil
}

type counts struct {
	nodes        int
	edges        int
	indexedFiles int
}

func loadCounts(ctx context.Context, db *sql.DB) (counts, error) {
	var c counts
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&c.nodes); err != nil {
		return c, graphErrors.Wrap(err, "count nodes")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM edges`).Scan(&c.edges); err != nil {
		return c, graphErrors.Wrap(err, "count edges")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM indexed_files`).Scan(&c.indexedFiles); err != nil {
		return c, graphErrors.Wrap(err, "count indexed_files")
	}
	return c, nil
}

func loadAllMeta(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM graph_meta`)
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

// UpdateRefreshMeta records the timestamp and mode of the most recent
// refresh operation. Mode is "refresh" or "rebuild".
func UpdateRefreshMeta(ctx context.Context, db *sql.DB, mode string, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	entries := map[string]string{
		"last_refresh_at":   at.Format(time.RFC3339),
		"last_refresh_mode": mode,
	}
	for k, v := range entries {
		if _, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO graph_meta (key, value) VALUES (?, ?)`, k, v,
		); err != nil {
			return graphErrors.Wrapf(err, "set %s", k)
		}
	}
	return nil
}

// boolFromMeta parses a stored "true"/"false" meta value into a Go
// boolean. Missing values default to false.
func boolFromMeta(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
