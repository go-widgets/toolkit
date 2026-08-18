// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// This file is the Axis-B (touch density) proof for the CONTROLS + CHROME
// widget family. Every case asserts BOTH halves of the density contract:
//
//   - DensityCompact: metrics and hit rects are EXACTLY their pre-change values
//     (byte-identical — the whole historical suite must, and does, stay green).
//   - DensityTouch: metrics scale by EXACTLY 1.5 (round(base*1.5)) and interactive
//     hit dimensions clamp to EXACTLY the 44-logical-pixel finger floor, with the
//     precise resulting numbers asserted — never merely "it got bigger".
//
// All cases run through defer restoreDensity() (density_test.go) so a failure can
// never leak a non-default density into the rest of the suite.

// touchHitWidget is the shared shape of a control that clamps its whole-widget
// hit area up to the density finger floor: it draws within Bounds but exposes a
// (possibly larger) HitRect that HitTest consults.
type touchHitWidget interface {
	Bounds() Rect
	SetBounds(r Rect)
	HitTest(px, py int) bool
	HitRect() Rect
}

// TestControlsTouchHitClamp is the whole-family hit-clamp proof. Every
// single-target control is given the SAME small 30x16 drawn bounds so one exact
// expected rect covers them all, and the Switch (whose clamp density_test.go
// already proves independently) is included as the KNOWN-GOOD CONTROL: the
// generic touchHitRect seam must reproduce, widget-for-widget, the exact rect the
// hand-written Switch worked example produces. That is the control-run for the
// shared helper — a new probe validated against a proven one before it is trusted
// across the family.
func TestControlsTouchHitClamp(t *testing.T) {
	defer restoreDensity()

	// The drawn bounds shared by every case, and the exact clamped+centred rect
	// the 44px floor yields for it: X 100-(44-30)/2=93, Y 200-(44-16)/2=186, 44x44.
	// These are the very numbers TestSwitchHitRectDemonstratesTouchTarget pins.
	drawn := Rect{X: 100, Y: 200, W: 30, H: 16}
	wantTouch := Rect{X: 93, Y: 186, W: 44, H: 44}

	// A point OUTSIDE the drawn 30x16 box but INSIDE the 44x44 touch rect: it must
	// miss at compact (hit area == drawn bounds) and land at touch (area grown).
	const outerX, outerY = 94, 205
	// A point well outside even the touch rect: it must miss at both densities.
	const farX, farY = 40, 205
	// A point inside the drawn box: it must hit at both densities.
	const innerX, innerY = 110, 205

	cases := []struct {
		name string
		w    touchHitWidget
	}{
		{"Switch(control)", NewSwitch(false)},
		{"Button", NewButton("B", nil)},
		{"IconButton", NewIconButton("+", nil)},
		{"CycleButton", NewCycleButton("a", "b")},
		{"ToggleButton", NewToggleButton("T", false)},
		{"CheckButton", NewCheckButton("C", false)},
		{"RadioButton", NewRadioButton("R")},
		{"SplitButton", NewSplitButton("S", nil)},
		{"Rating", NewRating(3, 5)},
	}
	for _, tc := range cases {
		tc.w.SetBounds(drawn)

		// --- DensityCompact: byte-identical to the drawn bounds. ---
		SetDensity(DensityCompact)
		if got := tc.w.HitRect(); got != drawn {
			t.Errorf("%s: compact HitRect = %+v, want the drawn bounds %+v (byte-identical)", tc.name, got, drawn)
		}
		if !tc.w.HitTest(innerX, innerY) {
			t.Errorf("%s: compact HitTest(%d,%d) = false, want true (inside drawn bounds)", tc.name, innerX, innerY)
		}
		if tc.w.HitTest(outerX, outerY) {
			t.Errorf("%s: compact HitTest(%d,%d) = true, want false (outside drawn bounds, no clamp)", tc.name, outerX, outerY)
		}

		// --- DensityTouch: exact 44x44 clamp centred over the unchanged bounds. ---
		SetDensity(DensityTouch)
		if got := tc.w.HitRect(); got != wantTouch {
			t.Errorf("%s: touch HitRect = %+v, want %+v (clamped to 44 + centred)", tc.name, got, wantTouch)
		}
		if got := tc.w.Bounds(); got != drawn {
			t.Errorf("%s: touch must NOT move the drawn bounds; got %+v, want %+v", tc.name, got, drawn)
		}
		if !tc.w.HitTest(innerX, innerY) {
			t.Errorf("%s: touch HitTest(%d,%d) = false, want true", tc.name, innerX, innerY)
		}
		if !tc.w.HitTest(outerX, outerY) {
			t.Errorf("%s: touch HitTest(%d,%d) = false, want true (enlarged finger target)", tc.name, outerX, outerY)
		}
		if tc.w.HitTest(farX, farY) {
			t.Errorf("%s: touch HitTest(%d,%d) = true, want false (beyond the 44px rect)", tc.name, farX, farY)
		}
	}
}

