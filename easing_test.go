// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

const epsilon = 1e-9

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilon
}

func TestClampUnit(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{-1, 0},
		{-0.0001, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{1.0001, 1},
		{2, 1},
	}
	for _, c := range cases {
		if got := clampUnit(c.in); !almostEqual(got, c.want) {
			t.Errorf("clampUnit(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNamedEasings(t *testing.T) {
	easings := map[string]Easing{
		"Linear":         Linear,
		"EaseInQuad":     EaseInQuad,
		"EaseOutQuad":    EaseOutQuad,
		"EaseInOutQuad":  EaseInOutQuad,
		"EaseInCubic":    EaseInCubic,
		"EaseOutCubic":   EaseOutCubic,
		"EaseInOutCubic": EaseInOutCubic,
	}

	for name, ease := range easings {
		if got := ease(0); !almostEqual(got, 0) {
			t.Errorf("%s(0) = %v, want 0", name, got)
		}
		if got := ease(1); !almostEqual(got, 1) {
			t.Errorf("%s(1) = %v, want 1", name, got)
		}
		// Out-of-range inputs must clamp.
		if got := ease(-1); !almostEqual(got, 0) {
			t.Errorf("%s(-1) = %v, want 0 (clamp)", name, got)
		}
		if got := ease(2); !almostEqual(got, 1) {
			t.Errorf("%s(2) = %v, want 1 (clamp)", name, got)
		}
	}

	midCases := []struct {
		name string
		ease Easing
		want float64
	}{
		{"Linear", Linear, 0.5},
		{"EaseInQuad", EaseInQuad, 0.25},
		{"EaseOutQuad", EaseOutQuad, 0.75},
		{"EaseInOutQuad", EaseInOutQuad, 0.5},
		{"EaseInCubic", EaseInCubic, 0.125},
		{"EaseOutCubic", EaseOutCubic, 0.875},
		{"EaseInOutCubic", EaseInOutCubic, 0.5},
	}
	for _, c := range midCases {
		if got := c.ease(0.5); !almostEqual(got, c.want) {
			t.Errorf("%s(0.5) = %v, want %v", c.name, got, c.want)
		}
	}

	// Exercise the "below half" and "above half" branches of the
	// in-out variants explicitly.
	if got := EaseInOutQuad(0.25); !almostEqual(got, 2*0.25*0.25) {
		t.Errorf("EaseInOutQuad(0.25) = %v, want %v", got, 2*0.25*0.25)
	}
	if got := EaseInOutQuad(0.75); got <= 0.5 || got >= 1 {
		t.Errorf("EaseInOutQuad(0.75) = %v, want value in (0.5, 1)", got)
	}
	if got := EaseInOutCubic(0.25); got <= 0 || got >= 0.5 {
		t.Errorf("EaseInOutCubic(0.25) = %v, want value in (0, 0.5)", got)
	}
	if got := EaseInOutCubic(0.75); got <= 0.5 || got >= 1 {
		t.Errorf("EaseInOutCubic(0.75) = %v, want value in (0.5, 1)", got)
	}
}

func TestNewTweenDefaultsEaseToLinear(t *testing.T) {
	tw := NewTween(0, 10, 4, nil)
	if tw.Ease == nil {
		t.Fatal("NewTween with nil ease should default Ease to Linear")
	}
	if got := tw.Value(); !almostEqual(got, 0) {
		t.Errorf("Value() at start = %v, want 0", got)
	}
	for i := 0; i < 4; i++ {
		tw.Tick()
	}
	if !tw.Done() {
		t.Error("expected tween to be Done after Duration ticks")
	}
	if got := tw.Value(); !almostEqual(got, 10) {
		t.Errorf("Value() at end = %v, want 10", got)
	}
}

func TestTweenProgression(t *testing.T) {
	tw := NewTween(0, 10, 4, Linear)
	if tw.Done() {
		t.Error("tween should not be Done before any ticks")
	}
	want := []float64{2.5, 5, 7.5, 10}
	for i, w := range want {
		got := tw.Tick()
		if !almostEqual(got, w) {
			t.Errorf("Tick() #%d = %v, want %v", i, got, w)
		}
	}
	if !tw.Done() {
		t.Error("expected tween to be Done after 4 ticks of duration 4")
	}
}

func TestTweenClampsPastDuration(t *testing.T) {
	tw := NewTween(0, 10, 2, Linear)
	tw.Tick()
	tw.Tick()
	if !tw.Done() {
		t.Fatal("expected Done after reaching duration")
	}
	// Additional ticks beyond Duration must clamp, not overshoot.
	for i := 0; i < 3; i++ {
		got := tw.Tick()
		if !almostEqual(got, 10) {
			t.Errorf("Tick() beyond duration = %v, want 10", got)
		}
	}
}

func TestTweenDurationZeroOrNegativeImmediatelyDone(t *testing.T) {
	for _, d := range []int{0, -1, -5} {
		tw := NewTween(3, 9, d, Linear)
		if !tw.Done() {
			t.Errorf("duration %d: expected immediately Done", d)
		}
		if got := tw.Value(); !almostEqual(got, 9) {
			t.Errorf("duration %d: Value() = %v, want To=9", d, got)
		}
		// Ticking further should stay at To and remain Done.
		got := tw.Tick()
		if !almostEqual(got, 9) {
			t.Errorf("duration %d: Tick() = %v, want To=9", d, got)
		}
		if !tw.Done() {
			t.Errorf("duration %d: expected still Done after Tick", d)
		}
	}
}

func TestTweenValueVsTick(t *testing.T) {
	tw := NewTween(0, 100, 10, EaseInQuad)
	before := tw.Value()
	if !almostEqual(before, 0) {
		t.Errorf("Value() before ticking = %v, want 0", before)
	}
	// Value must not advance state.
	tw.Value()
	tw.Value()
	if tw.elapsed != 0 {
		t.Errorf("Value() must not mutate elapsed, got %d", tw.elapsed)
	}
	tw.Tick()
	if tw.elapsed != 1 {
		t.Errorf("Tick() should advance elapsed to 1, got %d", tw.elapsed)
	}
	got := tw.Value()
	want := EaseInQuad(0.1) * 100
	if !almostEqual(got, want) {
		t.Errorf("Value() after one tick = %v, want %v", got, want)
	}
}

func TestTweenReset(t *testing.T) {
	tw := NewTween(0, 10, 4, Linear)
	tw.Tick()
	tw.Tick()
	if tw.elapsed == 0 {
		t.Fatal("expected elapsed to have advanced")
	}
	tw.Reset()
	if tw.elapsed != 0 {
		t.Errorf("Reset() should zero elapsed, got %d", tw.elapsed)
	}
	if got := tw.Value(); !almostEqual(got, 0) {
		t.Errorf("Value() after Reset() = %v, want From=0", got)
	}
	if tw.Done() {
		t.Error("tween should not be Done immediately after Reset (duration > 0)")
	}
}

func TestTweenValueWithNilEaseField(t *testing.T) {
	// Simulate a Tween constructed as a struct literal (bypassing
	// NewTween), so Ease is left nil; Value must still default to
	// Linear rather than panicking.
	tw := &Tween{From: 0, To: 10, Duration: 4}
	tw.Tick()
	got := tw.Value()
	want := 2.5
	if !almostEqual(got, want) {
		t.Errorf("Value() with nil Ease = %v, want %v (linear default)", got, want)
	}
}
