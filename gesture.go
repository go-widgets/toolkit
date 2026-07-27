// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// SwipeDir is the dominant-axis direction of a recognized swipe gesture.
type SwipeDir int

const (
	// SwipeLeft is a horizontal swipe whose net motion is negative in X.
	SwipeLeft SwipeDir = iota
	// SwipeRight is a horizontal swipe whose net motion is positive in X.
	SwipeRight
	// SwipeUp is a vertical swipe whose net motion is negative in Y.
	SwipeUp
	// SwipeDown is a vertical swipe whose net motion is positive in Y.
	SwipeDown
)

// GestureRecognizer turns a stream of EventTouchStart / EventTouchMove /
// EventTouchEnd events (see widget.go) into higher-level tap, long-press and
// swipe callbacks. It is pure logic — it does not draw or hold a Widget
// reference — so any widget (or a host, ahead of dispatch) can embed one.
//
// State machine:
//
// EventTouchStart always (re)arms the recognizer: it becomes "active",
// remembers the touch/pointer id (Event.Code) and the start position, and
// resets the hold-tick counter and the long-press-fired flag. Only one
// touch is tracked at a time — a recognizer is meant to sit behind a
// single widget's single active contact; multi-touch gestures (pinch, ...)
// are out of scope and left as future work.
//
// EventTouchMove updates the current position, but only when it carries
// the id of the active touch; anything else (no active touch, or a
// different id — e.g. a second finger) is ignored.
//
// Event has no timestamp, so long-press timing is driven by the caller
// calling Tick() on its own clock (e.g. once per animation frame or
// per timer tick) instead of wall-clock time. While a touch is active and
// has not yet moved past TapSlop, each Tick() increments a hold counter;
// once it reaches LongPressTicks, OnLongPress fires exactly once for that
// touch and a flag suppresses the Tap that would otherwise fire when the
// touch is released. Once movement exceeds TapSlop, Tick() stops counting
// (a long press requires holding still).
//
// EventTouchEnd resolves the gesture from the net displacement between the
// start and end positions (ignored if the id doesn't match the active
// touch):
//
//   - if the displacement's largest-axis magnitude is >= SwipeMinDist, a
//     swipe fired along whichever axis moved further, in the direction of
//     travel;
//   - otherwise, if the magnitude is <= TapSlop and no long-press already
//     fired for this touch, OnTap fires;
//   - anything in between (moved more than a tap, but not far enough for a
//     swipe) resolves to nothing.
type GestureRecognizer struct {
	// TapSlop is the largest movement (in pixels, on whichever axis moved
	// most) still considered "held still" for tap and long-press purposes.
	TapSlop int
	// LongPressTicks is the number of Tick() calls a touch must be held
	// (without moving past TapSlop) before OnLongPress fires.
	LongPressTicks int
	// SwipeMinDist is the minimum net displacement (in pixels, on the
	// dominant axis) for a release to be recognized as a swipe.
	SwipeMinDist int

	// OnTap fires when a touch starts and ends within TapSlop pixels and
	// wasn't already resolved as a long press. x, y are the release
	// position (widget-local).
	OnTap func(x, y int)
	// OnLongPress fires once a held touch reaches LongPressTicks without
	// moving past TapSlop. x, y are the current (held) position.
	OnLongPress func(x, y int)
	// OnSwipe fires on release when the net displacement reaches
	// SwipeMinDist on its dominant axis.
	OnSwipe func(dir SwipeDir)

	active         bool
	id             string
	startX, startY int
	curX, curY     int
	holdTicks      int
	longPressFired bool
}

// NewGestureRecognizer returns a GestureRecognizer with sensible default
// thresholds (TapSlop=8px, LongPressTicks=30, SwipeMinDist=24px). Callers
// wanting different behaviour can override any field, or build a
// GestureRecognizer{} literal directly with their own thresholds — the
// callbacks and Feed/Tick logic don't depend on how the struct was built.
func NewGestureRecognizer() *GestureRecognizer {
	return &GestureRecognizer{
		TapSlop:        8,
		LongPressTicks: 30,
		SwipeMinDist:   24,
	}
}

// Feed consumes one input event. Only EventTouchStart, EventTouchMove and
// EventTouchEnd are meaningful to a GestureRecognizer; every other kind is
// ignored so a host can feed it its full event stream unfiltered.
func (g *GestureRecognizer) Feed(ev Event) {
	switch ev.Kind {
	case EventTouchStart:
		g.active = true
		g.id = ev.Code
		g.startX, g.startY = ev.X, ev.Y
		g.curX, g.curY = ev.X, ev.Y
		g.holdTicks = 0
		g.longPressFired = false
	case EventTouchMove:
		if !g.active || ev.Code != g.id {
			return
		}
		g.curX, g.curY = ev.X, ev.Y
	case EventTouchEnd:
		if !g.active || ev.Code != g.id {
			return
		}
		g.active = false
		g.resolveEnd(ev.X, ev.Y)
	}
}

// resolveEnd fires OnTap or OnSwipe (or neither) from the net displacement
// between the recorded start position and (x, y), the release position.
func (g *GestureRecognizer) resolveEnd(x, y int) {
	dx, dy := x-g.startX, y-g.startY
	adx, ady := abs(dx), abs(dy)
	moved := adx
	if ady > moved {
		moved = ady
	}
	longFired := g.longPressFired
	g.longPressFired = false

	switch {
	case moved >= g.SwipeMinDist:
		if g.OnSwipe == nil {
			return
		}
		if adx >= ady {
			if dx >= 0 {
				g.OnSwipe(SwipeRight)
			} else {
				g.OnSwipe(SwipeLeft)
			}
		} else {
			if dy >= 0 {
				g.OnSwipe(SwipeDown)
			} else {
				g.OnSwipe(SwipeUp)
			}
		}
	case moved <= g.TapSlop && !longFired:
		if g.OnTap != nil {
			g.OnTap(x, y)
		}
	}
}

// Tick advances the long-press timer by one caller-driven step. It is a
// no-op unless a touch is currently active and held within TapSlop of its
// start position; once the hold reaches LongPressTicks, OnLongPress fires
// exactly once for that touch (subsequent ticks, and the eventual
// EventTouchEnd's Tap, are then suppressed for it).
func (g *GestureRecognizer) Tick() {
	if !g.active || g.longPressFired {
		return
	}
	dx, dy := g.curX-g.startX, g.curY-g.startY
	adx, ady := abs(dx), abs(dy)
	moved := adx
	if ady > moved {
		moved = ady
	}
	if moved > g.TapSlop {
		return
	}
	g.holdTicks++
	if g.holdTicks < g.LongPressTicks {
		return
	}
	g.longPressFired = true
	if g.OnLongPress != nil {
		g.OnLongPress(g.curX, g.curY)
	}
}
