package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case queryResultMsg:
		if msg.gen != m.queryGen {
			return m, nil
		}
		m.queryCancel = nil
		if msg.err != nil {
			m.results = nil
		} else {
			m.results = msg.results
		}
		// TODO(task-05): m.groups = groupByType(m.results)
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.mode == modeMicrograph {
			return m.updateMicrograph(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "/":
		m.searching = true
		m.search.Focus()
		return m, m.search.Cursor.BlinkCmd()
	case "tab":
		m.relationMode = m.relationMode.Cycle()
	case "a":
		// Widen from scope anchors to all nodes on demand.
		m.showingAnchors = false
		m.filtered = m.nodes
		m.cursor = 0
	case "s":
		// Return to scope-anchor view.
		m.showingAnchors = true
		m.filtered = m.scopeAnchors
		m.cursor = 0
	case "enter":
		if len(m.filtered) > 0 {
			m.enterMicrograph(m.filtered[m.cursor])
		}
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Reset()
		m.search.Blur()
		m.filtered = m.nodes
		m.cursor = 0
		return m, nil
	case "enter":
		m.searching = false
		m.search.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.search.Value())
	if query == "" {
		m.filtered = m.nodes
		return
	}
	var result []*graph.Node
	for _, n := range m.nodes {
		if strings.Contains(strings.ToLower(n.Name), query) ||
			strings.Contains(strings.ToLower(string(n.Type)), query) {
			result = append(result, n)
		}
	}
	m.filtered = result
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}
