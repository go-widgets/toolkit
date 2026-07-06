// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Scale is a horizontal slider over a continuous Min..Max range.
// Click on the track jumps the thumb to that x-position + fires
// OnChange. The 4-px track sits across the vertical midpoint in
// Theme.SurfaceAlt; the 10-px square thumb sits at the value's
// position in Theme.Accent.
type Scale struct {
	Base
	Min, Max float64
	Value    float64
	OnChange func(v float64)
}

// scaleThumbSize is the pixel side length of the thumb.
const scaleThumbSize = 16

// NewScale builds a Scale spanning [min, max] with the given initial
// value. Min == Max is allowed but renders a non-interactive track.
func NewScale(min, max, initial float64) *Scale {
	s := &Scale{Min: min, Max: max}
	s.SetValue(initial)
	return s
}

// SetValue clamps to [Min, Max] before assigning.
func (s *Scale) SetValue(v float64) {
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.Value = v
}

// Draw paints a macOS-style slider: a rounded track whose filled portion (up
// to the thumb) is Accent and whose remainder is SurfaceAlt, with a circular
// white thumb -- matching the Switch's pill track + circular knob.
func (s *Scale) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	const trackH = 4
	trackY := r.Y + (r.H-trackH)/2
	trackR := trackH / 2
	// Full (unfilled) track first, then the Accent fill up to the thumb centre.
	fillRoundRect(p, r.X, trackY, r.W, trackH, trackR, theme.SurfaceAlt)
	// Position the thumb. When Max == Min, sit at the left.
	var pos float64
	if s.Max > s.Min {
		pos = (s.Value - s.Min) / (s.Max - s.Min)
	}
	tx := r.X + int(pos*float64(r.W-scaleThumbSize))
	fillRoundRect(p, r.X, trackY, tx+scaleThumbSize/2-r.X, trackH, trackR, theme.Accent)
	// Circular white thumb + border (same shape as the Switch knob).
	ty := r.Y + (r.H-scaleThumbSize)/2
	fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Surface)
	strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, theme.Border)
}

// OnEvent: click jumps the thumb to the clicked x-position +
// fires OnChange.
func (s *Scale) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := s.Bounds()
	if r.W <= 0 || s.Max <= s.Min {
		return
	}
	pos := float64(ev.X) / float64(r.W)
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	s.SetValue(s.Min + pos*(s.Max-s.Min))
	if s.OnChange != nil {
		s.OnChange(s.Value)
	}
}