// TestFamilyRoutedMetricsExact pins, for EVERY box metric this family routes
// through scaled, the exact device value at compact (== the logical constant,
// byte-identical) and at touch (== round(constant*1.5)). Because each widget's
// Draw now derives that metric from scaled(constant) — proven to be actually used
// by TestMetricScaleAudit, which sees each interior double at 2x — these exact
// numbers are the widgets' drawn metrics, not a re-test of scaled() in isolation.
func TestFamilyRoutedMetricsExact(t *testing.T) {
	defer restoreDensity()

	cases := []struct {
		name              string
		base, wantTouch15 int
	}{
		{"buttonRadius", buttonRadius, 9},                   // 6 * 1.5
		{"IconButtonSize", IconButtonSize, 42},              // 28 * 1.5
		{"SplitButtonArrowW", SplitButtonArrowW, 30},        // 20 * 1.5
		{"checkBoxSize", checkBoxSize, 18},                  // 12 * 1.5
		{"checkLabelGap", checkLabelGap, 6},                 // 4 * 1.5
		{"radioBoxSize", radioBoxSize, 18},                  // 12 * 1.5
		{"radioDotInset", radioDotInset, 5},                 // 3 * 1.5 = 4.5 -> 5
		{"radioLabelGap", radioLabelGap, 6},                 // 4 * 1.5
		{"switchPad", switchPad, 3},                         // 2 * 1.5
		{"ChipPadX", ChipPadX, 12},                          // 8 * 1.5
		{"ChipPadY", ChipPadY, 3},                           // 2 * 1.5
		{"ChipCloseW", ChipCloseW, 18},                      // 12 * 1.5
		{"ChipCloseGap", ChipCloseGap, 6},                   // 4 * 1.5
		{"ChipDotD", ChipDotD, 9},                           // 6 * 1.5
		{"ChipDotGap", ChipDotGap, 6},                       // 4 * 1.5
		{"RatingStarW", RatingStarW, 21},                    // 14 * 1.5
		{"RatingStarGap", RatingStarGap, 3},                 // 2 * 1.5
		{"spinButtonW", spinButtonW, 24},                    // 16 * 1.5
		{"spinTextPad", spinTextPad, 6},                     // 4 * 1.5
		{"ToolbarButtonW", ToolbarButtonW, 36},              // 24 * 1.5
		{"ToolbarButtonH", ToolbarButtonH, 36},              // 24 * 1.5
		{"ToolbarSepW", ToolbarSepW, 12},                    // 8 * 1.5
		{"HeaderBarPad", HeaderBarPad, 12},                  // 8 * 1.5
		{"HeaderBarSubtitleGap", HeaderBarSubtitleGap, 3},   // 2 * 1.5
		{"StatusbarSegmentMinW", StatusbarSegmentMinW, 120}, // 80 * 1.5
		{"StatusbarPadX", StatusbarPadX, 9},                 // 6 * 1.5
		{"scrollbarMinThumb", scrollbarMinThumb, 36},        // 24 * 1.5 (before the 44 hit floor)
	}
	for _, tc := range cases {
		SetDensity(DensityCompact)
		if got := scaled(tc.base); got != tc.base {
			t.Errorf("%s: compact scaled(%d) = %d, want %d (byte-identical)", tc.name, tc.base, got, tc.base)
		}
		SetDensity(DensityTouch)
		if got := scaled(tc.base); got != tc.wantTouch15 {
			t.Errorf("%s: touch scaled(%d) = %d, want %d (exactly base*1.5)", tc.name, tc.base, got, tc.wantTouch15)
		}
	}
}

