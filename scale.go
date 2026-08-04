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
	focusState
	Min, Max    float64
	Value       float64
	Orientation Orientation
	OnChange    func(v float64)
	// Step is the increment an arrow key applies to Value. When it is <= 0 the
	// scale falls back to keyStep (1% of the range), so a caller that never sets
	// Step still gets sensible keyboard nudges. PageUp/PageDown always move a
	// whole page (keyPage, 10% of the range) regardless of Step.
	Step float64
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
	var pos float64
	if s.Max > s.Min {
		pos = (s.Value - s.Min) / (s.Max - s.Min)
	}
	// Track / fill / thumb / border colours. A disabled scale mutes them all so
	// it reads as inert; the enabled draw is byte-identical (branch only taken
	// when Disabled).
	trackC, fillC, thumbC, borderC := theme.SurfaceAlt, theme.Accent, theme.Surface, theme.Border
	if s.Disabled {
		trackC, fillC, thumbC, borderC = mutedFace(theme), mutedInk(theme), mutedFace(theme), mutedInk(theme)
	}
	if s.Orientation == Vertical {
		const trackW = 4
		trackX := r.X + (r.W-trackW)/2
		trackR := trackW / 2
		fillRoundRect(p, trackX, r.Y, trackW, r.H, trackR, trackC)
		// pos=1 (Max) sits at the top; the fill runs from the thumb centre
		// down to the bottom (a fader reads "filled" below the knob).
		ty := r.Y + int((1-pos)*float64(r.H-scaleThumbSize))
		centreY := ty + scaleThumbSize/2
		fillRoundRect(p, trackX, centreY, trackW, r.Y+r.H-centreY, trackR, fillC)
		tx := r.X + (r.W-scaleThumbSize)/2
		fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, thumbC)
		strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, borderC)
		s.drawFocusRing(p, theme, r)
		return
	}
	const trackH = 4
	trackY := r.Y + (r.H-trackH)/2
	trackR := trackH / 2
	// Full (unfilled) track first, then the fill up to the thumb centre.
	fillRoundRect(p, r.X, trackY, r.W, trackH, trackR, trackC)
	// Position the thumb (pos computed above). When Max == Min, sit at the left.
	tx := r.X + int(pos*float64(r.W-scaleThumbSize))
	fillRoundRect(p, r.X, trackY, tx+scaleThumbSize/2-r.X, trackH, trackR, fillC)
	// Circular white thumb + border (same shape as the Switch knob).
	ty := r.Y + (r.H-scaleThumbSize)/2
	fillRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, thumbC)
	strokeRoundRect(p, tx, ty, scaleThumbSize, scaleThumbSize, scaleThumbSize/2, borderC)
	s.drawFocusRing(p, theme, r)
}

// keyStep is the arrow-key increment: Step when the caller set a positive one,
// else 1% of the range (so an unconfigured scale still nudges sensibly). Zero
// when the range is empty.
func (s *Scale) keyStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	return (s.Max - s.Min) / 100
}

// keyPage is the PageUp/PageDown increment: 10% of the range.
func (s *Scale) keyPage() float64 { return (s.Max - s.Min) / 10 }

// nudge adds delta to Value (clamped by SetValue) and fires OnChange -- the
// shared mutate+callback path every arrow / Home / End / Page key reuses, so a
// key move behaves exactly like a click that lands on the same value.
func (s *Scale) nudge(delta float64) {
	s.SetValue(s.Value + delta)
	if s.OnChange != nil {
		s.OnChange(s.Value)
	}
}

// setTo assigns v (clamped) and fires OnChange -- used by Home/End.
func (s *Scale) setTo(v float64) {
	s.SetValue(v)
	if s.OnChange != nil {
		s.OnChange(s.Value)
	}
}

// OnEvent: click jumps the thumb to the clicked x-position +
// fires OnChange; arrow / Home / End / Page keys move Value while focused.
func (s *Scale) OnEvent(ev Event) {
	if s.Disabled {
		return
	}
	if ev.Kind == EventKeyDown {
		if s.Max <= s.Min {
			return
		}
		switch ev.Code {
		case "ArrowRight", "ArrowUp":
			s.nudge(s.keyStep())
		case "ArrowLeft", "ArrowDown":
			s.nudge(-s.keyStep())
		case "PageUp":
			s.nudge(s.keyPage())
		case "PageDown":
			s.nudge(-s.keyPage())
		case "Home":
			s.setTo(s.Min)
		case "End":
			s.setTo(s.Max)
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	r := s.Bounds()
	if s.Max <= s.Min {
		return
	}
	// Map the click across the track the THUMB centre actually travels (extent −
	// scaleThumbSize), offset by half a thumb — the inverse of Draw's placement.
	// Vertical is flipped so the top is Max (a fader reads up = more).
	var pos float64
	if s.Orientation == Vertical {
		span := r.H - scaleThumbSize
		if span <= 0 {
			return
		}
		pos = 1 - float64(ev.Y-scaleThumbSize/2)/float64(span)
	} else {
		span := r.W - scaleThumbSize
		if span <= 0 {
			return
		}
		pos = float64(ev.X-scaleThumbSize/2) / float64(span)
	}
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
