// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ScaleMode selects how an [Image]'s source pixels map onto its bounds.
type ScaleMode int

const (
	// ScaleStretch fills the whole bounds, ignoring the source aspect ratio
	// (nearest-neighbour). It is the zero value, so existing callers are
	// unchanged.
	ScaleStretch ScaleMode = iota
	// ScaleFit ("contain") preserves the source aspect ratio, scaling the image
	// to the largest size that fits entirely within the bounds and centring it;
	// the margin around it is left untouched.
	ScaleFit
)

// Image paints a caller-supplied RGBA byte buffer into its bounds. Scaling is
// nearest-neighbour; Scale selects stretch-to-fill (default) or aspect-preserving
// fit-and-centre.
type Image struct {
	Base
	Pixels []byte    // RGBA bytes, W*H*4 in length
	W, H   int       // source dimensions
	Scale  ScaleMode // how the source maps onto the bounds (default ScaleStretch)
}

// NewImage wraps pixels (length must equal w*h*4) + the source dimensions in a
// stretch-to-fill image. Caller owns the pixels; the toolkit just reads them.
func NewImage(pixels []byte, w, h int) *Image {
	return &Image{Pixels: pixels, W: w, H: h}
}

// NewImageFit is [NewImage] with [ScaleFit]: the image preserves its aspect ratio
// and is centred within its bounds.
func NewImageFit(pixels []byte, w, h int) *Image {
	return &Image{Pixels: pixels, W: w, H: h, Scale: ScaleFit}
}

// Draw paints the image into bounds (or, for ScaleFit, into the aspect-preserving
// centred sub-rect of bounds). Scaling is nearest-neighbour.
func (i *Image) Draw(p painter.Painter, theme *Theme) {
	_ = theme // images don't read the theme
	if i.W <= 0 || i.H <= 0 || len(i.Pixels) < i.W*i.H*4 {
		return
	}
	dst := i.Bounds()
	if dst.W <= 0 || dst.H <= 0 {
		return
	}
	if i.Scale == ScaleFit {
		dst = fitRect(i.W, i.H, dst)
	}
	for dy := 0; dy < dst.H; dy++ {
		sy := dy * i.H / dst.H
		for dx := 0; dx < dst.W; dx++ {
			sx := dx * i.W / dst.W
			sOff := (sy*i.W + sx) * 4
			ink := RGBA{R: i.Pixels[sOff], G: i.Pixels[sOff+1], B: i.Pixels[sOff+2], A: i.Pixels[sOff+3]}
			p.PutPixel(dst.X+dx, dst.Y+dy, ink)
		}
	}
}

// fitRect returns the largest rect of source aspect (sw:sh) that fits within b,
// centred in b. sw/sh are assumed positive (Draw guards W/H > 0).
func fitRect(sw, sh int, b Rect) Rect {
	w := b.W
	h := sh * w / sw
	if h > b.H {
		h = b.H
		w = sw * h / sh
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return Rect{X: b.X + (b.W-w)/2, Y: b.Y + (b.H-h)/2, W: w, H: h}
}
