// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Button is a clickable rectangle with a centred label. Paints a
// 1-pixel border in Theme.Border on a Theme.Surface body; hovered /
// pressed states cycle through SurfaceAlt + Accent so the user sees
// click feedback before the callback fires.
//
// Wire a handler via OnClick; the button calls it from OnEvent when
// it receives an EventClick. Callers re-paint via Draw after any
// state mutation (the toolkit doesn't drive its own frame loop --
// the wasmbox compositor's tick is the redraw trigger).
type Button struct {
	Base
	Label   string
	OnClick func()

	hovered bool
	pressed bool
}

// NewButton constructs a Button with the given label + click handler.
// Handler may be nil (a no-op button is still rendered).
func NewButton(label string, onClick func()) *Button {
	return &Button{Label: label, OnClick: onClick}
}

// SetHovered/SetPressed are wired by the parent container's mouse
// dispatcher so the button can render its hover/press visual states.
// Direct setters (vs deducing from OnEvent kinds) keep the parent
// in control of state propagation -- enter/leave events would
// duplicate the same logic in every leaf widget.
func (b *Button) SetHovered(v bool) { b.hovered = v }
func (b *Button) SetPressed(v bool) { b.pressed = v }

// Draw paints the button into surface. Surface is the row-major RGBA
// framebuffer of width surfaceW; theme supplies the palette.
func (b *Button) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := b.Bounds()
	face := theme.Surface
	if b.pressed {
		face = theme.Accent
	} else if b.hovered {
		face = theme.SurfaceAlt
	}
	fillRect(surface, surfaceW, r.X, r.Y, r.W, r.H, face)
	strokeRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Border)
	// Label rendering is a TODO until the font package lands. For now
	// the button is a solid styled rectangle; downstream callers can
	// also draw their own label into b.Bounds() after Draw if they
	// have a font ready.
}

// OnEvent dispatches click events to the OnClick callback. Other
// event kinds are ignored at this level (the parent container may
// have already pre-filtered).
func (b *Button) OnEvent(ev Event) {
	if ev.Kind == EventClick && b.OnClick != nil {
		b.OnClick()
	}
}
