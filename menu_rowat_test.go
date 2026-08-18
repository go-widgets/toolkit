// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// rowAtItems is the fixed row layout every RowAt test drives: one enabled action
// row, a separator, a disabled (no Action, no Submenu) row, another action row, a
// submenu-parent row, and a checkable action row. It exercises each -1 case
// (separator, disabled) alongside each row RowAt must resolve.
//
// buildRowAtMenu returns a fresh menu plus a recorder: every enabled non-submenu
// row's Action writes its index into *fired, so a control-run can observe exactly
// which row a click acted on and cross-check it against RowAt.
func buildRowAtMenu() (m *Menu, fired *int) {
	f := -1
	fired = &f
	set := func(i int) func() { return func() { *fired = i } }
	m = NewMenu([]MenuItem{
		{Label: "New", Action: set(0)},                   // 0 enabled
		{Separator: true},                                // 1 separator -> -1
		{Label: "Disabled"},                              // 2 disabled  -> -1
		{Label: "Open", Action: set(3)},                  // 3 enabled
		{Label: "Recent", Submenu: NewMenu(nil)},         // 4 submenu-parent (enabled)
		{Label: "Wrap", Action: set(5), Checkable: true}, // 5 enabled checkable
	})
	return m, fired
}

// itemSeps mirrors buildRowAtMenu's separator flags (index 1 is the separator).
var itemSeps = []bool{false, true, false, false, false, false}

// isEnabledRow mirrors buildRowAtMenu's enabled rows: 0,3,4,5 act; the separator
// (1) and the disabled row (2) do not.
func isEnabledRow(i int) bool { return i == 0 || i == 3 || i == 4 || i == 5 }

// actedIndex drives a single click at (x, y) on a FRESH menu and reports the row
// index it acted on, or -1 when the click did nothing. It is the OnEvent side of
// the control-run: an enabled action/checkable row records its index via *fired;
// a submenu-parent row is detected through openSub; a separator, disabled, or
// out-of-body click leaves both untouched and yields -1.
func actedIndex(x, y int, scale float64, dens DensityLevel, bounds Rect) int {
	SetDensity(dens)
	m, fired := buildRowAtMenu()
	m.Scale = scale
	m.SetBounds(bounds)
	m.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if *fired >= 0 {
		return *fired
	}
	if m.openSub >= 0 {
		return m.openSub
	}
	return -1
}

