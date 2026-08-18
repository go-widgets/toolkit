// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// bdPx reads the RGBA pixel at (x, y) from a width-strided RGBA buffer.
func bdPx(buf []byte, width, x, y int) painter.RGBA {
	i := (y*width + x) * 4
	return painter.RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

// bdRender draws b onto a freshly zeroed w×h pixel buffer and returns it. An
// untouched pixel stays the zero RGBA (A=0), so tests can tell painted from
// unpainted ground apart.
func bdRender(b *Backdrop, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b.Draw(p, theme)
	return buf
}

func TestNewBackdrop(t *testing.T) {
	fill := painter.RGB(0x11, 0x13, 0x1a)
	grid := painter.RGB(0x17, 0x1a, 0x24)
	b := NewBackdrop(fill, grid, 40)
	if b.Fill != fill {
		t.Errorf("Fill = %v, want %v", b.Fill, fill)
	}
	if b.Grid != grid {
		t.Errorf("Grid = %v, want %v", b.Grid, grid)
	}
	if b.Step != 40 {
		t.Errorf("Step = %d, want 40", b.Step)
	}
}

func TestBackdropDrawFillAndGrid(t *testing.T) {
	fill := painter.RGB(0x11, 0x13, 0x1a)
	grid := painter.RGB(0x17, 0x1a, 0x24)
	const w, h, step = 20, 20, 10
	buf := bdRender(NewBackdrop(fill, grid, step), w, h, DefaultDark())

	// A vertical grid line sits at x=0 and x=step; a horizontal one at y=0 and
	// y=step. A cell interior pixel (5,5) is neither, so it shows the fill.
	if got := bdPx(buf, w, 5, 5); got != fill {
		t.Errorf("interior (5,5) = %v, want fill %v", got, fill)
	}
	if got := bdPx(buf, w, step, 5); got != grid {
		t.Errorf("vertical line (%d,5) = %v, want grid %v", step, got, grid)
	}
	if got := bdPx(buf, w, 5, step); got != grid {
		t.Errorf("horizontal line (5,%d) = %v, want grid %v", step, got, grid)
	}
	if got := bdPx(buf, w, 0, 0); got != grid {
		t.Errorf("origin (0,0) = %v, want grid %v", got, grid)
	}
}

func TestBackdropGradient(t *testing.T) {
	from := painter.RGB(0x20, 0x40, 0x80)
	to := painter.RGB(0xE0, 0xC0, 0x40)
	const w, h = 16, 16
	th := DefaultLight()
	render := func(d GradientDir) []byte {
		return bdRender(&Backdrop{Fill: from, GradientTo: to, GradientDir: d}, w, h, th)
	}
	check := func(name string, buf []byte, fx, fy, tx, ty int) {
		if got := bdPx(buf, w, fx, fy); got != from {
			t.Errorf("%s start (%d,%d) = %v, want from %v", name, fx, fy, got, from)
		}
		if got := bdPx(buf, w, tx, ty); got != to {
			t.Errorf("%s end (%d,%d) = %v, want to %v", name, tx, ty, got, to)
		}
	}
	check("vertical", render(GradientVertical), 8, 0, 8, h-1)
	check("horizontal", render(GradientHorizontal), 0, 8, w-1, 8)
	check("diagonal", render(GradientDiagonal), 0, 0, w-1, h-1)
	check("cross-diagonal", render(GradientCrossDiagonal), w-1, 0, 0, h-1)
}

func TestBackdropBevel(t *testing.T) {
	const w, h = 16, 16
	th := DefaultLight()
	lum := func(c RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
	fill := painter.RGB(0x80, 0x80, 0x80)

	// Raised: bright top edge over a dark bottom edge (pushed out).
	rb := bdRender(&Backdrop{Fill: fill, Bevel: BevelRaised}, w, h, th)
	if lum(bdPx(rb, w, 8, 0)) <= lum(bdPx(rb, w, 8, h-1)) {
		t.Errorf("raised bevel: top %+v should be brighter than bottom %+v", bdPx(rb, w, 8, 0), bdPx(rb, w, 8, h-1))
	}
	// Sunken: the inverse.
	sb := bdRender(&Backdrop{Fill: fill, Bevel: BevelSunken}, w, h, th)
	if lum(bdPx(sb, w, 8, 0)) >= lum(bdPx(sb, w, 8, h-1)) {
		t.Errorf("sunken bevel: top %+v should be darker than bottom %+v", bdPx(sb, w, 8, 0), bdPx(sb, w, 8, h-1))
	}
	// Fill unset → the bevel derives its hi/lo from the theme background (no panic,
	// still a visible raised edge).
	db := bdRender(&Backdrop{Bevel: BevelRaised}, w, h, th)
	if lum(bdPx(db, w, 8, 0)) <= lum(bdPx(db, w, 8, h-1)) {
		t.Error("default-fill raised bevel: top should still be brighter than bottom")
	}
}

func TestBackdropDrawNoGrid(t *testing.T) {
	fill := painter.RGB(0x22, 0x22, 0x22)
	const w, h = 12, 12
	// Step <= 0 paints the fill only — no pixel carries the grid colour.
	buf := bdRender(NewBackdrop(fill, painter.RGB(0x99, 0x99, 0x99), 0), w, h, DefaultDark())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if got := bdPx(buf, w, x, y); got != fill {
				t.Fatalf("(%d,%d) = %v, want fill %v (grid must not paint)", x, y, got, fill)
			}
		}
	}
}

