// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

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
		{Title: "b"}, // auto
		{Title: "c", Width: 20},
		{Title: "d"}, // auto
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

// --- Per-column alignment (cellTextX) -----------------------------------

func TestCellTextXLeft(t *testing.T) {
	// AlignLeft always sits at the left padding, regardless of width.
	if got := cellTextX(&Base{}, 100, 200, "hi", AlignLeft); got != 100+TableCellPadX {
		t.Fatalf("left = %d, want %d", got, 100+TableCellPadX)
	}
}

func TestCellTextXRight(t *testing.T) {
	// AlignRight: right edge of text flush with the cell's right padding.
	w := TextWidth("hi")
	if got := cellTextX(&Base{}, 100, 200, "hi", AlignRight); got != 100+200-TableCellPadX-w {
		t.Fatalf("right = %d, want %d", got, 100+200-TableCellPadX-w)
	}
}

func TestCellTextXRightClamp(t *testing.T) {
	// A cell narrower than the text would push the start left of the inner
	// edge; it clamps to the left padding instead.
	if got := cellTextX(&Base{}, 100, 1, "wide text", AlignRight); got != 100+TableCellPadX {
		t.Fatalf("right clamp = %d, want %d", got, 100+TableCellPadX)
	}
}

func TestCellTextXCenter(t *testing.T) {
	w := TextWidth("hi")
	if got := cellTextX(&Base{}, 100, 200, "hi", AlignCenter); got != 100+(200-w)/2 {
		t.Fatalf("center = %d, want %d", got, 100+(200-w)/2)
	}
}

func TestCellTextXCenterClamp(t *testing.T) {
	if got := cellTextX(&Base{}, 100, 1, "wide text", AlignCenter); got != 100+TableCellPadX {
		t.Fatalf("center clamp = %d, want %d", got, 100+TableCellPadX)
	}
}

// TestTableDrawAligned exercises the Draw path with a right-aligned + a
// centre-aligned column so the header + body branches both run.
func TestTableDrawAligned(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 120, Align: AlignLeft},
		{Title: "Qty", Width: 60, Align: AlignRight},
		{Title: "OK", Width: 40, Align: AlignCenter},
	}, [][]string{{"widget", "42", "y"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 220, H: 80})
	tb.Draw(newP(makeTableSurface(220, 80), 220), DefaultLight())
}

// --- Sorting: header click + toggle ---------------------------------------

func TestNewTableSortColumnDefaultsToNone(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, nil)
	if tb.SortColumn != -1 {
		t.Fatalf("SortColumn default = %d, want -1", tb.SortColumn)
	}
}

func TestTableHeaderClickSortsAndFiresOnSort(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 50, Sortable: true},
		{Title: "B", Width: 50, Sortable: true},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	var gotCol int
	var gotAsc bool
	calls := 0
	tb.OnSort = func(col int, asc bool) { gotCol, gotAsc = col, asc; calls++ }

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if calls != 1 {
		t.Fatalf("OnSort calls = %d, want 1", calls)
	}
	if gotCol != 0 || !gotAsc {
		t.Fatalf("first click = (col %d, asc %v), want (0, true)", gotCol, gotAsc)
	}
	if tb.SortColumn != 0 || !tb.SortAsc {
		t.Fatalf("state after first click = (col %d, asc %v), want (0, true)", tb.SortColumn, tb.SortAsc)
	}

	// Clicking the SAME column toggles the direction.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if calls != 2 || gotCol != 0 || gotAsc {
		t.Fatalf("second click (toggle) = (col %d, asc %v, calls %d), want (0, false, 2)", gotCol, gotAsc, calls)
	}

	// Clicking a DIFFERENT column resets to ascending.
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	if calls != 3 || gotCol != 1 || !gotAsc {
		t.Fatalf("third click (new column) = (col %d, asc %v, calls %d), want (1, true, 3)", gotCol, gotAsc, calls)
	}
}

func TestTableHeaderClickNonSortableColumnIgnored(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, nil) // Sortable defaults false
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	called := false
	tb.OnSort = func(col int, asc bool) { called = true }
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if called {
		t.Fatal("OnSort fired for a non-Sortable column")
	}
	if tb.SortColumn != -1 {
		t.Fatalf("SortColumn = %d, want -1 (unchanged)", tb.SortColumn)
	}
}

func TestTableHeaderClickBelowHeaderRowIgnored(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100, Sortable: true}}, [][]string{{"x"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	called := false
	tb.OnSort = func(col int, asc bool) { called = true }
	// Y past the header row -- lands in the body, must be a no-op.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 5})
	if called {
		t.Fatal("OnSort fired for a click below the header row")
	}
}

func TestTableHeaderClickNilOnSortNoPanic(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100, Sortable: true}}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// OnSort left nil -- must not panic.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if tb.SortColumn != 0 || !tb.SortAsc {
		t.Fatalf("state = (col %d, asc %v), want (0, true)", tb.SortColumn, tb.SortAsc)
	}
}

func TestTableOnEventIgnoresNonClickWhenNotResizing(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// EventMouseDrag with no active resize + EventMouseUp with nothing
	// in progress must both be safe no-ops.
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 5})
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

// --- Sort indicator rendering ----------------------------------------------

func TestTableDrawSortIndicatorAscending(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"x"}})
	tb.SortColumn = 0
	tb.SortAsc = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	theme := DefaultLight()
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), theme)
	found := false
	for y := 0; y < TableHeaderHeight && !found; y++ {
		for x := 0; x < 100; x++ {
			if pixelAt(buf, 100, x, y) == theme.OnBackground {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no indicator ink found in header row for ascending sort")
	}
}

func TestTableDrawSortIndicatorDescending(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"x"}})
	tb.SortColumn = 0
	tb.SortAsc = false
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	theme := DefaultLight()
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), theme)
	found := false
	for y := 0; y < TableHeaderHeight && !found; y++ {
		for x := 0; x < 100; x++ {
			if pixelAt(buf, 100, x, y) == theme.OnBackground {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no indicator ink found in header row for descending sort")
	}
}

func TestTableDrawSortColumnOutOfRangeNoIndicator(t *testing.T) {
	// SortColumn positive-out-of-range must collapse to "no indicator",
	// same defensive pattern as Selected.
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"x"}})
	tb.SortColumn = 99
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	tb.Draw(newP(makeTableSurface(100, 100), 100), DefaultLight())
}

// --- ColumnSeparatorAt ------------------------------------------------------

func TestColumnSeparatorAtHit(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	if got := tb.ColumnSeparatorAt(60); got != 0 {
		t.Fatalf("ColumnSeparatorAt(60) = %d, want 0", got)
	}
	// Within tolerance either side.
	if got := tb.ColumnSeparatorAt(58); got != 0 {
		t.Fatalf("ColumnSeparatorAt(58) = %d, want 0", got)
	}
	if got := tb.ColumnSeparatorAt(63); got != 0 {
		t.Fatalf("ColumnSeparatorAt(63) = %d, want 0", got)
	}
}

func TestColumnSeparatorAtMiss(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	if got := tb.ColumnSeparatorAt(10); got != -1 {
		t.Fatalf("ColumnSeparatorAt(10) = %d, want -1", got)
	}
}

func TestColumnSeparatorAtSingleColumnAlwaysMiss(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if got := tb.ColumnSeparatorAt(50); got != -1 {
		t.Fatalf("single-column ColumnSeparatorAt = %d, want -1", got)
	}
}

