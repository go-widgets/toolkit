// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// ToggleButton is a Button with a sticky on/off state. Click flips
// Pressed + fires OnToggle. Pressed = Theme.Accent face, unpressed =
// Theme.Surface; the label is rendered centered in the button.
type ToggleButton struct {
	Base
	Label    string
	Pressed  bool
	OnToggle func(pressed bool)
}

// NewToggleButton constructs a ToggleButton with the given label +
// initial state.
func NewToggleButton(label string, pressed bool) *ToggleButton {
	return &ToggleButton{Label: label, Pressed: pressed}
}

// Draw paints the face + border + centred label.
func (t *ToggleButton) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := t.Bounds()
	face := theme.Surface
	if t.Pressed {
		face = theme.Accent
	}
	fillRect(surface, surfaceW, r.X, r.Y, r.W, r.H, face)
	strokeRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Border)
	tw := TextWidth(t.Label)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-GlyphHeight)/2
	DrawText(surface, surfaceW, tx, ty, t.Label, theme.OnSurface)
}

// OnEvent: click flips Pressed + fires OnToggle.
func (t *ToggleButton) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	t.Pressed = !t.Pressed
	if t.OnToggle != nil {
		t.OnToggle(t.Pressed)
	}
}
