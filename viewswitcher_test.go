// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- API surface ---------------------------------------------------------

func TestViewSwitcherConstants(t *testing.T) {
	if ViewSwitcherH != 32 || ViewSwitcherPadX != 12 {
		t.Fatalf("constants drifted: H=%d PadX=%d", ViewSwitcherH, ViewSwitcherPadX)
	}
}

func TestNewViewSwitcherClamps(t *testing.T) {
	// Empty views: current forced to 0.
	if v := NewViewSwitcher(nil, 7); v.Current != 0 {
		t.Fatalf("empty views current = %d, want 0", v.Current)
	}
	// Negative current clamped to 0.
	if v := NewViewSwitcher([]string{"A", "B"}, -5); v.Current != 0 {
		t.Fatalf("negative current = %d, want 0", v.Current)
	}
	// Overshoot clamped to len-1.
	if v := NewViewSwitcher([]string{"A", "B", "C"}, 99); v.Current != 2 {
		t.Fatalf("overshoot current = %d, want 2", v.Current)
	}
	// In-range preserved.
	if v := NewViewSwitcher([]string{"A", "B", "C"}, 1); v.Current != 1 {
		t.Fatalf("in-range current = %d, want 1", v.Current)
	}
}

// --- Draw: no views ------------------------------------------------------

func TestViewSwitcherDrawEmpty(t *testing.T) {
	const w, h = 100, 32
	theme := DefaultLight()
	v := NewViewSwitcher(nil, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	// Background is SurfaceAlt.
	if got := pixelAt(buf, w, 50, 5); got != theme.SurfaceAlt {
		t.Fatalf("empty background = %+v, want SurfaceAlt", got)
	}
	// Bottom border in Border.
	if got := pixelAt(buf, w, 50, h-1); got != theme.Border {
		t.Fatalf("bottom border = %+v, want Border", got)
	}
	// No OnSurface / Accent ink anywhere.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := pixelAt(buf, w, x, y)
			if p == theme.OnSurface {
				t.Fatalf("unexpected OnSurface at (%d,%d)", x, y)
			}
		}
	}
}

// --- Draw: single view (current is the only segment) --------------------