func TestColumnSeparatorAtNoColumnsAlwaysMiss(t *testing.T) {
	tb := NewTable(nil, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if got := tb.ColumnSeparatorAt(50); got != -1 {
		t.Fatalf("no-columns ColumnSeparatorAt = %d, want -1", got)
	}
}

// --- SetColumnWidth ----------------------------------------------------

func TestSetColumnWidthClampsToMinimum(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, nil)
	var gotCol, gotW int
	tb.OnColumnResize = func(col, w int) { gotCol, gotW = col, w }
	tb.SetColumnWidth(0, 1)
	if tb.Columns[0].Width != tableMinColumnWidth {
		t.Fatalf("Width = %d, want clamp to %d", tb.Columns[0].Width, tableMinColumnWidth)
	}
	if gotCol != 0 || gotW != tableMinColumnWidth {
		t.Fatalf("OnColumnResize = (%d, %d), want (0, %d)", gotCol, gotW, tableMinColumnWidth)
	}
}

func TestSetColumnWidthSetsExactAboveMinimum(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 50}}, nil)
	tb.SetColumnWidth(0, 80)
	if tb.Columns[0].Width != 80 {
		t.Fatalf("Width = %d, want 80", tb.Columns[0].Width)
	}
}

func TestSetColumnWidthOutOfRangeColumnNoOp(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 50}}, nil)
	called := false
	tb.OnColumnResize = func(col, w int) { called = true }
	tb.SetColumnWidth(5, 80)
	tb.SetColumnWidth(-1, 80)
	if called {
		t.Fatal("OnColumnResize fired for an out-of-range column")
	}
	if tb.Columns[0].Width != 50 {
		t.Fatalf("Width mutated by out-of-range SetColumnWidth: %d", tb.Columns[0].Width)
	}
}

func TestSetColumnWidthNilOnColumnResizeNoPanic(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 50}}, nil)
	tb.SetColumnWidth(0, 80) // OnColumnResize left nil -- must not panic.
	if tb.Columns[0].Width != 80 {
		t.Fatalf("Width = %d, want 80", tb.Columns[0].Width)
	}
}

// --- Resize drag via OnEvent ------------------------------------------------

func TestTableResizeDragAdjustsWidthAndFiresOnColumnResize(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	var gotCol, gotW int
	calls := 0
	tb.OnColumnResize = func(col, w int) { gotCol, gotW = col, w; calls++ }

	// Grab the separator at x=60 (between col 0 + col 1).
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	// Drag it to x=90 -- column 0 should now be 90px wide.
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 5})
	if calls != 1 {
		t.Fatalf("OnColumnResize calls = %d, want 1", calls)
	}
	if gotCol != 0 || gotW != 90 {
		t.Fatalf("resize = (col %d, w %d), want (0, 90)", gotCol, gotW)
	}
	if tb.Columns[0].Width != 90 {
		t.Fatalf("Columns[0].Width = %d, want 90", tb.Columns[0].Width)
	}

	// A further drag continues adjusting the same column.
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 100, Y: 5})
	if tb.Columns[0].Width != 100 {
		t.Fatalf("Columns[0].Width after second drag = %d, want 100", tb.Columns[0].Width)
	}

	// Mouse-up ends the drag; further EventMouseDrag ticks are no-ops.
	tb.OnEvent(Event{Kind: EventMouseUp, X: 100, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 20, Y: 5})
	if tb.Columns[0].Width != 100 {
		t.Fatalf("Columns[0].Width after release = %d, want unchanged 100", tb.Columns[0].Width)
	}
}

func TestTableResizeDragClampsToMinimum(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	// Drag far left of column 0's left edge.
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 0, Y: 5})
	if tb.Columns[0].Width != tableMinColumnWidth {
		t.Fatalf("Columns[0].Width = %d, want clamp to %d", tb.Columns[0].Width, tableMinColumnWidth)
	}
}

func TestTableHeaderClickPastLastColumnIgnored(t *testing.T) {
	// columnAt's "not inside any column" branch: click X is past the
	// last column's right edge (fixed widths short of the widget's
	// full Bounds().W), so no column -- and therefore no sort -- fires.
	tb := NewTable([]TableColumn{{Title: "A", Width: 40, Sortable: true}}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	called := false
	tb.OnSort = func(col int, asc bool) { called = true }
	tb.OnEvent(Event{Kind: EventClick, X: 80, Y: 5})
	if called {
		t.Fatal("OnSort fired for a click past the last column")
	}
}

func TestTableResizeDragOnNonFirstSeparatorSumsLeadingWidths(t *testing.T) {
	// Grabbing a separator OTHER than index 0 exercises the leading-
	// width accumulation loop in the EventMouseDrag branch.
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 40},
		{Title: "B", Width: 40},
		{Title: "C", Width: 40},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	var gotCol, gotW int
	tb.OnColumnResize = func(col, w int) { gotCol, gotW = col, w }

	// Separator 1 sits at x=80 (col0 40 + col1 40).
	tb.OnEvent(Event{Kind: EventClick, X: 80, Y: 5})
	if !tb.resizing || tb.resizingCol != 1 {
		t.Fatalf("resize state = (resizing %v, col %d), want (true, 1)", tb.resizing, tb.resizingCol)
	}
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 100, Y: 5})
	if gotCol != 1 || gotW != 60 {
		t.Fatalf("resize = (col %d, w %d), want (1, 60)", gotCol, gotW)
	}
	if tb.Columns[1].Width != 60 {
		t.Fatalf("Columns[1].Width = %d, want 60", tb.Columns[1].Width)
	}
}

// --- Multi-row selection: model methods --------------------------------

func TestIsRowSelectedDefaultFalse(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"x"}, {"y"}})
	if tb.IsRowSelected(0) {
		t.Fatal("fresh Table reports row 0 selected")
	}
	if tb.IsRowSelected(-1) {
		t.Fatal("negative index must never report selected")
	}
}

func TestSetRowSelectionReplacesAndDropsNegative(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"a"}, {"b"}, {"c"}})
	tb.SetRowSelection(0, 2, -5)
	if !tb.IsRowSelected(0) || !tb.IsRowSelected(2) || tb.IsRowSelected(1) {
		t.Fatalf("selection after SetRowSelection(0,2,-5) wrong: %v", tb.SelectedRows())
	}
	// A second call fully replaces the first.
	tb.SetRowSelection(1)
	if tb.IsRowSelected(0) || tb.IsRowSelected(2) || !tb.IsRowSelected(1) {
		t.Fatalf("SetRowSelection did not replace prior selection: %v", tb.SelectedRows())
	}
	// No-args (or all-negative) clears.
	tb.SetRowSelection()
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SetRowSelection() = %v, want nil", got)
	}
	tb.SetRowSelection(1)
	tb.SetRowSelection(-1, -2)
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SetRowSelection(all-negative) = %v, want nil", got)
	}
}

func TestClearRowSelection(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"a"}, {"b"}})
	tb.SetRowSelection(0, 1)
	tb.ClearRowSelection()
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows after Clear = %v, want nil", got)
	}
	if tb.IsRowSelected(0) || tb.IsRowSelected(1) {
		t.Fatal("rows still report selected after ClearRowSelection")
	}
}

