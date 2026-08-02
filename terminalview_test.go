// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

func TestNewTerminalView(t *testing.T) {
	v := NewTerminalView(4, 3)
	if v.Cols != 4 || v.Rows != 3 {
		t.Fatalf("dims = %dx%d, want 4x3", v.Cols, v.Rows)
	}
	if len(v.Cells) != 12 {
		t.Fatalf("len(Cells) = %d, want 12", len(v.Cells))
	}
	if v.CursorCol != 0 || v.CursorRow != 0 {
		t.Fatal("cursor not homed at (0,0)")
	}
}

func TestNewTerminalViewPanicsOnNonPositive(t *testing.T) {
	for _, d := range [][2]int{{0, 3}, {3, 0}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewTerminalView(%d,%d) did not panic", d[0], d[1])
				}
			}()
			NewTerminalView(d[0], d[1])
		}()
	}
}

func TestTerminalViewSetGetCell(t *testing.T) {
	v := NewTerminalView(3, 2)
	fg := RGBA{R: 1, G: 2, B: 3, A: 0xFF}
	bg := RGBA{R: 4, G: 5, B: 6, A: 0xFF}
	v.SetCell(1, 1, 'Z', fg, bg)
	got := v.Cell(1, 1)
	if got.Rune != 'Z' || got.FG != fg || got.BG != bg {
		t.Fatalf("Cell(1,1) = %+v", got)
	}
	// Out-of-range writes are no-ops; out-of-range reads return zero.
	v.SetCell(-1, 0, 'x', fg, bg)
	v.SetCell(0, 9, 'x', fg, bg)
	if v.Cell(9, 9) != (TermCell{}) {
		t.Fatal("out-of-range Cell should be zero")
	}
	if v.Cell(-1, -1) != (TermCell{}) {
		t.Fatal("negative Cell should be zero")
	}
}

func TestTerminalViewPut(t *testing.T) {
	v := NewTerminalView(2, 2)
	v.DefaultFG = RGBA{R: 9, A: 0xFF}
	v.DefaultBG = RGBA{B: 9, A: 0xFF}
	v.Put(0, 0, 'q')
	got := v.Cell(0, 0)
	if got.Rune != 'q' || got.FG != v.DefaultFG || got.BG != v.DefaultBG {
		t.Fatalf("Put cell = %+v", got)
	}
}

func TestTerminalViewWrite(t *testing.T) {
	v := NewTerminalView(2, 2)
	v.Write("abc") // a(0,0) b(1,0) wrap c(0,1)
	if v.Cell(0, 0).Rune != 'a' || v.Cell(1, 0).Rune != 'b' || v.Cell(0, 1).Rune != 'c' {
		t.Fatal("Write did not wrap correctly")
	}
	if v.CursorCol != 1 || v.CursorRow != 1 {
		t.Fatalf("cursor = (%d,%d), want (1,1)", v.CursorCol, v.CursorRow)
	}
}

func TestTerminalViewWriteCarriageReturnAndNewline(t *testing.T) {
	v := NewTerminalView(4, 3)
	v.Write("ab\ncd")
	if v.Cell(0, 0).Rune != 'a' || v.Cell(1, 0).Rune != 'b' {
		t.Fatal("row 0 wrong")
	}
	if v.Cell(0, 1).Rune != 'c' || v.Cell(1, 1).Rune != 'd' {
		t.Fatal("newline did not move to row 1")
	}
	// Carriage return returns to column 0 without changing the row.
	v.Write("\rX")
	if v.Cell(0, 1).Rune != 'X' {
		t.Fatal("carriage return did not home the column")
	}
}

func TestTerminalViewWriteScrolls(t *testing.T) {
	v := NewTerminalView(2, 2)
	v.Write("abcd") // fills both rows, the final wrap scrolls
	if v.Cell(0, 0).Rune != 'c' || v.Cell(1, 0).Rune != 'd' {
		t.Fatalf("after scroll row0 = %q%q, want cd",
			v.Cell(0, 0).Rune, v.Cell(1, 0).Rune)
	}
	if v.CursorRow != 1 {
		t.Fatalf("cursor row = %d, want 1", v.CursorRow)
	}
}

