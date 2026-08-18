// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// countColorInRect (defined in agenda_calendars_test.go) counts how many pixels
// inside r exactly match c, letting a test assert "this colour appears only
// here" precisely instead of "something painted".

// --- API surface / constants ---------------------------------------------

func TestTabBarConstants(t *testing.T) {
	if TabBarHeight != 56 || TabBarIndicatorH != 3 || TabBarIconLabelGap != 2 {
		t.Fatalf("constants drifted: H=%d indicator=%d gap=%d",
			TabBarHeight, TabBarIndicatorH, TabBarIconLabelGap)
	}
	if RoleTab != "tab" {
		t.Fatalf("RoleTab = %q, want \"tab\"", RoleTab)
	}
}

func sampleItems() []TabItem {
	return []TabItem{
		{Icon: "H", Label: "Home"},
		{Icon: "S", Label: "Search"},
		{Icon: "P", Label: "Profile"},
		{Icon: "M", Label: "More"},
	}
}

func TestNewTabBarClamps(t *testing.T) {
	if b := NewTabBar(nil, 7); b.Selected != 0 {
		t.Fatalf("empty items selected = %d, want 0", b.Selected)
	}
	if b := NewTabBar(sampleItems(), -5); b.Selected != 0 {
		t.Fatalf("negative selected = %d, want 0", b.Selected)
	}
	if b := NewTabBar(sampleItems(), 99); b.Selected != 3 {
		t.Fatalf("overshoot selected = %d, want 3", b.Selected)
	}
	if b := NewTabBar(sampleItems(), 2); b.Selected != 2 {
		t.Fatalf("in-range selected = %d, want 2", b.Selected)
	}
	if b := NewTabBar(sampleItems(), 0); b.Gestures == nil {
		t.Fatal("NewTabBar left Gestures nil")
	}
}

// --- Geometry: ItemRect / itemWidth --------------------------------------

func TestTabBarItemRectExact(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	want := []Rect{
		{X: 0, Y: 0, W: 50, H: 56},
		{X: 50, Y: 0, W: 50, H: 56},
		{X: 100, Y: 0, W: 50, H: 56},
		{X: 150, Y: 0, W: 50, H: 56},
	}
	for i, w := range want {
		if got := b.ItemRect(i); got != w {
			t.Fatalf("ItemRect(%d) = %+v, want %+v", i, got, w)
		}
	}
}

func TestTabBarItemRectSurfaceOffset(t *testing.T) {
	// Bounds offset from the origin: ItemRect is in SURFACE coords, so its X
	// carries the bar's X, while hit-testing (selectAt) still uses the
	// widget-local x delivered in an event.
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 56})
	if got, want := b.ItemRect(2), (Rect{X: 110, Y: 5, W: 50, H: 56}); got != want {
		t.Fatalf("ItemRect(2) offset = %+v, want %+v", got, want)
	}
}

func TestTabBarItemRectRemainderStaysRight(t *testing.T) {
	// 203 / 4 = 50 with 3 leftover pixels: every column is exactly 50 wide and
	// the last one stops at 200, leaving the 3-px remainder as background.
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 203, H: 56})
	if got, want := b.ItemRect(3), (Rect{X: 150, Y: 0, W: 50, H: 56}); got != want {
		t.Fatalf("ItemRect(3) = %+v, want %+v", got, want)
	}
}

func TestTabBarItemRectOutOfRange(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	if got := b.ItemRect(-1); got != (Rect{}) {
		t.Fatalf("ItemRect(-1) = %+v, want zero", got)
	}
	if got := b.ItemRect(4); got != (Rect{}) {
		t.Fatalf("ItemRect(4) = %+v, want zero", got)
	}
	// Zero-width bar: itemWidth 0 => zero rect.
	b.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 56})
	if got := b.ItemRect(0); got != (Rect{}) {
		t.Fatalf("ItemRect on zero-width bar = %+v, want zero", got)
	}
	// No items at all: itemWidth 0.
	if iw := NewTabBar(nil, 0).itemWidth(); iw != 0 {
		t.Fatalf("itemWidth with no items = %d, want 0", iw)
	}
}

