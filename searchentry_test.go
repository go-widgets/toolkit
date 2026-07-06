// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewSearchEntryStoresText(t *testing.T) {
	s := NewSearchEntry("hi")
	if s.Text != "hi" {
		t.Fatalf("NewSearchEntry: Text = %q, want %q", s.Text, "hi")
	}
}

func TestNewSearchEntryEmpty(t *testing.T) {
	s := NewSearchEntry("")
	if s.Text != "" {
		t.Fatalf("NewSearchEntry empty: Text = %q, want empty", s.Text)
	}
}

// --- Draw branches -------------------------------------------------------

// Empty text: no clear affordance appears — the right-icon branch is
// skipped. Every painted pixel is either Surface, Border, or the
// OnSurface prefix "?" ink.
func TestSearchEntryDrawEmptyNoClearIcon(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	s := NewSearchEntry("")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Search a horizontal band on the right for any Border-coloured
	// ink OUTSIDE the outer frame's stroke — such ink would be the
	// clear "x". The frame stroke itself lives on the four outermost
	// rows/columns; anything strictly interior in Border is the
	// affordance. There should be none.
	textY := (h - GlyphHeight) / 2
	interiorRight := w - SearchEntryPadX - SearchEntryIconW
	for y := textY; y < textY+GlyphHeight; y++ {
		for x := interiorRight; x < w-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Border && y > 0 && y < h-1 {
				t.Fatalf("empty SearchEntry painted a clear affordance at (%d,%d)", x, y)
			}
		}
	}
}

// Non-empty text: the clear affordance must land inside the right
// icon slot, drawn in theme.Border.
func TestSearchEntryDrawWithTextPaintsClearIcon(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	s := NewSearchEntry("hi")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Look inside the right icon slot's interior rows for at least
	// one Border-coloured pixel that isn't part of the outer stroke.
	textY := (h - GlyphHeight) / 2
	interiorLeft := w - SearchEntryPadX - SearchEntryIconW
	found := false
	for y := textY; y < textY+GlyphHeight && !found; y++ {
		for x := interiorLeft; x < w-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Border {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("non-empty SearchEntry: no clear-affordance ink in right slot")
	}
}

// Prefix glyph appears in theme.OnSurface — verifies the left-slot
// branch always runs (even without text).
func TestSearchEntryDrawPaintsPrefixGlyph(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	s := NewSearchEntry("")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Some OnSurface ink lands in the left icon slot's interior.
	textY := (h - GlyphHeight) / 2
	interiorRight := SearchEntryPadX + SearchEntryIconW
	found := false
	for y := textY; y < textY+GlyphHeight && !found; y++ {
		for x := 1; x < interiorRight; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("prefix glyph not painted")
	}
}

// Dark theme exercise: switching palettes must not change branch
// behaviour but does need coverage of the code paths reading each
// theme field.
func TestSearchEntryDrawDarkTheme(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultDark()
	s := NewSearchEntry("hi")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("dark border top-left = %+v, want Border", pixelAt(buf, w, 0, 0))
	}
}

// Zero-width bounds must not panic and must not paint anything.
func TestSearchEntryDrawZeroBounds(t *testing.T) {
	const w, h = 8, 8
	theme := DefaultLight()
	s := NewSearchEntry("hi")
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0).R != 0xC8 {
		t.Fatal("zero-bounds Draw painted pixels")
	}
}

// --- OnEvent branches ----------------------------------------------------

func TestSearchEntryCharAppendsAndFiresOnChange(t *testing.T) {
	changes := 0
	last := ""
	s := NewSearchEntry("ab")
	s.OnChange = func(v string) { changes++; last = v }
	s.OnEvent(Event{Kind: EventChar, Code: "c"})
	if s.Text != "abc" || changes != 1 || last != "abc" {
		t.Fatalf("Char append: Text=%q changes=%d last=%q", s.Text, changes, last)
	}
}

func TestSearchEntryEmptyCharIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	changes := 0
	s.OnChange = func(v string) { changes++ }
	s.OnEvent(Event{Kind: EventChar, Code: ""})
	if s.Text != "ab" || changes != 0 {
		t.Fatalf("empty Char: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryBackspaceDropsLastRune(t *testing.T) {
	changes := 0
	s := NewSearchEntry("ab")
	s.OnChange = func(v string) { changes++ }
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if s.Text != "a" || changes != 1 {
		t.Fatalf("Backspace: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryBackspaceOnEmptyIsNoOp(t *testing.T) {
	s := NewSearchEntry("")
	changes := 0
	s.OnChange = func(v string) { changes++ }
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if s.Text != "" || changes != 0 {
		t.Fatalf("empty Backspace: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryUnknownKeyIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	changes := 0
	s.OnChange = func(v string) { changes++ }
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.Text != "ab" || changes != 0 {
		t.Fatalf("unknown key: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryClickInClearSlotClearsText(t *testing.T) {
	changes := 0
	last := "unchanged"
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	s.OnChange = func(v string) { changes++; last = v }
	// Right slot occupies [W-Pad-Icon, W-Pad) = [60, 76).
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
	if s.Text != "" || changes != 1 || last != "" {
		t.Fatalf("clear-click: Text=%q changes=%d last=%q", s.Text, changes, last)
	}
}

func TestSearchEntryClickInClearSlotWhenEmptyIsNoOp(t *testing.T) {
	s := NewSearchEntry("")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	changes := 0
	s.OnChange = func(v string) { changes++ }
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
	if s.Text != "" || changes != 0 {
		t.Fatalf("empty clear-click: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryClickOutsideClearSlotIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	changes := 0
	s.OnChange = func(v string) { changes++ }
	// Click in the middle of the entry (not in the right slot).
	s.OnEvent(Event{Kind: EventClick, X: 40, Y: 12})
	if s.Text != "ab" || changes != 0 {
		t.Fatalf("middle click: Text=%q changes=%d", s.Text, changes)
	}
}

func TestSearchEntryIgnoresKeyUp(t *testing.T) {
	s := NewSearchEntry("ab")
	s.OnEvent(Event{Kind: EventKeyUp, Code: "a"})
	if s.Text != "ab" {
		t.Fatal("KeyUp should not mutate")
	}
}

func TestSearchEntryNilOnChangeNoPanic(t *testing.T) {
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	// Fire every mutation path; nil OnChange must be safe.
	s.OnEvent(Event{Kind: EventChar, Code: "c"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
}
