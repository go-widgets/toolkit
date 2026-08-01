// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Border arranges up to five named regions — North, South, West, East and
// Center — the classic five-region border layout for application shells. North and
// South span the full width and take a fixed height; West and East then span the
// height that remains between them and take a fixed width; Center fills whatever
// is left. The precedence is structural, so regions may be assigned in any order
// and still lay out correctly (unlike Dock, which carves in insertion order).
//
// Any region may be nil (that edge simply contributes no band). Sizes are the
// extent along each region's own axis — NorthSize/SouthSize are heights,
// West/EastSize are widths — clamped to what the container can give (negative → 0).
//
// Border is a Widget: Draw paints every non-nil region; OnEvent routes by Bounds,
// translating into the matched region's local space.
type Border struct {
	Base
	North, South, East, West, Center         Widget
	NorthSize, SouthSize, EastSize, WestSize int

	// NorthSplit/… add a draggable splitter between that edge region and the
	// centre (with an optional resizable split). The app drives the drag — like Paned —
	// via SplitHandleAt (which handle a point is on) and ResizeSplit (set the new
	// size); OnResize fires after each resize.
	NorthSplit, SouthSplit, EastSplit, WestSplit bool
	OnResize                                     func(side DockSide, size int)

	handles []splitHandle
}

// BorderSplitW is the pixel thickness of a Border splitter handle (matches
// PanedHandleW).
const BorderSplitW = 6

// splitHandle records a drawn splitter's edge and rectangle for hit-testing.
type splitHandle struct {
	side DockSide
	rect Rect
}

// NewBorder builds an empty Border; assign the region fields and their sizes
// directly before the first SetBounds.
func NewBorder() *Border { return &Border{} }

// SetBounds lays out the regions in border precedence (N, S, then W, E, then the
// Center fills the remainder), reusing dockCarve for each edge.
func (b *Border) SetBounds(r Rect) {
	b.Base.SetBounds(r)
	b.handles = b.handles[:0]
	avail := r
	place := func(w Widget, side DockSide, size int, split bool) {
		if w == nil {
			return
		}
		if size < 0 {
			size = 0
		}
		var bar Rect
		bar, avail = dockCarve(side, size, avail)
		w.SetBounds(bar)
		if split {
			var handle Rect
			handle, avail = dockCarve(side, BorderSplitW, avail)
			b.handles = append(b.handles, splitHandle{side: side, rect: handle})
		}
	}
	place(b.North, DockTop, b.NorthSize, b.NorthSplit)
	place(b.South, DockBottom, b.SouthSize, b.SouthSplit)
	place(b.West, DockLeft, b.WestSize, b.WestSplit)
	place(b.East, DockRight, b.EastSize, b.EastSplit)
	if b.Center != nil {
		b.Center.SetBounds(avail)
	}
}

// SplitHandleAt reports which splitter handle (if any) contains the surface point
// (px,py) — the app calls it on mouse-down to start a region resize drag.
func (b *Border) SplitHandleAt(px, py int) (side DockSide, ok bool) {
	for _, h := range b.handles {
		if h.rect.Contains(px, py) {
			return h.side, true
		}
	}
	return 0, false
}

// ResizeSplit sets the given edge region's size (clamped to [0, the border's
// extent on that axis]), re-lays out, and fires OnResize. The app computes size
// from the drag — e.g. NorthSize + dy for the north handle.
func (b *Border) ResizeSplit(side DockSide, size int) {
	if size < 0 {
		size = 0
	}
	r := b.Bounds()
	switch side {
	case DockTop, DockBottom:
		if size > r.H {
			size = r.H
		}
	default: // DockLeft, DockRight
		if size > r.W {
			size = r.W
		}
	}
	switch side {
	case DockTop:
		b.NorthSize = size
	case DockBottom:
		b.SouthSize = size
	case DockLeft:
		b.WestSize = size
	default: // DockRight
		b.EastSize = size
	}
	b.SetBounds(r)
	if b.OnResize != nil {
		b.OnResize(side, size)
	}
}

// regions returns the non-nil regions in draw/hit order (edges before centre).
func (b *Border) regions() []Widget {
	out := make([]Widget, 0, 5)
	for _, w := range []Widget{b.North, b.South, b.West, b.East, b.Center} {
		if w != nil {
			out = append(out, w)
		}
	}
	return out
}

// Draw paints every non-nil region, then any splitter handles over the seams.
func (b *Border) Draw(p painter.Painter, theme *Theme) {
	for _, w := range b.regions() {
		w.Draw(p, theme)
	}
	for _, h := range b.handles {
		fillRect(p, h.rect.X, h.rect.Y, h.rect.W, h.rect.H, theme.SurfaceAlt)
	}
}

// OnEvent forwards to the first region whose Bounds contains the point,
// translated into that region's local space.
func (b *Border) OnEvent(ev Event) {
	pr := b.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, w := range b.regions() {
		if w.Bounds().Contains(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
}