// --- Draw: empty bar -----------------------------------------------------

func TestTabBarDrawEmpty(t *testing.T) {
	const w, h = 120, 56
	theme := DefaultLight()
	b := NewTabBar(nil, 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	// Background band is SurfaceAlt below the top border.
	if got := pixelAt(buf, w, 60, 20); got != theme.SurfaceAlt {
		t.Fatalf("empty background = %+v, want SurfaceAlt", got)
	}
	// Top border row is Border.
	if got := pixelAt(buf, w, 60, 0); got != theme.Border {
		t.Fatalf("top border = %+v, want Border", got)
	}
	// No Accent or OnSurface ink anywhere on an empty bar.
	if n := countColorInRect(buf, w, b.Bounds(), theme.Accent); n != 0 {
		t.Fatalf("empty bar has %d Accent pixels, want 0", n)
	}
	if n := countColorInRect(buf, w, b.Bounds(), theme.OnSurface); n != 0 {
		t.Fatalf("empty bar has %d OnSurface pixels, want 0", n)
	}
}

// --- Draw: selected indicator + per-item ink -----------------------------

func TestTabBarSelectedIndicatorExact(t *testing.T) {
	const w, h = 200, 56
	theme := DefaultLight()
	b := NewTabBar(sampleItems(), 1) // select item 1: column [50,100)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	// The indicator band is EXACTLY item 1's column, TabBarIndicatorH tall,
	// entirely Accent (no glyph reaches into the top band).
	band := Rect{X: 50, Y: 0, W: 50, H: scaled(TabBarIndicatorH)}
	if n := countColorInRect(buf, w, band, theme.Accent); n != band.W*band.H {
		t.Fatalf("indicator band Accent pixels = %d, want %d (full band)", n, band.W*band.H)
	}
	// An unselected column's top band carries only the Border row (y=0), never
	// an Accent indicator.
	unselBand := Rect{X: 0, Y: 0, W: 50, H: scaled(TabBarIndicatorH)}
	if n := countColorInRect(buf, w, unselBand, theme.Accent); n != 0 {
		t.Fatalf("unselected column band has %d Accent pixels, want 0", n)
	}
	if got := pixelAt(buf, w, 5, 0); got != theme.Border {
		t.Fatalf("unselected top row = %+v, want Border", got)
	}
	// Just below the border, an unselected column is plain background.
	if got := pixelAt(buf, w, 5, 1); got != theme.SurfaceAlt {
		t.Fatalf("unselected column below border = %+v, want SurfaceAlt", got)
	}
}

func TestTabBarSelectedInkVsUnselected(t *testing.T) {
	const w, h = 200, 56
	theme := DefaultLight()
	b := NewTabBar(sampleItems(), 1)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	// Below the indicator band so we only see glyph ink, not the indicator.
	sel := Rect{X: 50, Y: scaled(TabBarIndicatorH), W: 50, H: h - scaled(TabBarIndicatorH)}
	unsel := Rect{X: 0, Y: scaled(TabBarIndicatorH), W: 50, H: h - scaled(TabBarIndicatorH)}

	// Selected column: glyphs are Accent, never OnSurface.
	if n := countColorInRect(buf, w, sel, theme.Accent); n == 0 {
		t.Fatal("selected column has no Accent glyph pixels")
	}
	if n := countColorInRect(buf, w, sel, theme.OnSurface); n != 0 {
		t.Fatalf("selected column has %d OnSurface pixels, want 0", n)
	}
	// Unselected column: glyphs are OnSurface, never Accent.
	if n := countColorInRect(buf, w, unsel, theme.OnSurface); n == 0 {
		t.Fatal("unselected column has no OnSurface glyph pixels")
	}
	if n := countColorInRect(buf, w, unsel, theme.Accent); n != 0 {
		t.Fatalf("unselected column has %d Accent pixels, want 0", n)
	}
}

func TestTabBarIconOnlyItemCentersGlyph(t *testing.T) {
	// No label: the icon is vertically centred in the full bar height.
	const w, h = 100, 56
	theme := DefaultLight()
	b := NewTabBar([]TabItem{{Icon: "A"}, {Icon: "B"}}, 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	gh := GlyphHeight()
	iconY := (h - gh) / 2 // 24
	// Glyph ink for item 1 (unselected, OnSurface) lands within [iconY, iconY+gh).
	strip := Rect{X: 50, Y: iconY, W: 50, H: gh}
	if n := countColorInRect(buf, w, strip, theme.OnSurface); n == 0 {
		t.Fatalf("icon-only glyph not found in centred strip y=[%d,%d)", iconY, iconY+gh)
	}
	// Nothing above the centred strip (bar top area is clean apart from border).
	above := Rect{X: 50, Y: scaled(TabBarIndicatorH), W: 50, H: iconY - scaled(TabBarIndicatorH)}
	if n := countColorInRect(buf, w, above, theme.OnSurface); n != 0 {
		t.Fatalf("icon-only glyph leaked above centred strip: %d px", n)
	}
}

// --- Draw: badge placement -----------------------------------------------

func TestTabBarBadgeSizeMatchesStandaloneBadge(t *testing.T) {
	// badgeSize must equal what a standalone Badge auto-sizes to, so a counter
	// in the bar is pixel-identical to one anywhere else.
	b := NewTabBar(nil, 0)
	for _, text := range []string{"9", "12", "99+"} {
		bw, bh := b.badgeSize(text)
		badge := NewBadge(text)
		badge.Draw(newP(makeSurface(60, 20), 60), DefaultLight())
		if got := badge.Bounds(); got.W != bw || got.H != bh {
			t.Fatalf("badgeSize(%q) = %dx%d, standalone Badge = %dx%d",
				text, bw, bh, got.W, got.H)
		}
	}
}

func TestTabBarBadgeExactPlacement(t *testing.T) {
	const w, h = 200, 56
	theme := DefaultLight()
	items := sampleItems()
	items[1].Badge = "9" // badge on the UNSELECTED item 1 => only Accent there is the pill
	b := NewTabBar(items, 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	// Recompute the expected pill rect exactly as drawItem does.
	vr := b.ItemRect(1)
	gh := GlyphHeight()
	iconW := GlyphAdvance() * len(items[1].Icon)
	iconX := vr.X + (vr.W-iconW)/2
	gap := scaled(TabBarIconLabelGap)
	contentTop := vr.Y + (vr.H-(gh+gap+gh))/2
	iconY := contentTop
	bw, bh := b.badgeSize("9")
	bx := iconX + iconW - bw/2
	by := iconY - bh/2

	// The pill body (Accent) is present at its vertical middle-left edge.
	if got := pixelAt(buf, w, bx, by+bh/2); got != theme.Accent {
		t.Fatalf("badge body at (%d,%d) = %+v, want Accent", bx, by+bh/2, got)
	}
	// No Accent a few pixels to the LEFT of the pill: proves the left bound.
	if got := pixelAt(buf, w, bx-3, by+bh/2); got == theme.Accent {
		t.Fatalf("unexpected Accent left of badge at x=%d", bx-3)
	}
	// Item 0 is selected (its column carries the accent indicator + ink), so the
	// ONLY accent in the unselected columns 1..3 is item 1's badge pill — and it
	// must all sit inside item 1's own column (the clamp holds, nothing leaks to
	// columns 2 or 3).
	col1 := vr
	unselected := Rect{X: 50, Y: 0, W: 150, H: h}
	inCol1 := countColorInRect(buf, w, col1, theme.Accent)
	inUnselected := countColorInRect(buf, w, unselected, theme.Accent)
	if inCol1 == 0 {
		t.Fatal("badge drew no Accent pixels in item 1")
	}
	if inCol1 != inUnselected {
		t.Fatalf("badge Accent escaped item 1 column: in-col=%d in-unselected=%d", inCol1, inUnselected)
	}
}

func TestTabBarBadgeClampsIntoColumn(t *testing.T) {
	// A wide badge on a narrow, short, icon-only item forces every clamp branch
	// (right edge, left edge, and top edge). The contract: no badge pixel ever
	// escapes the item column.
	const w, h = 20, 8
	theme := DefaultLight()
	b := NewTabBar([]TabItem{{Icon: "I", Badge: "999"}}, 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)

	// Selected single item => Accent is indicator + badge body; all must be in-column.
	col := b.ItemRect(0)
	if in, all := countColorInRect(buf, w, col, theme.Accent),
		countColorInRect(buf, w, b.Bounds(), theme.Accent); in != all {
		t.Fatalf("badge/indicator Accent escaped column: in=%d all=%d", in, all)
	}
}

// --- OnEvent: tap selects, fires once ------------------------------------

func TestTabBarTapSelectsExactlyOnce(t *testing.T) {
	const w = 200
	for i := 0; i < 4; i++ {
		b := NewTabBar(sampleItems(), 0)
		b.SetBounds(Rect{X: 0, Y: 0, W: w, H: 56})
		calls := 0
		gotIdx := -1
		b.OnSelect = func(idx int) { calls++; gotIdx = idx }
		localX := i*50 + 25 // centre of column i
		b.OnEvent(Event{Kind: EventClick, X: localX})
		if b.Selected != i {
			t.Fatalf("tap col %d: Selected = %d, want %d", i, b.Selected, i)
		}
		if calls != 1 || gotIdx != i {
			t.Fatalf("tap col %d: calls=%d idx=%d, want calls=1 idx=%d", i, calls, gotIdx, i)
		}
	}
}

func TestTabBarTapRemainderAndOutOfRange(t *testing.T) {
	b := NewTabBar(sampleItems(), 1)
	b.SetBounds(Rect{X: 0, Y: 0, W: 203, H: 56}) // 3-px remainder past x=200
	calls := 0
	b.OnSelect = func(int) { calls++ }
	b.OnEvent(Event{Kind: EventClick, X: 201}) // in the remainder region
	if calls != 0 || b.Selected != 1 {
		t.Fatalf("remainder tap: calls=%d selected=%d, want 0/1", calls, b.Selected)
	}
	b.OnEvent(Event{Kind: EventClick, X: -4}) // negative local x
	if calls != 0 || b.Selected != 1 {
		t.Fatalf("negative tap: calls=%d selected=%d, want 0/1", calls, b.Selected)
	}
	// Zero-width bar: no column to hit.
	b.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 56})
	b.OnEvent(Event{Kind: EventClick, X: 0})
	if calls != 0 {
		t.Fatalf("zero-width tap fired %d times, want 0", calls)
	}
	// Empty bar: selectAt short-circuits on n==0.
	eb := NewTabBar(nil, 0)
	eb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 56})
	eb.OnSelect = func(int) { t.Fatal("empty bar tap fired OnSelect") }
	eb.OnEvent(Event{Kind: EventClick, X: 5})
}

