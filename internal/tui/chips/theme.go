// Local theme shim for the chips package (D002). Supplies only the
// palette fields referenced by styles.go. Values mirror
// camp/internal/ui/theme.TUI() at commit 5c82d35b9b4b7d8870c4354c58e6a11114f30257
// so visual presentation matches camp intent explore.
package chips

import "github.com/charmbracelet/lipgloss"

var pal = struct {
	Border        lipgloss.TerminalColor
	BorderFocus   lipgloss.TerminalColor
	Accent        lipgloss.TerminalColor
	TextPrimary   lipgloss.TerminalColor
	TextSecondary lipgloss.TerminalColor
	TextMuted     lipgloss.TerminalColor
	BgSelected    lipgloss.TerminalColor
}{
	Border:        lipgloss.AdaptiveColor{Light: "250", Dark: "240"},
	BorderFocus:   lipgloss.AdaptiveColor{Light: "205", Dark: "205"},
	Accent:        lipgloss.AdaptiveColor{Light: "205", Dark: "205"},
	TextPrimary:   lipgloss.AdaptiveColor{Light: "232", Dark: "255"},
	TextSecondary: lipgloss.AdaptiveColor{Light: "238", Dark: "250"},
	TextMuted:     lipgloss.AdaptiveColor{Light: "243", Dark: "246"},
	BgSelected:    lipgloss.AdaptiveColor{Light: "254", Dark: "237"},
}
