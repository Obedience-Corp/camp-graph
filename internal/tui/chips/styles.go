// Copied from camp/internal/intent/tui/filterchip at commit 5c82d35b9b4b7d8870c4354c58e6a11114f30257.
// See festivals/active/camp-graph-tui-search-upgrade-CG0005/002_PLAN/decisions/D002_copy_filterchip_not_import.md.
//
// Theme dependency (D002): camp's theme package is not imported. Instead,
// a local pal struct in theme.go supplies the subset of colors referenced
// below (Border, BorderFocus, Accent, TextPrimary, TextSecondary,
// TextMuted, BgSelected). Values mirror camp/internal/ui/theme.TUI() so
// the visual presentation matches camp intent explore.

package chips

import (
	"github.com/charmbracelet/lipgloss"
)

// Chip styles
var (
	// Base chip style
	chipStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(pal.Border).
			Foreground(pal.TextPrimary).
			Padding(0, 1)

	// Focused chip (has keyboard focus)
	chipFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(pal.BorderFocus).
				Padding(0, 1).
				Bold(true)

	// Active chip (has a non-default selection)
	chipActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(pal.Accent).
			Padding(0, 1).
			Foreground(pal.Accent)

	// Dropdown container — subtle left border for visual hierarchy
	dropdownStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(pal.BorderFocus).
			PaddingLeft(1)

	// Inline chip styles (no border, used alongside open dropdowns)
	chipInlineStyle = lipgloss.NewStyle().
			Foreground(pal.TextPrimary).
			Padding(0, 1)

	chipFocusedInlineStyle = lipgloss.NewStyle().
				Foreground(pal.TextPrimary).
				Bold(true).
				Padding(0, 1)

	chipActiveInlineStyle = lipgloss.NewStyle().
				Foreground(pal.Accent).
				Padding(0, 1)

	// Regular dropdown option
	optionStyle = lipgloss.NewStyle().
			Foreground(pal.TextSecondary)

	// Selected/highlighted option in dropdown
	optionSelectedStyle = lipgloss.NewStyle().
				Background(pal.BgSelected).
				Foreground(pal.TextPrimary).
				Bold(true)

	// Current selection indicator
	optionCurrentStyle = lipgloss.NewStyle().
				Foreground(pal.Accent)

	// Number prefix for quick-select
	numberStyle = lipgloss.NewStyle().
			Foreground(pal.TextMuted)

	// Label style
	labelStyle = lipgloss.NewStyle().
			Foreground(pal.TextSecondary)

	// Value style
	valueStyle = lipgloss.NewStyle().
			Foreground(pal.TextPrimary)

	// Dropdown indicator arrow
	arrowStyle = lipgloss.NewStyle().
			Foreground(pal.TextMuted)
)
