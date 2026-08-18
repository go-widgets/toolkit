// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Touch-density adoption for the INPUTS + PICKERS family. Two guarantees are
// proven for every widget here:
//
//   - DensityCompact is byte-identical: a HitRect equals the drawn rectangle it
//     wraps to the pixel, and every metric routed through scaled keeps its
//     historical value, so a desktop host sees nothing change.
//   - DensityTouch clamps interactive targets to the 44-logical-pixel floor
//     (exact values, not "bigger") and scales chrome by ×1.5, without moving or
//     resizing what Draw paints.
//
// The suite restores the two globals (MetricScale, Density) via restoreDensity
// (density_test.go) so a case can never leak a non-default profile.

// touchFloor is MinHitTarget at DensityTouch and MetricScale 1 — the Apple HIG /
// Material finger floor the clamps must reach. Hard-coded here (not read back
// from the package) so the oracle below is an INDEPENDENT check of the widgets'
// arithmetic, not a restatement of it.
const touchFloor = 44

// wantTouchHit is the independent oracle for touchHitRect at DensityTouch / 1x: it
// clamps each axis up to touchFloor and re-centres the enlarged rect over the
// original, reproducing the intended geometry from first principles so a widget
// that computes it a different way is caught.
func wantTouchHit(b Rect) Rect {
	clamp := func(v int) int {
		if v < touchFloor {
			return touchFloor
		}
		return v
	}
	w, h := clamp(b.W), clamp(b.H)
	return Rect{X: b.X - (w-b.W)/2, Y: b.Y - (h-b.H)/2, W: w, H: h}
}

// TestTouchHitRectControlRun validates the shared touchHitRect helper against a
// KNOWN-GOOD control — the already-shipped, already-tested [Switch.HitRect] — and
// against the independent wantTouchHit oracle, at both densities, before any
// family widget relies on it. A new instrument is proven against a trusted one
// first: if touchHitRect ever diverges from Switch's clamp, this fails here rather
// than in thirteen widget tests at once.
func TestTouchHitRectControlRun(t *testing.T) {
	defer restoreDensity()

	rects := []Rect{
		{X: 100, Y: 200, W: 30, H: 16}, // both axes below the floor
		{X: 0, Y: 0, W: 60, H: 60},     // both already above the floor
		{X: 5, Y: 7, W: 200, H: 28},    // wide but short (one axis clamps)
		{X: 5, Y: 7, W: 20, H: 200},    // tall but narrow (the other axis clamps)
	}

	// Compact: touchHitRect is a pure pass-through, equal to the input rect and to
	// the control Switch's hit rect byte-for-byte.
	for _, b := range rects {
		s := NewSwitch(false)
		s.SetBounds(b)
		if got := touchHitRect(b); got != b {
			t.Fatalf("compact touchHitRect(%+v) = %+v, want the input rect", b, got)
		}
		if got, ctrl := touchHitRect(b), s.HitRect(); got != ctrl {
			t.Fatalf("compact touchHitRect(%+v) = %+v, control Switch.HitRect = %+v", b, got, ctrl)
		}
	}

	// Touch: touchHitRect must equal BOTH the control and the oracle.
	SetDensity(DensityTouch)
	for _, b := range rects {
		s := NewSwitch(false)
		s.SetBounds(b)
		got := touchHitRect(b)
		if ctrl := s.HitRect(); got != ctrl {
			t.Fatalf("touch touchHitRect(%+v) = %+v, control Switch.HitRect = %+v", b, got, ctrl)
		}
		if want := wantTouchHit(b); got != want {
			t.Fatalf("touch touchHitRect(%+v) = %+v, oracle = %+v", b, got, want)
		}
		if got.W < touchFloor || got.H < touchFloor {
			t.Fatalf("touch touchHitRect(%+v) = %+v, both axes must reach %d", b, got, touchFloor)
		}
	}
}

// touchField is the shared shape of every field/toggle-style widget whose whole
// body is one tap target: it can be positioned and expose a HitRect.
type touchField interface {
	SetBounds(Rect)
	HitRect() Rect
}

