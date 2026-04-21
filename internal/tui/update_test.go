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
