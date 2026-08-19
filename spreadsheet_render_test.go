// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// scanColor reports whether any pixel in [x0,x1) x [y0,y1) equals c.
func scanColor(buf []byte, w, x0, y0, x1, y1 int, c RGBA) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

// inkExtentX returns the min and max x of any pixel equal to c inside the given
// rectangle, and whether any such pixel was found.
func inkExtentX(buf []byte, w, x0, y0, x1, y1 int, c RGBA) (minX, maxX int, found bool) {
	minX, maxX = x1, x0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if pixelAt(buf, w, x, y) == c {
				found = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	return minX, maxX, found
}

// TestSpreadsheetDrawZeroBoundsNoOp covers Draw's early return on an empty box.
func TestSpreadsheetDrawZeroBoundsNoOp(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	buf := makeSurface(10, 10)
	s.Draw(newP(buf, 10), DefaultLight())
	// The sentinel fill must be untouched.
	if pixelAt(buf, 10, 5, 5) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Error("zero-bounds Draw painted something")
	}
}

// TestSpreadsheetDrawBandsAndSelection asserts the exact positions of the
// corner, the active/inactive header cells, and the active-cell selection box.
func TestSpreadsheetDrawBandsAndSelection(t *testing.T) {
	th := DefaultLight()
	s := NewSpreadsheet(3, 3) // active cell A1
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	buf := makeSurface(300, 200)
	s.Draw(newP(buf, 300), th)

	// Corner box (rowHeader x header) is SurfaceAlt.
	if got := pixelAt(buf, 300, 18, 10); got != th.SurfaceAlt {
		t.Errorf("corner = %+v, want SurfaceAlt", got)
	}
	// Column A header heads the active column -> Accent face.
	if got := pixelAt(buf, 300, 40, 4); got != th.Accent {
		t.Errorf("col-A header = %+v, want Accent", got)
	}
	// Column B header is inactive -> SurfaceAlt face.
	if got := pixelAt(buf, 300, 110, 4); got != th.SurfaceAlt {
		t.Errorf("col-B header = %+v, want SurfaceAlt", got)
	}
	// Row 1 header heads the active row -> Accent.
	if got := pixelAt(buf, 300, 4, 28); got != th.Accent {
		t.Errorf("row-1 header = %+v, want Accent", got)
	}
	// Row 2 header is inactive -> SurfaceAlt.
	if got := pixelAt(buf, 300, 4, 48); got != th.SurfaceAlt {
		t.Errorf("row-2 header = %+v, want SurfaceAlt", got)
	}
	// Active-cell selection box: A1 spans x[36,100) y[20,40); its top edge is at
	// y=20 and its left edge at x=36, both in Accent.
	if got := pixelAt(buf, 300, 50, 20); got != th.Accent {
		t.Errorf("selection top edge = %+v, want Accent", got)
	}
	if got := pixelAt(buf, 300, 36, 30); got != th.Accent {
		t.Errorf("selection left edge = %+v, want Accent", got)
	}
	// A1 interior (blank) is the Surface fill.
	if got := pixelAt(buf, 300, 60, 30); got != th.Surface {
		t.Errorf("A1 interior = %+v, want Surface", got)
	}
	// B1's right grid line lands at x=163 in Border.
	if got := pixelAt(buf, 300, 163, 30); got != th.Border {
		t.Errorf("B1 right grid line = %+v, want Border", got)
	}
}

// TestSpreadsheetCellValueAlignment asserts numbers align right, text aligns
// left, and error values ink in the error colour, at exact cell positions.
func TestSpreadsheetCellValueAlignment(t *testing.T) {
	th := DefaultLight()
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.SetCell(2, 0, "7")    // C1 number
	s.SetCell(2, 1, "x")    // C2 text
	s.SetCell(2, 2, "=1/0") // C3 error
	buf := makeSurface(300, 200)
	s.Draw(newP(buf, 300), th)

	// C1 spans x[164,228) y[20,40); centre x = 196. A right-aligned number's ink
	// sits entirely to the right of centre.
	if minX, _, ok := inkExtentX(buf, 300, 165, 21, 227, 39, th.OnSurface); !ok || minX <= 196 {
		t.Errorf("number ink minX = %d (found=%v), want > 196 (right-aligned)", minX, ok)
	}
	// C2 spans y[40,60); a left-aligned text's ink sits entirely left of centre.
	if _, maxX, ok := inkExtentX(buf, 300, 165, 41, 227, 59, th.OnSurface); !ok || maxX >= 196 {
		t.Errorf("text ink maxX = %d (found=%v), want < 196 (left-aligned)", maxX, ok)
	}
	// C3 spans y[60,80); the #DIV/0! error inks in the error colour.
	if !scanColor(buf, 300, 165, 61, 227, 79, spreadsheetErrorInk) {
		t.Error("error cell not inked in the error colour")
	}
}

