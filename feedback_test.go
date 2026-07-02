// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- ProgressBar ---------------------------------------------------------

func TestProgressBarSetFractionClamps(t *testing.T) {
	p := NewProgressBar()
	p.SetFraction(-0.5)
	if p.Fraction != 0 {
		t.Fatalf("negative clamp: got %v", p.Fraction)
	}
	p.SetFraction(1.7)
	if p.Fraction != 1 {
		t.Fatalf("over-1 clamp: got %v", p.Fraction)
	}
	p.SetFraction(0.3)
	if p.Fraction != 0.3 {
		t.Fatalf("normal set: got %v", p.Fraction)
	}
}

func TestProgressBarDrawHalfFill(t *testing.T) {
	const w, h = 64, 20
	theme := DefaultLight()
	p := NewProgressBar()
	p.SetFraction(0.5)
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Left third should be Accent, right third should be SurfaceAlt.
	if pixelAt(buf, w, 10, 8) != theme.Accent {
		t.Fatalf("left third = %+v, want Accent", pixelAt(buf, w, 10, 8))
	}
	if pixelAt(buf, w, 50, 8) != theme.SurfaceAlt {
		t.Fatalf("right third = %+v, want SurfaceAlt", pixelAt(buf, w, 50, 8))
	}
}

func TestProgressBarDrawClampInDraw(t *testing.T) {
	// Draw also clamps Fraction defensively in case caller bypasses
	// SetFraction. Cover both branches.
	const w, h = 64, 20
	theme := DefaultLight()
	p := &ProgressBar{Fraction: -1}
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	p.Draw(newP(makeSurface(w, h), w), theme)
	p.Fraction = 2
	p.Draw(newP(makeSurface(w, h), w), theme)
}

func TestProgressBarDrawWithLabel(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	p := NewProgressBar()
	p.SetFraction(0.5)
	p.Label = "50%"
	p.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	p.Draw(newP(makeSurface(w, h), w), theme)
}

// --- LevelBar ------------------------------------------------------------

func TestLevelBarNewClampsMax(t *testing.T) {
	l := NewLevelBar(-3)
	if l.Max != 1 {
		t.Fatalf("Max clamp: got %d, want 1", l.Max)
	}
}

func TestLevelBarDraw(t *testing.T) {
	const w, h = 64, 12
	theme := DefaultLight()
	l := NewLevelBar(5)
	l.Value = 3
	l.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 10})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	// First cell (idx 0) should be Accent (Value=3 > 0).
	if pixelAt(buf, w, 2, 5) != theme.Accent {
		t.Fatalf("cell 0 = %+v, want Accent", pixelAt(buf, w, 2, 5))
	}
}

func TestLevelBarDrawMaxZeroNoOp(t *testing.T) {
	l := &LevelBar{Max: 0}
	l.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 10})
	l.Draw(newP(makeSurface(64, 12), 64), DefaultLight())
}

func TestLevelBarTinyCellWidth(t *testing.T) {
	// Max=20 cells in a 5-px wide bar -> cellW < 1, clamps to 1.
	l := NewLevelBar(20)
	l.Value = 10
	l.SetBounds(Rect{X: 0, Y: 0, W: 5, H: 10})
	l.Draw(newP(makeSurface(64, 12), 64), DefaultLight())
}

// --- Scale ---------------------------------------------------------------

func TestScaleSetValueClamps(t *testing.T) {
	s := NewScale(0, 100, 50)
	s.SetValue(-5)
	if s.Value != 0 {
		t.Fatal("low clamp")
	}
	s.SetValue(200)
	if s.Value != 100 {
		t.Fatal("high clamp")
	}
}

func TestScaleClickSetsValueAndFires(t *testing.T) {
	got := 0.0
	s := NewScale(0, 100, 0)
	s.OnChange = func(v float64) { got = v }
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 10})
	if s.Value != 50 {
		t.Fatalf("after click x=50: Value = %v", s.Value)
	}
	if got != 50 {
		t.Fatalf("OnChange got %v", got)
	}
}

func TestScaleClickClampsPosition(t *testing.T) {
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: -10, Y: 10})
	if s.Value != 0 {
		t.Fatalf("negative click should clamp to Min, got %v", s.Value)
	}
	s.OnEvent(Event{Kind: EventClick, X: 200, Y: 10})
	if s.Value != 100 {
		t.Fatalf("over-W click should clamp to Max, got %v", s.Value)
	}
}

