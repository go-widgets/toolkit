// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// metricFont is a font with chosen metrics.
type metricFont struct{ adv, h int }

func (f metricFont) Advance() int                                 { return f.adv }
func (f metricFont) Height() int                                  { return f.h }
func (f metricFont) Measure(s string) int                         { return f.adv * len(s) }
func (f metricFont) Draw(painter.Painter, int, int, string, RGBA) {}

func TestTerminalSetCellSizeRefitsAndFiresOnResize(t *testing.T) {
	var sizes [][2]int
	term := NewTerminal(1, 1, 0)
	term.SetCellSize(16, 16) // before bounds: takes effect at SetBounds
	term.OnResize = func(c, r int) { sizes = append(sizes, [2]int{c, r}) }
	if term.CellWidth() != 0 || term.CellHeight() != 0 {
		t.Fatal("cell size resolved before SetBounds")
	}
	term.SetBounds(Rect{W: 320, H: 160})
	if c, r := term.Screen().Size(); c != 20 || r != 10 {
		t.Fatalf("Size = %d,%d, want 20,10", c, r)
	}
	if term.CellWidth() != 16 || term.CellHeight() != 16 {
		t.Fatalf("cell = %dx%d", term.CellWidth(), term.CellHeight())
	}
	term.SetCellSize(8, 20) // after bounds: refits at once
	if c, r := term.Screen().Size(); c != 40 || r != 8 {
		t.Fatalf("Size = %d,%d, want 40,8", c, r)
	}
	term.SetCellSize(8, 20) // unchanged: no event
	term.SetCellSize(0, 0)  // back to the font's metrics
	if term.CellWidth() != GlyphAdvance() || term.CellHeight() != GlyphHeight() {
		t.Fatalf("cleared cell = %dx%d", term.CellWidth(), term.CellHeight())
	}
	want := [][2]int{{20, 10}, {40, 8}, {320 / GlyphAdvance(), 160 / GlyphHeight()}}
	if len(sizes) != 3 || sizes[0] != want[0] || sizes[1] != want[1] || sizes[2] != want[2] {
		t.Errorf("OnResize calls = %v, want %v", sizes, want)
	}
}

func TestTerminalSetCellSizeOneAxisKeepsFontOnTheOther(t *testing.T) {
	term := NewTerminal(1, 1, 0)
	term.SetFont(metricFont{adv: 7, h: 13})
	term.SetBounds(Rect{W: 140, H: 130})
	term.SetCellSize(0, 10)
	if term.CellWidth() != 7 || term.CellHeight() != 10 {
		t.Fatalf("cell = %dx%d, want 7x10", term.CellWidth(), term.CellHeight())
	}
	if c, r := term.Screen().Size(); c != 20 || r != 13 {
		t.Fatalf("Size = %d,%d, want 20,13", c, r)
	}
}

func TestTerminalSetFontRefitsAndFiresOnResize(t *testing.T) {
	var sizes [][2]int
	term := NewTerminal(1, 1, 0)
	term.SetFont(metricFont{adv: 10, h: 10}) // before bounds: no grid change
	term.OnResize = func(c, r int) { sizes = append(sizes, [2]int{c, r}) }
	term.SetBounds(Rect{W: 100, H: 100})
	if got := term.SetFont(metricFont{adv: 5, h: 20}); got != &term.Base {
		t.Error("SetFont does not return the Base")
	}
	if c, r := term.Screen().Size(); c != 20 || r != 5 {
		t.Fatalf("Size = %d,%d, want 20,5", c, r)
	}
	term.SetCellSize(10, 10) // pinned: a font change no longer moves the grid
	term.SetFont(metricFont{adv: 3, h: 3})
	if c, r := term.Screen().Size(); c != 10 || r != 10 {
		t.Fatalf("pinned Size = %d,%d, want 10,10", c, r)
	}
	term.SetFont(nil) // back to the global font
	want := [][2]int{{10, 10}, {20, 5}, {10, 10}}
	if len(sizes) != 3 || sizes[1] != want[1] || sizes[2] != want[2] {
		t.Errorf("OnResize calls = %v", sizes)
	}
}

func TestTerminalPinnedCellSizeMapsClicks(t *testing.T) {
	term := NewTerminal(1, 1, 0)
	term.SetCellSize(16, 16)
	term.SetBounds(Rect{W: 160, H: 160})
	term.OnEvent(Event{Kind: EventClick, X: 33, Y: 17})
	if p := term.posAt(33, 17); p.Col != 2 || p.Line != 1 {
		t.Errorf("posAt = %+v, want col 2 line 1", p)
	}
	buf := makeSurface(160, 160)
	term.Draw(newP(buf, 160), DefaultLight())
}
