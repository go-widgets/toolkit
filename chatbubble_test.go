// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

// --- API surface ---------------------------------------------------------

func TestChatBubbleConstants(t *testing.T) {
	if ChatBubblePadX != 10 || ChatBubblePadY != 6 ||
		ChatBubbleMaxW != 220 || ChatBubbleLineSpacing != 2 {
		t.Fatalf("constants drifted: PadX=%d PadY=%d MaxW=%d LineSpacing=%d",
			ChatBubblePadX, ChatBubblePadY, ChatBubbleMaxW, ChatBubbleLineSpacing)
	}
	if ChatFromUser == ChatFromOther {
		t.Fatal("ChatFromUser must differ from ChatFromOther")
	}
}

func TestNewChatBubble(t *testing.T) {
	c := NewChatBubble("hi", ChatFromUser)
	if c.Text != "hi" || c.Sender != ChatFromUser {
		t.Fatalf("bubble = %+v, want text=hi sender=User", c)
	}
	c2 := NewChatBubble("", ChatFromOther)
	if c2.Text != "" || c2.Sender != ChatFromOther {
		t.Fatalf("empty bubble = %+v", c2)
	}
}

// --- Draw: ChatFromUser (right-aligned, Accent fill) --------------------

func TestChatBubbleDrawFromUser(t *testing.T) {
	const w, h = 260, 40
	theme := DefaultLight()
	c := NewChatBubble("HI", ChatFromUser)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// Expected bubble width for "HI": TextWidth("HI") = 12; bubbleW = 12+20 = 32.
	// Right-aligned: bx = w - 32 = 228; interior pixel at (240, 10) is Accent.
	if got := pixelAt(buf, w, 240, 10); got != theme.Accent {
		t.Fatalf("interior user fill = %+v, want Accent", got)
	}
	// Top-left of bubble at (228, 0): border.
	if got := pixelAt(buf, w, 228, 0); got != theme.Border {
		t.Fatalf("border pixel = %+v, want Border", got)
	}
	// Text ink somewhere inside the bubble is theme.Background (accentInk fallback).
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 228; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected Background-tone ink inside user bubble")
	}
}

// --- Draw: ChatFromOther (left-aligned, SurfaceAlt fill) ----------------

func TestChatBubbleDrawFromOther(t *testing.T) {
	const w, h = 260, 40
	theme := DefaultLight()
	c := NewChatBubble("HI", ChatFromOther)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// Interior pixel just inside left edge is SurfaceAlt.
	if got := pixelAt(buf, w, 16, 10); got != theme.SurfaceAlt {
		t.Fatalf("interior other fill = %+v, want SurfaceAlt", got)
	}
	// Top-left of bubble at (0, 0): border.
	if got := pixelAt(buf, w, 0, 0); got != theme.Border {
		t.Fatalf("border pixel = %+v, want Border", got)
	}
	// OnSurface ink present.
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 0; x < 40 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected OnSurface ink inside other bubble")
	}
}

// --- Draw: dark theme ---------------------------------------------------

func TestChatBubbleDrawDarkTheme(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultDark()
	c := NewChatBubble("HI", ChatFromOther)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 16, 10); got != theme.SurfaceAlt {
		t.Fatalf("dark other fill = %+v, want SurfaceAlt", got)
	}
}

// --- Draw: multi-line text ----------------------------------------------

func TestChatBubbleDrawMultiLine(t *testing.T) {
	const w, h = 200, 60
	theme := DefaultLight()
	c := NewChatBubble("A\nBB\nCCC", ChatFromOther)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// The third line ("CCC", width=18) sits at y = padY + 2*(GlyphHeight+LineSpacing)
	// = 6 + 2*9 = 24. Its glyphs are 7 rows tall so ink should appear
	// somewhere in rows 24..30. Just verify some OnSurface ink exists past
	// the first-line region.
	found := false
	for y := 20; y < 32 && !found; y++ {
		for x := ChatBubblePadX; x < ChatBubblePadX+30 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected multi-line ink past row 20")
	}
}

// --- Draw: line longer than ChatBubbleMaxW triggers width cap -----------

func TestChatBubbleDrawWidthCap(t *testing.T) {
	const w, h = 400, 40
	theme := DefaultLight()
	longText := strings.Repeat("A", 60) // TextWidth = 6*60 = 360 > 220-20 = 200.
	c := NewChatBubble(longText, ChatFromOther)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// Bubble width capped at ChatBubbleMaxW = 220. Check bottom-pad row
	// (y=15) so we avoid pixels where the overflow text ink lands:
	//   - (100, 15) sits inside the bubble body's bottom pad -> SurfaceAlt.
	//   - (ChatBubbleMaxW+5, 15) sits past the cap -> untouched sentinel.
	if got := pixelAt(buf, w, 100, 15); got != theme.SurfaceAlt {
		t.Fatalf("interior at (100,15) = %+v, want SurfaceAlt", got)
	}
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	if got := pixelAt(buf, w, ChatBubbleMaxW+5, 15); got != sentinel {
		t.Fatalf("bubble fill exceeded ChatBubbleMaxW: (%d,15) = %+v",
			ChatBubbleMaxW+5, got)
	}
}

// --- Draw: OnAccent override ---------------------------------------------

func TestChatBubbleDrawUsesOnAccentOverride(t *testing.T) {
	const w, h = 260, 40
	theme := DefaultLight()
	custom := RGB(0x12, 0x34, 0x56)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	c := NewChatBubble("HI", ChatFromUser)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	found := false
	for y := 0; y < h && !found; y++ {
		for x := 220; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == custom {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("custom OnAccent ink missing from user bubble")
	}
}

// --- Draw: Extra map without OnAccent falls back to Background ---------

func TestChatBubbleDrawExtraWithoutOnAccent(t *testing.T) {
	const w, h = 260, 40
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{"OtherKey": RGB(9, 9, 9)}
	c := NewChatBubble("HI", ChatFromUser)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// Fallback ink = theme.Background.
	found := false
	for y := 0; y < h && !found; y++ {
		for x := 220; x < w && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("fallback Background ink missing from user bubble")
	}
}

// --- Draw: zero-width bounds --------------------------------------------

func TestChatBubbleDrawZeroWidthNoPanic(t *testing.T) {
	theme := DefaultLight()
	c := NewChatBubble("hi", ChatFromUser)
	c.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 40})
	c.Draw(newP(makeSurface(1, 40), 1), theme)
}

// --- Draw: empty text ---------------------------------------------------

func TestChatBubbleDrawEmptyText(t *testing.T) {
	const w, h = 100, 40
	theme := DefaultLight()
	c := NewChatBubble("", ChatFromOther)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// Empty text: bubble is 20x19 (2*PadX x GlyphHeight+2*PadY). Interior
	// pixel at (10,10) is inside the SurfaceAlt fill.
	if got := pixelAt(buf, w, 10, 10); got != theme.SurfaceAlt {
		t.Fatalf("empty bubble interior = %+v, want SurfaceAlt", got)
	}
}
