// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"

	"github.com/go-widgets/painter"
)

// resetMetrics restores the package globals a test may have perturbed, so the
// exact row-height / offset assertions never bleed across tests.
func resetMetrics(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SetMetricScale(1)
		SetDensity(DensityCompact)
	})
	SetMetricScale(1)
	SetDensity(DensityCompact)
}

// months is a convenient 12-value column used across the tests.
func months() []string {
	return []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
}

// tune forces a column onto the hand-computable friction=0.5 / dt=1 physics so a
// spin's offsets are exact binary fractions (the momentum control-run style).
func tune(c *wheelColumn) {
	c.mom.Friction = 0.5
	c.mom.StopVelocity = 0.3
	// Keep the default stiff spring so a dt=1 snap lands in one overshoot-clamp
	// step, exactly onto the detent.
}

// --- Control run -----------------------------------------------------------
//
// Validate the METHOD before trusting the widget: an independent, hand-written
// reference of the SAME fling recurrence plus a nearest-row snap must reproduce a
// set of literally hand-computed offsets. Only once the control (reference ==
// hand math) holds do we assert the widget (the new instrument) reproduces the
// very same literals — proving the spin+snap values are authored, not merely
// whatever the widget happens to emit.

// refSpinSnap is the reference: an exponential-deceleration fling in ROW units
// (velocity keeps friction each unit-dt tick, stops at/under stop, position
// integrated with the post-decay velocity) followed by a snap to the nearest
// whole row once the coast has stopped. It returns the offset after each of
// ticks frames. It never touches Momentum, so agreement with the widget is real
// corroboration.
func refSpinSnap(v0, friction, stop float64, ticks int) []float64 {
	off := 0.0
	v := v0
	stopped := false
	snapped := false
	out := make([]float64, 0, ticks)
	for i := 0; i < ticks; i++ {
		switch {
		case !stopped:
			v *= friction
			if math.Abs(v) <= stop {
				stopped = true // coast dies; offset holds this frame
			} else {
				off += v
			}
		case !snapped:
			off = math.Round(off) // snap onto the nearest detent
			snapped = true
		}
		out = append(out, off)
	}
	return out
}

func TestWheelPickerControlRunSpinAndSnapUp(t *testing.T) {
	resetMetrics(t)

	// Hand-computed with friction=0.5, dt=1, v0=4, stop=0.3, from offset 0:
	//   t1 v=2.0   off=2.0
	//   t2 v=1.0   off=3.0
	//   t3 v=0.5   off=3.5
	//   t4 v=0.25 <= 0.3 -> coast stops, off holds 3.5
	//   t5 snap round(3.5)=4 -> off=4.0
	hand := []float64{2.0, 3.0, 3.5, 3.5, 4.0}

	// Control: the independent reference reproduces the hand math exactly.
	ref := refSpinSnap(4, 0.5, 0.3, len(hand))
	for i, want := range hand {
		exactEq(t, ref[i], want, "reference spin+snap tick %d", i+1)
	}

	// Instrument: the widget, with the same forced physics, reproduces them too.
	w := NewWheelPicker(months())
	tune(w.columns[0])
	w.columns[0].mom.SetBounds(0, w.columns[0].maxOffset())
	w.columns[0].mom.Fling(4)
	for i, want := range hand {
		w.Tick(1)
		exactEq(t, w.columns[0].offsetRows(), want, "widget spin+snap tick %d", i+1)
	}
	if got := w.SelectedIndex(0); got != 4 {
		t.Fatalf("landed index: got %d want 4", got)
	}
	if w.Settling() {
		t.Fatalf("still settling after snap")
	}
}