// TestMenuRowAtDensityAndScale is the core table: at three touch densities and at
// an extra HiDPI scale, it pins the exact row metrics (inset/rowH/sepH), then
// walks every row asserting RowAt returns the precise index at the row's top,
// centre, and last pixel — and -1 over the top pad, separators, disabled rows,
// and just past the last row. Each probe is CROSS-CHECKED against what an OnEvent
// click actually does at the same point (actedIndex), so RowAt can never claim a
// row a click would not act on, nor miss one it would.
func TestMenuRowAtDensityAndScale(t *testing.T) {
	defer SetDensity(DensityCompact)
	cases := []struct {
		name              string
		density           DensityLevel
		scale             float64
		inset, rowH, sepH int
	}{
		{"compact", DensityCompact, 0, 2, 22, 6},
		{"comfortable", DensityComfortable, 0, 3, 36, 8},
		{"touch", DensityTouch, 0, 3, 44, 9},
		{"scale2-compact", DensityCompact, 2, 4, 44, 12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SetDensity(c.density)
			defer SetDensity(DensityCompact)
			m, _ := buildRowAtMenu()
			m.Scale = c.scale

			// Pin the metrics so the pixel coordinates below are exact and
			// independent of the code under test.
			if got := m.sc(2); got != c.inset {
				t.Fatalf("inset sc(2) = %d, want %d", got, c.inset)
			}
			if got := m.rowH(); got != c.rowH {
				t.Fatalf("rowH = %d, want %d", got, c.rowH)
			}
			if got := m.sc(MenuSeparatorH); got != c.sepH {
				t.Fatalf("sepH = %d, want %d", got, c.sepH)
			}

			// Compute each row's top from the pinned metrics.
			tops := make([]int, len(m.Items))
			cy := c.inset
			for i := range m.Items {
				tops[i] = cy
				if itemSeps[i] {
					cy += c.sepH
				} else {
					cy += c.rowH
				}
			}
			total := cy
			W, H := 200, total+20
			bounds := Rect{X: 0, Y: 0, W: W, H: H}
			m.SetBounds(bounds)
			cx := W / 2

			height := func(i int) int {
				if itemSeps[i] {
					return c.sepH
				}
				return c.rowH
			}
			check := func(y, want int) {
				t.Helper()
				if got := m.RowAt(cx, y); got != want {
					t.Errorf("RowAt(%d,%d) = %d, want %d", cx, y, got, want)
				}
				if got := actedIndex(cx, y, c.scale, c.density, bounds); got != want {
					t.Errorf("control-run: click(%d,%d) acted on %d, RowAt wants %d", cx, y, got, want)
				}
			}

			// Top pad above the first row is dead space.
			check(c.inset-1, -1)
			// Every row: top, centre, last pixel resolve to its index when
			// enabled, else -1.
			for i := range m.Items {
				want := -1
				if isEnabledRow(i) {
					want = i
				}
				top := tops[i]
				h := height(i)
				check(top, want)
				check(top+h/2, want)
				check(top+h-1, want)
			}
			// Just past the last row, still inside the bounds: no row.
			check(total, -1)
			check(total+5, -1)

			// RowTop / RowHeight agree with the pinned layout (unscrolled).
			for i := range m.Items {
				if got := m.RowTop(i); got != tops[i] {
					t.Errorf("RowTop(%d) = %d, want %d", i, got, tops[i])
				}
				if got := m.RowHeight(i); got != height(i) {
					t.Errorf("RowHeight(%d) = %d, want %d", i, got, height(i))
				}
			}
		})
	}
}

// TestMenuRowAtBoundsGuard covers the x/y out-of-bounds branch: a point outside
// [0,W)×[0,H) is -1 even when its Y would otherwise land on an enabled row. This
// is the guard specific to RowAt (a host performs the same bounds check before
// forwarding a click), so it is asserted directly rather than through OnEvent.
func TestMenuRowAtBoundsGuard(t *testing.T) {
	m, _ := buildRowAtMenu()
	W, H := 160, 200
	m.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	// Row 0 spans [2,24); its centre y=13 resolves to 0 for an in-bounds x.
	if got := m.RowAt(W/2, 13); got != 0 {
		t.Fatalf("RowAt(in-bounds, row0) = %d, want 0", got)
	}
	for _, p := range []struct {
		name string
		x, y int
	}{
		{"x<0", -1, 13},
		{"x>=W", W, 13},
		{"y<0", W / 2, -1},
		{"y>=H", W / 2, H},
	} {
		if got := m.RowAt(p.x, p.y); got != -1 {
			t.Errorf("RowAt(%s) = %d, want -1 (outside bounds)", p.name, got)
		}
	}
}

