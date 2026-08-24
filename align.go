// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Anchor positions a child on ONE axis inside an [AlignBox]: fill the axis, or
// keep the child's intrinsic size and pin it to the start (left / top), the
// centre, or the end (right / bottom). The zero value AnchorFill preserves the
// plain "child fills the box" behaviour, so a bare AlignBox is transparent until
// an axis is anchored.
type Anchor int

const (
	// AnchorFill stretches the child to fill the axis (the default).
	AnchorFill Anchor = iota
	// AnchorStart pins the child, at its intrinsic size, to the leading edge
	// (left horizontally, top vertically).
	AnchorStart
	// AnchorMiddle centres the child, at its intrinsic size, on the axis.
	AnchorMiddle
	// AnchorEnd pins the child, at its intrinsic size, to the trailing edge
	// (right horizontally, bottom vertically).
	AnchorEnd
)

// AlignBox seats a single child within its own bounds, positioning it on each
// axis per [Anchor]: fill, or take the child's intrinsic size and pin it
// start / centre / end. It is the shared home for the fixed-height "short control
// centred in a taller toolbar" wrapper apps kept hand-rolling (go-widgets/desktop
// carried a vcenterWidget for exactly this), and more generally for centring any
// intrinsically-sized content in a region.
//
// The child's intrinsic size comes from [Measurer] when the child implements it,
// else from its current Bounds. FixedW / FixedH pin the child's size on that axis
// to a LOGICAL-pixel value (routed through [scaled]) instead — the fader / view
// switcher case, where the control has a known content height it should keep
// while the toolbar around it is taller. A FixedW / FixedH of zero means "use the
// intrinsic size"; a fixed value is ignored on an axis left at AnchorFill.
//
// AlignBox paints nothing of its own, forwards events to the child, and reports
// itself presentational so a reader looks through to the child.
type AlignBox struct {
	Base
	// Horizontal anchors the child on the X axis; the zero value AnchorFill
	// fills the width.
	Horizontal Anchor
	// Vertical anchors the child on the Y axis; the zero value AnchorFill fills
	// the height.
	Vertical Anchor
	// FixedW / FixedH pin the child's size on that axis to this many LOGICAL
	// pixels (scaled at layout), overriding the intrinsic size. Zero uses the
	// intrinsic size; ignored on an axis anchored AnchorFill.
	FixedW, FixedH int

	child Widget
}

// NewAlignBox wraps child in an AlignBox that fills both axes until an anchor is
// set. child may be nil.
func NewAlignBox(child Widget) *AlignBox { return &AlignBox{child: child} }

// NewCenter wraps child in an AlignBox that centres it, at its intrinsic size, on
// both axes — the "drop this control into the middle of the region" shorthand.
func NewCenter(child Widget) *AlignBox {
	return &AlignBox{Horizontal: AnchorMiddle, Vertical: AnchorMiddle, child: child}
}

// NewVCenter wraps child in an AlignBox that fills the width but centres the
// child vertically at a fixed content height of fixedH LOGICAL pixels — a short
// control seated in a taller toolbar. A fixedH of zero centres the child at its
// own intrinsic height instead.
func NewVCenter(child Widget, fixedH int) *AlignBox {
	return &AlignBox{Horizontal: AnchorFill, Vertical: AnchorMiddle, FixedH: fixedH, child: child}
}

// scaledFixed converts a logical-pixel fixed size to device pixels, treating any
// non-positive value as "unset" (0).
func scaledFixed(v int) int {
	if v <= 0 {
		return 0
	}
	return scaled(v)
}

// axisLayout places a child of intrinsic size nat along one axis of extent ext
// starting at base, honouring the anchor and an optional scaled fixed size. An
// AnchorFill axis (or a child with no usable intrinsic size) fills the extent;
// otherwise the child keeps nat (or the fixed size) and is pinned start / centre
// / end.
func axisLayout(base, ext, nat int, a Anchor, fixed int) (off, size int) {
	if a == AnchorFill {
		return base, ext
	}
	if fixed > 0 {
		nat = fixed
	}
	if nat <= 0 || nat >= ext {
		return base, ext
	}
	switch a {
	case AnchorMiddle:
		return base + (ext-nat)/2, nat
	case AnchorEnd:
		return base + (ext - nat), nat
	default: // AnchorStart
		return base, nat
	}
}

// childNatural is the child's intrinsic size: its [Measurer] result when it
// implements one (offered avail on each axis), else its current Bounds size.
func (a *AlignBox) childNatural(availW, availH int) (int, int) {
	if m, ok := a.child.(Measurer); ok {
		return m.Measure(availW, availH)
	}
	b := a.child.Bounds()
	return b.W, b.H
}

// SetBounds positions the AlignBox and seats its child per the two anchors.
func (a *AlignBox) SetBounds(r Rect) {
	a.Base.SetBounds(r)
	if a.child == nil {
		return
	}
	natW, natH := a.childNatural(r.W, r.H)
	x, w := axisLayout(r.X, r.W, natW, a.Horizontal, scaledFixed(a.FixedW))
	y, h := axisLayout(r.Y, r.H, natH, a.Vertical, scaledFixed(a.FixedH))
	a.child.SetBounds(Rect{X: x, Y: y, W: w, H: h})
}

// Measure reports the child's intrinsic size (or the fixed override on each axis)
// so an AlignBox composes inside a box as a measurable child.
func (a *AlignBox) Measure(availW, availH int) (int, int) {
	if a.child == nil {
		return 0, 0
	}
	w, h := a.childNatural(availW, availH)
	if fw := scaledFixed(a.FixedW); fw > 0 {
		w = fw
	}
	if fh := scaledFixed(a.FixedH); fh > 0 {
		h = fh
	}
	return w, h
}

// Draw forwards to the child; the wrapper paints nothing.
func (a *AlignBox) Draw(p painter.Painter, theme *Theme) {
	if a.child != nil {
		a.child.Draw(p, theme)
	}
}

// Children yields the wrapped child so generic tree walks descend into it.
func (a *AlignBox) Children() []Widget { return nonNil(a.child) }

// focusableChildren yields the wrapped child so the focus walker can reach a
// focusable descendant.
func (a *AlignBox) focusableChildren() []Widget { return nonNil(a.child) }

// A11y reports the AlignBox as presentational: it positions a child and is not
// itself content.
func (a *AlignBox) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// OnEvent routes keyboard events through the focus system, then forwards pointer
// events to the child (in child-local coordinates) when they land inside it — a
// move unconditionally, so the child can clear its hover face when the pointer
// leaves.
func (a *AlignBox) OnEvent(ev Event) {
	if routeFocusKey(a, ev) {
		return
	}
	if a.child == nil {
		return
	}
	pr := a.Bounds()
	if ev.Kind == EventMouseMove {
		a.child.OnEvent(translateEvent(ev, pr, a.child.Bounds()))
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(a, sx, sy)
	}
	if a.child.HitTest(sx, sy) {
		a.child.OnEvent(translateEvent(ev, pr, a.child.Bounds()))
	}
}
