// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// stencil is a picture whose colours are wrong on purpose: a red left half and
// a blue right half, all opaque, with a transparent border. Anything that read
// the SOURCE colour instead of the ink would show red or blue.
func stencil(w, h int) []byte {
	pix := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				continue // transparent border
			}
			if x < w/2 {
				pix[i] = 0xFF
			} else {
				pix[i+2] = 0xFF
			}
			pix[i+3] = 0xFF
		}
	}
	return pix
}

func TestAStencilTakesTheInkAndNotItsOwnColours(t *testing.T) {
	src := stencil(8, 8)
	draw := StencilIcon(src, 8, 8)
	for _, ink := range []RGBA{RGB(0x00, 0xFF, 0x00), RGB(0xFF, 0xFF, 0xFF)} {
		buf := makeSurface(32, 32)
		bg := pixelAt(buf, 32, 0, 0)
		draw(newP(buf, 32), Rect{X: 4, Y: 4, W: 24, H: 24}, ink)
		painted, wrong := 0, 0
		for y := range 32 {
			for x := range 32 {
				got := pixelAt(buf, 32, x, y)
				if got == bg {
					continue
				}
				painted++
				if got != ink {
					wrong++
				}
			}
		}
		if painted == 0 {
			t.Fatalf("ink %v: nothing was drawn", ink)
		}
		if wrong > 0 {
			t.Errorf("ink %v: %d of %d pixels are not the ink; the source's own colours came through",
				ink, wrong, painted)
		}
	}
	// The source is left alone: a caller reusing the pixels elsewhere would
	// otherwise find them recoloured behind its back.
	for i, b := range stencil(8, 8) {
		if src[i] != b {
			t.Fatalf("the source was modified at byte %d", i)
		}
	}
}

func TestAStencilKeepsItsShapeInTheBox(t *testing.T) {
	// Wide source, square box: the picture must not be stretched to fill it.
	draw := StencilIcon(stencil(40, 10), 40, 10)
	buf := makeSurface(48, 48)
	bg := pixelAt(buf, 48, 0, 0)
	draw(newP(buf, 48), Rect{X: 4, Y: 4, W: 40, H: 40}, RGB(0, 0xFF, 0))
	minX, minY, maxX, maxY := 48, 48, -1, -1
	for y := range 48 {
		for x := range 48 {
			if pixelAt(buf, 48, x, y) != bg {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		t.Fatal("nothing was drawn")
	}
	gotW, gotH := maxX-minX+1, maxY-minY+1
	if gotH >= gotW {
		t.Errorf("a 40x10 picture came out %dx%d; it was stretched to the box", gotW, gotH)
	}
	// And centred in it rather than pinned to the top.
	if top, bottom := minY-4, 43-maxY; top-bottom > 2 || bottom-top > 2 {
		t.Errorf("%d above and %d below; the picture is not centred", top, bottom)
	}
}

func TestAStencilWithNothingToDraw(t *testing.T) {
	buf := makeSurface(16, 16)
	bg := pixelAt(buf, 16, 0, 0)
	ink := RGB(0, 0xFF, 0)
	box := Rect{X: 0, Y: 0, W: 16, H: 16}
	// A box with no size, a source with no size, and a source shorter than it
	// claims: each draws nothing rather than reading past its own buffer.
	StencilIcon(stencil(4, 4), 4, 4)(newP(buf, 16), Rect{}, ink)
	StencilIcon(nil, 0, 0)(newP(buf, 16), box, ink)
	StencilIcon(make([]byte, 4), 4, 4)(newP(buf, 16), box, ink)
	for y := range 16 {
		for x := range 16 {
			if pixelAt(buf, 16, x, y) != bg {
				t.Fatalf("something was drawn at %d,%d", x, y)
			}
		}
	}
	// A back end WITHOUT the image primitive draws the same picture one pixel
	// at a time rather than nothing: the primitive is a speed-up, not a
	// requirement, and an icon that vanished on such a back end would be a
	// toolkit that works on some hosts only.
	fast := makeSurface(16, 16)
	slow := makeSurface(16, 16)
	draw := StencilIcon(stencil(8, 8), 8, 8)
	draw(newP(fast, 16), box, ink)
	draw(&nonClipPainter{inner: painter.NewPixelPainter(slow, 16, 16)}, box, ink)
	if string(fast) != string(slow) {
		t.Error("the picture differs on a back end without the image primitive")
	}
}

func TestAStencilTintsOncePerInk(t *testing.T) {
	// The tinted copy is kept: drawing twice with the same ink must give the
	// same picture, and changing the ink must actually change it.
	draw := StencilIcon(stencil(8, 8), 8, 8)
	shot := func(ink RGBA) []byte {
		buf := makeSurface(16, 16)
		draw(newP(buf, 16), Rect{X: 0, Y: 0, W: 16, H: 16}, ink)
		return buf
	}
	green, again := shot(RGB(0, 0xFF, 0)), shot(RGB(0, 0xFF, 0))
	if string(green) != string(again) {
		t.Error("the same ink drew two different pictures")
	}
	if white := shot(RGB(0xFF, 0xFF, 0xFF)); string(white) == string(green) {
		t.Error("a different ink drew the same picture; the tint was not redone")
	}
	// A see-through ink thins the stencil rather than replacing its coverage.
	faint := shot(RGBA{R: 0, G: 0xFF, B: 0, A: 0x80})
	if string(faint) == string(green) {
		t.Error("a half-transparent ink drew the same picture as an opaque one")
	}
}