func TestTerminalViewScrollUp(t *testing.T) {
	v := NewTerminalView(1, 3)
	v.SetCell(0, 0, 'a', RGBA{A: 0xFF}, RGBA{})
	v.SetCell(0, 1, 'b', RGBA{A: 0xFF}, RGBA{})
	v.SetCell(0, 2, 'c', RGBA{A: 0xFF}, RGBA{})

	v.ScrollUp(0) // no-op
	if v.Cell(0, 0).Rune != 'a' {
		t.Fatal("ScrollUp(0) should be a no-op")
	}
	v.ScrollUp(1)
	if v.Cell(0, 0).Rune != 'b' || v.Cell(0, 1).Rune != 'c' || v.Cell(0, 2).Rune != 0 {
		t.Fatal("ScrollUp(1) did not shift rows up + blank the tail")
	}
	v.ScrollUp(9) // >= Rows clears the grid
	for i, c := range v.Cells {
		if c != (TermCell{}) {
			t.Fatalf("ScrollUp(>=Rows) left cell %d non-empty", i)
		}
	}
}

func TestTerminalViewResizePreservesAndClamps(t *testing.T) {
	v := NewTerminalView(2, 2)
	v.SetCell(0, 0, 'a', RGBA{A: 0xFF}, RGBA{})
	v.SetCell(1, 1, 'd', RGBA{A: 0xFF}, RGBA{})
	v.CursorCol, v.CursorRow = 1, 1

	// Grow: the top-left rectangle is preserved, new cells blank.
	v.Resize(4, 4)
	if v.Cols != 4 || v.Rows != 4 {
		t.Fatalf("grow dims = %dx%d", v.Cols, v.Rows)
	}
	if v.Cell(0, 0).Rune != 'a' || v.Cell(1, 1).Rune != 'd' {
		t.Fatal("grow did not preserve content")
	}
	if v.Cell(3, 3) != (TermCell{}) {
		t.Fatal("grown cells should be blank")
	}

	// Shrink below the cursor: content clipped, cursor clamped in.
	v.CursorCol, v.CursorRow = 3, 3
	v.Resize(2, 2)
	if v.CursorCol != 1 || v.CursorRow != 1 {
		t.Fatalf("cursor not clamped: (%d,%d)", v.CursorCol, v.CursorRow)
	}
	if v.Cell(0, 0).Rune != 'a' {
		t.Fatal("shrink did not preserve the top-left cell")
	}
}

func TestTerminalViewResizePanicsOnNonPositive(t *testing.T) {
	v := NewTerminalView(2, 2)
	defer func() {
		if recover() == nil {
			t.Fatal("Resize(0,0) did not panic")
		}
	}()
	v.Resize(0, 0)
}

func TestTerminalViewCellSize(t *testing.T) {
	v := NewTerminalView(2, 2)
	if v.CellWidth() != 0 || v.CellHeight() != 0 {
		t.Fatal("cell size should be 0 before SetBounds")
	}
	v.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	if v.CellWidth() != GlyphAdvance() || v.CellHeight() != GlyphHeight() {
		t.Fatalf("cell size = %dx%d, want %dx%d",
			v.CellWidth(), v.CellHeight(), GlyphAdvance(), GlyphHeight())
	}
}

func TestTerminalViewDrawBeforeBoundsIsNoOp(t *testing.T) {
	const w, h = 20, 20
	v := NewTerminalView(2, 2)
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), DefaultLight())
	// No cell size yet: the surface keeps its sentinel fill.
	if pixelAt(buf, w, 0, 0) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("Draw before SetBounds should paint nothing")
	}
}