// TestSpreadsheetEditorOverlay covers Draw's editor-overlay branch and the
// "show formula while editing, computed value otherwise" contract.
func TestSpreadsheetEditorOverlay(t *testing.T) {
	th := DefaultLight()
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.SetCell(0, 0, "=1+2")
	s.BeginEdit()
	// The editor shows the raw formula while the cell still computes its value.
	if s.editor.Text().Get() != "=1+2" {
		t.Errorf("editor text = %q, want =1+2", s.editor.Text().Get())
	}
	if got := s.CellDisplay(0, 0); got != "3" {
		t.Errorf("cell value during edit = %q, want 3", got)
	}
	buf := makeSurface(300, 200)
	s.Draw(newP(buf, 300), th)
	// The editor is positioned exactly over cell A1.
	if got := s.editor.Bounds(); got != (Rect{X: 36, Y: 20, W: 64, H: 20}) {
		t.Errorf("editor bounds = %+v, want A1 rect", got)
	}
}

// TestSpreadsheetStaysWithinBounds is a per-widget containment sweep (the
// Spreadsheet is not in the shared bounds sweep to avoid editing that file): a
// scrolling sheet drawn into a padded surface must not paint outside its box.
func TestSpreadsheetStaysWithinBounds(t *testing.T) {
	r := Rect{X: 30, Y: 30, W: 200, H: 140}
	const w, h = 280, 220
	s := NewSpreadsheet(20, 30) // both axes overflow -> scrollbars painted
	s.SetBounds(r)
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), DefaultLight())
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			inside := x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
			if !inside && pixelAt(buf, w, x, y) != sentinel {
				t.Fatalf("painted outside bounds at (%d,%d)", x, y)
			}
		}
	}
}

// --- Scrollbar geometry + interaction -----------------------------------------

func TestSpreadsheetScrollGeometry(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if !s.vOverflow() || !s.hOverflow() {
		t.Fatal("fixture must overflow on both axes")
	}
	gv, ok := s.vscrollGeom()
	if !ok {
		t.Fatal("vscrollGeom must be live")
	}
	if gv.cross0 != 288 || gv.trackStart != 20 || gv.trackLen != 168 || gv.maxScroll != 22 || gv.thumbLen != 47 || gv.thumbStart != 20 {
		t.Errorf("vscrollGeom = %+v, want cross0=288 trackStart=20 trackLen=168 maxScroll=22 thumbLen=47 thumbStart=20", gv)
	}
	gh, ok := s.hscrollGeom()
	if !ok {
		t.Fatal("hscrollGeom must be live")
	}
	if gh.cross0 != 188 || gh.trackStart != 36 || gh.trackLen != 252 || gh.maxScroll != 17 || gh.thumbLen != 49 || gh.thumbStart != 36 {
		t.Errorf("hscrollGeom = %+v, want cross0=188 trackStart=36 trackLen=252 maxScroll=17 thumbLen=49 thumbStart=36", gh)
	}
}

func TestSpreadsheetScrollGeometryNotLive(t *testing.T) {
	s := NewSpreadsheet(3, 3) // fits, no overflow
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if _, ok := s.vscrollGeom(); ok {
		t.Error("vscrollGeom must not be live on a fitting sheet")
	}
	if _, ok := s.hscrollGeom(); ok {
		t.Error("hscrollGeom must not be live on a fitting sheet")
	}
}

func TestSpreadsheetThumbClampsToMinimum(t *testing.T) {
	tall := NewSpreadsheet(3, 1000)
	tall.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if g, _ := tall.vscrollGeom(); g.thumbLen != spreadsheetThumbMin {
		t.Errorf("tall sheet vthumb = %d, want %d", g.thumbLen, spreadsheetThumbMin)
	}
	wide := NewSpreadsheet(1000, 3)
	wide.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if g, _ := wide.hscrollGeom(); g.thumbLen != spreadsheetThumbMin {
		t.Errorf("wide sheet hthumb = %d, want %d", g.thumbLen, spreadsheetThumbMin)
	}
	// Drawing them exercises drawVScroll / drawHScroll.
	tbuf := makeSurface(300, 200)
	tall.Draw(newP(tbuf, 300), DefaultLight())
	wbuf := makeSurface(300, 200)
	wide.Draw(newP(wbuf, 300), DefaultLight())
}

func TestSpreadsheetWheelScrolls(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if _, row := s.ScrollOffset(); row != 5 {
		t.Errorf("scrollRow after wheel = %d, want 5", row)
	}
}

func TestSpreadsheetScrollByClamps(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.ScrollBy(0, 100) // past bottom
	if _, row := s.ScrollOffset(); row != 22 {
		t.Errorf("scrollRow = %d, want 22 (max)", row)
	}
	s.ScrollBy(0, -100) // past top
	if _, row := s.ScrollOffset(); row != 0 {
		t.Errorf("scrollRow = %d, want 0", row)
	}
	s.ScrollBy(100, 0) // past right
	if col, _ := s.ScrollOffset(); col != 17 {
		t.Errorf("scrollCol = %d, want 17 (max)", col)
	}
	s.ScrollBy(-100, 0) // past left
	if col, _ := s.ScrollOffset(); col != 0 {
		t.Errorf("scrollCol = %d, want 0", col)
	}
}

