// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Border arranges up to five named regions — North, South, West, East and
// Center — the classic Ext JS border layout for application shells. North and
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
}

// NewBorder builds an empty Border; assign the region fields and their sizes
// directly before the first SetBounds.
func NewBorder() *Border { return &Border{} }

// SetBounds lays out the regions in border precedence (N, S, then W, E, then the
// Center fills the remainder), reusing dockCarve for each edge.
func (b *Border) SetBounds(r Rect) {
	b.Base.SetBounds(r)
	avail := r
	place := func(w Widget, side DockSide, size int) {
		if w == nil {
			return
		}
		if size < 0 {
			size = 0
		}
		var bar Rect
		bar, avail = dockCarve(side, size, avail)
		w.SetBounds(bar)
	}
	place(b.North, DockTop, b.NorthSize)
	place(b.South, DockBottom, b.SouthSize)
	place(b.West, DockLeft, b.WestSize)
	place(b.East, DockRight, b.EastSize)
	if b.Center != nil {
		b.Center.SetBounds(avail)
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

// Draw paints every non-nil region.
func (b *Border) Draw(p painter.Painter, theme *Theme) {
	for _, w := range b.regions() {
		w.Draw(p, theme)
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