// TestIconButtonAutoSizeScalesWithDensity proves the observable auto-size grows
// by exactly 1.5: a zero-sized IconButton resolves to 28x28 at compact and 42x42
// at touch through its first Draw.
func TestIconButtonAutoSizeScalesWithDensity(t *testing.T) {
	defer restoreDensity()
	theme := DefaultLight()
	buf := makeSurface(64, 64)
	p := newP(buf, 64)

	SetDensity(DensityCompact)
	c := NewIconButton("+", nil)
	c.Draw(p, theme)
	if b := c.Bounds(); b.W != 28 || b.H != 28 {
		t.Fatalf("compact auto-size = %dx%d, want 28x28", b.W, b.H)
	}

	SetDensity(DensityTouch)
	tch := NewIconButton("+", nil)
	tch.Draw(p, theme)
	if b := tch.Bounds(); b.W != 42 || b.H != 42 {
		t.Fatalf("touch auto-size = %dx%d, want 42x42 (28*1.5)", b.W, b.H)
	}
}

// TestChipAutoSizeAndCloseSlotScale proves both the chip's auto-sized width and
// its close-slot hit region scale exactly. The non-text width is
// 2*pad+closeGap+closeW: 32 at compact, 48 at touch; the shared text width cancels
// out so the difference is purely the scaled pads. The close slot's tap zone
// widens from [40,52) to the finger-floored [4,48).
func TestChipAutoSizeAndCloseSlotScale(t *testing.T) {
	defer restoreDensity()
	theme := DefaultLight()
	buf := makeSurface(200, 64)
	p := newP(buf, 64)

	measure := (&Base{}).textWidth("x") // shared across densities (same font)

	SetDensity(DensityCompact)
	cc := NewClosableChip("x", nil)
	cc.SetBounds(Rect{})
	cc.Draw(p, theme)
	if got, want := cc.Bounds().W, measure+32; got != want {
		t.Fatalf("compact chip auto width = %d, want %d (text + 2*8 + 4 + 12)", got, want)
	}

	SetDensity(DensityTouch)
	tc := NewClosableChip("x", nil)
	tc.SetBounds(Rect{})
	tc.Draw(p, theme)
	if got, want := tc.Bounds().W, measure+48; got != want {
		t.Fatalf("touch chip auto width = %d, want %d (text + 2*12 + 6 + 18)", got, want)
	}

	// Close-slot hit boundaries, W=60: compact drawn slot [40,52); touch [4,48).
	slot := func(density DensityLevel) (fires []int) {
		SetDensity(density)
		for x := 0; x < 60; x++ {
			c := NewClosableChip("x", func() {})
			c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
			hit := false
			c.OnClose = func() { hit = true }
			c.OnEvent(Event{Kind: EventClick, X: x, Y: 8})
			if hit {
				fires = append(fires, x)
			}
		}
		return fires
	}
	compact := slot(DensityCompact)
	if len(compact) == 0 || compact[0] != 40 || compact[len(compact)-1] != 51 {
		t.Fatalf("compact close-slot fires on x=%v, want a contiguous [40,52) run", compact)
	}
	touch := slot(DensityTouch)
	if len(touch) == 0 || touch[0] != 4 || touch[len(touch)-1] != 47 {
		t.Fatalf("touch close-slot fires on x=%v, want the widened [4,48) run", touch)
	}
}

