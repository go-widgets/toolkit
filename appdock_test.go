// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func dockIcon(hits *int) func(painter.Painter, Rect, RGBA) {
	return func(painter.Painter, Rect, RGBA) { *hits++ }
}

// TestAppDockRestingLayout checks the flat (un-magnified) layout: items run left
// to right at the resting width, one gap apart, vertically centred.
func TestAppDockRestingLayout(t *testing.T) {
	d := NewAppDock(AppDockItem{Id: "a"}, AppDockItem{Id: "b"})
	d.SetBounds(Rect{X: 10, Y: 0, W: 400, H: 40})
	rs := d.ItemRects() // cursor not inside → resting
	if len(rs) != 2 {
		t.Fatalf("ItemRects = %d, want 2", len(rs))
	}
	iw, g := scaled(AppDockItemW), scaled(AppDockGap)
	if rs[0].X != 10+g || rs[0].W != iw {
		t.Errorf("item0 = %+v, want X=%d W=%d", rs[0], 10+g, iw)
	}
	if rs[1].X != rs[0].X+iw+g {
		t.Errorf("item1 X = %d, want %d", rs[1].X, rs[0].X+iw+g)
	}
	if want := (40 - scaled(AppDockItemH)) / 2; rs[0].Y != want {
		t.Errorf("item0 Y = %d, want centred %d", rs[0].Y, want)
	}
	if d.HitTest(rs[1].X+2, rs[1].Y+2) != 1 {
		t.Error("HitTest inside item1 should be 1")
	}
	if d.HitTest(5, 5) != -1 {
		t.Error("HitTest in the end padding should miss")
	}
	if d.A11y().Role != RoleToolbar {
		t.Errorf("A11y role = %q, want toolbar", d.A11y().Role)
	}
}

// TestAppDockVariableWidth checks that a per-item Width is honoured by the
// resting layout, hit-testing and magnification (a host with window task buttons
// sized to their title needs this), while an unset Width keeps the default.
func TestAppDockVariableWidth(t *testing.T) {
	d := NewAppDock(
		AppDockItem{Id: "def"}, // default width
		AppDockItem{Id: "wide", Width: 200},
		AppDockItem{Id: "narrow", Width: 80},
	)
	d.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 40})
	g := scaled(AppDockGap)
	def := scaled(AppDockItemW)

	rs := d.ItemRects() // resting (no cursor)
	if rs[0].W != def || rs[1].W != 200 || rs[2].W != 80 {
		t.Fatalf("resting widths = %d/%d/%d, want %d/200/80", rs[0].W, rs[1].W, rs[2].W, def)
	}
	// Items follow one another by their own width + the gap.
	if rs[1].X != rs[0].X+def+g || rs[2].X != rs[1].X+200+g {
		t.Errorf("variable-width items not laid out sequentially: %+v", rs)
	}
	// Hit-testing honours the per-item width.
	if d.HitTest(rs[1].X+150, rs[1].Y+2) != 1 {
		t.Error("HitTest deep inside the wide item should return 1")
	}
	if d.HitTest(rs[2].X+120, rs[2].Y+2) == 2 {
		t.Error("HitTest past the narrow item's width should not match it")
	}

	// Magnification swells each item from its own resting width.
	d.SetCursor(rs[1].X+100, true) // over the wide item
	mg := d.ItemRects()
	if mg[1].W <= 200 {
		t.Errorf("magnified wide item W = %d, want > 200", mg[1].W)
	}
	for i := 1; i < len(mg); i++ {
		if mg[i].X < mg[i-1].X+mg[i-1].W {
			t.Errorf("variable-width magnified items overlap at %d", i)
		}
	}
}

