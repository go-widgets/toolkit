// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ToggleButton is a Button with a sticky on/off state. A click flips
// Pressed, notifying the Pressed Observable's subscribers. Pressed =
// Theme.Accent face, unpressed = Theme.Surface; the label is rendered
// centered in the button.
//
// The reactive pressed state is MVVM-only: it lives in an unexported
// Observable exposed via [ToggleButton.Pressed]. There is no settable
// Pressed field and no OnToggle callback — a host binds Pressed()
// (Set / Subscribe / two-way).
type ToggleButton struct {
	Base
	focusState
	// Label is the button caption (config).
	Label string

	pressed *mvvm.Observable[bool]

	hovered bool
}

// Pressed is the sticky on/off state as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Pressed field. A click
// or a Space/Enter key press flips it; subscribers are notified. A bare
// ToggleButton (no NewToggleButton) lazily initialises to false on first access.
func (t *ToggleButton) Pressed() *mvvm.Observable[bool] {
	if t.pressed == nil {
		t.pressed = mvvm.NewObservable(false)
	}
	return t.pressed
}

// NewToggleButton constructs a ToggleButton with the given label +
// initial state.
func NewToggleButton(label string, pressed bool) *ToggleButton {
	t := &ToggleButton{Label: label}
	t.pressed = mvvm.NewObservable(pressed)
	return t
}

// Draw paints the face + border + centred label.
func (t *ToggleButton) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	face := theme.Surface
	if t.Pressed().Get() {
		face = theme.Accent
	} else if t.hovered {
		// Hover raises the unpressed face to SurfaceAlt (matching Button); the
		// pressed Accent face wins so the sticky state stays legible.
		face = theme.SurfaceAlt
	}
	ink, border := theme.OnSurface, theme.Border
	if t.Disabled().Get() {
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

// OnEvent: a click flips Pressed; a move tracks the hover face. A Disabled
// toggle ignores every kind.
func (t *ToggleButton) OnEvent(ev Event) {
	if t.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		t.toggle()
	case EventKeyDown:
		// Space or Enter flips the sticky state while focused, reusing the click
		// path.
		switch ev.Code {
		case " ", "Space", "Enter":
			t.toggle()
		}
	case EventMouseMove:
		t.hovered = t.localInBounds(ev.X, ev.Y)
	}
}

// toggle flips the Pressed Observable -- the shared mutate path for a click and
// a Space/Enter key press. Subscribers are notified on change.
func (t *ToggleButton) toggle() {
	t.Pressed().Set(!t.Pressed().Get())
}

// HitRect is the toggle button's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]).
// Byte-identical to Bounds under DensityCompact; a finger-sized target under
// DensityTouch.
func (t *ToggleButton) HitRect() Rect { return touchHitRect(t.Bounds()) }

// HitTest reports whether a surface point falls on the toggle button's
// (touch-clamped) hit rect.
func (t *ToggleButton) HitTest(px, py int) bool { return t.HitRect().Contains(px, py) }
