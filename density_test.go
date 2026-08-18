// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// restoreDensity resets the two globals this axis touches so a failing case can
// never leak a non-default density/scale into the rest of the suite (which
// asserts the compact, 1x baseline).
func restoreDensity() {
	SetMetricScale(1)
	SetDensity(DensityCompact)
}

// TestDensityDefaultIsCompact pins the default: the zero value is DensityCompact,
// its factor is EXACTLY 1.0, and its hit floor is 0 — the byte-identical baseline
// every other test restores to.
func TestDensityDefaultIsCompact(t *testing.T) {
	defer restoreDensity()
	if got := Density(); got != DensityCompact {
		t.Fatalf("default Density() = %v, want DensityCompact", got)
	}
	if f := densityFactor(DensityCompact); f != 1.0 {
		t.Fatalf("densityFactor(DensityCompact) = %v, want exactly 1.0", f)
	}
	if m := minHitLogical(DensityCompact); m != 0 {
		t.Fatalf("minHitLogical(DensityCompact) = %d, want 0", m)
	}
}

// TestDensityFactorsExact asserts the exact multiplier chosen for each level —
// not "bigger", the precise value the whole toolkit multiplies by.
func TestDensityFactorsExact(t *testing.T) {
	for _, tc := range []struct {
		d    DensityLevel
		want float64
	}{
		{DensityCompact, 1.0},
		{DensityComfortable, 1.25},
		{DensityTouch, 1.5},
	} {
		if got := densityFactor(tc.d); got != tc.want {
			t.Errorf("densityFactor(%v) = %v, want %v", tc.d, got, tc.want)
		}
	}
}

// TestMinHitLogicalExact asserts the exact logical-pixel floor for each level.
func TestMinHitLogicalExact(t *testing.T) {
	for _, tc := range []struct {
		d    DensityLevel
		want int
	}{
		{DensityCompact, 0},
		{DensityComfortable, 36},
		{DensityTouch, 44},
	} {
		if got := minHitLogical(tc.d); got != tc.want {
			t.Errorf("minHitLogical(%v) = %d, want %d", tc.d, got, tc.want)
		}
	}
}

// TestSetDensityValidAndClamped covers both SetDensity branches: a valid level is
// stored; an out-of-range level (below compact and above touch) is ignored so the
// global never lands on an undefined value.
func TestSetDensityValidAndClamped(t *testing.T) {
	defer restoreDensity()

	SetDensity(DensityComfortable)
	if Density() != DensityComfortable {
		t.Fatalf("after SetDensity(Comfortable), Density() = %v", Density())
	}
	SetDensity(DensityTouch)
	if Density() != DensityTouch {
		t.Fatalf("after SetDensity(Touch), Density() = %v", Density())
	}

	// Below the range: ignored, keeps Touch.
	SetDensity(DensityCompact - 1)
	if Density() != DensityTouch {
		t.Fatalf("SetDensity(<compact) changed density to %v, want kept Touch", Density())
	}
	// Above the range: ignored, keeps Touch.
	SetDensity(DensityTouch + 1)
	if Density() != DensityTouch {
		t.Fatalf("SetDensity(>touch) changed density to %v, want kept Touch", Density())
	}

	// The valid low end still lands.
	SetDensity(DensityCompact)
	if Density() != DensityCompact {
		t.Fatalf("after SetDensity(Compact), Density() = %v", Density())
	}
}

// TestScaledCompactIsByteIdentical is the backward-compatibility proof: under the
// default DensityCompact, scaled(x) equals the pre-density formula round(x*scale)
// for a representative sweep, at both 1x and a non-1.0 HiDPI scale. If this ever
// diverges, every host that never set a density has silently shifted.
func TestScaledCompactIsByteIdentical(t *testing.T) {
	defer restoreDensity()

	// The exact values the density-less toolkit produced at 1x: the identity.
	xs := []int{0, 1, 2, 3, 4, 5, 8, 10, 12, 16, 24, 44, 100}
	for _, x := range xs {
		if got := scaled(x); got != x {
			t.Fatalf("compact scaled(%d) at 1x = %d, want %d (byte-identical identity)", x, got, x)
		}
	}

	// Under HiDPI the compact path must equal the pure round(x*scale), untouched
	// by any density factor.
	SetMetricScale(1.5)
	for _, x := range xs {
		want := int(float64(x)*1.5 + 0.5)
		if got := scaled(x); got != want {
			t.Fatalf("compact scaled(%d) at 1.5x = %d, want %d (pure HiDPI round)", x, got, want)
		}
	}
}