func TestToggleRowSelect(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"a"}, {"b"}})
	tb.ToggleRowSelect(0)
	if !tb.IsRowSelected(0) {
		t.Fatal("row 0 not selected after first toggle")
	}
	tb.ToggleRowSelect(0)
	if tb.IsRowSelected(0) {
		t.Fatal("row 0 still selected after second toggle")
	}
	// Negative index is a documented no-op -- must not panic or seed
	// a phantom entry.
	tb.ToggleRowSelect(-1)
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows after negative toggle = %v, want nil", got)
	}
}

func TestToggleRowSelectOnFreshTableWithNilMap(t *testing.T) {
	// Covers the selectedRows == nil branch of ToggleRowSelect -- a
	// Table constructed directly (not through NewTable) still works.
	tb := &Table{Columns: []TableColumn{{Title: "A"}}, Rows: [][]string{{"a"}}}
	tb.ToggleRowSelect(0)
	if !tb.IsRowSelected(0) {
		t.Fatal("ToggleRowSelect on nil selectedRows map failed to select")
	}
}

func TestSelectedRowsAscendingOrder(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"a"}, {"b"}, {"c"}, {"d"}})
	tb.SetRowSelection(3, 0, 2)
	got := tb.SelectedRows()
	want := []int{0, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("SelectedRows() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SelectedRows() = %v, want %v", got, want)
		}
	}
}

func TestSelectRowRangeBothDirections(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}})
	tb.SelectRowRange(1, 3)
	if got := tb.SelectedRows(); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("SelectRowRange(1,3) = %v, want [1 2 3]", got)
	}
	// Reversed endpoints (b < a) must yield the same inclusive range.
	tb.SelectRowRange(4, 2)
	if got := tb.SelectedRows(); len(got) != 3 || got[0] != 2 || got[2] != 4 {
		t.Fatalf("SelectRowRange(4,2) = %v, want [2 3 4]", got)
	}
	// A negative low endpoint (e.g. an unset -1 anchor) clamps to 0.
	tb.SelectRowRange(-1, 1)
	if got := tb.SelectedRows(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("SelectRowRange(-1,1) = %v, want [0 1]", got)
	}
	// Both endpoints negative -- empty selection, no panic.
	tb.SelectRowRange(-5, -1)
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SelectRowRange(-5,-1) = %v, want nil", got)
	}
}

// --- Multi-row selection: OnEvent body-row clicks -----------------------

func TestMultiSelectDisabledBodyClickIsNoOp(t *testing.T) {
	// MultiSelect defaults false -- a body-row click must be a total
	// no-op, exactly matching pre-MultiSelect behaviour.
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	tb.Selected = -1
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 5})
	if tb.Selected != -1 {
		t.Fatalf("Selected mutated by body click w/o MultiSelect: %d", tb.Selected)
	}
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows mutated by body click w/o MultiSelect: %v", got)
	}
}

func TestMultiSelectPlainClickSelectsOnlyThatRowAndMovesAnchor(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	tb.SetRowSelection(0, 2) // pre-seed a selection to prove it gets cleared

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + TableRowHeight + 1}) // row 1
	if tb.Selected != 1 {
		t.Fatalf("Selected = %d, want 1 (anchor moved to plain-clicked row)", tb.Selected)
	}
	got := tb.SelectedRows()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("SelectedRows after plain click = %v, want [1]", got)
	}
}

func TestMultiSelectCtrlClickTogglesWithoutMovingAnchor(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	tb.Selected = 0
	tb.SetRowSelection(0)

	// Ctrl-click row 2 adds it without disturbing the anchor.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2*TableRowHeight + 1, Ctrl: true})
	if tb.Selected != 0 {
		t.Fatalf("Selected = %d, want 0 (anchor unmoved by Ctrl-click)", tb.Selected)
	}
	got := tb.SelectedRows()
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("SelectedRows after Ctrl-click add = %v, want [0 2]", got)
	}

	// Ctrl-click row 0 again removes it (still selected).
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 1, Ctrl: true})
	got = tb.SelectedRows()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("SelectedRows after Ctrl-click remove = %v, want [2]", got)
	}
}

func TestMultiSelectShiftClickRangeBothDirections(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}, {"r3"}, {"r4"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})

	// Plain click sets the anchor at row 1.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + TableRowHeight + 1})
	if tb.Selected != 1 {
		t.Fatalf("anchor after plain click = %d, want 1", tb.Selected)
	}
	// Shift-click row 3 -- range extends DOWNWARD from the anchor.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 3*TableRowHeight + 1, Shift: true})
	if tb.Selected != 1 {
		t.Fatalf("anchor moved by Shift-click: %d, want unchanged 1", tb.Selected)
	}
	got := tb.SelectedRows()
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("SelectedRows after downward Shift-click = %v, want [1 2 3]", got)
	}

	// A second Shift-click, row 0 -- range recomputed UPWARD from the
	// SAME anchor (not cumulative with the previous range).
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 1, Shift: true})
	got = tb.SelectedRows()
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("SelectedRows after upward Shift-click = %v, want [0 1]", got)
	}
}

func TestMultiSelectShiftClickWithNoPriorAnchorRangesFromTop(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	// Selected starts at -1 (NewTable default) -- Shift-click row 2
	// must clamp the range to start at 0.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2*TableRowHeight + 1, Shift: true})
	got := tb.SelectedRows()
	if len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("SelectedRows after Shift-click w/ no anchor = %v, want [0 1 2]", got)
	}
}

func TestMultiSelectBodyClickPastLastRowIgnored(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	// Y falls in the body area but past the last row's band.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2*TableRowHeight + 5})
	if got := tb.SelectedRows(); got != nil {
		t.Fatalf("SelectedRows after past-last-row click = %v, want nil", got)
	}
	if tb.Selected != -1 {
		t.Fatalf("Selected mutated by past-last-row click: %d", tb.Selected)
	}
}

func TestMultiSelectModifierBodyClickDoesNotTriggerSortOrResize(t *testing.T) {
	// A Ctrl/Shift click on a BODY row must never be mistaken for a
	// header sort or a separator resize -- those only ever look at
	// ev.Y < TableHeaderHeight, regardless of modifiers.
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 50, Sortable: true},
		{Title: "B", Width: 50, Sortable: true},
	}, [][]string{{"a0", "b0"}, {"a1", "b1"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	sortFired := false
	tb.OnSort = func(col int, asc bool) { sortFired = true }
	resizeFired := false
	tb.OnColumnResize = func(col, w int) { resizeFired = true }

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 1, Ctrl: true})
	tb.OnEvent(Event{Kind: EventClick, X: 50, Y: TableHeaderHeight + TableRowHeight + 1, Shift: true})
	if sortFired {
		t.Fatal("OnSort fired for a modifier body-row click")
	}
	if resizeFired || tb.resizing {
		t.Fatal("resize state entered for a modifier body-row click")
	}
	if got := tb.SelectedRows(); len(got) == 0 {
		t.Fatalf("modifier body clicks produced no selection: %v", got)
	}
}

