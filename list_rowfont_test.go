package toolkit

import "testing"

// TestListBoxRowPitchFollowsTheFont: a list built at one metric scale and drawn
// at a larger one (a host that scales after construction) must still advance
// each row by at least the glyph height, or descenders sit under the next line.
func TestListBoxRowPitchFollowsTheFont(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(1)
	lb := NewListBox([]string{"a", "b", "c"}) // RowHeight baked at scale 1
	for _, scale := range []float64{1, 1.5, 2, 3} {
		SetMetricScale(scale)
		if got, want := lb.rowHeight(), GlyphHeight(); got < want {
			t.Fatalf("scale %v: row pitch %d < glyph height %d", scale, got, want)
		}
	}
}
