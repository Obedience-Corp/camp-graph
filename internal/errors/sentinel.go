package errors

import "errors"

// Sentinel errors for common graph error cases.
var (
	// ErrCycleDetected indicates the graph contains a cycle where one is not allowed.
	ErrCycleDetected = errors.New("cycle detected in graph")

	// ErrNodeNotFound indicates a node ID does not exist in the graph.
	ErrNodeNotFound = errors.New("node not found")

	// ErrEdgeConflict indicates a conflicting edge definition.
	ErrEdgeConflict = errors.New("conflicting edge definition")

	// ErrInvalidGraph indicates the graph structure is invalid.
	ErrInvalidGraph = errors.New("invalid graph structure")

	// ErrDuplicateNode indicates a node ID already exists.
	ErrDuplicateNode = errors.New("duplicate node ID")

	// ErrStoreNotFound indicates the graph database does not exist.
	ErrStoreNotFound = errors.New("graph database not found")

	// ErrNoFrontmatter indicates a file has no YAML frontmatter.
	ErrNoFrontmatter = errors.New("no frontmatter")
)
