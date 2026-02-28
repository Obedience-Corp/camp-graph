package errors

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestGraphValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      *GraphValidationError
		wantMsg  string
		wantWrap error
	}{
		{
			name:     "with node and underlying error",
			err:      NewGraphValidation("proj-1", "missing edges", io.EOF),
			wantMsg:  "graph validation [proj-1]: missing edges: EOF",
			wantWrap: io.EOF,
		},
		{
			name:     "with node no underlying error",
			err:      NewGraphValidation("proj-1", "orphaned node", nil),
			wantMsg:  "graph validation [proj-1]: orphaned node",
			wantWrap: nil,
		},
		{
			name:     "no node with underlying error",
			err:      NewGraphValidation("", "empty graph", io.EOF),
			wantMsg:  "graph validation: empty graph: EOF",
			wantWrap: io.EOF,
		},
		{
			name:     "no node no underlying error",
			err:      NewGraphValidation("", "empty graph", nil),
			wantMsg:  "graph validation: empty graph",
			wantWrap: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); got != tt.wantWrap {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantWrap)
			}
		})
	}
}

func TestNodeResolutionError(t *testing.T) {
	tests := []struct {
		name     string
		err      *NodeResolutionError
		wantMsg  string
		wantWrap error
	}{
		{
			name:     "with underlying error",
			err:      NewNodeResolution("proj-abc", io.EOF),
			wantMsg:  `node "proj-abc" not found: EOF`,
			wantWrap: io.EOF,
		},
		{
			name:     "without underlying error",
			err:      NewNodeResolution("proj-abc", nil),
			wantMsg:  `node "proj-abc" not found`,
			wantWrap: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); got != tt.wantWrap {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantWrap)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ParseError
		wantMsg  string
		wantWrap error
	}{
		{
			name:     "with line and underlying error",
			err:      NewParse("intent.md", 5, "invalid frontmatter", io.EOF),
			wantMsg:  "parse error at intent.md:5: invalid frontmatter: EOF",
			wantWrap: io.EOF,
		},
		{
			name:     "without line number",
			err:      NewParse("intent.md", 0, "missing delimiter", nil),
			wantMsg:  "parse error at intent.md: missing delimiter",
			wantWrap: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); got != tt.wantWrap {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantWrap)
			}
		})
	}
}

func TestStoreError(t *testing.T) {
	tests := []struct {
		name     string
		err      *StoreError
		wantMsg  string
		wantWrap error
	}{
		{
			name:     "with underlying error",
			err:      NewStore("open", "/tmp/graph.db", io.EOF),
			wantMsg:  "store open /tmp/graph.db: EOF",
			wantWrap: io.EOF,
		},
		{
			name:     "without underlying error",
			err:      NewStore("save", "/tmp/graph.db", nil),
			wantMsg:  "store save /tmp/graph.db",
			wantWrap: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); got != tt.wantWrap {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantWrap)
			}
		})
	}
}

func TestErrorsIs(t *testing.T) {
	wrapped := Wrap(ErrNodeNotFound, "resolving query")
	if !Is(wrapped, ErrNodeNotFound) {
		t.Error("Is() should match wrapped sentinel error")
	}
	if Is(wrapped, ErrCycleDetected) {
		t.Error("Is() should not match unrelated sentinel error")
	}
}

func TestErrorsAs(t *testing.T) {
	original := NewGraphValidation("x", "bad", nil)
	wrapped := fmt.Errorf("outer: %w", original)

	var ve *GraphValidationError
	if !As(wrapped, &ve) {
		t.Fatal("As() should extract GraphValidationError from wrapped error")
	}
	if ve.Node != "x" {
		t.Errorf("Node = %q, want %q", ve.Node, "x")
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		msg     string
		wantNil bool
		wantMsg string
	}{
		{name: "nil error", err: nil, msg: "ctx", wantNil: true},
		{name: "wraps error", err: io.EOF, msg: "loading graph", wantMsg: "loading graph: EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.msg)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrap(nil) = %v, want nil", got)
				}
				return
			}
			if got.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), tt.wantMsg)
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		format  string
		args    []any
		wantNil bool
		wantMsg string
	}{
		{name: "nil error", err: nil, format: "op %s", args: []any{"x"}, wantNil: true},
		{name: "wraps with format", err: io.EOF, format: "loading %s", args: []any{"graph"}, wantMsg: "loading graph: EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrapf(tt.err, tt.format, tt.args...)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrapf(nil) = %v, want nil", got)
				}
				return
			}
			if got.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), tt.wantMsg)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	t.Run("wrap context.Canceled", func(t *testing.T) {
		err := Wrap(context.Canceled, "scanning")
		if !Is(err, context.Canceled) {
			t.Error("wrapped context.Canceled should match")
		}
	})
	t.Run("wrap context.DeadlineExceeded", func(t *testing.T) {
		err := Wrap(context.DeadlineExceeded, "loading")
		if !Is(err, context.DeadlineExceeded) {
			t.Error("wrapped DeadlineExceeded should match")
		}
	})
	t.Run("typed error wrapping context.Canceled", func(t *testing.T) {
		se := NewStore("open", "/tmp/db", context.Canceled)
		if !Is(se, context.Canceled) {
			t.Error("StoreError wrapping Canceled should match")
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		err  error
		text string
	}{
		{ErrCycleDetected, "cycle detected in graph"},
		{ErrNodeNotFound, "node not found"},
		{ErrEdgeConflict, "conflicting edge definition"},
		{ErrInvalidGraph, "invalid graph structure"},
		{ErrDuplicateNode, "duplicate node ID"},
		{ErrStoreNotFound, "graph database not found"},
		{ErrNoFrontmatter, "no frontmatter"},
	}
	for _, tt := range sentinels {
		t.Run(tt.text, func(t *testing.T) {
			if tt.err.Error() != tt.text {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.text)
			}
		})
	}
}

func TestUnwrapAndNew(t *testing.T) {
	original := New("base")
	wrapped := Wrap(original, "context")
	unwrapped := Unwrap(wrapped)
	if unwrapped == nil || unwrapped.Error() != "base" {
		t.Errorf("Unwrap().Error() = %v, want %q", unwrapped, "base")
	}
}
