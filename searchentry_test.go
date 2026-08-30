// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// assertFocusedCaret renders "hello" unfocused and focused and requires the
// focused render to differ — i.e. the end-of-text caret painted something.
func assertFocusedCaret(t *testing.T) {
	t.Helper()
	const w, h = 120, 24
	theme := DefaultLight()
	draw := func(focused bool) []byte {
		s := NewSearchEntry("hello")
		s.SetFocused(focused)
		s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
		buf := makeSurface(w, h)
		s.Draw(newP(buf, w), theme)
		return buf
	}
	if bytes.Equal(draw(false), draw(true)) {
		t.Fatal("focused caret produced no visible change")
	}
}

// Bitmap font (glyph height < 12) exercises the caretW<1 clamp.
func TestSearchEntryFocusedCaretBitmap(t *testing.T) { assertFocusedCaret(t) }

// A taller TrueType font (glyph height ≥ 12) gives a caret wider than 1px.
func TestSearchEntryFocusedCaretTrueType(t *testing.T) {
	SetFont(newTTF(t, 16))
	defer SetFont(nil)
	assertFocusedCaret(t)
}

// --- Constructor ---------------------------------------------------------

func TestNewSearchEntryStoresText(t *testing.T) {
	s := NewSearchEntry("hi")
	if s.Text().Get() != "hi" {
		t.Fatalf("NewSearchEntry: Text = %q, want %q", s.Text().Get(), "hi")
	}
}

func TestNewSearchEntryEmpty(t *testing.T) {
	s := NewSearchEntry("")
	if s.Text().Get() != "" {
		t.Fatalf("NewSearchEntry empty: Text = %q, want empty", s.Text().Get())
	}
}

// --- Text / MVVM ---------------------------------------------------------

