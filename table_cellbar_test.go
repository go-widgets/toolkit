// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"math"
	"testing"
)

// barSamples reads the pixels of the bar in cell (row 0, column 0) of a heatTable
// (two auto columns, 200x120): one inside the left edge, one a fraction of the way
// along, and the track near the right edge.
func barSamples(s []byte, tb *Table) (left, mid, right RGBA) {
	y := scaled(TableHeaderHeight) + tb.rowH()/2
	// The text is on the middle of the cell: sample on the bar's upper edge band.
	y = scaled(TableHeaderHeight) + scaled(2) + strokeWidth()
	colW := 100
	return pxAt(s, 200, tb.cellPadX()/2+strokeWidth()+1, y),
		pxAt(s, 200, colW/2, y),
		pxAt(s, 200, colW-tb.cellPadX()/2-strokeWidth()-1, y)
}

func TestTableCellBarPaintsTrackAndFill(t *testing.T) {
	theme := DefaultLight()
	green := RGB(0x2E, 0x9E, 0x55)
	tb := heatTable()
	tb.CellBar = func(row, col int) (float64, RGBA, bool) {
		if row == 0 && col == 0 {
			return 0.5, green, true
		}
		return 0, RGBA{}, false
	}
	s := surface(200, 120)
	tb.Draw(s, theme)
	left, _, right := barSamples(s.Buf, tb)
	if left != green {
		t.Fatalf("the filled part = %v, want the fill %v", left, green)
	}
	if right != theme.SurfaceAlt {
		t.Fatalf("the unfilled part = %v, want the track %v", right, theme.SurfaceAlt)
	}
	// A declined cell is a plain text cell.
	y := scaled(TableHeaderHeight) + scaled(2) + strokeWidth()
	if got := pxAt(s.Buf, 200, 110, y); got != theme.Surface {
		t.Fatalf("a declined cell = %v, want the row background %v", got, theme.Surface)
	}
}

func TestTableCellBarClampsTheFraction(t *testing.T) {
	theme := DefaultLight()
	green := RGB(0x2E, 0x9E, 0x55)
	for name, f := range map[string]float64{"negative": -3, "NaN": math.NaN()} {
		tb := heatTable()
		tb.CellBar = func(int, int) (float64, RGBA, bool) { return f, green, true }
		s := surface(200, 120)
		tb.Draw(s, theme)
		if left, _, _ := barSamples(s.Buf, tb); left != theme.SurfaceAlt {
			t.Fatalf("%s: the bar is not empty (%v)", name, left)
		}
	}
	tb := heatTable()
	tb.CellBar = func(int, int) (float64, RGBA, bool) { return 7, green, true }
	s := surface(200, 120)
	tb.Draw(s, theme)
	if _, _, right := barSamples(s.Buf, tb); right != green {
		t.Fatalf("a fraction over 1 does not fill the bar (%v)", right)
	}
}

// A transparent fill is the theme's Accent.
func TestTableCellBarDefaultsToAccent(t *testing.T) {
	theme := DefaultDark()
	tb := heatTable()
	tb.CellBar = func(int, int) (float64, RGBA, bool) { return 1, RGBA{}, true }
	s := surface(200, 120)
	tb.Draw(s, theme)
	if left, _, _ := barSamples(s.Buf, tb); left != theme.Accent {
		t.Fatalf("the fill = %v, want Accent %v", left, theme.Accent)
	}
}

// A CellBar that declines every cell renders byte-for-byte like none.
func TestTableCellBarDecliningIsIdentity(t *testing.T) {
	theme := DefaultDark()
	a, b := heatTable(), heatTable()
	b.CellBar = func(int, int) (float64, RGBA, bool) { return 0, RGBA{}, false }
	sa, sb := surface(200, 120), surface(200, 120)
	a.Draw(sa, theme)
	b.Draw(sb, theme)
	if !bytes.Equal(sa.Buf, sb.Buf) {
		t.Fatal("a declining CellBar changed the picture")
	}
}

// The selected row keeps its band, and the bar sits on it.
func TestTableCellBarOnTheSelectedRow(t *testing.T) {
	theme := DefaultLight()
	green := RGB(0x2E, 0x9E, 0x55)
	tb := heatTable()
	tb.Selected().Set(0)
	tb.CellBar = func(int, int) (float64, RGBA, bool) { return 1, green, true }
	s := surface(200, 120)
	tb.Draw(s, theme)
	y0 := scaled(TableHeaderHeight) + 1
	if got := pxAt(s.Buf, 200, 1, y0); got != theme.Accent {
		t.Fatalf("the selection band = %v, want %v", got, theme.Accent)
	}
	if left, _, _ := barSamples(s.Buf, tb); left != green {
		t.Fatalf("the bar on the selected row = %v, want %v", left, green)
	}
}

// The frozen and the scrolled passes both draw the bar.
func TestTableCellBarFrozenColumns(t *testing.T) {
	theme := DefaultLight()
	green := RGB(0x2E, 0x9E, 0x55)
	tb := frozenTable()
	tb.CellBar = func(int, int) (float64, RGBA, bool) { return 1, green, true }
	s := surface(150, 200)
	tb.Draw(s, theme)
	y := scaled(TableHeaderHeight) + scaled(2) + strokeWidth()
	if got := pxAt(s.Buf, 150, tb.cellPadX()/2+strokeWidth()+1, y); got != green {
		t.Fatalf("frozen cell bar = %v, want %v", got, green)
	}
	if got := pxAt(s.Buf, 150, 60+tb.cellPadX()/2+strokeWidth()+1, y); got != green {
		t.Fatalf("scrolled cell bar = %v, want %v", got, green)
	}
}

func TestProgressBarFillColour(t *testing.T) {
	theme := DefaultLight()
	red := RGB(0xC0, 0x30, 0x30)
	for _, c := range []struct {
		name  string
		setup func(*ProgressBar)
		x, y  int
	}{
		{"horizontal", func(p *ProgressBar) { p.SetFraction(1) }, 50, 10},
		{"vertical", func(p *ProgressBar) { p.Orientation = Vertical; p.SetFraction(1) }, 10, 50},
		{"indeterminate", func(p *ProgressBar) { p.Indeterminate = true; p.Phase = 0.5 }, 50, 10},
		{"indeterminate vertical", func(p *ProgressBar) { p.Indeterminate = true; p.Orientation = Vertical; p.Phase = 0.5 }, 10, 50},
	} {
		def, set := NewProgressBar(), NewProgressBar()
		c.setup(def)
		c.setup(set)
		set.FillColor().Set(red)
		for _, p := range []*ProgressBar{def, set} {
			p.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
		}
		sd, ss := surface(100, 100), surface(100, 100)
		def.Draw(sd, theme)
		set.Draw(ss, theme)
		if got := pxAt(sd.Buf, 100, c.x, c.y); got != theme.Accent {
			t.Fatalf("%s: default fill = %v, want Accent %v", c.name, got, theme.Accent)
		}
		if got := pxAt(ss.Buf, 100, c.x, c.y); got != red {
			t.Fatalf("%s: Fill = %v, want %v", c.name, got, red)
		}
	}
}
