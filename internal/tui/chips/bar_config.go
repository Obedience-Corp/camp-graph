package chips

// NewTypeChip constructs the NodeType filter chip. "All" is the
// default first option; subsequent options are the NodeType strings
// exposed by internal/graph. Callers pass the concrete NodeType list
// so this package stays independent of internal/graph.
func NewTypeChip(nodeTypes []string) Chip {
	options := make([]string, 0, len(nodeTypes)+1)
	options = append(options, "All")
	options = append(options, nodeTypes...)
	return NewChip("Type", options)
}

// NewTrackedChip constructs the tracked-state filter chip. Defaults to
// "All"; the other two values map to Tracked and Untracked filters on
// search.QueryOptions per the TUI_CONTRACT.md QueryOptions mapping.
func NewTrackedChip() Chip {
	return NewChip("Tracked", []string{"All", "Tracked only", "Untracked only"})
}

// NewModeChip constructs the query-mode filter chip. Defaults to
// "hybrid"; values are the QueryMode strings defined by
// internal/search.
func NewModeChip() Chip {
	return NewChip("Mode", []string{"hybrid", "structural", "explicit", "semantic"})
}
