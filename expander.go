// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Expander is a header row that toggles a content area's visibility.
// The header is ExpanderHeaderH px tall, shows a chevron + label;
// clicking the header flips the expanded state.
//
// When expanded, Content occupies the remaining bounds below the
// header. When collapsed, only the header is drawn. Label and Content
// are set-once config; the reactive expanded/collapsed state is
// MVVM-only, exposed via [Expander.Expanded].
type Expander struct {
	Base
	focusState
	// Label is the header caption (config); Content is the child widget shown
	// below the header when expanded (config, may be nil for header-only).
	Label   string
	Content Widget

	expanded *mvvm.Observable[bool]
}

// Expanded is the open/closed state as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Expanded field. A header
// click or an Enter/Space key press Sets it; subscribers are notified. It starts
// collapsed (false) and is lazily allocated so a zero-value Expander still runs.
func (e *Expander) Expanded() *mvvm.Observable[bool] {
	if e.expanded == nil {
		e.expanded = mvvm.NewObservable(false)
	}
	return e.expanded
}

// ExpanderHeaderH is the LOGICAL height of the clickable header row. Use
// [ExpanderHeaderHeight] for the height to lay out with: at a metric scale
// above 1 the header is taller, like every other metric.
const ExpanderHeaderH = 24

// ExpanderHeaderHeight is the header height in device pixels at the current
// [MetricScale] and touch [Density]: the scaled [ExpanderHeaderH] clamped UP to
// the density minimum hit target via [TouchTarget]. The header is the clickable
// row, so at [DensityTouch] it grows to the finger floor (>=44 device px);
// under the default [DensityCompact] the clamp is a pass-through, so at
// MetricScale 1 it is exactly the historical raw ExpanderHeaderH. Accordion
// shares this height for its own header rows.
func ExpanderHeaderHeight() int { return TouchTarget(scaled(ExpanderHeaderH)) }

// NewExpander builds an Expander with a label + initial content
// widget (may be nil to render header-only). It starts collapsed; the
// expanded state is an [mvvm.Observable] a host binds via [Expander.Expanded].
func NewExpander(label string, content Widget) *Expander {
	e := &Expander{Label: label, Content: content}
	e.expanded = mvvm.NewObservable(false)
	return e
}

// Draw paints the header (chevron + label) + the content widget
// when Expanded.
func (e *Expander) Draw(p painter.Painter, theme *Theme) {
	r := e.Bounds()
	// Header background.
	fillRect(p, r.X, r.Y, r.W, ExpanderHeaderHeight(), theme.SurfaceAlt)
	// Chevron: small triangle in Theme.OnSurface. Collapsed → right-
	// pointing (▶), expanded → down-pointing (▼). 5-px tall.
	cx := r.X + scaled(6)
	cy := r.Y + ExpanderHeaderHeight()/2
	if e.Expanded().Get() {
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
	textY := r.Y + (ExpanderHeaderHeight()-e.glyphHeight())/2
	e.drawText(p, r.X+scaled(16), textY, e.Label, theme.OnSurface)
	if e.Expanded().Get() && e.Content != nil {
		body := Rect{X: r.X, Y: r.Y + ExpanderHeaderHeight(), W: r.W, H: r.H - ExpanderHeaderHeight()}
		e.Content.SetBounds(body)
		e.Content.Draw(p, theme)
	}
	// Focus ring around the clickable header row (paints nothing when unfocused,
	// so an unfocused render is byte-identical).
	e.drawFocusRing(p, theme, Rect{X: r.X, Y: r.Y, W: r.W, H: ExpanderHeaderHeight()})
}

// OnEvent: click on the header toggles the expanded state; clicks below the
// header forward to Content (when expanded). While focused, Enter/Space toggles
// the header (same path as a header click).
func (e *Expander) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown {
		if e.Disabled().Get() {
			return
		}
		switch ev.Code {
		case "Enter", " ", "Space":
			e.toggle()
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	if ev.Y < ExpanderHeaderHeight() {
		e.toggle()
		return
	}
	if e.Expanded().Get() && e.Content != nil {
		// Content occupies the body below the ExpanderHeaderHeight()-tall header. Bound
		// it (matching Draw) and translate the click into its local frame, so a
		// click on interactive content isn't shifted down by the header height
		// (plus the Expander's own origin) and misrouted.
		r := e.Bounds()
		body := Rect{X: r.X, Y: r.Y + ExpanderHeaderHeight(), W: r.W, H: r.H - ExpanderHeaderHeight()}
		e.Content.SetBounds(body)
		e.Content.OnEvent(translateEvent(ev, r, body))
	}
}

// toggle flips the expanded Observable (notifying subscribers) -- the shared
// mutate path for a header click and an Enter/Space key press.
func (e *Expander) toggle() {
	e.Expanded().Set(!e.Expanded().Get())
}
