// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// A dialog with no action Buttons collapses its bottom strip: no SurfaceAlt band
// and no hairline at the bottom, so a modal that carries its controls inside
// Content is a clean card. A dialog WITH buttons keeps the strip.
func TestDialogEmptyButtonsCollapseStrip(t *testing.T) {
	th := DefaultLight()
	const W, H = 200, 140

	bottomBand := func(d *Dialog) (RGBA, int) {
		d.SetBounds(Rect{W: W, H: H})
		buf := make([]byte, 4*W*H)
		d.Draw(painter.NewPixelPainter(buf, W, H), th)
		y := H - scaled(DialogButtonStripH) + 2 // inside where the strip would be
		i := (y*W + W/2) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}, d.stripH()
	}

	// No buttons: zero strip height, and the bottom band is the plain Background
	// body — not the SurfaceAlt strip.
	empty, sh := bottomBand(NewDialog("T", nil))
	if sh != 0 {
		t.Fatalf("a button-less dialog reserves a %d strip, want 0", sh)
	}
	if empty == th.SurfaceAlt {
		t.Errorf("a button-less dialog still painted the SurfaceAlt strip: %v", empty)
	}

	// With a button: a real strip (SurfaceAlt) and a non-zero height.
	withBtn, sh2 := bottomBand(NewDialog("T", nil, NewButton("OK", nil)))
	if sh2 == 0 {
		t.Fatal("a dialog with a button should reserve the action strip")
	}
	if withBtn != th.SurfaceAlt {
		t.Errorf("the action strip should be SurfaceAlt, got %v", withBtn)
	}
}
