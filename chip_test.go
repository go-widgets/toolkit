// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestChipDefaultsToNotClosable covers the NewChip constructor: the
// passive tag path leaves Closable false and OnClose nil.
func TestChipDefaultsToNotClosable(t *testing.T) {
	c := NewChip("hello")
	if c.Text != "hello" {
		t.Fatalf("Text = %q, want %q", c.Text, "hello")
	}
	if c.Closable {
		t.Fatal("NewChip must default Closable to false")
	}
	if c.OnClose != nil {
		t.Fatal("NewChip must leave OnClose nil")
	}
}

// TestClosableChipConstructor covers NewClosableChip: Closable is true
// and OnClose is set to the caller-supplied callback.
func TestClosableChipConstructor(t *testing.T) {
	fires := 0
	c := NewClosableChip("tag", func() { fires++ })
	if !c.Closable {
		t.Fatal("NewClosableChip must set Closable=true")
	}
	if c.OnClose == nil {
		t.Fatal("NewClosableChip must wire OnClose")
	}
	c.OnClose()
	if fires != 1 {
		t.Fatalf("OnClose fires = %d, want 1", fires)
	}
}

// TestChipAutoSizesWhenWZero covers the auto-size branch on the
// non-closable path: W and H both start at zero, and Draw() must set
// them to the text-fit size.
func TestChipAutoSizesWhenWZero(t *testing.T) {
	c := NewChip("hi")
	c.SetBounds(Rect{X: 4, Y: 4, W: 0, H: 0})
	theme := DefaultLight()
	c.Draw(newP(makeSurface(80, 40), 80), theme)
	got := c.Bounds()
	wantW := TextWidth("hi") + 2*ChipPadX
	wantH := GlyphHeight() + 2*ChipPadY
	if got.W != wantW {
		t.Fatalf("auto-sized W = %d, want %d", got.W, wantW)
	}
	if got.H != wantH {
		t.Fatalf("auto-sized H = %d, want %d", got.H, wantH)
	}
}

// TestChipAutoSizesClosableExtraWidth covers the closable auto-size
// branch: W must include the close slot's ChipCloseGap + ChipCloseW
// on top of the text width.
func TestChipAutoSizesClosableExtraWidth(t *testing.T) {
	c := NewClosableChip("hi", nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	theme := DefaultLight()
	c.Draw(newP(makeSurface(80, 40), 80), theme)
	got := c.Bounds()
	wantW := TextWidth("hi") + 2*ChipPadX + ChipCloseGap + ChipCloseW
	if got.W != wantW {
		t.Fatalf("closable auto-sized W = %d, want %d", got.W, wantW)
	}
}

// TestChipAutoSizesWZeroPreservesH covers the sub-branch where W is
// zero but H is caller-supplied: only W should be filled in, H stays.
func TestChipAutoSizesWZeroPreservesH(t *testing.T) {
	c := NewChip("A")
	c.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	theme := DefaultLight()
	c.Draw(newP(makeSurface(80, 40), 80), theme)
	if c.Bounds().H != 20 {
		t.Fatalf("caller-supplied H clobbered: got H=%d, want 20", c.Bounds().H)
	}
	if c.Bounds().W == 0 {
		t.Fatal("W should have been auto-sized")
	}
}

// TestChipPreSizedNoOverride covers the non-auto-size branch: when W
// is already set, Draw must respect the caller's Bounds verbatim.
func TestChipPreSizedNoOverride(t *testing.T) {
	c := NewChip("tag")
	c.SetBounds(Rect{X: 5, Y: 5, W: 60, H: 18})
	theme := DefaultLight()
	c.Draw(newP(makeSurface(100, 40), 100), theme)
	got := c.Bounds()
	if got.W != 60 || got.H != 18 {
		t.Fatalf("pre-sized Bounds changed: %+v", got)
	}
}

// TestChipDrawFillsSurfaceAlt samples an interior pixel above the
// text row to prove the pill body is painted in Theme.SurfaceAlt.
func TestChipDrawFillsSurfaceAlt(t *testing.T) {
	const w, h = 60, 20
	theme := DefaultLight()
	c := NewChip("hi")
	c.SetBounds(Rect{X: 2, Y: 2, W: 40, H: 14})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// (20, 3) is inside the pill body, above the text row (ty = 2 +
	// (14-7)/2 = 5).
	if got := pixelAt(buf, w, 20, 3); got != theme.SurfaceAlt {
		t.Fatalf("chip body at (20,3) = %+v, want SurfaceAlt", got)
	}
	// Top-left border pixel painted.
	if got := pixelAt(buf, w, 2, 2); got != theme.Border {
		t.Fatalf("top-left border = %+v, want Border", got)
	}
}

// TestChipDrawWithDarkTheme covers Draw with DefaultDark to exercise
// the palette-swap path.
func TestChipDrawWithDarkTheme(t *testing.T) {
	const w, h = 60, 20
	theme := DefaultDark()
	c := NewClosableChip("hi", nil)
	c.SetBounds(Rect{X: 2, Y: 2, W: 50, H: 14})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 20, 3); got != theme.SurfaceAlt {
		t.Fatalf("dark chip body = %+v, want dark SurfaceAlt", got)
	}
}

