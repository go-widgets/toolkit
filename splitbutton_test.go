// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestSplitButtonNewDefaults(t *testing.T) {
	s := NewSplitButton("Go", nil)
	if s.Label != "Go" {
		t.Fatalf("Label = %q, want %q", s.Label, "Go")
	}
	if !s.Arrow {
		t.Fatal("Arrow should default to true")
	}
	if s.OnClick != nil {
		t.Fatal("OnClick should default to the caller-supplied value (nil here)")
	}
	if s.OnArrow != nil {
		t.Fatal("OnArrow should default to nil")
	}
}

func TestSplitButtonNewEmptyLabel(t *testing.T) {
	s := NewSplitButton("", nil)
	if s.Label != "" || !s.Arrow {
		t.Fatalf("empty-label constructor bad defaults: %+v", *s)
	}
}

// --- Draw: Arrow = true --------------------------------------------------

func TestSplitButtonDrawWithArrow(t *testing.T) {
	const w, h = 100, 20
	theme := DefaultLight()
	s := NewSplitButton("OK", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Main-slot fill: sample well before the centred label glyphs.
	if got := pixelAt(buf, w, 5, 5); got != theme.Accent {
		t.Fatalf("main slot fill = %+v, want Accent %+v", got, theme.Accent)
	}
	// Separator at the main/arrow boundary (X = w - SplitButtonArrowW).
	if got := pixelAt(buf, w, w-SplitButtonArrowW, 10); got != theme.Border {
		t.Fatalf("separator pixel = %+v, want Border %+v", got, theme.Border)
	}
	// Arrow-slot fill: sample far from the centred "v" glyph.
	if got := pixelAt(buf, w, w-2, h-2); got != theme.Accent {
		t.Fatalf("arrow slot fill = %+v, want Accent %+v", got, theme.Accent)
	}
}

// --- Draw: Arrow = false -------------------------------------------------

func TestSplitButtonDrawNoArrow(t *testing.T) {
	const w, h = 100, 20
	theme := DefaultDark()
	s := NewSplitButton("Go", nil)
	s.Arrow = false
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// No separator: the pixel at what would have been the boundary is
	// part of the full-width Accent fill.
	if got := pixelAt(buf, w, w-SplitButtonArrowW, 3); got != theme.Accent {
		t.Fatalf("Arrow=false pixel at boundary = %+v, want Accent %+v", got, theme.Accent)
	}
	// Right-edge sample: still Accent.
	if got := pixelAt(buf, w, w-2, 2); got != theme.Accent {
		t.Fatalf("Arrow=false right-edge = %+v, want Accent %+v", got, theme.Accent)
	}
}

// --- Draw: empty label ---------------------------------------------------

func TestSplitButtonDrawEmptyLabel(t *testing.T) {
	const w, h = 80, 20
	theme := DefaultLight()
	s := NewSplitButton("", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Main slot still fills; there just isn't a centred label.
	if got := pixelAt(buf, w, 2, 2); got != theme.Accent {
		t.Fatalf("empty-label main fill = %+v, want Accent", got)
	}
}

// --- Draw: OnAccent override ---------------------------------------------

func TestSplitButtonDrawUsesOnAccentFromExtra(t *testing.T) {
	const w, h = 100, 20
	theme := DefaultLight()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	s := NewSplitButton("OK", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in split button")
	}
}

// --- Draw: empty Extra falls back to Background --------------------------

func TestSplitButtonDrawEmptyExtraFallsBackToBackground(t *testing.T) {
	const w, h = 100, 20
	theme := DefaultDark()
	theme.Extra = map[string]RGBA{} // non-nil but no "OnAccent" key
	s := NewSplitButton("OK", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no Background-coloured glyph pixel found (fallback path)")
	}
}

// --- Draw: zero-width bounds is a no-op ----------------------------------

func TestSplitButtonDrawZeroWidth(t *testing.T) {
	theme := DefaultLight()
	s := NewSplitButton("X", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	buf := makeSurface(10, 20)
	s.Draw(newP(buf, 10), theme) // must not panic
}

// --- OnEvent -------------------------------------------------------------

func TestSplitButtonClickMainSlotFiresOnClick(t *testing.T) {
	clicked := 0
	arrowed := 0
	s := NewSplitButton("X", func() { clicked++ })
	s.OnArrow = func() { arrowed++ }
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if clicked != 1 || arrowed != 0 {
		t.Fatalf("main click: clicked=%d arrowed=%d, want 1/0", clicked, arrowed)
	}
}

func TestSplitButtonClickArrowSlotFiresOnArrow(t *testing.T) {
	clicked := 0
	arrowed := 0
	s := NewSplitButton("X", func() { clicked++ })
	s.OnArrow = func() { arrowed++ }
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 90, Y: 5})
	if clicked != 0 || arrowed != 1 {
		t.Fatalf("arrow click: clicked=%d arrowed=%d, want 0/1", clicked, arrowed)
	}
}

func TestSplitButtonClickArrowFalseRoutesToOnClick(t *testing.T) {
	clicked := 0
	arrowed := 0
	s := NewSplitButton("X", func() { clicked++ })
	s.OnArrow = func() { arrowed++ }
	s.Arrow = false
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	// X=90 would have hit the arrow slot; Arrow=false routes to OnClick.
	s.OnEvent(Event{Kind: EventClick, X: 90, Y: 5})
	if clicked != 1 || arrowed != 0 {
		t.Fatalf("Arrow=false right-region click: clicked=%d arrowed=%d, want 1/0", clicked, arrowed)
	}
}

func TestSplitButtonClickNilOnClickNoPanic(t *testing.T) {
	s := NewSplitButton("X", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
}

func TestSplitButtonClickNilOnArrowNoPanic(t *testing.T) {
	s := NewSplitButton("X", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	// Arrow slot click with OnArrow left nil.
	s.OnEvent(Event{Kind: EventClick, X: 90, Y: 5})
}

func TestSplitButtonIgnoresNonClickEvents(t *testing.T) {
	fired := 0
	s := NewSplitButton("X", func() { fired++ })
	s.OnArrow = func() { fired++ }
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	s.OnEvent(Event{Kind: EventChar, Code: "a"})
	if fired != 0 {
		t.Fatalf("non-click fired handlers: %d", fired)
	}
}
