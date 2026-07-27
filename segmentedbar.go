// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// BarSegment is one band of a SegmentedBar: a non-negative Value (its share
// of the whole), the RGBA it paints with, and an optional Label (reserved
// for a future legend / tooltip — Draw does not render it today).
type BarSegment struct {
	Value float64
	Fill  RGBA
	Label string
}

// SegmentedBar is a single bar split into proportional colored bands laid
// end to end -- e.g. a disk-usage meter showing used/free/reserved as one
// stacked strip. Orientation picks the layout axis: Horizontal (default)
// lays segments left→right, Vertical lays them bottom→top (the first
// segment sits at the bottom), matching ProgressBar/LevelBar's vertical
// convention.
type SegmentedBar struct {
	Base
	Segments    []BarSegment
	Orientation Orientation
}

// NewSegmentedBar builds a SegmentedBar with the given segments.
func NewSegmentedBar(segs []BarSegment) *SegmentedBar {
	return &SegmentedBar{Segments: segs}
}

// Total sums every segment's Value.
func (s *SegmentedBar) Total() float64 {
	total := 0.0
	for _, seg := range s.Segments {
		total += seg.Value
	}
	return total
}

// Draw paints a 1-px border around the whole bar + each segment's
// proportional share filled with its own color, separated by a 1-px
// Theme.Border line. A zero (or empty) total draws a bare
// Theme.SurfaceAlt track. Integer-rounding leftover is pushed onto the
// last segment so the bands always sum to exactly the bar's length,
// mirroring Table.columnWidths's remainder handling.
func (s *SegmentedBar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)

	n := len(s.Segments)
	if n == 0 {
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return
	}

	// Clamp negative Values to 0 before summing so one defensively bad
	// segment can't drag the total negative and invert every fraction.
	values := make([]float64, n)
	total := 0.0
	for i, seg := range s.Segments {
		v := seg.Value
		if v < 0 {
			v = 0
		}
		values[i] = v
		total += v
	}
	if total <= 0 {
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return
	}

	length := r.W
	if s.Orientation == Vertical {
		length = r.H
	}

	// Distribute length proportionally, pushing the integer-rounding
	// leftover onto the last segment so the bands sum exactly to length.
	extents := make([]int, n)
	sum := 0
	for i, v := range values {
		e := int(float64(length) * v / total)
		extents[i] = e
		sum += e
	}
	extents[n-1] += length - sum

	if s.Orientation == Vertical {
		// Bottom-up: the first segment sits at the bottom edge.
		y := r.Y + r.H
		for i, seg := range s.Segments {
			e := extents[i]
			if e <= 0 {
				continue
			}
			y -= e
			fillRect(p, r.X, y, r.W, e, seg.Fill)
			if i > 0 {
				fillRect(p, r.X, y+e-1, r.W, 1, theme.Border)
			}
		}
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return
	}

	x := r.X
	for i, seg := range s.Segments {
		e := extents[i]
		if e <= 0 {
			continue
		}
		fillRect(p, x, r.Y, e, r.H, seg.Fill)
		if i > 0 {
			fillRect(p, x, r.Y, 1, r.H, theme.Border)
		}
		x += e
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}
