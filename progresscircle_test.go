// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constants -----------------------------------------------------------

func TestProgressCircleConstants(t *testing.T) {
	if ProgressCircleSize != 40 {
		t.Fatalf("ProgressCircleSize = %d, want 40", ProgressCircleSize)
	}
	if ProgressCircleStroke != 4 {
		t.Fatalf("ProgressCircleStroke = %d, want 4", ProgressCircleStroke)
	}
}

// --- Constructor ---------------------------------------------------------

func TestNewProgressCircleDefaults(t *testing.T) {
	pc := NewProgressCircle()
	if pc.Fraction().Get() != 0 {
		t.Fatalf("Fraction default = %v, want 0", pc.Fraction().Get())
	}
	if pc.Bounds() != (Rect{}) {
		t.Fatalf("Bounds default = %+v, want zero", pc.Bounds())
	}
}

// --- Fraction Observable (lazy-init + host binding) ----------------------

// TestProgressCircleFractionObservable covers the zero-value lazy-init of the
// Fraction accessor and the host binding path: a ProgressCircle built as a bare
// struct (no NewProgressCircle) still yields a usable Observable, Setting it
// from outside is reflected by Get and by a subscriber, and a subsequent Draw
// paints the band at the Set value (there is no imperative Fraction field).
func TestProgressCircleFractionObservable(t *testing.T) {
	pc := &ProgressCircle{} // no NewProgressCircle → fraction Observable is nil until accessed
	if pc.Fraction().Get() != 0 {
		t.Fatalf("zero-value Fraction = %v, want 0", pc.Fraction().Get())
	}
	// A second access returns the same (already-initialised) Observable.
	if pc.Fraction() != pc.Fraction() {
		t.Fatal("Fraction() returned different Observables across calls")
	}
	seen := -1.0
	pc.Fraction().Subscribe(func(v float64) { seen = v })
	pc.Fraction().Set(1) // a host drives the widget through the Observable
	if pc.Fraction().Get() != 1 || seen != 1 {
		t.Fatalf("host Set: value=%v subscriber=%v, want 1/1", pc.Fraction().Get(), seen)
	}

	// Draw reflects the host Set: the left ring column at mid-height is Accent.
	const w, h = 60, 60
	theme := DefaultLight()
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 4+1, 4+20) != theme.Accent {
		t.Fatalf("Draw did not reflect Fraction Set; left ring mid = %+v",
			pixelAt(buf, w, 4+1, 4+20))
	}
}

// --- Draw at zero, half, full --------------------------------------------

// At Fraction=0, no Accent-band pixels appear anywhere on the surface.
func TestProgressCircleDrawFractionZeroNoAccent(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)

	// Track pixel (top of ring) is SurfaceAlt.
	if pixelAt(buf, w, 4, 4) != theme.SurfaceAlt {
		t.Fatalf("track pixel = %+v, want SurfaceAlt", pixelAt(buf, w, 4, 4))
	}
	// Inner hole is Surface.
	if pixelAt(buf, w, 4+ProgressCircleStroke+2, 4+ProgressCircleStroke+2) != theme.Surface {
		t.Fatalf("inner hole not Surface at (%d,%d): %+v",
			4+ProgressCircleStroke+2, 4+ProgressCircleStroke+2,
			pixelAt(buf, w, 4+ProgressCircleStroke+2, 4+ProgressCircleStroke+2))
	}
	// No Accent pixels anywhere (bandH == 0).
	for y := 4; y < 44; y++ {
		for x := 4; x < 44; x++ {
			if pixelAt(buf, w, x, y) == theme.Accent {
				t.Fatalf("Accent ink at (%d,%d) with Fraction=0", x, y)
			}
		}
	}
}

// At Fraction=1, the entire ring shows Accent on the ring columns.
func TestProgressCircleDrawFractionOneFillsBand(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(1)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)

	// A pixel in the left ring column near the middle should be Accent.
	mid := 4 + 20
	if pixelAt(buf, w, 4+1, mid) != theme.Accent {
		t.Fatalf("left ring column at mid = %+v, want Accent",
			pixelAt(buf, w, 4+1, mid))
	}
	// The right ring column too.
	if pixelAt(buf, w, 4+40-2, mid) != theme.Accent {
		t.Fatalf("right ring column at mid = %+v, want Accent",
			pixelAt(buf, w, 4+40-2, mid))
	}
}

// At Fraction=0.5, band grows from the bottom; the bottom half of
// the ring shows Accent, the top half does not.
func TestProgressCircleDrawFractionHalfGrowsFromBottom(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(0.5)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)

	// Top of ring interior (y just below the stroke) should NOT be Accent.
	topY := 4 + ProgressCircleStroke + 1
	sawAccentTop := false
	for x := 4; x < 4+ProgressCircleStroke; x++ {
		if pixelAt(buf, w, x, topY) == theme.Accent {
			sawAccentTop = true
			break
		}
	}
	if sawAccentTop {
		t.Fatal("top of ring interior painted Accent — band should grow from bottom")
	}

	// Bottom of ring interior should be Accent.
	botY := 4 + 40 - ProgressCircleStroke - 1
	sawAccentBottom := false
	for x := 4; x < 4+ProgressCircleStroke; x++ {
		if pixelAt(buf, w, x, botY) == theme.Accent {
			sawAccentBottom = true
			break
		}
	}
	if !sawAccentBottom {
		t.Fatal("bottom of ring interior not painted Accent")
	}
}

