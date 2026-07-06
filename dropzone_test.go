// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestDropZoneDefaultPromptWhenEmpty covers the empty-prompt branch of
// NewDropZone: an "" input must be replaced with the fallback string
// so a zero-argument caller still renders a legible target.
func TestDropZoneDefaultPromptWhenEmpty(t *testing.T) {
	d := NewDropZone("")
	if d.Prompt != "Drop files here" {
		t.Fatalf("empty prompt not defaulted, got %q", d.Prompt)
	}
}

// TestDropZoneCustomPrompt covers the non-empty branch: caller-supplied
// prompts must be preserved verbatim.
func TestDropZoneCustomPrompt(t *testing.T) {
	d := NewDropZone("Drop images")
	if d.Prompt != "Drop images" {
		t.Fatalf("custom prompt not preserved, got %q", d.Prompt)
	}
	if d.Hover {
		t.Fatal("Hover should default to false")
	}
	if d.OnDrop != nil {
		t.Fatal("OnDrop should default to nil")
	}
}

// TestDropZoneDrawIdleUsesSurface samples an interior pixel above the
// text row to prove the idle-state fill is theme.Surface and that the
// dashed edges use theme.Border. Uses W=26, H=26 so the dash loop hits
// both the "fits exactly" and the "clip trailing dash" branches.
func TestDropZoneDrawIdleUsesSurface(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	d := NewDropZone("go")
	d.SetBounds(Rect{X: 2, Y: 2, W: 26, H: 26})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// Interior pixel: (10, 10) is inside the fill, above the centred
	// text row (which starts around y = 2 + (26-7)/2 = 11).
	if got := pixelAt(buf, w, 10, 10); got != theme.Surface {
		t.Fatalf("idle fill at (10,10) = %+v, want Surface", got)
	}
	// Top-edge dash pixel should be Border.
	if got := pixelAt(buf, w, 3, 2); got != theme.Border {
		t.Fatalf("idle border at (3,2) = %+v, want Border", got)
	}
}

// TestDropZoneDrawHoverUsesSurfaceAlt covers the Hover=true branch:
// the fill swaps to SurfaceAlt and the dashed border swaps to Accent.
func TestDropZoneDrawHoverUsesSurfaceAlt(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	d := NewDropZone("hi")
	d.Hover = true
	d.SetBounds(Rect{X: 2, Y: 2, W: 26, H: 26})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 10, 10); got != theme.SurfaceAlt {
		t.Fatalf("hover fill at (10,10) = %+v, want SurfaceAlt", got)
	}
	if got := pixelAt(buf, w, 3, 2); got != theme.Accent {
		t.Fatalf("hover border at (3,2) = %+v, want Accent", got)
	}
}

// TestDropZoneDrawDashClipping covers the clip branch of the dash
// loop: with W=26 the last horizontal dash starts at x=24 and would
// end at x=28, which exceeds r.X+r.W=28... wait, r.X=2 so r.X+r.W=28,
// dash w=4 ends at 28 which is NOT > 28. Use W=25 (odd) to force
// the clip.
func TestDropZoneDrawDashClipping(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	d := NewDropZone("x")
	// r.X=0, r.W=26: dashes at x=0,8,16,24. At x=24 dash ends at 28 > 26 → clip.
	d.SetBounds(Rect{X: 0, Y: 0, W: 26, H: 26})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// The clipped last dash on the top edge lands at (24..25, 0..1).
	if got := pixelAt(buf, w, 24, 0); got != theme.Border {
		t.Fatalf("clipped top dash at (24,0) = %+v, want Border", got)
	}
	// Pixel just past r.W (x=26) must NOT have been painted.
	if got := pixelAt(buf, w, 26, 0); got == theme.Border {
		t.Fatalf("clip failed: dash spilled past r.W at (26,0) = %+v", got)
	}
}

// TestDropZoneDrawDarkTheme covers Draw with DefaultDark to exercise
// the palette-swap path — Surface / Border take dark values but the
// same code path fires.
func TestDropZoneDrawDarkTheme(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultDark()
	d := NewDropZone("dark")
	d.SetBounds(Rect{X: 0, Y: 0, W: 26, H: 26})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 10, 10); got != theme.Surface {
		t.Fatalf("dark idle fill = %+v, want dark Surface", got)
	}
}