func TestWheelPickerControlRunSnapDown(t *testing.T) {
	resetMetrics(t)

	// friction=0.5, dt=1, v0=1.5, stop=0.3 from 0 (every value an exact binary
	// fraction, so the comparison is bit-exact):
	//   t1 v=0.75   off=0.75
	//   t2 v=0.375  off=1.125
	//   t3 v=0.1875 <= 0.3 -> stop, off holds 1.125
	//   t4 snap round(1.125)=1 -> off=1.0   (a mid-row rest snaps DOWN to 1)
	hand := []float64{0.75, 1.125, 1.125, 1.0}
	ref := refSpinSnap(1.5, 0.5, 0.3, len(hand))
	for i, want := range hand {
		exactEq(t, ref[i], want, "reference snap-down tick %d", i+1)
	}

	w := NewWheelPicker(months())
	tune(w.columns[0])
	w.columns[0].mom.SetBounds(0, w.columns[0].maxOffset())
	w.columns[0].mom.Fling(1.5)
	for i, want := range hand {
		w.Tick(1)
		exactEq(t, w.columns[0].offsetRows(), want, "widget snap-down tick %d", i+1)
	}
	if got := w.SelectedIndex(0); got != 1 {
		t.Fatalf("snap-down index: got %d want 1", got)
	}
}

// TestWheelPickerFlingDelegatesToMomentum sweeps the DEFAULT-tuned spin against
// an independent Momentum engine seeded identically, proving the widget reuses
// the engine verbatim for the coast rather than reinventing the physics — then
// that it settles on an exact integer detent.
func TestWheelPickerFlingDelegatesToMomentum(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	c := w.columns[0]

	// Independent reference with the same fields, bounds and launch.
	ref := &Momentum{
		Friction:      wheelFriction,
		StopVelocity:  wheelStopVelocity,
		Bounce:        true,
		Stiffness:     wheelStiffness,
		Damping:       wheelDamping,
		MaxOverscroll: wheelMaxOverscroll,
		SnapDistance:  wheelSnapDistance,
	}
	ref.SetBounds(0, c.maxOffset())
	ref.SetOffset(0)
	ref.Fling(3.0)

	c.mom.SetBounds(0, c.maxOffset())
	c.mom.Fling(3.0)

	const dt = 1.0 / 120.0
	for ref.Settling() {
		ref.Tick(dt)
		w.Tick(dt)
		exactEq(t, c.offsetRows(), ref.Offset(),
			"widget offset must equal reference momentum offset")
	}

	// The reference coasts to rest mid-row; the widget then snaps. Drive to rest.
	for w.Settling() {
		w.Tick(dt)
	}
	off := c.offsetRows()
	if off != math.Trunc(off) {
		t.Fatalf("widget did not settle on an integer detent: off=%v", off)
	}
	if got := w.SelectedIndex(0); float64(got) != off {
		t.Fatalf("index %d disagrees with detent %v", got, off)
	}
}

// TestWheelPickerSnapAlwaysLandsExact drives a family of mid-row rest positions
// and asserts each snaps to the exact nearest integer boundary (never a fraction)
// and to the arithmetically-correct row.
func TestWheelPickerSnapAlwaysLandsExact(t *testing.T) {
	resetMetrics(t)
	cases := []struct {
		start float64
		want  int
	}{
		{2.1, 2}, {2.49, 2}, {2.5, 3}, {2.9, 3}, {0.0, 0}, {11.0, 11}, {7.5, 8},
	}
	for _, tc := range cases {
		w := NewWheelPicker(months())
		c := w.columns[0]
		// Seat the offset off-detent by hand, then release from rest so only the
		// snap runs.
		c.mom.SetBounds(0, 100) // wide so SetOffset does not clamp the fractional seed
		c.mom.SetOffset(tc.start)
		c.mom.SetBounds(0, c.maxOffset())
		c.mom.Fling(0) // at rest, in bounds -> first Tick triggers the snap
		// Always tick at least once so the index reconciles even for a seed that
		// is already on a detent; then run out any snap in flight.
		for i := 0; i < 200; i++ {
			w.Tick(1.0 / 60.0)
			if !w.Settling() {
				break
			}
		}
		if w.Settling() {
			t.Fatalf("start %v: never settled", tc.start)
		}
		off := c.offsetRows()
		if off != float64(tc.want) {
			t.Fatalf("start %v: snapped to %v want %d", tc.start, off, tc.want)
		}
		if got := w.SelectedIndex(0); got != tc.want {
			t.Fatalf("start %v: index %d want %d", tc.start, got, tc.want)
		}
	}
}

