// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// tallFont is a font 30 units tall that records where each string is drawn.
type tallFont struct{ at map[string]int }

func (f *tallFont) Advance() int         { return 8 }
func (f *tallFont) Height() int          { return 30 }
func (f *tallFont) Measure(s string) int { return 8 * len(s) }
func (f *tallFont) Draw(_ painter.Painter, _, y int, s string, _ RGBA) {
	f.at[s] = y
}

// The legend is as tall as the font needs: with a 30-unit font its text
// lies wholly inside the widget, and nodes are laid out above it. (With the
// fixed 16-unit legend the text ran 17 units past the bottom edge.)
func TestGraphViewLegendFitsTheFont(t *testing.T) {
	f := &tallFont{at: map[string]int{}}
	g := NewGraphView(classGraph())
	g.Font = f
	g.ShowLegend = true
	r := Rect{X: 0, Y: 0, W: 400, H: 300}
	g.SetBounds(r)
	surf := makeSurface(400, 300)
	g.Draw(newP(surf, 400), DefaultLight())
	y, ok := f.at["denied"]
	if !ok {
		t.Fatal("no legend drawn")
	}
	if y < r.Y || y+f.Height() > r.Y+r.H {
		t.Fatalf("legend text at y=%d..%d, outside %d..%d", y, y+f.Height(), r.Y, r.Y+r.H)
	}
	for _, id := range []string{"a", "b", "c"} {
		box, _ := g.NodeBox(id)
		if box.Y+box.H > y {
			t.Fatalf("node %s (%v) overlaps the legend at y=%d", id, box, y)
		}
	}
	if h := g.legendHeight(); h < f.Height()+2 {
		t.Fatalf("legend height %d for a %d font", h, f.Height())
	}
}