// TestSplitButtonArrowSplitScales proves the main/arrow click boundary moves from
// r.W-20 to r.W-30 at touch (the arrow slot grows by exactly 1.5), so the same
// press resolves to the arrow at touch where it hit the main action at compact.
func TestSplitButtonArrowSplitScales(t *testing.T) {
	defer restoreDensity()

	probe := func(density DensityLevel, x int) (main, arrow bool) {
		SetDensity(density)
		s := NewSplitButton("S", func() { main = true })
		s.OnArrow = func() { arrow = true }
		s.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 32})
		s.OnEvent(Event{Kind: EventClick, X: x, Y: 16})
		return main, arrow
	}

	// Compact boundary is at 160-20 = 140: X=139 main, X=140 arrow.
	if m, a := probe(DensityCompact, 139); !m || a {
		t.Fatalf("compact X=139: main=%v arrow=%v, want main only", m, a)
	}
	if m, a := probe(DensityCompact, 140); m || !a {
		t.Fatalf("compact X=140: main=%v arrow=%v, want arrow only", m, a)
	}
	// Touch boundary is at 160-30 = 130: X=129 main, X=130 arrow. X=135 flips from
	// main (compact) to arrow (touch).
	if m, a := probe(DensityTouch, 129); !m || a {
		t.Fatalf("touch X=129: main=%v arrow=%v, want main only", m, a)
	}
	if m, a := probe(DensityTouch, 130); m || !a {
		t.Fatalf("touch X=130: main=%v arrow=%v, want arrow only", m, a)
	}
	if m, a := probe(DensityTouch, 135); m || !a {
		t.Fatalf("touch X=135: main=%v arrow=%v, want arrow (was main at compact)", m, a)
	}
}

// TestRatingCellPitchScales proves the click-to-star pitch grows from 16 to 24 at
// touch, so a press at X=20 selects star 2 at compact but star 1 at touch, and the
// exact touch boundary sits at multiples of 24.
func TestRatingCellPitchScales(t *testing.T) {
	defer restoreDensity()

	click := func(density DensityLevel, x int) int {
		SetDensity(density)
		r := NewRating(0, 5)
		r.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 20})
		r.OnEvent(Event{Kind: EventClick, X: x, Y: 5})
		return r.Value().Get()
	}

	if got := click(DensityCompact, 20); got != 2 { // 20/16 = 1 -> star 2
		t.Fatalf("compact click X=20 -> value %d, want 2 (pitch 16)", got)
	}
	if got := click(DensityTouch, 20); got != 1 { // 20/24 = 0 -> star 1
		t.Fatalf("touch click X=20 -> value %d, want 1 (pitch 24)", got)
	}
	if got := click(DensityTouch, 23); got != 1 { // still cell 0
		t.Fatalf("touch click X=23 -> value %d, want 1", got)
	}
	if got := click(DensityTouch, 24); got != 2 { // exact cell-1 boundary
		t.Fatalf("touch click X=24 -> value %d, want 2 (cell boundary at 24)", got)
	}
}

// TestSpinButtonStepperHitWidensAtTouch proves the +/- click column, drawn
// scaled(16)=24 wide, exposes a 44px-floored tap zone at touch: a press at X=90
// (over the inert body at compact) lands the increment at touch, with the exact
// boundary at r.W-44.
func TestSpinButtonStepperHitWidensAtTouch(t *testing.T) {
	defer restoreDensity()

	click := func(density DensityLevel, x int) int {
		SetDensity(density)
		s := NewSpinButton(0, 100, 50, 1)
		s.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
		s.OnEvent(Event{Kind: EventClick, X: x, Y: 5}) // Y<12 -> increment half
		return s.Value().Get()
	}

	// Compact button column is [120-16,120) = [104,120): X=103 body (no-op), X=104
	// increments.
	if got := click(DensityCompact, 103); got != 50 {
		t.Fatalf("compact X=103 -> %d, want 50 (inert body)", got)
	}
	if got := click(DensityCompact, 104); got != 51 {
		t.Fatalf("compact X=104 -> %d, want 51 (button column edge)", got)
	}
	if got := click(DensityCompact, 90); got != 50 {
		t.Fatalf("compact X=90 -> %d, want 50 (body, no widening)", got)
	}
	// Touch column widens to [120-44,120) = [76,120): X=75 body, X=76 & X=90 both
	// increment.
	if got := click(DensityTouch, 75); got != 50 {
		t.Fatalf("touch X=75 -> %d, want 50 (just left of widened column)", got)
	}
	if got := click(DensityTouch, 76); got != 51 {
		t.Fatalf("touch X=76 -> %d, want 51 (widened column edge at r.W-44)", got)
	}
	if got := click(DensityTouch, 90); got != 51 {
		t.Fatalf("touch X=90 -> %d, want 51 (reachable at touch, inert at compact)", got)
	}
}

