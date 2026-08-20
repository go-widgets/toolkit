// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// sampleIconGrid builds a 4-cell grid: a flat icon (raster false, with an
// image), a raster thumbnail with NO image (so its light chip is visible), a
// long-labelled cell to force elision, and a plain cell.
func sampleIconGrid() *IconGrid {
	return NewIconGrid(
		IconCell{Label: "One", Key: "k0", Image: solidIcon(RGB(0x40, 0x10, 0x10)), Raster: false},
		IconCell{Label: "Two", Key: "k1", Raster: true},
		IconCell{Label: "A cell label far too wide to fit its cell", Key: "k2"},
		IconCell{Label: "Four", Key: "k3"},
	)
}

func TestIconGridGeometry(t *testing.T) {
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	if g.cellW() != 76 || g.cellH() != 92 {
		t.Fatalf("cell size = %dx%d, want 76x92", g.cellW(), g.cellH())
	}
	if g.cols() != 3 {
		t.Fatalf("cols = %d, want 3", g.cols())
	}
	// A width narrower than a single cell still yields one column.
	g.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 200})
	if g.cols() != 1 {
		t.Fatalf("narrow cols = %d, want 1", g.cols())
	}
}

func TestIconGridDrawChrome(t *testing.T) {
	g := sampleIconGrid()
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	w, h := 240, 200
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), th)

	// Body above the first icon is the Surface fill.
	if got := pixelAt(buf, w, 30, 5); got != th.Surface {
		t.Fatalf("body fill = %+v, want Surface %+v", got, th.Surface)
	}
	// Cell 0 carries a real image: its icon centre is the image colour.
	imgC := RGB(0x40, 0x10, 0x10)
	if got := pixelAt(buf, w, 44, 40); got != imgC {
		t.Fatalf("cell0 icon = %+v, want image %+v", got, imgC)
	}
	// Cell 1 is a raster cell with no image: its icon area shows the light chip,
	// which on a dark theme is the near-white backing (so a dark photo reads).
	if got := pixelAt(buf, w, 120, 40); got != chipColor(th) {
		t.Fatalf("cell1 chip = %+v, want %+v", got, chipColor(th))
	}
}

func TestIconGridChipIsWhiteOnLightTheme(t *testing.T) {
	g := sampleIconGrid()
	th := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	w, h := 240, 200
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), th)
	if got, want := pixelAt(buf, w, 120, 40), RGB(0xFF, 0xFF, 0xFF); got != want {
		t.Fatalf("light-theme chip = %+v, want white %+v", got, want)
	}
}

func TestIconGridSelectionHighlight(t *testing.T) {
	g := sampleIconGrid()
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	g.SetSelected(0)
	w, h := 240, 200
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), th)

	// The selected cell paints an opaque Accent highlight behind its label. The
	// label band starts at cellY+igPadTop+IconSize+igIconLabel = 72; sample the
	// 1px above the glyph rows, at the label's horizontal centre.
	tw := TextWidth("One")
	lx := 6 + (76-tw)/2 // cell0 X0 = 6
	if got := pixelAt(buf, w, lx+tw/2, 71); got != th.Accent {
		t.Fatalf("selected label highlight = %+v, want Accent %+v", got, th.Accent)
	}
}

func TestIconGridEmptyState(t *testing.T) {
	th := DefaultLight()
	w, h := 200, 120

	// Default message.
	g := NewIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), th)
	def := TextWidth("No items")
	dx := (w - def) / 2
	dy := h/2 - GlyphHeight()/2
	if !anyInkAround(buf, w, dx, dy, def, mutedInk(th)) {
		t.Fatalf("default empty message not painted")
	}

	// Custom message.
	g2 := NewIconGrid()
	g2.Empty = "Nothing here"
	g2.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf2 := makeSurface(w, h)
	g2.Draw(newP(buf2, w), th)
	cw := TextWidth("Nothing here")
	cx := (w - cw) / 2
	if !anyInkAround(buf2, w, cx, dy, cw, mutedInk(th)) {
		t.Fatalf("custom empty message not painted")
	}
}