// TestAppDockMagnifyActiveGuards checks every condition that disables the swell
// leaves the layout flat, and that the fully-enabled case swells.
func TestAppDockMagnifyActiveGuards(t *testing.T) {
	base := func() *AppDock {
		d := NewAppDock(AppDockItem{Id: "a"}, AppDockItem{Id: "b"}, AppDockItem{Id: "c"})
		d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 40})
		d.SetCursor(scaled(AppDockGap)+scaled(AppDockItemW)/2, true) // over item 0
		return d
	}
	restingW := scaled(AppDockItemW)

	// Enabled + cursor inside → item under the cursor is wider than resting.
	if got := base().ItemRects()[0].W; got <= restingW {
		t.Errorf("magnified item0 W = %d, want > resting %d", got, restingW)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*AppDock)
	}{
		{"magnify-off", func(d *AppDock) { d.Magnify = false }},
		{"cursor-outside", func(d *AppDock) { d.SetCursor(0, false) }},
		{"zero-radius", func(d *AppDock) { d.Radius = 0 }},
		{"unit-scale", func(d *AppDock) { d.MaxScale = 1 }},
	} {
		d := base()
		tc.mutate(d)
		if got := d.ItemRects()[0].W; got != restingW {
			t.Errorf("%s: item0 W = %d, want flat %d", tc.name, got, restingW)
		}
	}

	// Empty dock: no items, magnification inactive, no panic.
	empty := NewAppDock()
	empty.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
	empty.SetCursor(10, true)
	if len(empty.ItemRects()) != 0 {
		t.Error("empty dock should have no item rects")
	}
}

// TestAppDockMagnifyReflowAndAnchor checks the swollen row never overlaps and
// stays anchored under the cursor, and covers the gap (no-anchor) path.
func TestAppDockMagnifyReflowAndAnchor(t *testing.T) {
	d := NewAppDock(AppDockItem{Id: "a"}, AppDockItem{Id: "b"}, AppDockItem{Id: "c"})
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 40})

	// Cursor over item 1, off its centre so the anchor shift is non-zero.
	iw, g := scaled(AppDockItemW), scaled(AppDockGap)
	item1X := g + iw + g
	d.SetCursor(item1X+3, true)
	rs := d.ItemRects()
	// No overlap: each item starts at or after the previous item's right edge.
	for i := 1; i < len(rs); i++ {
		if rs[i].X < rs[i-1].X+rs[i-1].W {
			t.Errorf("item %d overlaps its neighbour: %+v after %+v", i, rs[i], rs[i-1])
		}
	}
	// The pointer still lands inside item 1 after the reflow (anchor held).
	if d.HitTest(item1X+3, rs[1].Y+2) != 1 {
		t.Error("cursor-anchored item 1 should still be under the pointer")
	}

	// Cursor in the far end padding (no item under it) → the anchor loop finds
	// nothing and the row is not shifted; still no overlap.
	d.SetCursor(590, true)
	rs = d.ItemRects()
	for i := 1; i < len(rs); i++ {
		if rs[i].X < rs[i-1].X+rs[i-1].W {
			t.Errorf("gap-cursor item %d overlaps", i)
		}
	}
}

// TestAppDockBump checks the raised-cosine falloff endpoints and shoulder.
func TestAppDockBump(t *testing.T) {
	if appDockBump(-0.5) != 1 || appDockBump(0) != 1 {
		t.Error("bump at/below the cursor must be 1")
	}
	if appDockBump(1) != 0 || appDockBump(2) != 0 {
		t.Error("bump at/beyond the edge must be 0")
	}
	if m := appDockBump(0.5); m <= 0 || m >= 1 {
		t.Errorf("bump(0.5) = %v, want in (0,1)", m)
	}
}

// TestAppDockClip covers the label truncation helper's four outcomes.
func TestAppDockClip(t *testing.T) {
	d := NewAppDock()
	if d.clip("anything", 0) != "" {
		t.Error("clip with no width must be empty")
	}
	if d.clip("hi", 10_000) != "hi" {
		t.Error("clip that fits must return the text unchanged")
	}
	trunc := d.clip("a very long label indeed", d.textWidth("a very")+2)
	if trunc == "" || []rune(trunc)[len([]rune(trunc))-1] != '…' {
		t.Errorf("clip that overflows must end in an ellipsis, got %q", trunc)
	}
	if d.clip("x", 1) != "" {
		t.Error("clip too narrow for even the ellipsis must be empty")
	}
}

