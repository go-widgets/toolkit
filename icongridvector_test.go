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

// TestIconGridMinCellWidth: a grid whose label identifies the cell can widen it,
// so the name is not elided away.
//
// A cell is otherwise as wide as its icon plus padding, which is right for a
// grid of files and wrong for a grid of devices: "VITURE Luma Ultra" under a
// 40-pixel icon came out as "VITURE ...", which is the one thing the tile exists
// to say.
func TestIconGridMinCellWidth(t *testing.T) {
	g := NewIconGrid(IconCell{Icon: DrawIconGlasses, Label: "VITURE Luma Ultra"})
	narrow := g.cellW()

	g.MinCellW = narrow * 2
	if wide := g.cellW(); wide != narrow*2 {
		t.Errorf("the floor gave %d, want %d", wide, narrow*2)
	}
	// A floor under the icon-derived width changes nothing: it is a floor.
	g.MinCellW = 1
	if got := g.cellW(); got != narrow {
		t.Errorf("a floor of 1 changed the width to %d, want %d", got, narrow)
	}

	// The label survives at the wider cell. Measured by what the grid would
	// elide it to, which is the thing that was going wrong.
	g.MinCellW = 0
	g.SetBounds(Rect{X: 0, Y: 0, W: 4 * narrow, H: 200})
	short := ellipsize(g.EffectiveFont(), "VITURE Luma Ultra", g.cellW()-scaled(igLabelPad))
	g.MinCellW = 220
	full := ellipsize(g.EffectiveFont(), "VITURE Luma Ultra", g.cellW()-scaled(igLabelPad))
	if len(full) <= len(short) {
		t.Errorf("a wider cell shows %q, no more of the name than the narrow "+
			"cell's %q", full, short)
	}

	// And the floor scales, like every other metric.
	was := MetricScale()
	defer SetMetricScale(was)
	SetMetricScale(2)
	if got, want := g.cellW(), scaled(220); got != want {
		t.Errorf("at 2x the floor gave %d, want %d", got, want)
	}
}

// TestDrawIconAppStaysInsideItsBox and paints a window rather than a blank
// frame: the bar is what makes it read as an application at 40 pixels.
func TestDrawIconAppStaysInsideItsBox(t *testing.T) {
	const w, h = 48, 48
	buf := makeSurface(w, h)
	box := Rect{X: 8, Y: 8, W: 32, H: 32}
	ink := RGB(0xFF, 0x00, 0x00)
	DrawIconApp(newP(buf, w), box, ink)

	painted, inBar := 0, 0
	bar := box.H/8 + (box.H-2*(box.H/8))/5 // inset + a fifth of the content
	for y := range h {
		for x := range w {
			if pixelAt(buf, w, x, y) != ink {
				continue
			}
			painted++
			if x < box.X || x > box.X+box.W || y < box.Y || y > box.Y+box.H {
				t.Errorf("ink at %d,%d, outside the box %+v", x, y, box)
			}
			if y <= box.Y+bar {
				inBar++
			}
		}
	}
	if painted < 40 {
		t.Errorf("the icon painted %d pixels; it is meant to be a frame, a title "+
			"bar and a button", painted)
	}
	// The frame alone would put almost nothing in the top fifth beyond its own
	// edge; the bar and the dot are what this counts.
	if inBar < box.W {
		t.Errorf("only %d pixels in the title bar band; it is a blank frame, not "+
			"a window", inBar)
	}

	// Too small to divide: it must draw nothing rather than dividing by zero.
	tiny := makeSurface(4, 4)
	DrawIconApp(newP(tiny, 4), Rect{X: 0, Y: 0, W: 4, H: 4}, ink)
	// And a rect with no room at all.
	DrawIconApp(newP(tiny, 4), Rect{X: 0, Y: 0, W: 1, H: 1}, ink)
	// Wide and SHORT: there is room for a frame and none for a button under the
	// bar, which must leave the button out rather than draw it over the frame.
	flat := makeSurface(24, 8)
	DrawIconApp(newP(flat, 24), Rect{X: 0, Y: 0, W: 20, H: 5}, ink)
}

