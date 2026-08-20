// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// MomentumScroller drives a [ScrollView] from a host that routes touch itself,
// with its own clock.
//
// A ScrollView now drives the same [Momentum] engines on its own — see
// ScrollView.Tick — so nothing has to be wrapped to get touch scrolling,
// rubber-band overscroll and spring-back. This type remains for the host that
// wants to own the timing, feed its own dt, or retune an axis: wrapping claims
// the view (see [ScrollView.SetScrollDriver]) so the two never both move it.
//
// It is deliberately ADDITIVE: a ScrollView that is never wrapped keeps its own
// behaviour — wheel, keys, scrollbar
// drag and content pan all run through ScrollView.OnEvent as before — because
// this type modifies no ScrollView code and only ever runs when a host routes
// touch events to it explicitly. That is the gate: default (non-touch) scrolling
// is unchanged because nothing on the default path was touched.
//
// It composes two engines, one per axis (exactly as ScrollView keeps OffsetX
// and OffsetY as independent clamps), reads the ScrollView's own geometry for
// the bounds so nothing is duplicated, and writes the resulting offsets back to
// the ScrollView's exported OffsetX / OffsetY — writing them directly (not via
// Scroll) so an overscrolled offset can sit briefly past a bound for the
// rubber-band, which Scroll would otherwise clamp away.
//
// A host wires it up as: TouchDown on press, TouchMove on each drag sample,
// TouchUp on release, then Tick(dt) each frame while Settling reports true.
type MomentumScroller struct {
	// View is the wrapped scroll view. Its OffsetX / OffsetY are driven by the
	// two axis engines.
	View *ScrollView
	// AxisX, AxisY are the per-axis inertial engines. A caller may retune either
	// (friction, bounce, spring) or disable one by leaving Bounce off and its
	// motion clamped; both are live by default.
	AxisX, AxisY *Momentum

	trackX, trackY VelocityTracker
	lastX, lastY   int
	dragging       bool
}

// NewMomentumScroller wraps v with a default-tuned inertial engine on each axis.
func NewMomentumScroller(v *ScrollView) *MomentumScroller {
	return &MomentumScroller{
		View:  v,
		AxisX: NewMomentum(),
		AxisY: NewMomentum(),
	}
}

// syncBounds refreshes each axis's clamp window from the ScrollView's current
// geometry: 0..(content − viewport) on each axis, floored at 0 when the content
// is not larger than the viewport (nothing to scroll). It reads the view's own
// viewport() and content extent, so the engine never re-derives the geometry.
func (s *MomentumScroller) syncBounds() {
	vp := s.View.viewport()
	maxX := s.View.contentW - vp.W
	if maxX < 0 {
		maxX = 0
	}
	maxY := s.View.contentH - vp.H
	if maxY < 0 {
		maxY = 0
	}
	s.AxisX.SetBounds(0, float64(maxX))
	s.AxisY.SetBounds(0, float64(maxY))
}

// TouchDown starts a touch drag at ev's position. It stops any coast in
// progress, syncs the bounds and seeds each engine from the view's current
// offset, so the drag tracks from exactly where the content sits.
func (s *MomentumScroller) TouchDown(ev Event) {
	// Claim the view on the FIRST gesture, not at construction: a wrapper that
	// is never used must leave the view exactly as it was. From here on the
	// view stops panning and flinging itself, because a host delivers this same
	// drag to the widget tree as well -- buttons and lists need it -- and the
	// gesture would otherwise land twice, scrolling at double speed.
	s.View.SetScrollDriver(s)
	s.syncBounds()
	s.AxisX.SetOffset(float64(s.View.OffsetX().Get()))
	s.AxisY.SetOffset(float64(s.View.OffsetY().Get()))
	s.AxisX.BeginDrag()
	s.AxisY.BeginDrag()
	s.trackX.Reset()
	s.trackY.Reset()
	s.lastX, s.lastY = ev.X, ev.Y
	s.dragging = true
}

// TouchMove feeds one drag sample taken dt seconds after the previous one. The
// content follows the finger — moving the finger up (decreasing Y) scrolls
// toward the end, an increasing offset — so the per-axis delta is the finger's
// movement negated, exactly the convention ScrollView's content pan uses. It is
// a no-op until a TouchDown has armed the drag.
func (s *MomentumScroller) TouchMove(ev Event, dt float64) {
	if !s.dragging {
		return
	}
	dx := float64(s.lastX - ev.X)
	dy := float64(s.lastY - ev.Y)
	s.lastX, s.lastY = ev.X, ev.Y
	s.AxisX.DragBy(dx)
	s.AxisY.DragBy(dy)
	s.trackX.Sample(dx, dt)
	s.trackY.Sample(dy, dt)
	s.pushToView()
}

// TouchUp releases the drag, launching each axis into a coast at the velocity
// its tracker smoothed from the recent samples (a release while overscrolled
// springs home regardless). Harmless if no drag was active.
func (s *MomentumScroller) TouchUp() {
	s.dragging = false
	s.AxisX.EndDrag(s.trackX.Velocity())
	s.AxisY.EndDrag(s.trackY.Velocity())
}

// Tick advances both axes by dt seconds, writes the new offsets to the view,
// and reports whether either axis is still settling — the host schedules
// another frame while this is true.
func (s *MomentumScroller) Tick(dt float64) bool {
	bx := s.AxisX.Tick(dt)
	by := s.AxisY.Tick(dt)
	s.pushToView()
	return bx || by
}

// Settling reports whether either axis still owes motion.
func (s *MomentumScroller) Settling() bool {
	return s.AxisX.Settling() || s.AxisY.Settling()
}

// pushToView writes the engines' current (rounded) offsets to the ScrollView's
// exported offsets. It writes them directly rather than through Scroll so a
// live overscroll can show past the bound; the engines never rest past a bound,
// so the resting offset is always in range.
func (s *MomentumScroller) pushToView() {
	s.View.OffsetX().Set(s.AxisX.OffsetInt())
	s.View.OffsetY().Set(s.AxisY.OffsetInt())
}
