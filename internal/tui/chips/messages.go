// Copied from camp/internal/intent/tui/filterchip at commit 5c82d35b9b4b7d8870c4354c58e6a11114f30257.
// See festivals/active/camp-graph-tui-search-upgrade-CG0005/002_PLAN/decisions/D002_copy_filterchip_not_import.md.

// Package chips provides interactive filter chip components for TUI.
package chips

// FilterChangedMsg is sent when a filter selection changes.
type FilterChangedMsg struct {
	Label string // Which filter changed (e.g., "Type", "Status")
	Value string // The new selected value
	Index int    // Index of the selected option
}

// FilterBarFocusMsg is sent when focus enters/exits the filter bar.
type FilterBarFocusMsg struct {
	Focused bool // Whether the filter bar gained or lost focus
}