func TestBackdropDrawThemeDefaults(t *testing.T) {
	// Zero-value Fill/Grid fall back to theme.Background / theme.Border.
	theme := DefaultDark()
	const w, h, step = 16, 16, 8
	buf := bdRender(NewBackdrop(painter.RGBA{}, painter.RGBA{}, step), w, h, theme)

	if got := bdPx(buf, w, 3, 3); got != theme.Background {
		t.Errorf("interior (3,3) = %v, want theme.Background %v", got, theme.Background)
	}
	if got := bdPx(buf, w, step, 3); got != theme.Border {
		t.Errorf("grid line (%d,3) = %v, want theme.Border %v", step, got, theme.Border)
	}
}

func TestBackdropDrawEmptyBounds(t *testing.T) {
	theme := DefaultDark()
	// A zero-width backdrop paints nothing (W<=0 short-circuits)...
	zeroW := &Backdrop{Fill: painter.RGB(1, 2, 3), Step: 4}
	zeroW.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	p1 := painter.NewPixelPainter(make([]byte, 0), 0, 10)
	zeroW.Draw(p1, theme) // must not panic; nothing to assert on an empty buffer

	// ...and a zero-height backdrop (W>0, H<=0) exercises the second predicate.
	buf := make([]byte, 4*10*1)
	zeroH := &Backdrop{Fill: painter.RGB(1, 2, 3), Step: 4}
	zeroH.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 0})
	p2 := painter.NewPixelPainter(buf, 10, 1)
	zeroH.Draw(p2, theme)
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("empty-height backdrop painted byte %d = %d, want 0", i, v)
		}
	}
}

func TestBackdropHitTestTransparentByDefault(t *testing.T) {
	// A default (Interactive==false) full-cover Backdrop is event-transparent:
	// HitTest returns false even for a point squarely inside its bounds, so a
	// container routing by HitTest skips it and the click reaches the widget on
	// top.
	b := NewBackdrop(painter.RGB(0x11, 0x13, 0x1a), painter.RGB(0x17, 0x1a, 0x24), 8)
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	if b.HitTest(20, 15) {
		t.Fatalf("default Backdrop.HitTest(20,15) = true, want false (event-transparent)")
	}
	if b.HitTest(0, 0) {
		t.Fatalf("default Backdrop.HitTest(0,0) = true, want false (event-transparent)")
	}
}

func TestBackdropHitTestInteractive(t *testing.T) {
	// With Interactive set, the Backdrop hit-tests against its Bounds like a
	// normal widget: true inside, false outside.
	b := NewBackdrop(painter.RGBA{}, painter.RGBA{}, 0)
	b.Interactive = true
	b.SetBounds(Rect{X: 5, Y: 5, W: 20, H: 20})
	if !b.HitTest(10, 10) {
		t.Errorf("interactive Backdrop.HitTest(10,10) = false, want true (inside bounds)")
	}
	if b.HitTest(0, 0) {
		t.Errorf("interactive Backdrop.HitTest(0,0) = true, want false (outside bounds)")
	}
}

func TestBackdropHitTestRoutesThroughOverlay(t *testing.T) {
	// End-to-end: a default Backdrop as an Overlay layer under a Button lets a
	// click reach the button; an Interactive backdrop swallows it. The Overlay
	// routes to the topmost layer whose HitTest covers the point.
	const w, h = 40, 20
	clicked := false
	btn := NewButton("ok", func() { clicked = true })

	// Backdrop is the bottom layer, button the top layer; both full-cover.
	back := NewBackdrop(painter.RGBA{}, painter.RGBA{}, 0)
	ov := &Overlay{Layers: []Widget{back, btn}}
	ov.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	back.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	btn.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ov.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if !clicked {
		t.Fatalf("click did not reach the button over a transparent Backdrop")
	}

	// Now make the backdrop interactive: as the layer above the button it is the
	// topmost hit, so it consumes the click and the button never fires.
	clicked = false
	scrim := NewBackdrop(painter.RGBA{}, painter.RGBA{}, 0)
	scrim.Interactive = true
	ov2 := &Overlay{Layers: []Widget{btn, scrim}}
	ov2.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	scrim.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	btn.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ov2.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if clicked {
		t.Fatalf("interactive Backdrop scrim did not swallow the click")
	}
}

