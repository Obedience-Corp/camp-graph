package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func (m Model) updateMicrograph(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.microCursor > 0 {
			m.microCursor--
		}
	case "down", "j":
		if m.microCursor < len(m.neighbors)-1 {
			m.microCursor++
		}
	case "tab":
		m.relationMode = m.relationMode.Cycle()
		m.reloadNeighborsForFocus()
	case "enter":
		if len(m.neighbors) > 0 {
			m.history = append(m.history, m.focusNode)
			m.enterMicrograph(m.neighbors[m.microCursor].node)
		}
	case "esc":
		if len(m.history) > 0 {
			prev := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			m.enterMicrograph(prev)
		} else {
			m.mode = modeList
		}
	}
	return m, nil
}

func (m *Model) enterMicrograph(n *graph.Node) {
	m.mode = modeMicrograph
	m.focusNode = n
	m.microCursor = 0
	m.reloadNeighborsForFocus()
}

// reloadNeighborsForFocus rebuilds the neighbor list based on the
// current relation mode filter.
func (m *Model) reloadNeighborsForFocus() {
	m.neighbors = nil
	if m.focusNode == nil {
		return
	}
	for _, e := range m.graph.EdgesFrom(m.focusNode.ID) {
		if !edgeAllowedInMode(e, m.relationMode) {
			continue
		}
		if neighbor := m.graph.Node(e.ToID); neighbor != nil {
			m.neighbors = append(m.neighbors, &neighborEntry{
				node: neighbor, edge: e, direction: "→",
			})
		}
	}
	for _, e := range m.graph.EdgesTo(m.focusNode.ID) {
		if !edgeAllowedInMode(e, m.relationMode) {
			continue
		}
		if neighbor := m.graph.Node(e.FromID); neighbor != nil {
			m.neighbors = append(m.neighbors, &neighborEntry{
				node: neighbor, edge: e, direction: "←",
			})
		}
	}
}

// edgeAllowedInMode reports whether an edge passes the current
// relation-mode filter. Hybrid allows every edge; structural keeps
// SourceStructural; explicit keeps SourceExplicit; semantic keeps
// SourceInferred.
func edgeAllowedInMode(e *graph.Edge, mode RelationMode) bool {
	switch mode {
	case RelationStructural:
		return e.Source == graph.SourceStructural
	case RelationExplicit:
		return e.Source == graph.SourceExplicit
	case RelationSemantic:
		return e.Source == graph.SourceInferred
	default:
		return true
	}
}

func (m Model) viewMicrograph() string {
	var b strings.Builder

	// Breadcrumb
	var crumbs []string
	for _, h := range m.history {
		crumbs = append(crumbs, h.Name)
	}
	crumbs = append(crumbs, m.focusNode.Name)
	b.WriteString(breadcrumbStyle.Render(strings.Join(crumbs, " > ")) + "\n\n")

	// Focus node details
	b.WriteString(titleStyle.Render(m.focusNode.Name) + "\n")
	b.WriteString(detailLabelStyle.Render("Type:   ") + string(m.focusNode.Type) + "\n")
	b.WriteString(detailLabelStyle.Render("Path:   ") + m.focusNode.Path + "\n")
	if m.focusNode.Status != "" {
		b.WriteString(detailLabelStyle.Render("Status: ") + m.focusNode.Status + "\n")
	}
	b.WriteString("\n")

	// Neighbor list
	if len(m.neighbors) == 0 {
		b.WriteString(defaultStyle.Render("No neighbors") + "\n")
	} else {
		b.WriteString(detailLabelStyle.Render(fmt.Sprintf("Neighbors (%d):", len(m.neighbors))) + "\n\n")

		visibleLines := max(1, m.height-10)
		start := 0
		if m.microCursor >= visibleLines {
			start = m.microCursor - visibleLines + 1
		}

		for i := start; i < len(m.neighbors) && i < start+visibleLines; i++ {
			ne := m.neighbors[i]
			line := fmt.Sprintf("%s %s [%s] (%s)", ne.direction, ne.node.Name, ne.edge.Type, ne.node.Type)

			styled := styleForType(ne.node.Type).Render(line)
			if i == m.microCursor {
				styled = cursorStyle.Render(line)
			}

			b.WriteString("  " + styled + "\n")
		}
	}

	b.WriteString("\n" + defaultStyle.Render("enter: focus  esc: back  q: quit"))

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
