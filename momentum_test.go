// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"
)

// exactEq asserts a float equals want to the bit — the whole point of the
// engine's determinism is that a chosen configuration produces exactly these
// values, not merely "close" ones.
func exactEq(t *testing.T, got, want float64, whatf string, args ...any) {
	t.Helper()
	if got != want {
		t.Fatalf(whatf+": got %v (%b), want %v (%b)", append(args, got, got, want, want)...)
	}
}

// --- Control run -----------------------------------------------------------
//
// Before trusting the engine's exact-value assertions, validate the METHOD:
// an independent, hand-written reference of the same exponential-deceleration
// recurrence must reproduce a set of literally hand-computed offsets. Only once
// the control (reference == hand math) holds do we assert the engine (the new
// instrument) reproduces the very same literals. This proves the expected
// values are authored, not merely whatever the engine happens to emit.

// refFling is the reference decelerator: velocity retains Friction each tick,
// stops at/under stop, position integrated with the post-decay velocity — the
// semi-implicit exponential model, spelled out independently of Momentum.
func refFling(offset, v, friction, stop float64, ticks int) []float64 {
	out := make([]float64, 0, ticks)
	for i := 0; i < ticks; i++ {
		v *= friction
		if math.Abs(v) <= stop {
			out = append(out, offset) // settled: offset holds
			continue
		}
		offset += v
		out = append(out, offset)
	}
	return out
}

func TestMomentumControlRunFlingRecurrence(t *testing.T) {
	// Hand-computed by the recurrence with Friction=0.5, dt=1, v0=100, stop=1,
	// starting at offset 0 — every value an exact binary fraction:
	//   t1 v=50    off=50
	//   t2 v=25    off=75
	//   t3 v=12.5  off=87.5
	//   t4 v=6.25  off=93.75
	//   t5 v=3.125 off=96.875
	//   t6 v=1.5625 off=98.4375
	//   t7 v=0.78125 <= 1 -> stop, off stays 98.4375
	handComputed := []float64{50, 75, 87.5, 93.75, 96.875, 98.4375, 98.4375}

	// Control: the independent reference reproduces the hand math exactly.
	ref := refFling(0, 100, 0.5, 1, len(handComputed))
	for i, want := range handComputed {
		exactEq(t, ref[i], want, "reference fling tick %d", i+1)
	}

	// Instrument: the engine, wide bounds so nothing clamps, reproduces them too.
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	m.SetBounds(0, 1e9)
	m.SetOffset(0)
	m.Fling(100)
	for i, want := range handComputed {
		last := i == len(handComputed)-1
		busy := m.Tick(1)
		exactEq(t, m.Offset(), want, "engine fling tick %d", i+1)
		if last && busy {
			t.Fatalf("tick %d: still settling, want settled", i+1)
		}
		if !last && !busy {
			t.Fatalf("tick %d: settled early", i+1)
		}
	}
	if m.Settling() {
		t.Fatalf("Settling() = true after fling came to rest")
	}
	exactEq(t, m.Velocity(), 0, "velocity at rest")
}

// --- Headline exact-value scenarios ---------------------------------------

func TestMomentumFlingExactOffsets(t *testing.T) {
	// Same as the control's instrument leg but asserting velocity at each step,
	// covering the coasting (default) branch of tickFling tick by tick.
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: true}
	m.SetBounds(0, 1e9)
	m.Fling(100)
	wantOff := []float64{50, 75, 87.5, 93.75, 96.875, 98.4375}
	wantVel := []float64{50, 25, 12.5, 6.25, 3.125, 1.5625}
	for i := range wantOff {
		if !m.Tick(1) {
			t.Fatalf("tick %d: settled early", i+1)
		}
		exactEq(t, m.Offset(), wantOff[i], "fling offset tick %d", i+1)
		exactEq(t, m.Velocity(), wantVel[i], "fling velocity tick %d", i+1)
	}
	if m.Tick(1) { // v drops to 0.78125 <= 1 -> settle
		t.Fatalf("tick 7: still settling, want settled")
	}
	exactEq(t, m.Offset(), 98.4375, "offset frozen at settle")
	exactEq(t, m.Velocity(), 0, "velocity zeroed at settle")
}

