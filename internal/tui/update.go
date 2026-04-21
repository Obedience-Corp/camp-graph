package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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
		m.groups = groupByType(m.results)
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.mode == modeMicrograph {
			return m.updateMicrograph(msg)
		}
		switch m.focus {
		case focusTypeChip, focusTrackedChip, focusModeChip:
			return m.updateChipFocus(msg)
		case focusScopePicker:
			return m.updateScopePicker(msg)
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
		m.focus = focusSearch
		m.search.Focus()
		return m, m.search.Cursor.BlinkCmd()
	case "t":
		m.focus = focusTypeChip
		m.chips.Type.Focus()
		return m, nil
	case "s":
		m.focus = focusTrackedChip
		m.chips.Tracked.Focus()
		return m, nil
	case "m":
		m.focus = focusModeChip
		m.chips.Mode.Focus()
		return m, nil
	case "c":
		m.focus = focusScopePicker
		m.scopePicker.open = true
		m.scopePicker.cursor = 0
		return m, nil
	case "tab":
		m.relationMode = m.relationMode.Cycle()
	case "a":
		// Widen from scope anchors to all nodes on demand.
		m.showingAnchors = false
		m.filtered = m.nodes
		m.cursor = 0
	case "enter":
		if len(m.filtered) > 0 {
			m.enterMicrograph(m.filtered[m.cursor])
		}
	}
	return m, nil
}

// updateChipFocus routes a keystroke to the currently focused chip.
// On esc, the chip is blurred and focus returns to the list without
// reissuing a query. Other keys are forwarded to the chip's Update;
// the re-issue-on-change hookup lands in task 05.
func (m Model) updateChipFocus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		switch m.focus {
		case focusTypeChip:
			m.chips.Type.Blur()
		case focusTrackedChip:
			m.chips.Tracked.Blur()
		case focusModeChip:
			m.chips.Mode.Blur()
		}
		m.focus = focusList
		return m, nil
	}

	var (
		cmd      tea.Cmd
		oldValue string
		newValue string
	)
	switch m.focus {
	case focusTypeChip:
		oldValue = m.chips.Type.SelectedValue()
		m.chips.Type, cmd = m.chips.Type.Update(msg)
		newValue = m.chips.Type.SelectedValue()
	case focusTrackedChip:
		oldValue = m.chips.Tracked.SelectedValue()
		m.chips.Tracked, cmd = m.chips.Tracked.Update(msg)
		newValue = m.chips.Tracked.SelectedValue()
	case focusModeChip:
		oldValue = m.chips.Mode.SelectedValue()
		m.chips.Mode, cmd = m.chips.Mode.Update(msg)
		newValue = m.chips.Mode.SelectedValue()
	}
	if newValue != oldValue {
		if queryCmd := m.issueQuery(); queryCmd != nil {
			return m, tea.Batch(cmd, queryCmd)
		}
	}
	return m, cmd
}

// updateScopePicker routes keys while the scope picker overlay owns
// focus. esc closes without changing m.scope; enter applies the
// highlighted option and reissues the query; other keys (j/k) are
// forwarded to the picker's own Update for cursor movement.
func (m Model) updateScopePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.scopePicker.open = false
		m.focus = focusList
		return m, nil
	case "enter":
		if sel := m.scopePicker.Selected(); sel != "" {
			m.scope = sel
		}
		m.scopePicker.open = false
		m.focus = focusList
		return m, m.issueQuery()
	default:
		var cmd tea.Cmd
		m.scopePicker, cmd = m.scopePicker.Update(msg)
		return m, cmd
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Reset()
		m.search.Blur()
		m.cursor = 0
		if m.queryCancel != nil {
			m.queryCancel()
			m.queryCancel = nil
		}
		m.results = nil
		m.groups = nil
		m.filteredAnchors = filterAnchors(m.scopeAnchors, chipTypeValue(m), chipTrackedValue(m), m.scope)
		return m, nil
	case "enter":
		m.searching = false
		m.search.Blur()
		return m, nil
	}

	var inputCmd tea.Cmd
	m.search, inputCmd = m.search.Update(msg)

	queryCmd := m.issueQuery()
	if queryCmd == nil {
		return m, inputCmd
	}
	return m, tea.Batch(inputCmd, queryCmd)
}
