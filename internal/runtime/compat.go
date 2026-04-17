package runtime

import (
	"context"
	"database/sql"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// CompatibilityVerdict describes how an existing graph DB compares to
// the current release's schema expectations.
type CompatibilityVerdict string

const (
	// CompatFresh means the DB has no graph_schema_version yet and
	// should be treated as a first-time build.
	CompatFresh CompatibilityVerdict = "fresh"
	// CompatMatching means the persisted schema matches the current
	// release and incremental refresh is safe.
	CompatMatching CompatibilityVerdict = "matching"
	// CompatIncompatible means the persisted schema disagrees with
	// the current release and the caller must rebuild from scratch.
	CompatIncompatible CompatibilityVerdict = "incompatible"
)

// KnownCompatibleSchemas enumerates schema strings that the current
// release accepts. graphdb/v2alpha1 is the first upgraded schema
// string for the workspace-graph redesign; older strings (or missing
// values) force a rebuild.
var KnownCompatibleSchemas = map[string]bool{
	"graphdb/v2alpha1": true,
}

// CheckCompatibility inspects graph_meta.graph_schema_version and
// returns a verdict used by the refresh and status commands.
func CheckCompatibility(ctx context.Context, db *sql.DB) (CompatibilityVerdict, string, error) {
	row := db.QueryRowContext(ctx, `SELECT value FROM graph_meta WHERE key = 'graph_schema_version'`)
	var schema string
	err := row.Scan(&schema)
	if err != nil && err != sql.ErrNoRows {
		return CompatIncompatible, "", graphErrors.Wrap(err, "read graph_schema_version")
	}
	if schema == "" {
		return CompatFresh, "", nil
	}
	if schema == search.GraphSchemaVersion && KnownCompatibleSchemas[schema] {
		return CompatMatching, schema, nil
	}
	return CompatIncompatible, schema, nil
}
