// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package anim

import (
	"testing"
	"time"

	"github.com/go-widgets/toolkit"
)

// base is a fixed, arbitrary wall-clock origin. Every test drives the driver
// with a fake clock derived from base — no real time is ever read, so timing
// assertions are exact and deterministic.
var base = time.Date(2026, 8, 7, 14, 25, 0, 0, time.UTC)

// at returns base advanced by ms milliseconds.
func at(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }

// TestTimingEaseOutQuad asserts exact eased values at t=0/100/200ms for a
// 200ms EaseOutQuad animation, and that it reaches progress 1.0 exactly at
// 200ms. EaseOutQuad(0.5) = 1-(0.5)^2 = 0.75.
func TestTimingEaseOutQuad(t *testing.T) {
	d := NewDriver()
	var got float64
	d.Start(&Animation{
		Dur:   200 * time.Millisecond,
		Ease:  toolkit.EaseOutQuad,
		Apply: func(p float64) { got = p },
	})

	if busy := d.Tick(at(0)); !busy {
		t.Fatalf("t=0: want busy=true, got false")
	}
	if got != 0 {
		t.Fatalf("t=0: want eased 0, got %v", got)
	}

	if busy := d.Tick(at(100)); !busy {
		t.Fatalf("t=100: want busy=true, got false")
	}
	if got != 0.75 {
		t.Fatalf("t=100: want eased 0.75 (EaseOutQuad(0.5)), got %v", got)
	}

	// At exactly Dur the animation completes: final Apply(1.0) runs and the
	// active set empties in the same tick, so busy flips to false.
	if busy := d.Tick(at(200)); busy {
		t.Fatalf("t=200: want busy=false (idle suppression), got true")
	}
	if got != 1 {
		t.Fatalf("t=200: want eased 1.0, got %v", got)
	}
}

// TestIdleSuppression asserts that once the only animation finishes, Tick
// keeps returning busy=false so the host can stop scheduling frames.
func TestIdleSuppression(t *testing.T) {
	d := NewDriver()
	d.Start(&Animation{Dur: 100 * time.Millisecond, Apply: func(float64) {}})

	if !d.Tick(at(0)) {
		t.Fatalf("first tick: want busy=true")
	}
	if d.Tick(at(100)) {
		t.Fatalf("completing tick: want busy=false")
	}
	// Every subsequent tick with an empty active set stays quiet.
	if d.Tick(at(200)) {
		t.Fatalf("post-completion tick: want busy=false")
	}
}

// TestEmptyDriverBusyFalse asserts a driver with zero active animations
// reports busy=false.
func TestEmptyDriverBusyFalse(t *testing.T) {
	d := NewDriver()
	if d.Tick(at(0)) {
		t.Fatalf("empty driver: want busy=false")
	}
}

// TestZeroValueDriver asserts the zero-value Driver is usable without
// NewDriver.
func TestZeroValueDriver(t *testing.T) {
	var d Driver
	ended := false
	d.Start(&Animation{Dur: 10 * time.Millisecond, OnEnd: func() { ended = true }})
	d.Tick(at(0))
	d.Tick(at(10))
	if !ended {
		t.Fatalf("zero-value driver: OnEnd did not fire")
	}
}

// TestCancelMidFlight asserts Cancel drops the animation on the next tick with
// no further Apply and no OnEnd.
func TestCancelMidFlight(t *testing.T) {
	d := NewDriver()
	applies := 0
	ended := false
	h := d.Start(&Animation{
		Dur:   200 * time.Millisecond,
		Apply: func(float64) { applies++ },
		OnEnd: func() { ended = true },
	})

	d.Tick(at(0)) // one Apply so far
	if applies != 1 {
		t.Fatalf("before cancel: want 1 apply, got %d", applies)
	}
	h.Cancel()
	if busy := d.Tick(at(50)); busy {
		t.Fatalf("after cancel: want busy=false (dropped), got true")
	}
	if applies != 1 {
		t.Fatalf("after cancel: Apply ran again (%d)", applies)
	}
	if ended {
		t.Fatalf("after cancel: OnEnd fired, must not")
	}
	// Cancelling again is a harmless no-op.
	h.Cancel()
}

// TestOnEndAndApplyNil covers the nil-Apply branch (pure timer) and confirms
// OnEnd fires exactly once at completion.
func TestOnEndAndApplyNil(t *testing.T) {
	d := NewDriver()
	ends := 0
	d.Start(&Animation{Dur: 100 * time.Millisecond, OnEnd: func() { ends++ }})

	d.Tick(at(0))  // progress 0, Apply nil -> no-op
	d.Tick(at(50)) // progress 0.5, still running
	d.Tick(at(100))
	if ends != 1 {
		t.Fatalf("want OnEnd once, got %d", ends)
	}
	// No further ticks fire it again.
	d.Tick(at(150))
	if ends != 1 {
		t.Fatalf("OnEnd fired after completion, got %d", ends)
	}
}

