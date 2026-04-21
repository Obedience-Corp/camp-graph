package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func TestCountPrefixJ(t *testing.T) {
	cases := []struct {
		name     string
		keys     []string
		wantStep int
	}{
		{"bare j", []string{"j"}, 1},
		{"5j", []string{"5", "j"}, 5},
		{"10k", []string{"1", "0", "k"}, -10},
		{"reset on unrelated key then j", []string{"3", "x", "j"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := *New(context.Background(), newTestStore(t), newTestGraph())
			// Synthesize a large filtered list so motions can take
			// meaningful steps without clamping.
			m.filtered = make([]*graph.Node, 100)
			for i := range m.filtered {
				m.filtered[i] = &graph.Node{ID: "node" + string(rune('A'+i%26)), Name: "n"}
			}
			m.filteredAnchors = nil
			m.cursor = 50
			startCursor := m.cursor

			var model tea.Model = m
			for _, k := range tc.keys {
				var key tea.KeyMsg
				if len(k) == 1 && k >= "a" && k <= "z" || len(k) == 1 && k >= "A" && k <= "Z" || len(k) == 1 && k >= "0" && k <= "9" {
					key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
				} else if k == "enter" {
					key = tea.KeyMsg{Type: tea.KeyEnter}
				} else {
					key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
				}
				next, _ := model.Update(key)
				model = next
			}

			got := model.(Model)
			delta := got.cursor - startCursor
			if delta != tc.wantStep {
				t.Fatalf("cursor delta=%d want %d (cursor %d -> %d)", delta, tc.wantStep, startCursor, got.cursor)
			}
		})
	}
}

func TestGJumpsToTopImmediately(t *testing.T) {
	m := *New(context.Background(), newTestStore(t), newTestGraph())
	m.filtered = make([]*graph.Node, 20)
	for i := range m.filtered {
		m.filtered[i] = &graph.Node{ID: "node", Name: "n"}
	}
	m.filteredAnchors = nil
	m.cursor = 10

	got := updateModel(t, m, keyRunes("g"))
	if got.cursor != 0 {
		t.Fatalf("cursor=%d want 0 after bare g", got.cursor)
	}
	if got.countBuf != "" {
		t.Fatalf("countBuf=%q want empty after bare g", got.countBuf)
	}
}

func TestCountPrefixGGJumpsToNthRow(t *testing.T) {
	m := *New(context.Background(), newTestStore(t), newTestGraph())
	m.filtered = make([]*graph.Node, 20)
	for i := range m.filtered {
		m.filtered[i] = &graph.Node{ID: "node", Name: "n"}
	}
	m.filteredAnchors = nil
	m.cursor = 10

	got := updateModel(t, m, keyRunes("3"))
	got = updateModel(t, got, keyRunes("g"))
	if !got.pendingG {
		t.Fatal("pendingG=false want true after counted g")
	}
	got = updateModel(t, got, keyRunes("g"))

	if got.cursor != 2 {
		t.Fatalf("cursor=%d want 2 after 3gg", got.cursor)
	}
	if got.pendingG {
		t.Fatal("pendingG=true want false after completing gg")
	}
	if got.countBuf != "" {
		t.Fatalf("countBuf=%q want empty after 3gg", got.countBuf)
	}
}

func TestTabMovesFocusToPreviewWithoutChangingRelationMode(t *testing.T) {
	m := *New(context.Background(), newTestStore(t), newTestGraph())
	m.width = 120
	m.height = 40
	m.layout = layoutFor(m.width)
	m.listW, m.previewW, m.listH = paneSizes(m.layout, m.width, m.height)
	m.focus = focusList
	m.previewNode = &graph.Node{ID: "preview", Name: "preview"}
	m.relationMode = RelationHybrid

	got := updateModel(t, m, keyNamed(tea.KeyTab))
	if got.focus != focusPreview {
		t.Fatalf("focus=%v want %v", got.focus, focusPreview)
	}
	if got.relationMode != RelationHybrid {
		t.Fatalf("relationMode=%q want %q", got.relationMode, RelationHybrid)
	}
}

func TestModeChipChangeSyncsRelationMode(t *testing.T) {
	m := *New(context.Background(), newTestStore(t), newTestGraph())
	m.focus = focusModeChip
	m.chips.Mode.Focus()

	// enter opens the dropdown; down selects "structural"; enter applies.
	m = updateModel(t, m, keyNamed(tea.KeyEnter))
	m = updateModel(t, m, keyNamed(tea.KeyDown))
	m = updateModel(t, m, keyNamed(tea.KeyEnter))

	if got := m.chips.Mode.SelectedValue(); got != "structural" {
		t.Fatalf("chip mode=%q want structural", got)
	}
	if m.relationMode != RelationStructural {
		t.Fatalf("relationMode=%q want %q", m.relationMode, RelationStructural)
	}
}