// TestIconGridColumnsIsWhatTheArrowsNeed: the count a host adds or subtracts to
// move a selection down or up, and it must follow the width.
func TestIconGridColumnsIsWhatTheArrowsNeed(t *testing.T) {
	g := NewIconGrid(IconCell{Label: "a"}, IconCell{Label: "b"}, IconCell{Label: "c"})
	g.SetIconSize(40)

	// One cell's width, so one column, whatever the label wants.
	g.SetBounds(Rect{X: 0, Y: 0, W: 1, H: 200})
	if got := g.Columns(); got != 1 {
		t.Errorf("Columns() = %d in a one-pixel width, want 1", got)
	}

	narrow := Rect{X: 0, Y: 0, W: 200, H: 200}
	g.SetBounds(narrow)
	few := g.Columns()
	g.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 200})
	many := g.Columns()
	if !(many > few) {
		t.Errorf("Columns() = %d at 800 wide and %d at 200; it must follow the width", many, few)
	}
	// And it agrees with the layout the widget itself uses: fewer columns is
	// more rows, so the height it asks for must grow.
	if tall, short := g.Measure(200), g.Measure(800); !(tall > short) {
		t.Errorf("Measure = %d at 200 wide and %d at 800; %d columns against %d "+
			"must mean more rows", tall, short, few, many)
	}
}

// TestSetSelectedScrollsTheCellIntoView is what a KEYBOARD host needs: down
// means the cell a row further on, and a selection that walks off the bottom is
// a highlight the person cannot see.
func TestSetSelectedScrollsTheCellIntoView(t *testing.T) {
	var cells []IconCell
	for i := range 40 {
		cells = append(cells, IconCell{Label: string(rune('a' + i%26))})
	}
	g := NewIconGrid(cells...)
	g.SetIconSize(40)
	// Two columns and room for about two rows, so most of it is out of view.
	g.MinCellW = 100
	g.SetBounds(Rect{X: 0, Y: 0, W: 2 * 100, H: 200})

	// The last cell cannot be visible at scroll zero.
	g.SetSelected(len(cells) - 1)
	if g.scroll == 0 {
		t.Fatal("selecting the last cell left the grid scrolled to the top")
	}
	// It is now inside the view, not past the end.
	rows := (len(cells) + g.Columns() - 1) / g.Columns()
	if max := rows*g.cellH() - 200; g.scroll > max {
		t.Errorf("scroll = %d, past the end at %d", g.scroll, max)
	}

	// Back to the first: scrolled all the way up again.
	g.SetSelected(0)
	if g.scroll != 0 {
		t.Errorf("scroll = %d after selecting the first cell, want 0", g.scroll)
	}

	// A cell already in view moves nothing — the mouse path is unchanged.
	g.SetSelected(1)
	if g.scroll != 0 {
		t.Errorf("scroll = %d after selecting a visible cell, want it left alone", g.scroll)
	}

	// With no bounds yet there is nothing to reveal into, and it must not divide
	// by a height of nothing.
	fresh := NewIconGrid(cells...)
	fresh.SetSelected(30)
	if fresh.scroll != 0 {
		t.Errorf("scroll = %d before the grid has bounds, want 0", fresh.scroll)
	}
}

// TestTheIconIsCentredOverItsLabel, in a cell made wide by MinCellW.
//
// A cell is normally an icon plus its padding, where centred and left-padded are
// the same place — so this is invisible in a grid of files and glaring in a grid
// whose LABEL identifies the cell: the icon sat against the left padding with
// its name centred under a cell four times as wide.
func TestTheIconIsCentredOverItsLabel(t *testing.T) {
	const w, h = 600, 200
	buf := makeSurface(w, h)
	g := NewIconGrid(IconCell{Icon: DrawIconApp, Label: "Code (15 windows) on screen 1"})
	g.SetIconSize(40)
	g.MinCellW = 600 // one very wide cell
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	g.Draw(newP(buf, w), DefaultDark())

	// Where is the ink? The icon is the only thing in the top band.
	minX, maxX := w, -1
	for y := 0; y < 60; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == DefaultDark().Surface {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
	}
	if maxX < 0 {
		t.Fatal("nothing was drawn in the icon band")
	}
	mid := (minX + maxX) / 2
	// Within a few pixels of the middle of the cell, rather than against the
	// left padding at around 14.
	if got, want := mid, w/2; got < want-8 || got > want+8 {
		t.Errorf("the icon's middle is at %d, want about %d (the cell's); minX=%d maxX=%d",
			got, want, minX, maxX)
	}
}

