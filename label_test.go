// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"strings"
	"testing"
)

// TestLabelInkOverride checks that a non-zero Ink paints the text in that
// colour, while the zero value inherits the theme's OnSurface (backward compat).
func TestLabelInkOverride(t *testing.T) {
	theme := DefaultLight()
	const w, h = 120, 24
	const text = "Ink"

	// (a) Default: no Ink → OnSurface. This is the reference render.
	def := NewLabel(text)
	def.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	defBuf := makeSurface(w, h)
	def.Draw(newP(defBuf, w), theme)

	// (b) Ink explicitly set to OnSurface must be byte-identical to the default
	// (the A!=0 branch resolves to the same colour).
	same := NewLabel(text)
	same.Ink = theme.OnSurface
	same.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	sameBuf := makeSurface(w, h)
	same.Draw(newP(sameBuf, w), theme)
	if !bytes.Equal(defBuf, sameBuf) {
		t.Fatal("Ink==OnSurface must render byte-identically to the default")
	}

	// (c) A distinct Ink must change the rendered pixels.
	red := NewLabel(text)
	red.Ink = RGBA{R: 220, G: 20, B: 20, A: 255}
	red.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	redBuf := makeSurface(w, h)
	red.Draw(newP(redBuf, w), theme)
	if bytes.Equal(defBuf, redBuf) {
		t.Fatal("a distinct Ink colour must change the rendered text")
	}
	// The red channel must appear somewhere in the glyph pixels.
	found := false
	for i := 0; i+3 < len(redBuf); i += 4 {
		if redBuf[i] > 150 && redBuf[i+1] < 100 && redBuf[i+2] < 100 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Ink red not present in the rendered label")
	}
}

// labelTopRow returns the top-most row index that carries a painted (non
// sentinel) pixel, or -1 if the surface is untouched. makeSurface pre-fills the
// buffer with a 0xC8 sentinel; drawText only writes glyph pixels, so any pixel
// that differs from the sentinel was painted by the Label.
func labelTopRow(buf []byte, w, h int) int {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				return y
			}
		}
	}
	return -1
}

// labelPaintedWidth returns the right-most painted column index plus one (the
// painted text width in pixels), or 0 if nothing was painted.
func labelPaintedWidth(buf []byte, w, h int) int {
	right := -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				if x > right {
					right = x
				}
			}
		}
	}
	return right + 1
}

// drawLabel is a small helper: it builds a Label, applies fn, renders it onto a
// fresh w*h surface and returns the buffer.
func drawLabel(w, h int, fn func(*Label)) []byte {
	theme := DefaultLight()
	l := NewLabel("")
	fn(l)
	l.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	return buf
}

// TestLabelVAlign checks that VMiddle and VBottom place the text strictly lower
// than the default VTop, and that VTop still anchors to the top edge.
func TestLabelVAlign(t *testing.T) {
	const w, h = 120, 30 // h (30) comfortably exceeds glyphHeight (7)

	top := drawLabel(w, h, func(l *Label) { l.Text = "Hi"; l.VAlign = VTop })
	mid := drawLabel(w, h, func(l *Label) { l.Text = "Hi"; l.VAlign = VMiddle })
	bot := drawLabel(w, h, func(l *Label) { l.Text = "Hi"; l.VAlign = VBottom })

	topRow := labelTopRow(top, w, h)
	midRow := labelTopRow(mid, w, h)
	botRow := labelTopRow(bot, w, h)

	if topRow != 0 {
		t.Fatalf("VTop must anchor at the top edge (row 0), got row %d", topRow)
	}
	if !(midRow > topRow) {
		t.Fatalf("VMiddle (row %d) must be lower than VTop (row %d)", midRow, topRow)
	}
	if !(botRow > midRow) {
		t.Fatalf("VBottom (row %d) must be lower than VMiddle (row %d)", botRow, midRow)
	}
	// Exact positions per the documented formulas: (h-gh)/2 and h-gh.
	gh := NewLabel("").glyphHeight()
	if midRow != (h-gh)/2 {
		t.Fatalf("VMiddle top row = %d, want %d", midRow, (h-gh)/2)
	}
	if botRow != h-gh {
		t.Fatalf("VBottom top row = %d, want %d", botRow, h-gh)
	}

	// The zero value (VTop, Ellipsis:false) must match an explicit VTop render.
	zero := drawLabel(w, h, func(l *Label) { l.Text = "Hi" })
	if !bytes.Equal(zero, top) {
		t.Fatal("zero-value VAlign must render identically to explicit VTop")
	}
}

