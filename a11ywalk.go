// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// A11yNode is one accessible element together with WHERE it is.
//
// [CollectA11y] answers what a tree contains; this answers where each thing is,
// which is the other half every platform accessibility API asks for. A screen
// reader draws the focus ring, routes a touch or moves the pointer from this
// rectangle, so an element described without one can be read but not pointed
// at.
type A11yNode struct {
	A11yInfo

	// Rect is the element's placement in SURFACE coordinates.
	//
	// It is [Widget.Bounds] verbatim, because bounds in this toolkit are
	// already absolute rather than parent-relative — [translateEvent] converts
	// a parent-local event to child-local by `ev.X + parentRect.X -
	// childRect.X`, which only holds when both rectangles share the surface's
	// origin. Accumulating offsets during the walk, the obvious reading of
	// "placement within its parent surface", would double every position.
	Rect Rect
}

// WalkA11y returns every meaningful element of the tree rooted at w, each with
// its bounds, in visual order.
//
// It descends through any widget that exposes its children via
// [childContainer] — the same convention [CollectRuns] uses — so a host does
// not have to keep a flat list of everything it composed, which is what
// [CollectA11y] requires and what a deeply nested layout makes impractical.
//
// Widgets reporting RolePresentation are skipped exactly as [CollectA11y]
// skips them: ARIA's role="presentation" means "look through me to the content
// inside", so a box, a scrim or a scrollbar contributes nothing to announce —
// but the walk still descends INTO it, because its children usually do.
//
// Nothing else is filtered here. Whether an unnamed or zero-area element is
// worth publishing is a decision for the platform bridge consuming this, which
// knows what its own screen reader does with one.
func WalkA11y(w Widget) []A11yNode {
	var out []A11yNode
	var walk func(x Widget, dx, dy int)
	walk = func(x Widget, dx, dy int) {
		if x == nil {
			return
		}
		if a, ok := x.(Accessible); ok {
			if info := a.A11y(); info.Role != RolePresentation {
				r := x.Bounds()
				r.X, r.Y = r.X+dx, r.Y+dy
				out = append(out, A11yNode{A11yInfo: info, Rect: r})
			}
		}
		if o, ok := x.(childOffsetter); ok {
			ox, oy := o.ChildOffset()
			dx, dy = dx+ox, dy+oy
		}
		if c, ok := x.(childContainer); ok {
			for _, child := range c.Children() {
				walk(child, dx, dy)
			}
		}
	}
	walk(w, 0, 0)
	return out
}

// childOffsetter is implemented by a widget that PAINTS its children at a
// translated origin — a scrolled viewport being the only case today.
//
// This toolkit has no viewport in the painting model: clipping exists
// (painter.Clipper) but coordinate translation does not, so ScrollView draws by
// moving its child's bounds and putting them back. Everything reading bounds
// outside Draw — this walk included — therefore sees the UNSCROLLED position.
// Measured before this existed: a button painted at y=50 under a 250px scroll
// was reported at y=300, which points a screen reader 250 pixels below the
// control it is describing.
//
// Declaring the offset is the smallest honest fix. The real one is a viewport
// that translates as well as clips, at which point this interface disappears
// and the bounds are simply correct.
type childOffsetter interface {
	ChildOffset() (dx, dy int)
}
