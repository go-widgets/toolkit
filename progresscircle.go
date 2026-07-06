// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strconv"

import "github.com/go-widgets/painter"

// ProgressCircleSize is the default side-length in pixels of a
// ProgressCircle rendered with a zero-sized Bounds. Roughly matches
// the "large" circular-progress indicator in Material / Adwaita
// dashboards; small enough to sit next to a status label yet big
// enough for the "XX%" caption to read on the toolkit's 5x7 font.
const ProgressCircleSize = 40

// ProgressCircleStroke is the ring thickness in pixels: the offset
// between the outer track square and the inner "hole" that carries
// the percentage caption. A thicker stroke leaves less room for the
// text; 4px keeps a two-digit percentage centred inside the ring
// with pixels to spare on either side.
const ProgressCircleStroke = 4

// ProgressCircle is a fake-circular progress indicator: a rounded
// square track with a "cup-filling" band that rises from the bottom
// as Fraction grows from 0 to 1. Not a true arc — the pixel-blitting
// toolkit does not carry a curve rasteriser — but it conveys the
// same "circular progress" intent at the same abstraction level as
// Spinner (a rotating radial line) and Avatar (a rounded square).
//
// Layout: an outer square filled in theme.SurfaceAlt (the "track"),
// an inner square inset by ProgressCircleStroke on all sides filled
// in theme.Surface (the "hole" that the caption sits in), and a
// horizontal Accent band inside the ring whose height is proportional
// to Fraction. The band grows from the bottom edge upward for the
// familiar "filling up" visual. The percentage caption ("XX%") is
// drawn in theme.OnSurface centred inside the inner square.
type ProgressCircle struct {
	Base
	Fraction float64 // 0..1; clamped by Draw
}

// NewProgressCircle constructs a ProgressCircle at Fraction=0.
func NewProgressCircle() *ProgressCircle { return &ProgressCircle{} }

// SetFraction clamps + assigns Fraction. 0 = empty, 1 = full. Kept
// as a symmetrical helper to ProgressBar.SetFraction so both widgets
// present the same knob to callers.
func (pc *ProgressCircle) SetFraction(f float64) {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	pc.Fraction = f
}

// Draw paints the track, the hole, the fill band, and the centred
// percentage caption. Draw clamps Fraction defensively so callers
// bypassing SetFraction still render a valid frame.
func (pc *ProgressCircle) Draw(p painter.Painter, theme *Theme) {
	r := pc.Bounds()
	// Fall back to a square of ProgressCircleSize when Bounds is
	// zero-sized. Callers that place the widget with SetBounds keep
	// full control; callers that just call Draw get a sensible
	// default without a helper.
	if r.W <= 0 || r.H <= 0 {
		r.W = ProgressCircleSize
		r.H = ProgressCircleSize
	}
	// Clamp to a square from the smaller of (W, H) so the widget
	// still reads as "roughly circular" when the caller passes a
	// non-square rect.
	side := r.W
	if r.H < side {
		side = r.H
	}
	// Outer track fill.
	fillRect(p, r.X, r.Y, side, side, theme.SurfaceAlt)
	// Inner hole (leaves a ring of ProgressCircleStroke pixels).
	inner := Rect{
		X: r.X + ProgressCircleStroke,
		Y: r.Y + ProgressCircleStroke,
		W: side - 2*ProgressCircleStroke,
		H: side - 2*ProgressCircleStroke,
	}
	if inner.W > 0 && inner.H > 0 {
		fillRect(p, inner.X, inner.Y, inner.W, inner.H, theme.Surface)
	}
	// Fill band: Accent band inside the ring, growing from the
	// bottom edge upward as Fraction grows from 0 to 1. Clamp
	// Fraction so callers that bypassed SetFraction still render.
	f := pc.Fraction
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	ringInnerY := r.Y + ProgressCircleStroke
	ringInnerH := side - 2*ProgressCircleStroke
	if ringInnerH > 0 {
		bandH := int(float64(ringInnerH) * f)
		if bandH > 0 {
			bandY := ringInnerY + ringInnerH - bandH
			// Left ring column.
			fillRect(p, r.X, bandY, ProgressCircleStroke, bandH, theme.Accent)
			// Right ring column.
			fillRect(p, r.X+side-ProgressCircleStroke, bandY,
				ProgressCircleStroke, bandH, theme.Accent)
		}
	}
	// Percentage caption centred in the inner square. f is already
	// clamped to [0, 1] above, so pct lands in [0, 100] without a
	// second guard.
	pct := int(f*100 + 0.5)
	caption := strconv.Itoa(pct) + "%"
	tw := TextWidth(caption)
	if inner.W > 0 && inner.H > 0 {
		tx := inner.X + (inner.W-tw)/2
		ty := inner.Y + (inner.H-GlyphHeight)/2
		DrawText(p, tx, ty, caption, theme.OnSurface)
	}
}
