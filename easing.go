// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Easing maps a normalized time t in [0, 1] to an eased progress value,
// typically also in [0, 1] (some easings may overshoot before settling,
// though none of the named easings below do). Implementations should treat
// t outside [0, 1] as clamped to the nearest bound.
type Easing func(t float64) float64

// clampUnit clamps t to the [0, 1] range.
func clampUnit(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// Linear returns t unchanged: constant velocity from start to end.
func Linear(t float64) float64 {
	return clampUnit(t)
}

// EaseInQuad starts slow and accelerates towards the end (t^2).
func EaseInQuad(t float64) float64 {
	t = clampUnit(t)
	return t * t
}

// EaseOutQuad starts fast and decelerates towards the end (1-(1-t)^2).
func EaseOutQuad(t float64) float64 {
	t = clampUnit(t)
	u := 1 - t
	return 1 - u*u
}

// EaseInOutQuad accelerates through the first half and decelerates through
// the second half, using EaseInQuad then EaseOutQuad symmetrically.
func EaseInOutQuad(t float64) float64 {
	t = clampUnit(t)
	if t < 0.5 {
		return 2 * t * t
	}
	u := -2*t + 2
	return 1 - u*u/2
}

// EaseInCubic starts slow and accelerates towards the end (t^3), more
// pronounced than EaseInQuad.
func EaseInCubic(t float64) float64 {
	t = clampUnit(t)
	return t * t * t
}

// EaseOutCubic starts fast and decelerates towards the end (1-(1-t)^3),
// more pronounced than EaseOutQuad.
func EaseOutCubic(t float64) float64 {
	t = clampUnit(t)
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOutCubic accelerates through the first half and decelerates
// through the second half, using EaseInCubic then EaseOutCubic
// symmetrically, more pronounced than EaseInOutQuad.
func EaseInOutCubic(t float64) float64 {
	t = clampUnit(t)
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// Tween animates a scalar value from From to To over Duration ticks, using
// Ease to shape the progress curve. Advance it once per frame/tick via
// Tick, or read the current value without advancing via Value.
type Tween struct {
	// From is the starting value.
	From float64
	// To is the ending value.
	To float64
	// Duration is the total number of ticks the tween takes to complete.
	// A Duration <= 0 makes the tween immediately Done, at To.
	Duration int
	// Ease shapes the progress curve. A nil Ease behaves as Linear.
	Ease Easing

	elapsed int
}

// NewTween creates a Tween animating from from to to over duration ticks
// using ease. A nil ease defaults to Linear. A duration <= 0 produces a
// Tween that is immediately Done, reporting to as its Value.
func NewTween(from, to float64, duration int, ease Easing) *Tween {
	if ease == nil {
		ease = Linear
	}
	return &Tween{
		From:     from,
		To:       to,
		Duration: duration,
		Ease:     ease,
	}
}

// Tick advances the tween by one tick, clamping elapsed progress at
// Duration, and returns the current eased value between From and To.
func (tw *Tween) Tick() float64 {
	if tw.Duration > 0 && tw.elapsed < tw.Duration {
		tw.elapsed++
	}
	return tw.Value()
}

// Value returns the current eased value between From and To without
// advancing the tween.
func (tw *Tween) Value() float64 {
	if tw.Duration <= 0 {
		return tw.To
	}
	ease := tw.Ease
	if ease == nil {
		ease = Linear
	}
	t := float64(tw.elapsed) / float64(tw.Duration)
	progress := ease(t)
	return tw.From + (tw.To-tw.From)*progress
}

// Done reports whether the tween has reached its Duration.
func (tw *Tween) Done() bool {
	return tw.elapsed >= tw.Duration
}

// Reset restarts the tween from its beginning (elapsed = 0).
func (tw *Tween) Reset() {
	tw.elapsed = 0
}
