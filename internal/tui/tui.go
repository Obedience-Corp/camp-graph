// Package tui provides a BubbleTea-based terminal graph browser.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

type viewMode int

const (
	modeList viewMode = iota
	modeMicrograph
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

type neighborEntry struct {
	node      *graph.Node
	edge      *graph.Edge
	direction string
}

// Model is the BubbleTea model for the graph browser.
type Model struct {
	graph       *graph.Graph
	nodes       []*graph.Node
	filtered    []*graph.Node
	cursor      int
	search      textinput.Model
	searching   bool
	width       int
	height      int
	mode        viewMode
	focusNode   *graph.Node
	neighbors   []*neighborEntry
	microCursor int
	history     []*graph.Node
}

// New creates a new TUI model from a populated graph.
func New(g *graph.Graph) Model {
	ti := textinput.New()
	ti.Placeholder = "search nodes..."
	ti.CharLimit = 64

	nodes := g.Nodes()
	return Model{
		graph:    g,
		nodes:    nodes,
		filtered: nodes,
		search:   ti,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.WindowSize()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
	case "enter":
		if len(m.filtered) > 0 {
			m.enterMicrograph(m.filtered[m.cursor])
		}
	}
	return m, nil
}

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
	m.neighbors = nil

	for _, e := range m.graph.EdgesFrom(n.ID) {
		if neighbor := m.graph.Node(e.ToID); neighbor != nil {
			m.neighbors = append(m.neighbors, &neighborEntry{
				node: neighbor, edge: e, direction: "→",
			})
		}
	}
	for _, e := range m.graph.EdgesTo(n.ID) {
		if neighbor := m.graph.Node(e.FromID); neighbor != nil {
			m.neighbors = append(m.neighbors, &neighborEntry{
				node: neighbor, edge: e, direction: "←",
			})
		}
	}
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

	return lipgloss.JoinHorizontal(lipgloss.Top, list, divider, detail)
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

func (m Model) renderList(width int) string {
	var b strings.Builder

	header := titleStyle.Render("Graph Browser")
	if m.searching {
		header += " " + m.search.View()
	} else {
		header += fmt.Sprintf(" (%d nodes)", len(m.filtered))
	}
	b.WriteString(header + "\n\n")

	visibleLines := max(1, m.height-4)

	start := 0
	if m.cursor >= visibleLines {
		start = m.cursor - visibleLines + 1
	}

	for i := start; i < len(m.filtered) && i < start+visibleLines; i++ {
		n := m.filtered[i]
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

	b.WriteString("\n" + defaultStyle.Render("enter: micrograph  ↑↓/jk: navigate  /: search  q: quit"))

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
