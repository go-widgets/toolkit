// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

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
	Style   ButtonStyle // resting appearance; default is ButtonDefault

	hovered bool
	pressed bool
}

// ButtonStyle selects a button's resting fill, giving a layout visual hierarchy
// (macOS "prominent"/default/secondary buttons). Hover + press still override
// the fill on top of the style.
type ButtonStyle int

const (
	// ButtonDefault is a Surface-faced button (the plain look).
	ButtonDefault ButtonStyle = iota
	// ButtonProminent is filled with Accent + accent-foreground text -- the
	// primary/default action (e.g. a calculator's operator keys, "OK").
	ButtonProminent
	// ButtonSecondary is filled with SurfaceAlt -- a muted grey key that sits
	// between Default and Prominent (e.g. a calculator's C / +/- / % keys).
	ButtonSecondary
)

// buttonRadius is the corner radius for the button + text-entry body, giving
// the WhiteSur / macOS soft-rectangle look.
const buttonRadius = 6

// accentFg returns the ink to draw on an Accent fill: the theme's
// accent_fg_color when present (GTK/WhiteSur themes set it to white), else a
// safe white fallback.
func accentFg(theme *Theme) RGBA {
	if c, ok := theme.Extra["accent_fg_color"]; ok {
		return c
	}
	return RGB(0xFF, 0xFF, 0xFF)
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

// Draw paints the button through p using theme's palette. Face
// cycles through Surface / SurfaceAlt (hovered) / Accent (pressed);
// the Label is centred in the body using the toolkit's 5x7 bitmap
// font. When the button is pressed the ink swaps to the theme's
// Background so the label stays legible against the Accent face.
func (b *Button) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	// Resting face/ink from the style.
	face := theme.Surface
	ink := theme.OnSurface
	switch b.Style {
	case ButtonProminent:
		face, ink = theme.Accent, accentFg(theme)
	case ButtonSecondary:
		face = theme.SurfaceAlt
	}
	// Interaction states override the resting fill.
	switch {
	case b.pressed:
		face = theme.Accent
		ink = theme.Background
	case b.hovered:
		face = theme.SurfaceAlt
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, face)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Border)
	if b.Label != "" {
		tw := TextWidth(b.Label)
		tx := r.X + (r.W-tw)/2
		ty := r.Y + (r.H-GlyphHeight())/2
		DrawText(p, tx, ty, b.Label, ink)
	}
}

// OnEvent dispatches click events to the OnClick callback. Other
// event kinds are ignored at this level (the parent container may
// have already pre-filtered).
func (b *Button) OnEvent(ev Event) {
	if ev.Kind == EventClick && b.OnClick != nil {
		b.OnClick()
	}
}
