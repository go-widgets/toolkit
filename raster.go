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

// fillRoundRect fills (x, y, w, h) with c, corners rounded to radius, through
// the Painter (anti-aliased on pixel back-ends; square on a cell grid). The
// naming mirrors fillRect so a widget's Draw reads as
// `fillRoundRect(p, r.X, r.Y, r.W, r.H, radius, colour)`.
func fillRoundRect(p painter.Painter, x, y, w, h, radius int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.FillRoundRect(painter.Rect{X: x, Y: y, W: w, H: h}, radius, c)
}

// strokeRoundRect paints a 1-pixel rounded border on (x, y, w, h) with c.
func strokeRoundRect(p painter.Painter, x, y, w, h, radius int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.StrokeRoundRect(painter.Rect{X: x, Y: y, W: w, H: h}, radius, c, 1)
}

// dimInk returns a mid-tone RGBA that reads as a "dim label" against
// theme.Surface in ANY theme. It's a 60/40 blend of OnSurface and
// Surface — enough contrast against Surface to stay readable, less
// weight than OnSurface itself so it forms a visual hierarchy.
//
// Widgets that need a subordinate/muted text tone (HeaderBar
// Subtitle, ActionRow Subtitle, Stat Title, Timeline event Detail,
// …) should use this instead of theme.Border. The old convention
// of using theme.Border for dim text was fine in the default light
// palette where Border is a mid-grey visible on white Surface, but
// broke in dark themes where Border is deliberately close to
// Surface (compact border strokes look better with low contrast).
func dimInk(theme *Theme) RGBA {
	on, sf := theme.OnSurface, theme.Surface
	return RGBA{
		R: uint8((3*int(on.R) + 2*int(sf.R)) / 5),
		G: uint8((3*int(on.G) + 2*int(sf.G)) / 5),
		B: uint8((3*int(on.B) + 2*int(sf.B)) / 5),
		A: 255,
	}
}
