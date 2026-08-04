// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

import "strconv"

// SpinButton is an integer input with `+` and `−` buttons on the
// right. Click `+` adds Step, click `−` subtracts Step (clamped to
// [Min, Max]). The value is rendered as a decimal string in the left
// portion of the body.
type SpinButton struct {
	Base
	Min, Max int
	Value    int
	Step     int
	OnChange func(v int)
}

// spinButtonW is the pixel width of each up/down button on the right.
const spinButtonW = 16

// NewSpinButton builds a SpinButton spanning [min, max] with the
// given initial + step. Step <= 0 is clamped to 1 so clicks never
// no-op silently.
func NewSpinButton(min, max, initial, step int) *SpinButton {
	if step <= 0 {
		step = 1
	}
	s := &SpinButton{Min: min, Max: max, Step: step}
	s.SetValue(initial)
	return s
}

// SetValue clamps + assigns.
func (s *SpinButton) SetValue(v int) {
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.Value = v
}

// Draw paints the body (with the value text) + the two stacked
// buttons on the right.
func (s *SpinButton) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// Value text in the left portion.
	text := strconv.Itoa(s.Value)
	textY := r.Y + (r.H-s.glyphHeight())/2
	s.drawText(p, r.X+4, textY, text, theme.OnSurface)
	// Two buttons on the right, vertically stacked.
	btnX := r.X + r.W - spinButtonW
	half := r.H / 2
	fillRect(p, btnX, r.Y, spinButtonW, half, theme.SurfaceAlt)
	fillRect(p, btnX, r.Y+half, spinButtonW, r.H-half, theme.SurfaceAlt)
	strokeRect(p, btnX, r.Y, spinButtonW, half, theme.Border)
	strokeRect(p, btnX, r.Y+half, spinButtonW, r.H-half, theme.Border)
	// Uniform stepper glyphs drawn as vector bars, centred in each button, so
	// the "+" and "−" align exactly — unlike font glyphs, whose hyphen-minus
	// sits at x-height while the plus is centred, making the pair look ragged.
	cx := btnX + spinButtonW/2
	const bar = 7 // arm length of the +/− (odd, so it centres on cx)
	ink := theme.OnSurface
	cyUp := r.Y + half/2
	fillRect(p, cx-bar/2, cyUp, bar, 1, ink) // + horizontal
	fillRect(p, cx, cyUp-bar/2, 1, bar, ink) // + vertical
	cyDn := r.Y + half + (r.H-half)/2
	fillRect(p, cx-bar/2, cyDn, bar, 1, ink) // − horizontal
}

// OnEvent: click on the upper-right button increments; click on the
// lower-right button decrements.
func (s *SpinButton) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := s.Bounds()
	if ev.X < r.W-spinButtonW {
		return // body click: no action in v0.2 (would open keypad)
	}
	if ev.Y < r.H/2 {
		s.SetValue(s.Value + s.Step)
	} else {
		s.SetValue(s.Value - s.Step)
	}
	if s.OnChange != nil {
		s.OnChange(s.Value)
	}
}