func TestMultiSelectHeaderSortAndResizeStillWorkWithMultiSelectTrue(t *testing.T) {
	// Regression: enabling MultiSelect must not disturb existing
	// header-click sort or separator-drag resize behaviour.
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60, Sortable: true},
		{Title: "B", Width: 60, Sortable: true},
	}, [][]string{{"a0", "b0"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})

	var gotCol int
	var gotAsc bool
	tb.OnSort = func(col int, asc bool) { gotCol, gotAsc = col, asc }
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if gotCol != 0 || !gotAsc {
		t.Fatalf("sort with MultiSelect=true: got (%d,%v), want (0,true)", gotCol, gotAsc)
	}

	var gotRCol, gotRW int
	tb.OnColumnResize = func(col, w int) { gotRCol, gotRW = col, w }
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 5})
	if gotRCol != 0 || gotRW != 90 {
		t.Fatalf("resize with MultiSelect=true: got (%d,%d), want (0,90)", gotRCol, gotRW)
	}
	tb.OnEvent(Event{Kind: EventMouseUp, X: 90, Y: 5})
}

func TestTableOnEventClickNegativeYIgnored(t *testing.T) {
	// Covers the ev.Y < 0 guard at the very top of the EventClick
	// branch -- a coordinate a container might hand down during an
	// edge-case hit-test translation must never panic or fall through
	// to header/body handling.
	tb := NewTable([]TableColumn{{Title: "A", Width: 100, Sortable: true}},
		[][]string{{"r0"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	called := false
	tb.OnSort = func(col int, asc bool) { called = true }
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: -5})
	if called {
		t.Fatal("OnSort fired for a negative-Y click")
	}
	if tb.Selected != -1 || tb.SelectedRows() != nil {
		t.Fatalf("selection mutated by negative-Y click: Selected=%d rows=%v", tb.Selected, tb.SelectedRows())
	}
}

func TestRowAtAboveHeaderAndPastLastRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"a"}, {"b"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	if got := tb.rowAt(0); got != -1 {
		t.Fatalf("rowAt(0) [in header] = %d, want -1", got)
	}
	if got := tb.rowAt(TableHeaderHeight - 1); got != -1 {
		t.Fatalf("rowAt(header bottom edge) = %d, want -1", got)
	}
	if got := tb.rowAt(TableHeaderHeight + 2*TableRowHeight); got != -1 {
		t.Fatalf("rowAt(past last row) = %d, want -1", got)
	}
	if got := tb.rowAt(TableHeaderHeight + TableRowHeight/2); got != 0 {
		t.Fatalf("rowAt(row 0 centre) = %d, want 0", got)
	}
}

// --- Multi-row selection: Draw rendering --------------------------------

func TestTableDrawMultiSelectHighlightsEverySelectedRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.MultiSelect = true
	tb.Selected = -1
	tb.SetRowSelection(0, 2)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	theme := DefaultLight()
	buf := makeTableSurface(100, 200)
	tb.Draw(newP(buf, 100), theme)

	if got := tableRowCentrePixel(buf, 100, 50, 0, 0); got != theme.Accent {
		t.Fatalf("row 0 fill = %+v, want Accent (multi-selected)", got)
	}
	if got := tableRowCentrePixel(buf, 100, 50, 0, 2); got != theme.Accent {
		t.Fatalf("row 2 fill = %+v, want Accent (multi-selected)", got)
	}
	// Row 1 is neither Selected nor in the selection set -- plain
	// zebra (odd index -> Background).
	if got := tableRowCentrePixel(buf, 100, 50, 0, 1); got != theme.Background {
		t.Fatalf("row 1 fill = %+v, want Background (unselected)", got)
	}
}

func TestTableDrawMultiSelectFalseIgnoresSelectedRowsSet(t *testing.T) {
	// Even if selectedRows was seeded directly (e.g. via the host API
	// before flipping MultiSelect back off), Draw with MultiSelect
	// false must ignore it entirely -- byte-identical to pre-feature
	// rendering, driven only by Selected.
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.SetRowSelection(0, 2)
	tb.Selected = 1
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	theme := DefaultLight()
	buf := makeTableSurface(100, 200)
	tb.Draw(newP(buf, 100), theme)

	if got := tableRowCentrePixel(buf, 100, 50, 0, 1); got != theme.Accent {
		t.Fatalf("row 1 (Selected) fill = %+v, want Accent", got)
	}
	if got := tableRowCentrePixel(buf, 100, 50, 0, 0); got != theme.Surface {
		t.Fatalf("row 0 fill = %+v, want Surface (selectedRows ignored w/ MultiSelect=false)", got)
	}
	if got := tableRowCentrePixel(buf, 100, 50, 0, 2); got != theme.Surface {
		t.Fatalf("row 2 fill = %+v, want Surface (selectedRows ignored w/ MultiSelect=false)", got)
	}
}

func TestTableSeparatorClickTakesPriorityOverSort(t *testing.T) {
	// A header cell that is ALSO Sortable must not fire OnSort when the
	// click actually lands on the adjacent separator -- resize wins.
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60, Sortable: true},
		{Title: "B", Width: 60, Sortable: true},
	}, nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	sortFired := false
	tb.OnSort = func(col int, asc bool) { sortFired = true }
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	if sortFired {
		t.Fatal("OnSort fired for a separator-hit click")
	}
	if !tb.resizing || tb.resizingCol != 0 {
		t.Fatalf("resize state = (resizing %v, col %d), want (true, 0)", tb.resizing, tb.resizingCol)
	}
}

// --- Vertical scroll window (body-row virtualization) -------------------

// tableManyRows builds n single-column rows labelled "r0".."r{n-1}", for
// tests that need a table taller than any reasonable Bounds().H.
func tableManyRows(n int) [][]string {
	rows := make([][]string, n)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("r%d", i)}
	}
	return rows
}

func TestTableBodyVisibleRowsTinyHeightIsZero(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"r0"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: TableHeaderHeight})
	if got := tb.bodyVisibleRows(); got != 0 {
		t.Fatalf("bodyVisibleRows() at H==TableHeaderHeight = %d, want 0", got)
	}
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: TableHeaderHeight - 5})
	if got := tb.bodyVisibleRows(); got != 0 {
		t.Fatalf("bodyVisibleRows() at H<TableHeaderHeight = %d, want 0", got)
	}
}

// TestTableScrollSmallTableUnchangedAndRegressionsStillWork is the core
// "byte-identical when everything fits" guarantee: no scrollbar paints,
// and header-click sort, separator-drag resize and multi-select all keep
// working exactly as before ScrollRow existed.
func TestTableScrollSmallTableUnchangedAndRegressionsStillWork(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60, Sortable: true},
		{Title: "B", Width: 60, Sortable: true},
	}, [][]string{{"a0", "b0"}, {"a1", "b1"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	theme := DefaultLight()
	buf := makeTableSurface(120, 200)
	tb.Draw(newP(buf, 120), theme)

	// No scrollbar: Accent must never appear in the would-be track
	// column (rightmost scrollbarWidth px) while nothing is selected.
	for y := TableHeaderHeight; y < TableHeaderHeight+2*TableRowHeight; y++ {
		for x := 120 - scrollbarWidth; x < 120; x++ {
			if got := pixelAt(buf, 120, x, y); got == theme.Accent {
				t.Fatalf("unexpected scrollbar-coloured pixel at (%d,%d) on a fully-fitting table", x, y)
			}
		}
	}

	// Sort still works.
	var gotCol int
	var gotAsc bool
	tb.OnSort = func(col int, asc bool) { gotCol, gotAsc = col, asc }
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if gotCol != 0 || !gotAsc || tb.SortColumn != 0 {
		t.Fatalf("sort broken: gotCol=%d gotAsc=%v SortColumn=%d", gotCol, gotAsc, tb.SortColumn)
	}

	// Separator-drag resize still works.
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 5})
	if tb.Columns[0].Width != 90 {
		t.Fatalf("resize broken: Columns[0].Width = %d, want 90", tb.Columns[0].Width)
	}
	tb.OnEvent(Event{Kind: EventMouseUp, X: 90, Y: 5})

	// Multi-select still works.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 1})
	if tb.Selected != 0 {
		t.Fatalf("multi-select broken: Selected = %d, want 0", tb.Selected)
	}
	if got := tb.SelectedRows(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("multi-select broken: SelectedRows() = %v, want [0]", got)
	}
}

