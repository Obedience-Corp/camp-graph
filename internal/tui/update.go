package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// quitModel cancels any in-flight query or preview fetch before the
// program returns tea.Quit, so no Cmd goroutine outlives the UI.
func quitModel(m *Model) {
	if m.queryCancel != nil {
		m.queryCancel()
		m.queryCancel = nil
	}
	if m.previewCancel != nil {
		m.previewCancel()
		m.previewCancel = nil
	}
}

// isExplorerFallback returns true when the view matches the UX_SPEC
// empty-query fallback predicate: no search text, all chips at their
// "All"/default values, and no scope set. Bindings like 'a' (widen
// anchors) only fire in this state so they do not mutate the list
// under a live FTS query.
func isExplorerFallback(m Model) bool {
	if m.search.Value() != "" {
		return false
	}
	if m.scope != "" {
		return false
	}
	if m.chips.Type.IsActive() || m.chips.Tracked.IsActive() || m.chips.Mode.IsActive() {
		return false
	}
	return true
}

// consumeCount parses and clears m.countBuf. Returns the parsed count
// clamped to [1, 9999], defaulting to 1 when the buffer is empty or
// unparseable. Always clears the buffer.
func consumeCount(m *Model) int {
	buf := m.countBuf
	m.countBuf = ""
	if buf == "" {
		return 1
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 {
		return 1
	}
	if n > 9999 {
		return 9999
	}
	return n
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = layoutFor(msg.Width)
		m.listW, m.previewW, m.listH = paneSizes(m.layout, msg.Width, msg.Height)
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
		// Re-clamp the cursor to the new visible range so the list and
		// the preview pane stay consistent when a query shrinks the
		// result set below the previous cursor position.
		m.clampCursor()
		return m, nil

	case previewMsg:
		if msg.id != m.focusedRowID() {
			return m, nil
		}
		m.previewCancel = nil
		m.previewNode = msg.node
		m.previewEdges = msg.edges
		m.previewRelated = msg.related
		m.previewScroll = 0
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			quitModel(&m)
			return m, tea.Quit
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		if m.mode == modeMicrograph {
			return m.updateMicrograph(msg)
		}
		if m.focus == focusHelp {
			key := msg.String()
			if key == "?" || key == "esc" {
				m.focus = m.prevFocus
			}
			return m, nil
		}
		if msg.String() == "?" {
			m.prevFocus = m.focus
			m.focus = focusHelp
			return m, nil
		}
		switch m.focus {
		case focusTypeChip, focusTrackedChip, focusModeChip:
			return m.updateChipFocus(msg)
		case focusScopePicker:
			return m.updateScopePicker(msg)
		case focusPreview:
			return m.updatePreviewFocus(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pendingG {
		m.pendingG = false
		if key == "g" {
			n := consumeCount(&m)
			if n > 1 {
				m.cursor = n - 1
			} else {
				m.cursor = 0
			}
			m.clampCursor()
			return m, m.issuePreview()
		}
		m.countBuf = ""
	}

	// Vim-style count prefix: digits accumulate on countBuf until a
	// motion consumes them. A bare leading 0 falls through so any
	// 0-bound action still fires; today there is none, so 0 simply
	// clears the buffer.
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		if m.countBuf == "" && key == "0" {
			m.countBuf = ""
		} else {
			m.countBuf += key
			return m, nil
		}
	}

	switch key {
	case "q", "ctrl+c":
		quitModel(&m)
		return m, tea.Quit
	case "up", "k":
		n := consumeCount(&m)
		m.cursor -= n
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, m.issuePreview()
	case "down", "j":
		n := consumeCount(&m)
		m.cursor += n
		m.clampCursor()
		return m, m.issuePreview()
	case "g":
		if m.countBuf != "" {
			m.pendingG = true
			return m, nil
		}
		consumeCount(&m)
		m.cursor = 0
		return m, m.issuePreview()
	case "G":
		consumeCount(&m)
		if ceiling := m.visibleCeiling(); ceiling > 0 {
			m.cursor = ceiling - 1
		}
		return m, m.issuePreview()
	case "ctrl+u":
		n := consumeCount(&m)
		step := m.listH / 2
		if step < 1 {
			step = 1
		}
		m.cursor -= step * n
		m.clampCursor()
		return m, m.issuePreview()
	case "ctrl+d":
		n := consumeCount(&m)
		step := m.listH / 2
		if step < 1 {
			step = 1
		}
		m.cursor += step * n
		m.clampCursor()
		return m, m.issuePreview()
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
	case "C":
		if m.scope != "" {
			m.scope = ""
			return m, m.issueQuery()
		}
		return m, nil
	case "tab":
		if m.layout == layoutNarrow {
			return m, nil
		}
		m.focus = focusPreview
		return m, nil
	case "a":
		// Widen from scope anchors to all nodes, but only while the
		// view is in the explorer fallback (no search text, chips at
		// defaults, no scope). Outside that state 'a' is a no-op so
		// it does not mutate the list under a live FTS query.
		if isExplorerFallback(m) {
			m.showingAnchors = false
			m.filtered = m.nodes
			m.cursor = 0
		}
	case "enter":
		if len(m.groups) > 0 {
			gi, ri := groupCursorTarget(m.groups, m.cursor)
			if gi < 0 {
				return m, nil
			}
			if ri == -1 {
				m.groups[gi].Expanded = !m.groups[gi].Expanded
				return m, nil
			}
			// Resolve the focused query result against the in-memory
			// graph so enterMicrograph receives a *graph.Node from the
			// same set navigation is indexing, not an unrelated entry
			// in m.filtered.
			if n := m.graph.Node(m.groups[gi].Rows[ri].NodeID); n != nil {
				m.enterMicrograph(n)
			}
			return m, nil
		}
		rows := m.filtered
		if m.filteredAnchors != nil {
			rows = m.filteredAnchors
		}
		if len(rows) > 0 && m.cursor >= 0 && m.cursor < len(rows) {
			m.enterMicrograph(rows[m.cursor])
		}
	}
	// Any key that falls through without consuming countBuf clears it
	// so stray input does not linger and corrupt the next motion.
	m.countBuf = ""
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
		if m.focus == focusModeChip {
			m.syncRelationMode()
		}
		if queryCmd := m.issueQuery(); queryCmd != nil {
			return m, tea.Batch(cmd, queryCmd)
		}
	}
	return m, cmd
}

// updatePreviewFocus routes keys while the preview pane owns focus.
// j/k scroll the pane without moving the list cursor or issuing a
// preview Cmd; tab returns focus to the list.
func (m Model) updatePreviewFocus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.previewScroll++
	case "k", "up":
		if m.previewScroll > 0 {
			m.previewScroll--
		}
	case "tab":
		m.focus = focusList
	}
	return m, nil
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
		m.applyAnchorFilters()
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