func TestTerminalViewDrawThemeFallbackAndInvert(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	v := NewTerminalView(2, 2)
	fg := RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}
	bg := RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF}
	v.SetCell(0, 0, 'A', fg, bg) // explicit colours
	v.SetCell(0, 1, ' ', fg, bg) // space: bg only, no glyph
	v.CursorVisible = true
	v.CursorCol, v.CursorRow = 1, 1 // invert an empty, unset cell
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	// Explicit cell: top-left pixel is the (unlit) background = blue.
	if pixelAt(buf, w, 0, 0) != bg {
		t.Fatalf("explicit cell bg = %+v, want blue", pixelAt(buf, w, 0, 0))
	}
	// Its glyph is painted somewhere in the cell box in fg = red.
	if !scanHasColor(buf, w, 0, 0, GlyphAdvance(), GlyphHeight(), fg) {
		t.Fatal("explicit cell glyph not painted in fg")
	}
	// Empty unset cell (1,0) falls back to the theme surface.
	cx := GlyphAdvance()
	if pixelAt(buf, w, cx, 0) != theme.Surface {
		t.Fatalf("unset cell bg = %+v, want theme.Surface", pixelAt(buf, w, cx, 0))
	}
	// Inverted cursor cell (1,1): empty + unset, so fg/bg swap makes the
	// whole cell the theme ink.
	ix, iy := GlyphAdvance(), GlyphHeight()
	if pixelAt(buf, w, ix, iy) != theme.OnSurface {
		t.Fatalf("inverted cursor cell = %+v, want theme.OnSurface", pixelAt(buf, w, ix, iy))
	}
}

func TestTerminalViewDrawDefaultColours(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	v := NewTerminalView(2, 2)
	v.DefaultFG = RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}
	v.DefaultBG = RGBA{R: 0x40, G: 0x50, B: 0x60, A: 0xFF}
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)
	// Empty cell with DefaultBG set uses it (not the theme).
	if pixelAt(buf, w, 0, 0) != v.DefaultBG {
		t.Fatalf("cell bg = %+v, want DefaultBG", pixelAt(buf, w, 0, 0))
	}
}

func TestTerminalViewSetCellSizeOverride(t *testing.T) {
	// Setting the override BEFORE SetBounds: the resolved size stays 0
	// until bounds are known (Draw is a no-op meanwhile), then reflects
	// the pinned geometry rather than the font metrics.
	v := NewTerminalView(2, 2)
	v.SetCellSize(11, 23)
	if v.CellWidth() != 0 || v.CellHeight() != 0 {
		t.Fatal("cell size should stay 0 before SetBounds even with an override")
	}
	v.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 80})
	if v.CellWidth() != 11 || v.CellHeight() != 23 {
		t.Fatalf("override not honoured by SetBounds: %dx%d, want 11x23",
			v.CellWidth(), v.CellHeight())
	}

	// Setting the override AFTER SetBounds: it takes effect immediately.
	v.SetCellSize(7, 9)
	if v.CellWidth() != 7 || v.CellHeight() != 9 {
		t.Fatalf("override after SetBounds not immediate: %dx%d, want 7x9",
			v.CellWidth(), v.CellHeight())
	}

	// A single-axis override: the other axis still derives from the font.
	v.SetCellSize(13, 0)
	if v.CellWidth() != 13 || v.CellHeight() != GlyphHeight() {
		t.Fatalf("mixed override = %dx%d, want 13x%d",
			v.CellWidth(), v.CellHeight(), GlyphHeight())
	}
	v.SetCellSize(0, 17)
	if v.CellWidth() != GlyphAdvance() || v.CellHeight() != 17 {
		t.Fatalf("mixed override = %dx%d, want %dx17",
			v.CellWidth(), v.CellHeight(), GlyphAdvance())
	}

	// Clearing both axes restores the font metrics entirely.
	v.SetCellSize(0, 0)
	if v.CellWidth() != GlyphAdvance() || v.CellHeight() != GlyphHeight() {
		t.Fatalf("cleared override = %dx%d, want font %dx%d",
			v.CellWidth(), v.CellHeight(), GlyphAdvance(), GlyphHeight())
	}
}

func TestTerminalViewDrawWithCellSizeOverride(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	v := NewTerminalView(2, 2)
	v.SetCellSize(8, 12)
	fg := RGBA{R: 0xFF, A: 0xFF}
	bg := RGBA{B: 0xFF, A: 0xFF}
	v.SetCell(1, 1, 'A', fg, bg)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)
	// Cell (1,1) sits at the pinned 8x12 offset, not the font-derived one.
	if pixelAt(buf, w, 1*8, 1*12) != bg {
		t.Fatalf("cell (1,1) bg at pinned offset = %+v, want blue",
			pixelAt(buf, w, 8, 12))
	}
}

