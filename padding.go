// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Padding wraps a single child, insetting it by a per-side margin so a control
// does not hug the edge of the region it sits in — the "toolbar label needs a
// little breathing room on the left" pattern that apps kept hand-rolling as a
// one-off wrapper widget (go-widgets/desktop's insetWidget was one). Left, Top,
// Right and Bottom are LOGICAL pixels routed through [scaled], so the inset grows
// with HiDPI ([MetricScale]) and touch [Density] exactly like every other box
// metric; negative values clamp to zero. [NewPadding] seeds all four sides to the
// same value for the common uniform case.
//
// Padding is a pure layout wrapper: it paints nothing of its own (Draw forwards
// to the child), forwards events to the child at its inset bounds, and reports
// itself as presentational so a screen reader looks straight through to the
// child. It implements [Measurer], reporting the child's measured size plus the
// two paddings on each axis, so a Padding composes inside an HBox/VBox with
// cross-axis alignment just like any other measurable child.
type Padding struct {
	Base
	// Left, Top, Right, Bottom are the per-side insets in LOGICAL pixels. The
	// zero value is a flush wrapper; negative values clamp to zero at layout
	// time. [NewPadding] sets all four to the same value.
	Left, Top, Right, Bottom int

	child Widget
}

// NewPadding wraps child in a Padding with a uniform inset of all logical pixels
// on every side. child may be nil (the wrapper then just occupies its bounds and
// accepts no events).
func NewPadding(child Widget, all int) *Padding {
	return &Padding{Left: all, Top: all, Right: all, Bottom: all, child: child}
}

// insets returns the four side paddings converted to device pixels at the current
// scale, each negative value clamped to zero.
func (pd *Padding) insets() (l, t, r, b int) {
	clamp := func(v int) int {
		if v <= 0 {
			return 0
		}
		return scaled(v)
	}
	return clamp(pd.Left), clamp(pd.Top), clamp(pd.Right), clamp(pd.Bottom)
}

// SetBounds positions the Padding and seats its child inside the scaled insets.
func (pd *Padding) SetBounds(r Rect) {
	pd.Base.SetBounds(r)
	if pd.child == nil {
		return
	}
	l, t, rr, b := pd.insets()
	pd.child.SetBounds(Rect{X: r.X + l, Y: r.Y + t, W: r.W - l - rr, H: r.H - t - b})
}

// Measure reports the child's measured size grown by the two paddings on each
// axis, so a Padding nested in a box aligns to child+2*pad. The child is offered
// the space that remains after the insets; a child that does not implement
// [Measurer] contributes its current Bounds size instead.
func (pd *Padding) Measure(availW, availH int) (int, int) {
	l, t, r, b := pd.insets()
	cw, ch := 0, 0
	if pd.child != nil {
		iw, ih := availW-l-r, availH-t-b
		if m, ok := pd.child.(Measurer); ok {
			cw, ch = m.Measure(iw, ih)
		} else {
			cb := pd.child.Bounds()
			cw, ch = cb.W, cb.H
		}
	}
	return cw + l + r, ch + t + b
}

// Draw forwards to the child; the wrapper itself paints nothing.
func (pd *Padding) Draw(p painter.Painter, theme *Theme) {
	if pd.child != nil {
		pd.child.Draw(p, theme)
	}
}

// Children yields the wrapped child so generic tree walks (accessibility, text
// selection) descend into it.
func (pd *Padding) Children() []Widget { return nonNil(pd.child) }

// focusableChildren yields the wrapped child so the focus walker can reach a
// focusable descendant through the padding.
func (pd *Padding) focusableChildren() []Widget { return nonNil(pd.child) }

// A11y reports the Padding as presentational: it arranges a child and is not
// itself content, so a reader looks through it to the child.
func (pd *Padding) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// OnEvent routes keyboard events through the focus system, then forwards pointer
// events to the child (in the child's local coordinates) when they land inside
// it. A move is forwarded unconditionally so the child can clear a hover face
// once the pointer leaves it; a click also moves focus to the focusable it hits.
func (pd *Padding) OnEvent(ev Event) {
	if routeFocusKey(pd, ev) {
		return
	}
	if pd.child == nil {
		return
	}
	pr := pd.Bounds()
	if ev.Kind == EventMouseMove {
		pd.child.OnEvent(translateEvent(ev, pr, pd.child.Bounds()))
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(pd, sx, sy)
	}
	if pd.child.HitTest(sx, sy) {
		pd.child.OnEvent(translateEvent(ev, pr, pd.child.Bounds()))
	}
}