// --- Draw out-of-range clamp inside Draw ---------------------------------

// Draw clamps Fraction defensively so an out-of-range Set still
// renders safely. Cover both branches.
func TestProgressCircleDrawClampInDraw(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(-1)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	pc.Draw(newP(makeSurface(w, h), w), theme)
	pc.Fraction().Set(2)
	pc.Draw(newP(makeSurface(w, h), w), theme)
}

// --- Draw caption --------------------------------------------------------

// OnSurface caption ink lands somewhere inside the ring's hole.
func TestProgressCircleDrawCaptionInsideHole(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(0.5)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)

	holeMinX := 4 + ProgressCircleStroke
	holeMinY := 4 + ProgressCircleStroke
	holeMaxX := 4 + 40 - ProgressCircleStroke
	holeMaxY := 4 + 40 - ProgressCircleStroke
	found := false
	for y := holeMinY; y < holeMaxY && !found; y++ {
		for x := holeMinX; x < holeMaxX && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no OnSurface caption ink found inside the ring hole")
	}
}

// --- Dark theme ----------------------------------------------------------

// Verify Draw works with the dark palette (track = SurfaceAlt-dark).
func TestProgressCircleDrawDarkTheme(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultDark()
	pc := NewProgressCircle()
	pc.Fraction().Set(0.75)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)
	// Track corner uses SurfaceAlt of the dark theme.
	if pixelAt(buf, w, 4, 4) != theme.SurfaceAlt {
		t.Fatalf("dark track pixel = %+v, want SurfaceAlt",
			pixelAt(buf, w, 4, 4))
	}
}

// --- Zero-width bounds ---------------------------------------------------

// Zero Bounds() → widget falls back to ProgressCircleSize.
func TestProgressCircleDrawZeroBoundsUsesDefaultSize(t *testing.T) {
	// Allocate a surface at least ProgressCircleSize x ProgressCircleSize.
	const w, h = 64, 64
	theme := DefaultLight()
	pc := NewProgressCircle()
	// Bounds() zero → default fallback engages.
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)
	// Track ink appears at (0, 0) (the default rect origin).
	if pixelAt(buf, w, 0, 0) != theme.SurfaceAlt {
		t.Fatalf("default-size track pixel (0,0) = %+v, want SurfaceAlt",
			pixelAt(buf, w, 0, 0))
	}
}

// --- Non-square bounds ---------------------------------------------------

// Non-square Bounds() → widget clamps to a square from the smaller
// dimension.
func TestProgressCircleDrawNonSquareBoundsClampsToSquare(t *testing.T) {
	const w, h = 100, 60
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(0.5)
	pc.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	buf := makeSurface(w, h)
	pc.Draw(newP(buf, w), theme)
	// Track corner (0,0) is SurfaceAlt.
	if pixelAt(buf, w, 0, 0) != theme.SurfaceAlt {
		t.Fatalf("track (0,0) = %+v, want SurfaceAlt", pixelAt(buf, w, 0, 0))
	}
	// Pixel at (0, 40) is beyond the square (side = 30) so it must
	// remain sentinel.
	if pixelAt(buf, w, 0, 40) == theme.SurfaceAlt {
		t.Fatalf("pixel outside the clamped square was painted")
	}
}

// --- Tiny bounds ---------------------------------------------------------

// Bounds smaller than 2*ProgressCircleStroke: the inner rect
// collapses; Draw must still not panic and must skip the inner /
// caption paints.
func TestProgressCircleDrawTinyBoundsInnerCollapse(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	pc := NewProgressCircle()
	pc.Fraction().Set(0.75)
	pc.SetBounds(Rect{X: 0, Y: 0, W: 2 * ProgressCircleStroke, H: 2 * ProgressCircleStroke})
	pc.Draw(newP(makeSurface(w, h), w), theme)
}

// Bounds exactly 2*ProgressCircleStroke sized: side - 2*Stroke = 0
// → the inner hole has zero W/H and its fill + caption branches are
// skipped. Combined with tiny-bounds test to hammer the guards.
func TestProgressCircleDrawExactStrokeBounds(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	pc := NewProgressCircle()
	// Fraction full to exercise the band branch even when inner is 0.
	pc.Fraction().Set(1)
	pc.SetBounds(Rect{X: 0, Y: 0, W: 2 * ProgressCircleStroke, H: 2 * ProgressCircleStroke})
	pc.Draw(newP(makeSurface(w, h), w), theme)
}

// --- Empty Extra map -----------------------------------------------------

func TestProgressCircleDrawWithEmptyExtraMap(t *testing.T) {
	const w, h = 60, 60
	theme := DefaultLight()
	theme.Extra = nil
	pc := NewProgressCircle()
	pc.Fraction().Set(0.4)
	pc.SetBounds(Rect{X: 4, Y: 4, W: 40, H: 40})
	pc.Draw(newP(makeSurface(w, h), w), theme)
}
