// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"math"
	"testing"
)

// pxAt reads the RGBA pixel (x, y) of a surface.
func pxAt(s []byte, w, x, y int) RGBA {
	o := (y*w + x) * 4
	return RGBA{R: s[o], G: s[o+1], B: s[o+2], A: s[o+3]}
}

// heatTable is a 2x2 all-auto table, 200x120, the shape a heatmap takes.
func heatTable() *Table {
	tb := NewTable([]TableColumn{{Title: "0"}, {Title: "1"}},
		[][]string{{"1", "2"}, {"3", "4"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	return tb
}

// A filled cell carries its fill (sampled in a corner, away from the text);
// a declined cell keeps the row background; text on the fill is inked for
// contrast (white on the dark end of viridis).
func TestTableCellFillPaintsCells(t *testing.T) {
	theme := DefaultLight()
	tb := heatTable()
	dark := Viridis(0)
	tb.CellFill = func(row, col int) (RGBA, bool) {
		if row == 0 && col == 0 {
			return dark, true
		}
		return RGBA{}, false
	}
	s := surface(200, 120)
	tb.Draw(s, theme)
	y0 := scaled(TableHeaderHeight) + 1 // first body row, one pixel in
	if got := pxAt(s.Buf, 200, 2, y0); got != dark {
		t.Fatalf("filled cell (0,0) corner = %v, want the fill %v", got, dark)
	}
	if got := pxAt(s.Buf, 200, 198, y0); got != theme.Surface {
		t.Fatalf("declined cell (0,1) corner = %v, want the row background %v", got, theme.Surface)
	}
	// Some white text ink must have landed inside the filled cell.
	white := 0
	for y := y0; y < y0+tb.rowH()-1; y++ {
		for x := 0; x < 99; x++ {
			if pxAt(s.Buf, 200, x, y) == RGB(0xFF, 0xFF, 0xFF) {
				white++
			}
		}
	}
	if white == 0 {
		t.Fatal("no white ink in a dark filled cell: the text was not re-inked for contrast")
	}
}

// The selected row keeps its accent band; CellFill does not paint over it.
func TestTableCellFillYieldsToSelection(t *testing.T) {
	theme := DefaultLight()
	tb := heatTable()
	tb.Selected().Set(0)
	tb.CellFill = func(row, col int) (RGBA, bool) { return Viridis(1), true }
	s := surface(200, 120)
	tb.Draw(s, theme)
	y0 := scaled(TableHeaderHeight) + 1
	if got := pxAt(s.Buf, 200, 2, y0); got != theme.Accent {
		t.Fatalf("selected row corner = %v, want the accent %v", got, theme.Accent)
	}
	if got := pxAt(s.Buf, 200, 2, y0+tb.rowH()); got != Viridis(1) {
		t.Fatalf("unselected row corner = %v, want the fill %v", got, Viridis(1))
	}
}

// A CellFill that declines every cell renders byte-for-byte like no CellFill.
func TestTableCellFillDecliningIsIdentity(t *testing.T) {
	theme := DefaultDark()
	a, b := heatTable(), heatTable()
	b.CellFill = func(int, int) (RGBA, bool) { return RGBA{}, false }
	sa, sb := surface(200, 120), surface(200, 120)
	a.Draw(sa, theme)
	b.Draw(sb, theme)
	if !bytes.Equal(sa.Buf, sb.Buf) {
		t.Fatal("a declining CellFill changed the picture")
	}
}

// The frozen/scrolled path fills both frozen and scrolled cells.
func TestTableCellFillFrozenColumns(t *testing.T) {
	theme := DefaultLight()
	tb := frozenTable()
	fill := Viridis(0.5)
	tb.CellFill = func(int, int) (RGBA, bool) { return fill, true }
	s := surface(150, 200)
	tb.Draw(s, theme)
	y0 := scaled(TableHeaderHeight) + 1
	if got := pxAt(s.Buf, 150, 2, y0); got != fill {
		t.Fatalf("frozen cell corner = %v, want %v", got, fill)
	}
	if got := pxAt(s.Buf, 150, 62, y0); got != fill {
		t.Fatalf("scrolled cell corner = %v, want %v", got, fill)
	}
}

func TestViridis(t *testing.T) {
	if Viridis(0) != RGB(0x44, 0x01, 0x54) || Viridis(-1) != Viridis(0) || Viridis(math.NaN()) != Viridis(0) {
		t.Fatal("Viridis must clamp at 0 (and map NaN there) to dark purple")
	}
	if Viridis(1) != RGB(0xfd, 0xe7, 0x25) || Viridis(7) != Viridis(1) {
		t.Fatal("Viridis must clamp at 1 to yellow")
	}
	if Viridis(0.5) != RGB(0x21, 0x90, 0x8d) {
		t.Fatalf("Viridis(0.5) = %v, want the middle sample", Viridis(0.5))
	}
	// Halfway between the first two samples.
	if got := Viridis(1.0 / 16); got != RGB(0x46, 0x17, 0x67) {
		t.Fatalf("Viridis(1/16) = %v, want the midpoint of the first two samples", got)
	}
	// Monotone in luminance: the property that makes it readable in greyscale.
	prev := -1.0
	for i := 0; i <= 64; i++ {
		l := relativeLuminance(Viridis(float64(i) / 64))
		if l < prev-1e-3 {
			t.Fatalf("luminance falls at t=%d/64", i)
		}
		prev = l
	}
}

func TestContrastInk(t *testing.T) {
	cases := []struct {
		bg   RGBA
		want RGBA
	}{
		{RGB(0, 0, 0), RGB(0xFF, 0xFF, 0xFF)},
		{RGB(0xFF, 0xFF, 0xFF), RGB(0, 0, 0)},
		{Viridis(0), RGB(0xFF, 0xFF, 0xFF)},
		{Viridis(1), RGB(0, 0, 0)},
		{RGB(0x03, 0x03, 0x03), RGB(0xFF, 0xFF, 0xFF)}, // the linear segment of sRGB
	}
	for _, c := range cases {
		if got := ContrastInk(c.bg); got != c.want {
			t.Errorf("ContrastInk(%v) = %v, want %v", c.bg, got, c.want)
		}
	}
}

// A value Set through Text() (as a two-way binding does) that is shorter than
// where the caret was must not make the next edit slice past the end.
func TestEntryCaretClampedAfterExternalSet(t *testing.T) {
	e := NewEntry("0, 12, 1")
	e.Text().Set("2")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if got := e.Text().Get(); got != "" {
		t.Fatalf("Backspace after an external Set left %q, want the caret clamped to the end and the 2 deleted", got)
	}
}