func TestTabBarTapUsesLocalCoords(t *testing.T) {
	// Bar offset in the surface; the event x is widget-local, so column 0 is hit
	// at local x=25 regardless of the bar's surface X.
	b := NewTabBar(sampleItems(), 3)
	b.SetBounds(Rect{X: 40, Y: 0, W: 200, H: 56})
	b.OnEvent(Event{Kind: EventClick, X: 25})
	if b.Selected != 0 {
		t.Fatalf("local-coord tap: Selected = %d, want 0", b.Selected)
	}
}

func TestTabBarOnSelectNilSafe(t *testing.T) {
	b := NewTabBar(sampleItems(), 0) // OnSelect nil
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	b.OnEvent(Event{Kind: EventClick, X: 125}) // must not panic
	if b.Selected != 2 {
		t.Fatalf("nil OnSelect tap: Selected = %d, want 2", b.Selected)
	}
}

// --- OnEvent: keyboard stepping ------------------------------------------

func TestTabBarArrowKeysStepClamped(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	calls := 0
	b.OnSelect = func(int) { calls++ }

	// Left at index 0 clamps => no change, no fire.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if b.Selected != 0 || calls != 0 {
		t.Fatalf("clamp-left: selected=%d calls=%d, want 0/0", b.Selected, calls)
	}
	// Right advances and fires once.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if b.Selected != 1 || calls != 1 {
		t.Fatalf("right: selected=%d calls=%d, want 1/1", b.Selected, calls)
	}
	// ArrowDown is an alias for forward.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if b.Selected != 2 {
		t.Fatalf("down alias: selected=%d, want 2", b.Selected)
	}
	// ArrowUp is an alias for backward.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if b.Selected != 1 {
		t.Fatalf("up alias: selected=%d, want 1", b.Selected)
	}
	// Jump to last, then Right clamps.
	b.Selected = 3
	before := calls
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if b.Selected != 3 || calls != before {
		t.Fatalf("clamp-right: selected=%d calls delta=%d, want 3/0", b.Selected, calls-before)
	}
	// An unhandled key is a no-op.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if b.Selected != 3 {
		t.Fatalf("Enter changed selection to %d", b.Selected)
	}
}