func TestMomentumHardClampSettlesExactlyAtMaxBoundary(t *testing.T) {
	// Bounce off: a fling that would overrun the content stops dead on the edge.
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	m.SetBounds(0, 100)
	m.SetOffset(90)
	m.Fling(100) // t1: v=50, next=140 > 100 -> clamp to 100, rest
	if m.Tick(1) {
		t.Fatalf("still settling, want settled at boundary")
	}
	exactEq(t, m.Offset(), 100, "offset at max boundary")
	exactEq(t, m.Velocity(), 0, "velocity at boundary")
	if m.Settling() {
		t.Fatalf("Settling() true after hard clamp")
	}
}

func TestMomentumHardClampSettlesExactlyAtMinBoundary(t *testing.T) {
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	m.SetBounds(0, 100)
	m.SetOffset(10)
	m.Fling(-100) // t1: v=-50, next=-40 < 0 -> clamp to 0, rest
	if m.Tick(1) {
		t.Fatalf("still settling, want settled at min boundary")
	}
	exactEq(t, m.Offset(), 0, "offset at min boundary")
	exactEq(t, m.Velocity(), 0, "velocity at boundary")
}

func TestMomentumOverscrollSpringsBackToExactBound(t *testing.T) {
	// Drag 30 px past the top with a rubber band whose asymptote is 60, so the
	// displayed overshoot is exactly 30/(1+30/60) = 20 px below min, then release
	// at rest and let the damped spring (k=0.25, c=0.5, dt=1) return it home.
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 60, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.SetOffset(0)
	m.BeginDrag()
	got := m.DragBy(-30)
	exactEq(t, got, -20, "rubber-banded drag offset")
	exactEq(t, m.Offset(), -20, "offset after resisted drag")

	m.EndDrag(0) // release at rest while overscrolled -> spring
	if !m.Settling() {
		t.Fatalf("Settling() false right after releasing overscrolled")
	}
	// Hand-computed spring return from x0=-20, v0=0:
	//   t1 a=5      v=5      off=-15
	//   t2 a=1.25   v=6.25   off=-8.75
	//   t3 a=-0.9375 v=5.3125 off=-3.4375
	//   t4 v=3.515625 off=0.078125 >= 0 -> snap to 0, rest
	wantOff := []float64{-15, -8.75, -3.4375}
	for i, want := range wantOff {
		if !m.Tick(1) {
			t.Fatalf("spring tick %d settled early", i+1)
		}
		exactEq(t, m.Offset(), want, "spring offset tick %d", i+1)
	}
	if m.Tick(1) { // t4 crosses the bound -> snaps exactly, rests
		t.Fatalf("spring tick 4 still settling, want snapped home")
	}
	exactEq(t, m.Offset(), 0, "spring snapped exactly to bound")
	exactEq(t, m.Velocity(), 0, "velocity zeroed at home")
}

func TestMomentumOverscrollSpringsBackToExactMaxBound(t *testing.T) {
	// Symmetric case at the far edge, seeded white-box, to cover the !below
	// crossing branch of tickSpring. Just assert it lands exactly on max.
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 1000, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.offset = 120
	m.phase = momentumSpring
	settled := false
	for i := 0; i < 1000; i++ {
		if !m.Tick(1) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatalf("max-side spring never settled")
	}
	exactEq(t, m.Offset(), 100, "max-side spring snapped exactly to bound")
	exactEq(t, m.Velocity(), 0, "velocity zeroed at max home")
}

func TestMomentumFlingIntoBounceSettlesExactlyAtBound(t *testing.T) {
	// A live fling whose momentum carries past max: it hands off to the spring
	// and must eventually rest exactly on the bound (couples tickFling's Bounce
	// handoff with the spring return).
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 1000, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.SetOffset(90)
	m.Fling(40) // t1 fling: v=20 next=110 > 100 -> overscroll, spring
	if !m.Tick(1) {
		t.Fatalf("expected the crossing tick to keep settling")
	}
	if m.phase != momentumSpring {
		t.Fatalf("phase after crossing = %d, want spring", m.phase)
	}
	exactEq(t, m.Offset(), 110, "offset just past max after handoff")
	settled := false
	for i := 0; i < 1000; i++ {
		if !m.Tick(1) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatalf("fling-into-bounce never settled")
	}
	exactEq(t, m.Offset(), 100, "fling-into-bounce landed exactly on max")
}

