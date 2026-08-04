// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// RangeSlider is a two-handle slider selecting a sub-interval [Low, High]
// within a continuous Min..Max range -- a price band, a date window, a volume
// gate. It is the two-thumb sibling of Scale: the same rounded track and
// circular white thumbs, but the Accent fill spans the selected band between
// the handles rather than from the left edge.
//
// A click grabs whichever handle is nearest the cursor and jumps it there; a
// subsequent drag moves that same handle, clamped so Low never crosses High.
type RangeSlider struct {
	Base
	focusState
	Min, Max    float64
	Low, High   float64
	Orientation Orientation
	OnChange    func(low, high float64)

	// active is the handle grabbed by the current click/drag: 0 = none,
	// 1 = Low, 2 = High. It is set on EventClick and cleared on EventMouseUp.
	active int
}

// NewRangeSlider builds a RangeSlider spanning [min, max] with the given
// initial band. The band is clamped and ordered so Low <= High.
func NewRangeSlider(min, max, low, high float64) *RangeSlider {
	s := &RangeSlider{Min: min, Max: max}
	s.SetRange(low, high)
	return s
}

// SetRange clamps both bounds to [Min, Max] and swaps them if low > high, so
// the invariant Low <= High always holds.
func (s *RangeSlider) SetRange(low, high float64) {
	if low > high {
		low, high = high, low
	}
	s.Low = s.clamp(low)
	s.High = s.clamp(high)
}

// clamp confines v to [Min, Max].
func (s *RangeSlider) clamp(v float64) float64 {
	if v < s.Min {
		return s.Min
	}
	if v > s.Max {
		return s.Max
	}
	return v
}

// axis returns the start coordinate and length of the slider along its active
// axis: (r.X, r.W) horizontal, (r.Y, r.H) vertical.
func (s *RangeSlider) axis() (start, length int) {
	r := s.Bounds()
	if s.Orientation == Vertical {
		return r.Y, r.H
	}
	return r.X, r.W
}

// valueAt maps a widget-local coordinate along the active axis into a value,
// clamped to range. Vertical is flipped so the top is Max.
func (s *RangeSlider) valueAt(coord int) float64 {
	_, length := s.axis()
	span := length - scaleThumbSize
	if span <= 0 {
		return s.Min
	}
	pos := float64(coord-scaleThumbSize/2) / float64(span)
	if s.Orientation == Vertical {
		pos = 1 - pos
	}
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	return s.Min + pos*(s.Max-s.Min)
}

// thumbPos returns the thumb's top-left along the active axis for value v (an X
// horizontal, a Y vertical, where the top = Max).
func (s *RangeSlider) thumbPos(v float64) int {
	start, length := s.axis()
	var pos float64
	if s.Max > s.Min {
		pos = (v - s.Min) / (s.Max - s.Min)
	}
	if s.Orientation == Vertical {
		pos = 1 - pos
	}
	return start + int(pos*float64(length-scaleThumbSize))
}

// Draw paints the rounded track, the Accent band between the two handles, and
// a circular white thumb at each handle -- matching Scale's macOS styling.
func (s *RangeSlider) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	const trackThick = 4
	trackR := trackThick / 2
	lowP := s.thumbPos(s.Low)
	highP := s.thumbPos(s.High)
	if s.Orientation == Vertical {
		trackX := r.X + (r.W-trackThick)/2
		fillRoundRect(p, trackX, r.Y, trackThick, r.H, trackR, theme.SurfaceAlt)
		// High sits at the top (smaller Y), Low at the bottom; the band runs
		// between their centres.
		bandY := highP + scaleThumbSize/2
		bandH := lowP - highP
		if bandH > 0 {
			fillRoundRect(p, trackX, bandY, trackThick, bandH, trackR, theme.Accent)
		}
		tx := r.X + (r.W-scaleThumbSize)/2
		for _, ty := range []int{lowP, highP} {
			fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Surface)
			strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Border)
		}
		s.drawFocusRing(p, theme, r)
		return
	}
	trackY := r.Y + (r.H-trackThick)/2
	fillRoundRect(p, r.X, trackY, r.W, trackThick, trackR, theme.SurfaceAlt)
	// Accent band spans from the low thumb centre to the high thumb centre.
	bandX := lowP + scaleThumbSize/2
	bandW := highP - lowP
	if bandW > 0 {
		fillRoundRect(p, bandX, trackY, bandW, trackThick, trackR, theme.Accent)
	}
	ty := r.Y + (r.H-scaleThumbSize)/2
	for _, tx := range []int{lowP, highP} {
		fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Surface)
		strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Border)
	}
	s.drawFocusRing(p, theme, r)
}

// OnEvent: a click grabs the nearer handle and jumps it to the cursor; a drag
// moves the grabbed handle; a mouse-up releases it. Each move re-clamps so the
// handles never cross, and fires OnChange.
func (s *RangeSlider) OnEvent(ev Event) {
	start, length := s.axis()
	if length <= 0 || s.Max <= s.Min {
		return
	}
	// coord is the click along the active axis, widget-local.
	coord := ev.X
	if s.Orientation == Vertical {
		coord = ev.Y
	}
	switch ev.Kind {
	case EventClick:
		// Grab whichever handle's thumb centre is nearer the cursor, comparing in
		// the same widget-local frame as coord (thumbPos returns an absolute
		// coordinate, so subtract the axis start). Without this, clicking one
		// thumb grabs the other when the slider is not at the origin.
		lowC := s.thumbPos(s.Low) - start + scaleThumbSize/2
		highC := s.thumbPos(s.High) - start + scaleThumbSize/2
		if abs(coord-lowC) <= abs(coord-highC) {
			s.active = 1
		} else {
			s.active = 2
		}
		s.moveActive(coord)
	case EventMouseDrag:
		if s.active != 0 {
			s.moveActive(coord)
		}
	case EventMouseUp:
		s.active = 0
	}
}

// moveActive sets the grabbed handle to the value under x, clamped so Low never
// crosses High, then fires OnChange.
func (s *RangeSlider) moveActive(x int) {
	v := s.valueAt(x)
	switch s.active {
	case 1:
		if v > s.High {
			v = s.High
		}
		s.Low = v
	case 2:
		if v < s.Low {
			v = s.Low
		}
		s.High = v
	}
	if s.OnChange != nil {
		s.OnChange(s.Low, s.High)
	}
}

// abs is the integer absolute value.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
