// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// IconButton is a compact toolbar button whose entire face is one
// short glyph string ("+", "OK", "v", ...). Distinct from Button
// (which carries a text label with hover/press states) and
// ToggleButton (which carries toggle state) — IconButton is a passive
// Surface-faced tile meant for dense toolbars where the glyph itself
// is the semantic content.
//
// The face is theme.Surface with a 1-px theme.Border stroke; the
// glyph renders in theme.OnSurface. No accent fill by default — this
// keeps the button reading as a subtle toolbar affordance rather than
// a primary action.
//
// Auto-sizing: if Bounds().W is zero the first Draw() resizes the
// button to IconButtonSize x IconButtonSize (H preserved when
// non-zero). A pre-sized Bounds is honoured verbatim so a fixed
// toolbar column doesn't shift when the widget is dropped in.
type IconButton struct {
	Base
	focusState
	Icon    string
	OnClick func()

	hovered bool
	pressed bool
}

// IconButtonSize is the default square dimension in pixels when
// Bounds() is zero-sized. Matches the 28-px toolbar icon buttons
// GTK / Aqua headers use so an IconButton drops naturally next to a
// Label or a Button without extra layout maths.
const IconButtonSize = 28

// NewIconButton constructs an IconButton carrying the given glyph +
// click handler. onClick may be nil (a no-op button is still
// rendered). Bounds default to zero so the first Draw() auto-sizes
// the widget to IconButtonSize x IconButtonSize.
func NewIconButton(icon string, onClick func()) *IconButton {
	return &IconButton{Icon: icon, OnClick: onClick}
}

// Draw paints the surface + border and centres Icon inside using the
// toolkit's 5x7 bitmap font. If Bounds().W is zero the widget resizes
// itself to IconButtonSize x IconButtonSize (H preserved when already
// non-zero) before painting.
func (i *IconButton) Draw(p painter.Painter, theme *Theme) {
	r := i.Bounds()
	if r.W == 0 {
		// The auto-size default routes through scaled so a zero-sized button grows
		// with HiDPI and touch density; at compact/1x scaled(IconButtonSize) ==
		// IconButtonSize, so the auto-sized bounds are byte-identical to before.
		r.W = scaled(IconButtonSize)
		if r.H == 0 {
			r.H = scaled(IconButtonSize)
		}
		i.SetBounds(r)
	}
	// Resting face/ink, then the hover/press faces (to match Button), then the
	// muted disabled face. The zero-value hovered/pressed keep the resting
	// draw byte-identical to before these faces existed.
	face, ink, border := theme.Surface, theme.OnSurface, theme.Border
	switch {
	case i.pressed:
		face, ink = theme.Accent, theme.Background
	case i.hovered:
		face = theme.SurfaceAlt
	}
	if i.Disabled().Get() {
		face, ink, border = mutedFace(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)
	if i.Icon != "" {
		tw := i.textWidth(i.Icon)
		tx := r.X + (r.W-tw)/2
		ty := r.Y + (r.H-i.glyphHeight())/2
		i.drawText(p, tx, ty, i.Icon, ink)
	}
	i.drawFocusRing(p, theme, r)
}

// OnEvent drives the button from pointer events: EventClick presses it (showing
// the pressed face) and fires OnClick, EventMouseUp releases it, EventMouseMove
// tracks the hover face. A Disabled button ignores every kind. OnClick is
// nil-safe.
func (i *IconButton) OnEvent(ev Event) {
	if i.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		i.pressed = true
		i.activate()
	case EventKeyDown:
		// Enter or Space fires OnClick while focused, reusing the click path.
		switch ev.Code {
		case "Enter", " ", "Space":
			i.activate()
		}
	case EventMouseUp:
		i.pressed = false
	case EventMouseMove:
		i.hovered = i.localInBounds(ev.X, ev.Y)
	}
}

// activate fires OnClick (nil-safe) -- the shared mutate+callback path for both
// an EventClick and an Enter/Space key press.
func (i *IconButton) activate() {
	if i.OnClick != nil {
		i.OnClick()
	}
}

// HitRect is the icon button's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]).
// Byte-identical to Bounds under DensityCompact; a compact 28px toolbar button
// exposes a >=44px finger target under DensityTouch without changing its glyph.
func (i *IconButton) HitRect() Rect { return touchHitRect(i.Bounds()) }

// HitTest reports whether a surface point falls on the icon button's
// (touch-clamped) hit rect — the default Bounds().Contains at compact, the
// finger-sized area at touch.
func (i *IconButton) HitTest(px, py int) bool { return i.HitRect().Contains(px, py) }