// TestChipDrawWithExtraTheme covers Draw against a theme whose Extra
// map is populated: the widget does not consume Extra keys but must
// still render cleanly against such themes.
func TestChipDrawWithExtraTheme(t *testing.T) {
	const w, h = 60, 20
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{"OnAccent": RGB(0xFF, 0xFF, 0xFF)}
	c := NewChip("extra")
	c.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 14})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 20, 3); got != theme.SurfaceAlt {
		t.Fatalf("Extra-populated theme changed the fill: %+v", got)
	}
}

// TestChipEmptyText covers the empty-Text auto-size branch: W collapses
// to 2*ChipPadX for a non-closable chip, and Draw must not panic on the
// empty glyph loop.
func TestChipEmptyText(t *testing.T) {
	c := NewChip("")
	c.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	theme := DefaultLight()
	c.Draw(newP(makeSurface(40, 20), 40), theme)
	if c.Bounds().W != 2*ChipPadX {
		t.Fatalf("empty chip W = %d, want %d", c.Bounds().W, 2*ChipPadX)
	}
}

// TestChipClickInCloseSlotFiresOnClose covers the fire path: an
// EventClick with widget-local X inside the close slot must invoke
// OnClose exactly once.
func TestChipClickInCloseSlotFiresOnClose(t *testing.T) {
	fires := 0
	c := NewClosableChip("tag", func() { fires++ })
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	// Close slot spans widget-local X in [60-8-12, 60-8) = [40, 52).
	c.OnEvent(Event{Kind: EventClick, X: 45, Y: 8})
	if fires != 1 {
		t.Fatalf("OnClose fires = %d, want 1", fires)
	}
}

// TestChipClickLeftOfCloseSlot covers the "click left of slot" branch
// of the boundary check: ev.X < left short-circuits the OR.
func TestChipClickLeftOfCloseSlot(t *testing.T) {
	fires := 0
	c := NewClosableChip("tag", func() { fires++ })
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	// Click on the text region (X=10), well left of the close slot.
	c.OnEvent(Event{Kind: EventClick, X: 10, Y: 8})
	if fires != 0 {
		t.Fatalf("click outside close slot fired OnClose %d times, want 0", fires)
	}
}

// TestChipClickRightOfCloseSlot covers the right side of the boundary
// check: ev.X >= right must also skip the fire.
func TestChipClickRightOfCloseSlot(t *testing.T) {
	fires := 0
	c := NewClosableChip("tag", func() { fires++ })
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	// right = 60 - ChipPadX = 52; ev.X = 52 is >= right → outside slot.
	c.OnEvent(Event{Kind: EventClick, X: 52, Y: 8})
	if fires != 0 {
		t.Fatalf("click at right edge fired OnClose %d times, want 0", fires)
	}
}

// TestChipClickWhenNotClosableIgnored covers the !Closable branch:
// clicks on a passive chip must never fire OnClose (even if a callback
// were somehow set post-construction).
func TestChipClickWhenNotClosableIgnored(t *testing.T) {
	fires := 0
	c := NewChip("tag")
	c.OnClose = func() { fires++ } // forced-on to prove Closable gates the call
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	c.OnEvent(Event{Kind: EventClick, X: 45, Y: 8})
	if fires != 0 {
		t.Fatalf("non-closable chip fired OnClose %d times, want 0", fires)
	}
}

// TestChipClickWithNilOnCloseNoPanic covers the fire path with a nil
// callback: the widget must not panic.
func TestChipClickWithNilOnCloseNoPanic(t *testing.T) {
	c := NewClosableChip("tag", nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	// Must not panic.
	c.OnEvent(Event{Kind: EventClick, X: 45, Y: 8})
}

// TestChipIgnoresOtherKinds covers the ev.Kind != EventClick branch:
// EventKeyDown (and every non-click event) must be a no-op.
func TestChipIgnoresOtherKinds(t *testing.T) {
	fires := 0
	c := NewClosableChip("tag", func() { fires++ })
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fires != 0 {
		t.Fatalf("KeyDown fired OnClose %d times, want 0", fires)
	}
}