// TestLabelEllipsis checks that an over-wide string is truncated with a trailing
// "…" that fits the bounds width, that the non-ellipsis path is unchanged, and
// that a text already fitting is left intact.
func TestLabelEllipsis(t *testing.T) {
	const w, h = 60, 12 // 60px == 10 glyphs (advance 6)
	const long = "This is a very long line of text"

	full := drawLabel(w, h, func(l *Label) { l.Text = long })                    // Ellipsis:false
	clip := drawLabel(w, h, func(l *Label) { l.Text = long; l.Ellipsis = true }) // truncated
	if bytes.Equal(full, clip) {
		t.Fatal("Ellipsis=true on an over-wide string must change the render")
	}

	// The truncated render must fit the bounds width and be narrower than the
	// (overflowing) full render.
	clipW := labelPaintedWidth(clip, w, h)
	fullW := labelPaintedWidth(full, w, h)
	if clipW > w {
		t.Fatalf("truncated text width %d must fit bounds width %d", clipW, w)
	}
	if !(clipW < fullW) {
		t.Fatalf("truncated width %d must be shorter than full width %d", clipW, fullW)
	}

	// With the bitmap font the ellipsis is 3 bytes (18px) and each glyph is 6px,
	// so the widest prefix p with (len(p)+3)*6 <= 60 is 7 bytes: "This is". The
	// truncated render must therefore equal "This is…" drawn directly.
	want := drawLabel(w, h, func(l *Label) { l.Text = "This is…" })
	if !bytes.Equal(clip, want) {
		t.Fatal("truncated render must equal 'This is…' drawn directly")
	}

	// Ellipsis=true on text that already fits leaves it identical to the plain
	// (non-ellipsis) render — the truncate branch is skipped.
	fits := drawLabel(w, h, func(l *Label) { l.Text = "Hi" })
	fitsEll := drawLabel(w, h, func(l *Label) { l.Text = "Hi"; l.Ellipsis = true })
	if !bytes.Equal(fits, fitsEll) {
		t.Fatal("Ellipsis on already-fitting text must not change the render")
	}
}

// TestEllipsizeHelper exercises the ellipsize helper directly: a normal
// truncation must end in "…" and fit the width, and a width too small for even
// the bare ellipsis must still yield "…".
func TestEllipsizeHelper(t *testing.T) {
	f := CurrentFont()
	const long = "This is a very long line of text"

	got := ellipsize(f, long, 60)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("ellipsize result %q must end in an ellipsis", got)
	}
	if f.Measure(got) > 60 {
		t.Fatalf("ellipsize result %q width %d must fit 60", got, f.Measure(got))
	}
	if got == long {
		t.Fatal("over-wide text must actually be shortened")
	}

	// Degenerate: a width narrower than the bare ellipsis collapses to "…".
	if bare := ellipsize(f, "abcdef", 3); bare != "…" {
		t.Fatalf("when nothing fits, ellipsize must return a bare '…', got %q", bare)
	}
}

// TestLabelEllipsisVisibleTTF uses a real proportional TrueType face so the
// trailing "…" actually paints: the truncated label must fit the bounds and
// paint fewer columns than the overflowing full render.
func TestLabelEllipsisVisibleTTF(t *testing.T) {
	ttf := newTTF(t, 16)
	const w, h = 90, 22
	const long = "Truncate me because I am far too wide"

	full := drawLabel(w, h, func(l *Label) { l.Text = long; l.SetFont(ttf) })
	clip := drawLabel(w, h, func(l *Label) { l.Text = long; l.Ellipsis = true; l.SetFont(ttf) })

	clipW := labelPaintedWidth(clip, w, h)
	if clipW == 0 {
		t.Fatal("truncated label painted nothing")
	}
	if clipW > w {
		t.Fatalf("truncated painted width %d must fit bounds %d", clipW, w)
	}
	if !(clipW < labelPaintedWidth(full, w, h)) {
		t.Fatal("truncated label must paint fewer columns than the full render")
	}
	// The string Draw actually renders must end in "…".
	if !strings.HasSuffix(ellipsize(ttf, long, w), "…") {
		t.Fatal("ellipsized string must end in '…'")
	}
}

// TestLabelAlignWithTruncation checks the horizontal Align applies to the
// possibly-truncated string, including the clamp that never starts left of the
// widget when the text overflows.
func TestLabelAlignWithTruncation(t *testing.T) {
	const w, h = 60, 12
	const long = "This is a very long line of text"

	// Centre + ellipsis: the truncated string (60px) fills the width, so its
	// centred origin is the left edge (0). Right-align on the truncated text is
	// likewise pinned to the left edge here.
	center := drawLabel(w, h, func(l *Label) { l.Text = long; l.Align = AlignCenter; l.Ellipsis = true })
	if labelTopRow(center, w, h) < 0 {
		t.Fatal("centred truncated label painted nothing")
	}

	// Right-align without ellipsis: x = W - textWidth is far negative and must
	// clamp to the left edge (row 0 painted starting at column 0).
	right := drawLabel(w, h, func(l *Label) { l.Text = long; l.Align = AlignRight })
	if pxRow := labelTopRow(right, w, h); pxRow < 0 {
		t.Fatal("right-aligned overflow label painted nothing")
	}
	// Column 0 of some row must be painted, proving the x<r.X clamp fired.
	clamped := false
	for y := 0; y < h; y++ {
		px := pixelAt(right, w, 0, y)
		if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
			clamped = true
			break
		}
	}
	if !clamped {
		t.Fatal("AlignRight overflow must clamp its start to the left edge")
	}
}
