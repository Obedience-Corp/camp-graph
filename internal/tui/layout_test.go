package tui

import "testing"

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
