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

// strokeRect paints a one-LOGICAL-pixel border on the outline of (x, y, w, h)
// with c. Used by widgets that draw a frame around their body — Button, Frame,
// focus indicator, etc.
//
// One logical pixel, not one device pixel: a border is a metric like any other,
// and at twice the resolution it has to be twice as thick or it reads as a
// hairline against chrome that grew around it. This one line is why Button,
// Card, Badge, ProgressBar and every other framed widget scale at all.
func strokeRect(p painter.Painter, x, y, w, h int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.StrokeRect(painter.Rect{X: x, Y: y, W: w, H: h}, c, strokeWidth())
}

// strokeWidth is one logical pixel in device pixels, never less than one: a
// border rounded to zero is a border that disappeared.
func strokeWidth() int {
	if w := scaled(1); w > 1 {
		return w
	}
	return 1
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

// strokeRoundRect paints a one-logical-pixel rounded border on (x, y, w, h)
// with c. See strokeRect on the thickness.
func strokeRoundRect(p painter.Painter, x, y, w, h, radius int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	p.StrokeRoundRect(painter.Rect{X: x, Y: y, W: w, H: h}, radius, c, strokeWidth())
}

// drawLine paints a 1-unit-wide line from (x0, y0) to (x1, y1) with c using
// Bresenham's algorithm over putPixel — the only primitive both back-ends
// share for arbitrary slopes (on a CellPainter each pixel promotes to a filled
// cell). Chart widgets use it for polylines and axis rules.
func drawLine(p painter.Painter, x0, y0, x1, y1 int, c RGBA) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		putPixel(p, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
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

// blitImage puts a block of RGBA pixels on the surface: src is srcW*srcH
// pixels, sampled nearest-neighbour into dst and restricted to clip (pass dst
// for "no restriction beyond the destination itself").
//
// Widgets showing an image used to spell this out one pixel at a time — Image,
// Thumbnail, Wallpaper and Browser each carried their own copy of the same two
// loops. A back-end carrying painter.ImagePainter now does it in one call,
// deciding per ROW rather than per pixel, which measured 7.5x on a full
// 1000x700 window (painter v0.5.0).
//
// The capability is an optimisation, never a requirement: a back-end without it
// gets exactly the loop the widgets used to write, so output is identical
// either way. That equivalence is what TestBlitImageMatchesThePerPixelLoop
// asserts byte for byte.
func blitImage(p painter.Painter, dst, clip Rect, src []byte, srcW, srcH int) {
	if srcW <= 0 || srcH <= 0 || dst.W <= 0 || dst.H <= 0 || len(src) < srcW*srcH*4 {
		return
	}
	if ip, ok := p.(painter.ImagePainter); ok {
		// The enclosing case is tested FIRST, and not merely as the consolation
		// prize for a back-end without a Clipper: pushing a clip that cannot bite
		// still costs the primitive its fast paths, which only run unclipped.
		// Image and Thumbnail pass their own destination as the clip, so this is
		// the common case, not the corner one.
		if rectEncloses(clip, dst) {
			ip.DrawImage(dst, src, srcW, srcH)
			return
		}
		if clr, canClip := p.(painter.Clipper); canClip {
			clr.PushClip(clip)
			ip.DrawImage(dst, src, srcW, srcH)
			clr.PopClip()
			return
		}
	}
	for dy := 0; dy < dst.H; dy++ {
		y := dst.Y + dy
		if y < clip.Y || y >= clip.Y+clip.H {
			continue
		}
		sy := dy * srcH / dst.H
		for dx := 0; dx < dst.W; dx++ {
			x := dst.X + dx
			if x < clip.X || x >= clip.X+clip.W {
				continue
			}
			o := (sy*srcW + dx*srcW/dst.W) * 4
			putPixel(p, x, y, RGBA{R: src[o], G: src[o+1], B: src[o+2], A: src[o+3]})
		}
	}
}

// rectEncloses reports whether outer covers every pixel of inner. Rect.Contains
// answers the same question for a POINT, hence the different name.
func rectEncloses(outer, inner Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.W <= outer.X+outer.W && inner.Y+inner.H <= outer.Y+outer.H
}