// TestTableScrollWindowsLargeTable is the core windowing test: only the
// visible slice paints, and the scrollbar thumb appears while it does.
func TestTableScrollWindowsLargeTable(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	// Body = exactly 5 rows (24 + 5*22 = 134): more rows than fit, so
	// the body overflows and a scrollbar is expected.
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134})
	theme := DefaultLight()

	// The surface is much taller than the widget -- an accidental
	// un-windowed render (all 20 rows, as the pre-feature Table always
	// did) would leave ink far below Bounds().H. The sentinel colour
	// surviving there proves the windowing loop actually stopped early.
	buf := makeTableSurface(120, 600)
	tb.Draw(newP(buf, 120), theme)

	// In-window: absolute row 2 (screen position 2) painted with its
	// zebra colour (even index -> Surface).
	inWinY := TableHeaderHeight + 2*TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 120, 50, inWinY); got != theme.Surface {
		t.Fatalf("in-window row 2 fill = %+v, want Surface", got)
	}

	// Off-window: row 19's position had the Table rendered UN-windowed
	// is far below Bounds().H -- must remain the untouched sentinel.
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	offWinY := TableHeaderHeight + 19*TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 120, 50, offWinY); got != sentinel {
		t.Fatalf("off-window row 19 was painted at %+v, want untouched sentinel", got)
	}

	// Scrollbar thumb: an Accent-coloured pixel exists in the track
	// column while the body overflows (no row is selected, so Accent
	// can only be the thumb here).
	foundThumb := false
	for y := TableHeaderHeight; y < 134 && !foundThumb; y++ {
		if pixelAt(buf, 120, 115, y) == theme.Accent {
			foundThumb = true
		}
	}
	if !foundThumb {
		t.Fatal("no scrollbar thumb pixel found while body overflows")
	}
}

// TestTableScrollZebraParityPreservedByAbsoluteIndex proves the zebra
// stripe is keyed on each row's ABSOLUTE index, not its on-screen
// position -- scrolling must never shift which rows read as odd/even.
func TestTableScrollZebraParityPreservedByAbsoluteIndex(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5 visible rows
	tb.ScrollRow = 11                              // odd absolute index lands at screen position 0
	theme := DefaultLight()
	buf := makeTableSurface(120, 200)
	tb.Draw(newP(buf, 120), theme)

	// Absolute row 11 (odd) at SCREEN position 0 must be Background --
	// a zebra keyed on screen position would wrongly paint Surface here
	// (position 0 is even).
	y0 := TableHeaderHeight + TableRowHeight/2
	if got := pixelAt(buf, 120, 50, y0); got != theme.Background {
		t.Fatalf("row 11 at screen pos 0 fill = %+v, want Background (zebra keyed by absolute index)", got)
	}
	// Absolute row 12 (even) at screen position 1 must be Surface.
	y1 := TableHeaderHeight + TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 120, 50, y1); got != theme.Surface {
		t.Fatalf("row 12 at screen pos 1 fill = %+v, want Surface", got)
	}
}

// TestTableScrollClipsPartiallyVisibleLastRow covers the Clipper branch:
// when Bounds().H isn't an exact multiple of TableRowHeight, the last
// windowed row only partially fits -- without clipping it would spill
// past the widget's own bottom edge.
func TestTableScrollClipsPartiallyVisibleLastRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(10))
	// Body height 50px is NOT a multiple of TableRowHeight (22): 2 full
	// rows (44px) + a 6px sliver of a 3rd. bodyVisibleRows rounds up to
	// 3, so row 2 (the 3rd visible row) paints only partially inside
	// Bounds().H.
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TableHeaderHeight + 50})
	theme := DefaultLight()
	// Surface taller than the widget so an unclipped row 2 would spill
	// visibly below Bounds().H into the sentinel area.
	buf := makeTableSurface(120, 150)
	tb.Draw(newP(buf, 120), theme)

	widgetBottom := TableHeaderHeight + 50 // 74
	// Row 2's unclipped band spans y in [68, 90); this y sits inside
	// that band but past the widget's own bottom edge (74).
	spillY := widgetBottom + 5 // 79
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	if got := pixelAt(buf, 120, 50, spillY); got != sentinel {
		t.Fatalf("clip failed: pixel at (50,%d) below Bounds().H = %+v, want untouched sentinel", spillY, got)
	}
}

// TestTableScrollClickWithNonZeroScrollRowSelectsCorrectRow covers the
// rowAt mapping through clampScrollRow: a click's local Y must resolve
// to the ABSOLUTE row currently painted there, not the on-screen offset.
func TestTableScrollClickWithNonZeroScrollRowSelectsCorrectRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5 visible rows
	tb.ScrollRow = 10

	// Click at screen row 2 (3rd visible band) -- must resolve to
	// ABSOLUTE row 12 (10+2), not row 2.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2*TableRowHeight + 1})
	if tb.Selected != 12 {
		t.Fatalf("Selected = %d, want 12 (ScrollRow 10 + screen row 2)", tb.Selected)
	}
}

// TestTableScrollRowClampBothEnds exercises ScrollTo's clamp at both the
// low (negative) and high (past maxScrollRow) ends, plus the
// pass-through middle case.
func TestTableScrollRowClampBothEnds(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5 visible rows -> maxScrollRow == 15

	tb.ScrollTo(-5)
	if tb.ScrollRow != 0 {
		t.Fatalf("ScrollTo(-5) = %d, want clamp to 0", tb.ScrollRow)
	}
	tb.ScrollTo(999)
	if tb.ScrollRow != 15 {
		t.Fatalf("ScrollTo(999) = %d, want clamp to 15 (maxScrollRow)", tb.ScrollRow)
	}
	tb.ScrollTo(7)
	if tb.ScrollRow != 7 {
		t.Fatalf("ScrollTo(7) = %d, want 7 (within range, unclamped)", tb.ScrollRow)
	}
}

func TestTableScrollByAdjustsAndClamps(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // maxScrollRow == 15

	tb.ScrollBy(5)
	if tb.ScrollRow != 5 {
		t.Fatalf("ScrollBy(5) = %d, want 5", tb.ScrollRow)
	}
	tb.ScrollBy(-100)
	if tb.ScrollRow != 0 {
		t.Fatalf("ScrollBy(-100) = %d, want clamp to 0", tb.ScrollRow)
	}
	tb.ScrollBy(1000)
	if tb.ScrollRow != 15 {
		t.Fatalf("ScrollBy(1000) = %d, want clamp to 15", tb.ScrollRow)
	}
}