// --- Overscroll cap --------------------------------------------------------

func TestMomentumFlingCapsAtMaxOverscrollAboveMax(t *testing.T) {
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 10, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.SetOffset(95)
	m.Fling(1000) // t1: v=500, next=595 -> capped to 100+10 = 110, velocity zeroed
	m.Tick(1)
	exactEq(t, m.Offset(), 110, "offset capped at max+MaxOverscroll")
	exactEq(t, m.Velocity(), 0, "velocity zeroed hitting the far wall")
}

func TestMomentumFlingCapsAtMaxOverscrollBelowMin(t *testing.T) {
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 10, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.SetOffset(5)
	m.Fling(-1000) // t1: v=-500, next=-495 -> capped to 0-10 = -10
	m.Tick(1)
	exactEq(t, m.Offset(), -10, "offset capped at min-MaxOverscroll")
	exactEq(t, m.Velocity(), 0, "velocity zeroed hitting the far wall")
}

func TestMomentumFlingBounceUncappedBelowMin(t *testing.T) {
	// next < min but within the cap: offset keeps its just-past-bound value and
	// the phase becomes spring (covers the uncapped tickFling Bounce branch and
	// clampOverscroll's pass-through).
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 1000, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.SetOffset(5)
	m.Fling(-20) // t1: v=-10, next=-5 -> overscroll to -5, spring
	m.Tick(1)
	exactEq(t, m.Offset(), -5, "offset just below min, uncapped")
	if m.phase != momentumSpring {
		t.Fatalf("phase = %d, want spring", m.phase)
	}
}

func TestMomentumSpringSnapsViaThreshold(t *testing.T) {
	// A slow, tiny overshoot that never actually crosses the bound in one tick
	// still terminates on it via the SnapDistance/StopVelocity threshold.
	m := &Momentum{
		Friction: 0.5, StopVelocity: 1, Bounce: true,
		Stiffness: 0.25, Damping: 0.5, MaxOverscroll: 1000, SnapDistance: 0.5,
	}
	m.SetBounds(0, 100)
	m.offset = -0.3
	m.velocity = 0
	m.phase = momentumSpring
	// t1: a=0.075, v=0.075, off=-0.225; |off|<=0.5 and |v|<=1 -> snap to 0.
	if m.Tick(1) {
		t.Fatalf("threshold snap did not settle")
	}
	exactEq(t, m.Offset(), 0, "threshold snap landed on bound")
	exactEq(t, m.Velocity(), 0, "threshold snap zeroed velocity")
}

// --- Rubber-band drag resistance ------------------------------------------

func TestMomentumDragWithinBoundsTracksOneForOne(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.SetOffset(10)
	m.BeginDrag()
	exactEq(t, m.DragBy(20), 30, "in-bounds drag tracks finger")
	exactEq(t, m.DragBy(-5), 25, "in-bounds drag tracks back")
}

func TestMomentumDragPastMaxRubberBands(t *testing.T) {
	m := &Momentum{Bounce: true, MaxOverscroll: 60}
	m.SetBounds(0, 100)
	m.SetOffset(100)
	m.BeginDrag()
	// raw = 130 -> over 30 -> 30/(1+30/60)=20 -> 100+20 = 120.
	exactEq(t, m.DragBy(30), 120, "drag past max rubber-bands")
}

func TestMomentumDragImplicitBeginDrag(t *testing.T) {
	// DragBy without BeginDrag begins one implicitly from the current offset.
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.SetOffset(40)
	exactEq(t, m.DragBy(5), 45, "implicit BeginDrag tracks from current offset")
}

func TestMomentumDragBounceOffHardClamps(t *testing.T) {
	m := &Momentum{Bounce: false, MaxOverscroll: 60}
	m.SetBounds(0, 100)
	m.SetOffset(100)
	m.BeginDrag()
	exactEq(t, m.DragBy(30), 100, "Bounce off clamps drag at max")
}

