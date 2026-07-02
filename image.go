// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Image paints a caller-supplied RGBA byte buffer into its bounds.
// If source dims == bounds, the blit is 1:1; otherwise the image is
// nearest-neighbour scaled to fit the bounds (no aspect-ratio
// preservation in v0.2).
type Image struct {
	Base
	Pixels []byte // RGBA bytes, W*H*4 in length
	W, H   int    // source dimensions
}

// NewImage wraps pixels (length must equal w*h*4) + the source
// dimensions. Caller owns the pixels; the toolkit just reads them.
func NewImage(pixels []byte, w, h int) *Image {
	return &Image{Pixels: pixels, W: w, H: h}
}

// Draw paints the image into bounds. Scaling is nearest-neighbour.
func (i *Image) Draw(p painter.Painter, theme *Theme) {
	_ = theme // images don't read the theme
	r := i.Bounds()
	if i.W <= 0 || i.H <= 0 || len(i.Pixels) < i.W*i.H*4 {
		return
	}
	for dy := 0; dy < r.H; dy++ {
		sy := dy * i.H / r.H
		for dx := 0; dx < r.W; dx++ {
			sx := dx * i.W / r.W
			sOff := (sy*i.W + sx) * 4
			ink := RGBA{R: i.Pixels[sOff], G: i.Pixels[sOff+1], B: i.Pixels[sOff+2], A: i.Pixels[sOff+3]}
			p.PutPixel(r.X+dx, r.Y+dy, ink)
		}
	}
}
