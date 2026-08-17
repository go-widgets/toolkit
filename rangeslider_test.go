// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

// newTestRangeSlider returns a 200-px-wide slider over [0,100] with band
// [20,80], already bounded so pixel math is exercised.
func newTestRangeSlider() *RangeSlider {
	s := NewRangeSlider(0, 100, 20, 80)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: scaleThumbSize})
	return s
}

func TestRangeSliderNewOrdersAndClamps(t *testing.T) {
	// low > high is swapped, and both are clamped into range.
	s := NewRangeSlider(0, 100, 140, -30)
	if s.Low != 0 || s.High != 100 {
		t.Errorf("SetRange(140,-30) = (%v,%v), want (0,100)", s.Low, s.High)
	}
}

func TestRangeSliderDrawBand(t *testing.T) {
	s := newTestRangeSlider()
	surf := makeSurface(200, scaleThumbSize)
	s.Draw(newP(surf, 200), DefaultLight())

	// The midpoint (value 50) sits inside the [20,80] band → Accent-tinted,
	// not the bare SurfaceAlt track.
	acc := DefaultLight().Accent
	got := pixelAt(surf, 200, 100, scaleThumbSize/2)
	if got != acc {
		t.Errorf("band midpoint = %+v, want Accent %+v", got, acc)
	}
}

// TestRangeSliderDisabledMuted checks a disabled range slider mutes its band (and
// so its whole chrome) instead of drawing the live Accent, matching Scale.
func TestRangeSliderDisabledMuted(t *testing.T) {
	theme := DefaultLight()
	s := newTestRangeSlider()
	s.Disabled = true
	surf := makeSurface(200, scaleThumbSize)
	s.Draw(newP(surf, 200), theme)

	got := pixelAt(surf, 200, 100, scaleThumbSize/2)
	if got == theme.Accent {
		t.Fatal("disabled range slider band still painted live Accent")
	}
	if got != mutedInk(theme) {
		t.Errorf("disabled band midpoint = %+v, want muted %+v", got, mutedInk(theme))
	}
}

func TestRangeSliderDrawZeroBand(t *testing.T) {
	// Low == High → bandW == 0 → the fill branch is skipped (no panic).
	s := NewRangeSlider(0, 100, 50, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: scaleThumbSize})
	surf := makeSurface(200, scaleThumbSize)
	s.Draw(newP(surf, 200), DefaultLight())
}

func TestRangeSliderClickGrabsNearestHandle(t *testing.T) {
	s := newTestRangeSlider()
	var gotLow, gotHigh float64
	s.OnChange = func(lo, hi float64) { gotLow, gotHigh = lo, hi }

	// Click near the left third → grabs Low and drags it toward the cursor.
	s.OnEvent(Event{Kind: EventClick, X: 30})
	if s.active != 1 {
		t.Fatalf("active = %d, want 1 (Low)", s.active)
	}
	if s.Low >= 20 {
		t.Errorf("Low did not move down: %v", s.Low)
	}
	if gotLow != s.Low || gotHigh != s.High {
		t.Errorf("OnChange saw (%v,%v), want (%v,%v)", gotLow, gotHigh, s.Low, s.High)
	}

	// Click near the right edge → grabs High.
	s.OnEvent(Event{Kind: EventClick, X: 180})
	if s.active != 2 {
		t.Fatalf("active = %d, want 2 (High)", s.active)
	}
}

func TestRangeSliderHandlePickIsLocalAtNonZeroBounds(t *testing.T) {
	// Regression: at a non-zero X, clicking near the High thumb must grab High,
	// not Low. The bug compared local ev.X to absolute thumb centres, so a click
	// near High (local) looked nearer Low (absolute) and grabbed the wrong one.
	s := NewRangeSlider(0, 100, 20, 80)
	s.SetBounds(Rect{X: 100, Y: 0, W: 200, H: scaleThumbSize})
	// High thumb's local centre ≈ 155 (abs 247 − origin 100 + thumb/2).
	s.OnEvent(Event{Kind: EventClick, X: 150})
	if s.active != 2 {
		t.Fatalf("active = %d, want 2 (High) — handle-pick used absolute coords", s.active)
	}
	// High grabbed → Low is untouched; the buggy path would have snapped Low up.
	if s.Low != 20 {
		t.Errorf("Low moved to %v (wrong handle grabbed), want 20", s.Low)
	}
	if s.High >= 80 || s.High < 70 {
		t.Errorf("High = %v, want it dragged near the click (~77)", s.High)
	}
}

