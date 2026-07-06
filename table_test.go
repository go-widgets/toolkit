// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// makeTableSurface allocates a w*h RGBA byte slice pre-filled with a
// sentinel colour so the Table tests can distinguish painted pixels
// from untouched pixels.
func makeTableSurface(w, h int) []byte { return makeSurface(w, h) }

// findRow returns the pixel at the horizontal centre of the widget on
// the vertical centre of body row idx. Used to confirm which theme
// colour landed on that row.
func tableRowCentrePixel(buf []byte, w, x, y0 int, rowIdx int) RGBA {
	cy := y0 + TableHeaderHeight + rowIdx*TableRowHeight + TableRowHeight/2
	// Centre of the widget horizontally; caller passes the widget's
	// mid-column x so this helper is independent of Bounds.
	return pixelAt(buf, w, x, cy)
}

// --- Constructor + defaults ---------------------------------------------

func TestNewTableDefaults(t *testing.T) {
	cols := []TableColumn{{Title: "A"}, {Title: "B"}}
	rows := [][]string{{"1", "2"}}
	tb := NewTable(cols, rows)
	if tb.Selected != -1 {
		t.Fatalf("Selected default = %d, want -1", tb.Selected)
	}
	if len(tb.Columns) != 2 || len(tb.Rows) != 1 {
		t.Fatalf("columns/rows lost through constructor: %+v / %+v", tb.Columns, tb.Rows)
	}
}

// --- columnWidths -------------------------------------------------------

func TestTableColumnWidthsAllFixed(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "a", Width: 30},
		{Title: "b", Width: 40},
	}, nil)
	got := tb.columnWidths(200)
	if len(got) != 2 || got[0] != 30 || got[1] != 40 {
		t.Fatalf("all-fixed widths = %v, want [30 40]", got)
	}
}

func TestTableColumnWidthsAllAutoDivideEqually(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "a"}, {Title: "b"}, {Title: "c"}, {Title: "d"},
	}, nil)
	got := tb.columnWidths(200)
	sum := 0
	for _, w := range got {
		sum += w
	}
	if sum != 200 {
		t.Fatalf("all-auto widths %v sum = %d, want 200", got, sum)
	}
	// Equal split: 200/4 == 50, no leftover, so every column matches.
	for i, w := range got {
		if w != 50 {
			t.Fatalf("all-auto col %d width = %d, want 50", i, w)
		}
	}
}

func TestTableColumnWidthsAllAutoLeftoverGoesToLast(t *testing.T) {
	// 200 / 3 = 66 rem 2 -- last auto column absorbs the +2.
	tb := NewTable([]TableColumn{
		{Title: "a"}, {Title: "b"}, {Title: "c"},
	}, nil)
	got := tb.columnWidths(200)
	if got[0] != 66 || got[1] != 66 || got[2] != 68 {
		t.Fatalf("all-auto+leftover widths = %v, want [66 66 68]", got)
	}
	if got[0]+got[1]+got[2] != 200 {
		t.Fatalf("all-auto+leftover sum = %d, want 200", got[0]+got[1]+got[2])
	}
}

func TestTableColumnWidthsMixSumsToTotal(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "a", Width: 30},
		{Title: "b"},            // auto
		{Title: "c", Width: 20},
		{Title: "d"},            // auto
	}, nil)
	got := tb.columnWidths(200)
	sum := 0
	for _, w := range got {
		sum += w
	}
	if sum != 200 {
		t.Fatalf("mix widths %v sum = %d, want 200", got, sum)
	}
	if got[0] != 30 || got[2] != 20 {
		t.Fatalf("mix widths lost fixed values: %v", got)
	}
	if got[1] != got[3] && got[1]+1 != got[3] {
		// The two auto columns should be equal (or off-by-one via
		// the leftover push onto the last one).
		t.Fatalf("mix auto widths differ by more than 1: %v", got)
	}
}

func TestTableColumnWidthsFixedOverflowsTotal(t *testing.T) {
	// fixedTotal > total -- exercises the `remaining < 0 -> 0`
	// and last-auto-clamp branches.
	tb := NewTable([]TableColumn{
		{Title: "a", Width: 300},
		{Title: "b"}, // auto -- gets clamped to 0
	}, nil)
	got := tb.columnWidths(200)
	if got[0] != 300 {
		t.Fatalf("fixed overflow: col0 width = %d, want 300", got[0])
	}
	if got[1] != 0 {
		t.Fatalf("fixed overflow: auto col width = %d, want 0", got[1])
	}
}

func TestTableColumnWidthsNoColumns(t *testing.T) {
	tb := NewTable(nil, nil)
	if got := tb.columnWidths(200); got != nil {
		t.Fatalf("no-columns widths = %v, want nil", got)
	}
}