// TestScaledComposesDensityWithDPI asserts the exact composed metric at each
// density, alone and multiplied with a non-1.0 HiDPI scale.
func TestScaledComposesDensityWithDPI(t *testing.T) {
	defer restoreDensity()

	// density alone, MetricScale 1: base × factor, rounded.
	cases := []struct {
		d          DensityLevel
		base, want int
	}{
		{DensityCompact, 10, 10},     // 10 × 1.0
		{DensityComfortable, 10, 13}, // 10 × 1.25 = 12.5 → 13
		{DensityComfortable, 8, 10},  // 8 × 1.25 = 10.0 → 10
		{DensityTouch, 10, 15},       // 10 × 1.5 = 15
		{DensityTouch, 3, 5},         // 3 × 1.5 = 4.5 → 5
	}
	for _, tc := range cases {
		SetDensity(tc.d)
		if got := scaled(tc.base); got != tc.want {
			t.Errorf("scaled(%d) at %v, 1x = %d, want %d", tc.base, tc.d, got, tc.want)
		}
	}

	// Composed with MetricScale 2: base × 2 × factor.
	SetMetricScale(2)
	comp := []struct {
		d          DensityLevel
		base, want int
	}{
		{DensityCompact, 10, 20},     // 10 × 2 × 1.0
		{DensityComfortable, 10, 25}, // 10 × 2 × 1.25
		{DensityTouch, 10, 30},       // 10 × 2 × 1.5
	}
	for _, tc := range comp {
		SetDensity(tc.d)
		if got := scaled(tc.base); got != tc.want {
			t.Errorf("scaled(%d) at %v, 2x = %d, want %d", tc.base, tc.d, got, tc.want)
		}
	}
}

// TestMinHitTargetExact asserts the exact device-pixel floor at each density and
// its composition with HiDPI: the floor scales with MetricScale ONLY, never with
// the density factor. 44 logical → 44 at 1x, 88 at 2x.
func TestMinHitTargetExact(t *testing.T) {
	defer restoreDensity()

	for _, tc := range []struct {
		d    DensityLevel
		want int
	}{
		{DensityCompact, 0},
		{DensityComfortable, 36},
		{DensityTouch, 44},
	} {
		SetDensity(tc.d)
		if got := MinHitTarget(); got != tc.want {
			t.Errorf("MinHitTarget() at %v, 1x = %d, want %d", tc.d, got, tc.want)
		}
	}

	SetMetricScale(2)
	for _, tc := range []struct {
		d    DensityLevel
		want int
	}{
		{DensityCompact, 0},      // no floor scales to no floor
		{DensityComfortable, 72}, // 36 × 2
		{DensityTouch, 88},       // 44 × 2 — the finger floor at Retina
	} {
		SetDensity(tc.d)
		if got := MinHitTarget(); got != tc.want {
			t.Errorf("MinHitTarget() at %v, 2x = %d, want %d", tc.d, got, tc.want)
		}
	}
}

