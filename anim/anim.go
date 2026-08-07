// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package anim is a backend-agnostic timeline driver for the go-widgets
// toolkit. It turns the host's existing per-frame tick (a tui EventTick, a
// wasm requestAnimationFrame callback, …) into wall-clock-eased animations,
// and suppresses idle frames once nothing is animating so the host can stop
// scheduling work (saving battery and CPU).
//
// The driver owns no goroutine and no timer: the host owns the loop, exactly
// like the rest of the toolkit, whose widgets already expose a manual
// Tick(dt). The host calls [Driver.Tick] with the current wall-clock time
// each frame; when it returns busy=false the active set is empty and the host
// may stop scheduling frames until the next interaction starts a new
// animation.
//
// Animations reuse the toolkit's existing [toolkit.Easing] curves, so the
// package makes easing.go load-bearing.
package anim

import (
	"time"

	"github.com/go-widgets/toolkit"
)

// Animation describes a single time-bounded animation. Its progress runs from
// 0 at the first tick after it starts to 1 at Dur, shaped by Ease, and is
// pushed to the target via Apply. OnEnd fires once, on the tick that reaches
// progress 1, right after the final Apply.
type Animation struct {
	// Dur is the wall-clock duration of the animation. A Dur <= 0 completes
	// the animation on its very first tick (progress jumps straight to 1).
	Dur time.Duration
	// Ease shapes the linear 0..1 progress into the eased value handed to
	// Apply. A nil Ease behaves as [toolkit.Linear].
	Ease toolkit.Easing
	// Apply pushes the eased progress (0..1) to the target each tick. A nil
	// Apply makes the animation a pure timer (only OnEnd matters).
	Apply func(progress float64)
	// OnEnd, if non-nil, is called once when the animation completes.
	OnEnd func()
}

// entry is the driver's per-animation bookkeeping: the animation itself, the
// wall-clock instant it first ticked, and lifecycle flags.
type entry struct {
	anim     *Animation
	start    time.Time
	started  bool
	canceled bool
}

// Handle refers to a running animation so the caller can cancel it. It is
// returned by [Driver.Start]; a zero Handle is not useful on its own.
type Handle struct {
	e *entry
}

// Cancel stops the animation before it completes. The next [Driver.Tick]
// drops it without a further Apply and without firing OnEnd. Cancelling an
// already-finished (or already-cancelled) animation is a harmless no-op.
func (h *Handle) Cancel() {
	if h.e != nil {
		h.e.canceled = true
	}
}

// Driver advances a set of active animations from the host's frame loop. It
// holds no goroutine and no timer; the zero value is ready to use, though
// [NewDriver] reads more clearly at call sites.
type Driver struct {
	active []*entry
}

// NewDriver returns an empty, ready-to-use Driver.
func NewDriver() *Driver { return &Driver{} }

// Start registers a to run and returns a [Handle] for cancelling it. The
// animation begins on the next [Driver.Tick] (that tick is progress 0). Start
// may grow the active set, but [Driver.Tick] itself never allocates.
func (d *Driver) Start(a *Animation) *Handle {
	e := &entry{anim: a}
	d.active = append(d.active, e)
	return &Handle{e: e}
}

// Tween is a convenience over [Driver.Start] that animates a scalar from the
// tween's From to its To over dur, shaped by the tween's Ease, invoking set
// with each interpolated value and onEnd (if non-nil) once at the end. It
// reuses the toolkit's [toolkit.Tween] fields, making easing.go's Tween
// load-bearing here too. The integer Tween.Duration (tick-based) is ignored
// in favour of the wall-clock dur.
func (d *Driver) Tween(tw *toolkit.Tween, dur time.Duration, set func(v float64), onEnd func()) *Handle {
	from, to, ease := tw.From, tw.To, tw.Ease
	return d.Start(&Animation{
		Dur:  dur,
		Ease: ease,
		Apply: func(p float64) {
			set(from + (to-from)*p)
		},
		OnEnd: onEnd,
	})
}

// Tick advances every active animation by the wall-clock delta since it
// started, calls each animation's Apply with the eased progress, fires OnEnd
// and removes any animation that has reached progress 1 (or was cancelled),
// and reports whether any animation remains active. When busy is false the
// active set is empty and the host may stop scheduling frames.
//
// Tick performs no allocation: finished and cancelled entries are filtered in
// place, reusing the backing array.
func (d *Driver) Tick(now time.Time) (busy bool) {
	w := 0
	for _, e := range d.active {
		if e.canceled {
			continue
		}
		if !e.started {
			e.start = now
			e.started = true
		}
		var progress float64
		done := false
		if e.anim.Dur <= 0 {
			progress = 1
			done = true
		} else {
			progress = float64(now.Sub(e.start)) / float64(e.anim.Dur)
			if progress >= 1 {
				progress = 1
				done = true
			}
		}
		eased := progress
		if e.anim.Ease != nil {
			eased = e.anim.Ease(progress)
		}
		if e.anim.Apply != nil {
			e.anim.Apply(eased)
		}
		if done {
			if e.anim.OnEnd != nil {
				e.anim.OnEnd()
			}
			continue
		}
		d.active[w] = e
		w++
	}
	for i := w; i < len(d.active); i++ {
		d.active[i] = nil
	}
	d.active = d.active[:w]
	return w > 0
}
