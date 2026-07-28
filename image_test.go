// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// rgbaBuf builds a w*h opaque source with a per-pixel colour so scaling/placement
// can be checked. The colour encodes the pixel's column and row.
func rgbaBuf(w, h int) []byte {
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			buf[o], buf[o+1], buf[o+2], buf[o+3] = byte(x+1), byte(y+1), 0x40, 0xFF
		}
	}
	return buf
}

// at returns the RGBA at (x,y) in a w-wide RGBA byte buffer.
func at(buf []byte, w, x, y int) (r, g, b, a byte) {
	o := (y*w + x) * 4
	return buf[o], buf[o+1], buf[o+2], buf[o+3]
}

func TestImageStretchFillsBounds(t *testing.T) {
	const w, h = 6, 6
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	img := NewImage(rgbaBuf(2, 2), 2, 2) // default ScaleStretch
	img.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	img.Draw(p, nil)
	// Every destination pixel is painted (alpha 0xFF everywhere).
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if _, _, _, a := at(buf, w, x, y); a != 0xFF {
				t.Fatalf("stretch left (%d,%d) unpainted", x, y)
			}
		}
	}
	// Top-left maps to source (0,0)=(1,1); bottom-right to source (1,1)=(2,2).
	if r, g, _, _ := at(buf, w, 0, 0); r != 1 || g != 1 {
		t.Fatalf("top-left = (%d,%d), want source (1,1)", r, g)
	}
	if r, g, _, _ := at(buf, w, w-1, h-1); r != 2 || g != 2 {
		t.Fatalf("bottom-right = (%d,%d), want source (2,2)", r, g)
	}
}

func TestImageFitPreservesAspectAndCentres(t *testing.T) {
	const w, h = 10, 10
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	// A 4:1 wide source fitted into a 10x10 box: drawn 10x2, centred at rows 4-5.
	img := NewImageFit(rgbaBuf(4, 1), 4, 1)
	img.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	img.Draw(p, nil)
	if _, _, _, a := at(buf, w, 5, 4); a != 0xFF {
		t.Fatal("fit did not paint the centred band")
	}
	if _, _, _, a := at(buf, w, 5, 0); a != 0 {
		t.Fatal("fit painted into the top margin (aspect not preserved)")
	}
	if _, _, _, a := at(buf, w, 5, 9); a != 0 {
		t.Fatal("fit painted into the bottom margin")
	}
}

func TestImageDrawGuards(t *testing.T) {
	const w, h = 4, 4
	zero := func() []byte { return make([]byte, w*h*4) }
	allZero := func(buf []byte) bool {
		for _, v := range buf {
			if v != 0 {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name string
		img  *Image
		b    Rect
	}{
		{"zero-source-dims", NewImage(nil, 0, 0), Rect{W: w, H: h}},
		{"short-pixels", &Image{Pixels: make([]byte, 3), W: 2, H: 2}, Rect{W: w, H: h}},
		{"zero-bounds", NewImage(rgbaBuf(2, 2), 2, 2), Rect{}},
	}
	for _, c := range cases {
		buf := zero()
		p := painter.NewPixelPainter(buf, w, h)
		c.img.SetBounds(c.b)
		c.img.Draw(p, nil) // must not panic
		if !allZero(buf) {
			t.Fatalf("%s: expected no paint", c.name)
		}
	}
}

func TestFitRect(t *testing.T) {
	b := Rect{X: 0, Y: 0, W: 10, H: 10}
	// Wide source: width-bound, letterboxed vertically.
	if got := fitRect(4, 1, b); got != (Rect{X: 0, Y: 4, W: 10, H: 2}) {
		t.Fatalf("wide fit = %+v", got)
	}
	// Tall source: height-bound (exercises the h>b.H branch), pillarboxed.
	if got := fitRect(1, 4, b); got != (Rect{X: 4, Y: 0, W: 2, H: 10}) {
		t.Fatalf("tall fit = %+v", got)
	}
	// Degenerate aspects clamp to a 1px minimum (w<1 and h<1 guards).
	if got := fitRect(1, 1000, b); got.W != 1 {
		t.Fatalf("ultra-tall W = %d, want 1", got.W)
	}
	if got := fitRect(1000, 1, b); got.H != 1 {
		t.Fatalf("ultra-wide H = %d, want 1", got.H)
	}
}
