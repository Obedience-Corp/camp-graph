package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

// stubPreviewFetcher records every Fetch invocation with its ctx and
// id, and blocks on release per call so tests can assert cancellation
// of superseded fetches.
type stubPreviewFetcher struct {
	mu      sync.Mutex
	calls   []*stubPreviewCall
	release chan struct{}
}

type stubPreviewCall struct {
	id  string
	ctx context.Context
}

func (s *stubPreviewFetcher) Fetch(ctx context.Context, id string) (*graph.Node, previewEdges, []search.RelatedItem, error) {
	s.mu.Lock()
	call := &stubPreviewCall{id: id, ctx: ctx}
	s.calls = append(s.calls, call)
	s.mu.Unlock()
	select {
	case <-s.release:
		return &graph.Node{ID: id, Name: id}, previewEdges{}, nil, nil
	case <-ctx.Done():
		return nil, previewEdges{}, nil, ctx.Err()
	}
}

func (s *stubPreviewFetcher) callByID(id string) *stubPreviewCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.calls {
		if c.id == id {
			return c
		}
	}
	return nil
}

func (s *stubPreviewFetcher) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func TestPreviewCancellation(t *testing.T) {
	stub := &stubPreviewFetcher{release: make(chan struct{})}

	ids := []string{"r1", "r2", "r3", "r4"}
	cancelsByID := make(map[string]context.CancelFunc, len(ids))
	msgCh := make(chan tea.Msg, len(ids))
	for _, id := range ids {
		ctx, cancel := context.WithCancel(context.Background())
		cancelsByID[id] = cancel
		cmd := runPreviewCmd(ctx, stub, id)
		go func() { msgCh <- cmd() }()
	}

	waitFor(t, func() bool { return stub.callCount() == len(ids) })

	// Cancel all but the last id, as if the cursor moved through r1..r3
	// and then settled on r4 before any fetch returned.
	for _, id := range ids[:len(ids)-1] {
		cancelsByID[id]()
	}
	defer cancelsByID[ids[len(ids)-1]]()

	for _, id := range ids[:len(ids)-1] {
		call := stub.callByID(id)
		if call == nil {
			t.Fatalf("no call recorded for id %q", id)
		}
		select {
		case <-call.ctx.Done():
		case <-time.After(time.Second):
			t.Fatalf("context for id %q not cancelled", id)
		}
	}

	close(stub.release)

	received := make([]previewMsg, 0, len(ids))
	timeout := time.After(2 * time.Second)
	for len(received) < len(ids) {
		select {
		case msg := <-msgCh:
			pm, ok := msg.(previewMsg)
			if !ok {
				t.Fatalf("unexpected msg type %T", msg)
			}
			received = append(received, pm)
		case <-timeout:
			t.Fatalf("only received %d/%d msgs", len(received), len(ids))
		}
	}

	// Simulate Model state where the cursor ended on r4 so only that
	// previewMsg is accepted by Update.
	m := &Model{
		filtered: []*graph.Node{{ID: "r4"}},
		cursor:   0,
	}
	for _, msg := range received {
		next, _ := m.Update(msg)
		mm := next.(Model)
		m = &mm
	}

	if m.previewNode == nil || m.previewNode.ID != "r4" {
		t.Fatalf("expected accepted previewMsg for r4, got %+v", m.previewNode)
	}
}