func TestTabBarStepEmptyNoop(t *testing.T) {
	b := NewTabBar(nil, 0)
	b.OnSelect = func(int) { t.Fatal("empty step fired OnSelect") }
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if b.Selected != 0 {
		t.Fatalf("empty step selected = %d, want 0", b.Selected)
	}
}

// --- OnEvent: touch tap + swipe navigation -------------------------------

func TestTabBarTouchTapSelects(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	calls := 0
	b.OnSelect = func(int) { calls++ }
	// A touch that lands and lifts in the same spot is a tap on column 1.
	b.OnEvent(Event{Kind: EventTouchStart, X: 75, Y: 30, Code: "t0"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 75, Y: 30, Code: "t0"})
	if b.Selected != 1 || calls != 1 {
		t.Fatalf("touch tap: selected=%d calls=%d, want 1/1", b.Selected, calls)
	}
}

func TestTabBarSwipeNavigationGated(t *testing.T) {
	b := NewTabBar(sampleItems(), 1)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	// SwipeNavigation off (default): a swipe does nothing.
	b.OnEvent(Event{Kind: EventTouchStart, X: 100, Y: 30, Code: "s"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 60, Y: 30, Code: "s"}) // dx=-40 => SwipeLeft
	if b.Selected != 1 {
		t.Fatalf("gated-off swipe changed selection to %d", b.Selected)
	}

	// Turn it on: SwipeLeft advances, SwipeRight retreats, clamped.
	b.SwipeNavigation = true
	b.OnEvent(Event{Kind: EventTouchStart, X: 100, Y: 30, Code: "s"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 60, Y: 30, Code: "s"}) // SwipeLeft => +1
	if b.Selected != 2 {
		t.Fatalf("swipe-left: selected=%d, want 2", b.Selected)
	}
	b.OnEvent(Event{Kind: EventTouchStart, X: 60, Y: 30, Code: "s"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 100, Y: 30, Code: "s"}) // SwipeRight => -1
	if b.Selected != 1 {
		t.Fatalf("swipe-right: selected=%d, want 1", b.Selected)
	}
	// A vertical swipe is ignored by the horizontal-only navigation.
	b.OnEvent(Event{Kind: EventTouchStart, X: 100, Y: 10, Code: "s"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 100, Y: 50, Code: "s"}) // SwipeDown
	if b.Selected != 1 {
		t.Fatalf("vertical swipe changed selection to %d", b.Selected)
	}
}

