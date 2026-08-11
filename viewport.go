// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ViewportRegion names one of the five slots a Viewport fills: the four
// dockable edges plus the centre that takes whatever is left.
type ViewportRegion int

const (
	// ViewportCenter is the flexible middle slot. It always fills the space the
	// edges leave behind, so it has no size of its own.
	ViewportCenter ViewportRegion = iota
	// ViewportTop docks a bar across the full width at the top, given height.
	ViewportTop
	// ViewportBottom docks a bar across the full width at the bottom, given height.
	ViewportBottom
	// ViewportLeft docks a bar down the left, given width, between the top and
	// bottom bars.
	ViewportLeft
	// ViewportRight docks a bar down the right, given width, between the top and
	// bottom bars.
	ViewportRight

	// regionCount is the exclusive upper bound of the region values, used to
	// size the per-region arrays and to bound-check Set/RegionRect.
	regionCount
)

// Viewport is the application root: a single widget that fills the whole
// surface a window hands it and parcels that surface out to five slots — a top
// bar, a bottom bar, a left bar, a right bar, and a centre that fills the rest.
// It is what a native window or a wasmbox client sets as its root and resizes to
// the drawable area, so the shell re-fills to the window on every resize.
//
// The edges are carved in a FIXED precedence — top, then bottom, then left, then
// right — so the top and bottom bars span the full width and the side bars only
// take the height that remains between them (the five-region shell shape). This
// makes the layout independent of the order regions are assigned (unlike Dock,
// which carves in insertion order); the centre always fills whatever is left,
// even when it holds no widget.
//
// Viewport is a Widget: SetBounds re-parcels every slot, Draw paints the centre
// then the edges, and OnEvent routes to the slot under the pointer, translating
// into that slot's local space. Any slot may be nil (that region simply
// contributes no bar and its space folds into the centre).
type Viewport struct {
	Base
	// region holds the widget in each slot (nil = empty); size holds each edge's
	// extent along its own axis (height for top/bottom, width for left/right).
	// ViewportCenter's size entry is unused. Kept as fixed-length arrays so layout
	// order is deterministic and no map iteration leaks into the geometry.
	region [regionCount]Widget
	size   [regionCount]int
	// rect caches each slot's laid-out rectangle in surface coordinates, filled
	// by SetBounds and read back by RegionRect (and by Draw / OnEvent).
	rect [regionCount]Rect
}

// A11y marks the Viewport as a presentational shell: it holds no content of its
// own, so a screen reader announces its docked panels and its centre directly
// rather than the container. CollectA11y skips a RolePresentation node but still
// descends into its Children — the same treatment as Container, Border and Dock.
func (v *Viewport) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// NewViewport builds an empty Viewport. Assign slots with Set before (or after)
// the first SetBounds; an unset Viewport lays out a single full-surface centre.
func NewViewport() *Viewport { return &Viewport{} }

// Set places w in the given region with size pixels along that edge's axis
// (height for top/bottom, width for left/right; ignored for ViewportCenter, which
// always fills). A negative size clamps to 0; a nil w clears the slot. An
// out-of-range region is ignored. Re-lays out immediately so RegionRect is
// current.
func (v *Viewport) Set(region ViewportRegion, w Widget, size int) {
	if region < 0 || region >= regionCount {
		return
	}
	if size < 0 {
		size = 0
	}
	v.region[region] = w
	v.size[region] = size
	v.SetBounds(v.Bounds())
}

// carveOrder is the fixed precedence the edges are carved in: top and bottom
// first (full width), then left and right (between them). ViewportCenter is not
// carved — it takes the remainder.
var carveOrder = [...]struct {
	region ViewportRegion
	side   DockSide
}{
	{ViewportTop, DockTop},
	{ViewportBottom, DockBottom},
	{ViewportLeft, DockLeft},
	{ViewportRight, DockRight},
}

// SetBounds fills the surface: it carves each present edge off the available
// rectangle in the fixed precedence order, then gives the centre whatever
// remains. Every slot's rectangle is cached for RegionRect, and an absent edge
// gets a zero rect so a stale bar cannot linger.
func (v *Viewport) SetBounds(r Rect) {
	v.Base.SetBounds(r)
	avail := r
	for _, e := range carveOrder {
		if v.region[e.region] == nil {
			v.rect[e.region] = Rect{}
			continue
		}
		var bar Rect
		bar, avail = dockCarve(e.side, v.size[e.region], avail)
		v.rect[e.region] = bar
		v.region[e.region].SetBounds(bar)
	}
	v.rect[ViewportCenter] = avail
	if v.region[ViewportCenter] != nil {
		v.region[ViewportCenter].SetBounds(avail)
	}
}

// RegionRect reports the surface-space rectangle currently allotted to a region
// (the zero Rect for an out-of-range region, or for an edge with no widget). The
// centre's rectangle is always the remainder, whether or not it holds a widget.
func (v *Viewport) RegionRect(region ViewportRegion) Rect {
	if region < 0 || region >= regionCount {
		return Rect{}
	}
	return v.rect[region]
}

// Draw paints the centre first, then the edge bars over it (they never overlap,
// so the order only decides which wins a shared seam).
func (v *Viewport) Draw(p painter.Painter, theme *Theme) {
	if w := v.region[ViewportCenter]; w != nil {
		w.Draw(p, theme)
	}
	for _, e := range carveOrder {
		if w := v.region[e.region]; w != nil {
			w.Draw(p, theme)
		}
	}
}

// Children yields the present slots in reading order: the edges clockwise from
// the top, then the centre — the order a screen reader announces a shell in, so
// a generic tree walk reaches every docked panel and the content area.
func (v *Viewport) Children() []Widget {
	order := [...]ViewportRegion{ViewportTop, ViewportRight, ViewportBottom, ViewportLeft, ViewportCenter}
	out := make([]Widget, 0, regionCount)
	for _, region := range order {
		if w := v.region[region]; w != nil {
			out = append(out, w)
		}
	}
	return out
}

// OnEvent forwards to the first slot whose Bounds contains the surface point,
// translated into that slot's local space. The edges are tested before the
// centre so a bar wins any seam it shares with the content area.
func (v *Viewport) OnEvent(ev Event) {
	pr := v.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, e := range carveOrder {
		w := v.region[e.region]
		if w != nil && w.Bounds().Contains(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
	if w := v.region[ViewportCenter]; w != nil && w.Bounds().Contains(sx, sy) {
		w.OnEvent(translateEvent(ev, pr, w.Bounds()))
	}
}
