// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// sbGeom describes one scrollbar's track + thumb geometry along a single axis,
// in WIDGET-LOCAL pixel coordinates (origin at the widget's top-left). It is the
// single source of truth shared by a widget's drawScrollbar (which paints the
// thumb) and its OnEvent (which hit-tests + drags it), so the drawn thumb and the
// drag target can never drift apart.
//
// The scroll axis is called "along"; the perpendicular axis is "cross". For a
// vertical bar the along axis is Y and the cross axis is X; for a horizontal bar
// (only ScrollView's bottom bar) they swap. The thumb's leading edge is a linear
// function of the scroll value:
//
//	thumbStart = trackStart + scroll*travelNum/travelDen
//
// which drawScrollbar evaluates and scrollForGrabStart inverts to turn a grabbed
// pixel back into a clamped scroll value. cross0 is the fixed cross-axis start of
// the track column; the track is scrollbarWidth wide on the cross axis.
type sbGeom struct {
	horizontal           bool
	cross0               int // cross-axis start of the track column
	trackStart, trackLen int // along-axis track extent
	thumbStart, thumbLen int // along-axis thumb extent
	travelNum, travelDen int // thumbStart = trackStart + scroll*travelNum/travelDen
	maxScroll            int // largest legal scroll value; clamp bound
}

// along returns the along-axis component of a widget-local point.
func (g sbGeom) along(x, y int) int {
	if g.horizontal {
		return x
	}
	return y
}

// cross returns the cross-axis component of a widget-local point.
func (g sbGeom) cross(x, y int) int {
	if g.horizontal {
		return y
	}
	return x
}

// onTrack reports whether widget-local (x, y) falls anywhere in the scrollbar's
// track column.
func (g sbGeom) onTrack(x, y int) bool {
	c, a := g.cross(x, y), g.along(x, y)
	return c >= g.cross0 && c < g.cross0+scrollbarWidth &&
		a >= g.trackStart && a < g.trackStart+g.trackLen
}

// onThumb reports whether widget-local (x, y) falls on the thumb.
func (g sbGeom) onThumb(x, y int) bool {
	c, a := g.cross(x, y), g.along(x, y)
	return c >= g.cross0 && c < g.cross0+scrollbarWidth &&
		a >= g.thumbStart && a < g.thumbStart+g.thumbLen
}

// scrollForGrabStart inverts the thumbStart formula: it maps a desired thumb
// leading-edge pixel (widget-local, along the scroll axis) back to a scroll
// value, rounded to the nearest unit and clamped to [0, maxScroll]. A
// non-positive travel (a thumb that fills or exceeds the track) has nowhere to
// move, so it pins to 0.
func (g sbGeom) scrollForGrabStart(start int) int {
	if g.travelNum <= 0 {
		return 0
	}
	rel := start - g.trackStart
	sc := (rel*g.travelDen + g.travelNum/2) / g.travelNum
	if sc < 0 {
		return 0
	}
	if sc > g.maxScroll {
		return g.maxScroll
	}
	return sc
}

// scrollDrag tracks an in-progress scrollbar thumb drag for a scrollable widget.
// A widget embeds one per scrollbar, exposes its scrollbar geometry, and routes
// press/drag/release through it so the interaction policy lives in one place
// instead of being re-derived in each widget's OnEvent.
type scrollDrag struct {
	active bool // true between grabbing the thumb and releasing it
	grab   int  // along-axis pixels from the pointer to the thumb's leading edge
}

// press processes an EventClick against geometry g (ok reports whether the
// scrollbar is live). It returns true when the press landed on the scrollbar —
// so the caller must NOT also treat it as a content click. A press on the thumb
// begins a drag (recording the grab offset); a press on the track off the thumb
// pages one window toward the click via scrollBy. page is the page step in scroll
// units.
func (d *scrollDrag) press(g sbGeom, ok bool, ev Event, page int, scrollBy func(int)) bool {
	if !ok || !g.onTrack(ev.X, ev.Y) {
		return false
	}
	if g.onThumb(ev.X, ev.Y) {
		d.active = true
		d.grab = g.along(ev.X, ev.Y) - g.thumbStart
		return true
	}
	if g.along(ev.X, ev.Y) < g.thumbStart {
		scrollBy(-page)
	} else {
		scrollBy(page)
	}
	return true
}

// drag processes an EventMouseDrag while a thumb grab is active, mapping the
// pointer to a clamped scroll position via scrollTo. It is a no-op when no drag
// is active or the scrollbar is no longer live (ok is false).
func (d *scrollDrag) drag(g sbGeom, ok bool, ev Event, scrollTo func(int)) {
	if !d.active || !ok {
		return
	}
	scrollTo(g.scrollForGrabStart(g.along(ev.X, ev.Y) - d.grab))
}

// release ends any in-progress drag on EventMouseUp.
func (d *scrollDrag) release() { d.active = false }
