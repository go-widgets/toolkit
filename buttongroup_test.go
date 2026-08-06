// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestButtonGroupMarksMembersFlat(t *testing.T) {
	a, b := NewButton("A", nil), NewButton("B", nil)
	g := NewButtonGroup(a, b)
	if !a.Flat || !b.Flat {
		t.Fatal("NewButtonGroup should mark every member Flat")
	}
	if len(g.Buttons) != 2 {
		t.Fatalf("Buttons = %d, want 2", len(g.Buttons))
	}
}

func TestButtonGroupLayoutHorizontal(t *testing.T) {
	g := NewButtonGroup(NewButton("1", nil), NewButton("2", nil), NewButton("3", nil))
	g.SetBounds(Rect{X: 10, Y: 5, W: 90, H: 30})
	// Equal thirds; members tile the group exactly (last absorbs the remainder).
	want := []Rect{{X: 10, Y: 5, W: 30, H: 30}, {X: 40, Y: 5, W: 30, H: 30}, {X: 70, Y: 5, W: 30, H: 30}}
	for i, w := range want {
		if got := g.Buttons[i].Bounds(); got != w {
			t.Fatalf("member %d bounds = %+v, want %+v", i, got, w)
		}
	}
	// Non-divisible width: the last member absorbs the remainder to fill exactly.
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20}) // 100/3 = 33, last = 34
	last := g.Buttons[2].Bounds()
	if last.X+last.W != 100 {
		t.Fatalf("last member right edge = %d, want 100 (fills bounds)", last.X+last.W)
	}
}

func TestButtonGroupLayoutVerticalAndEmpty(t *testing.T) {
	g := NewButtonGroup(NewButton("1", nil), NewButton("2", nil))
	g.Orientation = Vertical
	g.SetBounds(Rect{X: 2, Y: 4, W: 24, H: 41}) // 41/2 = 20, last = 21
	if a := g.Buttons[0].Bounds(); a != (Rect{X: 2, Y: 4, W: 24, H: 20}) {
		t.Fatalf("top member = %+v", a)
	}
	if b := g.Buttons[1].Bounds(); b.Y != 24 || b.Y+b.H != 45 {
		t.Fatalf("bottom member = %+v, want to fill to 45", b)
	}
	// Empty group: SetBounds is a safe no-op (no divide-by-zero).
	empty := NewButtonGroup()
	empty.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 10})
	if empty.Bounds() != (Rect{X: 0, Y: 0, W: 40, H: 10}) {
		t.Fatal("empty group should still record its bounds")
	}
}

func TestButtonGroupDrawHorizontal(t *testing.T) {
	const w, h = 100, 40
	theme := DefaultLight()
	g := NewButtonGroup(NewButton("A", nil), NewButton("B", nil))
	g.SetBounds(Rect{X: 4, Y: 4, W: 90, H: 30})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), theme)
	// The divider between the two members paints a Border pixel at member[1].X.
	div := g.Buttons[1].Bounds().X
	found := false
	for y := 8; y < 30; y++ {
		if pixelAt(buf, w, div, y) == theme.Border {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("horizontal group drew no divider between members")
	}
}

func TestButtonGroupDrawVertical(t *testing.T) {
	const w, h = 40, 100
	theme := DefaultLight()
	g := NewButtonGroup(NewButton("A", nil), NewButton("B", nil))
	g.Orientation = Vertical
	g.SetBounds(Rect{X: 4, Y: 4, W: 30, H: 90})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), theme)
	div := g.Buttons[1].Bounds().Y
	found := false
	for x := 8; x < 30; x++ {
		if pixelAt(buf, w, x, div) == theme.Border {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("vertical group drew no divider between members")
	}
}

func TestButtonGroupOnEventRoutesToMember(t *testing.T) {
	clicked := -1
	mk := func(i int) *Button { return NewButton("x", func() { clicked = i }) }
	g := NewButtonGroup(mk(0), mk(1), mk(2))
	g.SetBounds(Rect{X: 20, Y: 0, W: 90, H: 30}) // members at abs x 20/50/80, each 30 wide

	// A click over member 1's cell (group-local x maps to abs 50..80) fires it.
	g.OnEvent(Event{Kind: EventClick, X: 45, Y: 10}) // abs x = 20+45 = 65 → member 1
	if clicked != 1 {
		t.Fatalf("click routed to member %d, want 1", clicked)
	}
	g.OnEvent(Event{Kind: EventMouseUp, X: 45, Y: 10}) // release routes too (no panic)

	// A coordinate outside every member (past the group) hits nothing (no-op).
	clicked = -1
	g.OnEvent(Event{Kind: EventClick, X: 500, Y: 10})
	if clicked != -1 {
		t.Fatalf("out-of-range click should route nowhere, got %d", clicked)
	}
}
