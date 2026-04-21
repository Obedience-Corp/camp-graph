package tui

import "testing"

func TestPaneSizes(t *testing.T) {
	cases := []struct {
		name          string
		mode          layoutMode
		width, height int
		wantListW     int
		wantPreviewW  int
		wantSumExact  bool
	}{
		{"narrow 79", layoutNarrow, 79, 24, 79, 0, false},
		{"normal 100", layoutNormal, 100, 24, 60, 40, true},
		{"wide 160", layoutWide, 160, 40, 80, 80, true},
		{"normal 119", layoutNormal, 119, 30, 71, 48, true},
		{"wide 121", layoutWide, 121, 30, 60, 61, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listW, previewW, listH := paneSizes(tc.mode, tc.width, tc.height)
			if listW != tc.wantListW {
				t.Errorf("listW=%d want %d", listW, tc.wantListW)
			}
			if previewW != tc.wantPreviewW {
				t.Errorf("previewW=%d want %d", previewW, tc.wantPreviewW)
			}
			if tc.wantSumExact && listW+previewW != tc.width {
				t.Errorf("listW+previewW=%d want %d", listW+previewW, tc.width)
			}
			if tc.mode == layoutNarrow && previewW != 0 {
				t.Errorf("narrow previewW=%d want 0", previewW)
			}
			if wantH := tc.height - chromeRows; listH != wantH {
				t.Errorf("listH=%d want %d", listH, wantH)
			}
		})
	}
}

func TestLayoutFor(t *testing.T) {
	cases := []struct {
		width int
		want  layoutMode
	}{
		{79, layoutNarrow},
		{80, layoutNormal},
		{81, layoutNormal},
		{119, layoutNormal},
		{120, layoutNormal},
		{121, layoutWide},
	}
	for _, tc := range cases {
		if got := layoutFor(tc.width); got != tc.want {
			t.Errorf("layoutFor(%d) = %v, want %v", tc.width, got, tc.want)
		}
	}
}