// TestTableScrollScrollbarThumbFloorClamp covers drawScrollbar's
// tableScrollbarThumbMin clamp: with a huge row count crammed behind a
// short track, the proportional thumb height would compute to less than
// the floor and must be clamped up to it.
func TestTableScrollScrollbarThumbFloorClamp(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(500))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5-row track vs. 500 rows of content
	theme := DefaultLight()
	buf := makeTableSurface(120, 200)
	tb.Draw(newP(buf, 120), theme)

	// The clamped-up thumb must still appear (at least
	// tableScrollbarThumbMin px tall) at the top of the track.
	found := false
	for y := TableHeaderHeight; y < 134 && !found; y++ {
		if pixelAt(buf, 120, 115, y) == theme.Accent {
			found = true
		}
	}
	if !found {
		t.Fatal("no floor-clamped scrollbar thumb pixel found")
	}
}

// TestTableScrollContentWidthNegativeClamp covers contentWidth's w<0
// clamp: a widget narrower than scrollbarWidth, with an overflowing
// body, must not hand columnWidths a negative budget.
func TestTableScrollContentWidthNegativeClamp(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, tableManyRows(10))
	// W (5) < scrollbarWidth (8); body overflows vertically (10 rows,
	// only 5 fit in H=134), so contentWidth reserves scrollbarWidth and
	// must clamp the result to 0 instead of going negative.
	tb.SetBounds(Rect{X: 0, Y: 0, W: 5, H: 134})
	if got := tb.contentWidth(); got != 0 {
		t.Fatalf("contentWidth() = %d, want clamp to 0", got)
	}
	// Draw must not panic with this degenerate width either.
	tb.Draw(newP(makeTableSurface(5, 200), 5), DefaultLight())
}

func TestTableScrollToSelectedNoopWhenSelectedNegative(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	tb.Selected = -1
	tb.ScrollRow = 3
	tb.scrollToSelected()
	if tb.ScrollRow != 3 {
		t.Fatalf("ScrollRow mutated by scrollToSelected with Selected=-1: %d, want unchanged 3", tb.ScrollRow)
	}
}

func TestTableScrollToSelectedNoopWhenNoVisibleCapacity(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: TableHeaderHeight}) // 0 visible rows
	tb.Selected = 1
	tb.scrollToSelected()
	if tb.ScrollRow != 0 {
		t.Fatalf("ScrollRow = %d, want unchanged 0 when body has no visible capacity", tb.ScrollRow)
	}
}

// --- Drag-to-reorder body rows ------------------------------------------

// tableBodyClickY returns the Table-local Y that lands inside body row
// idx's band (its vertical centre), assuming ScrollRow == 0 -- the same
// geometry TableHeaderHeight + row*TableRowHeight + TableRowHeight/2
// tests elsewhere already inline.
func tableBodyClickY(idx int) int {
	return TableHeaderHeight + idx*TableRowHeight + TableRowHeight/2
}

// tableInsertY returns the Table-local Y that rowInsertIndexAt resolves
// back to exactly idx, for a Table currently scrolled to scroll. Derived
// from rowInsertIndexAt's own rounding formula: the row boundary itself
// (no half-row offset) always floors back to idx.
func tableInsertY(scroll, idx int) int {
	return TableHeaderHeight + (idx-scroll)*TableRowHeight
}

// TestTableReorderableFalseIsByteIdentical is the core regression guard:
// with Reorderable left at its zero value (false), DragData/AcceptsDrop
// are inert, every drag event is a no-op (Rows never mutate), and
// pre-existing sort/resize/multi-select behaviour is completely
// unaffected.
func TestTableReorderableFalseIsByteIdentical(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60, Sortable: true},
		{Title: "B", Width: 60, Sortable: true},
	}, [][]string{{"a0", "b0"}, {"a1", "b1"}, {"a2", "b2"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})

	// A body-row press must not arm a drag.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(0)})
	if got := tb.DragData(); got != "" {
		t.Fatalf("DragData() with Reorderable=false = %q, want \"\"", got)
	}
	if tb.AcceptsDrop("tablerow:0") {
		t.Fatal("AcceptsDrop(own scheme) = true with Reorderable=false")
	}

	// A drop carrying a well-formed tablerow payload must not touch Rows.
	before := fmt.Sprintf("%v", tb.Rows)
	tb.OnEvent(Event{Kind: EventDragMove, Y: tableBodyClickY(1)})
	tb.OnEvent(Event{Kind: EventDrop, Y: tableBodyClickY(1), Code: "tablerow:0"})
	if got := fmt.Sprintf("%v", tb.Rows); got != before {
		t.Fatalf("Rows mutated by a drop with Reorderable=false: %v", tb.Rows)
	}

	// Sort still works.
	var gotCol int
	var gotAsc bool
	tb.OnSort = func(col int, asc bool) { gotCol, gotAsc = col, asc }
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if gotCol != 0 || !gotAsc {
		t.Fatalf("sort broken: (%d,%v), want (0,true)", gotCol, gotAsc)
	}

	// Separator resize still works.
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 5})
	if tb.Columns[0].Width != 90 {
		t.Fatalf("resize broken: Columns[0].Width = %d, want 90", tb.Columns[0].Width)
	}
	tb.OnEvent(Event{Kind: EventMouseUp, X: 90, Y: 5})

	// Multi-select still works.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(0)})
	if tb.Selected != 0 {
		t.Fatalf("multi-select broken: Selected = %d, want 0", tb.Selected)
	}

	// Draw must not paint any indicator line -- sample every row boundary
	// in the body for a stray Accent pixel (none of the rows/selection
	// above should have landed one at these exact boundary rows either,
	// but row 0 IS selected/Accent-filled, so only sample boundary rows
	// that are neither selected nor a zebra Background/Surface fill
	// ambiguity: use row 2, which is unselected).
	theme := DefaultLight()
	buf := makeTableSurface(120, 200)
	tb.Draw(newP(buf, 120), theme)
	boundaryY := TableHeaderHeight + 2*TableRowHeight
	if got := pixelAt(buf, 120, 30, boundaryY); got == theme.Accent {
		t.Fatalf("indicator-coloured pixel found at row-2 boundary with Reorderable=false: %+v", got)
	}
}

// TestTableReorderableDragDataForPressedBodyRow covers the DragSource
// half of the contract: a body-row press records dragRow, and DragData
// reports it via the private "tablerow:" scheme.
func TestTableReorderableDragDataForPressedBodyRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})

	if got := tb.DragData(); got != "" {
		t.Fatalf("DragData() before any press = %q, want \"\"", got)
	}
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(1)})
	if got := tb.DragData(); got != "tablerow:1" {
		t.Fatalf("DragData() after pressing row 1 = %q, want %q", got, "tablerow:1")
	}
	// Pressing a different row re-arms the payload.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(0)})
	if got := tb.DragData(); got != "tablerow:0" {
		t.Fatalf("DragData() after pressing row 0 = %q, want %q", got, "tablerow:0")
	}
}

// TestTableReorderableHeaderAndSeparatorPressDoNotArmDrag covers the
// "header/separator presses do NOT start a row drag" requirement: both
// land above TableHeaderHeight and must leave dragRow (and therefore
// DragData) untouched, while a subsequent body-row press still works.
func TestTableReorderableHeaderAndSeparatorPressDoNotArmDrag(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60, Sortable: true},
		{Title: "B", Width: 60, Sortable: true},
	}, [][]string{{"a0", "b0"}, {"a1", "b1"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})

	// Header-cell click (sorts column 0).
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if got := tb.DragData(); got != "" {
		t.Fatalf("DragData() after header click = %q, want \"\"", got)
	}

	// Separator click (starts a resize).
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: 5})
	if !tb.resizing {
		t.Fatal("separator click did not start a resize (test setup broken)")
	}
	if got := tb.DragData(); got != "" {
		t.Fatalf("DragData() after separator click = %q, want \"\"", got)
	}
	tb.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: 5})

	// A genuine body-row press still arms the drag afterwards.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(1)})
	if got := tb.DragData(); got != "tablerow:1" {
		t.Fatalf("DragData() after body press = %q, want %q", got, "tablerow:1")
	}
}

