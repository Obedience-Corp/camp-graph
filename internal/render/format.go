package render

import (
	"fmt"
	"strings"
)

// Format represents a supported output format.
type Format string

const (
	FormatDOT Format = "dot"
	FormatSVG Format = "svg"
	FormatPNG Format = "png"
)

// ValidFormats returns all supported format strings.
func ValidFormats() []string {
	return []string{string(FormatDOT), string(FormatSVG), string(FormatPNG)}
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
	default:
		return "", fmt.Errorf("unsupported format %q (valid: %s)", s, strings.Join(ValidFormats(), ", "))
	}
}
