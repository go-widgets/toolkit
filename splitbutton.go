// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SplitButton is a two-part button: a primary action face on the left
// plus an attached narrow arrow face on the right that opens a
// secondary action (typically a menu). Mirrors GTK's SplitButton and
// GtkMenuButton — one click target for the default action, a separate
// click target for "show the alternatives".
//
// When Arrow is false the arrow slot is not drawn and OnArrow is
// ignored — the widget degrades to a solid Accent-face action button,
// so a caller can toggle the split visual at runtime without swapping
// widgets.
//
// The two faces share theme.Accent as their fill; the label + arrow
// glyph render in accentInk(theme) — theme.Extra["OnAccent"] with a
// fall-through to theme.Background, matching Button + Table + Avatar.
type SplitButton struct {
	Base
	Label   string
	Arrow   bool
	OnClick func()
	OnArrow func()
}

// SplitButtonArrowW is the pixel width of the arrow slot on the right
// edge when Arrow is true. Sized to comfortably fit the 5x7 arrow
// glyph plus symmetric padding on either side.
const SplitButtonArrowW = 20

// SplitButtonPadX is the horizontal padding a caller should reserve
// on either side of the label when positioning a sibling widget flush
// with the main slot's inner edge. The label itself is rendered
// centred; this constant is exported for external layout code that
// wants to align against the same inset.
const SplitButtonPadX = 12

// NewSplitButton constructs a SplitButton with Arrow enabled by
// default and OnArrow left nil. onClick may be nil (a no-op primary
// action is still rendered).
func NewSplitButton(label string, onClick func()) *SplitButton {
	return &SplitButton{Label: label, Arrow: true, OnClick: onClick}
}

// Draw paints the two-slot face. Both slots fill in theme.Accent;
// when Arrow is true a 1-px theme.Border separator is drawn between
// them and a small "v" glyph is centred in the arrow slot. Ink for
// both the label and the arrow glyph is accentInk(theme) so the
// text stays legible against the Accent face and honours any
// theme.Extra["OnAccent"] override.
func (s *SplitButton) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	mainW := r.W
	if s.Arrow {
		mainW = r.W - SplitButtonArrowW
	}
	ink := accentInk(theme)
	// Main slot fill.
	fillRect(p, r.X, r.Y, mainW, r.H, theme.Accent)
	if s.Arrow {
		// Arrow slot fill (same Accent face) then a 1-px Border
		// separator at the boundary + the "v" arrow glyph centred
		// in the arrow slot.
		fillRect(p, r.X+mainW, r.Y, SplitButtonArrowW, r.H, theme.Accent)
		fillRect(p, r.X+mainW, r.Y, 1, r.H, theme.Border)
		aw := TextWidth("v")
		ax := r.X + mainW + (SplitButtonArrowW-aw)/2
		ay := r.Y + (r.H-GlyphHeight())/2
		DrawText(p, ax, ay, "v", ink)
	}
	if s.Label != "" {
		tw := TextWidth(s.Label)
		tx := r.X + (mainW-tw)/2
		ty := r.Y + (r.H-GlyphHeight())/2
		DrawText(p, tx, ty, s.Label, ink)
	}
}

// OnEvent routes clicks to OnClick or OnArrow depending on where the
// click landed. ev.X is widget-local; when Arrow is true a click in
// the right SplitButtonArrowW pixels fires OnArrow, otherwise OnClick.
// Both callbacks are nil-safe. Non-click event kinds are ignored.
func (s *SplitButton) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := s.Bounds()
	if s.Arrow && ev.X >= r.W-SplitButtonArrowW {
		if s.OnArrow != nil {
			s.OnArrow()
		}
		return
	}
	if s.OnClick != nil {
		s.OnClick()
	}
}
