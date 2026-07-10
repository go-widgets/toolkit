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
	Icon    string
	OnClick func()
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
		r.W = IconButtonSize
		if r.H == 0 {
			r.H = IconButtonSize
		}
		i.SetBounds(r)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if i.Icon != "" {
		tw := TextWidth(i.Icon)
		tx := r.X + (r.W-tw)/2
		ty := r.Y + (r.H-GlyphHeight())/2
		DrawText(p, tx, ty, i.Icon, theme.OnSurface)
	}
}

// OnEvent fires OnClick on EventClick; other event kinds are ignored.
// OnClick is nil-safe.
func (i *IconButton) OnEvent(ev Event) {
	if ev.Kind == EventClick && i.OnClick != nil {
		i.OnClick()
	}
}
