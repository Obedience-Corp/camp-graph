package tui

type layoutMode int

const (
	layoutNarrow layoutMode = iota
	layoutNormal
	layoutWide
)

// layoutFor maps terminal width to a layout mode per UX_SPEC:
//
//	narrow: width < 80
//	normal: 80 <= width <= 120
//	wide:   width > 120
func layoutFor(width int) layoutMode {
	switch {
	case width < 80:
		return layoutNarrow
	case width <= 120:
		return layoutNormal
	default:
		return layoutWide
	}
}
