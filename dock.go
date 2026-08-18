// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DockSide names the edge a docked bar attaches to.
type DockSide int

const (
	DockTop    DockSide = iota // bar spans the top, given height = size
	DockBottom                 // bar spans the bottom
	DockLeft                   // bar spans the left, given width = size
	DockRight                  // bar spans the right
)

// dockItem pairs a docked widget with its edge and its extent along the dock
// axis (height for top/bottom, width for left/right).
type dockItem struct {
	w    Widget
	side DockSide
	size int
}

// Dock arranges bars against the edges of its bounds and lets a single body
// widget fill whatever space is left in the centre — a docked-items
// model. Bars are docked in insertion order, so a top bar added before a left bar
// spans the full width above the left bar, and the left bar only gets the height
// that remains. Any edge may hold several bars, which stack inward in order.
//
// Dock is a Widget: Draw paints the body then the bars; OnEvent routes by Bounds,
// translating into the matched child's local space. The body may be nil (a
// bars-only frame).
type Dock struct {
	Base
	body   Widget
	docked []dockItem
}

// NewDock builds a Dock around body (nil for a bars-only frame). Add bars with
// Dock().
func NewDock(body Widget) *Dock { return &Dock{body: body} }

// Dock attaches w to the given edge with size pixels along the dock axis (its
// cross extent fills the space still available). size is clamped to ≥0.
func (d *Dock) Dock(w Widget, side DockSide, size int) {
	if size < 0 {
		size = 0
	}
	d.docked = append(d.docked, dockItem{w: w, side: side, size: size})
	d.SetBounds(d.Bounds())
}

// dockCarve splits avail into a bar rect against the given side (its size clamped
// to what avail can give on the dock axis) and the rectangle that remains.
func dockCarve(side DockSide, size int, avail Rect) (bar, rest Rect) {
	switch side {
	case DockTop:
		if size > avail.H {
			size = avail.H
		}
		bar = Rect{X: avail.X, Y: avail.Y, W: avail.W, H: size}
		rest = Rect{X: avail.X, Y: avail.Y + size, W: avail.W, H: avail.H - size}
	case DockBottom:
		if size > avail.H {
			size = avail.H
		}
		bar = Rect{X: avail.X, Y: avail.Y + avail.H - size, W: avail.W, H: size}
		rest = Rect{X: avail.X, Y: avail.Y, W: avail.W, H: avail.H - size}
	case DockLeft:
		if size > avail.W {
			size = avail.W
		}
		bar = Rect{X: avail.X, Y: avail.Y, W: size, H: avail.H}
		rest = Rect{X: avail.X + size, Y: avail.Y, W: avail.W - size, H: avail.H}
	default: // DockRight
		if size > avail.W {
			size = avail.W
		}
		bar = Rect{X: avail.X + avail.W - size, Y: avail.Y, W: size, H: avail.H}
		rest = Rect{X: avail.X, Y: avail.Y, W: avail.W - size, H: avail.H}
	}
	return bar, rest
}

// SetBounds carves each docked bar off the current available rectangle in
// insertion order, then gives the body whatever remains.
func (d *Dock) SetBounds(r Rect) {
	d.Base.SetBounds(r)
	avail := r
	for _, it := range d.docked {
		var bar Rect
		bar, avail = dockCarve(it.side, it.size, avail)
		it.w.SetBounds(bar)
	}
	if d.body != nil {
		d.body.SetBounds(avail)
	}
}

// Draw paints the body first, then the bars over any shared edge (they never
// overlap, so order is cosmetic).
func (d *Dock) Draw(p painter.Painter, theme *Theme) {
	if d.body != nil {
		d.body.Draw(p, theme)
	}
	for _, it := range d.docked {
		it.w.Draw(p, theme)
	}
}

// OnEvent forwards to the first bar whose Bounds contains the point, else the
// body, translating into that child's local space.
func (d *Dock) OnEvent(ev Event) {
	pr := d.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, it := range d.docked {
		if it.w.HitTest(sx, sy) {
			it.w.OnEvent(translateEvent(ev, pr, it.w.Bounds()))
			return
		}
	}
	if d.body != nil && d.body.HitTest(sx, sy) {
		d.body.OnEvent(translateEvent(ev, pr, d.body.Bounds()))
	}
}