// TestSearchEntryTextObservable covers the zero-value lazy-init of the Text
// accessor and the host binding path: a SearchEntry built as a bare struct (no
// NewSearchEntry) still yields a usable Observable, and Setting it from outside
// is reflected by the widget (there is no imperative Text field).
func TestSearchEntryTextObservable(t *testing.T) {
	s := &SearchEntry{} // no NewSearchEntry → text Observable is nil until accessed
	if s.Text().Get() != "" {
		t.Fatalf("zero-value SearchEntry Text = %q, want empty", s.Text().Get())
	}
	seen := "unseen"
	s.Text().Subscribe(func(v string) { seen = v })
	s.Text().Set("query") // a host drives the field through the Observable
	if s.Text().Get() != "query" || seen != "query" {
		t.Fatalf("host Set: text=%q subscriber=%q, want query/query", s.Text().Get(), seen)
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
	textY := (h - GlyphHeight()) / 2
	interiorRight := w - SearchEntryPadX - SearchEntryIconW
	for y := textY; y < textY+GlyphHeight(); y++ {
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
	textY := (h - GlyphHeight()) / 2
	interiorLeft := w - SearchEntryPadX - SearchEntryIconW
	found := false
	for y := textY; y < textY+GlyphHeight() && !found; y++ {
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
	textY := (h - GlyphHeight()) / 2
	interiorRight := SearchEntryPadX + SearchEntryIconW
	found := false
	for y := textY; y < textY+GlyphHeight() && !found; y++ {
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

// A non-nil Icon takes over the left prefix slot: the callback is invoked
// exactly once with the prefix rect + the OnSurface ink, and the "?" text
// stand-in is NOT drawn (no OnSurface text ink lands in the slot, since the
// stub callback paints nothing).
func TestSearchEntryDrawIconReplacesPrefix(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	s := NewSearchEntry("")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	calls := 0
	var gotRect Rect
	var gotInk RGBA
	s.Icon = func(p painter.Painter, r Rect, ink RGBA) {
		calls++
		gotRect = r
		gotInk = ink
	}
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if calls != 1 {
		t.Fatalf("Icon invoked %d times, want 1", calls)
	}
	wantRect := Rect{X: SearchEntryPadX, Y: 0, W: SearchEntryIconW, H: h}
	if gotRect != wantRect {
		t.Fatalf("Icon rect = %+v, want %+v", gotRect, wantRect)
	}
	if gotInk != theme.OnSurface {
		t.Fatalf("Icon ink = %+v, want OnSurface %+v", gotInk, theme.OnSurface)
	}
	// The stub drew nothing, so the left slot carries no OnSurface "?" ink.
	textY := (h - GlyphHeight()) / 2
	interiorRight := SearchEntryPadX + SearchEntryIconW
	for y := textY; y < textY+GlyphHeight(); y++ {
		for x := 1; x < interiorRight; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				t.Fatalf("Icon set but '?' stand-in still painted at (%d,%d)", x, y)
			}
		}
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

func TestSearchEntryCharAppendsAndNotifies(t *testing.T) {
	changes := 0
	last := ""
	s := NewSearchEntry("ab")
	s.Text().Subscribe(func(v string) { changes++; last = v })
	s.OnEvent(Event{Kind: EventChar, Code: "c"})
	if s.Text().Get() != "abc" || changes != 1 || last != "abc" {
		t.Fatalf("Char append: Text=%q changes=%d last=%q", s.Text().Get(), changes, last)
	}
}

func TestSearchEntryEmptyCharIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	changes := 0
	s.Text().Subscribe(func(v string) { changes++ })
	s.OnEvent(Event{Kind: EventChar, Code: ""})
	if s.Text().Get() != "ab" || changes != 0 {
		t.Fatalf("empty Char: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryBackspaceDropsLastRune(t *testing.T) {
	changes := 0
	s := NewSearchEntry("ab")
	s.Text().Subscribe(func(v string) { changes++ })
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if s.Text().Get() != "a" || changes != 1 {
		t.Fatalf("Backspace: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryBackspaceOnEmptyIsNoOp(t *testing.T) {
	s := NewSearchEntry("")
	changes := 0
	s.Text().Subscribe(func(v string) { changes++ })
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if s.Text().Get() != "" || changes != 0 {
		t.Fatalf("empty Backspace: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryUnknownKeyIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	changes := 0
	s.Text().Subscribe(func(v string) { changes++ })
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.Text().Get() != "ab" || changes != 0 {
		t.Fatalf("unknown key: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryClickInClearSlotClearsText(t *testing.T) {
	changes := 0
	last := "unchanged"
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	s.Text().Subscribe(func(v string) { changes++; last = v })
	// Right slot occupies [W-Pad-Icon, W-Pad) = [60, 76).
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
	if s.Text().Get() != "" || changes != 1 || last != "" {
		t.Fatalf("clear-click: Text=%q changes=%d last=%q", s.Text().Get(), changes, last)
	}
}

func TestSearchEntryClickInClearSlotWhenEmptyIsNoOp(t *testing.T) {
	s := NewSearchEntry("")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	changes := 0
	s.Text().Subscribe(func(v string) { changes++ })
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
	if s.Text().Get() != "" || changes != 0 {
		t.Fatalf("empty clear-click: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryClickOutsideClearSlotIsNoOp(t *testing.T) {
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	changes := 0
	s.Text().Subscribe(func(v string) { changes++ })
	// Click in the middle of the entry (not in the right slot).
	s.OnEvent(Event{Kind: EventClick, X: 40, Y: 12})
	if s.Text().Get() != "ab" || changes != 0 {
		t.Fatalf("middle click: Text=%q changes=%d", s.Text().Get(), changes)
	}
}

func TestSearchEntryIgnoresKeyUp(t *testing.T) {
	s := NewSearchEntry("ab")
	s.OnEvent(Event{Kind: EventKeyUp, Code: "a"})
	if s.Text().Get() != "ab" {
		t.Fatal("KeyUp should not mutate")
	}
}

func TestSearchEntryNoSubscriberNoPanic(t *testing.T) {
	s := NewSearchEntry("ab")
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	// Fire every mutation path; no subscriber must be safe.
	s.OnEvent(Event{Kind: EventChar, Code: "c"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	s.OnEvent(Event{Kind: EventClick, X: 65, Y: 12})
}

// --- caret navigation (SearchEntry gained Entry-style cursor movement) ---

func TestSearchEntryInsertsAtCaret(t *testing.T) {
	s := NewSearchEntry("ac")
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // caret between a and c
	if s.cursor != 1 {
		t.Fatalf("ArrowLeft cursor = %d, want 1", s.cursor)
	}
	s.OnEvent(Event{Kind: EventChar, Code: "b"})
	if got := s.Text().Get(); got != "abc" {
		t.Fatalf("insert at caret = %q, want abc", got)
	}
	if s.cursor != 2 {
		t.Fatalf("cursor after insert = %d, want 2", s.cursor)
	}
}

func TestSearchEntryArrowAndHomeEndBounds(t *testing.T) {
	s := NewSearchEntry("ab")                                // cursor parks at end (2)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // at end → no-op
	if s.cursor != 2 {
		t.Fatalf("ArrowRight past end → %d", s.cursor)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if s.cursor != 0 {
		t.Fatalf("Home → %d, want 0", s.cursor)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // at start → no-op
	if s.cursor != 0 {
		t.Fatalf("ArrowLeft before start → %d", s.cursor)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if s.cursor != 1 {
		t.Fatalf("ArrowRight → %d, want 1", s.cursor)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if s.cursor != 2 {
		t.Fatalf("End → %d, want 2", s.cursor)
	}
}

func TestSearchEntryBackspaceAtCaret(t *testing.T) {
	s := NewSearchEntry("abc")
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // caret=1, between a and b
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})  // deletes the 'a' before it
	if got := s.Text().Get(); got != "bc" {
		t.Fatalf("backspace at caret = %q, want bc", got)
	}
	if s.cursor != 0 {
		t.Fatalf("cursor after backspace = %d, want 0", s.cursor)
	}
}

func TestSearchEntryClickPlacesCaret(t *testing.T) {
	s := NewSearchEntry("hello")
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})
	// A click at the text region's left edge parks the caret at the start.
	s.OnEvent(Event{Kind: EventClick, X: scaled(SearchEntryPadX) + scaled(SearchEntryIconW), Y: 12})
	if s.cursor != 0 {
		t.Fatalf("click at text start → cursor %d, want 0", s.cursor)
	}
	// A click far to the right (past the text) parks it at the end.
	s.OnEvent(Event{Kind: EventClick, X: 150, Y: 12})
	if s.cursor != len([]rune("hello")) {
		t.Fatalf("click past text → cursor %d, want end", s.cursor)
	}
}

func TestSearchEntryCursorClamps(t *testing.T) {
	s := NewSearchEntry("hello") // cursor = 5
	s.Text().Set("hi")           // external shrink leaves cursor stale at 5
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	s.SetFocused(true)
	buf := makeSurface(80, 24)
	s.Draw(newP(buf, 80), DefaultLight())
	if s.cursor != 2 {
		t.Fatalf("Draw did not clamp a stale cursor: %d, want 2", s.cursor)
	}
	// An event clamps too — including a negative index up to 0, then a move.
	s.cursor = 99
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // clamps 99→2 then 2→1
	if s.cursor != 1 {
		t.Fatalf("over-length cursor not clamped before move: %d", s.cursor)
	}
	s.cursor = -3
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // clamps -3→0 then 0→1
	if s.cursor != 1 {
		t.Fatalf("negative cursor not clamped before move: %d", s.cursor)
	}
}