// TestToolbarDefaultButtonSizeScales proves the toolbar's DEFAULT square cell
// resolves through scaled (24 compact -> 36 touch) so hit-testing shifts, while an
// EXPLICIT ButtonW/ButtonH is honoured verbatim (never scaled), covering both
// resolution branches.
func TestToolbarDefaultButtonSizeScales(t *testing.T) {
	defer restoreDensity()
	items := []ToolbarItem{{Label: "A"}, {Label: "B"}}

	// Default cells: compact 24 -> item0 [0,24), item1 [24,48); touch 36 -> item0
	// [0,36). X=30 flips from item1 to item0.
	def := NewToolbar(items)
	def.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 36})
	SetDensity(DensityCompact)
	if got := def.hitTest(30, 10); got != 1 {
		t.Fatalf("compact hitTest(30,10) = %d, want item 1 (24px cells)", got)
	}
	SetDensity(DensityTouch)
	if got := def.hitTest(30, 10); got != 0 {
		t.Fatalf("touch hitTest(30,10) = %d, want item 0 (36px cells)", got)
	}

	// Explicit cells are verbatim at every density: 50x40 -> item0 [0,50), item1
	// [50,100), and cross-extent 40 (y must be < 40).
	fixed := NewToolbar(items)
	fixed.ButtonW = 50
	fixed.ButtonH = 40
	fixed.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 40})
	SetDensity(DensityTouch)
	if got := fixed.hitTest(30, 10); got != 0 {
		t.Fatalf("touch explicit hitTest(30,10) = %d, want item 0 (50px verbatim)", got)
	}
	if got := fixed.hitTest(55, 10); got != 1 {
		t.Fatalf("touch explicit hitTest(55,10) = %d, want item 1 (cell 1 starts at 50)", got)
	}
	if got := fixed.hitTest(30, 39); got != 0 {
		t.Fatalf("touch explicit hitTest(30,39) = %d, want item 0 (cross-extent 40 verbatim)", got)
	}
	if got := fixed.hitTest(30, 40); got != -1 {
		t.Fatalf("touch explicit hitTest(30,40) = %d, want -1 (below verbatim height 40)", got)
	}
}

// TestHeaderBarPaddingScales proves the bar's inner inset scales exactly: a Start
// child fitted to the bar goes from Y=4,H=32,X=8 at compact to Y=6,H=28,X=12 at
// touch (pad 8 -> 12), while its own width is preserved.
func TestHeaderBarPaddingScales(t *testing.T) {
	defer restoreDensity()

	fit := func(density DensityLevel) Rect {
		SetDensity(density)
		hb := NewHeaderBar("T")
		child := NewButton("x", nil)
		child.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 0})
		hb.Start = []Widget{child}
		hb.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 40}) // re-runs layout at this density
		return child.Bounds()
	}

	if got, want := fit(DensityCompact), (Rect{X: 8, Y: 4, W: 40, H: 32}); got != want {
		t.Fatalf("compact Start child = %+v, want %+v (pad 8)", got, want)
	}
	if got, want := fit(DensityTouch), (Rect{X: 12, Y: 6, W: 40, H: 28}); got != want {
		t.Fatalf("touch Start child = %+v, want %+v (pad 12)", got, want)
	}
}