// TestTableAcceptsDropOwnSchemeVsForeign covers AcceptsDrop's payload
// validation: only a well-formed "tablerow:<non-negative int>" payload
// is accepted; any other scheme, a negative index, or garbage is not.
func TestTableAcceptsDropOwnSchemeVsForeign(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"r0"}})
	tb.Reorderable = true

	cases := []struct {
		payload string
		want    bool
	}{
		{"tablerow:0", true},
		{"tablerow:5", true}, // AcceptsDrop only validates the scheme/format, not row range
		{"/tmp/file.txt", false},
		{"tablerow:-1", false},
		{"tablerow:abc", false},
		{"tablerow:", false},
		{"", false},
	}
	for _, c := range cases {
		if got := tb.AcceptsDrop(c.payload); got != c.want {
			t.Errorf("AcceptsDrop(%q) = %v, want %v", c.payload, got, c.want)
		}
	}
}

// TestTableDragMoveSetsIndicatorAndDrawPaintsIt covers EventDragMove
// computing dropIndicator + Draw painting the insertion line, and
// EventDragLeave clearing it again.
func TestTableDragMoveSetsIndicatorAndDrawPaintsIt(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	theme := DefaultLight()

	// Move the drag to the boundary between row 0 and row 1 (insertion
	// index 1).
	tb.OnEvent(Event{Kind: EventDragMove, X: 10, Y: tableInsertY(0, 1)})
	if tb.dropIndicator != 1 {
		t.Fatalf("dropIndicator after DragMove = %d, want 1", tb.dropIndicator)
	}
	buf := makeTableSurface(100, 200)
	tb.Draw(newP(buf, 100), theme)
	indicatorY := TableHeaderHeight + 1*TableRowHeight
	if got := pixelAt(buf, 100, 50, indicatorY); got != theme.Accent {
		t.Fatalf("indicator pixel at (50,%d) = %+v, want Accent", indicatorY, got)
	}

	// Leaving clears the indicator: a fresh Draw no longer paints it.
	tb.OnEvent(Event{Kind: EventDragLeave})
	if tb.dropIndicator != -1 {
		t.Fatalf("dropIndicator after DragLeave = %d, want -1", tb.dropIndicator)
	}
	buf2 := makeTableSurface(100, 200)
	tb.Draw(newP(buf2, 100), theme)
	if got := pixelAt(buf2, 100, 50, indicatorY); got == theme.Accent {
		t.Fatalf("indicator pixel still painted after DragLeave: %+v", got)
	}
}

// TestTableDragMoveNoopWhenNotReorderable covers EventDragMove's guard:
// with Reorderable false, dropIndicator must stay untouched (-1's
// zero-value equivalent is never reached because Draw never consults it
// either -- but the field itself must not be mutated).
func TestTableDragMoveNoopWhenNotReorderable(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	tb.OnEvent(Event{Kind: EventDragMove, X: 10, Y: tableBodyClickY(0)})
	if tb.dropIndicator != -1 {
		t.Fatalf("dropIndicator mutated by DragMove with Reorderable=false: %d", tb.dropIndicator)
	}
}

// TestTableReorderDropMovesRowDownSelectionFollowsAndFiresOnReorder
// covers the "down" direction: dragging row 0 to insertion index 3
// among 5 rows lands it at resting index 2 (removing it first shifts
// the later rows left by one), Selected/selectedRows follow it there,
// an untouched row's selection membership is preserved at its shifted
// index, and OnReorder fires with (from, actual resting index).
func TestTableReorderDropMovesRowDownSelectionFollowsAndFiresOnReorder(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}, {"r3"}, {"r4"}})
	tb.Reorderable = true
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 5 * TableRowHeight + TableHeaderHeight})

	// Press + pre-select row 0 (anchor), row 2 (falls INSIDE the
	// insertion window (from, final] -- it must shift left by one to
	// make room), and row 4 (stays past the window, unaffected).
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(0)})
	tb.SetRowSelection(0, 2, 4)
	if got := tb.DragData(); got != "tablerow:0" {
		t.Fatalf("DragData() = %q, want %q", got, "tablerow:0")
	}

	var gotFrom, gotTo int
	calls := 0
	tb.OnReorder = func(from, to int) { gotFrom, gotTo = from, to; calls++ }

	tb.OnEvent(Event{Kind: EventDragMove, Y: tableInsertY(0, 3)})
	tb.OnEvent(Event{Kind: EventDrop, Y: tableInsertY(0, 3), Code: tb.DragData()})

	want := [][]string{{"r1"}, {"r2"}, {"r0"}, {"r3"}, {"r4"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows after down-move = %v, want %v", tb.Rows, want)
	}
	if calls != 1 {
		t.Fatalf("OnReorder calls = %d, want 1", calls)
	}
	if gotFrom != 0 || gotTo != 2 {
		t.Fatalf("OnReorder = (%d,%d), want (0,2)", gotFrom, gotTo)
	}
	if tb.Selected != 2 {
		t.Fatalf("Selected after move = %d, want 2 (followed the moved row)", tb.Selected)
	}
	got := tb.SelectedRows()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 4 {
		t.Fatalf("SelectedRows after move = %v, want [1 2 4] (row 2 shifted to 1, moved row at 2, row 4 unaffected)", got)
	}
	if tb.dropIndicator != -1 {
		t.Fatalf("dropIndicator after drop = %d, want -1", tb.dropIndicator)
	}
}

// TestTableReorderDropMovesRowUpAndShiftsIntermediateSelection covers
// the "up" direction: dragging the LAST row to insertion index 1 lands
// it at resting index 1 exactly (to <= from, so no -1 adjustment), and
// an intermediate row's selection shifts +1 to make room.
func TestTableReorderDropMovesRowUpAndShiftsIntermediateSelection(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}, {"r3"}, {"r4"}})
	tb.Reorderable = true
	tb.Selected = 2 // an intermediate row, in the shifted range [to, from)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 5 * TableRowHeight + TableHeaderHeight})

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(4)})
	if got := tb.DragData(); got != "tablerow:4" {
		t.Fatalf("DragData() = %q, want %q", got, "tablerow:4")
	}

	var gotFrom, gotTo int
	tb.OnReorder = func(from, to int) { gotFrom, gotTo = from, to }
	tb.OnEvent(Event{Kind: EventDrop, Y: tableInsertY(0, 1), Code: "tablerow:4"})

	want := [][]string{{"r0"}, {"r4"}, {"r1"}, {"r2"}, {"r3"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows after up-move = %v, want %v", tb.Rows, want)
	}
	if gotFrom != 4 || gotTo != 1 {
		t.Fatalf("OnReorder = (%d,%d), want (4,1)", gotFrom, gotTo)
	}
	if tb.Selected != 3 {
		t.Fatalf("Selected after up-move = %d, want 3 (shifted +1 out of the way)", tb.Selected)
	}
}