func TestMomentumDragBounceOnZeroOverscrollHardClamps(t *testing.T) {
	m := &Momentum{Bounce: true, MaxOverscroll: 0}
	m.SetBounds(0, 100)
	m.SetOffset(0)
	m.BeginDrag()
	exactEq(t, m.DragBy(-30), 0, "MaxOverscroll 0 clamps drag at min")
}

// --- Fling / phase entry ---------------------------------------------------

func TestMomentumFlingBelowStopVelocityRests(t *testing.T) {
	m := &Momentum{Friction: 0.5, StopVelocity: 10, Bounce: true}
	m.SetBounds(0, 100)
	m.SetOffset(50)
	m.Fling(5) // |5| <= 10 -> rest immediately
	if m.Settling() {
		t.Fatalf("Settling() true after sub-threshold fling")
	}
	exactEq(t, m.Velocity(), 0, "sub-threshold fling zeroes velocity")
}

func TestMomentumFlingWhileOverscrolledEntersSpring(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.offset = -5 // already overscrolled
	m.Fling(1000)
	if m.phase != momentumSpring {
		t.Fatalf("phase = %d, want spring when flinging while overscrolled", m.phase)
	}
}

func TestMomentumFlingAboveMaxEntersSpring(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.offset = 105
	m.Fling(1000)
	if m.phase != momentumSpring {
		t.Fatalf("phase = %d, want spring when flinging above max", m.phase)
	}
}

// --- Tick guards -----------------------------------------------------------

func TestMomentumTickNonPositiveDtReportsSettling(t *testing.T) {
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	m.SetBounds(0, 1e9)
	m.Fling(100)
	if !m.Tick(0) {
		t.Fatalf("Tick(0) while flinging = false, want still-settling true")
	}
	exactEq(t, m.Offset(), 0, "Tick(0) must not move offset")
	m.Stop()
	if m.Tick(-1) {
		t.Fatalf("Tick(-1) at rest = true, want false")
	}
}

func TestMomentumTickAtRestIsNoOp(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.SetOffset(20)
	if m.Tick(1) {
		t.Fatalf("Tick at rest reported settling")
	}
	exactEq(t, m.Offset(), 20, "Tick at rest moved offset")
}

// --- Accessors / mutators --------------------------------------------------

func TestMomentumNewDefaults(t *testing.T) {
	m := NewMomentum()
	if m.Friction <= 0 || m.Friction >= 1 {
		t.Fatalf("Friction = %v, want in (0,1)", m.Friction)
	}
	if m.StopVelocity <= 0 {
		t.Fatalf("StopVelocity = %v, want > 0", m.StopVelocity)
	}
	if !m.Bounce {
		t.Fatalf("Bounce = false, want true by default")
	}
	if m.Stiffness <= 0 || m.Damping <= 0 || m.MaxOverscroll <= 0 || m.SnapDistance <= 0 {
		t.Fatalf("spring defaults must be positive: %+v", m)
	}
}

func TestMomentumSetBoundsOrdersAndReports(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(100, 0) // reversed
	min, max := m.Bounds()
	exactEq(t, min, 0, "SetBounds swapped min")
	exactEq(t, max, 100, "SetBounds swapped max")
	m.SetBounds(5, 50)
	min, max = m.Bounds()
	exactEq(t, min, 5, "SetBounds min")
	exactEq(t, max, 50, "SetBounds max")
}

func TestMomentumSetOffsetClamps(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.SetOffset(-20)
	exactEq(t, m.Offset(), 0, "SetOffset clamps below min")
	m.SetOffset(250)
	exactEq(t, m.Offset(), 100, "SetOffset clamps above max")
	m.SetOffset(40)
	exactEq(t, m.Offset(), 40, "SetOffset in range")
	if m.Settling() {
		t.Fatalf("SetOffset left engine settling")
	}
}