// TestMenuRowAtScrolled proves RowAt tracks the scroll offset: with the body
// clamped shorter than its rows, a scrolled menu resolves widget-local points to
// the row actually painted there, RowTop shifts up (negative for a row above the
// fold), and each probe still agrees with an OnEvent click at the same point.
func TestMenuRowAtScrolled(t *testing.T) {
	defer SetDensity(DensityCompact)
	SetDensity(DensityCompact)
	m, _ := buildRowAtMenu()
	// Compact, scale 1: inset 2, rowH 22, sepH 6. Content tops:
	// r0=2 r1(sep)=24 r2=30 r3=52 r4=74 r5=96, rowsHeight=116.
	const inset, rowH = 2, 22
	W, H := 200, 60 // shorter than the 116px of rows -> scrolls
	bounds := Rect{X: 0, Y: 0, W: W, H: H}
	m.SetBounds(bounds)
	if got := m.maxScroll(); got != m.rowsHeight()+m.sc(4)-H {
		t.Fatalf("maxScroll = %d, want %d", got, m.rowsHeight()+m.sc(4)-H)
	}
	m.scroll = 30
	if got := m.clampedScroll(); got != 30 {
		t.Fatalf("clampedScroll = %d, want 30", got)
	}
	cx := W / 2

	// Row 3's content top is 52; scrolled up by 30 it paints at widget-local 22.
	if got := m.RowTop(3); got != 22 {
		t.Errorf("RowTop(3) scrolled = %d, want 22", got)
	}
	if got := m.RowAt(cx, 22); got != 3 {
		t.Errorf("RowAt(scrolled row3 top) = %d, want 3", got)
	}
	if got := m.RowAt(cx, 22+rowH/2); got != 3 {
		t.Errorf("RowAt(scrolled row3 centre) = %d, want 3", got)
	}
	// Row 0 (content top 2) is scrolled above the fold: RowTop negative, and a
	// widget-local point there is out of bounds -> -1.
	if got := m.RowTop(0); got != inset-30 {
		t.Errorf("RowTop(0) scrolled = %d, want %d", got, inset-30)
	}
	// Control-run at the visible row: an OnEvent click at the same point acts on 3.
	if got := actedIndexScrolled(cx, 22+rowH/2, bounds, 30); got != 3 {
		t.Errorf("control-run scrolled: click acted on %d, want 3", got)
	}
	// A disabled row scrolled into view still yields -1. Row 2 content top 30 ->
	// widget-local 0 at scroll 30; its centre (widget-local 11) is disabled.
	if got := m.RowAt(cx, 11); got != -1 {
		t.Errorf("RowAt(scrolled disabled row2) = %d, want -1", got)
	}
	if got := actedIndexScrolled(cx, 11, bounds, 30); got != -1 {
		t.Errorf("control-run scrolled: disabled click acted on %d, want -1", got)
	}
}

// actedIndexScrolled is actedIndex for a menu pre-scrolled to a fixed offset.
func actedIndexScrolled(x, y int, bounds Rect, scroll int) int {
	m, fired := buildRowAtMenu()
	m.SetBounds(bounds)
	m.scroll = scroll
	m.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if *fired >= 0 {
		return *fired
	}
	if m.openSub >= 0 {
		return m.openSub
	}
	return -1
}

// TestMenuRowHeightOutOfRange covers RowHeight's guard: a negative or past-the-end
// index has no row and returns -1.
func TestMenuRowHeightOutOfRange(t *testing.T) {
	m, _ := buildRowAtMenu()
	if got := m.RowHeight(-1); got != -1 {
		t.Errorf("RowHeight(-1) = %d, want -1", got)
	}
	if got := m.RowHeight(len(m.Items)); got != -1 {
		t.Errorf("RowHeight(len) = %d, want -1", got)
	}
}

// TestMenuHitRowMatchesActivate is the direct control-run on the shared helper:
// for every index, hitRow's answer is exactly the set of rows activate acts on.
// Since OnEvent's EventClick is activate(hitRow(y)), this proves the extracted
// helper can't diverge from the click path.
func TestMenuHitRowMatchesActivate(t *testing.T) {
	m, _ := buildRowAtMenu()
	m.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	for i := range m.Items {
		// Query at the row's centre in widget-local coords.
		y := m.RowTop(i) + m.RowHeight(i)/2
		got := m.hitRow(y)
		want := -1
		if isEnabledRow(i) {
			want = i
		}
		if got != want {
			t.Errorf("hitRow(row %d centre) = %d, want %d", i, got, want)
		}
		// hitRow's verdict must match enabledItem, the gate activate uses.
		if (got >= 0) != m.enabledItem(i) {
			t.Errorf("hitRow(row %d) enabled=%v disagrees with enabledItem=%v", i, got >= 0, m.enabledItem(i))
		}
	}
}
