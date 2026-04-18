// Package search provides document indexing and lexical retrieval
// over the camp-graph SQLite database.
//
// The package operates on two sibling tables managed by
// internal/graph/store.go: search_docs (row-backed metadata plus body)
// and search_docs_fts (FTS5 virtual table mirroring the columns). The
// indexer upserts into search_docs and synchronously mirrors the row
// into search_docs_fts so FTS matches stay consistent.
package search

import "time"

// Schema version constants are published so callers and test fixtures
// can reason about compatibility across releases.
const (
	// GraphSchemaVersion is the persistent graph DB schema version
	// stored in graph_meta.graph_schema_version. Bumps force full
	// rebuilds via the refresh flow.
	GraphSchemaVersion = "graphdb/v2alpha1"
	// QueryResultSchemaVersion is the on-wire schema tag embedded in
	// query --json payloads.
	QueryResultSchemaVersion = "graph-query/v1alpha1"
	// RelatedResultSchemaVersion is the on-wire schema tag embedded
	// in related --json payloads.
	RelatedResultSchemaVersion = "graph-related/v1alpha1"
	// RefreshResultSchemaVersion is the on-wire schema tag embedded
	// in refresh --json payloads.
	RefreshResultSchemaVersion = "graph-refresh/v1alpha1"
	// StatusResultSchemaVersion is the on-wire schema tag embedded
	// in status --json payloads.
	StatusResultSchemaVersion = "graph-status/v1alpha1"
)

// Document is the canonical search document stored in search_docs.
// Aliases and Tags are represented as ordered []string in memory and
// serialized as JSON arrays in TEXT columns per the implementation
// contract.
type Document struct {
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

// QueryResult is a single item returned by a search query.
type QueryResult struct {
	NodeID       string   `json:"node_id"`
	NodeType     string   `json:"node_type"`
	Title        string   `json:"title"`
	RelativePath string   `json:"relative_path"`
	Scope        string   `json:"scope"`
	Snippet      string   `json:"snippet"`
	TrackedState string   `json:"tracked_state"`
	Score        float64  `json:"score"`
	Reasons      []string `json:"reasons"`
}

// QueryResponse is the full payload for `query --json`.
type QueryResponse struct {
	SchemaVersion string        `json:"schema_version"`
	CampaignRoot  string        `json:"campaign_root"`
	Query         string        `json:"query"`
	Mode          string        `json:"mode"`
	Limit         int           `json:"limit"`
	Results       []QueryResult `json:"results"`
}

// RelatedItem is a single item returned by the related-item command.
type RelatedItem struct {
	NodeID       string  `json:"node_id"`
	NodeType     string  `json:"node_type"`
	Title        string  `json:"title"`
	RelativePath string  `json:"relative_path"`
	Scope        string  `json:"scope"`
	Reason       string  `json:"reason"`
	Score        float64 `json:"score"`
}

// RelatedResponse is the full payload for `related --json`.
type RelatedResponse struct {
	SchemaVersion string        `json:"schema_version"`
	CampaignRoot  string        `json:"campaign_root"`
	QueryPath     string        `json:"query_path"`
	Mode          string        `json:"mode"`
	Stale         bool          `json:"stale"`
	Items         []RelatedItem `json:"items"`
}

// StatusResponse is the full payload for `status --json`.
type StatusResponse struct {
	SchemaVersion      string `json:"schema_version"`
	CampaignRoot       string `json:"campaign_root"`
	DBPath             string `json:"db_path"`
	GraphSchemaVersion string `json:"graph_schema_version"`
	PluginVersion      string `json:"plugin_version"`
	BuiltAt            string `json:"built_at"`
	LastRefreshAt      string `json:"last_refresh_at"`
	LastRefreshMode    string `json:"last_refresh_mode"`
	SearchAvailable    bool   `json:"search_available"`
	Stale              bool   `json:"stale"`
	IndexedFiles       int    `json:"indexed_files"`
	Nodes              int    `json:"nodes"`
	Edges              int    `json:"edges"`
}

// RefreshResponse is the full payload for `refresh --json`.
type RefreshResponse struct {
	SchemaVersion     string `json:"schema_version"`
	CampaignRoot      string `json:"campaign_root"`
	DBPath            string `json:"db_path"`
	Mode              string `json:"mode"`
	ReindexedFiles    int    `json:"reindexed_files"`
	DeletedFiles      int    `json:"deleted_files"`
	NodesWritten      int    `json:"nodes_written"`
	EdgesWritten      int    `json:"edges_written"`
	SearchDocsWritten int    `json:"search_docs_written"`
	DurationMs        int64  `json:"duration_ms"`
	StaleBefore       bool   `json:"stale_before"`
	StaleAfter        bool   `json:"stale_after"`
}
