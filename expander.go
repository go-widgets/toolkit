// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Expander is a header row that toggles a content area's visibility.
// The header is ExpanderHeaderH px tall, shows a chevron + label;
// clicking the header flips Expanded + fires OnExpand.
//
// When Expanded, Content occupies the remaining bounds below the
// header. When collapsed, only the header is drawn.
type Expander struct {
	Base
	Label    string
	Expanded bool
	Content  Widget
	OnExpand func(expanded bool)
}

// ExpanderHeaderH is the pixel height of the clickable header row.
const ExpanderHeaderH = 24

// NewExpander builds an Expander with a label + initial content
// widget (may be nil to render header-only).
func NewExpander(label string, content Widget) *Expander {
	return &Expander{Label: label, Content: content}
}

// Draw paints the header (chevron + label) + the content widget
// when Expanded.
func (e *Expander) Draw(p painter.Painter, theme *Theme) {
	r := e.Bounds()
	// Header background.
	fillRect(p, r.X, r.Y, r.W, ExpanderHeaderH, theme.SurfaceAlt)
	// Chevron: small triangle in Theme.OnSurface. Collapsed → right-
	// pointing (▶), expanded → down-pointing (▼). 5-px tall.
	cx := r.X + 6
	cy := r.Y + ExpanderHeaderH/2
	if e.Expanded {
		// ▼ : flat top (widest row), point at bottom (narrow tip).
		// At t=0 the 1-pixel tip lands at cy+2; at t=4 the 9-pixel
		// base lands at cy-2.
		for t := 0; t < 5; t++ {
			fillRect(p, cx-t, cy+2-t, 1+2*t, 1, theme.OnSurface)
		}
	} else {
		// ▶ : flat left (tallest column), point at right (narrow tip).
		// At t=0 the 1-pixel tip lands at cx+2; at t=4 the 9-pixel
		// base lands at cx-2.
		for t := 0; t < 5; t++ {
			fillRect(p, cx+2-t, cy-t, 1, 1+2*t, theme.OnSurface)
		}
	}
	textY := r.Y + (ExpanderHeaderH-GlyphHeight())/2
	DrawText(p, r.X+16, textY, e.Label, theme.OnSurface)
	if e.Expanded && e.Content != nil {
		body := Rect{X: r.X, Y: r.Y + ExpanderHeaderH, W: r.W, H: r.H - ExpanderHeaderH}
		e.Content.SetBounds(body)
		e.Content.Draw(p, theme)
	}
}

// OnEvent: click on the header toggles Expanded + fires OnExpand;
// clicks below the header forward to Content (when expanded).
func (e *Expander) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if ev.Y < ExpanderHeaderH {
		e.Expanded = !e.Expanded
		if e.OnExpand != nil {
			e.OnExpand(e.Expanded)
		}
		return
	}
	if e.Expanded && e.Content != nil {
		e.Content.OnEvent(ev)
	}
}
