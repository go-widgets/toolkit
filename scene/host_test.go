// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Date: 2026-08-09
package scene

import (
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// damageRenderer mirrors the structural capability github.com/go-widgets/window
// type-asserts. HostRoot must satisfy it (and be a plain widget too).
type damageRenderer interface {
	RenderDamaged(p painter.Painter, th *toolkit.Theme) []toolkit.Rect
}

var (
	_ toolkit.Widget = (*HostRoot)(nil)
	_ damageRenderer = (*HostRoot)(nil)
	_ SelfDrawer     = (*bgContainer)(nil)
	_ childProvider  = (*bgContainer)(nil)
)

// fullRepaint reproduces a damage-UNAWARE host's frame: clear to the theme
// background, then draw the application root — the exact reference the
// incremental path must match pixel for pixel.
func fullRepaint(buf []byte, p painter.Painter, appRoot toolkit.Widget, full Rect, th *toolkit.Theme) {
	fill(buf, 0)
	p.FillRect(full, th.Background)
	appRoot.SetBounds(full)
	appRoot.Draw(p, th)
}

// TestHostRootPixelIdentityOverEventLog is the window-seam correctness gate: a
// buffer updated ONLY through HostRoot.RenderDamaged (incremental, damage-clipped)
// must stay byte-identical to a fresh full-surface repaint (fill-then-draw) over
// a scripted event log — the exact contract the window backend relies on when it
// switches from full-surface to incremental present.
func TestHostRootPixelIdentityOverEventLog(t *testing.T) {
	const W, H, rows, cols = 160, 120, 6, 8
	theme := th()
	full := Rect{X: 0, Y: 0, W: W, H: H}

	app, cells := buildDense(W, H, rows, cols)
	root := NewHostRoot(app)
	root.SetBounds(full)

	// A = persisted, updated only by incremental RenderDamaged.
	bufA, pa := newBuf(W, H)
	// B = fresh full repaint each step, built directly from the same app tree.
	bufB, pb := newBuf(W, H)

	// Initial frame.
	root.RenderDamaged(pa, theme)
	fullRepaint(bufB, pb, app, full, theme)
	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("initial frame: incremental buffer != full repaint")
	}

	palette := []toolkit.RGBA{red, green, blue, grey, red, green}
	log := []int{0, 7, 47, 23, 11, 0, 46, 47, 30, 15, 8, 8, 40, 12}
	for step, idx := range log {
		c := cells[idx]
		c.col = palette[(step+idx)%len(palette)]
		root.Invalidate(c)
		rects := root.RenderDamaged(pa, theme)

		// The incremental frame must touch only a small area, never the whole
		// surface (that would defeat the point and hide an over-damage bug).
		var area int
		for _, r := range rects {
			area += r.W * r.H
		}
		if area == 0 || area >= W*H {
			t.Fatalf("step %d (cell %d): damage area %d px, want small non-zero (< %d)", step, idx, area, W*H)
		}

		fullRepaint(bufB, pb, app, full, theme)
		if fnv1a(bufA) != fnv1a(bufB) {
			t.Fatalf("step %d (cell %d): incremental buffer diverged from full repaint", step, idx)
		}
	}
}

// TestHostRootDrawMatchesRenderDamaged proves the full-surface Draw path (used
// by a damage-unaware host) paints the same pixels as an initial full-seed
// RenderDamaged — so a plain host and a damage-aware host agree.
func TestHostRootDrawMatchesRenderDamaged(t *testing.T) {
	const W, H = 100, 80
	theme := th()
	full := Rect{X: 0, Y: 0, W: W, H: H}

	app1, _ := buildDense(W, H, 4, 5)
	r1 := NewHostRoot(app1)
	r1.SetBounds(full)
	bufA, pa := newBuf(W, H)
	r1.RenderDamaged(pa, theme)

	app2, _ := buildDense(W, H, 4, 5)
	r2 := NewHostRoot(app2)
	r2.SetBounds(full)
	bufB, pb := newBuf(W, H)
	r2.Draw(pb, theme)

	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("HostRoot.Draw != initial RenderDamaged")
	}
}

// TestHostRootResizeIsFullDamage proves a size change routes a FULL repaint
// through the SAME incremental path: RenderDamaged after SetBounds(newSize)
// returns damage covering the whole new surface and repaints the fresh buffer
// completely (pixel-identical to a from-scratch full repaint at the new size).
func TestHostRootResizeIsFullDamage(t *testing.T) {
	theme := th()
	app, cells := buildDense(120, 90, 3, 4)
	root := NewHostRoot(app)

	// First lay out + present at the initial size.
	root.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90})
	_, p0 := newBuf(120, 90)
	root.RenderDamaged(p0, theme)

	// Recolour a cell but DO NOT present it — the pending small damage must be
	// subsumed by the resize's full damage.
	cells[0].col = red
	root.Invalidate(cells[0])

	// Resize larger.
	const W, H = 200, 150
	full := Rect{X: 0, Y: 0, W: W, H: H}
	root.SetBounds(full)

	bufA, pa := newBuf(W, H)
	rects := root.RenderDamaged(pa, theme)
	var area int
	for _, r := range rects {
		area += r.W * r.H
	}
	if area < W*H {
		t.Fatalf("resize damage area %d px, want >= full surface %d px", area, W*H)
	}

	bufB, pb := newBuf(W, H)
	fullRepaint(bufB, pb, app, full, theme)
	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("resize: incremental buffer != full repaint at new size")
	}
}

// TestHostRootSetBoundsNoOp proves an unchanged SetBounds neither invalidates
// nor re-lays-out (no damage accrues).
func TestHostRootSetBoundsNoOp(t *testing.T) {
	theme := th()
	app, _ := buildDense(60, 40, 2, 3)
	root := NewHostRoot(app)
	b := Rect{X: 0, Y: 0, W: 60, H: 40}
	root.SetBounds(b)

	_, p := newBuf(60, 40)
	root.RenderDamaged(p, theme) // consume the initial full-surface damage

	root.SetBounds(b) // identical bounds: must be a no-op
	if rects := root.RenderDamaged(p, theme); len(rects) != 0 {
		t.Fatalf("no-op SetBounds produced damage %v, want none", rects)
	}
}

// TestHostRootDelegation covers the pass-through widget methods and the Scene
// accessor.
func TestHostRootDelegation(t *testing.T) {
	app := newCell(Rect{X: 0, Y: 0, W: 50, H: 40}, red)
	root := NewHostRoot(app)
	root.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})

	if !root.HitTest(10, 10) {
		t.Fatal("HitTest inside bounds = false")
	}
	if root.HitTest(1000, 1000) {
		t.Fatal("HitTest outside bounds = true")
	}
	if root.Bounds() != (Rect{X: 0, Y: 0, W: 50, H: 40}) {
		t.Fatalf("Bounds = %v", root.Bounds())
	}
	if root.Scene() == nil {
		t.Fatal("Scene() nil")
	}
	// OnEvent + Invalidate must not panic and must reach the child; a cell
	// ignores events, so we just assert the calls are wired.
	root.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: 5, Y: 5})
	root.Invalidate(app)

	// Invalidate then render must produce exactly the cell's damage.
	theme := th()
	_, p := newBuf(50, 40)
	root.RenderDamaged(p, theme) // drain initial
	app.col = green
	root.Invalidate(app)
	rects := root.RenderDamaged(p, theme)
	if len(rects) != 1 || rects[0] != (Rect{X: 0, Y: 0, W: 50, H: 40}) {
		t.Fatalf("cell damage = %v, want one full-cell rect", rects)
	}
}