// TestAppDockDraw renders every item variant and checks the ground + an active
// face paint, exercising the drawItem branches (active, running, badge, clipped
// label, icon-less, magnified glyph clamp).
func TestAppDockDraw(t *testing.T) {
	theme := DefaultLight()
	icons := 0
	d := NewAppDock(
		AppDockItem{Id: "a", Label: "Files but with a very long label", Icon: dockIcon(&icons), Running: true, Active: true, Badge: 3},
		AppDockItem{Id: "b"}, // icon-only, no label, resting
	)
	W, H := 500, 40
	d.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})

	// Resting render: items are centred (H=28 in a 40 bar), so y=1 is above them.
	buf := makeSurface(W, H)
	d.Draw(newP(buf, W), theme)

	if icons == 0 {
		t.Error("the item's Icon hook was never called")
	}
	if got := pixelAt(buf, W, 1, 1); got != theme.SurfaceAlt {
		t.Errorf("ground pixel = %+v, want SurfaceAlt %+v", got, theme.SurfaceAlt)
	}
	// The active item's face is Accent: scan its rect for an Accent pixel.
	r0 := d.ItemRects()[0]
	found := false
	for y := r0.Y + 2; y < r0.Y+r0.H-2 && !found; y++ {
		for x := r0.X + 1; x < r0.X+r0.W-1; x++ {
			if pixelAt(buf, W, x, y) == theme.Accent {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("active item face (Accent) not painted")
	}

	// Magnified render: the cursor over item 0 with a large peak swells its glyph
	// past the face, exercising the swell + glyph-size clamp. No panic, and the
	// item is now wider than its resting width.
	d.MaxScale = 3
	d.SetCursor(scaled(AppDockGap)+scaled(AppDockItemW)/2, true)
	d.Draw(newP(makeSurface(W, H), W), theme)
	if d.ItemRects()[0].W <= scaled(AppDockItemW) {
		t.Error("magnified item 0 should be wider than resting")
	}

	// Zero-size dock is a no-op (early return), no panic.
	z := NewAppDock(AppDockItem{Id: "a"})
	z.SetBounds(Rect{})
	z.Draw(newP(makeSurface(4, 4), 4), theme)
}

// TestAppDockOnEvent covers hover tracking and click activation.
func TestAppDockOnEvent(t *testing.T) {
	activated := -1
	d := NewAppDock(AppDockItem{Id: "a"}, AppDockItem{Id: "b"})
	d.SetBounds(Rect{X: 10, Y: 0, W: 400, H: 40})
	d.OnActivate = func(i int) { activated = i }

	// A move inside the bounds marks the cursor inside; one outside clears it.
	d.OnEvent(Event{Kind: EventMouseMove, X: 20, Y: 10})
	if !d.cursorInside {
		t.Error("move inside should mark the cursor inside")
	}
	d.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: 10})
	if d.cursorInside {
		t.Error("move outside should clear the cursor-inside flag")
	}

	// A click on item 1 activates it (event coords are widget-local).
	r1 := d.ItemRects()[1]
	d.OnEvent(Event{Kind: EventClick, X: r1.X - 10 + 2, Y: r1.Y + 2})
	if activated != 1 {
		t.Errorf("click activated %d, want 1", activated)
	}

	// A click that hits no item does not activate.
	activated = -1
	d.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	if activated != -1 {
		t.Error("click in the padding should not activate")
	}

	// With no OnActivate wired a click is a silent no-op (nil-safe), as is an
	// unrelated event kind.
	d.OnActivate = nil
	d.OnEvent(Event{Kind: EventClick, X: r1.X - 10 + 2, Y: r1.Y + 2})
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}
