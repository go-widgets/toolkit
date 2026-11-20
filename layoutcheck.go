// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
)

// LayoutKind is what is wrong with a widget's placement.
type LayoutKind int

const (
	// LayoutOutside is a child whose rectangle is not inside its parent's. It is
	// always a defect: a widget must not draw outside its own bounds, so a child
	// that sticks out of its parent is drawing over a sibling, over the window's
	// chrome, or off the surface entirely. Nothing in this package does it on
	// purpose -- a popover reaches outside its OWNER's bounds, but it is drawn
	// through [PopoverHost] rather than by having bounds out there.
	LayoutOutside LayoutKind = iota
	// LayoutUnplaced is a child with no width or no height inside a parent that
	// has both. It is USUALLY a defect -- a widget nobody laid out, which then
	// draws nothing and hit-tests nothing, silently.
	//
	// It is not always: a [CardLayout] collapses every item but the active one to
	// an empty rectangle on purpose, and so does a box given an empty rect, which
	// is how a hidden container leaves no leaf with stale bounds. So this kind is
	// reported separately, for a caller to read rather than to assert on blindly.
	LayoutUnplaced
)

// String names the kind.
func (k LayoutKind) String() string {
	if k == LayoutOutside {
		return "outside its parent"
	}
	return "never placed"
}

// LayoutProblem is one thing wrong with a laid-out tree.
type LayoutProblem struct {
	// Kind is what is wrong.
	Kind LayoutKind
	// Widget is the widget it is wrong about, and Parent the one that should
	// have placed it.
	Widget, Parent Widget
	// Path names the widgets from the root down to this one by type, so a
	// message says where in the tree to look.
	Path string
	// Rect and In are the child's rectangle and its parent's.
	Rect, In Rect
}

// Error is the sentence to print.
func (p LayoutProblem) Error() string {
	return fmt.Sprintf("%s: %v %+v, parent %+v", p.Path, p.Kind, p.Rect, p.In)
}

// LayoutProblems walks a laid-out tree and reports every child that is outside
// its parent or was never placed.
//
// It is the check for the failure this package cannot make impossible: layout is
// a widget positioning its children, and a widget that positions them WRONG --
// or forgets to, and does it inside Draw instead, so nothing has bounds until the
// first frame -- produces a tree that compiles, passes every unit test, and shows
// a window with a control missing or drawn over another one. Four widgets in this
// package did exactly that, and each was found by a person looking at a picture.
//
// It costs one walk and no allocation when nothing is wrong, so a host may call
// it after a resize behind a debug flag; but the place it belongs is a TEST,
// where a layout is checked without a display, a window server or a person.
//
// Call it after SetBounds. Before that, everything is unplaced and rightly so.
func LayoutProblems(root Widget) []LayoutProblem {
	if root == nil {
		return nil
	}
	var out []LayoutProblem
	checkLayout(root, fmt.Sprintf("%T", root), &out)
	return out
}

// checkLayout is the recursive half: it compares each child against the parent
// it was given, then descends.
func checkLayout(parent Widget, path string, out *[]LayoutProblem) {
	if g, ok := parent.(*GraphView); ok {
		*out = append(*out, graphProblems(g, path)...)
	}
	c, ok := parent.(childContainer)
	if !ok {
		return
	}
	pr := parent.Bounds()
	_, scrolls := parent.(scrollViewport)
	for _, kid := range c.Children() {
		if kid == nil {
			continue
		}
		kr := kid.Bounds()
		where := path + " > " + strings.TrimPrefix(fmt.Sprintf("%T", kid), "*toolkit.")
		switch {
		case kr.W <= 0 || kr.H <= 0:
			if pr.W > 0 && pr.H > 0 {
				*out = append(*out, LayoutProblem{
					Kind: LayoutUnplaced, Widget: kid, Parent: parent,
					Path: where, Rect: kr, In: pr,
				})
			}
		case scrolls:
			// A scroll viewport's content is meant to reach past it: that is what
			// the scrolling is for.
		case !inside(kr, pr):
			*out = append(*out, LayoutProblem{
				Kind: LayoutOutside, Widget: kid, Parent: parent,
				Path: where, Rect: kr, In: pr,
			})
		}
		checkLayout(kid, where, out)
	}
}

// scrollViewport is a container whose children may extend past its bounds
// because it scrolls them into view: LayoutProblems does not call that outside.
type scrollViewport interface{ ScrollsContent() bool }

// ScrollsContent says a [ScrollView] is a scroll viewport.
func (*ScrollView) ScrollsContent() bool { return true }

// graphProblems checks a GraphView's own geometry, which is not made of
// widgets: every node's box must be inside its cluster's box, and every cluster's
// box must be as wide as the area the graph flows in (the widget less its scroll
// bar), however narrow that is. A graph that scrolls vertically is not a problem;
// a box reaching past the right edge is, since nothing scrolls sideways.
func graphProblems(g *GraphView, path string) []LayoutProblem {
	g.layout()
	var out []LayoutProblem
	r := g.Bounds()
	right := r.X + r.W
	if g.hasBar {
		right -= scrollbarWidth
	}
	for _, c := range g.Model.Clusters.Slice() {
		cb, ok := g.clusterBox[c.ID]
		if !ok {
			continue
		}
		where := path + " > cluster " + c.ID
		if cb.X < r.X || cb.X+cb.W > right {
			out = append(out, LayoutProblem{Kind: LayoutOutside, Widget: g, Parent: g, Path: where, Rect: cb, In: r})
		}
	}
	for _, n := range g.Model.Nodes.Slice() {
		nb, ok := g.nodeBox[n.ID]
		if !ok {
			continue
		}
		if cb := g.clusterBox[n.Cluster]; !inside(nb, cb) {
			out = append(out, LayoutProblem{Kind: LayoutOutside, Widget: g, Parent: g, Path: path + " > node " + n.ID, Rect: nb, In: cb})
		}
	}
	return out
}

// inside reports whether the whole of a is within b. An empty b contains
// nothing, which is why the caller tests that first.
func inside(a, b Rect) bool {
	return a.X >= b.X && a.Y >= b.Y && a.X+a.W <= b.X+b.W && a.Y+a.H <= b.Y+b.H
}