// TestThePlusIsSymmetric, which is the whole reason it exists rather than a
// typeset "+".
//
// Measured the way a person sees it: the ink's own bounding box, mirrored both
// ways. A glyph from a font fails this — it sits left of centre in its box and
// its arms are unequal, which at a hand's width on a headset is plain.
func TestThePlusIsSymmetric(t *testing.T) {
	for _, side := range []int{16, 17, 24, 31, 32, 48, 100} {
		w, h := side+8, side+8
		buf := makeSurface(w, h)
		box := Rect{X: 4, Y: 4, W: side, H: side}
		ink := RGB(0xFF, 0x00, 0x00)
		DrawIconPlus(newP(buf, w), box, ink)

		// Anything that is not the surface as it was: a disc's rim is
		// anti-aliased, so counting only the pixels that came out exactly
		// the colour measures the square inscribed in it -- which is how the
		// first version of this test called a disc a square -- and counting
		// "any green" counts a background that is not black.
		bg := pixelAt(buf, w, 0, 0)
		minX, minY, maxX, maxY := w, h, -1, -1
		painted := 0
		for y := range h {
			for x := range w {
				if pixelAt(buf, w, x, y) == bg {
					continue
				}
				painted++
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
		if painted == 0 {
			t.Errorf("side %d: nothing was drawn", side)
			continue
		}
		// Mirror the ink about the middle of its own bounding box, both ways.
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				on := pixelAt(buf, w, x, y) == ink
				if got := pixelAt(buf, w, minX+maxX-x, y) == ink; got != on {
					t.Fatalf("side %d: not symmetric left-to-right at %d,%d", side, x, y)
				}
				if got := pixelAt(buf, w, x, minY+maxY-y) == ink; got != on {
					t.Fatalf("side %d: not symmetric top-to-bottom at %d,%d", side, x, y)
				}
			}
		}
	}

	// Degenerate boxes draw nothing rather than dividing by one.
	tiny := makeSurface(4, 4)
	DrawIconPlus(newP(tiny, 4), Rect{X: 0, Y: 0, W: 1, H: 1}, RGB(0xFF, 0, 0))
	DrawIconPlus(newP(tiny, 4), Rect{X: 0, Y: 0, W: 4, H: 4}, RGB(0xFF, 0, 0))
	// A rectangle gives a plus, not a cross: the square is centred in it, either
	// way round.
	wide := makeSurface(60, 20)
	DrawIconPlus(newP(wide, 60), Rect{X: 0, Y: 0, W: 60, H: 20}, RGB(0xFF, 0, 0))
	tall := makeSurface(20, 60)
	DrawIconPlus(newP(tall, 20), Rect{X: 0, Y: 0, W: 20, H: 60}, RGB(0xFF, 0, 0))
	// And a box small enough that a fifth of it is nothing.
	small := makeSurface(12, 12)
	DrawIconPlus(newP(small, 12), Rect{X: 0, Y: 0, W: 8, H: 8}, RGB(0xFF, 0, 0))
}