func TestWheelPickerOnChangeSequenceExact(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	tune(w.columns[0])
	var got [][2]int
	w.OnChange = func(col, idx int) { got = append(got, [2]int{col, idx}) }
	w.columns[0].mom.SetBounds(0, w.columns[0].maxOffset())
	w.columns[0].mom.Fling(4) // same spin as the up-snap control run

	for w.Settling() {
		w.Tick(1)
	}
	// Offsets 2.0, 3.0, 3.5(round=4); the snap lands on 4 (no further change).
	want := [][2]int{{0, 2}, {0, 3}, {0, 4}}
	if len(got) != len(want) {
		t.Fatalf("OnChange count: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OnChange[%d]: got %v want %v", i, got[i], want[i])
		}
	}
}

// --- Rubber-band overscroll at the ends ------------------------------------

func TestWheelPickerFlingPastEndSpringsToLastRow(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a", "b", "c"})
	c := w.columns[0]
	c.mom.SetBounds(0, c.maxOffset())
	c.mom.SetOffset(2) // last row
	c.mom.Fling(50)    // hurl hard past the end
	for i := 0; i < 2000 && w.Settling(); i++ {
		w.Tick(1.0 / 120.0)
	}
	if w.Settling() {
		t.Fatalf("overscroll never settled")
	}
	if off := c.offsetRows(); off != 2 {
		t.Fatalf("did not spring back to last row: off=%v", off)
	}
	if got := w.SelectedIndex(0); got != 2 {
		t.Fatalf("index after overscroll: got %d want 2", got)
	}
}

func TestWheelPickerFlingPastStartSpringsToFirstRow(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a", "b", "c"})
	c := w.columns[0]
	c.mom.SetBounds(0, c.maxOffset())
	c.mom.SetOffset(0)
	c.mom.Fling(-50)
	for i := 0; i < 2000 && w.Settling(); i++ {
		w.Tick(1.0 / 120.0)
	}
	if off := c.offsetRows(); off != 0 {
		t.Fatalf("did not spring back to first row: off=%v", off)
	}
}

// --- Touch drag path -------------------------------------------------------

func TestWheelPickerTouchDragThenFlingAndSnap(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 140})
	rowH := w.rowHeight()
	if rowH != 28 {
		t.Fatalf("precondition rowHeight: got %d want 28", rowH)
	}
	// Finger down at row centre, then drag UP by exactly two rows over two equal
	// samples: the strip should advance two rows.
	w.TouchDown(Event{Kind: EventTouchStart, X: 10, Y: 50})
	w.TouchMove(Event{Kind: EventTouchMove, X: 10, Y: 50 - rowH}, 1.0/60.0)
	w.TouchMove(Event{Kind: EventTouchMove, X: 10, Y: 50 - 2*rowH}, 1.0/60.0)
	if off := w.columns[0].offsetRows(); off != 2 {
		t.Fatalf("drag offset: got %v want 2", off)
	}
	w.TouchUp()
	for i := 0; i < 500 && w.Settling(); i++ {
		w.Tick(1.0 / 60.0)
	}
	off := w.columns[0].offsetRows()
	if off != math.Trunc(off) {
		t.Fatalf("post-fling offset not a detent: %v", off)
	}
	if off < 2 {
		t.Fatalf("fling went backwards: %v", off)
	}
}

