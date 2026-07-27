// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Corner names one of the six standard docking positions inside a host
// rectangle, used to anchor transient overlays (Toast, Notification) to a
// screen edge. TopLeft is the zero value.
//
// The two *Center corners centre the overlay horizontally; the four true
// corners inset it from the nearer horizontal edge. All six inset from the
// nearer vertical edge, so top corners stack downward and bottom corners
// stack upward.
type Corner int

const (
	// TopLeft docks against the top + left edges.
	TopLeft Corner = iota
	// TopRight docks against the top + right edges.
	TopRight
	// BottomLeft docks against the bottom + left edges.
	BottomLeft
	// BottomRight docks against the bottom + right edges.
	BottomRight
	// TopCenter docks against the top edge, horizontally centred.
	TopCenter
	// BottomCenter docks against the bottom edge, horizontally centred.
	BottomCenter
)

// anchorCorner returns a w×h rect placed at corner of host, inset by margin
// from the docked edges and pushed inward by offset px along the vertical
// stacking axis (top corners downward, bottom corners upward). offset lets a
// caller lay out a stack of overlays sharing one corner.
func anchorCorner(host Rect, w, h int, corner Corner, margin, offset int) Rect {
	var x, y int
	switch corner {
	case TopRight:
		x = host.X + host.W - margin - w
		y = host.Y + margin + offset
	case BottomLeft:
		x = host.X + margin
		y = host.Y + host.H - margin - h - offset
	case BottomRight:
		x = host.X + host.W - margin - w
		y = host.Y + host.H - margin - h - offset
	case TopCenter:
		x = host.X + (host.W-w)/2
		y = host.Y + margin + offset
	case BottomCenter:
		x = host.X + (host.W-w)/2
		y = host.Y + host.H - margin - h - offset
	default: // TopLeft
		x = host.X + margin
		y = host.Y + margin + offset
	}
	return Rect{X: x, Y: y, W: w, H: h}
}
