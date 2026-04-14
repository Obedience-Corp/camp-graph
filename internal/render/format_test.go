package render

import (
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    Format
		wantErr bool
	}{
		{"dot", FormatDOT, false},
		{"DOT", FormatDOT, false},
		{" svg ", FormatSVG, false},
		{"SVG", FormatSVG, false},
		{"png", FormatPNG, false},
		{"PNG", FormatPNG, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{" json ", FormatJSON, false},
		{"html", FormatHTML, false},
		{"HTML", FormatHTML, false},
		{"pdf", "", true},
		{"", "", true},
		{"jpeg", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidFormats(t *testing.T) {
	formats := ValidFormats()
	if len(formats) != 5 {
		t.Errorf("ValidFormats() returned %d formats, want 5", len(formats))
	}
}
