// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// The panel is a rounded sheet: its corners are transparent where a square one
// would have painted, and its outline follows the rounding.
func TestDialogPanelIsRounded(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable = true
	d.SetBounds(Rect{W: 200, H: 140})
	buf := make([]byte, 4*200*140)
	d.Draw(painter.NewPixelPainter(buf, 200, 140), DefaultLight())

	at := func(x, y int) byte { return buf[(y*200+x)*4+3] }
	for _, c := range [][2]int{{0, 0}, {199, 0}, {0, 139}, {199, 139}} {
		if at(c[0], c[1]) != 0 {
			t.Errorf("corner %v is painted; the panel must be rounded", c)
		}
	}
	if at(100, 0) == 0 {
		t.Error("the middle of the top edge must be painted")
	}
	if at(100, 139) == 0 {
		t.Error("the middle of the bottom edge must be painted")
	}
}

// The title bar and the action strip are separated from the body by a hairline,
// so they read as bands rather than one field of colour.
func TestDialogBandsAreSeparated(t *testing.T) {
	th := DefaultLight()
	d := NewDialog("Title", nil)
	d.SetBounds(Rect{W: 200, H: 140})
	buf := make([]byte, 4*200*140)
	d.Draw(painter.NewPixelPainter(buf, 200, 140), th)

	rgb := func(x, y int) RGBA {
		i := (y*200 + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	titleH := scaled(DialogTitleH)
	if got := rgb(100, titleH-strokeWidth()); got != th.Border {
		t.Errorf("no hairline under the title bar: %v at y=%d, want %v", got, titleH-strokeWidth(), th.Border)
	}
	stripY := 140 - scaled(DialogButtonStripH)
	if got := rgb(100, stripY); got != th.Border {
		t.Errorf("no hairline above the action strip: %v at y=%d, want %v", got, stripY, th.Border)
	}
}

// The close control is a real icon on a flat button, not a letter in a box.
func TestDialogCloseButtonIsAFlatIcon(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable = true
	cb := d.closeButton()
	if !cb.Flat {
		t.Error("the close button must be flat: a framed square belongs to the content, not the window")
	}
	if cb.Glyph == nil {
		t.Error("the close button must draw a real icon, not a text glyph")
	}
	if cb.Icon != "" {
		t.Errorf("the stand-in letter must be gone, got %q", cb.Icon)
	}
}