func TestScaleIgnoresNonClick(t *testing.T) {
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if s.Value != 50 {
		t.Fatal("KeyDown should not move the value")
	}
}

func TestScaleZeroWidthOrDegenerateRangeNoOp(t *testing.T) {
	// Zero-width bounds.
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 10})
	if s.Value != 50 {
		t.Fatal("zero-width click should be no-op")
	}
	// Degenerate Min == Max.
	s2 := NewScale(5, 5, 5)
	s2.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s2.OnEvent(Event{Kind: EventClick, X: 50, Y: 10})
	if s2.Value != 5 {
		t.Fatal("Min==Max click should be no-op")
	}
}

func TestScaleDrawDegenerateRange(t *testing.T) {
	s := NewScale(5, 5, 5)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.Draw(newP(makeSurface(100, 20), 100), DefaultLight())
}

func TestScaleDrawNormal(t *testing.T) {
	const w, h = 100, 20
	theme := DefaultLight()
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Thumb at x = 45 (half of 100-10), Accent fill.
	if pixelAt(buf, w, 48, 10) != theme.Accent {
		t.Fatalf("thumb pixel = %+v, want Accent", pixelAt(buf, w, 48, 10))
	}
}

func TestScaleNilOnChangeNoPanic(t *testing.T) {
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventClick, X: 30, Y: 10})
}

// --- SpinButton ----------------------------------------------------------

func TestSpinButtonStepZeroClampsToOne(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 0)
	if s.Step != 1 {
		t.Fatalf("Step = %d, want 1 (clamp)", s.Step)
	}
}

func TestSpinButtonSetValueClamps(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 1)
	s.SetValue(-5)
	if s.Value != 0 {
		t.Fatal("low clamp")
	}
	s.SetValue(50)
	if s.Value != 10 {
		t.Fatal("high clamp")
	}
}

func TestSpinButtonClickPlusIncrements(t *testing.T) {
	got := -1
	s := NewSpinButton(0, 10, 5, 2)
	s.OnChange = func(v int) { got = v }
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	// Click on the upper button: x in [60-16, 60), y < 12.
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 4})
	if s.Value != 7 || got != 7 {
		t.Fatalf("after +: Value=%d got=%d", s.Value, got)
	}
}

func TestSpinButtonClickMinusDecrements(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 2)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 20})
	if s.Value != 3 {
		t.Fatalf("after -: Value=%d", s.Value)
	}
}

func TestSpinButtonBodyClickNoOp(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if s.Value != 5 {
		t.Fatal("body click must not change value")
	}
}

func TestSpinButtonIgnoresNonClick(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Up"})
	if s.Value != 5 {
		t.Fatal("KeyDown should not change value")
	}
}

