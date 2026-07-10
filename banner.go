// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Banner is a full-width persistent inline message strip, modelled on
// GTK 4's AdwBanner. Distinct from Alert (persistent, coloured by
// severity) in two ways:
//
//  1. Banner is REVEAL-driven: Revealed toggles the whole strip on and
//     off, letting the host wire dismiss and re-show without dropping
//     the widget from the tree.
//  2. Banner carries an optional right-aligned action button; a click
//     inside the button fires OnAction. Alert has no interactive slot.
//
// The banner paints in Theme.Accent so it reads as a system message
// rather than a semantic-severity Alert; the action button is drawn
// as a bordered box in the accent-inverted ink so it stays legible.
type Banner struct {
	Base
	Text        string
	ButtonLabel string
	Revealed    bool
	OnAction    func()
}

// Banner sizing constants. BannerPadX/PadY are the internal margin
// between the strip edges and the text; BannerButtonPadX is the inner
// horizontal inset between the button label and its border box.
const (
	BannerPadX       = 12
	BannerPadY       = 8
	BannerButtonPadX = 8
)

// NewBanner constructs a Banner with the given Text. Revealed starts
// true so a freshly-constructed banner is visible; ButtonLabel is
// empty by default (no action slot rendered).
func NewBanner(text string) *Banner {
	return &Banner{Text: text, Revealed: true}
}

// buttonRect returns the surface-coordinate rect of the action button
// + a flag reporting whether the banner currently has an actionable
// button. When ButtonLabel is empty the flag is false and callers
// skip drawing / hit-testing entirely.
func (b *Banner) buttonRect() (Rect, bool) {
	if b.ButtonLabel == "" {
		return Rect{}, false
	}
	r := b.Bounds()
	btnW := TextWidth(b.ButtonLabel) + 2*BannerButtonPadX
	btnH := GlyphHeight() + 2*BannerPadY
	return Rect{
		X: r.X + r.W - btnW - BannerPadX,
		Y: r.Y + (r.H-btnH)/2,
		W: btnW,
		H: btnH,
	}, true
}

// Draw paints the accent-filled strip + the Text ink. When ButtonLabel
// is non-empty an outlined action button is drawn right-aligned inside
// BannerPadX of the trailing edge. Nothing drawn when !Revealed.
func (b *Banner) Draw(p painter.Painter, theme *Theme) {
	if !b.Revealed {
		return
	}
	r := b.Bounds()
	ink := accentInk(theme)
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Accent)
	DrawText(p, r.X+BannerPadX, r.Y+BannerPadY, b.Text, ink)
	if br, ok := b.buttonRect(); ok {
		strokeRect(p, br.X, br.Y, br.W, br.H, ink)
		DrawText(p, br.X+BannerButtonPadX, br.Y+BannerPadY, b.ButtonLabel, ink)
	}
}

// OnEvent handles a click inside the action button. Events with a
// Kind other than EventClick are ignored; a click that falls outside
// the button rect is dropped; a click on a Banner without an action
// button (empty ButtonLabel) is dropped; a click with a nil OnAction
// is dropped silently -- the button is drawable but inert.
func (b *Banner) OnEvent(ev Event) {
	if ev.Kind != EventClick || !b.Revealed {
		return
	}
	br, ok := b.buttonRect()
	if !ok {
		return
	}
	// ev.X / ev.Y are widget-local per the package convention; convert
	// to surface coords for the hit-test against buttonRect (which is
	// itself in surface coords).
	r := b.Bounds()
	sx, sy := ev.X+r.X, ev.Y+r.Y
	if !br.Contains(sx, sy) {
		return
	}
	if b.OnAction != nil {
		b.OnAction()
	}
}
