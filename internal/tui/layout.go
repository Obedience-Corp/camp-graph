package tui

type layoutMode int

const (
	layoutNarrow layoutMode = iota
	layoutNormal
	layoutWide
)

// chromeRows is the fixed vertical cost of the header, chip bar,
// active-filters row, and footer chrome. Tune once to match the row
// count produced by the renderer.
const chromeRows = 5

// paneSizes allocates list and preview widths for the given layout
// mode plus a vertical budget for the list body (height minus the
// fixed chrome). Narrow collapses the preview; normal is 60/40;
// wide is 50/50. listW + previewW always equals width exactly (the
// second return uses width - listW rather than a second percentage,
// avoiding rounding loss).
func paneSizes(mode layoutMode, width, height int) (listW, previewW, listH int) {
	listH = height - chromeRows
	if listH < 0 {
		listH = 0
	}
	switch mode {
	case layoutNarrow:
		return width, 0, listH
	case layoutNormal:
		listW = (width * 60) / 100
		return listW, width - listW, listH
	case layoutWide:
		listW = width / 2
		return listW, width - listW, listH
	}
	return width, 0, listH
}

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
