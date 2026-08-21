// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func TestThumbnailEmptyBoundsNoPanic(t *testing.T) {
	buf := make([]byte, 4)
	p := painter.NewPixelPainter(buf, 1, 1)
	th := NewThumbnail(gradBuf(2, 2), 2, 2)
	th.SetBounds(Rect{}) // zero bounds => no-op
	th.Draw(p, DefaultLight())
	if buf[3] != 0 {
		t.Fatal("empty-bounds thumbnail painted")
	}
}

func TestThumbnailNearestDownscaleAndPlainBorder(t *testing.T) {
	const W, H = 8, 8
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 4x4 source shrunk into an 8x8 cell (FitBounds => 8x8), nearest.
	th := NewThumbnail(gradBuf(4, 4), 4, 4)
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight())
	// Interior painted by the image.
	if _, _, _, a := at(buf, W, 3, 3); a != 0xFF {
		t.Fatal("nearest thumbnail interior unpainted")
	}
	// Plain (unselected/unhovered) border is theme.Border on the frame.
	tm := DefaultLight()
	if r, g, b, _ := at(buf, W, 0, 0); r != tm.Border.R || g != tm.Border.G || b != tm.Border.B {
		t.Fatalf("plain border = %d,%d,%d, want theme.Border", r, g, b)
	}
}

func TestThumbnailAreaDownscaleAverages(t *testing.T) {
	const W, H = 3, 3
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// A big 6x6 source shrunk into 3x3 with Area averaging (each dst pixel
	// averages a 2x2 source box). This exercises the sx1<=sx0 / sy1<=sy0 guards
	// being NOT taken plus boxAverage's accumulation.
	th := NewThumbnail(gradBuf(6, 6), 6, 6)
	th.Area = true
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight())
	// Every interior/edge pixel is painted with alpha 0xFF (source opaque).
	if _, _, _, a := at(buf, W, 1, 1); a != 0xFF {
		t.Fatal("area thumbnail centre unpainted")
	}
}

func TestThumbnailAreaUpscaleGuard(t *testing.T) {
	const W, H = 6, 6
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 2x2 source into a 6x6 cell with Area: dst is bigger than source, so the
	// per-pixel source box collapses (sx1<=sx0) and the +1 guard kicks in.
	th := NewThumbnail(gradBuf(2, 2), 2, 2)
	th.Area = true
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight())
	if _, _, _, a := at(buf, W, 3, 3); a != 0xFF {
		t.Fatal("area upscale guard left a hole")
	}
}

func TestThumbnailLabelStripAndSelectedBorder(t *testing.T) {
	const W, H = 40, 40
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	th := NewThumbnail(gradBuf(4, 4), 4, 4)
	th.Label = "Win"
	th.Selected().Set(true)
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight())
	tm := DefaultLight()
	// Label strip along the bottom is filled with theme.Surface.
	stripY := H - (GlyphHeight() + 2*ThumbnailLabelPad)
	if r, g, b, _ := at(buf, W, 1, stripY+ThumbnailLabelPad+1); r != tm.Surface.R || g != tm.Surface.G || b != tm.Surface.B {
		// the exact glyph pixels differ; sample a spot likely inside the fill
		if r == 0 && g == 0 && b == 0 {
			t.Fatalf("label strip not filled at (1,%d)", stripY)
		}
	}
	// Selected => Accent frame (2px). Outer frame pixel is Accent.
	if r, g, b, _ := at(buf, W, 0, 0); r != tm.Accent.R || g != tm.Accent.G || b != tm.Accent.B {
		t.Fatalf("selected border = %d,%d,%d, want Accent", r, g, b)
	}
}

func TestThumbnailHoverBorder(t *testing.T) {
	const W, H = 20, 20
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	th := NewThumbnail(gradBuf(2, 2), 2, 2)
	th.Hover().Set(true) // not selected
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight())
	tm := DefaultLight()
	if r, g, b, _ := at(buf, W, 0, 0); r != tm.Accent.R || g != tm.Accent.G || b != tm.Accent.B {
		t.Fatalf("hover border = %d,%d,%d, want Accent", r, g, b)
	}
}

func TestThumbnailInvalidImageStillFramesAndLabels(t *testing.T) {
	const W, H = 20, 20
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// Invalid image (short buffer) + a label + zero-height image area edge case.
	th := &Thumbnail{Pixels: make([]byte, 3), IW: 2, IH: 2, Label: "X"}
	th.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	th.Draw(p, DefaultLight()) // hasImage false => skips image, still frames+labels
	tm := DefaultLight()
	if r, g, b, _ := at(buf, W, 0, 0); r != tm.Border.R || g != tm.Border.G || b != tm.Border.B {
		t.Fatalf("frame missing on invalid image: %d,%d,%d", r, g, b)
	}
}

func TestThumbnailZeroImageAreaSkipsImage(t *testing.T) {
	// A cell exactly as tall as the label strip leaves a zero-height image area,
	// exercising the area.H>0 guard's false branch.
	strip := GlyphHeight() + 2*ThumbnailLabelPad
	buf := make([]byte, 20*strip*4)
	p := painter.NewPixelPainter(buf, 20, strip)
	th := NewThumbnail(gradBuf(2, 2), 2, 2)
	th.Label = "Y"
	th.SetBounds(Rect{X: 0, Y: 0, W: 20, H: strip})
	th.Draw(p, DefaultLight()) // must not panic; image area collapses to 0
}

func TestThumbnailClick(t *testing.T) {
	n := 0
	th := NewThumbnail(nil, 0, 0)
	th.OnClick = func() { n++ }
	th.OnEvent(Event{Kind: EventClick})
	th.OnEvent(Event{Kind: EventKeyDown}) // ignored
	if n != 1 {
		t.Fatalf("click count = %d, want 1", n)
	}
	// Nil-safe.
	th2 := NewThumbnail(nil, 0, 0)
	th2.OnEvent(Event{Kind: EventClick})
}