// TestFieldHitRects proves the field/toggle HitRect on every INPUTS/PICKERS
// widget that clamps its whole body: identical to Bounds at compact, and clamped
// to a centred ≥44 rect at touch (asserted against the oracle AND a hand-computed
// exact value).
func TestFieldHitRects(t *testing.T) {
	defer restoreDensity()

	b := Rect{X: 10, Y: 20, W: 160, H: 28}
	fields := func() []struct {
		name string
		w    touchField
	} {
		return []struct {
			name string
			w    touchField
		}{
			{"Entry", NewEntry("hi")},
			{"SearchEntry", NewSearchEntry("hi")},
			{"ComboBox", NewComboBox([]string{"a", "b"})},
			{"DropDown", NewDropDown([]string{"a", "b"}, 0)},
			{"DatePicker", NewDatePicker(2026, 8, 18)},
			{"TimePicker", NewTimePicker(9, 30)},
			{"ColorChooser", NewColorChooser(RGBA{R: 1, G: 2, B: 3, A: 0xFF})},
			{"TagField", NewTagField("x")},
		}
	}

	// Compact: HitRect == Bounds, byte-for-byte.
	for _, f := range fields() {
		f.w.SetBounds(b)
		if got := f.w.HitRect(); got != b {
			t.Fatalf("%s compact HitRect = %+v, want Bounds %+v", f.name, got, b)
		}
	}

	// Touch: HitRect grows only the short axis to exactly 44, centred.
	SetDensity(DensityTouch)
	wantExact := Rect{X: 10, Y: 12, W: 160, H: 44} // H 28->44 centred (Y-8); W unchanged
	if wantTouchHit(b) != wantExact {
		t.Fatalf("oracle disagrees with hand value: %+v vs %+v", wantTouchHit(b), wantExact)
	}
	for _, f := range fields() {
		f.w.SetBounds(b)
		got := f.w.HitRect()
		if got != wantExact {
			t.Fatalf("%s touch HitRect = %+v, want %+v", f.name, got, wantExact)
		}
		if got.H != touchFloor {
			t.Fatalf("%s touch HitRect H = %d, want exactly %d", f.name, got.H, touchFloor)
		}
	}
}

// TestSearchEntryClearHitRect covers the trailing clear affordance: a narrow
// 16-logical-pixel slot that must still expose a 44px grab at touch. Compact hit
// equals the drawn slot; touch clamps it to a centred 44×44.
func TestSearchEntryClearHitRect(t *testing.T) {
	defer restoreDensity()

	s := NewSearchEntry("hi")
	b := Rect{X: 10, Y: 20, W: 160, H: 28}
	s.SetBounds(b)

	// Compact drawn slot: padX=4, iconW=16 -> X=10+160-4-16=150.
	wantSlot := Rect{X: 150, Y: 20, W: 16, H: 28}
	if got := s.clearSlot(); got != wantSlot {
		t.Fatalf("compact clearSlot = %+v, want %+v", got, wantSlot)
	}
	if got := s.ClearHitRect(); got != wantSlot {
		t.Fatalf("compact ClearHitRect = %+v, want drawn slot %+v", got, wantSlot)
	}

	// Touch: padX=6, iconW=24 -> slot X=10+160-6-24=140, then clamp 24/28 -> 44.
	SetDensity(DensityTouch)
	touchSlot := Rect{X: 140, Y: 20, W: 24, H: 28}
	if got := s.clearSlot(); got != touchSlot {
		t.Fatalf("touch clearSlot = %+v, want %+v", got, touchSlot)
	}
	wantHit := Rect{X: 130, Y: 12, W: 44, H: 44}
	if got := s.ClearHitRect(); got != wantHit || got != wantTouchHit(touchSlot) {
		t.Fatalf("touch ClearHitRect = %+v, want %+v", got, wantHit)
	}
}

