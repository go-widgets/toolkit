// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Switch is a compact iOS-style toggle: a wide horizontal track with a
// small square knob that sits on the left when Off and on the right
// when On. Distinct from ToggleButton in shape + intent: ToggleButton
// is a full-face button whose entire body flips colour with state, so
// it reads as "an action that stays pressed"; Switch is decorative
// chrome — a settings-row indicator whose knob position is the entire
// affordance ("is this feature on?").
//
// Track fill flips between SurfaceAlt (Off) and Accent (On) so the
// on-state stands out at a glance; the knob is drawn in Surface with a
// Border stroke so it stays visible against either track colour.
//
// Click flips On + fires OnToggle. Non-click events are ignored.
type Switch struct {
	Base
	On       bool
	OnToggle func(on bool)
}

// switchPad is the inset from the track edge to the knob's edge, in
// pixels. Matches the visual gap most iOS-style switches use so the
// knob never touches the track border.
const switchPad = 2

// NewSwitch constructs a Switch with the given initial state. The
// OnToggle callback is nil by default; assign it after construction if
// the caller wants a click hook.
func NewSwitch(on bool) *Switch { return &Switch{On: on} }

// Draw paints the track + knob. Track colour is picked by On; the knob
// slides between left + right edges by rewriting knobX in the On
// branch. Zero-height or extremely narrow Bounds degrade to a no-op
// via fillRect's own dimension guard.
func (s *Switch) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	track := theme.SurfaceAlt
	if s.On {
		track = theme.Accent
	}
	// Fully-rounded pill track + circular knob -- the iOS/macOS switch shape.
	fillRoundRect(p, r.X, r.Y, r.W, r.H, r.H/2, track)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, r.H/2, theme.Border)
	knobH := r.H - 2*switchPad
	knobW := knobH
	knobX := r.X + switchPad
	if s.On {
		knobX = r.X + r.W - knobW - switchPad
	}
	fillRoundRect(p, knobX, r.Y+switchPad, knobW, knobH, knobH/2, theme.Surface)
	strokeRoundRect(p, knobX, r.Y+switchPad, knobW, knobH, knobH/2, theme.Border)
}

// OnEvent flips On + fires OnToggle on click. All other event kinds
// pass through without effect (matches ToggleButton / CheckButton).
func (s *Switch) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	s.On = !s.On
	if s.OnToggle != nil {
		s.OnToggle(s.On)
	}
}