// --- Draw: no-op paths --------------------------------------------------

func TestTableDrawZeroBoundsNoOp(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "a"}}, [][]string{{"x"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 100})
	buf := makeTableSurface(50, 50)
	tb.Draw(newP(buf, 50), DefaultLight())
	// Sentinel colour must survive untouched.
	if got := pixelAt(buf, 50, 5, 5); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatalf("zero-W bounds still painted at (5,5): %+v", got)
	}
	tb.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 0})
	tb.Draw(newP(buf, 50), DefaultLight())
	if got := pixelAt(buf, 50, 5, 5); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatalf("zero-H bounds still painted at (5,5): %+v", got)
	}
}

// --- Draw: empty rows placeholder ---------------------------------------

func TestTableDrawEmptyRowsShowsPlaceholder(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}, {Title: "B"}}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	buf := makeTableSurface(200, 100)
	tb.Draw(newP(buf, 200), DefaultLight())
	// Header must have painted its SurfaceAlt fill somewhere in row 0.
	theme := DefaultLight()
	headerPx := pixelAt(buf, 200, 100, TableHeaderHeight/2)
	if headerPx != theme.SurfaceAlt {
		t.Fatalf("header fill missing: got %+v, want SurfaceAlt %+v", headerPx, theme.SurfaceAlt)
	}
	// Placeholder text lands within the first body-row slot; assert
	// at least one OnSurface-coloured pixel exists there.
	found := false
	yLo := TableHeaderHeight
	yHi := TableHeaderHeight + TableRowHeight
	for y := yLo; y < yHi && !found; y++ {
		for x := 0; x < 200; x++ {
			if pixelAt(buf, 200, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no placeholder ink found in body area")
	}
}

// --- Draw: unselected row zebra + separators ----------------------------

func TestTableDrawZebraAndSeparators(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
	}, [][]string{
		{"a0", "b0"},
		{"a1", "b1"},
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	buf := makeTableSurface(120, 200)
	theme := DefaultLight()
	tb.Draw(newP(buf, 120), theme)

	// Row 0 -> Surface, sampled well below its top edge + off the
	// column-separator column and off the header bottom stroke.
	row0Px := tableRowCentrePixel(buf, 120, 30, 0, 0)
	if row0Px != theme.Surface {
		t.Fatalf("row 0 fill = %+v, want Surface %+v", row0Px, theme.Surface)
	}
	// Row 1 -> Background.
	row1Px := tableRowCentrePixel(buf, 120, 30, 0, 1)
	if row1Px != theme.Background {
		t.Fatalf("row 1 fill = %+v, want Background %+v", row1Px, theme.Background)
	}
	// Column separator between col 0 (60px) and col 1 lands at x=60.
	sepPx := pixelAt(buf, 120, 60, TableHeaderHeight+TableRowHeight/2)
	if sepPx != theme.Border {
		t.Fatalf("column separator = %+v, want Border %+v", sepPx, theme.Border)
	}
}

// --- Draw: single-column table has NO separator -------------------------

func TestTableDrawSingleColumnHasNoSeparator(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "Only"}},
		[][]string{{"one"}, {"two"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	buf := makeTableSurface(100, 100)
	theme := DefaultLight()
	tb.Draw(newP(buf, 100), theme)
	// Sample the rightmost column of the widget (x=99). Since the
	// column spans the whole width, no vertical Border stroke should
	// have painted there.
	got := pixelAt(buf, 100, 99, TableHeaderHeight+TableRowHeight/2)
	if got == theme.Border {
		t.Fatalf("single-column table drew a separator at x=99: %+v", got)
	}
}

// --- Draw: selected row highlight ---------------------------------------

func TestTableDrawSelectedRowUsesAccent(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Selected = 1
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	buf := makeTableSurface(100, 200)
	theme := DefaultLight()
	tb.Draw(newP(buf, 100), theme)
	got := tableRowCentrePixel(buf, 100, 50, 0, 1)
	if got != theme.Accent {
		t.Fatalf("selected row fill = %+v, want Accent %+v", got, theme.Accent)
	}
	// Unselected rows unaffected: row 0 -> Surface, row 2 -> Surface
	// (row 2 has even index in zebra pattern).
	if px := tableRowCentrePixel(buf, 100, 50, 0, 0); px != theme.Surface {
		t.Fatalf("row 0 fill w/ selection = %+v, want Surface", px)
	}
	if px := tableRowCentrePixel(buf, 100, 50, 0, 2); px != theme.Surface {
		t.Fatalf("row 2 fill w/ selection = %+v, want Surface", px)
	}
}

// --- Draw: Selected out of range -> no highlight (no crash) -------------

func TestTableDrawSelectedOutOfRangeIgnored(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	buf := makeTableSurface(100, 100)
	theme := DefaultLight()

	// Positive out-of-range.
	tb.Selected = 99
	tb.Draw(newP(buf, 100), theme)
	if px := tableRowCentrePixel(buf, 100, 50, 0, 0); px != theme.Surface {
		t.Fatalf("row 0 fill w/ Selected=99 = %+v, want Surface", px)
	}
	if px := tableRowCentrePixel(buf, 100, 50, 0, 1); px != theme.Background {
		t.Fatalf("row 1 fill w/ Selected=99 = %+v, want Background", px)
	}

	// Negative (other than -1) -- must also be a no-op.
	buf2 := makeTableSurface(100, 100)
	tb.Selected = -42
	tb.Draw(newP(buf2, 100), theme)
	if px := tableRowCentrePixel(buf2, 100, 50, 0, 0); px != theme.Surface {
		t.Fatalf("row 0 fill w/ Selected=-42 = %+v, want Surface", px)
	}
}

// --- Draw: Selected == -1 -> no highlight -------------------------------

func TestTableDrawSelectedMinusOneNoHighlight(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	buf := makeTableSurface(100, 100)
	theme := DefaultLight()
	// Selected is -1 by construction; explicit for readability.
	tb.Selected = -1
	tb.Draw(newP(buf, 100), theme)
	// Neither row should carry the Accent colour anywhere in its band.
	for row := 0; row < 2; row++ {
		yLo := TableHeaderHeight + row*TableRowHeight
		yHi := yLo + TableRowHeight
		for y := yLo; y < yHi; y++ {
			for x := 0; x < 100; x++ {
				if pixelAt(buf, 100, x, y) == theme.Accent {
					t.Fatalf("Accent pixel found at (%d,%d) with Selected=-1", x, y)
				}
			}
		}
	}
}

// --- Draw: OnAccent override via theme.Extra ----------------------------

func TestTableDrawUsesOnAccentFromExtra(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"XYZ"}})
	tb.Selected = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	theme := DefaultLight()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), theme)
	// Somewhere inside the selected row's cell rectangle, at least one
	// glyph pixel must have landed in the custom OnAccent colour.
	found := false
	yLo := TableHeaderHeight
	yHi := TableHeaderHeight + TableRowHeight
	for y := yLo; y < yHi && !found; y++ {
		for x := 0; x < 100; x++ {
			if pixelAt(buf, 100, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in selected row")
	}
}

// --- Draw: Row shorter than Columns -> render only carried cells --------

func TestTableDrawShortRowRendersAvailableCellsOnly(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 50},
		{Title: "B", Width: 50},
		{Title: "C", Width: 50},
	}, [][]string{
		{"aaa"}, // len 1 -- cols B + C are empty
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 150, H: 100})
	buf := makeTableSurface(150, 100)
	theme := DefaultLight()
	// Just prove it doesn't panic + still paints the row background.
	tb.Draw(newP(buf, 150), theme)
	got := tableRowCentrePixel(buf, 150, 75, 0, 0)
	if got != theme.Surface {
		t.Fatalf("short-row body fill = %+v, want Surface %+v", got, theme.Surface)
	}
}