// TestScaleThumbHitRect proves the single-thumb slider grab. thumbRect is first
// shown to land on the ACTUAL drawn thumb (a pixel probe — a control run against
// what Draw paints, not just the formula), then the hit rect is asserted equal to
// the drawn thumb at compact and clamped to a centred 44×44 at touch.
func TestScaleThumbHitRect(t *testing.T) {
	defer restoreDensity()

	s := NewScale(0, 100, 50)
	b := Rect{X: 0, Y: 0, W: 200, H: 40}
	s.SetBounds(b)

	// Compact drawn thumb: sz=16, tx=int(0.5*(200-16))=92, ty=(40-16)/2=12.
	wantThumb := Rect{X: 92, Y: 12, W: 16, H: 16}
	if got := s.thumbRect(); got != wantThumb {
		t.Fatalf("compact thumbRect = %+v, want %+v", got, wantThumb)
	}
	if got := s.ThumbHitRect(); got != wantThumb {
		t.Fatalf("compact ThumbHitRect = %+v, want drawn thumb %+v", got, wantThumb)
	}

	// Pixel probe: the thumb's centre pixel is painted with the thumb fill
	// (theme.Surface), while a track pixel to its right is the unfilled track
	// (theme.SurfaceAlt) — so thumbRect really points at the drawn knob.
	theme := DefaultDark()
	if theme.Surface == theme.SurfaceAlt {
		t.Fatal("theme Surface and SurfaceAlt must differ for the probe to mean anything")
	}
	buf := makeSurface(200, 40)
	s.Draw(newP(buf, 200), theme)
	cx, cy := wantThumb.X+wantThumb.W/2, wantThumb.Y+wantThumb.H/2 // (100, 20)
	if got := pixelAt(buf, 200, cx, cy); got != theme.Surface {
		t.Fatalf("thumb centre pixel (%d,%d) = %+v, want thumb fill %+v", cx, cy, got, theme.Surface)
	}
	if got := pixelAt(buf, 200, 190, cy); got != theme.SurfaceAlt {
		t.Fatalf("far-right track pixel = %+v, want unfilled track %+v", got, theme.SurfaceAlt)
	}

	// Touch: thumb grows to sz=24 and the grab clamps to a centred 44×44.
	SetDensity(DensityTouch)
	touchThumb := Rect{X: 88, Y: 8, W: 24, H: 24} // sz=24, tx=int(0.5*(200-24))=88
	if got := s.thumbRect(); got != touchThumb {
		t.Fatalf("touch thumbRect = %+v, want %+v", got, touchThumb)
	}
	got := s.ThumbHitRect()
	if got != wantTouchHit(touchThumb) {
		t.Fatalf("touch ThumbHitRect = %+v, oracle %+v", got, wantTouchHit(touchThumb))
	}
	if got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch ThumbHitRect = %+v, thumb grab must be exactly %dx%d", got, touchFloor, touchFloor)
	}
}

// TestRangeSliderThumbHitRects proves both handles of the two-thumb slider: each
// drawn thumb at compact, each clamped to a centred 44×44 at touch.
func TestRangeSliderThumbHitRects(t *testing.T) {
	defer restoreDensity()

	s := NewRangeSlider(0, 100, 25, 75)
	b := Rect{X: 0, Y: 0, W: 200, H: 40}
	s.SetBounds(b)

	// Compact: sz=16, span=184. Low pos .25 -> x=46; High pos .75 -> x=138.
	wantLow := Rect{X: 46, Y: 12, W: 16, H: 16}
	wantHigh := Rect{X: 138, Y: 12, W: 16, H: 16}
	if got := s.thumbRect(s.Low); got != wantLow {
		t.Fatalf("compact low thumbRect = %+v, want %+v", got, wantLow)
	}
	if got := s.thumbRect(s.High); got != wantHigh {
		t.Fatalf("compact high thumbRect = %+v, want %+v", got, wantHigh)
	}
	if got := s.LowThumbHitRect(); got != wantLow {
		t.Fatalf("compact LowThumbHitRect = %+v, want %+v", got, wantLow)
	}
	if got := s.HighThumbHitRect(); got != wantHigh {
		t.Fatalf("compact HighThumbHitRect = %+v, want %+v", got, wantHigh)
	}

	// Touch: sz=24, span=176. Low x=int(.25*176)=44; High x=int(.75*176)=132.
	SetDensity(DensityTouch)
	lowThumb := Rect{X: 44, Y: 8, W: 24, H: 24}
	highThumb := Rect{X: 132, Y: 8, W: 24, H: 24}
	if got := s.LowThumbHitRect(); got != wantTouchHit(lowThumb) || got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch LowThumbHitRect = %+v, want %+v (44x44)", got, wantTouchHit(lowThumb))
	}
	if got := s.HighThumbHitRect(); got != wantTouchHit(highThumb) || got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch HighThumbHitRect = %+v, want %+v (44x44)", got, wantTouchHit(highThumb))
	}
}

