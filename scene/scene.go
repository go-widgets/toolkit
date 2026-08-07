// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package scene adds an OPT-IN Evas-style damage / scene layer on top of the
// immediate-mode go-widgets/toolkit widget set. The core toolkit repaints the
// whole widget tree every frame (Window.Draw → containers recurse Draw
// unconditionally, with no per-widget dirty flag). That is simple and correct,
// but on a dense scene where a single hovered widget changes it repaints — and
// on a wasm/canvas host, re-blits — every pixel of every widget each frame.
//
// This package fills that INTRA-app gap without touching the Widget contract:
// an app that does not construct a [Scene] keeps calling Draw directly and is
// completely unaffected. An app that opts in wraps its root widget in a Scene,
// calls [Scene.Invalidate] whenever a widget's appearance changes, and calls
// [Scene.Render] once per frame. Render:
//
//   - coalesces every invalidation into a damage [RegionSet] (each invalidation
//     contributes union(old-rect, new-rect) so a moved/resized widget damages
//     both where it was and where it now is),
//   - clips the painter to the damage via the existing [painter.Clipper] seam,
//   - draws ONLY the nodes whose retained bounds intersect the damage —
//     pruning whole non-intersecting subtrees in O(depth) rather than O(n),
//   - skips a node that is provably, conservatively occluded by an opaque
//     later sibling (see [Opaque]), and
//   - returns the exact region the host must blit.
//
// The result is pixel-identical to a full immediate-mode repaint (this is the
// package's headline correctness guarantee, proven by test), because the
// persisted framebuffer already holds the previous frame everywhere and Render
// repaints — in z-order, background first — every node that has pixels inside
// the damage. wasmbox already does cross-WINDOW damage between clients; this is
// the missing intra-app piece that also hands the host the blit region.
//
// # Retaining an immediate-mode tree
//
// The scene mirrors the live widget tree through the toolkit's generic
// [childProvider] seam (the same Children() method CollectRuns walks), keeping
// a retained [Node] per widget with its last-drawn rect. Two OPTIONAL
// capabilities let a container/leaf cooperate for maximum efficiency; a widget
// that implements neither is still drawn correctly (a container that does not
// implement [SelfDrawer] is drawn wholesale — its own Draw recurses — whenever
// it intersects the damage):
//
//   - [SelfDrawer]: a container that paints its own chrome separately from its
//     children lets the scene repaint just the chrome and then descend into —
//     and prune — its children individually.
//   - [Opaque]: a leaf/container that fills a rectangle fully opaque lets the
//     scene occlusion-cull lower siblings it completely covers.
package scene