// --- Draw: cell text wider than column -- must not panic ----------------

func TestTableDrawCellTextWiderThanColumn(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz" // > TextWidth than 20px column
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 20},
		{Title: "B", Width: 20},
	}, [][]string{
		{long, long},
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 100})
	buf := makeTableSurface(40, 100)
	// The painter clips per-pixel -- no assertion needed beyond
	// "does not panic".
	tb.Draw(newP(buf, 40), DefaultLight())
}

// --- Draw: nil Extra map covers the accentInk fall-through --------------

func TestTableDrawAccentInkFallbackWithNilExtra(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}})
	tb.Selected = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	theme := DefaultLight()
	theme.Extra = nil
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), theme)
	// Selected row must still paint in Accent (the fill), independent
	// of the ink fall-through picked.
	got := tableRowCentrePixel(buf, 100, 50, 0, 0)
	if got != theme.Accent {
		t.Fatalf("row 0 fill = %+v, want Accent %+v", got, theme.Accent)
	}
}

// --- Draw: Extra map with no OnAccent key covers the second branch ------

func TestTableDrawAccentInkFallbackWithExtraNoKey(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}})
	tb.Selected = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	theme := DefaultLight()
	// Non-nil map without an OnAccent entry -- exercises the ok==false
	// branch of accentInk.
	theme.Extra = map[string]RGBA{"headerbar_bg_color": RGB(1, 2, 3)}
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), theme)
	got := tableRowCentrePixel(buf, 100, 50, 0, 0)
	if got != theme.Accent {
		t.Fatalf("row 0 fill = %+v, want Accent %+v", got, theme.Accent)
	}
}