// TestVerticalSliderThumbRects covers the Vertical arm of both sliders'
// thumbRect: the knob is centred across the width and travels down the height,
// with the top (pos=1) at Max. Asserted at compact with exact hand values, and
// the touch grab shown to reach the 44px floor.
func TestVerticalSliderThumbRects(t *testing.T) {
	defer restoreDensity()

	// Scale vertical: bounds 40x200, Value 50 -> ty=int(0.5*(200-16))=92,
	// x centred (40-16)/2=12.
	sc := NewScale(0, 100, 50)
	sc.Orientation = Vertical
	sc.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200})
	wantSc := Rect{X: 12, Y: 92, W: 16, H: 16}
	if got := sc.thumbRect(); got != wantSc {
		t.Fatalf("compact vertical Scale thumbRect = %+v, want %+v", got, wantSc)
	}
	if got := sc.ThumbHitRect(); got != wantSc {
		t.Fatalf("compact vertical Scale ThumbHitRect = %+v, want %+v", got, wantSc)
	}

	// RangeSlider vertical: top=Max, so High (75) sits nearer the top.
	// Low=25 -> pos flips to .75 -> y=int(.75*184)=138; High=75 -> y=int(.25*184)=46.
	rs := NewRangeSlider(0, 100, 25, 75)
	rs.Orientation = Vertical
	rs.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200})
	wantLow := Rect{X: 12, Y: 138, W: 16, H: 16}
	wantHigh := Rect{X: 12, Y: 46, W: 16, H: 16}
	if got := rs.thumbRect(rs.Low); got != wantLow {
		t.Fatalf("compact vertical RangeSlider low thumbRect = %+v, want %+v", got, wantLow)
	}
	if got := rs.thumbRect(rs.High); got != wantHigh {
		t.Fatalf("compact vertical RangeSlider high thumbRect = %+v, want %+v", got, wantHigh)
	}

	// Touch: the vertical grab still clamps to the 44px floor on both axes.
	SetDensity(DensityTouch)
	if got := sc.ThumbHitRect(); got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch vertical Scale ThumbHitRect = %+v, want 44x44", got)
	}
	if got := rs.LowThumbHitRect(); got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch vertical RangeSlider LowThumbHitRect = %+v, want 44x44", got)
	}
}

// TestDateRangePickerArrowHitRects proves the two month-paging arrows: each drawn
// cell at compact, each clamped to a centred 44×44 at touch, and the two enlarged
// grabs never overlap (they sit at opposite ends of the header).
func TestDateRangePickerArrowHitRects(t *testing.T) {
	defer restoreDensity()

	rp := NewDateRangePicker(2026, 8)
	rp.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 220})

	// Compact drawn cells: cellW=24, hdrH=22.
	wantPrev := Rect{X: 0, Y: 0, W: 24, H: 22}
	wantNext := Rect{X: 276, Y: 0, W: 24, H: 22}
	if got := rp.prevArrowRect(); got != wantPrev {
		t.Fatalf("compact prevArrowRect = %+v, want %+v", got, wantPrev)
	}
	if got := rp.nextArrowRect(); got != wantNext {
		t.Fatalf("compact nextArrowRect = %+v, want %+v", got, wantNext)
	}
	if got := rp.PrevArrowHitRect(); got != wantPrev {
		t.Fatalf("compact PrevArrowHitRect = %+v, want %+v", got, wantPrev)
	}
	if got := rp.NextArrowHitRect(); got != wantNext {
		t.Fatalf("compact NextArrowHitRect = %+v, want %+v", got, wantNext)
	}

	// Touch: cellW=36, hdrH=33 -> each clamps to a centred 44x44.
	SetDensity(DensityTouch)
	prev := rp.PrevArrowHitRect()
	next := rp.NextArrowHitRect()
	if want := wantTouchHit(Rect{X: 0, Y: 0, W: 36, H: 33}); prev != want || prev.W != touchFloor || prev.H != touchFloor {
		t.Fatalf("touch PrevArrowHitRect = %+v, want %+v (44x44)", prev, want)
	}
	if want := wantTouchHit(Rect{X: 264, Y: 0, W: 36, H: 33}); next != want || next.W != touchFloor || next.H != touchFloor {
		t.Fatalf("touch NextArrowHitRect = %+v, want %+v (44x44)", next, want)
	}
	if prev.X+prev.W > next.X {
		t.Fatalf("touch arrow grabs overlap: prev right=%d, next left=%d", prev.X+prev.W, next.X)
	}
}