func TestWheelPickerTouchTapReleaseSnapsBackToDetent(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 140})
	rowH := w.rowHeight()
	// A slow quarter-row drag (dt a full second, so the release velocity 0.25
	// rows/s is under the stop threshold) then release: the coast dies at once
	// and the snap returns to the original row 0.
	w.TouchDown(Event{Kind: EventTouchStart, X: 5, Y: 50})
	w.TouchMove(Event{Kind: EventTouchMove, X: 5, Y: 50 - rowH/4}, 1.0)
	w.TouchUp()
	for i := 0; i < 200 && w.Settling(); i++ {
		w.Tick(1.0 / 60.0)
	}
	if got := w.SelectedIndex(0); got != 0 {
		t.Fatalf("small drag should snap back to 0, got %d", got)
	}
}

func TestWheelPickerTouchMoveWithoutDownIsNoop(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.TouchMove(Event{Kind: EventTouchMove, X: 5, Y: 10}, 1.0/60.0)
	if off := w.columns[0].offsetRows(); off != 0 {
		t.Fatalf("stray TouchMove moved the wheel: %v", off)
	}
	w.TouchUp() // no drag: harmless
}

func TestWheelPickerTouchDownOutsideColumnsIgnored(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 140})
	w.TouchDown(Event{Kind: EventTouchStart, X: 999, Y: 10}) // past the right edge
	if w.dragging {
		t.Fatalf("drag armed on an out-of-column press")
	}
}

func TestWheelPickerDisabledIgnoresTouch(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 140})
	w.Disabled().Set(true)
	w.TouchDown(Event{Kind: EventTouchStart, X: 10, Y: 50})
	if w.dragging {
		t.Fatalf("disabled wheel armed a drag")
	}
	w.dragging = true // even if forced, a disabled move does nothing
	w.dragCol = 0
	before := w.columns[0].offsetRows()
	w.TouchMove(Event{Kind: EventTouchMove, X: 10, Y: 0}, 1.0/60.0)
	if w.columns[0].offsetRows() != before {
		t.Fatalf("disabled wheel moved on drag")
	}
}

// rowHeight must be reachable as 0 so the guard is exercised; a tiny metric scale
// under compact density collapses it.
func TestWheelPickerZeroRowHeightGuards(t *testing.T) {
	resetMetrics(t)
	SetMetricScale(0.001)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 10})
	if rh := w.rowHeight(); rh != 0 {
		t.Fatalf("precondition: want rowHeight 0, got %d", rh)
	}
	// Both drag and click early-return on a zero row height.
	w.TouchDown(Event{Kind: EventTouchStart, X: 10, Y: 5})
	w.TouchMove(Event{Kind: EventTouchMove, X: 10, Y: 0}, 1.0/60.0)
	w.OnEvent(Event{Kind: EventClick, X: 10, Y: 0})
	if got := w.SelectedIndex(0); got != 0 {
		t.Fatalf("zero-row-height interactions changed selection: %d", got)
	}
}

// --- Keyboard (a11y) path --------------------------------------------------

func TestWheelPickerKeyboardStepsSelection(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	var fired [][2]int
	w.OnChange = func(col, idx int) { fired = append(fired, [2]int{col, idx}) }

	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if got := w.SelectedIndex(0); got != 1 {
		t.Fatalf("after down,down,up: got %d want 1", got)
	}
	if w.Settling() {
		t.Fatalf("keyboard step must not fling")
	}
	want := [][2]int{{0, 1}, {0, 2}, {0, 1}}
	if len(fired) != len(want) {
		t.Fatalf("OnChange seq: got %v want %v", fired, want)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Fatalf("OnChange[%d]: got %v want %v", i, fired[i], want[i])
		}
	}
}

