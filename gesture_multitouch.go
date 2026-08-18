// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "math"

// mtContact is one live touch contact tracked by a MultiTouchRecognizer. It
// is identified by its host-assigned Event.Code — the same stable, per-contact
// id that EventTouchStart / EventTouchMove / EventTouchEnd carry (see
// widget.go). x, y are the contact's most recent widget-local position.
type mtContact struct {
	id string
	x  int
	y  int
}

// MultiTouchState is the geometry of an engaged two-contact gesture, sampled
// relative to the moment the gesture engaged (its "begin"). Every field is a
// float64 so sub-pixel scale, radian angles and half-pixel centroids stay
// exact — the two integer contact positions are averaged, so a centroid can
// legitimately land on a .5.
//
// At begin, Scale is 1, Rotation is 0 and PanX/PanY are 0 by construction;
// they then track continuously as the two anchor contacts move.
type MultiTouchState struct {
	// Scale is the current distance between the two anchor contacts divided
	// by that distance at begin — a pinch factor: > 1 spreading apart, < 1
	// pinching together. When the two contacts engaged on the exact same
	// point (a zero initial span, from which no ratio is defined) Scale is
	// held at 1 for the life of the gesture.
	Scale float64
	// Rotation is the signed change, in radians, of the vector from the
	// first anchor to the second since begin, normalized to (-Pi, Pi]:
	// positive is the atan2 direction (counter-clockwise in a
	// y-down-is-positive screen it reads as clockwise).
	Rotation float64
	// CenterX, CenterY are the current centroid (midpoint) of the two anchor
	// contacts, in widget-local pixels.
	CenterX float64
	CenterY float64
	// PanX, PanY are the centroid's translation since begin: CenterX minus
	// the begin centroid, and likewise for Y — a two-finger pan.
	PanX float64
	PanY float64
	// Span is the current distance between the two anchor contacts.
	Span float64
}

// MultiTouchRecognizer turns a stream of EventTouchStart / EventTouchMove /
// EventTouchEnd events (see widget.go) carrying two or more concurrent
// contacts into the three canonical two-finger gestures — pinch (scale),
// rotate (angle) and two-finger pan (centroid translation). Like
// GestureRecognizer it is pure logic: it does not draw, hold a Widget
// reference or consult a clock, so any widget (or a host, ahead of dispatch)
// can embed one and feed it its full event stream unfiltered.
//
// Contacts are identified by Event.Code, exactly as the single-touch
// GestureRecognizer identifies its one pointer; a MultiTouchRecognizer simply
// keeps every live contact instead of one. Contacts are held in arrival
// order.
//
// Engagement. The gesture is disengaged while fewer than two contacts are
// down. The instant a second contact lands, the recognizer engages: it adopts
// the first two contacts (in arrival order) as its two "anchors", records
// their span, vector angle and centroid as the begin reference, and fires
// OnMultiBegin with a fresh state (Scale 1, Rotation 0, Pan 0). While engaged,
// every move of either anchor recomputes the state and fires OnPinch,
// OnRotate, OnPan and OnMultiUpdate (in that order). A move of any other
// (non-anchor) contact is tracked but changes nothing and fires nothing.
//
// Disengagement. When an anchor lifts, the gesture ends: OnMultiEnd fires with
// the last state. If two or more contacts are still down (e.g. a third finger
// was resting, or more than two were down), the recognizer immediately
// re-engages on the first two survivors and fires a new OnMultiBegin — so a
// finger leaving a three-finger gesture cleanly hands off to the remaining
// pair. Lifting a non-anchor contact leaves the gesture untouched.
//
// This is additive to and independent of GestureRecognizer: the single-touch
// tap / long-press / swipe path is unchanged. A host that wants both simply
// feeds the same events to both recognizers.
type MultiTouchRecognizer struct {
	// OnPinch fires on each anchor move while engaged with the current
	// Scale (see MultiTouchState.Scale).
	OnPinch func(scale float64)
	// OnRotate fires on each anchor move while engaged with the current
	// Rotation in radians (see MultiTouchState.Rotation).
	OnRotate func(radians float64)
	// OnPan fires on each anchor move while engaged with the centroid
	// translation since begin (dx, dy).
	OnPan func(dx, dy float64)

	// OnMultiBegin fires when the gesture engages (a second contact lands,
	// or the survivors of an anchor lift re-engage). The passed state is the
	// begin reference: Scale 1, Rotation 0, Pan 0.
	OnMultiBegin func(m MultiTouchState)
	// OnMultiUpdate fires after OnPinch/OnRotate/OnPan on each anchor move,
	// carrying the full current state.
	OnMultiUpdate func(m MultiTouchState)
	// OnMultiEnd fires when an anchor lifts, carrying the last state before
	// the gesture ended.
	OnMultiEnd func(m MultiTouchState)

	contacts  []mtContact
	engaged   bool
	anchorA   string
	anchorB   string
	initSpan  float64
	initAngle float64
	initCX    float64
	initCY    float64
	state     MultiTouchState
}

// NewMultiTouchRecognizer returns a ready-to-use MultiTouchRecognizer. It has
// no thresholds to configure — every anchor move reports the exact geometry —
// so the zero value works identically; the constructor exists only for parity
// with NewGestureRecognizer and for callers who prefer it.
func NewMultiTouchRecognizer() *MultiTouchRecognizer {
	return &MultiTouchRecognizer{}
}

// Engaged reports whether a two-contact gesture is currently in progress
// (between an OnMultiBegin and its OnMultiEnd).
func (g *MultiTouchRecognizer) Engaged() bool { return g.engaged }

