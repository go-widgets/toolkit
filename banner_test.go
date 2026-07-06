// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewBannerDefaults(t *testing.T) {
	b := NewBanner("heads up")
	if b.Text != "heads up" {
		t.Fatalf("Text = %q", b.Text)
	}
	if !b.Revealed {
		t.Fatal("fresh Banner must be Revealed=true")
	}
	if b.ButtonLabel != "" {
		t.Fatalf("ButtonLabel = %q, want empty", b.ButtonLabel)
	}
	if b.OnAction != nil {
		t.Fatal("fresh Banner OnAction must be nil")
	}
}

// --- Draw: !Revealed is a no-op ----------------------------------------

func TestBannerDrawWhenNotRevealedNoOp(t *testing.T) {
	b := NewBanner("x")
	b.Revealed = false
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	surf := makeSurface(200, 30)
	before := make([]byte, len(surf))
	copy(before, surf)
	b.Draw(newP(surf, 200), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw on !Revealed Banner touched byte %d", i)
		}
	}
}

// --- Draw: revealed strip in Accent + text ink from accentInk ---------

func TestBannerDrawRevealedPaintsAccent(t *testing.T) {
	b := NewBanner("hi")
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	theme := DefaultLight()
	buf := makeSurface(200, 30)
	b.Draw(newP(buf, 200), theme)
	// Sample well inside the strip past the text (near right edge, no
	// button rendered so the fill dominates).
	if pixelAt(buf, 200, 180, 15) != theme.Accent {
		t.Fatalf("strip fill = %+v, want Accent", pixelAt(buf, 200, 180, 15))
	}
}

// --- Draw: with a button, both the strip and the button paint ---------

