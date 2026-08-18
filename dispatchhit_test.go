// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A container routes a pointer event by ASKING the child whether it was hit,
// not by testing the child's drawn rectangle.
//
// The difference is the whole touch-density floor. A widget under DensityTouch
// reports a hit rect clamped up to MinHitTarget and centred on its unchanged
// pixels — that is what makes a small control reachable with a fingertip. A
// parent that tested Bounds instead dropped the tap before the child was ever
// asked, so the floor every interactive widget computes was unreachable in the
// one place it matters: inside a layout.

// smallButtonInABox returns a box holding a button far smaller than the touch
// floor, plus the button and a point inside its hit rect but outside its pixels.
func smallButtonInABox(t *testing.T) (*VBox, *Button, *int, int, int) {
	t.Helper()
	fired := 0
	btn := NewButton("ok", func() { fired++ })
	box := NewVBox()
	box.Append(btn)
	box.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	btn.SetBounds(Rect{X: 80, Y: 90, W: 20, H: 20})

	b, hr := btn.Bounds(), btn.HitRect()
	px, py := b.X-8, b.Y+10
	if !hr.Contains(px, py) || b.Contains(px, py) {
		t.Fatalf("test point (%d,%d) must be inside the hit rect %+v and outside the bounds %+v",
			px, py, hr, b)
	}
	return box, btn, &fired, px, py
}

func TestContainerRoutesByHitTestNotBounds(t *testing.T) {
	SetDensity(DensityTouch)
	defer SetDensity(DensityCompact)

	box, btn, fired, px, py := smallButtonInABox(t)
	if got, want := btn.HitRect().W, MinHitTarget(); got != want {
		t.Fatalf("hit rect width %d, want the floor %d", got, want)
	}
	box.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	if *fired != 1 {
		t.Fatalf("the tap fired %d times: a container must route to the child's HitTest, "+
			"or the touch floor is unreachable inside any layout", *fired)
	}
}

func TestCompactDensityRoutesExactlyAsBefore(t *testing.T) {
	// With no floor the hit rect IS the drawn rectangle, so routing is
	// byte-for-byte what testing Bounds used to do: a tap outside the pixels
	// misses, a tap inside lands. (The helper above cannot be reused here —
	// under DensityCompact there is no point inside the hit rect and outside
	// the bounds, because they are the same rectangle.)
	SetDensity(DensityCompact)
	fired := 0
	btn := NewButton("ok", func() { fired++ })
	box := NewVBox()
	box.Append(btn)
	box.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	btn.SetBounds(Rect{X: 80, Y: 90, W: 20, H: 20})
	if btn.HitRect() != btn.Bounds() {
		t.Fatalf("compact hit rect %+v should equal the bounds %+v", btn.HitRect(), btn.Bounds())
	}

	box.OnEvent(Event{Kind: EventClick, X: 72, Y: 100})
	if fired != 0 {
		t.Fatalf("a tap outside a compact button fired %d times, want 0", fired)
	}
	box.OnEvent(Event{Kind: EventClick, X: 90, Y: 100})
	if fired != 1 {
		t.Fatalf("a tap inside the button fired %d times, want 1", fired)
	}
}

func TestPlainContainerRoutesByHitTestToo(t *testing.T) {
	// Container is the other dispatcher, and it keeps skipping zero-area
	// children: an invisible widget must not become reachable just because a
	// density floor would give it a hit rect.
	SetDensity(DensityTouch)
	defer SetDensity(DensityCompact)

	fired := 0
	btn := NewButton("ok", func() { fired++ })
	ghost := NewButton("ghost", func() { fired += 100 })
	c := NewContainer(nil)
	c.AddWidget(ghost)
	c.AddWidget(btn)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	// A nil layout leaves the children where they are put.
	btn.SetBounds(Rect{X: 80, Y: 90, W: 20, H: 20})
	ghost.SetBounds(Rect{})

	c.OnEvent(Event{Kind: EventClick, X: 72, Y: 100})
	if fired != 1 {
		t.Fatalf("fired=%d, want 1: the container should route to the real button "+
			"and never to a zero-area one", fired)
	}
}