// State returns the current gesture geometry. It is meaningful while Engaged
// is true and, after an OnMultiEnd, holds the last state the gesture reached.
func (g *MultiTouchRecognizer) State() MultiTouchState { return g.state }

// Feed consumes one input event. Only EventTouchStart, EventTouchMove and
// EventTouchEnd are meaningful; every other kind is ignored so a host can feed
// its full event stream unfiltered.
func (g *MultiTouchRecognizer) Feed(ev Event) {
	switch ev.Kind {
	case EventTouchStart:
		g.addContact(ev.Code, ev.X, ev.Y)
		g.reconcile()
	case EventTouchMove:
		i := g.indexOf(ev.Code)
		if i < 0 {
			return
		}
		g.contacts[i].x, g.contacts[i].y = ev.X, ev.Y
		if g.engaged && (ev.Code == g.anchorA || ev.Code == g.anchorB) {
			g.recompute()
			g.fireUpdate()
		}
	case EventTouchEnd:
		i := g.indexOf(ev.Code)
		if i < 0 {
			return
		}
		g.contacts = append(g.contacts[:i], g.contacts[i+1:]...)
		g.reconcile()
	}
}

// indexOf returns the position of the contact with the given id in the
// arrival-ordered slice, or -1 if no such contact is live.
func (g *MultiTouchRecognizer) indexOf(id string) int {
	for i := range g.contacts {
		if g.contacts[i].id == id {
			return i
		}
	}
	return -1
}

// addContact records a newly-started contact, or, if a contact with that id is
// somehow already live, updates its position in place.
func (g *MultiTouchRecognizer) addContact(id string, x, y int) {
	if i := g.indexOf(id); i >= 0 {
		g.contacts[i].x, g.contacts[i].y = x, y
		return
	}
	g.contacts = append(g.contacts, mtContact{id: id, x: x, y: y})
}

// reconcile brings engagement in line with the current set of live contacts
// after a contact was added or removed: it ends a gesture whose anchor lifted,
// then engages (or re-engages) whenever two or more contacts are down and none
// is currently driving a gesture.
func (g *MultiTouchRecognizer) reconcile() {
	if g.engaged && (g.indexOf(g.anchorA) < 0 || g.indexOf(g.anchorB) < 0) {
		g.disengage()
	}
	if !g.engaged && len(g.contacts) >= 2 {
		g.engage()
	}
}

// engage adopts the first two live contacts as anchors, records the begin
// reference geometry, and fires OnMultiBegin.
func (g *MultiTouchRecognizer) engage() {
	g.anchorA = g.contacts[0].id
	g.anchorB = g.contacts[1].id
	ax, ay, bx, by := g.anchorPoints()
	g.initSpan = math.Hypot(bx-ax, by-ay)
	g.initAngle = math.Atan2(by-ay, bx-ax)
	g.initCX = (ax + bx) / 2
	g.initCY = (ay + by) / 2
	g.engaged = true
	g.recompute()
	if g.OnMultiBegin != nil {
		g.OnMultiBegin(g.state)
	}
}

// disengage ends the gesture and fires OnMultiEnd with the last state.
func (g *MultiTouchRecognizer) disengage() {
	g.engaged = false
	if g.OnMultiEnd != nil {
		g.OnMultiEnd(g.state)
	}
}

// anchorPoints returns the two anchor contacts' current positions as floats.
// It is only ever called while both anchors are live, so both lookups hit.
func (g *MultiTouchRecognizer) anchorPoints() (ax, ay, bx, by float64) {
	a := g.contacts[g.indexOf(g.anchorA)]
	b := g.contacts[g.indexOf(g.anchorB)]
	return float64(a.x), float64(a.y), float64(b.x), float64(b.y)
}

// recompute recalculates the current state from the two anchors and the begin
// reference.
func (g *MultiTouchRecognizer) recompute() {
	ax, ay, bx, by := g.anchorPoints()
	span := math.Hypot(bx-ax, by-ay)
	angle := math.Atan2(by-ay, bx-ax)
	cx := (ax + bx) / 2
	cy := (ay + by) / 2
	scale := 1.0
	if g.initSpan != 0 {
		scale = span / g.initSpan
	}
	g.state = MultiTouchState{
		Scale:    scale,
		Rotation: normAngle(angle - g.initAngle),
		CenterX:  cx,
		CenterY:  cy,
		PanX:     cx - g.initCX,
		PanY:     cy - g.initCY,
		Span:     span,
	}
}

// fireUpdate dispatches the update callbacks in a fixed order: the three named
// gesture callbacks, then the whole-state OnMultiUpdate.
func (g *MultiTouchRecognizer) fireUpdate() {
	if g.OnPinch != nil {
		g.OnPinch(g.state.Scale)
	}
	if g.OnRotate != nil {
		g.OnRotate(g.state.Rotation)
	}
	if g.OnPan != nil {
		g.OnPan(g.state.PanX, g.state.PanY)
	}
	if g.OnMultiUpdate != nil {
		g.OnMultiUpdate(g.state)
	}
}

// normAngle folds a radian angle difference into (-Pi, Pi]. The inputs are a
// difference of two atan2 results, each in (-Pi, Pi], so the difference lies
// in (-2Pi, 2Pi) and a single add or subtract of 2Pi always suffices.
func normAngle(a float64) float64 {
	switch {
	case a > math.Pi:
		return a - 2*math.Pi
	case a <= -math.Pi:
		return a + 2*math.Pi
	default:
		return a
	}
}
