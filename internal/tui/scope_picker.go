package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// scopePickerModel is the modal scope-picker overlay. Options are the
// scope-anchor paths discovered during tui.New; the cursor tracks the
// currently highlighted row. open is true while the overlay is active.
type scopePickerModel struct {
	options []string
	cursor  int
	open    bool
}

// newScopePicker builds a picker seeded with scope anchor paths from
// the existing anchor list. Duplicates are dropped while preserving
// first-appearance order.
func newScopePicker(anchors []*graph.Node) scopePickerModel {
	seen := make(map[string]struct{}, len(anchors))
	options := make([]string, 0, len(anchors))
	for _, n := range anchors {
		if n == nil || n.Path == "" {
			continue
		}
		if _, dup := seen[n.Path]; dup {
			continue
		}
		seen[n.Path] = struct{}{}
		options = append(options, n.Path)
	}
	return scopePickerModel{options: options}
}

// Update handles cursor movement only. Accept and cancel keys are
// routed by the parent Update dispatcher; this model is intentionally
// self-contained so it can be rendered from a test harness.
func (p scopePickerModel) Update(msg tea.Msg) (scopePickerModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch key.String() {
	case "j", "down":
		if p.cursor < len(p.options)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	}
	return p, nil
}

// Selected returns the option under the cursor, or "" when empty.
func (p scopePickerModel) Selected() string {
	if p.cursor < 0 || p.cursor >= len(p.options) {
		return ""
	}
	return p.options[p.cursor]
}

// View renders the picker as a centered bordered list with a ">"
// cursor indicator on the focused row. Up to scopePickerViewRows rows
// are shown, scrolling around the cursor for longer lists.
const scopePickerViewRows = 20

func (p scopePickerModel) View() string {
	if !p.open {
		return ""
	}

	start := 0
	if p.cursor >= scopePickerViewRows {
		start = p.cursor - scopePickerViewRows + 1
	}
	end := start + scopePickerViewRows
	if end > len(p.options) {
		end = len(p.options)
	}

	var b strings.Builder
	b.WriteString("Pick a scope (j/k to move, enter to accept, esc to cancel)\n\n")
	for i := start; i < end; i++ {
		cursor := "  "
		if i == p.cursor {
			cursor = "> "
		}
		b.WriteString(cursor + p.options[i] + "\n")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(b.String())
}
