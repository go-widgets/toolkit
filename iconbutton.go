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

	// Glyph draws the button's mark instead of rendering Icon as text. It is the
	// same seam [Button.Icon] uses, so a real icon — go-icons/iconoir, an SVG mask,
	// anything that can paint into a rect — replaces the letter that used to
	// stand in for one. Nil keeps the text path, so every existing caller draws
	// byte-identically.
	Glyph func(p painter.Painter, r Rect, ink RGBA)

	// Flat drops the resting face and border: the button is invisible until the
	// pointer is over it, and hover and press paint a ROUNDED background instead
	// of a square one. That is what a close affordance in a title bar wants — a
	// boxed square in the corner of a panel reads as a control that belongs to
	// the content, not to the window. False keeps the framed look every other
	// caller has.
	Flat bool

	hovered bool
	pressed bool
}

// IconButtonFlatRadius is the corner radius of a Flat button's hover and press
// background, in pixels before scaling. IconButtonFlatHoverAlpha and
// IconButtonFlatPressAlpha are how opaque that background is — a veil, not a
// face, so it reads on any ground the host paints behind it.
const (
	IconButtonFlatRadius     = 6
	IconButtonFlatHoverAlpha = 0x22
	IconButtonFlatPressAlpha = 0x44
)

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
	if i.Flat {
		// Nothing at rest; a rounded wash under the pointer.
		//
		// The wash is a translucent veil of the INK, not one of the theme's
		// faces: a flat button sits on whatever its host paints — a title bar in
		// SurfaceAlt, a toolbar in Surface — and a face-coloured hover is
		// invisible on the half of them that share its colour. A veil of the ink
		// darkens a light ground and lightens a dark one, so it shows on both.
		switch {
		case i.pressed:
			fillRoundRect(p, r.X, r.Y, r.W, r.H, scaled(IconButtonFlatRadius),
				withAlpha(ink, IconButtonFlatPressAlpha))
		case i.hovered:
			fillRoundRect(p, r.X, r.Y, r.W, r.H, scaled(IconButtonFlatRadius),
				withAlpha(ink, IconButtonFlatHoverAlpha))
		}
	} else {
		fillRect(p, r.X, r.Y, r.W, r.H, face)
		strokeRect(p, r.X, r.Y, r.W, r.H, border)
	}
	switch {
	case i.Glyph != nil:
		i.Glyph(p, iconButtonGlyphRect(r), ink)
	case i.Icon != "":
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

// iconButtonGlyphRect is the centred square a Glyph is drawn into, inset so the
// mark does not touch the button's edge (or the wash under it).
func iconButtonGlyphRect(r Rect) Rect {
	in := scaled(IconButtonGlyphInset)
	s := r.W - 2*in
	if r.H-2*in < s {
		s = r.H - 2*in
	}
	if s < 1 {
		s = 1
	}
	return Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}
}

// IconButtonGlyphInset is the inset from each edge to the glyph square.
const IconButtonGlyphInset = 7