func TestWheelPickerKeyboardClampsAtEnds(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a", "b", "c"})
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"}) // already at 0
	if got := w.SelectedIndex(0); got != 0 {
		t.Fatalf("up at top: got %d want 0", got)
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if got := w.SelectedIndex(0); got != 2 {
		t.Fatalf("End: got %d want 2", got)
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // already at last
	if got := w.SelectedIndex(0); got != 2 {
		t.Fatalf("down at bottom: got %d want 2", got)
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if got := w.SelectedIndex(0); got != 0 {
		t.Fatalf("Home: got %d want 0", got)
	}
}

func TestWheelPickerKeyboardFocusMovesBetweenColumns(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a", "b"}, []string{"x", "y", "z"})
	if w.Focus() != 0 {
		t.Fatalf("initial focus: got %d want 0", w.Focus())
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if w.Focus() != 1 {
		t.Fatalf("after Right: focus %d want 1", w.Focus())
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // steps column 1
	if w.SelectedIndex(1) != 1 || w.SelectedIndex(0) != 0 {
		t.Fatalf("Down after focus move: col0=%d col1=%d", w.SelectedIndex(0), w.SelectedIndex(1))
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // clamps at last column
	if w.Focus() != 1 {
		t.Fatalf("Right past end changed focus to %d", w.Focus())
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if w.Focus() != 0 {
		t.Fatalf("after Left: focus %d want 0", w.Focus())
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // clamps at first
	if w.Focus() != 0 {
		t.Fatalf("Left past start changed focus to %d", w.Focus())
	}
}

func TestWheelPickerUnknownKeyIgnored(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("unknown key changed selection")
	}
}

func TestWheelPickerDisabledIgnoresOnEvent(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.Disabled().Set(true)
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	w.OnEvent(Event{Kind: EventScroll, X: 5, Y: 5, Delta: 3})
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("disabled wheel reacted to input")
	}
}

func TestWheelPickerUnhandledEventKindIgnored(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5}) // not a wheel interaction
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("mouse-move changed selection")
	}
}

// --- Wheel notch + tap -----------------------------------------------------

func TestWheelPickerScrollSteps(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a", "b"}, months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 140})
	// Scroll over the SECOND column (right half) by 3 rows.
	w.OnEvent(Event{Kind: EventScroll, X: 60, Y: 70, Delta: 3})
	if w.SelectedIndex(1) != 3 {
		t.Fatalf("scroll col1: got %d want 3", w.SelectedIndex(1))
	}
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("scroll must not touch col0: got %d", w.SelectedIndex(0))
	}
	if w.Focus() != 1 {
		t.Fatalf("scroll should focus the scrolled column, focus=%d", w.Focus())
	}
}

func TestWheelPickerScrollOutsideIgnored(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 140})
	w.OnEvent(Event{Kind: EventScroll, X: 500, Y: 10, Delta: 2})
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("out-of-bounds scroll changed selection")
	}
}

func TestWheelPickerTapNudgesTowardRow(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	rowH := w.rowHeight()
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: rowH * 5})
	centreY := (rowH * 5) / 2
	// Tap two rows below the centre -> advance two rows.
	w.OnEvent(Event{Kind: EventClick, X: 10, Y: centreY + 2*rowH})
	if got := w.SelectedIndex(0); got != 2 {
		t.Fatalf("tap below: got %d want 2", got)
	}
	// Tap one row above the (new) centre -> step back one.
	w.OnEvent(Event{Kind: EventClick, X: 10, Y: centreY - rowH})
	if got := w.SelectedIndex(0); got != 1 {
		t.Fatalf("tap above: got %d want 1", got)
	}
}

func TestWheelPickerTapOnBandDoesNothing(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	rowH := w.rowHeight()
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: rowH * 5})
	centreY := (rowH * 5) / 2
	w.OnEvent(Event{Kind: EventClick, X: 10, Y: centreY}) // dead centre
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("tap on band changed selection")
	}
}

func TestWheelPickerTapOutsideIgnored(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 140})
	w.OnEvent(Event{Kind: EventClick, X: -5, Y: 10})
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("out-of-bounds tap changed selection")
	}
}

// --- Programmatic API + accessors ------------------------------------------