func TestMomentumSetOffsetCancelsDrag(t *testing.T) {
	m := NewMomentum()
	m.SetBounds(0, 100)
	m.BeginDrag()
	m.DragBy(10)
	m.SetOffset(0)
	if m.dragging {
		t.Fatalf("SetOffset did not cancel the drag")
	}
}

func TestMomentumStopHaltsWithoutReclamping(t *testing.T) {
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: true, MaxOverscroll: 1000}
	m.SetBounds(0, 100)
	m.offset = -7 // overscrolled
	m.velocity = 999
	m.phase = momentumFling
	m.dragging = true
	m.Stop()
	if m.Settling() {
		t.Fatalf("Stop left engine settling")
	}
	exactEq(t, m.Velocity(), 0, "Stop zeroed velocity")
	exactEq(t, m.Offset(), -7, "Stop must not re-clamp the offset")
	if m.dragging {
		t.Fatalf("Stop did not clear dragging")
	}
}

func TestMomentumOffsetInt(t *testing.T) {
	m := NewMomentum()
	m.offset = 2.4
	if got := m.OffsetInt(); got != 2 {
		t.Fatalf("OffsetInt(2.4) = %d, want 2", got)
	}
	m.offset = 2.6
	if got := m.OffsetInt(); got != 3 {
		t.Fatalf("OffsetInt(2.6) = %d, want 3", got)
	}
	m.offset = -2.5
	if got := m.OffsetInt(); got != -3 { // round half away from zero
		t.Fatalf("OffsetInt(-2.5) = %d, want -3", got)
	}
}

// --- VelocityTracker -------------------------------------------------------

func TestVelocityTrackerEmptyIsZero(t *testing.T) {
	var vt VelocityTracker
	exactEq(t, vt.Velocity(), 0, "empty tracker velocity")
}

func TestVelocityTrackerAveragesWindow(t *testing.T) {
	var vt VelocityTracker // default window 4
	exactEq(t, vt.Sample(10, 1), 10, "one sample")
	exactEq(t, vt.Sample(20, 1), 15, "mean of 10,20")
	exactEq(t, vt.Sample(30, 1), 20, "mean of 10,20,30")
	exactEq(t, vt.Sample(40, 1), 25, "mean of 10,20,30,40")
	// Fifth sample evicts the oldest (10): mean of 20,30,40,50 = 35.
	exactEq(t, vt.Sample(50, 1), 35, "windowed mean drops oldest")
}

func TestVelocityTrackerUsesDtForRate(t *testing.T) {
	var vt VelocityTracker
	// 10 units in 0.5 s -> 20 px/s.
	exactEq(t, vt.Sample(10, 0.5), 20, "rate uses dt")
}

func TestVelocityTrackerIgnoresNonPositiveDt(t *testing.T) {
	var vt VelocityTracker
	vt.Sample(10, 1)
	exactEq(t, vt.Sample(999, 0), 10, "dt<=0 sample ignored, estimate held")
	exactEq(t, vt.Sample(999, -1), 10, "dt<0 sample ignored")
}

func TestVelocityTrackerCustomWindow(t *testing.T) {
	vt := VelocityTracker{Window: 2}
	vt.Sample(10, 1)
	vt.Sample(20, 1)
	// Window 2: third sample evicts the first -> mean of 20,30 = 25.
	exactEq(t, vt.Sample(30, 1), 25, "custom window of 2")
}

func TestVelocityTrackerReset(t *testing.T) {
	var vt VelocityTracker
	vt.Sample(10, 1)
	vt.Sample(20, 1)
	vt.Reset()
	exactEq(t, vt.Velocity(), 0, "velocity after reset")
	exactEq(t, vt.Sample(100, 1), 100, "fresh sample after reset")
}

// feedTrackerIntoFling proves the tracker + engine compose: samples define a
// launch velocity that then drives a real coast to rest inside bounds.
func TestVelocityTrackerFeedsFling(t *testing.T) {
	var vt VelocityTracker
	vt.Sample(100, 1) // 100 px/s, steady
	m := &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	m.SetBounds(0, 1e9)
	m.Fling(vt.Velocity())
	m.Tick(1)
	exactEq(t, m.Offset(), 50, "tracked velocity drove the fling")
}
