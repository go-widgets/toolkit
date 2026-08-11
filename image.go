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
	// Alt is the image's accessible name — the short description a reader
	// announces in place of the picture. Set it to whatever the source calls the
	// image (a photo's caption, a chart's summary, a post's alt text); leave it
	// empty ONLY for decoration that carries no information, which is the same
	// rule as an empty HTML alt attribute.
	Alt string
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
		dst = FitBounds(i.W, i.H, dst)
	}
	blitImage(p, dst, dst, i.Pixels, i.W, i.H)
}

// FitBounds returns the largest rect of source aspect (srcW:srcH) that fits
// entirely within bounds, centred in it — the geometry [ScaleFit] paints into.
// Consumers can call it to size/lay out an image area (e.g. grow a box to the
// image's fitted height) before drawing. When srcW or srcH is non-positive the
// aspect is unknown and bounds is returned unchanged.
func FitBounds(srcW, srcH int, bounds Rect) Rect {
	if srcW <= 0 || srcH <= 0 {
		return bounds
	}
	w := bounds.W
	h := srcH * w / srcW
	if h > bounds.H {
		h = bounds.H
		w = srcW * h / srcH
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return Rect{X: bounds.X + (bounds.W-w)/2, Y: bounds.Y + (bounds.H-h)/2, W: w, H: h}
}