// TestDropZoneDrawTinyBounds covers Draw with a bounds smaller than
// one dash cycle (step = 8): the loop still enters once, hits the
// clip branch on the first iteration, and DrawText is called with a
// negative tx offset (safely clipped by the painter).
func TestDropZoneDrawTinyBounds(t *testing.T) {
	const w, h = 16, 16
	theme := DefaultLight()
	d := NewDropZone("tiny")
	d.SetBounds(Rect{X: 0, Y: 0, W: 3, H: 3})
	buf := makeSurface(w, h)
	// Must not panic.
	d.Draw(newP(buf, w), theme)
}

// TestDropZoneDrawWithExtraTheme covers Draw with a theme whose Extra
// map is populated: the widget does not consume Extra keys but must
// still render cleanly against such themes. Guards against a future
// regression that leaks nil-map access into DropZone's Draw path.
func TestDropZoneDrawWithExtraTheme(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{"OnAccent": RGB(0xFF, 0xFF, 0xFF)}
	d := NewDropZone("extra")
	d.SetBounds(Rect{X: 0, Y: 0, W: 26, H: 26})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 10, 10); got != theme.Surface {
		t.Fatalf("Extra-populated theme changed the fill: %+v", got)
	}
}

// TestDropZoneClickTogglesHover covers the EventClick branch of
// OnEvent: each click flips Hover. Two clicks return to the original
// state, so the toggle is symmetric.
func TestDropZoneClickTogglesHover(t *testing.T) {
	d := NewDropZone("x")
	if d.Hover {
		t.Fatal("initial Hover should be false")
	}
	d.OnEvent(Event{Kind: EventClick})
	if !d.Hover {
		t.Fatal("first click should set Hover=true")
	}
	d.OnEvent(Event{Kind: EventClick})
	if d.Hover {
		t.Fatal("second click should return Hover=false")
	}
}

// TestDropZoneCharDropsWhenHovered covers the EventChar + Hover=true
// + OnDrop non-nil branch: OnDrop must fire with a one-element slice
// carrying ev.Code.
func TestDropZoneCharDropsWhenHovered(t *testing.T) {
	var got []string
	d := NewDropZone("x")
	d.OnDrop = func(paths []string) { got = paths }
	d.Hover = true
	d.OnEvent(Event{Kind: EventChar, Code: "/tmp/foo.txt"})
	if len(got) != 1 || got[0] != "/tmp/foo.txt" {
		t.Fatalf("OnDrop got %v, want [/tmp/foo.txt]", got)
	}
}

// TestDropZoneCharIgnoredWhenIdle covers the EventChar + Hover=false
// branch: OnDrop must NOT fire when the widget is not in the drag-
// over state, even if a callback is registered.
func TestDropZoneCharIgnoredWhenIdle(t *testing.T) {
	fires := 0
	d := NewDropZone("x")
	d.OnDrop = func(paths []string) { fires++ }
	d.Hover = false
	d.OnEvent(Event{Kind: EventChar, Code: "/tmp/foo.txt"})
	if fires != 0 {
		t.Fatalf("OnDrop fired %d times while idle, want 0", fires)
	}
}

// TestDropZoneCharNilCallbackNoPanic covers the EventChar + Hover=true
// + OnDrop=nil branch: the widget must not panic when no callback is
// wired (the parent may register OnDrop lazily).
func TestDropZoneCharNilCallbackNoPanic(t *testing.T) {
	d := NewDropZone("x")
	d.Hover = true
	// Must not panic.
	d.OnEvent(Event{Kind: EventChar, Code: "/tmp/foo.txt"})
}

// TestDropZoneIgnoresOtherKinds covers the default branch of the
// switch: EventKeyDown (and every non-Click / non-Char event) must be
// a no-op — no state change, no callback fired.
func TestDropZoneIgnoresOtherKinds(t *testing.T) {
	fires := 0
	d := NewDropZone("x")
	d.OnDrop = func(paths []string) { fires++ }
	d.Hover = true
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fires != 0 {
		t.Fatalf("KeyDown fired OnDrop %d times, want 0", fires)
	}
	// Hover must be unchanged (was true, still true).
	if !d.Hover {
		t.Fatal("KeyDown must not clobber Hover")
	}
}
