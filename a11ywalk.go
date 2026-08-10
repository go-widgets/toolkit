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
	var walk func(Widget)
	walk = func(x Widget) {
		if x == nil {
			return
		}
		if a, ok := x.(Accessible); ok {
			if info := a.A11y(); info.Role != RolePresentation {
				out = append(out, A11yNode{A11yInfo: info, Rect: x.Bounds()})
			}
		}
		if c, ok := x.(childContainer); ok {
			for _, child := range c.Children() {
				walk(child)
			}
		}
	}
	walk(w)
	return out
}
