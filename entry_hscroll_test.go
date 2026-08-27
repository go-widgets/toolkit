// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// TestClampEntryScroll covers every branch of the offset clamp: the three
// early-out cases (no room / fits / unfocused), the caret running off the right,
// the caret coming back off the left, the never-past-the-end clamp, and the
// negative guard.
func TestClampEntryScroll(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		cur, caretW, totalW, innerW int
		focused                     bool
		want                        int
	}{
		{"no room", 0, 100, 200, 0, true, 0},
		{"fits", 5, 10, 30, 40, true, 0},
		{"unfocused reads from start", 50, 200, 300, 40, false, 0},
		{"caret off right pins to right edge", 0, 100, 200, 40, true, 60},
		{"caret already visible keeps offset", 60, 80, 200, 40, true, 60},
		{"caret back off left pins to left", 100, 30, 200, 40, true, 30},
		{"clamp to text end", 500, 195, 200, 40, true, 160},
		{"negative guard", -5, 0, 200, 40, true, 0},
	} {
		if got := clampEntryScroll(tc.cur, tc.caretW, tc.totalW, tc.innerW, tc.focused); got != tc.want {
			t.Errorf("%s: clampEntryScroll(%d,%d,%d,%d,%v) = %d, want %d",
				tc.name, tc.cur, tc.caretW, tc.totalW, tc.innerW, tc.focused, got, tc.want)
		}
	}
}

// TestCaretIndexAt covers the click-to-index mapping: left of the field parks at
// 0, far right parks at the end, and a click inside snaps to a boundary strictly
// between the two.
func TestCaretIndexAt(t *testing.T) {
	e := NewEntry("hello world")
	shown := e.display()
	if got := e.caretIndexAt(shown, -3); got != 0 {
		t.Errorf("click left of field: index = %d, want 0", got)
	}
	full := e.textWidth(shown)
	if got := e.caretIndexAt(shown, full+1000); got != len([]rune(shown)) {
		t.Errorf("click past end: index = %d, want %d", got, len([]rune(shown)))
	}
	// A click near the middle lands strictly inside the string.
	mid := e.caretIndexAt(shown, full/2)
	if mid <= 0 || mid >= len([]rune(shown)) {
		t.Errorf("mid click: index = %d, want strictly inside (0,%d)", mid, len([]rune(shown)))
	}
}

// TestEntryScrollFollowsCaret drives a value wider than the field and checks the
// caret stays inside the viewport: at the end the text scrolls left (offset > 0),
// and Home brings it back to the start (offset 0).
func TestEntryScrollFollowsCaret(t *testing.T) {
	e := NewEntry("https://sources.mesocentre.plateau-de-saclay.net/go-tex/examples.git")
	e.SetBounds(Rect{X: 10, Y: 0, W: 120, H: 20})
	e.focused = true
	pad := scaled(entryPadX)
	innerW := e.Bounds().W - 2*pad

	buf := make([]byte, 300*20*4)
	p := painter.NewPixelPainter(buf, 300, 20)
	th := DefaultLight()

	// Caret parked at end by NewEntry: the field must have scrolled.
	e.Draw(p, th)
	if e.scrollX <= 0 {
		t.Fatalf("caret at end of a long value should scroll; scrollX = %d", e.scrollX)
	}
	caretW := e.textWidth(e.Text().Get())
	if v := caretW - e.scrollX; v < 0 || v > innerW {
		t.Fatalf("caret x within viewport want [0,%d], got %d", innerW, v)
	}

	// Home returns to the head of the text.
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	e.Draw(p, th)
	if e.scrollX != 0 {
		t.Fatalf("Home should reset scroll to 0, got %d", e.scrollX)
	}
}

// TestEntryClickPositionsCaret checks a click inside the text moves the caret
// there rather than only focusing.
func TestEntryClickPositionsCaret(t *testing.T) {
	e := NewEntry("abcdefgh")
	e.SetBounds(Rect{X: 10, Y: 0, W: 200, H: 20})
	pad := scaled(entryPadX)
	// Click at the x of the 3rd rune boundary.
	target := e.Bounds().X + pad + e.textWidth("abc")
	e.OnEvent(Event{Kind: EventClick, X: target})
	if !e.focused {
		t.Fatal("click did not focus")
	}
	if e.cursor != 3 {
		t.Fatalf("click positioned caret at %d, want 3", e.cursor)
	}
}

// TestEntryClipsOverflow proves the text is clipped to the field: a value far
// wider than the field paints no ink beyond the field's right border.
func TestEntryClipsOverflow(t *testing.T) {
	const w, h = 200, 20
	e := NewEntry("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	e.SetBounds(Rect{X: 10, Y: 0, W: 40, H: h}) // narrow field, long value
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	e.Draw(p, DefaultLight())

	// Everything the field paints (fill, border, text) is inside [10,50). No
	// pixel to the right of the field's border may be set.
	right := e.Bounds().X + e.Bounds().W
	for y := 0; y < h; y++ {
		for x := right; x < w; x++ {
			i := (y*w + x) * 4
			if buf[i] != 0 || buf[i+1] != 0 || buf[i+2] != 0 || buf[i+3] != 0 {
				t.Fatalf("pixel painted at (%d,%d) past the clipped field right edge %d", x, y, right)
			}
		}
	}
}

// nonClipPainter implements painter.Painter but NOT painter.Clipper, to exercise
// the Entry's graceful-degradation path when the backend cannot clip.
type nonClipPainter struct{ inner *painter.PixelPainter }

func (n *nonClipPainter) FillRect(r painter.Rect, c painter.RGBA) { n.inner.FillRect(r, c) }
func (n *nonClipPainter) StrokeRect(r painter.Rect, c painter.RGBA, lineW int) {
	n.inner.StrokeRect(r, c, lineW)
}
func (n *nonClipPainter) FillRoundRect(r painter.Rect, radius int, c painter.RGBA) {
	n.inner.FillRoundRect(r, radius, c)
}
func (n *nonClipPainter) StrokeRoundRect(r painter.Rect, radius int, c painter.RGBA, lineW int) {
	n.inner.StrokeRoundRect(r, radius, c, lineW)
}
func (n *nonClipPainter) PutPixel(x, y int, c painter.RGBA) { n.inner.PutPixel(x, y, c) }
func (n *nonClipPainter) Text(x, y int, s string, ink painter.RGBA) {
	n.inner.Text(x, y, s, ink)
}
func (n *nonClipPainter) Size() (int, int) { return n.inner.Size() }

// TestEntryDrawWithoutClipper covers the branch where the painter is not a
// Clipper: Draw still renders (no panic, caret tracked) without clipping.
func TestEntryDrawWithoutClipper(t *testing.T) {
	const w, h = 200, 20
	e := NewEntry("a long enough value to exercise the scroll math here")
	e.SetBounds(Rect{X: 5, Y: 0, W: 60, H: h})
	e.focused = true
	buf := make([]byte, w*h*4)
	p := &nonClipPainter{inner: painter.NewPixelPainter(buf, w, h)}
	e.Draw(p, DefaultLight()) // must not panic on the non-Clipper path
	if e.scrollX <= 0 {
		t.Fatalf("expected the long value to scroll even without a clipper, got %d", e.scrollX)
	}
}
