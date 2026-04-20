package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/Obedience-Corp/camp-graph/internal/search"
)

func TestBuildOpts(t *testing.T) {
	cases := []struct {
		name string
		term string
		want search.QueryOptions
	}{
		{"empty", "", search.QueryOptions{Term: ""}},
		{"whitespace", "   ", search.QueryOptions{Term: "   "}},
		{"plain", "campaign", search.QueryOptions{Term: "campaign"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{search: newTestInput(tc.term)}
			got := buildOpts(m)
			if got != tc.want {
				t.Fatalf("buildOpts=%+v want %+v", got, tc.want)
			}
		})
	}
}

func newTestInput(term string) textinput.Model {
	ti := textinput.New()
	ti.SetValue(term)
	return ti
}