// TestTheDotIsRoundAndCentred.
//
// A status light is read at a glance and often out of the corner of an eye, so
// what matters is that it is a disc where it was asked for and not a square,
// and that it stays inside its box: a dot drawn over another icon that leaked
// a pixel would smear the glyph under it.
func TestTheDotIsRoundAndCentred(t *testing.T) {
	for _, side := range []int{8, 11, 16, 24, 40} {
		w, h := side+8, side+8
		buf := makeSurface(w, h)
		box := Rect{X: 4, Y: 4, W: side, H: side}
		DrawIconDot(newP(buf, w), box, RGB(0x00, 0xFF, 0x00))

		// Everything the dot TOUCHED, not only what came out exactly the
		// colour: the rim of a disc is anti-aliased, and the fully covered
		// pixels are a smaller disc drawn on a coarser grid -- 88% of its own
		// box at 16 pixels, which barely tells a disc from a square. What the
		// ink reached does: it is the disc itself.
		bg := pixelAt(buf, w, 0, 0)
		minX, minY, maxX, maxY := w, h, -1, -1
		painted := 0
		for y := range h {
			for x := range w {
				if pixelAt(buf, w, x, y) == bg {
					continue
				}
				painted++
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
		if painted == 0 {
			t.Errorf("side %d: nothing was drawn", side)
			continue
		}
		// Inside its box, always: the inset every icon here leaves is the
		// margin, and a dot that overflowed would smear whatever it sits on.
		if minX < box.X || minY < box.Y || maxX >= box.X+box.W || maxY >= box.Y+box.H {
			t.Errorf("side %d: ink spans %d..%d,%d..%d, outside the box at %d,%d %dx%d",
				side, minX, maxX, minY, maxY, box.X, box.Y, box.W, box.H)
		}
		// Square in a square box, to the pixel.
		if (maxX - minX) != (maxY - minY) {
			t.Errorf("side %d: the ink is %dx%d; a dot in a square box is square",
				side, maxX-minX+1, maxY-minY+1)
		}
		// And ROUND, not the square it would be without a radius. The corners
		// are the test: a square inks all four of them, a disc none. Area does
		// not say it as plainly -- an anti-aliased disc TOUCHES about 85% of
		// its box, where the disc itself is pi/4 of it, about 79%.
		//
		// Only where there is room to tell. A disc four pixels across touches
		// all sixteen of them, corners included, so nothing separates it from a
		// square there, and asserting otherwise would assert something untrue
		// of a dot at a real size.
		if maxX-minX+1 < 10 {
			continue
		}
		for _, c := range [][2]int{{minX, minY}, {maxX, minY}, {minX, maxY}, {maxX, maxY}} {
			if pixelAt(buf, w, c[0], c[1]) != bg {
				t.Errorf("side %d: the corner at %d,%d is inked; a disc leaves its box's corners alone",
					side, c[0], c[1])
			}
		}
	}
	// The corner check discriminates: a plain rectangle of the same size inks
	// the very corners the dot leaves alone. Without this, a DrawIconDot that
	// quietly lost its radius would pass everything above.
	sq := makeSurface(48, 48)
	bg := pixelAt(sq, 48, 0, 0)
	newP(sq, 48).FillRect(Rect{X: 4, Y: 4, W: 40, H: 40}, RGB(0x00, 0xFF, 0x00))
	if pixelAt(sq, 48, 4, 4) == bg {
		t.Error("a filled rectangle left its corner blank; the corner check above proves nothing")
	}

	// A box that is not square gives a dot that is not round: a caller asking
	// for a wide box means a wide dot, and the radius follows the SHORTER side
	// so the ends stay half-circles instead of overshooting into the long one.
	for _, box := range []Rect{{X: 4, Y: 4, W: 40, H: 16}, {X: 4, Y: 4, W: 16, H: 40}} {
		buf := makeSurface(48, 48)
		bg := pixelAt(buf, 48, 0, 0)
		DrawIconDot(newP(buf, 48), box, RGB(0x00, 0xFF, 0x00))
		minX, minY, maxX, maxY := 48, 48, -1, -1
		for y := range 48 {
			for x := range 48 {
				if pixelAt(buf, 48, x, y) != bg {
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
		gotW, gotH := maxX-minX+1, maxY-minY+1
		if wantWide := box.W > box.H; wantWide != (gotW > gotH) {
			t.Errorf("a %dx%d box gave a %dx%d dot", box.W, box.H, gotW, gotH)
		}
	}

	// A box with no room draws nothing rather than a stray pixel.
	tiny := makeSurface(4, 4)
	DrawIconDot(newP(tiny, 4), Rect{X: 0, Y: 0, W: 1, H: 1}, RGB(0xFF, 0, 0))
	DrawIconDot(newP(tiny, 4), Rect{X: 0, Y: 0, W: 4, H: 4}, RGB(0xFF, 0, 0))
}

// TestABarKeepsTheThicknessItWasGiven is the defect DrawIconBar exists for.
//
// DrawIconDot insets its box by at least two pixels a side, which is right for
// a dot and fatal for a bar: a four-pixel-tall box comes back EMPTY, silently.
// A caller asking for a bar has already chosen its thickness.
func TestABarKeepsTheThicknessItWasGiven(t *testing.T) {
	const w, h = 48, 24
	for _, thickness := range []int{2, 3, 4, 8} {
		buf := makeSurface(w, h)
		bg := pixelAt(buf, w, 0, 0)
		box := Rect{X: 8, Y: (h - thickness) / 2, W: 32, H: thickness}
		DrawIconBar(newP(buf, w), box, RGB(0x00, 0xFF, 0x00))

		minY, maxY, painted := h, -1, 0
		for y := range h {
			for x := range w {
				if pixelAt(buf, w, x, y) != bg {
					painted++
					minY, maxY = min(minY, y), max(maxY, y)
				}
			}
		}
		if painted == 0 {
			t.Errorf("a %d-pixel bar drew nothing at all", thickness)
			continue
		}
		// The thickness it was given, within the antialiasing of the rounded
		// ends -- not two pixels less on each side.
		if got := maxY - minY + 1; got < thickness || got > thickness+1 {
			t.Errorf("a %d-pixel bar came out %d pixels thick", thickness, got)
		}
	}

	// And the SAME box through DrawIconDot draws nothing, which is why this
	// exists rather than a caller passing a wider rectangle.
	buf := makeSurface(w, h)
	bg := pixelAt(buf, w, 0, 0)
	DrawIconDot(newP(buf, w), Rect{X: 8, Y: 10, W: 32, H: 4}, RGB(0x00, 0xFF, 0x00))
	for y := range h {
		for x := range w {
			if pixelAt(buf, w, x, y) != bg {
				t.Fatal("DrawIconDot drew a four-pixel bar after all; DrawIconBar is unnecessary")
			}
		}
	}
}

// TestABarWithNoRoomDrawsNothing rather than a stray pixel or a panic.
func TestABarWithNoRoomDrawsNothing(t *testing.T) {
	buf := makeSurface(8, 8)
	bg := pixelAt(buf, 8, 0, 0)
	DrawIconBar(newP(buf, 8), Rect{X: 1, Y: 1, W: 0, H: 4}, RGB(0xFF, 0, 0))
	DrawIconBar(newP(buf, 8), Rect{X: 1, Y: 1, W: 4, H: 0}, RGB(0xFF, 0, 0))
	for y := range 8 {
		for x := range 8 {
			if pixelAt(buf, 8, x, y) != bg {
				t.Fatalf("something was drawn at %d,%d", x, y)
			}
		}
	}
}

// TestTheEndsAreRounded: a lit line, not a filled box. The corners of the
// bounding box are what says which.
func TestTheEndsAreRounded(t *testing.T) {
	const w, h = 48, 24
	buf := makeSurface(w, h)
	bg := pixelAt(buf, w, 0, 0)
	box := Rect{X: 8, Y: 6, W: 32, H: 12}
	DrawIconBar(newP(buf, w), box, RGB(0x00, 0xFF, 0x00))
	for _, c := range [][2]int{
		{box.X, box.Y}, {box.X + box.W - 1, box.Y},
		{box.X, box.Y + box.H - 1}, {box.X + box.W - 1, box.Y + box.H - 1},
	} {
		if pixelAt(buf, w, c[0], c[1]) != bg {
			t.Errorf("the corner at %d,%d is inked; the ends are not rounded", c[0], c[1])
		}
	}
}