func TestTabBarTouchWithNilGesturesNoPanic(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	b.Gestures = nil // must not panic when a touch arrives
	b.OnEvent(Event{Kind: EventTouchStart, X: 10, Y: 10, Code: "x"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 10, Y: 10, Code: "x"})
}

func TestTabBarIgnoresUnrelatedEvent(t *testing.T) {
	b := NewTabBar(sampleItems(), 2)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	b.OnSelect = func(int) { t.Fatal("scroll event fired OnSelect") }
	b.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if b.Selected != 2 {
		t.Fatalf("scroll changed selection to %d", b.Selected)
	}
}

// --- Disabled -------------------------------------------------------------

func TestTabBarDisabledIgnoresInput(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	b.SwipeNavigation = true
	b.Disabled = true
	b.OnSelect = func(int) { t.Fatal("disabled bar fired OnSelect") }
	b.OnEvent(Event{Kind: EventClick, X: 125})
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	b.OnEvent(Event{Kind: EventTouchStart, X: 100, Y: 30, Code: "d"})
	b.OnEvent(Event{Kind: EventTouchEnd, X: 60, Y: 30, Code: "d"})
	if b.Selected != 0 {
		t.Fatalf("disabled bar selection moved to %d", b.Selected)
	}
}

func TestTabBarDisabledDrawIsMuted(t *testing.T) {
	const w, h = 200, 56
	theme := DefaultLight()
	b := NewTabBar(sampleItems(), 1)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b.Disabled = true
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	// No pure Accent anywhere: the indicator and selected ink are muted.
	if n := countColorInRect(buf, w, b.Bounds(), theme.Accent); n != 0 {
		t.Fatalf("disabled bar has %d Accent pixels, want 0", n)
	}
	// The muted ink appears (glyphs still render, just greyed).
	if n := countColorInRect(buf, w, b.Bounds(), mutedInk(theme)); n == 0 {
		t.Fatal("disabled bar drew no muted ink")
	}
}

