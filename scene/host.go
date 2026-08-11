// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package scene

import (
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// HostRoot adapts an immediate-mode widget tree to a windowing host's
// damage-aware present path. It is a plain [toolkit.Widget] — so it can be
// handed to ANY host, including one with no damage support, which simply calls
// Draw for a full-surface repaint — and it ADDITIONALLY reports the exact
// per-frame damage region through [HostRoot.RenderDamaged].
//
// A host (github.com/go-widgets/window) type-asserts the root it is given for
// the structural capability
//
//	interface {
//		RenderDamaged(p painter.Painter, th *toolkit.Theme) []toolkit.Rect
//	}
//
// When the assertion holds it draws the frame through RenderDamaged and
// packs+presents ONLY the returned rectangles' union; a plain widget keeps the
// full-surface path. HostRoot is that capability's reference implementation.
//
// HostRoot owns a [Scene] mirroring child and fills the window background — from
// the theme handed to each frame — as scene chrome, so an incremental frame
// repaints a widget's VACATED background exactly as a full immediate-mode frame
// (background fill, then the tree) would. The two are pixel-identical by
// construction (see the package's headline correctness guarantee), which is why
// a host may switch to the incremental path without any risk of dropping a
// pixel.
//
// Usage: build the tree, wrap the root, hand it to the host, and call
// [HostRoot.Invalidate] whenever a widget's appearance changes:
//
//	root := scene.NewHostRoot(appRoot)
//	// in an event handler, after mutating w's appearance:
//	root.Invalidate(w)
//	// ...
//	backend.Run(root) // window.Backend, drives RenderDamaged per frame
type HostRoot struct {
	bg    *bgContainer
	scene *Scene
}

// NewHostRoot wraps child (the application's root widget) so it can drive a
// host's damage-aware present path. child must be non-nil. The returned root is
// not yet laid out; the host calls SetBounds with the client area before the
// first frame (which seeds a full-surface first paint).
func NewHostRoot(child toolkit.Widget) *HostRoot {
	bg := &bgContainer{child: child}
	return &HostRoot{bg: bg, scene: New(bg)}
}

// Bounds returns the root's placement (the whole client area once laid out).
func (r *HostRoot) Bounds() toolkit.Rect { return r.bg.Bounds() }

// SetBounds lays the tree out to b. A size/position change damages the whole
// old and new area (so a resize repaints correctly through the SAME incremental
// path — RenderDamaged then returns full-surface damage and repaints the fresh
// framebuffer completely); an unchanged bounds is a cheap no-op that neither
// re-lays-out the child nor invalidates.
func (r *HostRoot) SetBounds(b toolkit.Rect) {
	if b == r.bg.Bounds() {
		return
	}
	r.bg.SetBounds(b)
	r.scene.Invalidate(r.bg)
}

// Draw paints the whole tree (background then children) — the full-surface path
// a damage-unaware host uses. It makes HostRoot a drop-in [toolkit.Widget].
func (r *HostRoot) Draw(p painter.Painter, th *toolkit.Theme) { r.bg.Draw(p, th) }

// HitTest reports whether (px, py) falls on the wrapped tree.
func (r *HostRoot) HitTest(px, py int) bool { return r.bg.HitTest(px, py) }

// OnEvent delivers an event to the wrapped tree. The application's own handlers
// are responsible for calling [HostRoot.Invalidate] on any widget whose
// appearance the event changed.
func (r *HostRoot) OnEvent(ev toolkit.Event) { r.bg.OnEvent(ev) }

// RenderDamaged paints this frame into p (confined, per damaged rectangle, to
// the coalesced damage via the painter's clip seam) and returns the rectangles
// it repainted, in surface coordinates. The host packs+presents exactly their
// union. An empty result means nothing changed this frame (the host presents
// nothing). The returned slice is owned by the underlying [Scene] and is reused
// on the next call — the host must consume it before the next frame (it does:
// it presents the rects immediately).
func (r *HostRoot) RenderDamaged(p painter.Painter, th *toolkit.Theme) []toolkit.Rect {
	reg := r.scene.Render(p, th)
	return reg.Rects()
}

// Invalidate records that widget w's appearance changed, damaging both where it
// was and where it now is. Call it from event handlers between frames.
// Invalidating a widget not in the tree is a no-op.
func (r *HostRoot) Invalidate(w toolkit.Widget) { r.scene.Invalidate(w) }

// Scene returns the underlying retained scene, for introspection or advanced
// invalidation. Most callers only need [HostRoot.Invalidate].
func (r *HostRoot) Scene() *Scene { return r.scene }

// Children exposes the single wrapped child container so a generic tree walk
// descends from the host root INTO the application tree instead of stopping at
// it. Every other toolkit container answers the Children() []toolkit.Widget
// convention; HostRoot did not, so a walker built on it — the accessibility
// walk [github.com/go-widgets/toolkit.WalkA11y] the platform bridges consume,
// the window drag-and-drop controller, [CollectRuns] — reached nothing under a
// desktop app's root and consumers had to unwrap via Scene().Root().Widget()
// by hand.
//
// The returned widget is exactly that: the live container Scene().Root().Widget()
// exposes, which itself yields the application's root. This is purely structural
// — it exposes the existing child for traversal and changes neither rendering
// nor damage.
func (r *HostRoot) Children() []toolkit.Widget { return []toolkit.Widget{r.bg} }

// bgContainer is the scene root HostRoot builds over the application tree: a
// single-child container whose own chrome is a full-bounds background fill. As
// a [SelfDrawer] the scene repaints just that fill inside the damage (and skips
// it entirely when an opaque child already covers the damaged area), and as a
// childProvider the scene descends into — and prunes — the application subtree.
// This is what makes an incremental frame paint "background where a widget left,
// widget pixels where it is", pixel-identical to fill-then-draw.
type bgContainer struct {
	child  toolkit.Widget
	bounds toolkit.Rect
}

// Bounds returns the container's placement.
func (b *bgContainer) Bounds() toolkit.Rect { return b.bounds }

// SetBounds places the container and lays the child out to the same rectangle
// (the child fills the client area, exactly as a host's full-surface draw lays
// its root out to fill the window).
func (b *bgContainer) SetBounds(r toolkit.Rect) {
	b.bounds = r
	b.child.SetBounds(r)
}

// Draw fills the background then draws the child — the wholesale full-surface
// paint used off the damage path.
func (b *bgContainer) Draw(p painter.Painter, th *toolkit.Theme) {
	p.FillRect(b.bounds, th.Background)
	b.child.Draw(p, th)
}

// DrawSelf fills only the background (the container's own chrome); the scene
// owns child traversal.
func (b *bgContainer) DrawSelf(p painter.Painter, th *toolkit.Theme) {
	p.FillRect(b.bounds, th.Background)
}

// HitTest defers to the child.
func (b *bgContainer) HitTest(px, py int) bool { return b.child.HitTest(px, py) }

// OnEvent forwards to the child.
func (b *bgContainer) OnEvent(ev toolkit.Event) { b.child.OnEvent(ev) }

// Children exposes the single application child for the scene's generic
// traversal.
func (b *bgContainer) Children() []toolkit.Widget { return []toolkit.Widget{b.child} }
