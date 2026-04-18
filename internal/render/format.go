package render

import (
	"strconv"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
)

// Format represents a supported output format.
type Format string

const (
	FormatDOT  Format = "dot"
	FormatSVG  Format = "svg"
	FormatPNG  Format = "png"
	FormatJSON Format = "json"
	FormatHTML Format = "html"
)

// ValidFormats returns all supported format strings.
func ValidFormats() []string {
	return []string{
		string(FormatDOT),
		string(FormatSVG),
		string(FormatPNG),
		string(FormatJSON),
		string(FormatHTML),
	}
}

// ParseFormat validates and returns a Format from a string.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dot":
		return FormatDOT, nil
	case "svg":
		return FormatSVG, nil
	case "png":
		return FormatPNG, nil
	case "json":
		return FormatJSON, nil
	case "html":
		return FormatHTML, nil
	default:
		return "", graphErrors.New("unsupported format " + strconv.Quote(s) + " (valid: " + strings.Join(ValidFormats(), ", ") + ")")
	}
}
