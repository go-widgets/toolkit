// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// scrollbarMinThumb is the smallest thumb length (px) so it stays grabbable even
// when the content dwarfs the viewport.
const scrollbarMinThumb = 24

// Scrollbar is a slim, grabbable scrollbar for scrollable content: a rounded
// track with a thumb sized to Viewport/Total and positioned by Offset, showing
// where the view sits within the whole. Vertical by default; set Horizontal for
// a bottom scrollbar. When everything fits (Total <= Viewport) the thumb fills
// the track.
//
// The thumb is a live affordance, not merely an indicator: dragging it, or
// clicking the track above/below it, moves Offset and fires OnScroll. Both the
// paint (ThumbRect) and the interaction (OnEvent) read one shared geometry
// (geom), the same sbGeom the embedded ScrollView/Table scrollbars use, so the
// drawn thumb and the drag target can never drift apart. A host that only wants
// a read-only indicator simply leaves OnScroll nil and never routes events to
// it.
type Scrollbar struct {
	Base
	Total      int                   // total content length along the scroll axis
	Viewport   int                   // visible length
	offset     *mvvm.Observable[int] // scroll offset, reactive via Offset()
	Horizontal bool                  // false = vertical (the default for a scrollbar)

	// drag carries the in-progress thumb grab (see scrollDrag), shared with the
	// same press/drag/release policy the embedded scrollbars use.
	drag scrollDrag
}

// Offset is the scroll offset as a shared [mvvm.Observable]; a drag/click Sets it
// (subscribers replace the old OnScroll callback). Lazily created.
func (s *Scrollbar) Offset() *mvvm.Observable[int] {
	if s.offset == nil {
		s.offset = mvvm.NewObservable(0)
	}
	return s.offset
}

// NewScrollbar builds an empty vertical scrollbar.
func NewScrollbar() *Scrollbar { return &Scrollbar{} }

// geom builds the widget-local track + thumb geometry for the current
// Total/Viewport/Offset as an sbGeom -- the same structure the embedded
// ScrollView/Table scrollbars use -- so ThumbRect (paint) and OnEvent
// (hit-test + drag) share one source of truth. ok is false when the widget has
// no area. The thumb is sized to view/total (floored at scrollbarMinThumb,
// capped at the track), and its leading edge is (length-thumb)*off/maxOff.
func (s *Scrollbar) geom() (sbGeom, bool) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return sbGeom{}, false
	}
	total, view := s.Total, s.Viewport
	if total < 1 {
		total = 1
	}
	if view < 1 {
		view = 1
	}
	if view > total {
		view = total
	}
	maxOff := total - view
	off := s.Offset().Get()
	if off < 0 {
		off = 0
	}
	if off > maxOff {
		off = maxOff
	}
	length := r.H
	if s.Horizontal {
		length = r.W
	}
	// The grabbable minimum thumb routes through scaled so it grows with HiDPI and
	// touch density, and is additionally floored at the density hit-target so a
	// dragged thumb is never smaller than a fingertip under DensityTouch. At
	// compact/1x scaled(scrollbarMinThumb) == scrollbarMinThumb and MinHitTarget is
	// 0, so the floor is byte-identical to the historical 24px minimum. A track
	// shorter than the minimum still clamps the thumb back down to the track.
	minThumb := scaled(scrollbarMinThumb)
	if m := MinHitTarget(); m > minThumb {
		minThumb = m
	}
	thumb := length * view / total
	if thumb < minThumb {
		thumb = minThumb
	}
	if thumb > length {
		thumb = length
	}
	// No travel when the whole track is one page (maxOff == 0): pin the thumb to
	// the start and give scrollForGrabStart a safe (0-travel) denominator.
	travelNum, travelDen, thumbStart := 0, 1, 0
	if maxOff > 0 {
		travelNum, travelDen = length-thumb, maxOff
		thumbStart = travelNum * off / travelDen
	}
	return sbGeom{
		horizontal: s.Horizontal,
		cross0:     0,
		trackStart: 0, trackLen: length,
		thumbStart: thumbStart, thumbLen: thumb,
		travelNum: travelNum, travelDen: travelDen,
		maxScroll: maxOff,
	}, true
}

// ThumbRect returns the thumb's rectangle for the current Total/Viewport/Offset,
// so callers can hit-test or animate it. It is empty when the widget has no area.
func (s *Scrollbar) ThumbRect() Rect {
	g, ok := s.geom()
	if !ok {
		return Rect{}
	}
	r := s.Bounds()
	if s.Horizontal {
		return Rect{X: r.X + g.thumbStart, Y: r.Y, W: g.thumbLen, H: r.H}
	}
	return Rect{X: r.X, Y: r.Y + g.thumbStart, W: r.W, H: g.thumbLen}
}

