// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ToggleButton is a Button with a sticky on/off state. Click flips
// Pressed + fires OnToggle. Pressed = Theme.Accent face, unpressed =
// Theme.Surface; the label is rendered centered in the button.
type ToggleButton struct {
	Base
	focusState
	Label    string
	Pressed  bool
	OnToggle func(pressed bool)

	hovered bool
}

// NewToggleButton constructs a ToggleButton with the given label +
// initial state.
func NewToggleButton(label string, pressed bool) *ToggleButton {
	return &ToggleButton{Label: label, Pressed: pressed}
}

// Draw paints the face + border + centred label.
func (t *ToggleButton) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	face := theme.Surface
	if t.Pressed {
		face = theme.Accent
	} else if t.hovered {
		// Hover raises the unpressed face to SurfaceAlt (matching Button); the
		// pressed Accent face wins so the sticky state stays legible.
		face = theme.SurfaceAlt
	}
	ink, border := theme.OnSurface, theme.Border
	if t.Disabled {
		face, ink, border = mutedFace(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)
	tw := t.textWidth(t.Label)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-t.glyphHeight())/2
	t.drawText(p, tx, ty, t.Label, ink)
	t.drawFocusRing(p, theme, r)
}

// OnEvent: click flips Pressed + fires OnToggle; a move tracks the hover face.
// A Disabled toggle ignores every kind.
func (t *ToggleButton) OnEvent(ev Event) {
	if t.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick:
		t.toggle()
	case EventKeyDown:
		// Space or Enter flips the sticky state while focused, reusing the click
		// path (OnToggle).
		switch ev.Code {
		case " ", "Space", "Enter":
			t.toggle()
		}
	case EventMouseMove:
		t.hovered = t.localInBounds(ev.X, ev.Y)
	}
}

// toggle flips Pressed and fires OnToggle (nil-safe) -- the shared mutate path
// for a click and a Space/Enter key press.
func (t *ToggleButton) toggle() {
	t.Pressed = !t.Pressed
	if t.OnToggle != nil {
		t.OnToggle(t.Pressed)
	}
}

// HitRect is the toggle button's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]).
// Byte-identical to Bounds under DensityCompact; a finger-sized target under
// DensityTouch.
func (t *ToggleButton) HitRect() Rect { return touchHitRect(t.Bounds()) }

// HitTest reports whether a surface point falls on the toggle button's
// (touch-clamped) hit rect.
func (t *ToggleButton) HitTest(px, py int) bool { return t.HitRect().Contains(px, py) }
