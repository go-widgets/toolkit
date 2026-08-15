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
	focusState
	On       bool
	OnToggle func(on bool)
}

// switchPad is the inset from the track edge to the knob's edge, in LOGICAL
// pixels. Matches the visual gap most iOS-style switches use so the knob never
// touches the track border.
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
	// A disabled switch mutes its track, knob and borders so it reads as inert.
	// Only taken when Disabled — the enabled draw is byte-identical.
	border, knob := theme.Border, theme.Surface
	if s.Disabled {
		track, border, knob = mutedFace(theme), mutedInk(theme), mutedFace(theme)
	}
	// Fully-rounded pill track + circular knob -- the iOS/macOS switch shape.
	fillRoundRect(p, r.X, r.Y, r.W, r.H, r.H/2, track)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, r.H/2, border)
	pad := scaled(switchPad)
	knobH := r.H - 2*pad
	knobW := knobH
	knobX := r.X + pad
	if s.On {
		knobX = r.X + r.W - knobW - pad
	}
	fillRoundRect(p, knobX, r.Y+pad, knobW, knobH, knobH/2, knob)
	strokeRoundRect(p, knobX, r.Y+pad, knobW, knobH, knobH/2, border)
	s.drawFocusRing(p, theme, r)
}

// OnEvent flips On + fires OnToggle on click. All other event kinds
// pass through without effect (matches ToggleButton / CheckButton).
func (s *Switch) OnEvent(ev Event) {
	if s.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick:
		s.toggle()
	case EventKeyDown:
		// Space or Enter flips the switch while focused, reusing the click path.
		switch ev.Code {
		case " ", "Space", "Enter":
			s.toggle()
		}
	}
}

// toggle flips On and fires OnToggle (nil-safe) -- the shared mutate path for a
// click and a Space/Enter key press.
func (s *Switch) toggle() {
	s.On = !s.On
	if s.OnToggle != nil {
		s.OnToggle(s.On)
	}
}