func TestWheelPickerSetIndexClampsAndFires(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	var last [2]int
	n := 0
	w.OnChange = func(col, idx int) { last = [2]int{col, idx}; n++ }

	w.SetIndex(0, 5)
	if w.SelectedIndex(0) != 5 || last != [2]int{0, 5} || n != 1 {
		t.Fatalf("SetIndex(5): idx=%d last=%v n=%d", w.SelectedIndex(0), last, n)
	}
	w.SetIndex(0, 100) // clamps to 11
	if w.SelectedIndex(0) != 11 {
		t.Fatalf("SetIndex(100) clamp: got %d want 11", w.SelectedIndex(0))
	}
	w.SetIndex(0, -3) // clamps to 0
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("SetIndex(-3) clamp: got %d want 0", w.SelectedIndex(0))
	}
	// Re-setting the same index fires nothing.
	before := n
	w.SetIndex(0, 0)
	if n != before {
		t.Fatalf("no-op SetIndex fired OnChange")
	}
	// Out-of-range column: no-op, no panic.
	w.SetIndex(9, 0)
}

func TestWheelPickerSetIndexHaltsMotion(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.columns[0].mom.SetBounds(0, w.columns[0].maxOffset())
	w.columns[0].mom.Fling(3) // start a coast
	if !w.Settling() {
		t.Fatalf("precondition: expected a live coast")
	}
	w.SetIndex(0, 6)
	if w.Settling() {
		t.Fatalf("SetIndex did not halt the coast")
	}
	if w.SelectedIndex(0) != 6 || w.columns[0].offsetRows() != 6 {
		t.Fatalf("SetIndex landed wrong: idx=%d off=%v", w.SelectedIndex(0), w.columns[0].offsetRows())
	}
}

func TestWheelPickerSelectedValue(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetIndex(0, 3)
	if v := w.SelectedValue(0); v != "Apr" {
		t.Fatalf("SelectedValue: got %q want Apr", v)
	}
	if v := w.SelectedValue(9); v != "" {
		t.Fatalf("out-of-range SelectedValue: got %q want empty", v)
	}
	if i := w.SelectedIndex(-1); i != -1 {
		t.Fatalf("out-of-range SelectedIndex: got %d want -1", i)
	}
}

func TestWheelPickerFocusAccessors(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"a"}, []string{"b"})
	w.SetFocus(1)
	if w.Focus() != 1 {
		t.Fatalf("SetFocus(1): got %d", w.Focus())
	}
	w.SetFocus(5) // ignored
	if w.Focus() != 1 {
		t.Fatalf("SetFocus(5) should be ignored, got %d", w.Focus())
	}
	w.SetFocus(-1) // ignored
	if w.Focus() != 1 {
		t.Fatalf("SetFocus(-1) should be ignored, got %d", w.Focus())
	}
}

func TestWheelPickerStepOnEmptyWheel(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker() // no columns
	w.Step(1)             // focus 0 out of range: no-op, no panic
	w.SetIndex(0, 0)      // no-op
	if w.NumColumns() != 0 {
		t.Fatalf("empty wheel NumColumns: got %d", w.NumColumns())
	}
	if w.SelectedValue(0) != "" || w.SelectedIndex(0) != -1 {
		t.Fatalf("empty wheel accessors misbehaved")
	}
}

// --- Degenerate columns ----------------------------------------------------

func TestWheelPickerSingleAndEmptyColumns(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker([]string{"only"}, []string{})
	// Single-item column cannot spin: maxOffset 0.
	if mo := w.columns[0].maxOffset(); mo != 0 {
		t.Fatalf("single-item maxOffset: got %v want 0", mo)
	}
	w.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if w.SelectedIndex(0) != 0 {
		t.Fatalf("single-item column moved: %d", w.SelectedIndex(0))
	}
	// Empty column: index math clamps to 0, value is empty.
	if w.columns[1].indexAt() != 0 {
		t.Fatalf("empty column indexAt: got %d want 0", w.columns[1].indexAt())
	}
	if w.SelectedValue(1) != "" {
		t.Fatalf("empty column value not empty")
	}
	w.SetIndex(1, 0) // no items: no-op
	w.SetFocus(1)
	w.OnEvent(Event{Kind: EventKeyDown, Code: "End"}) // count()-1 = -1, clamps
	if w.SelectedIndex(1) != 0 {
		t.Fatalf("empty column End: got %d", w.SelectedIndex(1))
	}
}