// TestTableReorderDropForeignOrGarbagePayloadIgnored covers EventDrop
// with a payload that fails parseTableRowDragPayload: a foreign scheme,
// and a well-formed scheme whose index is out of range for the current
// Rows -- neither must mutate Rows or fire OnReorder.
func TestTableReorderDropForeignOrGarbagePayloadIgnored(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	called := false
	tb.OnReorder = func(from, to int) { called = true }

	for _, payload := range []string{"/tmp/foo.txt", "garbage", "tablerow:abc", "tablerow:99"} {
		tb.OnEvent(Event{Kind: EventDrop, Y: tableBodyClickY(1), Code: payload})
	}
	if called {
		t.Fatal("OnReorder fired for a foreign/garbage/out-of-range payload")
	}
	want := [][]string{{"r0"}, {"r1"}, {"r2"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows mutated by an ignored drop: %v", tb.Rows)
	}
}

// TestTableReorderDropWhileScrolledTargetsAbsoluteRow covers the
// "reorder while scrolled" requirement: rowInsertIndexAt must resolve a
// drop's local Y through ScrollRow, targeting the ABSOLUTE row painted
// there rather than its on-screen offset.
func TestTableReorderDropWhileScrolledTargetsAbsoluteRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5 visible rows
	tb.ScrollRow = 10

	// Press absolute row 12 (screen position 2) to arm the drag.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: tableBodyClickY(2)})
	if got := tb.DragData(); got != "tablerow:12" {
		t.Fatalf("DragData() while scrolled = %q, want %q", got, "tablerow:12")
	}

	var gotFrom, gotTo int
	tb.OnReorder = func(from, to int) { gotFrom, gotTo = from, to }
	// Drop at screen position 4 (absolute insertion index 14).
	tb.OnEvent(Event{Kind: EventDrop, Y: tableInsertY(10, 14), Code: "tablerow:12"})

	if gotFrom != 12 {
		t.Fatalf("OnReorder from = %d, want 12", gotFrom)
	}
	// to>from -> resting index is to-1 == 13.
	if gotTo != 13 {
		t.Fatalf("OnReorder to = %d, want 13", gotTo)
	}
	if tb.Rows[13][0] != "r12" {
		t.Fatalf("Rows[13] = %v, want row r12 to have landed there", tb.Rows[13])
	}
}

// TestTableDragMoveAboveHeaderClampsToScroll covers rowInsertIndexAt's
// localY < TableHeaderHeight branch: hovering over the fixed header
// resolves to the top of the currently-visible window (ScrollRow),
// never a negative or otherwise out-of-range index.
func TestTableDragMoveAboveHeaderClampsToScroll(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})

	tb.OnEvent(Event{Kind: EventDragMove, X: 10, Y: 5}) // inside the header band
	if tb.dropIndicator != 0 {
		t.Fatalf("dropIndicator for a header-band DragMove = %d, want 0 (== ScrollRow)", tb.dropIndicator)
	}
}

// TestTableDragMovePastLastRowClampsToRowCount covers rowInsertIndexAt's
// idx > len(Rows) clamp: a drag far below the last row must resolve to
// exactly len(Rows) ("insert after the last row"), not an
// out-of-bounds index.
func TestTableDragMovePastLastRowClampsToRowCount(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.Reorderable = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 2000})

	tb.OnEvent(Event{Kind: EventDragMove, X: 10, Y: TableHeaderHeight + 100*TableRowHeight})
	if tb.dropIndicator != len(tb.Rows) {
		t.Fatalf("dropIndicator for a far-below-last-row DragMove = %d, want %d", tb.dropIndicator, len(tb.Rows))
	}
}

// TestRemapRowIndexIdentityMoveLeavesOtherRowsUnchanged covers
// remapRowIndex's final fallthrough (from == final, i != from): dropping
// a row back onto its own slot must not perturb any OTHER row's
// selection membership.
func TestRemapRowIndexIdentityMoveLeavesOtherRowsUnchanged(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.SetRowSelection(1, 2)
	tb.reorderRow(1, 1) // to <= from -> final == from == 1, a no-op move
	got := tb.SelectedRows()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("SelectedRows after identity move = %v, want [1 2] (unchanged)", got)
	}
	want := [][]string{{"r0"}, {"r1"}, {"r2"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows after identity move = %v, want unchanged %v", tb.Rows, want)
	}
}

// --- reorderRow: direct-call defensive clamps ---------------------------

// TestReorderRowOutOfRangeFromIsNoop covers the from<0||from>=n guard --
// an out-of-range from (reachable only via a direct call; EventDrop's
// parseTableRowDragPayload already rejects a negative index, and the
// out-of-range-positive case is covered end-to-end by
// TestTableReorderDropForeignOrGarbagePayloadIgnored) must leave Rows +
// OnReorder untouched.
func TestReorderRowOutOfRangeFromIsNoop(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"r0"}, {"r1"}})
	called := false
	tb.OnReorder = func(from, to int) { called = true }
	tb.reorderRow(-1, 0)
	tb.reorderRow(2, 0)
	if called {
		t.Fatal("OnReorder fired for an out-of-range from")
	}
	want := [][]string{{"r0"}, {"r1"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows mutated by an out-of-range from: %v", tb.Rows)
	}
}

// TestReorderRowClampsToArgument covers the to<0 / to>n defensive clamp
// documented on reorderRow -- reachable only via a direct call, since
// the only production caller (EventDrop) always passes an
// already-clamped rowInsertIndexAt result. Mirrors how other Table
// methods (e.g. SetColumnWidth) clamp a raw caller-supplied value.
func TestReorderRowClampsToArgument(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	// to < 0 clamps to 0: from=2 moved to the very front.
	tb.reorderRow(2, -5)
	want := [][]string{{"r2"}, {"r0"}, {"r1"}}
	if fmt.Sprintf("%v", tb.Rows) != fmt.Sprintf("%v", want) {
		t.Fatalf("Rows after to<0 clamp = %v, want %v", tb.Rows, want)
	}

	tb2 := NewTable([]TableColumn{{Title: "A"}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	// to > n clamps to n (3): from=0 moved to the very back.
	tb2.reorderRow(0, 999)
	want2 := [][]string{{"r1"}, {"r2"}, {"r0"}}
	if fmt.Sprintf("%v", tb2.Rows) != fmt.Sprintf("%v", want2) {
		t.Fatalf("Rows after to>n clamp = %v, want %v", tb2.Rows, want2)
	}
}

func TestTableScrollToSelectedScrollsUpAndDown(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 134}) // 5 visible rows

	// Selected below the window -> scrolls DOWN just enough that
	// Selected lands on the last visible row.
	tb.ScrollRow = 0
	tb.Selected = 8
	tb.scrollToSelected()
	if tb.ScrollRow != 4 { // 8 - 5 + 1
		t.Fatalf("ScrollRow after downward scrollToSelected = %d, want 4", tb.ScrollRow)
	}

	// Selected above the window -> scrolls UP to put Selected at top.
	tb.ScrollRow = 10
	tb.Selected = 3
	tb.scrollToSelected()
	if tb.ScrollRow != 3 {
		t.Fatalf("ScrollRow after upward scrollToSelected = %d, want 3", tb.ScrollRow)
	}

	// Selected already inside the window -> no change.
	tb.ScrollRow = 2
	tb.Selected = 4 // window is [2,7)
	tb.scrollToSelected()
	if tb.ScrollRow != 2 {
		t.Fatalf("ScrollRow after in-window scrollToSelected = %d, want unchanged 2", tb.ScrollRow)
	}
}