func TestBannerDrawRevealedWithButton(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	theme := DefaultLight()
	buf := makeSurface(200, 30)
	b.Draw(newP(buf, 200), theme)
	// The strip's leading edge is still Accent.
	if pixelAt(buf, 200, 5, 15) != theme.Accent {
		t.Fatal("leading-edge strip fill != Accent with button present")
	}
	// A stroke must have landed somewhere inside the trailing 60 px
	// (button rect area).
	ink := accentInk(theme)
	found := false
	for y := 0; y < 30 && !found; y++ {
		for x := 140; x < 200; x++ {
			if pixelAt(buf, 200, x, y) == ink {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no button-border pixel found near trailing edge")
	}
}

// --- Draw: dark theme + Extra OnAccent override -----------------------

func TestBannerDrawUsesOnAccentFromExtra(t *testing.T) {
	b := NewBanner("XYZ")
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	theme := DefaultDark()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	buf := makeSurface(200, 30)
	b.Draw(newP(buf, 200), theme)
	found := false
	for y := 0; y < 30 && !found; y++ {
		for x := 0; x < 200; x++ {
			if pixelAt(buf, 200, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in Banner text")
	}
}

// --- Draw: nil Extra covers the accentInk fallback --------------------

func TestBannerDrawWithNilExtra(t *testing.T) {
	b := NewBanner("x")
	b.ButtonLabel = "Go"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	theme := DefaultLight()
	theme.Extra = nil
	buf := makeSurface(200, 30)
	// Just prove it does not panic and still paints the accent strip.
	b.Draw(newP(buf, 200), theme)
	if pixelAt(buf, 200, 5, 15) != theme.Accent {
		t.Fatal("nil-Extra Banner strip fill != Accent")
	}
}

// --- Draw: zero-width bounds is a no-op -------------------------------

func TestBannerDrawZeroWidthBoundsSkipsFill(t *testing.T) {
	// Position the widget so its whole footprint sits above the surface
	// -- text glyphs clip out per-pixel + the fillRect guard is the only
	// pixel-writing path exercised, which we assert leaves the buffer
	// untouched.
	b := NewBanner("x")
	b.SetBounds(Rect{X: 0, Y: -40, W: 0, H: 30})
	surf := makeSurface(20, 30)
	before := make([]byte, len(surf))
	copy(before, surf)
	b.Draw(newP(surf, 20), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("zero-width Banner Draw painted byte %d", i)
		}
	}
	// Non-empty ButtonLabel + zero H -- exercises the button-branch
	// strokeRect + inner DrawText guard.
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: -40, W: 20, H: 0})
	surf2 := makeSurface(20, 30)
	copy(before, surf2)
	b.Draw(newP(surf2, 20), DefaultLight())
	for i := range surf2 {
		if surf2[i] != before[i] {
			t.Fatalf("zero-height Banner Draw painted byte %d", i)
		}
	}
}

// --- OnEvent: click on button with OnAction fires it -----------------

func TestBannerOnEventClicksButton(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	b.OnAction = func() { fired++ }
	// Compute button rect in surface coords + convert to widget-local
	// coords (identical here since bounds start at 0,0 but do the math
	// anyway to prove the OnEvent conversion works).
	br, ok := b.buttonRect()
	if !ok {
		t.Fatal("buttonRect not ok despite ButtonLabel != \"\"")
	}
	r := b.Bounds()
	ev := Event{Kind: EventClick, X: br.X + br.W/2 - r.X, Y: br.Y + br.H/2 - r.Y}
	b.OnEvent(ev)
	if fired != 1 {
		t.Fatalf("OnAction fired %d times, want 1", fired)
	}
}

// --- OnEvent: click on button with nil OnAction is inert -------------

func TestBannerOnEventClicksButtonWithNilOnAction(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	// Just prove it does not panic.
	br, _ := b.buttonRect()
	r := b.Bounds()
	b.OnEvent(Event{
		Kind: EventClick,
		X:    br.X + br.W/2 - r.X,
		Y:    br.Y + br.H/2 - r.Y,
	})
}

// --- OnEvent: click outside the button is ignored -------------------

func TestBannerOnEventClicksOutsideButton(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	b.OnAction = func() { fired++ }
	// Click near the leading edge, well away from the trailing button.
	b.OnEvent(Event{Kind: EventClick, X: 5, Y: 15})
	if fired != 0 {
		t.Fatalf("OnAction fired for non-button click: %d", fired)
	}
}

// --- OnEvent: click with empty ButtonLabel is ignored ----------------

func TestBannerOnEventClickWithEmptyButtonLabel(t *testing.T) {
	b := NewBanner("hi")
	// No ButtonLabel -> buttonRect returns ok=false -> click is dropped.
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	b.OnAction = func() { fired++ } // still assigned, still shouldn't fire
	b.OnEvent(Event{Kind: EventClick, X: 100, Y: 15})
	if fired != 0 {
		t.Fatalf("OnAction fired despite empty ButtonLabel: %d", fired)
	}
}

// --- OnEvent: non-click events are ignored --------------------------

func TestBannerOnEventNonClickIgnored(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	b.OnAction = func() { fired++ }
	br, _ := b.buttonRect()
	r := b.Bounds()
	b.OnEvent(Event{
		Kind: EventKeyDown,
		X:    br.X + br.W/2 - r.X,
		Y:    br.Y + br.H/2 - r.Y,
	})
	if fired != 0 {
		t.Fatalf("OnAction fired for non-Click event: %d", fired)
	}
}

// --- OnEvent: click when !Revealed is ignored -----------------------

func TestBannerOnEventClickWhenNotRevealed(t *testing.T) {
	b := NewBanner("hi")
	b.ButtonLabel = "OK"
	b.Revealed = false
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	b.OnAction = func() { fired++ }
	br, _ := b.buttonRect()
	r := b.Bounds()
	b.OnEvent(Event{
		Kind: EventClick,
		X:    br.X + br.W/2 - r.X,
		Y:    br.Y + br.H/2 - r.Y,
	})
	if fired != 0 {
		t.Fatalf("OnAction fired while !Revealed: %d", fired)
	}
}