// anyInkAround reports whether colour c appears on the text row [x0, x0+span) at
// any of the glyph rows starting at y0 — used to confirm a centred label painted.
func anyInkAround(buf []byte, w, x0, y0, span int, c RGBA) bool {
	for y := y0; y < y0+GlyphHeight(); y++ {
		for x := x0; x < x0+span; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

func TestIconGridClickSelectActivateDeselect(t *testing.T) {
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	var selected, activated = -9, -9
	g.Selected().Subscribe(func(i int) { selected = i })
	g.OnActivate = func(i int) { activated = i }

	// Click cell 0 (x0=6..82, first row): selects it.
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40})
	if g.Selected().Get() != 0 || selected != 0 {
		t.Fatalf("after click sel=%d cb=%d, want 0/0", g.Selected().Get(), selected)
	}
	// Click it again: activates (no re-select callback).
	selected = -9
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40})
	if activated != 0 || selected != -9 {
		t.Fatalf("re-click activated=%d selected=%d, want 0/-9", activated, selected)
	}
	// Click empty space to the right of the grid: deselects.
	g.OnEvent(Event{Kind: EventClick, X: 238, Y: 40})
	if g.Selected().Get() != -1 {
		t.Fatalf("click past grid: sel=%d, want -1", g.Selected().Get())
	}
}

func TestIconGridClickNilCallbacks(t *testing.T) {
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40}) // select, OnSelect nil
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40}) // activate, OnActivate nil
	if g.Selected().Get() != 0 {
		t.Fatalf("sel=%d, want 0", g.Selected().Get())
	}
}

func TestIconGridDisabledInert(t *testing.T) {
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	g.Disabled().Set(true)
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40})
	if g.Selected().Get() != -1 {
		t.Fatalf("disabled grid selected %d, want -1", g.Selected().Get())
	}
}

func TestIconGridIndexAtEdges(t *testing.T) {
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	if idx := g.IndexAt(2, 10); idx != -1 { // left of the centred grid (x0=6)
		t.Fatalf("IndexAt left gap = %d, want -1", idx)
	}
	if idx := g.IndexAt(120, 5000); idx != -1 { // below every cell
		t.Fatalf("IndexAt far below = %d, want -1", idx)
	}
	// Narrow grid (W < cell) forces the x0<0 clamp path.
	g.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 200})
	if idx := g.IndexAt(10, 10); idx != 0 {
		t.Fatalf("narrow IndexAt = %d, want 0", idx)
	}
}

func TestIconGridScrollAndClamp(t *testing.T) {
	// 10 cells at 3 cols = 4 rows; content 4*92=368 > 200 viewport.
	cells := make([]IconCell, 10)
	for i := range cells {
		cells[i] = IconCell{Label: fmt.Sprintf("c%d", i), Key: fmt.Sprintf("k%d", i)}
	}
	g := NewIconGrid(cells...)
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})

	// A large wheel delta clamps to the maximum scroll (368-200 = 168).
	g.OnEvent(Event{Kind: EventScroll, Delta: 10})
	if g.scroll != 168 {
		t.Fatalf("clamped scroll = %d, want 168", g.scroll)
	}
	// Drawing while scrolled exercises the "cell above the viewport" skip and,
	// with row 3 pushed below, the trailing break.
	buf := makeSurface(240, 200)
	g.Draw(newP(buf, 240), DefaultLight())

	// clampScroll branch coverage: below 0 -> 0, above max -> max.
	if got := g.clampScroll(-5); got != 0 {
		t.Fatalf("clampScroll(-5) = %d, want 0", got)
	}
	if got := g.clampScroll(9999); got != 168 {
		t.Fatalf("clampScroll(9999) = %d, want 168", got)
	}
	if got := g.clampScroll(50); got != 50 {
		t.Fatalf("clampScroll(50) = %d, want 50", got)
	}
}

