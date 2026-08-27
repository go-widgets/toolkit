// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SelectionBox is a rectangle outline and nothing else: a border that marks the
// region it encloses without covering any of it.
//
// It is the widget for "this one", where what is being pointed at is not a
// widget at all — a tile in a grid of thumbnails, a cell of a canvas, a captured
// desktop, a drop target, an empty slot where something could go. [Frame] is the
// neighbouring shape and a different thing: it OWNS a child, insets it, and can
// carry a title. This owns nothing and draws one border.
//
// It exists because the alternative is every host stroking its own rectangle.
// The consumer this was written for is a virtual-desktop application whose
// gallery shows six captured screens at once and has to say which one Enter
// would take; it had two painter.StrokeRect calls of its own, which is two
// places for a border to stop scaling with the metric scale, stop following the
// theme, and stop looking like the rest of the interface.
type SelectionBox struct {
	Base

	// Ink is the border's colour. A fully-transparent colour (A==0) is treated
	// as "unset" and falls back to [Theme.Accent], which is what a selection is
	// in the theme's own terms.
	//
	// A host with content it does not control may need a colour the theme does
	// not have: a selection over arbitrary captured desktops has to be found at
	// a glance against whatever is behind it, and an accent chosen to sit
	// politely inside an interface is the wrong tool for that.
	Ink RGBA

	// Label is what a screen reader says about the selection; empty means this
	// border has nothing to say and is skipped. See [SelectionBox.A11y].
	Label string

	// Weight is the border's thickness in LOGICAL pixels; zero means
	// [DefaultSelectionWeight].
	//
	// Logical, so it scales with [SetMetricScale] like every other border in
	// this package. A thickness in device pixels is a border that reads as a
	// hairline on the next display.
	Weight int
}

// DefaultSelectionWeight is how thick a SelectionBox is when nobody says, in
// logical pixels.
//
// Thicker than a widget's own one-pixel frame. A selection border is not chrome
// around a control the eye is already resting on — it is the answer to "which
// one", read across a whole screenful, sometimes at arm's length through a pair
// of glasses.
const DefaultSelectionWeight = 4

// NewSelectionBox returns a SelectionBox in ink. A zero ink takes the theme's
// accent.
func NewSelectionBox(ink RGBA) *SelectionBox { return &SelectionBox{Ink: ink} }

// Draw paints the border INSIDE the widget's own bounds.
//
// Inside, not around: a border drawn outside the rectangle it marks would sit in
// whatever gap the host left between tiles — over the neighbour, if there is
// none — and on an outermost tile it would fall off the surface altogether.
func (s *SelectionBox) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	ink := s.Ink
	if ink.A == 0 {
		ink = theme.Accent
	}
	w := s.Weight
	if w <= 0 {
		w = DefaultSelectionWeight
	}
	if w = scaled(w); w < 1 {
		w = 1
	}
	// Never thicker than half the shorter side: past that the two opposite
	// borders meet and the outline becomes a filled block, which hides the very
	// thing it was marking.
	//
	// And never below one, which is where a rectangle a pixel or two across
	// lands — half of one is nothing, and a border of nothing is a border that
	// disappeared.
	if half := min(r.W, r.H) / 2; w > half {
		if w = half; w < 1 {
			w = 1
		}
	}
	p.StrokeRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, ink, w)
}

// Label, when set, is what a screen reader says about the selection.
//
// A border has no name of its own: what is selected is whatever the host drew
// underneath, which may not be a widget at all — a captured desktop, a cell of a
// canvas. So the host says it, and a SelectionBox with nothing to say reports
// [RolePresentation] and is skipped rather than announced as an anonymous
// rectangle.
//
// It is a plain field and not an Observable because it changes when the
// selection does, which is the host's own event, and a border has no state of
// its own to share.
func (s *SelectionBox) A11y() A11yInfo {
	if s.Label == "" {
		return A11yInfo{Role: RolePresentation}
	}
	return A11yInfo{Role: RoleText, Name: s.Label}
}