func TestWheelPickerVisibleRowsCoercedOdd(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	if w.visibleRows() != wheelVisibleRows {
		t.Fatalf("default visibleRows: got %d", w.visibleRows())
	}
	w.VisibleRows = 4
	if w.visibleRows() != 5 {
		t.Fatalf("even coerced up: got %d want 5", w.visibleRows())
	}
	w.VisibleRows = 0
	if w.visibleRows() != 1 {
		t.Fatalf("zero coerced to 1: got %d", w.visibleRows())
	}
	w.VisibleRows = 3
	if w.visibleRows() != 3 {
		t.Fatalf("odd kept: got %d", w.visibleRows())
	}
}

// --- Density / HiDPI sizing ------------------------------------------------

func TestWheelPickerRowHeightByDensity(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	cases := []struct {
		d    DensityLevel
		want int
	}{
		{DensityCompact, 28},     // scaled(28)=28, no hit floor
		{DensityComfortable, 36}, // scaled(28)=35 clamped up to the 36 floor
		{DensityTouch, 44},       // scaled(28)=42 clamped up to the 44 finger floor
	}
	for _, tc := range cases {
		SetDensity(tc.d)
		if got := w.rowHeight(); got != tc.want {
			t.Fatalf("density %v rowHeight: got %d want %d", tc.d, got, tc.want)
		}
	}
	SetDensity(DensityCompact)
	SetMetricScale(2)
	if got := w.rowHeight(); got != 56 { // pure HiDPI: 28*2
		t.Fatalf("metricScale 2 rowHeight: got %d want 56", got)
	}
}

// --- Drawing ---------------------------------------------------------------

func TestWheelPickerDrawStaysInBounds(t *testing.T) {
	resetMetrics(t)
	const W, H = 96, 200
	// Guard rows of untouched pixels around the widget: any write outside the
	// widget's own rect flips a guard byte.
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	th := DefaultLight()

	w := NewWheelPicker([]string{"a", "b"}, months())
	inner := Rect{X: 8, Y: 8, W: 80, H: 180}
	w.SetBounds(inner)
	w.SetIndex(1, 6) // scroll a column so rows fan above and below
	w.Draw(p, th)

	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			in := x >= inner.X && x < inner.X+inner.W && y >= inner.Y && y < inner.Y+inner.H
			if in {
				continue
			}
			off := (y*W + x) * 4
			if buf[off] != 0 || buf[off+1] != 0 || buf[off+2] != 0 || buf[off+3] != 0 {
				t.Fatalf("pixel painted outside bounds at (%d,%d)", x, y)
			}
		}
	}
}

func TestWheelPickerDrawVariants(t *testing.T) {
	resetMetrics(t)
	const W, H = 60, 160
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	th := DefaultDark()

	// Single column, scrolled to the top edge (rows above index 0 are skipped).
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, th)

	// Scrolled to the bottom edge (rows past the last are skipped).
	w.SetIndex(0, 11)
	w.Draw(p, th)

	// Disabled face.
	w.Disabled().Set(true)
	w.Draw(p, th)

	// Empty wheel: draws only its frame, no columns, no separators.
	empty := NewWheelPicker()
	empty.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	empty.Draw(p, th)

	// Reaching here without a panic and with something painted is the check.
	painted := false
	for _, b := range buf {
		if b != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatalf("Draw painted nothing")
	}
}