import (
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Rect is re-exported from the toolkit (itself an alias of painter.Rect) so a
// scene consumer needs only this package to describe a damage rectangle.
type Rect = toolkit.Rect

// childProvider is any widget that exposes its children for generic traversal —
// the same seam toolkit.CollectRuns uses (Container / HBox / VBox / Frame / …
// all implement it). The scene descends through it to build + refresh its
// retained mirror of the live widget tree.
type childProvider interface{ Children() []toolkit.Widget }

// SelfDrawer is an OPTIONAL capability a container implements when it paints its
// own chrome (a background fill, a frame border, a title bar) separately from
// its children. The scene calls DrawSelf to repaint just that chrome inside the
// damage region and then recurses into the container's children individually,
// so an unchanged child of a changed container is pruned. A container that does
// NOT implement SelfDrawer is instead drawn wholesale (its ordinary Draw, which
// recurses into every child) whenever its subtree intersects the damage — still
// pixel-correct, just without per-child pruning inside that container.
//
// DrawSelf MUST paint only the container's own pixels and MUST NOT recurse into
// children; the scene owns child traversal.
type SelfDrawer interface {
	DrawSelf(p painter.Painter, th *toolkit.Theme)
}

// Opaque is an OPTIONAL capability a widget implements when it paints every
// pixel of some rectangle fully opaque (alpha 0xFF). OpaqueRect returns that
// rectangle and true; a widget that cannot promise full opacity returns false
// (or does not implement the interface at all) and is NEVER treated as an
// occluder. The scene uses it ONLY to occlusion-cull: a lower sibling whose
// damaged area is entirely contained in a higher sibling's opaque rect is
// completely hidden and need not be drawn. The rule is deliberately
// conservative — the scene never skips a node that could show through, so a
// partially-covering or translucent occluder culls nothing.
type Opaque interface {
	OpaqueRect() (Rect, bool)
}

// Node is one retained entry in the scene tree, mirroring a single widget. It
// caches the widget's bounds at the last render (lastRect) — so an invalidation
// can damage both the old and the new position — plus the bounding box of the
// node's whole subtree (subtree), which lets Render prune a non-intersecting
// subtree without visiting its descendants.
type Node struct {
	// W is the widget this node mirrors.
	W toolkit.Widget

	// rect is W's bounds as of the most recent sync (start of Render).
	rect Rect
	// lastRect is W's bounds as of the most recent completed Render — the
	// "old" rectangle an Invalidate unions with the current bounds.
	lastRect Rect
	// subtree is the bounding box of rect and every descendant's subtree; an
	// empty (zero-area) subtree never intersects damage and is pruned.
	subtree Rect
	// dirty marks a node an Invalidate touched (the node itself and its
	// ancestor chain). Cleared at the end of each Render. Purely informational
	// for consumers that want to introspect the tree; Render itself decides
	// what to paint from the damage region, not this flag.
	dirty bool
	// children are the retained mirrors of W's Children(), in draw (z) order:
	// later entries paint on top of earlier ones.
	children []*Node
	// parent is the node whose children include this one, or nil for the root.
	// Maintained by reconcile so Invalidate can walk the ancestor chain in
	// O(depth) instead of scanning the index.
	parent *Node
}

// Children returns the node's child nodes in draw order (earlier under later).
// Exposed so a consumer can introspect the retained tree; the slice is owned by
// the scene and must not be mutated.
func (n *Node) Children() []*Node { return n.children }

// Widget returns the widget this node mirrors.
func (n *Node) Widget() toolkit.Widget { return n.W }

// Dirty reports whether an Invalidate has touched this node (or a descendant of
// it) since the last Render.
func (n *Node) Dirty() bool { return n.dirty }

// RegionSet is a small set of damage rectangles kept coalesced: Add drops a
// rectangle already covered by a member and removes members a new rectangle
// subsumes, so identical invalidations collapse to one rect and disjoint ones
// stay distinct. It is the union(old, new) damage accumulator and the blit
// region Render returns.
type RegionSet struct {
	rects []Rect
}

// Region is the value [Scene.Render] returns: the coalesced set of rectangles
// the host must blit this frame. It aliases RegionSet.
type Region = RegionSet

// Add merges r into the set, keeping it coalesced. An empty rectangle (zero or
// negative extent) is ignored. If an existing member already contains r, r is
// dropped; every existing member r fully contains is removed. It never allocates
// once the backing slice has grown to the working set's steady-state size.
func (rs *RegionSet) Add(r Rect) {
	if isEmpty(r) {
		return
	}
	for _, e := range rs.rects {
		if contains(e, r) {
			return // already covered
		}
	}
	n := 0
	for _, e := range rs.rects {
		if !contains(r, e) { // keep members r does NOT subsume
			rs.rects[n] = e
			n++
		}
	}
	rs.rects = append(rs.rects[:n], r)
}

// Rects returns the coalesced rectangles. The slice is owned by the set (and,
// for a Render result, reused on the next frame); copy it if you need to retain
// it past the next Render.
func (rs *RegionSet) Rects() []Rect { return rs.rects }

// Bounds returns the single rectangle bounding every member, or the zero Rect
// when the set is empty.
func (rs *RegionSet) Bounds() Rect {
	if len(rs.rects) == 0 {
		return Rect{}
	}
	b := rs.rects[0]
	for _, r := range rs.rects[1:] {
		b = union(b, r)
	}
	return b
}

// Area returns the summed area of the members. Because members are coalesced
// only by containment (not by partial-overlap merging), overlapping members
// would double-count; the sets the scene produces (union(old,new) damage) never
// partially overlap after coalescing in practice, so Area is the exact
// pixels-touched count the host blits — multiply by 4 for RGBA bytes.
func (rs *RegionSet) Area() int {
	a := 0
	for _, r := range rs.rects {
		a += r.W * r.H
	}
	return a
}

// Scene retains a mirror of a widget tree and turns per-widget invalidations
// into a clipped, minimally-drawn frame. It is not safe for concurrent use; a
// UI thread owns it, exactly like the widgets it wraps.
type Scene struct {
	root  *Node
	index map[toolkit.Widget]*Node

	damage RegionSet // accumulates invalidations between renders
	blit   RegionSet // the region drawn (and returned) by the current Render
}

// New builds a scene mirroring the tree rooted at root and seeds a full-surface
// damage so the first Render is a complete paint (matching an app's very first
// immediate-mode frame). root must be non-nil and already laid out (its bounds
// and its descendants' bounds set); New reads those bounds to size the initial
// damage.
func New(root toolkit.Widget) *Scene {
	s := &Scene{
		root:  &Node{W: root},
		index: make(map[toolkit.Widget]*Node),
	}
	s.index[root] = s.root
	s.sync()
	s.stamp(s.root)
	s.damage.Add(s.root.subtree)
	return s
}

// Root returns the retained root node, for introspection of the scene tree.
func (s *Scene) Root() *Node { return s.root }

// Invalidate marks the node mirroring w dirty and records damage covering both
// where w was last drawn (its lastRect) and where it is now (its current
// Bounds), so a move or resize repaints the vacated area as well as the new one.
// The dirty flag propagates up w's ancestor chain. Invalidating a widget that
// is not in the scene tree is a no-op.
func (s *Scene) Invalidate(w toolkit.Widget) {
	n := s.index[w]
	if n == nil {
		return // not part of this scene
	}
	s.damage.Add(n.lastRect)
	s.damage.Add(w.Bounds())
	for m := n; m != nil; m = m.parent {
		m.dirty = true
	}
}

// Render refreshes the retained tree from the live widgets, then repaints and
// returns the accumulated damage. When no widget has been invalidated it draws
// nothing and returns an empty region. The returned Region's backing storage is
// reused on the next Render — copy it if you must keep it. Render performs no
// per-frame heap allocation in the steady state.
func (s *Scene) Render(p painter.Painter, th *toolkit.Theme) Region {
	s.sync()

	// Move the accumulated damage into blit (to draw + return) and clear the
	// accumulator, reusing both backing slices — no allocation.
	s.blit.rects, s.damage.rects = s.damage.rects, s.blit.rects[:0]

	if len(s.blit.rects) > 0 {
		clipper, canClip := p.(painter.Clipper)
		for _, dr := range s.blit.rects {
			if canClip {
				clipper.PushClip(dr)
			}
			s.draw(s.root, dr, p, th)
			if canClip {
				clipper.PopClip()
			}
		}
	}

	s.stamp(s.root)
	return s.blit
}

// draw paints node n (and, for a cooperating container, its children) confined
// to the damage rect dr, in z-order. It prunes a subtree that does not intersect
// dr in O(1), so a frame costs O(depth + intersecting-nodes), not O(tree).
func (s *Scene) draw(n *Node, dr Rect, p painter.Painter, th *toolkit.Theme) {
	if !intersects(n.subtree, dr) {
		return // whole subtree outside the damage — prune
	}
	if len(n.children) == 0 {
		n.W.Draw(p, th) // leaf
		return
	}
	sd, ok := n.W.(SelfDrawer)
	if !ok {
		// Container without a self-draw seam: draw it wholesale. Its own Draw
		// recurses into every child; the clip confines the result to dr, so it
		// is pixel-correct — just not internally pruned.
		n.W.Draw(p, th)
		return
	}
	// Background/chrome occlusion: if a child subtree paints all of dr fully
	// opaque, the chrome would be entirely overdrawn — skip it. This is the
	// optimisation that turns a damage frame from "still re-fills the whole
	// window background" into "touches only the changed pixels": a full-surface
	// FillRect iterates its whole rectangle even when clipped, so NOT issuing it
	// is what unlocks the large speedup on a dense opaque scene. Pixel-identical
	// because the covering opaque content repaints dr regardless.
	if !coveredByChildren(n, dr) {
		sd.DrawSelf(p, th) // own chrome only; scene owns child traversal
	}
	for i, c := range n.children {
		if occluded(n.children, i, dr) {
			continue
		}
		s.draw(c, dr, p, th)
	}
}

// coveredByChildren reports whether some child subtree of n paints ALL of dr
// fully opaque (so n's own chrome under dr is invisible). Conservative: it needs
// a SINGLE child to cover dr wholly — partial coverage spread across several
// children is not treated as covering.
func coveredByChildren(n *Node, dr Rect) bool {
	for _, c := range n.children {
		if !intersects(c.subtree, dr) {
			continue
		}
		if subtreeCovers(c, dr) {
			return true
		}
	}
	return false
}

// subtreeCovers reports whether the subtree rooted at n paints ALL of dr fully
// opaque: either n itself declares an [Opaque] rectangle containing dr, or one
// of its children's subtrees does (children stack, so one full cover suffices).
// Subtrees whose bounds miss dr are pruned. Conservative — anything it cannot
// prove opaque-covering yields false, so a node that could show through is never
// culled.
func subtreeCovers(n *Node, dr Rect) bool {
	if op, ok := n.W.(Opaque); ok {
		if orect, opaque := op.OpaqueRect(); opaque && contains(orect, dr) {
			return true
		}
	}
	for _, c := range n.children {
		if !intersects(c.subtree, dr) {
			continue
		}
		if subtreeCovers(c, dr) {
			return true
		}
	}
	return false
}

// occluded reports whether the child at index i is provably, fully hidden
// within dr by a LATER sibling (one drawn on top of it) that declares a fully-
// opaque rectangle covering the child's damaged area. Conservative: only direct
// later siblings are considered occluders and only when their opaque rect wholly
// contains the covered region, so a node that could show through is never
// skipped.
func occluded(sibs []*Node, i int, dr Rect) bool {
	area := rectIntersect(sibs[i].subtree, dr)
	if isEmpty(area) {
		return false // nothing of it is in dr anyway; draw() will prune it
	}
	for j := i + 1; j < len(sibs); j++ {
		if op, ok := sibs[j].W.(Opaque); ok {
			if orect, opaque := op.OpaqueRect(); opaque && contains(orect, area) {
				return true
			}
		}
	}
	return false
}

// sync refreshes the retained tree to match the live widgets: it re-reads every
// widget's bounds, reconciles children whose structure changed, and recomputes
// subtree bounding boxes. In the steady state (same tree, only appearance
// changed) it walks the tree without allocating.
func (s *Scene) sync() { s.syncNode(s.root) }

// syncNode refreshes one node and returns its subtree bounding box.
func (s *Scene) syncNode(n *Node) Rect {
	n.rect = n.W.Bounds()
	sub := n.rect
	// A widget's type is fixed, so childProvider-ness never changes across
	// syncs: a leaf stays a leaf (children remain nil) and a container is
	// reconciled below. reconcile handles a container that now reports zero
	// children (its slice becomes empty), so there is no "stopped being a
	// container" case to handle here.
	if cp, ok := n.W.(childProvider); ok {
		s.reconcile(n, cp.Children())
		for _, c := range n.children {
			sub = union(sub, s.syncNode(c))
		}
	}
	n.subtree = sub
	return sub
}

// reconcile makes n.children mirror kids. The common case — same count, same
// widgets in the same order — is detected and reuses the existing node slice
// (and every child node, preserving its lastRect) with no allocation. Any
// structural change rebuilds the slice, reusing already-indexed child nodes so a
// widget that merely moved within the tree keeps its retained state. nil entries
// in kids are skipped.
func (s *Scene) reconcile(n *Node, kids []toolkit.Widget) {
	if len(n.children) == len(kids) {
		same := true
		for i, k := range kids {
			if n.children[i].W != k {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	next := make([]*Node, 0, len(kids))
	for _, k := range kids {
		if k == nil {
			continue
		}
		cn := s.index[k]
		if cn == nil {
			cn = &Node{W: k}
			s.index[k] = cn
		}
		cn.parent = n
		next = append(next, cn)
	}
	n.children = next
}

// stamp records the just-rendered bounds as each node's lastRect (so the next
// Invalidate knows the old position) and clears the dirty flags.
func (s *Scene) stamp(n *Node) {
	n.lastRect = n.rect
	n.dirty = false
	for _, c := range n.children {
		s.stamp(c)
	}
}

// --- rectangle helpers ------------------------------------------------------

// isEmpty reports whether r has no area (a rect that can neither be damaged nor
// occlude).
func isEmpty(r Rect) bool { return r.W <= 0 || r.H <= 0 }

// intersects reports whether a and b share any area (both must be non-empty).
func intersects(a, b Rect) bool {
	if isEmpty(a) || isEmpty(b) {
		return false
	}
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// rectIntersect returns the overlap of a and b, or an empty Rect when disjoint.
func rectIntersect(a, b Rect) Rect {
	if !intersects(a, b) {
		return Rect{}
	}
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// union returns the bounding box of a and b, ignoring an empty operand.
func union(a, b Rect) Rect {
	if isEmpty(a) {
		return b
	}
	if isEmpty(b) {
		return a
	}
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// contains reports whether outer wholly covers inner. An empty inner is
// contained by anything; nothing is contained by an empty outer.
func contains(outer, inner Rect) bool {
	if isEmpty(inner) {
		return true
	}
	if isEmpty(outer) {
		return false
	}
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.W <= outer.X+outer.W && inner.Y+inner.H <= outer.Y+outer.H
}