func TestRangeSliderVertical(t *testing.T) {
	// Vertical: top = Max. A click near the top grabs High and drags it up; a
	// click near the bottom grabs Low. The Accent band spans the mid-track.
	s := NewRangeSlider(0, 100, 20, 80)
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 200})
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: 40}) // near the top
	if s.active != 2 {
		t.Fatalf("top click active=%d, want 2 (High)", s.active)
	}
	if s.Low != 20 {
		t.Errorf("Low moved to %v, want 20 (untouched)", s.Low)
	}
	if s.High <= 80 {
		t.Errorf("High=%v, want dragged up past 80", s.High)
	}

	s2 := NewRangeSlider(0, 100, 20, 80)
	s2.Orientation = Vertical
	s2.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 200})
	s2.OnEvent(Event{Kind: EventClick, X: 10, Y: 160}) // near the bottom
	if s2.active != 1 {
		t.Fatalf("bottom click active=%d, want 1 (Low)", s2.active)
	}

	surf := makeSurface(20, 200)
	s.Draw(newP(surf, 20), DefaultLight())
	if pixelAt(surf, 20, 10, 100) != DefaultLight().Accent {
		t.Errorf("vertical band missing mid-track: %+v", pixelAt(surf, 20, 10, 100))
	}
}

func TestRangeSliderVerticalNoTravelReturnsMin(t *testing.T) {
	// A vertical slider with no travel (H ≤ thumb) → valueAt returns Min.
	s := NewRangeSlider(0, 100, 20, 80)
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: scaleThumbSize - 1})
	if v := s.valueAt(5); v != s.Min {
		t.Fatalf("valueAt on zero vertical span = %v, want Min %v", v, s.Min)
	}
}

func TestRangeSliderHandlesDoNotCross(t *testing.T) {
	s := newTestRangeSlider()

	// Grab Low, then drag it far past High → clamps at High.
	s.OnEvent(Event{Kind: EventClick, X: 40}) // near Low
	s.active = 1
	s.OnEvent(Event{Kind: EventMouseDrag, X: 195}) // way past High
	if s.Low != s.High {
		t.Errorf("Low crossed High: Low=%v High=%v", s.Low, s.High)
	}

	// Grab High, drag it below Low → clamps at Low.
	s2 := newTestRangeSlider()
	s2.active = 2
	s2.OnEvent(Event{Kind: EventMouseDrag, X: 5}) // below Low
	if s2.High != s2.Low {
		t.Errorf("High crossed Low: Low=%v High=%v", s2.Low, s2.High)
	}
}

func TestRangeSliderDragWithoutGrabIsNoop(t *testing.T) {
	s := newTestRangeSlider()
	lo, hi := s.Low, s.High
	// active == 0 → a stray drag changes nothing.
	s.OnEvent(Event{Kind: EventMouseDrag, X: 100})
	if s.Low != lo || s.High != hi {
		t.Errorf("ungrabbed drag moved band to (%v,%v)", s.Low, s.High)
	}
}

func TestRangeSliderMouseUpReleases(t *testing.T) {
	s := newTestRangeSlider()
	s.active = 2
	s.OnEvent(Event{Kind: EventMouseUp, X: 100})
	if s.active != 0 {
		t.Errorf("active = %d after mouse-up, want 0", s.active)
	}
}

func TestRangeSliderDegenerateGeometryIgnored(t *testing.T) {
	// Zero width and Min==Max both make OnEvent a no-op (guard branches).
	s := NewRangeSlider(0, 100, 20, 80)
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: scaleThumbSize})
	s.OnEvent(Event{Kind: EventClick, X: 10}) // W<=0

	flat := NewRangeSlider(5, 5, 5, 5)
	flat.SetBounds(Rect{X: 0, Y: 0, W: 200, H: scaleThumbSize})
	flat.OnEvent(Event{Kind: EventClick, X: 10}) // Max<=Min

	// valueAt with a span <= 0 (width narrower than a thumb) returns Min.
	narrow := NewRangeSlider(0, 100, 20, 80)
	narrow.SetBounds(Rect{X: 0, Y: 0, W: scaleThumbSize - 1, H: scaleThumbSize})
	if v := narrow.valueAt(50); v != narrow.Min {
		t.Errorf("valueAt on zero span = %v, want Min %v", v, narrow.Min)
	}
}

func TestRangeSliderValueAtClamps(t *testing.T) {
	s := newTestRangeSlider()
	// Far left of the thumb centre → pos < 0 → clamps to Min.
	if v := s.valueAt(-50); v != s.Min {
		t.Errorf("valueAt(-50) = %v, want Min %v", v, s.Min)
	}
	// Far right past the track → pos > 1 → clamps to Max.
	if v := s.valueAt(10_000); v != s.Max {
		t.Errorf("valueAt(10000) = %v, want Max %v", v, s.Max)
	}
}

func TestRangeSliderMoveActiveNoneIsNoop(t *testing.T) {
	// moveActive with active==0 hits neither case; only OnChange (nil here) —
	// exercises the default path without a handler.
	s := newTestRangeSlider()
	s.active = 0
	s.moveActive(100) // no panic, no OnChange (nil)
}

func TestAbs(t *testing.T) {
	if abs(-3) != 3 || abs(4) != 4 {
		t.Error("abs")
	}
}