func TestIconGridDrawBreakAtBottom(t *testing.T) {
	cells := make([]IconCell, 10) // 4 rows; row 3 top=276 >= H(200) -> break
	for i := range cells {
		cells[i] = IconCell{Label: fmt.Sprintf("c%d", i)}
	}
	g := NewIconGrid(cells...)
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	g.Draw(newP(makeSurface(240, 200), 240), DefaultLight()) // must not panic
}

func TestIconGridDrawNarrowClampsGridLeft(t *testing.T) {
	// Width narrower than one cell: gridW (76) > W (50), so the centring offset
	// goes negative and is clamped to the left edge (x0 < b.X path).
	g := sampleIconGrid()
	g.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 200})
	g.Draw(newP(makeSurface(50, 200), 50), DefaultLight()) // must not panic
}

func TestIconGridClampScrollShortContent(t *testing.T) {
	// Content shorter than the viewport: max scroll floors at 0 (max<0 path).
	g := sampleIconGrid() // 4 cells, 2 rows -> content 184
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 400})
	if got := g.clampScroll(5); got != 0 {
		t.Fatalf("clampScroll on short content = %d, want 0", got)
	}
}

func TestIconGridSetIconSizeClamp(t *testing.T) {
	g := sampleIconGrid()
	g.scroll = 40
	g.SetIconSize(10) // below the floor
	if g.IconSize != 24 {
		t.Fatalf("clamped IconSize = %d, want 24", g.IconSize)
	}
	if g.scroll != 0 {
		t.Fatalf("SetIconSize should reset scroll, got %d", g.scroll)
	}
	g.SetIconSize(64)
	if g.IconSize != 64 {
		t.Fatalf("IconSize = %d, want 64", g.IconSize)
	}
}

func TestIconGridSetSelectedValidation(t *testing.T) {
	g := sampleIconGrid()
	g.SetSelected(2)
	if g.Selected().Get() != 2 {
		t.Fatalf("valid SetSelected = %d, want 2", g.Selected().Get())
	}
	g.SetSelected(99)
	if g.Selected().Get() != -1 {
		t.Fatalf("invalid SetSelected = %d, want -1", g.Selected().Get())
	}
}

func TestIconGridDragData(t *testing.T) {
	g := sampleIconGrid()
	if got := g.DragData(); got != "" {
		t.Fatalf("DragData with no selection = %q, want empty", got)
	}
	g.SetSelected(1)
	if got := g.DragData(); got != "k1" {
		t.Fatalf("DragData = %q, want k1", got)
	}
}

func TestIconGridA11y(t *testing.T) {
	g := sampleIconGrid()
	if got := g.A11y(); got.Role != RoleGrid || got.Value != "" {
		t.Fatalf("unselected A11y = %+v, want grid with empty value", got)
	}
	g.SetSelected(0)
	if got := g.A11y(); got.Role != RoleGrid || got.Value != "One" {
		t.Fatalf("selected A11y = %+v, want grid/One", got)
	}
}

func TestWithAlpha(t *testing.T) {
	c := withAlpha(RGB(0x10, 0x20, 0x30), 0x33)
	if c.A != 0x33 || c.R != 0x10 || c.G != 0x20 || c.B != 0x30 {
		t.Fatalf("withAlpha = %+v", c)
	}
}

// ExampleIconGrid builds a small thumbnail grid, selects a cell and reports it.
func ExampleIconGrid() {
	g := NewIconGrid(
		IconCell{Label: "Report.pdf", Key: "report"},
		IconCell{Label: "Photo.jpg", Key: "photo", Raster: true},
	)
	g.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 160})
	g.Draw(newP(makeSurface(200, 160), 200), DefaultLight())
	g.OnEvent(Event{Kind: EventClick, X: 40, Y: 40})
	fmt.Printf("selected cell %d\n", g.Selected().Get())
	// Output: selected cell 0
}
