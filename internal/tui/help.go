package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpSections encodes the UX_SPEC keybinding table. Sections are
// ordered to match the spec; each row is {Keys, Action} for
// two-column alignment.
var helpSections = []struct {
	Section string
	Rows    [][2]string
}{
	{
		Section: "Navigation",
		Rows: [][2]string{
			{"j / down", "Move cursor down one row"},
			{"k / up", "Move cursor up one row"},
			{"Ngg / Nj / Nk", "Move cursor N rows or jump to top (count prefix)"},
			{"g / gg", "Jump to first row"},
			{"G", "Jump to last row"},
			{"ctrl+d", "Half-page down"},
			{"ctrl+u", "Half-page up"},
			{"tab", "Toggle focus list <-> preview"},
		},
	},
	{
		Section: "Search and filter",
		Rows: [][2]string{
			{"/", "Focus search input (live FTS)"},
			{"enter (search)", "Exit input, keep query"},
			{"esc (search)", "Clear query, unfocus input"},
			{"t", "Focus Type chip"},
			{"s", "Focus Tracked chip"},
			{"m", "Focus Mode chip"},
			{"c", "Open scope picker"},
			{"C", "Clear scope filter"},
		},
	},
	{
		Section: "Actions on focused node",
		Rows: [][2]string{
			{"enter", "Open micrograph / expand group header"},
			{"esc", "Exit micrograph back to prior state"},
			{"a", "Widen from anchors to all nodes"},
		},
	},
	{
		Section: "View",
		Rows: [][2]string{
			{"?", "Toggle this help overlay"},
			{"q / ctrl+c", "Quit"},
		},
	},
}

// renderHelp produces the full-screen help overlay body. Width and
// height are passed in so future breakpoint-specific layout changes
// can adapt; today the render is a top-left aligned bordered box
// with section headers and two-column rows.
func renderHelp(width, height int) string {
	_ = height
	var b strings.Builder
	b.WriteString(titleStyle.Render("camp-graph browse - keybindings") + "\n\n")

	keyW := helpKeyColumnWidth()
	for _, sec := range helpSections {
		b.WriteString(detailLabelStyle.Render(sec.Section) + "\n")
		for _, row := range sec.Rows {
			fmt.Fprintf(&b, "  %-*s  %s\n", keyW, row[0], row[1])
		}
		b.WriteString("\n")
	}
	b.WriteString(breadcrumbStyle.Render("? or esc to close"))

	inner := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	if width > 0 {
		inner = inner.MaxWidth(width)
	}
	return inner.Render(b.String())
}

// helpKeyColumnWidth returns the maximum width of any Keys column
// value across every section, so both columns align under one another.
func helpKeyColumnWidth() int {
	w := 0
	for _, sec := range helpSections {
		for _, row := range sec.Rows {
			if n := lipgloss.Width(row[0]); n > w {
				w = n
			}
		}
	}
	return w
}