// Draw paints the track and the thumb. The thumb is drawn in Theme.Border so it
// reads against the SurfaceAlt track in both light and dark themes.
func (s *Scrollbar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	paintScrollTrack(p, theme, r.X, r.Y, r.W, r.H)
	t := s.ThumbRect()
	paintScrollThumb(p, theme, t.X, t.Y, t.W, t.H)
}

// OnEvent makes the thumb grabbable: an EventClick on the thumb starts a drag
// (recording the grab offset); an EventClick on the track above/below the thumb
// pages one viewport toward the click; an EventMouseDrag maps the pointer to a
// clamped Offset while the grab is active; EventMouseUp releases it. All of it
// runs through the shared scrollDrag policy against geom(), so a standalone
// Scrollbar drags exactly like an embedded one. A Disabled scrollbar ignores
// input. Coordinates are widget-local.
func (s *Scrollbar) OnEvent(ev Event) {
	if s.Disabled().Get() {
		return
	}
	g, ok := s.geom()
	switch ev.Kind {
	case EventClick:
		s.drag.press(g, ok, ev, s.Viewport, s.scrollBy)
	case EventMouseDrag:
		s.drag.drag(g, ok, ev, s.scrollTo)
	case EventMouseUp:
		s.drag.release()
	}
}

// scrollBy nudges Offset by delta (a page step from a track click).
func (s *Scrollbar) scrollBy(delta int) { s.setOffset(s.Offset().Get() + delta) }

// scrollTo sets Offset to off (a thumb-drag target; already clamped by
// scrollForGrabStart, re-clamped here for safety).
func (s *Scrollbar) scrollTo(off int) { s.setOffset(off) }

// setOffset clamps off to [0, Total-Viewport], assigns it, and fires OnScroll
// only when the value actually changed -- so a drag that stays pinned at an end
// doesn't emit a stream of no-op callbacks.
func (s *Scrollbar) setOffset(off int) {
	maxOff := s.Total - s.Viewport
	if maxOff < 0 {
		maxOff = 0
	}
	if off < 0 {
		off = 0
	}
	if off > maxOff {
		off = maxOff
	}
	if off == s.Offset().Get() {
		return
	}
	s.Offset().Set(off)
}

// scrollbarThumbMix is how far the thumb colour sits from the track toward the
// foreground: 0 = track colour, 1 = full OnSurface. ~0.45 reads as a clear
// medium grey on both light and dark themes.
const scrollbarThumbMix = 0.45

// scrollbarThumbColor is the muted grey every scrollbar thumb uses: OnSurface
// blended toward the SurfaceAlt track. It stays clearly visible even when the
// surrounding surface is itself SurfaceAlt (e.g. a sidebar), where the faint
// Border would vanish.
func scrollbarThumbColor(theme *Theme) RGBA {
	return blendRGBA(theme.OnSurface, theme.SurfaceAlt, scrollbarThumbMix)
}

// scrollbarRadius rounds a scrollbar track/thumb by half its short side, so its
// ends are fully rounded like a capsule regardless of orientation.
func scrollbarRadius(w, h int) int {
	if h < w {
		return h / 2
	}
	return w / 2
}

// paintScrollTrack and paintScrollThumb are the ONE house style for a scrollbar:
// a rounded SurfaceAlt track under a rounded muted-grey thumb. Every embedded
// scrollbar — ScrollView, List, TreeView, Table, Browser — and the standalone
// Scrollbar draw through these, so a scrollbar looks identical wherever it
// appears. Each caller keeps its own geometry (track thickness, position, the
// viewport/content proportion that sizes the thumb); only the fill is shared.
func paintScrollTrack(p painter.Painter, theme *Theme, x, y, w, h int) {
	fillRoundRect(p, x, y, w, h, scrollbarRadius(w, h), theme.SurfaceAlt)
}

func paintScrollThumb(p painter.Painter, theme *Theme, x, y, w, h int) {
	fillRoundRect(p, x, y, w, h, scrollbarRadius(w, h), scrollbarThumbColor(theme))
}

// blendRGBA mixes a over b at t in [0,1] (t=1 yields a, t=0 yields b), producing
// an opaque colour.
func blendRGBA(a, b RGBA, t float64) RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*t + float64(y)*(1-t) + 0.5) }
	return RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}
