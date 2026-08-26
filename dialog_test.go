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

	// A rounded corner does not carry the panel's own paint. Asserting merely
	// "unpainted" was too coarse: the drop shadow, offset down and right, lands
	// on the corners it falls past, and that is the shadow doing its job.
	at := func(x, y int) RGBA {
		i := (y*200 + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	topEdge, bottomEdge := at(100, 0), at(100, 139)
	if topEdge.A == 0 {
		t.Error("the middle of the top edge must be painted")
	}
	if bottomEdge.A == 0 {
		t.Error("the middle of the bottom edge must be painted")
	}
	for _, c := range [][2]int{{0, 0}, {199, 0}, {0, 139}, {199, 139}} {
		if got := at(c[0], c[1]); got == topEdge || got == bottomEdge {
			t.Errorf("corner %v carries the panel's own paint %v; it must be rounded away", c, got)
		}
	}
}

// The panel casts a shadow down and to the right, which is what makes a rounded
// sheet read as floating rather than merely as a rounded region. Rounding alone
// was measured on the live playground and was invisible: the corner showed the
// dark scrim through it at [16,18,21] where the edge read [58,62,70].
func TestDialogCastsAShadow(t *testing.T) {
	const W, H = 260, 200
	buf := make([]byte, 4*W*H)
	p := painter.NewPixelPainter(buf, W, H)
	d := NewDialog("Title", nil)
	d.SetBounds(Rect{X: 20, Y: 20, W: 200, H: 140})
	d.Draw(p, DefaultLight())

	at := func(x, y int) RGBA {
		i := (y*W + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	drop := scaled(DialogShadow)
	// Below the middle of the bottom edge, inside the shadow's offset. The
	// CORNERS are the wrong place to probe: the shadow is rounded too, so it has
	// already curved away there.
	if got := at(20+100, 20+140+drop/2); got.A == 0 {
		t.Errorf("no shadow below the panel: %v", got)
	}
	// And nothing further out than the shadow reaches.
	if got := at(20+100, 20+140+drop+2); got.A != 0 {
		t.Errorf("the shadow must not extend past its offset: %v", got)
	}
	// Same to the right.
	if got := at(20+200+drop/2, 20+70); got.A == 0 {
		t.Errorf("no shadow to the right of the panel: %v", got)
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
