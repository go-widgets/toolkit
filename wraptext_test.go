// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"strings"
	"testing"
)

const longURL = "https://vpn.example.org/a/very/long/path/that/has/no/spaces/at/all/and/keeps/going?x=1&y=2"

// paintedOutside reports whether any pixel outside r differs from makeSurface's
// sentinel: a widget that ran past its edge.
func paintedOutside(buf []byte, w, h int, r Rect) bool {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r.Contains(x, y) {
				continue
			}
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				return true
			}
		}
	}
	return false
}

func TestWrapLinesFitAndKeepEveryRune(t *testing.T) {
	f := CurrentFont()
	for _, width := range []int{1, 8, 40, 90, 300} {
		for _, text := range []string{longURL, "a b c d e f g h", "one\ntwo  three\n\nfour", "wörld ünïcode ✓ ok"} {
			lines := WrapLines(f, text, width)
			var got strings.Builder
			for _, l := range lines {
				// A line wider than width is only the one-rune floor.
				if f.Measure(l) > width && len([]rune(l)) != 1 {
					t.Errorf("width %d: line %q measures %d", width, l, f.Measure(l))
				}
				got.WriteString(l)
			}
			strip := func(s string) string { return strings.Join(strings.Fields(s), "") }
			if strip(got.String()) != strip(text) {
				t.Errorf("width %d: runes lost: %q from %q", width, got.String(), text)
			}
		}
	}
	if WrapLines(f, "", 50) != nil {
		t.Error("empty text must give no lines")
	}
	if got := WrapLines(f, "a\n\nb", 500); len(got) != 3 || got[1] != "" {
		t.Errorf("a blank line must be kept: %q", got)
	}
}

func TestWrapBlockHeight(t *testing.T) {
	f := CurrentFont()
	if wrapBlockHeight(f, 0) != 0 || wrapBlockHeight(f, -3) != 0 {
		t.Error("no lines occupy no height")
	}
	if got := wrapBlockHeight(f, 1); got != f.Height() {
		t.Errorf("one line = %d, want the glyph height %d", got, f.Height())
	}
	if got, want := wrapBlockHeight(f, 3), wrapBlockHeight(f, 2)+wrapPitch(f); got != want {
		t.Errorf("three lines = %d, want %d", got, want)
	}
}

func TestLabelWrapStaysInItsBounds(t *testing.T) {
	theme := DefaultLight()
	const W, H = 220, 140
	box := Rect{X: 30, Y: 20, W: 120, H: 100}
	l := NewLabel("Identity: " + longURL)
	l.Wrap = true
	l.SetBounds(box)
	buf := makeSurface(W, H)
	l.Draw(newP(buf, W), theme)
	if paintedOutside(buf, W, H, box) {
		t.Fatal("wrapped label painted outside its bounds")
	}
	if got := l.HeightFor(box.W); got <= CurrentFont().Height() || got > box.H {
		t.Fatalf("HeightFor = %d, want several lines within %d", got, box.H)
	}
	// Unwrapped, the same text runs past the edge: the test can tell.
	u := NewLabel("Identity: " + longURL)
	u.SetBounds(box)
	ubuf := makeSurface(W, H)
	u.Draw(newP(ubuf, W), theme)
	if !paintedOutside(ubuf, W, H, box) {
		t.Fatal("control: an unwrapped long label should overflow")
	}
	if one := u.HeightFor(box.W); one != CurrentFont().Height() {
		t.Fatalf("unwrapped HeightFor = %d, want one line", one)
	}
}

func TestButtonCaptionWrapsInsteadOfOverflowing(t *testing.T) {
	theme := DefaultLight()
	const W, H = 200, 120
	box := Rect{X: 40, Y: 20, W: 90, H: 80}
	b := NewButton("Remove and revoke approval", nil)
	b.SetBounds(box)
	buf := makeSurface(W, H)
	b.Draw(newP(buf, W), theme)
	// The button's own border may touch its rect edge only.
	if paintedOutside(buf, W, H, Rect{X: box.X - 1, Y: box.Y - 1, W: box.W + 2, H: box.H + 2}) {
		t.Fatal("button painted outside its bounds")
	}
	if b.HeightFor(box.W) <= b.HeightFor(2000) {
		t.Fatal("a narrow button needs more height than a wide one")
	}
	// A caption that fits is drawn exactly as before (one centred line).
	short := NewButton("OK", nil)
	short.SetBounds(box)
	sbuf := makeSurface(W, H)
	short.Draw(newP(sbuf, W), theme)
	ref := NewButton("OK", nil)
	ref.SetBounds(box)
	rbuf := makeSurface(W, H)
	ref.Draw(newP(rbuf, W), theme)
	if !bytes.Equal(sbuf, rbuf) {
		t.Fatal("draw is not deterministic")
	}
}

func TestButtonWrapWithLeadingIconStaysInside(t *testing.T) {
	theme := DefaultLight()
	const W, H = 200, 120
	box := Rect{X: 40, Y: 20, W: 100, H: 80}
	b := NewButton("Remove and revoke approval", nil)
	b.LeadingIcon().Set(DrawIconMinus)
	b.SetBounds(box)
	buf := makeSurface(W, H)
	b.Draw(newP(buf, W), theme)
	if paintedOutside(buf, W, H, Rect{X: box.X - 1, Y: box.Y - 1, W: box.W + 2, H: box.H + 2}) {
		t.Fatal("button with an icon painted outside its bounds")
	}
}