func TestViewSwitcherDrawSingle(t *testing.T) {
	const w, h = 100, 32
	theme := DefaultLight()
	v := NewViewSwitcher([]string{"OK"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	// The active fill blankets the whole strip except the bottom border.
	if got := pixelAt(buf, w, 50, 5); got != theme.Accent {
		t.Fatalf("single-view active fill = %+v, want Accent", got)
	}
	// Bottom border still visible (fillRect after the segment fills).
	if got := pixelAt(buf, w, 50, h-1); got != theme.Border {
		t.Fatalf("bottom border = %+v, want Border", got)
	}
	// Ink lands in accentInk fallback = theme.Background (nil Extra map).
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected accent-inverted ink somewhere in single-view draw")
	}
}

// --- Draw: multiple views -----------------------------------------------

func TestViewSwitcherDrawMulti(t *testing.T) {
	const w, h = 240, 32
	theme := DefaultLight()
	v := NewViewSwitcher([]string{"A", "B", "C"}, 1)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	segW := w / 3
	// Segment 0 (unselected): SurfaceAlt fill.
	if got := pixelAt(buf, w, segW/2, 5); got != theme.SurfaceAlt {
		t.Fatalf("segment 0 fill = %+v, want SurfaceAlt", got)
	}
	// Segment 1 (selected): Accent fill.
	if got := pixelAt(buf, w, segW+segW/2, 5); got != theme.Accent {
		t.Fatalf("segment 1 fill = %+v, want Accent", got)
	}
	// Segment 2 (unselected): SurfaceAlt fill.
	if got := pixelAt(buf, w, 2*segW+segW/2, 5); got != theme.SurfaceAlt {
		t.Fatalf("segment 2 fill = %+v, want SurfaceAlt", got)
	}
}

// --- Draw: dark theme ---------------------------------------------------

func TestViewSwitcherDrawDark(t *testing.T) {
	const w, h = 120, 32
	theme := DefaultDark()
	v := NewViewSwitcher([]string{"A", "B"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, w/4, 5); got != theme.Accent {
		t.Fatalf("dark active fill = %+v, want Accent", got)
	}
}

// --- Draw: Extra["OnAccent"] override -----------------------------------

func TestViewSwitcherDrawUsesOnAccentOverride(t *testing.T) {
	const w, h = 120, 32
	theme := DefaultLight()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	v := NewViewSwitcher([]string{"HI"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == custom {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("custom OnAccent ink missing from active segment")
	}
}

// --- Draw: Extra map without OnAccent falls back to Background ----------

func TestViewSwitcherDrawExtraWithoutOnAccentFallsBack(t *testing.T) {
	const w, h = 120, 32
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{"OtherKey": RGB(1, 2, 3)}
	v := NewViewSwitcher([]string{"HI"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	// Ink should have used theme.Background (fallback).
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("fallback Background ink missing from active segment")
	}
}

// --- Draw: zero-width bounds --------------------------------------------

func TestViewSwitcherDrawZeroWidthNoPanic(t *testing.T) {
	theme := DefaultLight()
	v := NewViewSwitcher([]string{"A", "B"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 0, H: ViewSwitcherH})
	v.Draw(newP(makeSurface(1, ViewSwitcherH), 1), theme)
}

// --- OnEvent: selects segment + fires OnChange --------------------------

func TestViewSwitcherClickSelectsSegment(t *testing.T) {
	got := -1
	v := NewViewSwitcher([]string{"A", "B", "C"}, 0)
	v.OnChange = func(i int) { got = i }
	v.SetBounds(Rect{X: 0, Y: 0, W: 240, H: ViewSwitcherH})
	// segW = 80; click at x=100 -> segment 1.
	v.OnEvent(Event{Kind: EventClick, X: 100, Y: 5})
	if v.Current != 1 || got != 1 {
		t.Fatalf("after click: Current=%d got=%d", v.Current, got)
	}
	// Click on segment 2.
	v.OnEvent(Event{Kind: EventClick, X: 200, Y: 5})
	if v.Current != 2 || got != 2 {
		t.Fatalf("after 2nd click: Current=%d got=%d", v.Current, got)
	}
}

// --- OnEvent: nil OnChange handler --------------------------------------

func TestViewSwitcherClickNilOnChangeNoPanic(t *testing.T) {
	v := NewViewSwitcher([]string{"A", "B"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 200, H: ViewSwitcherH})
	v.OnEvent(Event{Kind: EventClick, X: 150, Y: 5})
	if v.Current != 1 {
		t.Fatalf("Current = %d, want 1", v.Current)
	}
}

// --- OnEvent: empty views + click ---------------------------------------

func TestViewSwitcherClickEmptyViewsNoOp(t *testing.T) {
	v := NewViewSwitcher(nil, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: ViewSwitcherH})
	v.OnEvent(Event{Kind: EventClick, X: 50, Y: 5}) // must not panic
}

// --- OnEvent: zero-width bounds + click ---------------------------------

func TestViewSwitcherClickZeroWidthNoOp(t *testing.T) {
	v := NewViewSwitcher([]string{"A", "B"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 0, H: ViewSwitcherH})
	v.OnEvent(Event{Kind: EventClick, X: 0, Y: 5})
	if v.Current != 0 {
		t.Fatal("zero-width click should not mutate Current")
	}
}

// --- OnEvent: out-of-range X --------------------------------------------

func TestViewSwitcherClickBelowZero(t *testing.T) {
	v := NewViewSwitcher([]string{"A", "B", "C"}, 2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 240, H: ViewSwitcherH})
	// segW = 80; ev.X = -100 -> idx = -100/80 = -1 -> return without mutating.
	v.OnEvent(Event{Kind: EventClick, X: -100, Y: 5})
	if v.Current != 2 {
		t.Fatal("negative-X click should not mutate Current")
	}
}

func TestViewSwitcherClickBeyondEnd(t *testing.T) {
	v := NewViewSwitcher([]string{"A", "B", "C"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 240, H: ViewSwitcherH})
	// segW = 80; ev.X = 400 -> idx = 5 -> return without mutating.
	v.OnEvent(Event{Kind: EventClick, X: 400, Y: 5})
	if v.Current != 0 {
		t.Fatal("beyond-end click should not mutate Current")
	}
}

// --- OnEvent: non-click event -------------------------------------------

func TestViewSwitcherIgnoresNonClick(t *testing.T) {
	v := NewViewSwitcher([]string{"A", "B"}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: 200, H: ViewSwitcherH})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if v.Current != 0 {
		t.Fatal("KeyDown should not mutate Current")
	}
}
