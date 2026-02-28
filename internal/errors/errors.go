// Package errors provides typed error types, sentinel errors, and wrapping
// utilities for the camp-graph CLI.
package errors

import "fmt"

// GraphValidationError indicates a graph structure is invalid.
type GraphValidationError struct {
	// Node is the node ID involved, if any.
	Node string
	// Message describes the validation failure.
	Message string
	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *GraphValidationError) Error() string {
	if e.Node != "" && e.Err != nil {
		return fmt.Sprintf("graph validation [%s]: %s: %v", e.Node, e.Message, e.Err)
	}
	if e.Node != "" {
		return fmt.Sprintf("graph validation [%s]: %s", e.Node, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("graph validation: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("graph validation: %s", e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *GraphValidationError) Unwrap() error { return e.Err }

// NewGraphValidation creates a GraphValidationError.
func NewGraphValidation(node, message string, err error) *GraphValidationError {
	return &GraphValidationError{Node: node, Message: message, Err: err}
}

// NodeResolutionError indicates a node reference could not be resolved.
type NodeResolutionError struct {
	// NodeID is the unresolved node identifier.
	NodeID string
	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *NodeResolutionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("node %q not found: %v", e.NodeID, e.Err)
	}
	return fmt.Sprintf("node %q not found", e.NodeID)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *NodeResolutionError) Unwrap() error { return e.Err }

// NewNodeResolution creates a NodeResolutionError.
func NewNodeResolution(nodeID string, err error) *NodeResolutionError {
	return &NodeResolutionError{NodeID: nodeID, Err: err}
}

// ParseError indicates a parsing failure in graph definition files.
type ParseError struct {
	// File is the file that failed to parse.
	File string
	// Line is the line number of the failure, or 0 if unknown.
	Line int
	// Message describes what went wrong.
	Message string
	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	loc := e.File
	if e.Line > 0 {
		loc = fmt.Sprintf("%s:%d", e.File, e.Line)
	}
	if e.Err != nil {
		return fmt.Sprintf("parse error at %s: %s: %v", loc, e.Message, e.Err)
	}
	return fmt.Sprintf("parse error at %s: %s", loc, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *ParseError) Unwrap() error { return e.Err }

// NewParse creates a ParseError.
func NewParse(file string, line int, message string, err error) *ParseError {
	return &ParseError{File: file, Line: line, Message: message, Err: err}
}

// StoreError indicates a storage/persistence failure.
type StoreError struct {
	// Op is the operation that failed (e.g., "open", "save", "load").
	Op string
	// Path is the database path involved.
	Path string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *StoreError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("store %s %s: %v", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("store %s %s", e.Op, e.Path)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *StoreError) Unwrap() error { return e.Err }

// NewStore creates a StoreError.
func NewStore(op, path string, err error) *StoreError {
	return &StoreError{Op: op, Path: path, Err: err}
}