// --- Density: auto-size + hit targets ------------------------------------

func TestTabBarAutoSizeHeightPerDensity(t *testing.T) {
	defer restoreDensity()
	cases := []struct {
		level DensityLevel
		wantH int
	}{
		{DensityCompact, 56},     // scaled(56)=56, TouchTarget passthrough
		{DensityComfortable, 70}, // round(56*1.25)=70
		{DensityTouch, 84},       // round(56*1.5)=84
	}
	for _, c := range cases {
		SetDensity(c.level)
		b := NewTabBar(sampleItems(), 0)
		b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 0}) // zero H => auto
		b.Draw(newP(makeSurface(200, 200), 200), DefaultLight())
		if got := b.Bounds().H; got != c.wantH {
			t.Fatalf("density %d auto height = %d, want %d", c.level, got, c.wantH)
		}
		if got := b.Bounds().H; got < 44 {
			t.Fatalf("density %d auto height %d below the 44px touch floor", c.level, got)
		}
	}
}

func TestTabBarAutoSizeKeepsSetHeight(t *testing.T) {
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 64})
	b.Draw(newP(makeSurface(200, 64), 200), DefaultLight())
	if got := b.Bounds().H; got != 64 {
		t.Fatalf("pre-set height changed to %d, want 64", got)
	}
}

func TestTabBarItemHitRectCompactPassthrough(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityCompact)
	b := NewTabBar(sampleItems(), 0)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	for i := 0; i < 4; i++ {
		if got, want := b.ItemHitRect(i), b.ItemRect(i); got != want {
			t.Fatalf("compact ItemHitRect(%d) = %+v, want visual %+v", i, got, want)
		}
	}
	// Out-of-range mirrors ItemRect.
	if got := b.ItemHitRect(9); got != (Rect{}) {
		t.Fatalf("ItemHitRect(9) = %+v, want zero", got)
	}
}

