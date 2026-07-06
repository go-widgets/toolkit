// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

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
// Steps is a passive display container: hit-testing / event routing
// are not implemented — a caller who needs a click-to-jump interaction
// walks the same layout math externally.
type Steps struct {
	Base
	Labels  []string
	Current int
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
	return &Steps{Labels: labels, Current: current}
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
	y := r.Y
	if r.H > StepBoxH {
		y = r.Y + (r.H-StepBoxH)/2
	}
	x := r.X
	for i, lab := range s.Labels {
		if i > 0 {
			// Connector: 1-px horizontal line at the badge vertical centre.
			connY := y + StepBoxH/2
			fillRect(p, x, connY, StepConnectorW, 1, theme.Border)
			x += StepConnectorW
		}
		fill := theme.SurfaceAlt
		ink := theme.OnSurface
		if i <= s.Current {
			fill = theme.Accent
			ink = theme.Background
		}
		fillRect(p, x, y, StepBoxW, StepBoxH, fill)
		strokeRect(p, x, y, StepBoxW, StepBoxH, theme.Border)
		num := strconv.Itoa(i + 1)
		tw := TextWidth(num)
		tx := x + (StepBoxW-tw)/2
		ty := y + (StepBoxH-GlyphHeight)/2
		DrawText(p, tx, ty, num, ink)
		if lab != "" {
			lw := TextWidth(lab)
			lx := x + (StepBoxW-lw)/2
			ly := y + StepBoxH + StepLabelGap
			DrawText(p, lx, ly, lab, theme.OnBackground)
		}
		x += StepBoxW
	}
}
