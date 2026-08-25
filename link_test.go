// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// underlineRow reports whether row y of a w-wide RGBA buffer carries any pixel
// of colour c — the way the tests detect the link's accent underline.
func underlineRow(buf []byte, w, y int, c RGBA) bool {
	for x := 0; x < w; x++ {
		if pixelAt(buf, w, x, y) == c {
			return true
		}
	}
	return false
}

// TestLinkHoverUnderline is the core proof: a pointer-move onto the link rect
// raises the underline (an accent bar under the glyphs), and a move off it
// clears the underline again. It exercises the EventMouseMove on/off branches
// end-to-end through Draw pixels, not just the hovered flag.
func TestLinkHoverUnderline(t *testing.T) {
	th := DefaultLight()
	gh := GlyphHeight()
	w, h := 200, gh+6
	l := NewLink("go-tex/engine", nil)
	l.VAlign = VMiddle
	l.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ty := (h - gh) / 2 // VMiddle text top
	urow := ty + gh    // the underline sits one line under the glyph box

	// Resting: no hover, no underline.
	if l.Hovered() {
		t.Fatal("a fresh link should not be hovered")
	}
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), th)
	if underlineRow(buf, w, urow, th.Accent) {
		t.Fatal("resting link must not draw an underline")
	}

	// Pointer moves onto the link → hovered → underline drawn.
	l.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 3})
	if !l.Hovered() {
		t.Fatal("a move inside the link should set hovered")
	}
	buf = makeSurface(w, h)
	l.Draw(newP(buf, w), th)
	if !underlineRow(buf, w, urow, th.Accent) {
		t.Fatal("hovered link must draw an accent underline")
	}

	// Pointer moves off the link → hover clears → underline gone.
	l.OnEvent(Event{Kind: EventMouseMove, X: -1, Y: 3})
	if l.Hovered() {
		t.Fatal("a move outside the link should clear hovered")
	}
	buf = makeSurface(w, h)
	l.Draw(newP(buf, w), th)
	if underlineRow(buf, w, urow, th.Accent) {
		t.Fatal("un-hovered link must not draw an underline")
	}
}

// TestLinkForcedUnderline: Underline=true keeps the underline on with no hover.
func TestLinkForcedUnderline(t *testing.T) {
	th := DefaultLight()
	gh := GlyphHeight()
	w, h := 120, gh+6
	l := NewLink("always", nil)
	l.Underline = true
	l.VAlign = VMiddle
	l.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), th)
	if !underlineRow(buf, w, (h-gh)/2+gh, th.Accent) {
		t.Fatal("Underline=true must draw the underline without hover")
	}
}

// TestLinkClickActivates: a click and an Enter/Space key press each fire
// OnClick; a non-activating key and a nil handler are safe no-ops; a disabled
// link ignores every event.
func TestLinkClickActivates(t *testing.T) {
	fired := 0
	l := NewLink("nav", func() { fired++ })
	l.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 20})

	l.OnEvent(Event{Kind: EventClick})
	if fired != 1 {
		t.Fatalf("EventClick should fire OnClick once, got %d", fired)
	}
	for _, code := range []string{"Enter", " ", "Space"} {
		before := fired
		l.OnEvent(Event{Kind: EventKeyDown, Code: code})
		if fired != before+1 {
			t.Fatalf("key %q should activate the link", code)
		}
	}
	// A non-activating key does nothing.
	before := fired
	l.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if fired != before {
		t.Fatal("ArrowLeft must not activate the link")
	}

	// A disabled link ignores clicks entirely.
	l.Disabled().Set(true)
	before = fired
	l.OnEvent(Event{Kind: EventClick})
	if fired != before {
		t.Fatal("a disabled link must not activate")
	}
	l.Disabled().Set(false)

	// A nil handler is a safe no-op on activation.
	nl := NewLink("noop", nil)
	nl.OnEvent(Event{Kind: EventClick}) // must not panic
}

// TestLinkSetHovered covers the container-driven hover seam.
func TestLinkSetHovered(t *testing.T) {
	l := NewLink("x", nil)
	l.SetHovered(true)
	if !l.Hovered() {
		t.Fatal("SetHovered(true) should set the hover face")
	}
	l.SetHovered(false)
	if l.Hovered() {
		t.Fatal("SetHovered(false) should clear the hover face")
	}
}

// TestLinkInkAndAlign covers the Ink override branch and all Align / VAlign
// positioning branches, including the left-clamp when the text is wider than
// the bounds.
func TestLinkInkAndAlign(t *testing.T) {
	th := DefaultLight()
	gh := GlyphHeight()

	// Ink override: a hovered link with a custom Ink underlines in that colour,
	// not Accent.
	custom := RGB(0xC0, 0x10, 0x40)
	l := NewLink("tint", nil)
	l.Ink = custom
	l.VAlign = VMiddle
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: gh + 6})
	l.SetHovered(true)
	buf := makeSurface(100, gh+6)
	l.Draw(newP(buf, 100), th)
	if !underlineRow(buf, 100, (gh+6-gh)/2+gh, custom) {
		t.Fatal("Ink override should colour the underline")
	}
	if underlineRow(buf, 100, (gh+6-gh)/2+gh, th.Accent) {
		t.Fatal("Ink override must not underline in Accent")
	}

	// Every Align / VAlign combination draws without panicking; AlignRight with
	// a too-narrow box exercises the left clamp, AlignCenter with a wide box the
	// unclamped centre.
	cases := []struct {
		a Align
		v VAlign
		w int
	}{
		{AlignLeft, VTop, 200},
		{AlignCenter, VBottom, 200},
		{AlignRight, VAuto, 200}, // wide: no clamp
		{AlignRight, VMiddle, 4}, // narrow: tx clamps to r.X
		{AlignCenter, VAuto, 4},  // narrow + VAuto (H==gh so no vertical centre)
	}
	for _, c := range cases {
		lk := NewLink("wide-caption", nil)
		lk.Align, lk.VAlign = c.a, c.v
		height := gh + 6
		if c.v == VAuto && c.w == 4 {
			height = gh // H==gh: VAuto keeps the top anchor (no centre branch)
		}
		lk.SetBounds(Rect{X: 0, Y: 0, W: c.w, H: height})
		lk.Draw(newP(makeSurface(c.w, height), c.w), th)
	}
}

// TestLinkTextLazyAndA11y covers the lazy Text() accessor on a bare &Link{} and
// the accessibility description.
func TestLinkTextLazyAndA11y(t *testing.T) {
	var l Link // bare, no NewLink
	if l.Text().Get() != "" {
		t.Fatal("bare Link.Text() should be empty")
	}
	l.Text().Set("Docs")
	info := l.A11y()
	if info.Role != RoleLink || info.Name != "Docs" {
		t.Fatalf("A11y = %+v, want {link, Docs}", info)
	}
}
