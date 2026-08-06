// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ButtonGroup is a segmented cluster of adjacent Buttons rendered as one
// connected control: a single rounded border around the whole group, 1-pixel
// dividers between members, and no per-button border (the members are drawn
// Flat, so the group owns the chrome). Use it for related actions that read as a
// unit — a Back/Forward/Reload nav cluster, a zoom -/+ pair, a view switcher.
//
// The members are ordinary *Button widgets: set each one's Icon / Label /
// OnClick / Disabled / Selected as usual; the group lays them out equally along
// its axis, routes clicks to the member under the pointer, and paints the shared
// frame. Orientation is Horizontal (the zero value) or Vertical.
type ButtonGroup struct {
	Base
	Orientation Orientation
	Buttons     []*Button
}

// NewButtonGroup builds a group over the given buttons, marking each Flat so the
// group draws the shared border instead of per-button outlines.
func NewButtonGroup(buttons ...*Button) *ButtonGroup {
	for _, b := range buttons {
		b.Flat = true
	}
	return &ButtonGroup{Buttons: buttons}
}

// SetBounds positions the members: equal slices along the layout axis (the last
// member absorbs any rounding remainder so the group fills its bounds exactly).
func (g *ButtonGroup) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	n := len(g.Buttons)
	if n == 0 {
		return
	}
	if g.Orientation == Vertical {
		h := r.H / n
		for i, b := range g.Buttons {
			y := r.Y + i*h
			bh := h
			if i == n-1 {
				bh = r.Y + r.H - y // last fills the remainder
			}
			b.SetBounds(Rect{X: r.X, Y: y, W: r.W, H: bh})
		}
		return
	}
	w := r.W / n
	for i, b := range g.Buttons {
		x := r.X + i*w
		bw := w
		if i == n-1 {
			bw = r.X + r.W - x
		}
		b.SetBounds(Rect{X: x, Y: r.Y, W: bw, H: r.H})
	}
}

// Draw paints the group background, each Flat member, the inter-member dividers,
// and one rounded border around the whole cluster.
func (g *ButtonGroup) Draw(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Surface)
	for _, b := range g.Buttons {
		b.Draw(p, theme)
	}
	// Dividers between adjacent members (skip before the first).
	for i := 1; i < len(g.Buttons); i++ {
		mb := g.Buttons[i].Bounds()
		if g.Orientation == Vertical {
			fillRect(p, r.X, mb.Y, r.W, 1, theme.Border)
		} else {
			fillRect(p, mb.X, r.Y, 1, r.H, theme.Border)
		}
	}
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Border)
}

// OnEvent forwards the event to the member under its (group-local) coordinates.
// Button.OnEvent handles the press/release itself, so a routed EventClick fires
// that member's OnClick.
func (g *ButtonGroup) OnEvent(ev Event) {
	gr := g.Bounds()
	ax, ay := gr.X+ev.X, gr.Y+ev.Y // group-local → absolute
	for _, b := range g.Buttons {
		if b.Bounds().Contains(ax, ay) {
			b.OnEvent(ev)
			return
		}
	}
}
