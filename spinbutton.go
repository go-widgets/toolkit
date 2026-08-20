// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// SpinButton is an integer input with `+` and `−` buttons on the
// right. Click `+` adds Step, click `−` subtracts Step (clamped to
// [Min, Max]). The value is rendered as a decimal string in the left
// portion of the body.
//
// Min, Max and Step are config; the reactive value is MVVM-only: it lives in an
// unexported Observable exposed via [SpinButton.Value]. A host binds it (Set /
// Subscribe / two-way) — there is no settable Value field. A +/− click or a
// stepper key Sets it (clamped to [Min, Max]); subscribers are notified.
type SpinButton struct {
	Base
	focusState
	Min, Max int
	Step     int

	value *mvvm.Observable[int]
}

// Value is the current value as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Value field. A +/− click
// or a stepper key Sets it (clamped to [Min, Max]); subscribers are notified.
func (s *SpinButton) Value() *mvvm.Observable[int] {
	if s.value == nil {
		s.value = mvvm.NewObservable(0)
	}
	return s.value
}

// spinButtonW is the pixel width of each up/down button on the right.
const spinButtonW = 16

// spinTextPad is the logical left inset of the value text inside the body. It
// routes through scaled so the text keeps its inset proportional under HiDPI and
// touch density; scaled(spinTextPad) == spinTextPad at compact/1x (byte-identical).
const spinTextPad = 4

// NewSpinButton builds a SpinButton spanning [min, max] with the
// given initial + step. Step <= 0 is clamped to 1 so clicks never
// no-op silently.
func NewSpinButton(min, max, initial, step int) *SpinButton {
	if step <= 0 {
		step = 1
	}
	s := &SpinButton{Min: min, Max: max, Step: step}
	s.value = mvvm.NewObservable(0)
	s.SetValue(initial)
	return s
}

// SetValue clamps v to [Min, Max] and Sets the Value Observable — the shared
// mutate path for a +/− button click and every stepper key. Subscribers are
// notified on change (an unchanged value is a no-op, per mvvm.Observable).
func (s *SpinButton) SetValue(v int) {
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.Value().Set(v)
}

// Draw paints the body (with the value text) + the two stacked
// buttons on the right.
func (s *SpinButton) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	// Body / border / text / button-face / glyph colours. A disabled spinbutton
	// mutes them all; only taken when Disabled so the enabled draw is unchanged.
	bodyC, borderC, textC, btnC := theme.Surface, theme.Border, theme.OnSurface, theme.SurfaceAlt
	glyphC := theme.OnSurface
	if s.Disabled().Get() {
		bodyC, borderC, textC, btnC, glyphC = mutedFace(theme), mutedInk(theme), mutedInk(theme), mutedFace(theme), mutedInk(theme)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, bodyC)
	strokeRect(p, r.X, r.Y, r.W, r.H, borderC)
	// Value text in the left portion.
	text := strconv.Itoa(s.Value().Get())
	textY := r.Y + (r.H-s.glyphHeight())/2
	s.drawText(p, r.X+scaled(spinTextPad), textY, text, textC)
	// Two buttons on the right, vertically stacked.
	btnX := r.X + r.W - scaled(spinButtonW)
	half := r.H / 2
	fillRect(p, btnX, r.Y, scaled(spinButtonW), half, btnC)
	fillRect(p, btnX, r.Y+half, scaled(spinButtonW), r.H-half, btnC)
	strokeRect(p, btnX, r.Y, scaled(spinButtonW), half, borderC)
	strokeRect(p, btnX, r.Y+half, scaled(spinButtonW), r.H-half, borderC)
	// Uniform stepper glyphs drawn as vector bars, centred in each button, so
	// the "+" and "−" align exactly — unlike font glyphs, whose hyphen-minus
	// sits at x-height while the plus is centred, making the pair look ragged.
	cx := btnX + scaled(spinButtonW)/2
	const bar = 7 // arm length of the +/− (odd, so it centres on cx)
	ink := glyphC
	cyUp := r.Y + half/2
	fillRect(p, cx-bar/2, cyUp, bar, 1, ink) // + horizontal
	fillRect(p, cx, cyUp-bar/2, 1, bar, ink) // + vertical
	cyDn := r.Y + half + (r.H-half)/2
	fillRect(p, cx-bar/2, cyDn, bar, 1, ink) // − horizontal
	s.drawFocusRing(p, theme, r)
}

// OnEvent: click on the upper-right button increments; click on the
// lower-right button decrements.
func (s *SpinButton) OnEvent(ev Event) {
	if s.Disabled().Get() {
		return
	}
	if ev.Kind == EventKeyDown {
		// Up/Down step by Step (like the +/- buttons); PageUp/PageDown by a large
		// step (10x); Home/End jump to Min/Max. Each reuses SetValue.
		switch ev.Code {
		case "ArrowUp":
			s.SetValue(s.Value().Get() + s.Step)
		case "ArrowDown":
			s.SetValue(s.Value().Get() - s.Step)
		case "PageUp":
			s.SetValue(s.Value().Get() + 10*s.Step)
		case "PageDown":
			s.SetValue(s.Value().Get() - 10*s.Step)
		case "Home":
			s.SetValue(s.Min)
		case "End":
			s.SetValue(s.Max)
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	r := s.Bounds()
	// The stepper column is drawn scaled(spinButtonW) wide, but its click zone
	// clamps UP to the density finger floor via TouchTarget, so under DensityTouch
	// a press lands the +/- buttons from further left (the inert body yields the
	// space) without the drawn buttons moving. At compact TouchTarget is a
	// pass-through so the boundary is byte-identical to the drawn column edge.
	if ev.X < r.W-TouchTarget(scaled(spinButtonW)) {
		return // body click: no action in v0.2 (would open keypad)
	}
	if ev.Y < r.H/2 {
		s.SetValue(s.Value().Get() + s.Step)
	} else {
		s.SetValue(s.Value().Get() - s.Step)
	}
}