func TestTabBarItemHitRectTouchClampsNarrowItem(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch) // MinHitTarget = 44 at MetricScale 1
	b := NewTabBar(sampleItems(), 0)
	// 200/5-ish: use 5 items so a column is 40px wide, below the 44 floor.
	b.Items = []TabItem{{Icon: "A"}, {Icon: "B"}, {Icon: "C"}, {Icon: "D"}, {Icon: "E"}}
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56}) // itemW=40, height 56>44 passthrough
	// Item 0 visual rect {0,0,40,56}; hit width clamps up to 44, centred => X=-2.
	if got, want := b.ItemHitRect(0), (Rect{X: -2, Y: 0, W: 44, H: 56}); got != want {
		t.Fatalf("touch ItemHitRect(0) = %+v, want %+v", got, want)
	}
	// Item 2 visual {80,0,40,56} => hit {78,0,44,56}.
	if got, want := b.ItemHitRect(2), (Rect{X: 78, Y: 0, W: 44, H: 56}); got != want {
		t.Fatalf("touch ItemHitRect(2) = %+v, want %+v", got, want)
	}
	// Every hit rect is at least the 44px finger floor on both axes.
	for i := range b.Items {
		hr := b.ItemHitRect(i)
		if hr.W < 44 || hr.H < 44 {
			t.Fatalf("ItemHitRect(%d)=%+v below 44px floor", i, hr)
		}
	}
}

// --- Accessibility --------------------------------------------------------

func TestTabBarA11yTablist(t *testing.T) {
	b := NewTabBar(sampleItems(), 2)
	if got := b.A11y(); got.Role != RoleTablist || got.Name != "Profile" {
		t.Fatalf("A11y = %+v, want tablist named Profile", got)
	}
	// Icon fallback when the selected item has no label.
	b2 := NewTabBar([]TabItem{{Icon: "X"}}, 0)
	if got := b2.A11y(); got.Role != RoleTablist || got.Name != "X" {
		t.Fatalf("A11y icon fallback = %+v, want tablist named X", got)
	}
	// Empty bar: no name.
	if got := NewTabBar(nil, 0).A11y(); got.Role != RoleTablist || got.Name != "" {
		t.Fatalf("empty A11y = %+v, want tablist with empty name", got)
	}
}

func TestTabBarWalkA11yTabsWithSelectedState(t *testing.T) {
	b := NewTabBar(sampleItems(), 1)
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 56})
	nodes := WalkA11y(b)
	// tablist itself + one tab per item.
	if len(nodes) != 1+len(b.Items) {
		t.Fatalf("WalkA11y node count = %d, want %d", len(nodes), 1+len(b.Items))
	}
	if nodes[0].Role != RoleTablist {
		t.Fatalf("first node role = %q, want tablist", nodes[0].Role)
	}
	for i := range b.Items {
		node := nodes[1+i]
		if node.Role != RoleTab {
			t.Fatalf("tab %d role = %q, want tab", i, node.Role)
		}
		if node.Name != b.Items[i].Label {
			t.Fatalf("tab %d name = %q, want %q", i, node.Name, b.Items[i].Label)
		}
		wantVal := ""
		if i == 1 {
			wantVal = "selected"
		}
		if node.Value != wantVal {
			t.Fatalf("tab %d value = %q, want %q", i, node.Value, wantVal)
		}
		if node.Rect != b.ItemRect(i) {
			t.Fatalf("tab %d rect = %+v, want %+v", i, node.Rect, b.ItemRect(i))
		}
	}
}

func TestTabItemNodeIconFallback(t *testing.T) {
	// A tab node with no label reports its icon as the name.
	n := &tabItemNode{item: TabItem{Icon: "Q"}, selected: true}
	if got := n.A11y(); got.Role != RoleTab || got.Name != "Q" || got.Value != "selected" {
		t.Fatalf("tabItemNode A11y = %+v, want tab/Q/selected", got)
	}
	un := &tabItemNode{item: TabItem{Icon: "Q", Label: "Quiet"}, selected: false}
	if got := un.A11y(); got.Name != "Quiet" || got.Value != "" {
		t.Fatalf("tabItemNode A11y = %+v, want name Quiet, empty value", got)
	}
}

func TestTabBarChildrenEmpty(t *testing.T) {
	if got := NewTabBar(nil, 0).Children(); len(got) != 0 {
		t.Fatalf("empty Children len = %d, want 0", len(got))
	}
}