// --- Accessibility ---------------------------------------------------------

func TestWheelPickerA11y(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(
		[]string{"07", "08", "09"},
		[]string{"00", "15", "30", "45"},
	)
	w.SetIndex(0, 2) // "09"
	w.SetIndex(1, 2) // "30"
	info := w.A11y()
	if info.Role != RoleGroup {
		t.Fatalf("role: got %v want %v", info.Role, RoleGroup)
	}
	if info.Value != "09 30" {
		t.Fatalf("value: got %q want %q", info.Value, "09 30")
	}
	// Flows through the shared collector.
	got := CollectA11y([]Widget{w})
	if len(got) != 1 || got[0].Value != "09 30" {
		t.Fatalf("CollectA11y: got %+v", got)
	}
}

// --- Multi-column independence ---------------------------------------------

func TestWheelPickerColumnsAreIndependent(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months(), months(), months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 140})
	w.SetIndex(0, 1)
	w.SetIndex(2, 5)
	if w.SelectedIndex(0) != 1 || w.SelectedIndex(1) != 0 || w.SelectedIndex(2) != 5 {
		t.Fatalf("columns not independent: %d %d %d",
			w.SelectedIndex(0), w.SelectedIndex(1), w.SelectedIndex(2))
	}
	// columnAt splits the width into thirds.
	if c := w.columnAt(5); c != 0 {
		t.Fatalf("columnAt(5): got %d want 0", c)
	}
	if c := w.columnAt(45); c != 1 {
		t.Fatalf("columnAt(45): got %d want 1", c)
	}
	if c := w.columnAt(85); c != 2 {
		t.Fatalf("columnAt(85): got %d want 2", c)
	}
	if c := w.columnAt(-1); c != -1 {
		t.Fatalf("columnAt(-1): got %d want -1", c)
	}
	if c := w.columnAt(90); c != -1 {
		t.Fatalf("columnAt(90): got %d want -1", c)
	}
	empty := NewWheelPicker()
	if c := empty.columnAt(0); c != -1 {
		t.Fatalf("columnAt on empty: got %d want -1", c)
	}
}

func TestWheelPickerApplyBoundsSkippedWhileSnapping(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	c := w.columns[0]
	// Mid-snap the bounds are the degenerate detent span; applyBounds must leave
	// them alone so the spring in flight is not disturbed.
	c.snapping = true
	c.mom.SetBounds(4, 4)
	c.applyBounds()
	if lo, hi := c.mom.Bounds(); lo != 4 || hi != 4 {
		t.Fatalf("applyBounds disturbed a snap: bounds=[%v,%v]", lo, hi)
	}
	c.snapping = false
	c.applyBounds()
	if lo, hi := c.mom.Bounds(); lo != 0 || hi != 11 {
		t.Fatalf("applyBounds did not restore real bounds: [%v,%v]", lo, hi)
	}
}

func TestWheelPickerNotSettlingDuringActiveDrag(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	w.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 140})
	rowH := w.rowHeight()
	w.TouchDown(Event{Kind: EventTouchStart, X: 10, Y: 70})
	// Drag a fractional (off-detent) amount and keep the finger DOWN.
	w.TouchMove(Event{Kind: EventTouchMove, X: 10, Y: 70 - rowH/2}, 1.0/60.0)
	if w.columns[0].offsetRows() == w.columns[0].nearestDetent() {
		t.Fatalf("precondition: expected an off-detent offset under the finger")
	}
	if w.Settling() {
		t.Fatalf("Settling reported true while a finger is still dragging")
	}
}

func TestWheelPickerTickAtRestIsNoop(t *testing.T) {
	resetMetrics(t)
	w := NewWheelPicker(months())
	if w.Tick(1.0 / 60.0) {
		t.Fatalf("Tick at rest reported motion")
	}
	if w.Tick(-1) {
		t.Fatalf("Tick with non-positive dt reported motion")
	}
}
