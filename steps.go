// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Steps is a horizontal step indicator — [1]—[2]—[3]—[4] — for
// multi-step flows (a wizard, an on-boarding tour, a checkout page).
// Each entry is drawn as a small square badge carrying its 1-based
// index number, with a 1-px connector line between successive badges.
// A Labels entry that is not "" renders below its badge as caption
// text in Theme.OnBackground.
//
// Current is the 0-indexed cursor into Labels; badges up to AND
// including Current fill with Theme.Accent (the "done / active"
// colour), later badges fill with Theme.SurfaceAlt (the "pending"
// colour). A Current outside [0, len(Labels)) means either "no step
// active yet" (Current < 0 -> every badge is pending) or "all done"
// (Current >= len -> every badge is filled).
//
// A click on a badge jumps to that step: OnEvent hit-tests the same
// badge layout Draw paints and Sets the Current Observable to the clicked
// index, notifying its subscribers. A host binds Current to react to the
// jump; there is no click callback.
//
// The reactive cursor is MVVM-only: it lives in an unexported Observable
// exposed via [Steps.Current]. Labels and Orientation are set-once layout
// config and stay plain fields.
type Steps struct {
	Base
	Labels []string
	// Orientation lays the badges out left-to-right (Horizontal, the zero
	// value — a wizard strip) or top-to-bottom (Vertical — a side
	// checklist). Vertical draws its connectors as vertical lines and
	// renders each caption to the right of its badge instead of below it.
	Orientation Orientation

	current *mvvm.Observable[int]
}

// Current is the 0-indexed cursor into Labels as a shared [mvvm.Observable]:
// a host binds it (Set / Subscribe / two-way) — there is no settable Current
// field. A click on a badge Sets it to that index; subscribers are notified.
// A value outside [0, len(Labels)) means either "no step active yet"
// (Current < 0 -> every badge is pending) or "all done" (Current >= len ->
// every badge is filled). The Observable lazy-inits to 0 on first access so a
// zero-value &Steps{} is usable.
func (s *Steps) Current() *mvvm.Observable[int] {
	if s.current == nil {
		s.current = mvvm.NewObservable(0)
	}
	return s.current
}

// Steps sizing constants. Chosen so the badges + connectors fit inside
// a 40-px-tall bar (a common toolbar strip height).
const (
	// StepBoxW is the pixel width of each badge.
	StepBoxW = 16
	// StepBoxH is the pixel height of each badge.
	StepBoxH = 16
	// StepConnectorW is the horizontal length of the connector line
	// between two badges.
	StepConnectorW = 20
	// StepLabelGap is the vertical gap between a badge's bottom edge
	// and the caption text below it.
	StepLabelGap = 3
)

// NewSteps constructs a Steps indicator with the given labels + the
// initial current-step cursor.
func NewSteps(labels []string, current int) *Steps {
	s := &Steps{Labels: labels}
	s.current = mvvm.NewObservable(current)
	return s
}

// Draw paints each badge, its connector to the previous badge (if any)
// and the optional caption below it. The badge fill switches from
// Accent (index <= Current) to SurfaceAlt (index > Current); the
// number ink inverts accordingly so it stays legible.
func (s *Steps) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	n := len(s.Labels)
	if n == 0 {
		return
	}
	vertical := s.Orientation == Vertical
	current := s.Current().Get()
	boxW, boxH := scaled(StepBoxW), scaled(StepBoxH)
	conn, gap := scaled(StepConnectorW), scaled(StepLabelGap)
	line := max(1, scaled(1))
	// The badge column is pinned to one edge; the layout axis advances the
	// other coordinate. Horizontal centres the badge row vertically inside a
	// tall bar (unchanged); vertical leaves the badge at the left edge so the
	// caption has room to its right.
	x, y := r.X, r.Y
	if !vertical && r.H > boxH {
		y = r.Y + (r.H-boxH)/2
	}
	for i, lab := range s.Labels {
		if i > 0 {
			if vertical {
				// Connector: 1-px vertical line at the badge horizontal centre.
				connX := x + boxW/2
				fillRect(p, connX, y, line, conn, theme.Border)
				y += conn
			} else {
				// Connector: 1-px horizontal line at the badge vertical centre.
				connY := y + boxH/2
				fillRect(p, x, connY, conn, line, theme.Border)
				x += conn
			}
		}
		fill := theme.SurfaceAlt
		ink := theme.OnSurface
		if i <= current {
			fill = theme.Accent
			ink = theme.Background
		}
		fillRect(p, x, y, boxW, boxH, fill)
		strokeRect(p, x, y, boxW, boxH, theme.Border)
		num := strconv.Itoa(i + 1)
		tw := s.textWidth(num)
		tx := x + (boxW-tw)/2
		ty := y + (boxH-s.glyphHeight())/2
		s.drawText(p, tx, ty, num, ink)
		if lab != "" {
			if vertical {
				// Caption to the right of the badge, vertically centred on it.
				lx := x + boxW + gap
				ly := y + (boxH-s.glyphHeight())/2
				s.drawText(p, lx, ly, lab, theme.OnBackground)
			} else {
				lw := s.textWidth(lab)
				lx := x + (boxW-lw)/2
				// A caption wider than its badge is centred under it and would
				// poke past the left edge on the first step (or the right edge
				// on the last); keep it within Bounds().
				lx = clampInt(lx, r.X, r.X+r.W-lw)
				ly := y + boxH + gap
				s.drawText(p, lx, ly, lab, theme.OnBackground)
			}
		}
		if vertical {
			y += boxH
		} else {
			x += boxW
		}
	}
}

// OnEvent jumps to a clicked step: it hit-tests each badge against the same
// layout Draw paints (badge i advances by StepBoxW/StepBoxH plus one
// StepConnectorW per gap along the layout axis; the cross axis is the pinned
// badge column, vertically centred in a tall bar for the horizontal case), and
// on a hit Sets the Current Observable to that index (subscribers are notified;
// an unchanged index is a no-op per mvvm.Observable). Only the badge box is
// sensitive -- a click on a caption or a connector is ignored. Coordinates are
// widget-local, so the first badge's top-left is (0, cross-offset).
func (s *Steps) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	vertical := s.Orientation == Vertical
	r := s.Bounds()
	boxW, boxH := scaled(StepBoxW), scaled(StepBoxH)
	conn := scaled(StepConnectorW)
	// A badge is a small affordance drawn boxW x boxH; its TAP target clamps UP
	// to the density minimum on each axis and centres over the drawn badge (the
	// Switch.HitRect pattern), WITHOUT changing what's painted. At compact the
	// clamp is a pass-through, so the hit box is exactly the drawn badge and
	// byte-identical to before; at touch it reaches the >=44px finger floor.
	hitW, hitH := TouchTarget(boxW), TouchTarget(boxH)
	yOff := 0
	if !vertical && r.H > boxH {
		yOff = (r.H - boxH) / 2
	}
	for i := range s.Labels {
		bx, by := 0, yOff
		if vertical {
			by = i * (boxH + conn)
		} else {
			bx = i * (boxW + conn)
		}
		hx := bx - (hitW-boxW)/2
		hy := by - (hitH-boxH)/2
		if ev.X >= hx && ev.X < hx+hitW && ev.Y >= hy && ev.Y < hy+hitH {
			s.Current().Set(i)
			return
		}
	}
}