func TestSpinButtonNilCallbackNoPanic(t *testing.T) {
	s := NewSpinButton(0, 10, 5, 1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	s.OnEvent(Event{Kind: EventClick, X: 50, Y: 4})
}

func TestSpinButtonDraw(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	s := NewSpinButton(0, 10, 5, 1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	s.Draw(newP(makeSurface(w, h), w), theme)
}

// --- Image ---------------------------------------------------------------

func TestImageDrawIdentityBlit(t *testing.T) {
	const W = 16
	// Source 8x8 pure red.
	src := make([]byte, 8*8*4)
	for i := 0; i+3 < len(src); i += 4 {
		src[i+0], src[i+1], src[i+2], src[i+3] = 0xFF, 0, 0, 0xFF
	}
	img := NewImage(src, 8, 8)
	img.SetBounds(Rect{X: 2, Y: 2, W: 8, H: 8})
	buf := makeSurface(W, W)
	img.Draw(newP(buf, W), DefaultLight())
	if pixelAt(buf, W, 5, 5) != (RGBA{R: 0xFF, G: 0, B: 0, A: 0xFF}) {
		t.Fatalf("blitted pixel = %+v", pixelAt(buf, W, 5, 5))
	}
}

func TestImageDrawScaledBlit(t *testing.T) {
	const W = 32
	// Source 4x4 pure blue, target 16x16 -> 4x upscale via NN.
	src := make([]byte, 4*4*4)
	for i := 0; i+3 < len(src); i += 4 {
		src[i+0], src[i+1], src[i+2], src[i+3] = 0, 0, 0xFF, 0xFF
	}
	img := NewImage(src, 4, 4)
	img.SetBounds(Rect{X: 0, Y: 0, W: 16, H: 16})
	buf := makeSurface(W, W)
	img.Draw(newP(buf, W), DefaultLight())
	if pixelAt(buf, W, 8, 8) != (RGBA{R: 0, G: 0, B: 0xFF, A: 0xFF}) {
		t.Fatalf("scaled pixel = %+v", pixelAt(buf, W, 8, 8))
	}
}

func TestImageDrawEmptySourceNoOp(t *testing.T) {
	img := &Image{Pixels: nil, W: 0, H: 0}
	img.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	img.Draw(newP(makeSurface(16, 16), 16), DefaultLight())
}

func TestImageDrawShortPixelsNoOp(t *testing.T) {
	img := &Image{Pixels: make([]byte, 3), W: 4, H: 4} // way too short
	img.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	img.Draw(newP(makeSurface(16, 16), 16), DefaultLight())
}

func TestImageDrawClipsOffSurface(t *testing.T) {
	src := make([]byte, 4*4*4)
	img := NewImage(src, 4, 4)
	img.SetBounds(Rect{X: -2, Y: -2, W: 4, H: 4}) // partially off-surface
	img.Draw(newP(makeSurface(8, 8), 8), DefaultLight())
}

func TestImageDrawTruncatedSurface(t *testing.T) {
	// Per-pixel dOff+3 >= len(surface) guard triggers when target Y
	// falls past the surface's actual row count (surface buffer is
	// shorter than declared by surfaceW). Real callers pass correctly
	// sized buffers; this exercises the defensive clamp.
	src := make([]byte, 4*4*4)
	img := NewImage(src, 4, 4)
	img.SetBounds(Rect{X: 0, Y: 0, W: 4, H: 4})
	// Buffer big enough for surfaceW=16 stride but only 2 rows of data
	// so the third row trips the per-pixel guard.
	buf := make([]byte, 16*2*4+4)
	img.Draw(newP(buf, 16), DefaultLight())
}

// --- Spinner -------------------------------------------------------------

func TestSpinnerTickWrapsModuloOne(t *testing.T) {
	s := NewSpinner()
	s.Tick(0.3)
	s.Tick(0.4)
	s.Tick(0.4) // total 1.1, wraps to 0.1
	if s.Phase < 0.099 || s.Phase > 0.101 {
		t.Fatalf("Phase = %v, want ~0.1", s.Phase)
	}
}

func TestSpinnerInactiveDoesNotDraw(t *testing.T) {
	s := NewSpinner()
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	buf := makeSurface(32, 32)
	s.Draw(newP(buf, 32), DefaultLight())
	// Sentinel intact -> nothing painted.
	if pixelAt(buf, 32, 10, 10) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("inactive spinner painted")
	}
}

func TestSpinnerActivePaintsHand(t *testing.T) {
	theme := DefaultLight()
	s := NewSpinner()
	s.Active = true
	s.Phase = 0 // 0 radians -> hand points to the right (+x).
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	buf := makeSurface(32, 32)
	s.Draw(newP(buf, 32), theme)
	// Some pixel along the +x ray from centre (10,10) should be Accent.
	found := false
	for dx := 1; dx < 5; dx++ {
		if pixelAt(buf, 32, 10+dx, 10) == theme.Accent {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("active spinner did not paint hand")
	}
}

func TestSpinnerZeroBoundsNoCrash(t *testing.T) {
	s := NewSpinner()
	s.Active = true
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	s.Draw(newP(makeSurface(16, 16), 16), DefaultLight())
}

func TestSpinnerSmallerHeightRadius(t *testing.T) {
	// Cover the `if r.H < r.W` branch picking H/2 as the radius.
	s := NewSpinner()
	s.Active = true
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 10})
	s.Draw(newP(makeSurface(64, 16), 64), DefaultLight())
}

func TestSpinnerSubPixelStepsClamp(t *testing.T) {
	// radius < 1 forces steps to clamp to 1.
	s := NewSpinner()
	s.Active = true
	s.SetBounds(Rect{X: 0, Y: 0, W: 4, H: 4})
	s.Draw(newP(makeSurface(8, 8), 8), DefaultLight())
}
