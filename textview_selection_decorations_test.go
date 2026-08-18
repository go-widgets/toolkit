// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

// A TextView with a non-empty Selection paints a highlight band over the
// selected range, so a keyboard- or mouse-made selection is visible. The band
// is translucent, so rather than scan for an exact colour we prove it changed
// the pixels the unselected render left bare.
func TestTextViewPaintsSelectionBand(t *testing.T) {
	const w, h = 160, 40
	theme := DefaultDark()

	bare := NewTextView("hello world\nsecond line")
	bare.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	bare.Focused = true
	b0 := makeSurface(w, h)
	bare.Draw(newP(b0, w), theme)

	sel := NewTextView("hello world\nsecond line")
	sel.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	sel.Focused = true
	sel.Selection = Selection{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5} // "hello"
	b1 := makeSurface(w, h)
	sel.Draw(newP(b1, w), theme)

	if bytes.Equal(b0, b1) {
		t.Fatal("a non-empty Selection painted no band — render is identical to no selection")
	}

	// Clearing the selection returns to the bare pixels: the band is the only
	// difference.
	sel.Selection = Selection{}
	b2 := makeSurface(w, h)
	sel.Draw(newP(b2, w), theme)
	if !bytes.Equal(b0, b2) {
		t.Fatal("clearing the Selection did not return to the unselected pixels")
	}
}

// A multi-line selection highlights the trailing part of the first line, the
// whole of any spanned lines, and the leading part of the last line — proving
// the band is not confined to a single row.
func TestTextViewPaintsMultiLineSelection(t *testing.T) {
	const w, h = 160, 60
	theme := DefaultDark()

	single := NewTextView("aaaa\nbbbb\ncccc")
	single.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	single.Selection = Selection{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 4}
	bs := makeSurface(w, h)
	single.Draw(newP(bs, w), theme)

	multi := NewTextView("aaaa\nbbbb\ncccc")
	multi.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	multi.Selection = Selection{StartLine: 0, StartCol: 2, EndLine: 2, EndCol: 2}
	bm := makeSurface(w, h)
	multi.Draw(newP(bm, w), theme)

	if bytes.Equal(bs, bm) {
		t.Fatal("a multi-line selection painted the same as a single-line one")
	}
}

// A remote co-editor Decoration paints its caret + name tag in the co-editor's
// own opaque colour (so it is exactly findable) and tints its selection band.
func TestTextViewPaintsCoEditorDecoration(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultDark()
	magenta := RGBA{R: 0xFF, G: 0x00, B: 0xFF, A: 0xFF}

	bare := NewTextView("hello world\nsecond line")
	bare.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b0 := makeSurface(w, h)
	bare.Draw(newP(b0, w), theme)
	if scanHasColor(b0, w, 0, 0, w-1, h-1, magenta) {
		t.Fatal("test colour must be absent before a decoration is added")
	}

	deco := NewTextView("hello world\nsecond line")
	deco.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	deco.Decorations = []Decoration{{
		Label:      "Bob",
		Color:      magenta,
		CursorLine: 0, CursorCol: 3,
		Selection: Selection{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 6},
	}}
	b1 := makeSurface(w, h)
	deco.Draw(newP(b1, w), theme)

	// The opaque caret + tag paint exact magenta.
	if !scanHasColor(b1, w, 0, 0, w-1, h-1, magenta) {
		t.Error("co-editor caret/tag not painted in the co-editor colour")
	}
	// And the whole render differs from the undecorated one (band + caret).
	if bytes.Equal(b0, b1) {
		t.Error("a Decoration changed nothing on screen")
	}
}

// A decoration whose caret line is scrolled out of view, or whose caret column
// sits past the right edge, is skipped without panicking (the culling paths).
func TestTextViewDecorationCulling(t *testing.T) {
	const w, h = 120, 30 // ~1–2 visible rows
	theme := DefaultDark()
	v := NewTextView("l0\nl1\nl2\nl3\nl4\nl5")
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v.ScrollLine = 3                                  // lines 0..2 are above the viewport
	white := RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // bright tag → black tag ink
	v.Decorations = []Decoration{
		{Label: "above", Color: RGBA{R: 1, A: 0xFF}, CursorLine: 0, CursorCol: 0},      // scrolled off the top
		{Label: "offright", Color: RGBA{G: 1, A: 0xFF}, CursorLine: 3, CursorCol: 999}, // past the right edge
		{Label: "", Color: RGBA{B: 1, A: 0xFF}, CursorLine: 4, CursorCol: 1},           // visible, no tag
		{Label: "Ann", Color: white, CursorLine: 3, CursorCol: 1},                      // visible, bright tag
	}
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme) // must not panic
	_ = buf
}
