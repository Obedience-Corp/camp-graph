package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Styles for node type coloring.
var (
	projectStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))  // cyan
	festivalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))  // magenta
	phaseStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))  // yellow
	sequenceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // green
	taskStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))  // white
	intentStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))  // blue
	designDocStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // red
	defaultStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray
	cursorStyle      = lipgloss.NewStyle().Bold(true).Reverse(true)
	titleStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
	detailLabelStyle = lipgloss.NewStyle().Bold(true)
	breadcrumbStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.mode == modeMicrograph {
		return m.viewMicrograph()
	}

	listWidth := m.width / 2
	detailWidth := m.width - listWidth - 1

	list := m.renderList(listWidth)
	detail := m.renderDetail(detailWidth)

	divider := strings.Repeat("│\n", max(1, m.height-2))
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, divider, detail)

	header := m.renderHeader()
	view := body
	if header != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, header, body)
	}

	if m.scopePicker.open {
		overlay := m.scopePicker.View()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay, lipgloss.WithWhitespaceChars(" "))
	}
	return view
}

// renderHeader builds the top-of-view stack: chip bar plus (if any chip
// is off its default) an active-filters pill row. The search input
// continues to render inside the list header; the chip bar sits
// between the search input and the list per UX_SPEC.
func (m Model) renderHeader() string {
	bar := m.renderChipBar()
	active := m.renderActiveFilters()
	switch {
	case bar == "" && active == "":
		return ""
	case active == "":
		return bar
	case bar == "":
		return active
	}
	return lipgloss.JoinVertical(lipgloss.Left, bar, active)
}

// renderChipBar returns a single-line horizontal join of the three
// chip views.
func (m Model) renderChipBar() string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.chips.Type.View(), " ",
		m.chips.Tracked.View(), " ",
		m.chips.Mode.View(),
	)
}

// renderActiveFilters renders one pill per chip whose value is not its
// default, plus a Scope pill when m.scope is set. Returns "" when all
// filters are at defaults and no scope is selected.
func (m Model) renderActiveFilters() string {
	var pills []string
	if m.chips.Type.IsActive() {
		pills = append(pills, fmt.Sprintf("[Type: %s]", m.chips.Type.SelectedValue()))
	}
	if m.chips.Tracked.IsActive() {
		pills = append(pills, fmt.Sprintf("[Tracked: %s]", m.chips.Tracked.SelectedValue()))
	}
	if m.chips.Mode.IsActive() {
		pills = append(pills, fmt.Sprintf("[Mode: %s]", m.chips.Mode.SelectedValue()))
	}
	if m.scope != "" {
		pills = append(pills, fmt.Sprintf("[Scope: %s]", m.scopeLabel()))
	}
	if len(pills) == 0 {
		return ""
	}
	return breadcrumbStyle.Render(strings.Join(pills, " "))
}

const narrowWidth = 80

// scopeLabel renders m.scope for the active-filters pill, falling
// back to the last path segment on narrow terminals and truncating
// long paths to keep the row within a reasonable width.
func (m Model) scopeLabel() string {
	if m.width > 0 && m.width < narrowWidth {
		return lastPathSegment(m.scope)
	}
	const maxPill = 60
	if len(m.scope) <= maxPill {
		return m.scope
	}
	return "..." + m.scope[len(m.scope)-(maxPill-3):]
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (m Model) renderList(width int) string {
	var b strings.Builder

	// Grouped FTS rendering lands in a later sequence; until then we
	// fall through to list rendering with whichever slice is active:
	// m.filteredAnchors for the empty-query fallback, otherwise
	// m.filtered for legacy list/anchor views.
	rows := m.filtered
	if m.groups == nil && m.filteredAnchors != nil {
		rows = m.filteredAnchors
	}

	header := titleStyle.Render("Graph Browser")
	if m.searching {
		header += " " + m.search.View()
	} else {
		mode := "all"
		if m.showingAnchors {
			mode = "scopes"
		}
		header += fmt.Sprintf(" (%d %s, relation=%s)", len(rows), mode, m.relationMode)
	}
	b.WriteString(header + "\n\n")

	visibleLines := max(1, m.height-4)

	start := 0
	if m.cursor >= visibleLines {
		start = m.cursor - visibleLines + 1
	}

	for i := start; i < len(rows) && i < start+visibleLines; i++ {
		n := rows[i]
		tag := strings.ToUpper(string(n.Type)[:3])
		line := fmt.Sprintf("[%s] %s", tag, n.Name)

		if len(line) > width-2 {
			line = line[:width-5] + "..."
		}

		styled := styleForType(n.Type).Render(line)
		if i == m.cursor {
			styled = cursorStyle.Render(line)
		}

		b.WriteString("  " + styled + "\n")
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m Model) renderDetail(width int) string {
	if m.previewFocusID != "" || m.previewNode != nil {
		return lipgloss.NewStyle().Width(width).Render(renderPreview(m, width, m.height))
	}
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return lipgloss.NewStyle().Width(width).Render("No node selected")
	}

	n := m.filtered[m.cursor]
	var b strings.Builder

	b.WriteString(titleStyle.Render("Detail") + "\n\n")
	b.WriteString(detailLabelStyle.Render("Name:   ") + n.Name + "\n")
	b.WriteString(detailLabelStyle.Render("Type:   ") + string(n.Type) + "\n")
	b.WriteString(detailLabelStyle.Render("Path:   ") + n.Path + "\n")
	if n.Status != "" {
		b.WriteString(detailLabelStyle.Render("Status: ") + n.Status + "\n")
	}

	if len(n.Metadata) > 0 {
		b.WriteString("\n" + detailLabelStyle.Render("Metadata:") + "\n")
		for k, v := range n.Metadata {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	neighbors := m.graph.Neighbors(n.ID)
	if len(neighbors) > 0 {
		b.WriteString(fmt.Sprintf("\n"+detailLabelStyle.Render("Neighbors (%d):")+"\n", len(neighbors)))
		edges := m.graph.EdgesFrom(n.ID)
		for _, e := range edges {
			target := m.graph.Node(e.ToID)
			if target != nil {
				b.WriteString(fmt.Sprintf("  → %s (%s)\n", target.Name, e.Type))
			}
		}
		edgesTo := m.graph.EdgesTo(n.ID)
		for _, e := range edgesTo {
			source := m.graph.Node(e.FromID)
			if source != nil {
				b.WriteString(fmt.Sprintf("  ← %s (%s)\n", source.Name, e.Type))
			}
		}
	}

	b.WriteString("\n" + defaultStyle.Render(
		"enter: micrograph  ↑↓/jk: navigate  /: search  tab: cycle relation  a: all  s: scopes  q: quit"))

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func styleForType(t graph.NodeType) lipgloss.Style {
	switch t {
	case graph.NodeProject:
		return projectStyle
	case graph.NodeFestival:
		return festivalStyle
	case graph.NodePhase:
		return phaseStyle
	case graph.NodeSequence:
		return sequenceStyle
	case graph.NodeTask:
		return taskStyle
	case graph.NodeIntent:
		return intentStyle
	case graph.NodeDesignDoc:
		return designDocStyle
	default:
		return defaultStyle
	}
}