// TestZeroDurationImmediate covers Dur <= 0: the animation completes on its
// first tick, applies eased 1.0, and fires OnEnd.
func TestZeroDurationImmediate(t *testing.T) {
	d := NewDriver()
	var got float64
	ended := false
	d.Start(&Animation{
		Dur:   0,
		Apply: func(p float64) { got = p },
		OnEnd: func() { ended = true },
	})
	if busy := d.Tick(at(0)); busy {
		t.Fatalf("zero-duration: want busy=false on first tick")
	}
	if got != 1 {
		t.Fatalf("zero-duration: want eased 1.0, got %v", got)
	}
	if !ended {
		t.Fatalf("zero-duration: OnEnd did not fire")
	}
}

// TestNilEaseIsLinear covers the nil-Ease branch: progress passes through
// unshaped (linear).
func TestNilEaseIsLinear(t *testing.T) {
	d := NewDriver()
	var got float64
	d.Start(&Animation{Dur: 100 * time.Millisecond, Apply: func(p float64) { got = p }})
	d.Tick(at(0))
	d.Tick(at(25))
	if got != 0.25 {
		t.Fatalf("nil ease: want linear 0.25, got %v", got)
	}
}

// TestNilOnEnd covers the nil-OnEnd branch: an animation may complete without
// an end callback.
func TestNilOnEnd(t *testing.T) {
	d := NewDriver()
	d.Start(&Animation{Dur: 10 * time.Millisecond, Apply: func(float64) {}})
	d.Tick(at(0))
	if d.Tick(at(10)) {
		t.Fatalf("nil OnEnd: want busy=false after completion")
	}
}

// TestOverlappingAnimations runs two animations of different durations at once
// and asserts each advances independently and the driver stays busy until the
// last one ends.
func TestOverlappingAnimations(t *testing.T) {
	d := NewDriver()
	var a, b float64
	d.Start(&Animation{Dur: 100 * time.Millisecond, Apply: func(p float64) { a = p }})
	d.Start(&Animation{Dur: 200 * time.Millisecond, Apply: func(p float64) { b = p }})

	d.Tick(at(0))
	if !d.Tick(at(100)) {
		t.Fatalf("t=100: want busy=true (second still running)")
	}
	if a != 1 {
		t.Fatalf("t=100: first want 1.0, got %v", a)
	}
	if b != 0.5 {
		t.Fatalf("t=100: second want 0.5, got %v", b)
	}
	// First is gone; only the second remains and now completes.
	if d.Tick(at(200)) {
		t.Fatalf("t=200: want busy=false (both done)")
	}
	if b != 1 {
		t.Fatalf("t=200: second want 1.0, got %v", b)
	}
}

// TestTweenConvenience covers Driver.Tween: it interpolates a toolkit.Tween's
// From..To over a wall-clock duration and fires the end callback.
func TestTweenConvenience(t *testing.T) {
	d := NewDriver()
	tw := toolkit.NewTween(10, 20, 999 /*ignored*/, toolkit.Linear)
	var v float64
	ended := false
	d.Tween(tw, 100*time.Millisecond, func(x float64) { v = x }, func() { ended = true })

	d.Tick(at(0))
	if v != 10 {
		t.Fatalf("t=0: want 10, got %v", v)
	}
	d.Tick(at(50))
	if v != 15 {
		t.Fatalf("t=50: want 15, got %v", v)
	}
	if d.Tick(at(100)) {
		t.Fatalf("t=100: want busy=false")
	}
	if v != 20 {
		t.Fatalf("t=100: want 20, got %v", v)
	}
	if !ended {
		t.Fatalf("Tween: onEnd did not fire")
	}
}

// TestTickZeroAllocs asserts Tick performs no per-frame allocation with N
// active animations (battery/CPU-friendly steady state). The animations never
// complete under the fixed clock, so nothing is removed between runs.
func TestTickZeroAllocs(t *testing.T) {
	d := NewDriver()
	const n = 16
	for i := 0; i < n; i++ {
		d.Start(&Animation{
			Dur:   time.Hour,
			Ease:  toolkit.EaseInOutCubic,
			Apply: func(float64) {},
		})
	}
	// Prime the started flags outside the measured window.
	d.Tick(base)
	if allocs := testing.AllocsPerRun(100, func() { d.Tick(base) }); allocs != 0 {
		t.Fatalf("Tick allocated %v times per run, want 0", allocs)
	}
}
