// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestGaugeFrac exercises the clamped fill fraction: a mid value, a
// value below Min (clamps to 0), a value above Max (clamps to 1), and
// the degenerate Max==Min / Max<Min span (no divide-by-zero → 0).
func TestGaugeFrac(t *testing.T) {
	cases := []struct {
		name            string
		min, max, value float64
		want            float64
	}{
		{"mid", 0, 100, 50, 0.5},
		{"below-min-clamps-0", 0, 100, -10, 0},
		{"above-max-clamps-1", 0, 100, 150, 1},
		{"degenerate-equal", 5, 5, 5, 0},
		{"inverted-span", 10, 0, 5, 0},
	}
	for _, tc := range cases {
		g := NewGauge(tc.min, tc.max, tc.value)
		if got := g.frac(); got != tc.want {
			t.Errorf("%s: frac() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestGaugeValueColor covers the four valueColor paths: no bands →
// Accent; Value inside the first band; Value inside a later band; Value
// past every threshold → the last band (fallthrough).
func TestGaugeValueColor(t *testing.T) {
	th := DefaultLight()
	green := RGBA{G: 200, A: 255}
	yellow := RGBA{R: 220, G: 200, A: 255}
	red := RGBA{R: 255, A: 255}

	// No bands → Accent.
	g := NewGauge(0, 100, 40)
	if got := g.valueColor(th); got != th.Accent {
		t.Errorf("no bands: valueColor = %+v, want Accent %+v", got, th.Accent)
	}

	g.Bands = []GaugeBand{{Upto: 50, Color: green}, {Upto: 80, Color: yellow}, {Upto: 100, Color: red}}
	// Value 40 → first band (green).
	if got := g.valueColor(th); got != green {
		t.Errorf("value 40: valueColor = %+v, want green %+v", got, green)
	}
	// Value 70 → middle band (yellow).
	g.Value().Set(70)
	if got := g.valueColor(th); got != yellow {
		t.Errorf("value 70: valueColor = %+v, want yellow %+v", got, yellow)
	}
	// Value 130 → past every Upto → last band (red, fallthrough).
	g.Value().Set(130)
	if got := g.valueColor(th); got != red {
		t.Errorf("value 130: valueColor = %+v, want red %+v (fallthrough)", got, red)
	}
}

// TestGaugeDrawAccentArc: a no-band gauge paints its value arc in Accent
// and the remaining track in SurfaceAlt.
func TestGaugeDrawAccentArc(t *testing.T) {
	th := DefaultLight()
	g := NewGauge(0, 100, 60)
	g.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)

	if got := countInk(surf, 60, 60, th.Accent); got == 0 {
		t.Error("value arc painted 0 Accent pixels, want > 0")
	}
	if got := countInk(surf, 60, 60, th.SurfaceAlt); got == 0 {
		t.Error("track painted 0 SurfaceAlt pixels, want > 0")
	}
}

// TestGaugeDrawBands: with bands set, the whole value arc takes the
// colour of the band Value falls in — not Accent, not the other bands.
func TestGaugeDrawBands(t *testing.T) {
	th := DefaultLight()
	green := RGBA{G: 200, A: 255}
	red := RGBA{R: 255, A: 255}
	g := NewGauge(0, 100, 80)
	g.Bands = []GaugeBand{{Upto: 50, Color: green}, {Upto: 100, Color: red}}
	g.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)

	if got := countInk(surf, 60, 60, red); got == 0 {
		t.Error("value arc painted 0 red pixels, want the matching band colour")
	}
	if got := countInk(surf, 60, 60, green); got != 0 {
		t.Errorf("painted %d green pixels, want 0 (Value is in the red band)", got)
	}
	if got := countInk(surf, 60, 60, th.Accent); got != 0 {
		t.Errorf("painted %d Accent pixels, want 0 (bands override Accent)", got)
	}
}

// TestGaugeZeroValueNoValueArc: a Value at Min (frac 0) paints no value
// pixels, only the SurfaceAlt track.
func TestGaugeZeroValueNoValueArc(t *testing.T) {
	th := DefaultLight()
	g := NewGauge(0, 100, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)

	if got := countInk(surf, 60, 60, th.Accent); got != 0 {
		t.Errorf("zero value painted %d Accent pixels, want 0", got)
	}
	if got := countInk(surf, 60, 60, th.SurfaceAlt); got == 0 {
		t.Error("zero value painted 0 SurfaceAlt track pixels, want > 0")
	}
}

// TestGaugeCaption: a non-empty Caption paints OnSurface ink; an empty
// Caption paints none.
func TestGaugeCaption(t *testing.T) {
	th := DefaultLight()

	g := NewGauge(0, 100, 50)
	g.Caption = "50"
	g.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)
	if got := countInk(surf, 60, 60, th.OnSurface); got == 0 {
		t.Error("caption painted 0 OnSurface pixels, want > 0")
	}

	g2 := NewGauge(0, 100, 50)
	g2.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf2 := makeSurface(60, 60)
	g2.Draw(newP(surf2, 60), th)
	if got := countInk(surf2, 60, 60, th.OnSurface); got != 0 {
		t.Errorf("no caption painted %d OnSurface pixels, want 0", got)
	}
}

// TestGaugeThicknessClampsInner: a Thickness exceeding the radius clamps
// the inner radius to 0 (the arc fills to the centre) without panicking.
func TestGaugeThicknessClampsInner(t *testing.T) {
	th := DefaultLight()
	g := NewGauge(0, 100, 50)
	g.Thickness = 100 // > outer radius (30)
	g.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)
	if got := countInk(surf, 60, 60, th.SurfaceAlt); got == 0 {
		t.Error("thick gauge painted 0 SurfaceAlt pixels, want > 0")
	}
}

// TestGaugeEmptyBoundsNoop: a zero-sized bounds draws nothing and does
// not panic.
func TestGaugeEmptyBoundsNoop(t *testing.T) {
	th := DefaultLight()
	g := NewGauge(0, 100, 50)
	g.SetBounds(Rect{}) // W=H=0
	surf := makeSurface(60, 60)
	g.Draw(newP(surf, 60), th)
	if got := countInk(surf, 60, 60, th.SurfaceAlt); got != 0 {
		t.Errorf("empty bounds painted %d pixels, want 0", got)
	}
}

// TestGaugeTinyRadiusNoop: a sub-pixel bounds (radius 0) bails before
// the fill loop.
func TestGaugeTinyRadiusNoop(t *testing.T) {
	th := DefaultLight()
	g := NewGauge(0, 100, 50)
	g.SetBounds(Rect{X: 0, Y: 0, W: 1, H: 1}) // outer radius 0
	surf := makeSurface(4, 4)
	g.Draw(newP(surf, 4), th)
	if got := countInk(surf, 4, 4, th.SurfaceAlt); got != 0 {
		t.Errorf("radius<1 painted %d pixels, want 0", got)
	}
}

// TestGaugeZeroValueValueDefault covers the Value() accessor's lazy-init path on
// a literal Gauge{} (no NewGauge): the first call materialises the Observable at 0.
func TestGaugeZeroValueValueDefault(t *testing.T) {
	var g Gauge
	if got := g.Value().Get(); got != 0 {
		t.Fatalf("zero-value Gauge Value() = %v, want 0", got)
	}
	g.Value().Set(42)
	if got := g.Value().Get(); got != 42 {
		t.Fatalf("after Set(42), Value() = %v, want 42", got)
	}
}