// TestTouchTargetClamp asserts the exact clamp at, below, and above the minimum,
// for every density and composed with HiDPI. Compact is a pure pass-through
// (30→30); touch clamps 30→44 and passes 60→60; at 2x the floor is 88.
func TestTouchTargetClamp(t *testing.T) {
	defer restoreDensity()

	// Compact: no floor, everything passes through unchanged (both branches: the
	// "already >= min" branch, with min 0 nothing is ever below it).
	SetDensity(DensityCompact)
	for _, in := range []int{0, 1, 30, 44, 60, 1000} {
		if got := TouchTarget(in); got != in {
			t.Errorf("compact TouchTarget(%d) = %d, want %d (pass-through)", in, got, in)
		}
	}

	// Touch, 1x, floor 44: below clamps up, at stays, above stays.
	SetDensity(DensityTouch)
	for _, tc := range []struct{ in, want int }{
		{0, 44},  // far below → floor
		{30, 44}, // below → floor (the spec example)
		{43, 44}, // one below → floor
		{44, 44}, // exactly at → unchanged
		{45, 45}, // one above → unchanged
		{60, 60}, // above → unchanged (the spec example)
	} {
		if got := TouchTarget(tc.in); got != tc.want {
			t.Errorf("touch TouchTarget(%d) at 1x = %d, want %d", tc.in, got, tc.want)
		}
	}

	// Comfortable, 1x, floor 36 (the in-between level).
	SetDensity(DensityComfortable)
	for _, tc := range []struct{ in, want int }{
		{30, 36}, // below → floor
		{36, 36}, // at → unchanged
		{40, 40}, // above → unchanged
	} {
		if got := TouchTarget(tc.in); got != tc.want {
			t.Errorf("comfortable TouchTarget(%d) at 1x = %d, want %d", tc.in, got, tc.want)
		}
	}

	// Touch composed with MetricScale 2: floor 88.
	SetMetricScale(2)
	SetDensity(DensityTouch)
	for _, tc := range []struct{ in, want int }{
		{30, 88},   // below the doubled floor → 88
		{88, 88},   // at → unchanged
		{100, 100}, // above → unchanged
	} {
		if got := TouchTarget(tc.in); got != tc.want {
			t.Errorf("touch TouchTarget(%d) at 2x = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestSwitchHitRectDemonstratesTouchTarget is the worked-widget proof: under the
// default compact density the switch's hit rect equals its drawn bounds
// byte-for-byte (no clamp path taken); under touch it grows to the 44px floor on
// each axis, centred over the unchanged bounds, while Draw is not involved.
func TestSwitchHitRectDemonstratesTouchTarget(t *testing.T) {
	defer restoreDensity()

	s := NewSwitch(false)
	b := Rect{X: 100, Y: 200, W: 30, H: 16}
	s.SetBounds(b)

	// Compact: identical to Bounds, proving the pass-through leaves a widget's
	// interactive geometry unchanged on the desktop.
	if got := s.HitRect(); got != b {
		t.Fatalf("compact HitRect = %+v, want the drawn bounds %+v", got, b)
	}

	// Touch: each axis clamps up to 44 and the rect is centred over the bounds.
	SetDensity(DensityTouch)
	want := Rect{
		X: 100 - (44-30)/2, // 100 - 7 = 93
		Y: 200 - (44-16)/2, // 200 - 14 = 186
		W: 44,
		H: 44,
	}
	if got := s.HitRect(); got != want {
		t.Fatalf("touch HitRect = %+v, want %+v (clamped + centred)", got, want)
	}
	// The centred hit rect still covers the original bounds on both axes.
	if want.X > b.X || want.X+want.W < b.X+b.W || want.Y > b.Y || want.Y+want.H < b.Y+b.H {
		t.Fatalf("touch HitRect %+v does not fully cover bounds %+v", want, b)
	}
	// A switch already larger than the floor keeps its own size (the pass-through
	// branch inside HitRect via TouchTarget).
	big := Rect{X: 0, Y: 0, W: 60, H: 60}
	s.SetBounds(big)
	if got := s.HitRect(); got != big {
		t.Fatalf("touch HitRect for a 60x60 switch = %+v, want unchanged %+v", got, big)
	}
}

// TestDensityDoesNotDisturbDefaultWidgetMetrics is the widget-level
// byte-identical proof: representative widget metrics derived through scaled(...)
// at the default compact density equal the exact values they held before the
// density system existed (factor 1.0 ⇒ round(base×scale) ⇒ base at 1x).
func TestDensityDoesNotDisturbDefaultWidgetMetrics(t *testing.T) {
	defer restoreDensity()

	// switchPad routes through scaled: at compact/1x it is its logical constant.
	if got := scaled(switchPad); got != switchPad {
		t.Errorf("compact scaled(switchPad) = %d, want %d", got, switchPad)
	}
	// The normalized scrollbar gutter: track + gap, unchanged at compact/1x.
	if got := scrollGutter(); got != scrollbarWidth+scrollbarGap {
		t.Errorf("compact scrollGutter() = %d, want %d", got, scrollbarWidth+scrollbarGap)
	}
	if got := scrollbarTrack(); got != scrollbarWidth {
		t.Errorf("compact scrollbarTrack() = %d, want %d", got, scrollbarWidth)
	}
	// Exported seam sibling packages use is likewise pure identity at compact.
	if got := Scaled(21); got != 21 {
		t.Errorf("compact Scaled(21) = %d, want 21", got)
	}
}
