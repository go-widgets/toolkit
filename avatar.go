// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Avatar renders a user identity chip: a rounded-square body filled in
// a solid colour with the user's Initials centred inside it in the
// accent-inverted ink. Colour resolves to Theme.Accent when the
// caller-supplied Color is the zero RGBA (a natural default that
// respects the theme); a caller wanting a per-user tint sets Color to
// any opaque RGBA and it will be honoured verbatim.
//
// The rounded shape is faked by clipping the four corner pixels — the
// same three-band recipe Badge uses to look like a pill without touching
// a curve primitive. This keeps Avatar allocation-free and portable to
// every Painter back-end (PixelPainter, CellPainter, SvgPainter).
//
// Auto-sizing: if Bounds().W is zero the first Draw() resizes the
// avatar to AvatarSize x AvatarSize (or AvatarSize x preserved-H when H
// is non-zero). A pre-sized Bounds is honoured verbatim so a fixed
// layout column doesn't shift when the widget is dropped in.
//
// Avatar is passive: it displays and does not respond to input. The
// parent view is responsible for positioning it (typically top-left of a
// message row or the leading edge of a menu item).
type Avatar struct {
	Base
	Initials string
	// Color is the body fill. Leave at the zero RGBA to fall through to
	// Theme.Accent (the theme-tracking default); set to any opaque RGBA
	// to pin the avatar to a per-user tint.
	Color RGBA
}

// AvatarSize is the default square dimension in pixels when Bounds() is
// zero-sized. Matches the 32-px avatar most GTK / Material chat rows use
// so an Avatar drops naturally next to a Label without extra layout.
const AvatarSize = 32

// NewAvatar constructs an Avatar carrying the given initials. Bounds
// default to zero so the first Draw() auto-sizes the widget to
// AvatarSize x AvatarSize. Color defaults to the zero RGBA so the body
// tracks Theme.Accent unless the caller pins it.
func NewAvatar(initials string) *Avatar { return &Avatar{Initials: initials} }

// Draw paints the rounded-square body then centres Initials on top. If
// Bounds().W is zero the widget resizes itself to AvatarSize x
// AvatarSize (H preserved when already non-zero) before painting.
//
// Body colour is Color when non-zero, otherwise Theme.Accent. Ink is
// accentInk(theme) so a GTK-loaded theme's OnAccent override is honoured
// with a fall-through to Theme.Background — the same rule Table +
// Button use for their accent-face branches.
func (a *Avatar) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	if r.W == 0 {
		r.W = AvatarSize
		if r.H == 0 {
			r.H = AvatarSize
		}
		a.SetBounds(r)
	}
	face := a.Color
	if face == (RGBA{}) {
		face = theme.Accent
	}
	// Three-band pill fill: centre strip full-height, two 1-px side
	// columns skipping the top + bottom pixels so the corners read as
	// rounded. Same recipe Badge uses; keeps the shape consistent
	// between Badge + Avatar so they compose visually.
	fillRect(p, r.X+1, r.Y, r.W-2, r.H, face)
	fillRect(p, r.X, r.Y+1, 1, r.H-2, face)
	fillRect(p, r.X+r.W-1, r.Y+1, 1, r.H-2, face)

	tw := TextWidth(a.Initials)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-GlyphHeight)/2
	DrawText(p, tx, ty, a.Initials, accentInk(theme))
}
