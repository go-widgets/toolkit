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
	Min, Max  float64
	Low, High float64
	OnChange  func(low, high float64)

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

// valueAt maps a widget-local x into a value on the track, clamped to range.
func (s *RangeSlider) valueAt(x int) float64 {
	r := s.Bounds()
	span := r.W - scaleThumbSize
	if span <= 0 {
		return s.Min
	}
	pos := float64(x-scaleThumbSize/2) / float64(span)
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	return s.Min + pos*(s.Max-s.Min)
}

// thumbX returns the left pixel of the thumb for value v.
func (s *RangeSlider) thumbX(v float64) int {
	r := s.Bounds()
	var pos float64
	if s.Max > s.Min {
		pos = (v - s.Min) / (s.Max - s.Min)
	}
	return r.X + int(pos*float64(r.W-scaleThumbSize))
}

// Draw paints the rounded track, the Accent band between the two handles, and
// a circular white thumb at each handle -- matching Scale's macOS styling.
func (s *RangeSlider) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	const trackH = 4
	trackY := r.Y + (r.H-trackH)/2
	trackR := trackH / 2
	fillRoundRect(p, r.X, trackY, r.W, trackH, trackR, theme.SurfaceAlt)

	lowX := s.thumbX(s.Low)
	highX := s.thumbX(s.High)
	// Accent band spans from the low thumb centre to the high thumb centre.
	bandX := lowX + scaleThumbSize/2
	bandW := highX - lowX
	if bandW > 0 {
		fillRoundRect(p, bandX, trackY, bandW, trackH, trackR, theme.Accent)
	}
	// Two circular white thumbs with a border (same shape as Scale's knob).
	ty := r.Y + (r.H-scaleThumbSize)/2
	for _, tx := range []int{lowX, highX} {
		fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Surface)
		strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Border)
	}
}

// OnEvent: a click grabs the nearer handle and jumps it to the cursor; a drag
// moves the grabbed handle; a mouse-up releases it. Each move re-clamps so the
// handles never cross, and fires OnChange.
func (s *RangeSlider) OnEvent(ev Event) {
	r := s.Bounds()
	if r.W <= 0 || s.Max <= s.Min {
		return
	}
	switch ev.Kind {
	case EventClick:
		// Grab whichever handle's thumb centre is nearer the cursor.
		lowC := s.thumbX(s.Low) + scaleThumbSize/2
		highC := s.thumbX(s.High) + scaleThumbSize/2
		if abs(ev.X-lowC) <= abs(ev.X-highC) {
			s.active = 1
		} else {
			s.active = 2
		}
		s.moveActive(ev.X)
	case EventMouseDrag:
		if s.active != 0 {
			s.moveActive(ev.X)
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