func TestSpreadsheetVScrollbarThumbDrag(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Press on the vertical thumb (track x[288,300), thumb y[20,67)).
	s.OnEvent(Event{Kind: EventClick, X: 294, Y: 40})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Error("scrollbar press must not select a cell")
	}
	// Drag to the bottom -> clamps to maxScroll.
	s.OnEvent(Event{Kind: EventMouseDrag, X: 294, Y: 1000})
	if _, row := s.ScrollOffset(); row != 22 {
		t.Errorf("scrollRow after drag to bottom = %d, want 22", row)
	}
	// Drag back to the top.
	s.OnEvent(Event{Kind: EventMouseDrag, X: 294, Y: 0})
	if _, row := s.ScrollOffset(); row != 0 {
		t.Errorf("scrollRow after drag to top = %d, want 0", row)
	}
	// Release, after which a drag is inert.
	s.OnEvent(Event{Kind: EventMouseUp, X: 294, Y: 0})
	s.OnEvent(Event{Kind: EventMouseDrag, X: 294, Y: 1000})
	if _, row := s.ScrollOffset(); row != 0 {
		t.Errorf("scrollRow after release+drag = %d, want 0 (inert)", row)
	}
}

func TestSpreadsheetVScrollbarTrackPaging(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Press the track below the thumb -> pages down by fullRows (8).
	s.OnEvent(Event{Kind: EventClick, X: 294, Y: 150})
	if _, row := s.ScrollOffset(); row != 8 {
		t.Fatalf("scrollRow after page-down = %d, want 8", row)
	}
	// Jump to the bottom, then press the track above the thumb -> pages up.
	s.ScrollBy(0, 100) // row 22, thumb near the bottom
	s.OnEvent(Event{Kind: EventMouseUp})
	s.OnEvent(Event{Kind: EventClick, X: 294, Y: 25})
	if _, row := s.ScrollOffset(); row != 14 {
		t.Errorf("scrollRow after page-up = %d, want 14", row)
	}
}

func TestSpreadsheetHScrollbarDragAndPage(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Horizontal thumb: track y[188,200), thumb x[36,85). Press then drag right.
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 194})
	s.OnEvent(Event{Kind: EventMouseDrag, X: 1000, Y: 194})
	if col, _ := s.ScrollOffset(); col != 17 {
		t.Errorf("scrollCol after drag right = %d, want 17", col)
	}
	s.OnEvent(Event{Kind: EventMouseUp})
	// Fresh sheet: press the h-track right of the thumb -> pages right by fullCols.
	s2 := NewSpreadsheet(20, 30)
	s2.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s2.OnEvent(Event{Kind: EventClick, X: 200, Y: 194})
	if col, _ := s2.ScrollOffset(); col != 3 {
		t.Errorf("scrollCol after page-right = %d, want 3 (fullCols)", col)
	}
}

func TestSpreadsheetDragWithoutGrabIsInert(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// A drag with no prior thumb press does nothing.
	s.OnEvent(Event{Kind: EventMouseDrag, X: 294, Y: 100})
	if col, row := s.ScrollOffset(); col != 0 || row != 0 {
		t.Errorf("ScrollOffset after ungrabbed drag = (%d,%d), want (0,0)", col, row)
	}
}

// TestSpreadsheetEnsureVisibleScrolls covers all four ensureVisible arms via
// arrow navigation that pushes the active cell past each viewport edge.
func TestSpreadsheetEnsureVisibleScrolls(t *testing.T) {
	s := NewSpreadsheet(20, 30)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200}) // fullCols=3, fullRows=8
	for i := 0; i < 5; i++ {
		s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	}
	if col, _ := s.ScrollOffset(); col != 3 {
		t.Fatalf("scrollCol after 5x right = %d, want 3", col)
	}
	for i := 0; i < 5; i++ {
		s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	}
	if col, _ := s.ScrollOffset(); col != 0 {
		t.Fatalf("scrollCol after 5x left = %d, want 0", col)
	}
	for i := 0; i < 10; i++ {
		s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if _, row := s.ScrollOffset(); row != 3 {
		t.Fatalf("scrollRow after 10x down = %d, want 3", row)
	}
	for i := 0; i < 10; i++ {
		s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if _, row := s.ScrollOffset(); row != 0 {
		t.Fatalf("scrollRow after 10x up = %d, want 0", row)
	}
}

// TestSpreadsheetTinyBoundsNoPanic covers the degenerate-viewport arms
// (gridRect width/height <= 0) in the geometry + draw paths.
func TestSpreadsheetTinyBoundsNoPanic(t *testing.T) {
	s := NewSpreadsheet(5, 5)
	s.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 10}) // smaller than the header bands
	buf := makeSurface(30, 10)
	s.Draw(newP(buf, 30), DefaultLight())
	// With a non-positive viewport fullCols/fullRows are 0, so a scroll is not
	// bounded below the cell count; the point is it neither panics nor draws
	// out of bounds.
	s.ScrollBy(1, 1)
	if col, row := s.ScrollOffset(); col != 1 || row != 1 {
		t.Errorf("ScrollOffset on tiny sheet = (%d,%d), want (1,1)", col, row)
	}
}
