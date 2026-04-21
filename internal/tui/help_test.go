package tui

import (
	"strings"
	"testing"
)

// TestRenderHelpContainsAllKeyTokens asserts every Keys column value
// from every helpSections row survives into the rendered output at
// width 100. This catches regressions where a row is dropped or a
// key literal is mistyped.
func TestRenderHelpContainsAllKeyTokens(t *testing.T) {
	out := renderHelp(100, 40)
	for _, sec := range helpSections {
		if !strings.Contains(out, sec.Section) {
			t.Errorf("section %q missing from help output", sec.Section)
		}
		for _, row := range sec.Rows {
			if !strings.Contains(out, row[0]) {
				t.Errorf("key %q (%s) missing from help output", row[0], row[1])
			}
		}
	}
	if !strings.Contains(out, "? or esc to close") {
		t.Error("footer hint missing from help output")
	}
}