// TestHeaderBarDispatchHonoursChildHitClamp proves the bar routes a press through
// its child's HitTest, so a small child's touch-enlarged hit rect is reachable
// inside the bar: a click just outside the drawn child but inside its 44px rect
// fires at touch and misses at compact.
func TestHeaderBarDispatchHonoursChildHitClamp(t *testing.T) {
	defer restoreDensity()

	build := func(density DensityLevel) (*HeaderBar, *Button, *bool) {
		SetDensity(density)
		hb := NewHeaderBar("T")
		clicked := new(bool)
		child := NewButton("x", func() { *clicked = true })
		// A deliberately small 20-wide child placed at the far left.
		child.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 0})
		hb.End = []Widget{child}
		hb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
		return hb, child, clicked
	}

	// Compact: the child occupies its drawn End slot; a press one pixel left of it
	// misses.
	hb, child, clicked := build(DensityCompact)
	cb := child.Bounds()
	hb.OnEvent(Event{Kind: EventClick, X: cb.X - 3, Y: cb.Y + cb.H/2})
	if *clicked {
		t.Fatalf("compact: press 3px left of the child should miss (no hit clamp)")
	}

	// Touch: the child's HitRect grows to 44 and centres, so the same relative
	// press now lands via the bar's HitTest dispatch.
	hb, child, clicked = build(DensityTouch)
	cb = child.Bounds()
	if hr := child.HitRect(); hr.W != 44 || hr.H != 44 {
		t.Fatalf("touch child HitRect = %+v, want 44x44", hr)
	}
	hb.OnEvent(Event{Kind: EventClick, X: cb.X - 3, Y: cb.Y + cb.H/2})
	if !*clicked {
		t.Fatalf("touch: the bar must dispatch through the child's enlarged HitRect")
	}
}

// TestStatusbarSegmentMinScales proves the default non-last segment minimum width
// scales exactly (80 -> 120): the divider Border column after an empty first
// segment moves from x=79 to x=119.
func TestStatusbarSegmentMinScales(t *testing.T) {
	defer restoreDensity()
	theme := DefaultLight()

	dividerX := func(density DensityLevel) int {
		SetDensity(density)
		buf := makeSurface(300, 18)
		p := newP(buf, 300)
		sb := NewStatusbar([]string{"", ""}) // empty texts: only the divider is Border
		sb.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 18})
		sb.Draw(p, theme)
		// Scan from x=5 so the left perimeter stroke (which is itself scaled — 1px
		// at compact, 2px at touch) is never mistaken for the divider; the divider
		// sits far to the right at both densities (79 / 119).
		for x := 5; x < 295; x++ {
			if pixelAt(buf, 300, x, 9) == theme.Border {
				return x
			}
		}
		return -1
	}

	if got := dividerX(DensityCompact); got != 79 {
		t.Fatalf("compact divider column = %d, want 79 (segment min 80)", got)
	}
	if got := dividerX(DensityTouch); got != 119 {
		t.Fatalf("touch divider column = %d, want 119 (segment min 120)", got)
	}
}

// TestScrollbarMinThumbFloorsAtTouch proves the grabbable thumb is never smaller
// than a fingertip: with tiny viewport in huge content the minimum thumb is the
// historical 24 at compact, 36 at comfortable (scaled 24) floored to that level's
// hit target, and exactly the 44px finger floor at touch.
func TestScrollbarMinThumbFloorsAtTouch(t *testing.T) {
	defer restoreDensity()

	thumbH := func(density DensityLevel) int {
		SetDensity(density)
		sb := NewScrollbar()
		sb.Total = 10000
		sb.Viewport = 10
		sb.SetBounds(Rect{X: 0, Y: 0, W: 16, H: 200})
		return sb.ThumbRect().H
	}

	if got := thumbH(DensityCompact); got != 24 { // scaled(24)=24, no hit floor
		t.Fatalf("compact min thumb = %d, want 24", got)
	}
	if got := thumbH(DensityComfortable); got != 36 { // max(scaled(24)=30, floor 36)
		t.Fatalf("comfortable min thumb = %d, want 36 (hit floor wins)", got)
	}
	if got := thumbH(DensityTouch); got != 44 { // max(scaled(24)=36, floor 44)
		t.Fatalf("touch min thumb = %d, want 44 (finger floor wins)", got)
	}
}
