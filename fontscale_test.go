// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// The bitmap font scales by whole pixels, rounding to the nearest and never
// below one: a half-size font would be a smear of fractions of a pixel.
func TestScaleFontBitmap(t *testing.T) {
	for _, c := range []struct {
		from   int
		factor float64
		want   int
	}{{1, 2, 2}, {2, 1.5, 3}, {3, 0.5, 2}, {1, 0.25, 1}, {2, 1.24, 2}, {2, 1.26, 3}} {
		got := ScaleFont(NewBitmapFont(c.from), c.factor).(*bitmapFont).Scale
		if got != c.want {
			t.Errorf("scale %d x %v = %d, want %d", c.from, c.factor, got, c.want)
		}
	}
	f := NewBitmapFont(2)
	if ScaleFont(f, 1) != f {
		t.Fatal("a factor of 1 must hand the same font back")
	}
	if ScaleFont(f, 0) != f || ScaleFont(f, -3) != f {
		t.Fatal("a factor that is not positive must hand the same font back")
	}
}

// A font that cannot be sized is returned as it is: the text keeps its size and
// nothing is drawn wrong.
func TestScaleFontLeavesAFontThatCannotBeSized(t *testing.T) {
	var f Font = &unsizedFont{}
	if ScaleFont(f, 2) != f {
		t.Fatal("an unsizable font was replaced")
	}
}

type unsizedFont struct{}

func (*unsizedFont) Advance() int                                 { return 6 }
func (*unsizedFont) Height() int                                  { return 7 }
func (*unsizedFont) Measure(string) int                           { return 0 }
func (*unsizedFont) Draw(painter.Painter, int, int, string, RGBA) {}

// A TrueType font is rebuilt at the scaled pixel size from the same bytes.
func TestScaleFontTrueType(t *testing.T) {
	f := newTTF(t, 10)
	big := ScaleFont(f, 2).(*truetypeFont)
	if big.SizePx() != 20 || big.Height() <= f.Height() || string(big.FontData()) != string(f.(*truetypeFont).FontData()) {
		t.Fatalf("size %d height %d (was %d)", big.SizePx(), big.Height(), f.Height())
	}
	if got := ScaleFont(f, 0.01).(*truetypeFont).SizePx(); got != 1 {
		t.Fatalf("size floors at one pixel, got %d", got)
	}
	if ScaleFont(f, 1.04) != f {
		t.Fatal("a factor that rounds to the same pixel size must hand the same font back")
	}
}

// spyFont records how it is asked to be sized and what is drawn with it.
type spyFont struct {
	bitmapFont
	factor float64
	drawn  *[]string
	parent *spyFont
}

func (s *spyFont) Scaled(factor float64) Font {
	return &spyFont{bitmapFont: bitmapFont{Scale: 1}, factor: factor, drawn: &[]string{}, parent: s}
}

func (s *spyFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	*s.drawn = append(*s.drawn, text)
	s.bitmapFont.Draw(p, x, y, text, ink)
}

// ⛔ The text of a graph is the size of its boxes: at 200 % the labels are drawn
// with the widget's font scaled by 2, and at 100 % with the font itself.
func TestGraphViewDrawsItsTextAtTheZoom(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "cc"})
	m.Nodes.Append(GraphNode{ID: "a", Label: "aa", Cluster: "c", Category: "k"})
	var drawn []string
	spy := &spyFont{bitmapFont: bitmapFont{Scale: 1}, drawn: &drawn}
	g := NewGraphView(m)
	g.Font = spy
	g.SetBounds(Rect{W: 300, H: 200})
	surf := makeSurface(300, 200)

	g.Draw(newP(surf, 300), DefaultLight())
	if len(drawn) == 0 {
		t.Fatal("at 100 % the labels are drawn with the font itself")
	}
	drawn = nil

	g.SetZoom(200, 0, 0)
	g.Draw(newP(surf, 300), DefaultLight())
	if len(drawn) != 0 {
		t.Fatalf("at 200 %% %v was drawn with the unscaled font", drawn)
	}
	big, _ := g.zoomFont.(*spyFont)
	if big == nil || big.factor != 2 || big.parent != spy || len(*big.drawn) == 0 {
		t.Fatalf("the labels were not drawn with the font scaled by the zoom: %+v", g.zoomFont)
	}
}