func TestBackdropRadius(t *testing.T) {
	fill := painter.RGB(0x30, 0x60, 0x90)
	th := DefaultLight()
	const w, h = 24, 24

	// Radius 0: a plain fill covers every pixel including the corners.
	square := bdRender(&Backdrop{Fill: fill}, w, h, th)
	if bdPx(square, w, 0, 0) != fill {
		t.Fatalf("square backdrop should fill the corner, got %v", bdPx(square, w, 0, 0))
	}

	// Radius > 0: the fill is a rounded rect — the centre is filled but the very
	// corner is left unpainted (zero ground), proving FillRoundRect was taken.
	round := bdRender(&Backdrop{Fill: fill, Radius: 8}, w, h, th)
	if bdPx(round, w, w/2, h/2) != fill {
		t.Fatalf("rounded backdrop should fill the centre, got %v", bdPx(round, w, w/2, h/2))
	}
	if bdPx(round, w, 0, 0) == fill {
		t.Fatal("rounded backdrop should NOT fill the sharp corner")
	}
}

func TestBackdropStroke(t *testing.T) {
	fill := painter.RGB(0x30, 0x60, 0x90)
	border := painter.RGB(0xC0, 0x20, 0x20)
	th := DefaultLight()
	const w, h = 24, 24

	// No stroke (A==0): the border colour never appears.
	plain := bdRender(&Backdrop{Fill: fill}, w, h, th)
	if hasColor(plain, w, border) {
		t.Fatal("a Backdrop with no Stroke must not paint a border")
	}

	// Stroke set (Width < 1 -> treated as 1): the border colour appears on the edge.
	bordered := bdRender(&Backdrop{Fill: fill, Radius: 6, Stroke: border, StrokeWidth: 0}, w, h, th)
	if !hasColor(bordered, w, border) {
		t.Fatal("a Backdrop with Stroke should paint its border")
	}
	// The centre stays the fill (the border is only an outline).
	if bdPx(bordered, w, w/2, h/2) != fill {
		t.Fatalf("bordered backdrop centre = %v, want fill %v", bdPx(bordered, w, w/2, h/2), fill)
	}
}

// An outline-only Backdrop is the focus-ring / drop-target case: the outline is
// painted and the content underneath survives. The test paints the ground first
// and then checks that exact ground is still there afterwards -- an assertion a
// backdrop that filled with theme.Background (what a zero Fill means) fails.
func TestBackdropNoFillLeavesTheContentShowing(t *testing.T) {
	ground := painter.RGB(0x20, 0x80, 0x40)
	ring := painter.RGB(0xC0, 0x20, 0x20)
	th := DefaultLight()
	const w, h = 24, 24

	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	p.FillRect(Rect{X: 0, Y: 0, W: w, H: h}, ground) // content already on screen

	b := &Backdrop{NoFill: true, Radius: 6, Stroke: ring, StrokeWidth: 2}
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b.Draw(p, th)

	if !hasColor(buf, w, ring) {
		t.Fatal("NoFill dropped the outline as well as the fill")
	}
	if got := bdPx(buf, w, w/2, h/2); got != ground {
		t.Errorf("centre = %v, want the untouched ground %v -- NoFill painted a fill", got, ground)
	}
	if got := bdPx(buf, w, w/2, 4); got != ground {
		t.Errorf("inside the ring = %v, want the untouched ground %v", got, ground)
	}

	// NoFill still honours the grid: it is an overlay decoration too. On its own
	// ground, so the ring above cannot be mistaken for a grid line.
	gbuf := make([]byte, 4*w*h)
	gp := painter.NewPixelPainter(gbuf, w, h)
	gp.FillRect(Rect{X: 0, Y: 0, W: w, H: h}, ground)
	gridded := &Backdrop{NoFill: true, Grid: ring, Step: 8}
	gridded.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	gridded.Draw(gp, th)
	if got := bdPx(gbuf, w, 1, 1); got != ground {
		t.Errorf("off-grid pixel = %v, want the untouched ground %v", got, ground)
	}
	if got := bdPx(gbuf, w, 8, 1); got != ring {
		t.Errorf("grid line = %v, want %v", got, ring)
	}
}
