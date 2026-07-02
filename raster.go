// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// fillRect paints (x, y, w, h) with c through the Painter p.
// Coordinates may extend past the surface — the painter's own
// clipping prevents OOB writes so callers can pass arbitrary rects
// (e.g. a partially-occluded button) without pre-clipping.
//
// Retained as a package-internal shim so every widget's Draw reads
// naturally as `fillRect(p, r.X, r.Y, r.W, r.H, colour)` rather than
// constructing a Rect at every callsite. A future SIMD fast-path
// lands inside painter.PixelPainter + benefits every widget.
func fillRect(p painter.Painter, x, y, w, h int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.FillRect(painter.Rect{X: x, Y: y, W: w, H: h}, c)
}

// strokeRect paints a 1-pixel border on the outline of (x, y, w, h)
// with c. Used by widgets that draw a frame around their body —
// Button, Frame, focus indicator, etc.
func strokeRect(p painter.Painter, x, y, w, h int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.StrokeRect(painter.Rect{X: x, Y: y, W: w, H: h}, c, 1)
}
