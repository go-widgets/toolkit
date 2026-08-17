// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestDockStyles renders a dock with an active, a running and a resting item
// under each ship-with style and checks the style-specific chrome: the Windows
// accent underline, the Fluxbox raised/sunken bevel, and the Modern accent face.
func TestDockStyles(t *testing.T) {
	theme := DefaultLight()
	const W, H = 500, 40

	render := func(style DockStyle) (*AppDock, []byte) {
		d := NewAppDock(
			AppDockItem{Id: "a", Active: true},
			AppDockItem{Id: "b", Running: true},
			AppDockItem{Id: "c"},
		)
		d.Style = style
		d.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
		buf := makeSurface(W, H)
		d.Draw(newP(buf, W), theme)
		return d, buf
	}

	// Windows: the active and running items carry a centred accent underline; the
	// resting one does not.
	{
		d, buf := render(WindowsDockStyle{})
		rs := d.ItemRects()
		underline := func(i int) bool {
			r := rs[i]
			for x := r.X + r.W/2 - 2; x <= r.X+r.W/2+2; x++ {
				if pixelAt(buf, W, x, r.Y+r.H-1) == theme.Accent {
					return true
				}
			}
			return false
		}
		if !underline(0) {
			t.Error("Windows active item is missing its accent underline")
		}
		if !underline(1) {
			t.Error("Windows running item is missing its accent underline")
		}
		if underline(2) {
			t.Error("Windows resting item should carry no underline")
		}
	}

	// Fluxbox: a raised bevel at rest (bright top edge) vs a sunken bevel when
	// active (dark top edge), so the active item's top row is darker.
	{
		d, buf := render(BevelDockStyle{})
		rs := d.ItemRects()
		lum := func(c RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
		activeTop := pixelAt(buf, W, rs[0].X+rs[0].W/2, rs[0].Y)
		restTop := pixelAt(buf, W, rs[2].X+rs[2].W/2, rs[2].Y)
		if lum(activeTop) >= lum(restTop) {
			t.Errorf("Bevel active(sunken) top %+v should be darker than resting(raised) top %+v", activeTop, restTop)
		}
	}

	// Modern (set explicitly): the active face is Accent.
	{
		d, buf := render(ModernDockStyle{})
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
			t.Error("Modern active face (Accent) not painted")
		}
	}
}
