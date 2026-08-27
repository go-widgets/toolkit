// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// TestAnIconCellDrawsAVectorIcon.
//
// A cell took an Image and nothing else, so an application whose grid holds
// device classes -- pairs of glasses, printers, drives -- had to rasterise
// artwork to put anything in one, or hand-draw beside the widget. The toolkit
// already had IconFunc and a stock icon family; the grid could not take one.
func TestAnIconCellDrawsAVectorIcon(t *testing.T) {
	var got Rect
	called := 0
	icon := func(_ painter.Painter, r Rect, _ RGBA) { got = r; called++ }

	g := NewIconGrid(IconCell{Icon: icon, Label: "XREAL 1S"})
	g.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	g.Draw(newP(makeSurface(200, 200), 200), DefaultDark())

	if called != 1 {
		t.Fatalf("the icon was drawn %d times", called)
	}
	// Into the icon square, at the size the grid was told to use.
	if got.W != g.IconSize || got.H != g.IconSize {
		t.Errorf("the icon was given %dx%d, the icon size is %d",
			got.W, got.H, g.IconSize)
	}
	if got.X < 0 || got.Y < 0 {
		t.Errorf("the icon square is at %+v", got)
	}

	// The selected cell's icon is drawn in the accent, so the choice reads from
	// the icon and not only from the label chip under it.
	var inks []RGBA
	theme := DefaultDark()
	remember := func(_ painter.Painter, _ Rect, c RGBA) { inks = append(inks, c) }
	g3 := NewIconGrid(IconCell{Icon: remember}, IconCell{Icon: remember})
	g3.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 200})
	g3.SetSelected(1)
	g3.Draw(newP(makeSurface(400, 200), 400), theme)
	if len(inks) != 2 {
		t.Fatalf("%d icons drawn", len(inks))
	}
	if inks[0] != theme.OnSurface || inks[1] != theme.Accent {
		t.Errorf("unselected drawn in %v and selected in %v; want %v then %v",
			inks[0], inks[1], theme.OnSurface, theme.Accent)
	}

	// An Image wins when both are set: a caller who supplied pixels meant them.
	called = 0
	img := NewImage(make([]byte, 4*4*4), 4, 4)
	g2 := NewIconGrid(IconCell{Image: img, Icon: icon, Label: "both"})
	g2.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	g2.Draw(newP(makeSurface(200, 200), 200), DefaultDark())
	if called != 0 {
		t.Error("the vector icon was drawn over the image the caller gave")
	}
	if img.Bounds().W != g2.IconSize {
		t.Errorf("the image was given %+v", img.Bounds())
	}
}

// TestIconGridMeasure: as many rows as the cells make at that width.
//
// This is what makes a grid usable as content instead of something a caller has
// to give a height to -- and a caller computing that height is reproducing
// cellH() * ceil(n/cols) out of the toolkit's own constants.
func TestIconGridMeasure(t *testing.T) {
	cells := make([]IconCell, 7)
	g := NewIconGrid(cells...)

	// Three columns of the seven: three rows.
	wide := 3 * g.cellW()
	if got, want := g.Measure(wide), 3*g.cellH(); got != want {
		t.Errorf("seven cells in three columns measure %d, want %d", got, want)
	}
	// One column: seven rows.
	if got, want := g.Measure(g.cellW()), 7*g.cellH(); got != want {
		t.Errorf("seven cells in one column measure %d, want %d", got, want)
	}
	// Narrower than a single cell: still one column, never nought.
	if got, want := g.Measure(1), 7*g.cellH(); got != want {
		t.Errorf("an impossible width measured %d, want %d", got, want)
	}
	// Empty: one row's worth for the empty-state line, so a grid does not change
	// height the moment something is put in it.
	if got, want := NewIconGrid().Measure(wide), g.cellH(); got != want {
		t.Errorf("an empty grid measures %d, want %d", got, want)
	}
	// And it composes: a column sizes the grid to what it measures, with no size
	// from the caller at all.
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(g)
	col.SetBounds(Rect{X: 0, Y: 0, W: wide, H: 4000})
	if got := g.Bounds().H; got != g.Measure(wide) {
		t.Errorf("in a column the grid is %d tall, it measures %d",
			got, g.Measure(wide))
	}
}

// TestIconGridCellsGrowWithTheInterface.
//
// The cell metrics were raw device pixels while the icon size came from the
// caller, so a magnified interface got a large icon in a cell whose padding,
// label band and selection field had stayed the size they were at 1x.
func TestIconGridCellsGrowWithTheInterface(t *testing.T) {
	was := MetricScale()
	defer SetMetricScale(was)

	SetMetricScale(1)
	g := NewIconGrid(IconCell{Label: "one"})
	one := struct{ w, h int }{g.cellW(), g.cellH()}

	SetMetricScale(3)
	// The icon size is the caller's, and a caller scales it like any other
	// metric; what is being checked is the padding around it.
	g.SetIconSize(Scaled(48))
	three := struct{ w, h int }{g.cellW(), g.cellH()}

	if three.w < 3*one.w-3 || three.h < 3*one.h-3 {
		t.Errorf("at 3x a cell is %dx%d; at 1x it was %dx%d, so the padding did "+
			"not grow with it", three.w, three.h, one.w, one.h)
	}
}

// TestDrawIconGlassesPaintsSomething, inside the rect it was given.
//
// A stock icon that draws outside its box would land on whatever is beside it in
// a toolbar or a grid cell, so the assertion is the bounds as much as the ink.
func TestDrawIconGlassesPaintsSomething(t *testing.T) {
	const w, h = 48, 48
	buf := makeSurface(w, h)
	box := Rect{X: 8, Y: 8, W: 32, H: 32}
	ink := RGB(0xFF, 0x00, 0x00)
	DrawIconGlasses(newP(buf, w), box, ink)

	painted := 0
	for y := range h {
		for x := range w {
			if pixelAt(buf, w, x, y) != ink {
				continue
			}
			painted++
			if x < box.X-box.W/3 || x > box.X+box.W+box.W/3 ||
				y < box.Y || y > box.Y+box.H {
				t.Errorf("ink at %d,%d, well outside the box %+v", x, y, box)
			}
		}
	}
	if painted < 40 {
		t.Errorf("the icon painted %d pixels; it is meant to be two lenses, a "+
			"bridge and two temples", painted)
	}

	// A rect too small to divide still paints, rather than dividing by nothing:
	// a toolbar at a small density asks for exactly this.
	tiny := makeSurface(4, 4)
	DrawIconGlasses(newP(tiny, 4), Rect{X: 0, Y: 0, W: 4, H: 4}, ink)
}