// The background fast-path (skip cells whose bg matches the whole-bounds
// clear) must be pixel-identical to the naive path that span-fills every
// cell over the clear. This renders a mixed grid — default cells, custom
// bg cells, a blank custom cell, and an inverted cursor — twice: once via
// Draw and once via an inline naive reference that fills EVERY cell, then
// asserts the two buffers are byte-for-byte equal.
func TestTerminalViewDrawBackgroundFastPathPixelIdentical(t *testing.T) {
	const w, h = 60, 40
	theme := DefaultLight()

	build := func() *TerminalView {
		v := NewTerminalView(3, 2)
		v.DefaultFG = RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}
		v.DefaultBG = RGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xFF}
		v.SetCell(0, 0, 'x', RGBA{R: 0xA0, A: 0xFF}, RGBA{G: 0xA0, A: 0xFF}) // custom fg+bg
		v.SetCell(1, 0, 'y', RGBA{}, RGBA{})                                 // all default (skipped)
		v.SetCell(2, 0, ' ', RGBA{}, RGBA{B: 0x90, A: 0xFF})                 // blank + custom bg
		// (0,1),(1,1) left blank/default; (2,1) is the cursor cell.
		v.CursorVisible = true
		v.CursorCol, v.CursorRow = 2, 1
		v.SetBounds(Rect{X: 3, Y: 5, W: w - 6, H: h - 10})
		return v
	}

	// Fast-path render (production Draw).
	fast := makeSurface(w, h)
	build().Draw(newP(fast, w), theme)

	// Naive reference render: same colour logic, but fill EVERY cell.
	naive := makeSurface(w, h)
	{
		v := build()
		p := newP(naive, w)
		r := v.Bounds()
		defFG := v.DefaultFG
		defBG := v.DefaultBG
		fillRect(p, r.X, r.Y, r.W, r.H, defBG)
		for row := 0; row < v.Rows; row++ {
			for col := 0; col < v.Cols; col++ {
				cell := v.Cells[row*v.Cols+col]
				fg := cell.FG
				if fg.A == 0 {
					fg = defFG
				}
				bg := cell.BG
				if bg.A == 0 {
					bg = defBG
				}
				if v.CursorVisible && col == v.CursorCol && row == v.CursorRow {
					fg, bg = bg, fg
				}
				x := r.X + col*v.CellWidth()
				y := r.Y + row*v.CellHeight()
				fillRect(p, x, y, v.CellWidth(), v.CellHeight(), bg) // no skip
				if cell.Rune != 0 && cell.Rune != ' ' {
					v.drawText(p, x, y, string(cell.Rune), fg)
				}
			}
		}
	}

	if !bytes.Equal(fast, naive) {
		t.Fatal("fast-path Draw is not pixel-identical to the naive per-cell fill")
	}

	// Spot-check the meaningful colours so a regression in BOTH paths at
	// once still fails: custom cell, default cell, and inverted cursor.
	v := build()
	cw, ch := v.CellWidth(), v.CellHeight()
	if pixelAt(fast, w, 3, 5) != (RGBA{G: 0xA0, A: 0xFF}) {
		t.Fatalf("custom cell bg = %+v", pixelAt(fast, w, 3, 5))
	}
	if pixelAt(fast, w, 3+1*cw, 5) != v.DefaultBG {
		t.Fatalf("default cell bg = %+v, want DefaultBG", pixelAt(fast, w, 3+cw, 5))
	}
	// Cursor cell (2,1): empty + unset → inverted so the box becomes fg.
	if pixelAt(fast, w, 3+2*cw, 5+1*ch) != v.DefaultFG {
		t.Fatalf("inverted cursor cell = %+v, want DefaultFG",
			pixelAt(fast, w, 3+2*cw, 5+ch))
	}
}

func TestTerminalViewOnEvent(t *testing.T) {
	v := NewTerminalView(2, 2)
	// nil OnKey: no panic.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	got := ""
	v.OnKey = func(ev Event) { got = ev.Code }
	v.OnEvent(Event{Kind: EventChar, Code: "k"})
	if got != "k" {
		t.Fatalf("OnKey got %q, want k", got)
	}
}
