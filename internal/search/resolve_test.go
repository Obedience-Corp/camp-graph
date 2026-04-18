package search_test

import (
	"context"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func TestResolve_ExactNodeID(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()

	id, reason, err := search.Resolve(context.Background(), store.DB(), "note:Work/JobSearch/plan.md")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id != "note:Work/JobSearch/plan.md" || reason != "exact_id" {
		t.Errorf("got id=%q reason=%q; want id=note:Work/JobSearch/plan.md reason=exact_id", id, reason)
	}
}

func TestResolve_ExactRelPath(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()

	id, reason, err := search.Resolve(context.Background(), store.DB(), "Work/JobSearch/plan.md")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reason != "exact_rel_path" {
		t.Errorf("reason: got %q, want exact_rel_path", reason)
	}
	if id != "note:Work/JobSearch/plan.md" {
		t.Errorf("id: got %q, want note:Work/JobSearch/plan.md", id)
	}
}

func TestResolve_TopLexicalHit(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()

	id, reason, err := search.Resolve(context.Background(), store.DB(), "planning")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reason != "top_hit" {
		t.Errorf("reason: got %q, want top_hit", reason)
	}
	if id == "" {
		t.Error("expected top-hit id; got empty")
	}
}

func TestResolve_NoMatchReturnsEmpty(t *testing.T) {
	store, cleanup := seedTwoNotes(t)
	defer cleanup()

	id, reason, err := search.Resolve(context.Background(), store.DB(), "zzzzz-no-match-zzzzz")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id != "" || reason != "" {
		t.Errorf("unexpected match: id=%q reason=%q", id, reason)
	}
}