// TestColorPickerEyedropHitRect proves the eyedropper button grab (widget-local,
// matching OnEvent's frame): the drawn 20px chip at compact, a centred 44×44 at
// touch.
func TestColorPickerEyedropHitRect(t *testing.T) {
	defer restoreDensity()

	c := NewColorPicker(RGBA{R: 10, G: 20, B: 30, A: 0xFF})

	// Compact drawn button: swatch y=120+8+16+8=152, swatch 28 wide ->
	// eyedrop X=0+28+8=36, Y=152+(28-20)/2=156, 20x20.
	wantBtn := Rect{X: 36, Y: 156, W: 20, H: 20}
	if got := c.eyedropRectLocal(); got != wantBtn {
		t.Fatalf("compact eyedropRectLocal = %+v, want %+v", got, wantBtn)
	}
	if got := c.EyedropHitRect(); got != wantBtn {
		t.Fatalf("compact EyedropHitRect = %+v, want drawn button %+v", got, wantBtn)
	}

	// Touch: swatch y=180+12+24+12=228, swatch 42 wide ->
	// eyedrop X=0+42+12=54, Y=228+(42-30)/2=234, 30x30 -> clamp to 44x44.
	SetDensity(DensityTouch)
	touchBtn := Rect{X: 54, Y: 234, W: 30, H: 30}
	if got := c.eyedropRectLocal(); got != touchBtn {
		t.Fatalf("touch eyedropRectLocal = %+v, want %+v", got, touchBtn)
	}
	if got := c.EyedropHitRect(); got != wantTouchHit(touchBtn) || got.W != touchFloor || got.H != touchFloor {
		t.Fatalf("touch EyedropHitRect = %+v, want %+v (44x44)", got, wantTouchHit(touchBtn))
	}
}

// TestTouchMetricScaling pins the routed-through-scaled metrics that were raw
// bypasses before: exact at compact/1x (byte-identical) and exact ×1.5 at touch.
func TestTouchMetricScaling(t *testing.T) {
	defer restoreDensity()

	cb := NewComboBox([]string{"a", "b", "c"})
	dd := NewDropDown([]string{"a", "b", "c"}, 0)
	dd.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 30})
	cp := NewColorPicker(RGBA{A: 0xFF})
	gh := GlyphHeight()

	// Compact: every routed metric holds its historical literal.
	assertEq(t, "compact ComboBox.rowH", cb.rowH(), comboRowH)
	assertEq(t, "compact DropDown.rowH", dd.rowH(), PopoverRowH)
	assertEq(t, "compact DropDown.PopoverBounds.H", dd.PopoverBounds().H, 3*PopoverRowH)
	assertEq(t, "compact DatePickerFieldH", DatePickerFieldH(), gh+10)
	assertEq(t, "compact ColorPicker.alpha.W", cp.alphaRectLocal().W, ColorPickerWidth)
	assertEq(t, "compact SearchEntryPadX", scaled(SearchEntryPadX), 4)
	assertEq(t, "compact SearchEntryIconW", scaled(SearchEntryIconW), 16)

	// Touch: each grows by exactly ×1.5 (rounded).
	SetDensity(DensityTouch)
	assertEq(t, "touch ComboBox.rowH", cb.rowH(), 27)                       // 18*1.5
	assertEq(t, "touch DropDown.rowH", dd.rowH(), 27)                       // 18*1.5
	assertEq(t, "touch DropDown.PopoverBounds.H", dd.PopoverBounds().H, 81) // 3*27
	assertEq(t, "touch DatePickerFieldH", DatePickerFieldH(), gh+15)        // pad 10->15
	assertEq(t, "touch ColorPicker.alpha.W", cp.alphaRectLocal().W, 219)    // 180+12+27
	assertEq(t, "touch SearchEntryPadX", scaled(SearchEntryPadX), 6)        // 4*1.5
	assertEq(t, "touch SearchEntryIconW", scaled(SearchEntryIconW), 24)     // 16*1.5
}

// assertEq is a tiny exact-integer helper keeping the metric table readable.
func assertEq(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", name, got, want)
	}
}
