// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// paginationLayoutW returns the total pixel width the Pagination
// widget occupies when its slots() emits n numeric/ellipsis entries.
// The layout is: prev + n slots + next, each PaginationBtnW wide,
// separated by PaginationGap.
func paginationLayoutW(n int) int {
	return (n+2)*PaginationBtnW + (n+1)*PaginationGap
}

// --- Constructor ---------------------------------------------------------

func TestNewPaginationClampsCurrentLow(t *testing.T) {
	p := NewPagination(-3, 10)
	if p.Current != 1 || p.Total != 10 {
		t.Fatalf("clamp low: Current=%d Total=%d", p.Current, p.Total)
	}
}

func TestNewPaginationClampsCurrentHigh(t *testing.T) {
	p := NewPagination(99, 10)
	if p.Current != 10 || p.Total != 10 {
		t.Fatalf("clamp high: Current=%d Total=%d", p.Current, p.Total)
	}
}

func TestNewPaginationTotalZeroParksCurrentAtOne(t *testing.T) {
	p := NewPagination(5, 0)
	if p.Current != 1 || p.Total != 0 {
		t.Fatalf("total 0: Current=%d Total=%d", p.Current, p.Total)
	}
}

func TestNewPaginationTotalNegativeParksCurrentAtOne(t *testing.T) {
	p := NewPagination(5, -3)
	if p.Current != 1 || p.Total != -3 {
		t.Fatalf("total -3: Current=%d Total=%d", p.Current, p.Total)
	}
}

func TestNewPaginationRoundTripsValidInputs(t *testing.T) {
	p := NewPagination(3, 10)
	if p.Current != 3 || p.Total != 10 {
		t.Fatalf("valid inputs mangled: Current=%d Total=%d", p.Current, p.Total)
	}
}

// --- Draw branches -------------------------------------------------------

// Total <= 0 paints only the outer body — no buttons. Every non-body
// tint must be absent.
func TestPaginationDrawTotalZeroNoButtons(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(0)
	theme := DefaultLight()
	p := NewPagination(1, 0)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.SurfaceAlt {
				t.Fatalf("Total=0 painted a step button at (%d,%d)", x, y)
			}
			if pixelAt(buf, w, x, y) == theme.Accent {
				t.Fatalf("Total=0 painted an Accent tint at (%d,%d)", x, y)
			}
		}
	}
}

// Total == 1: prev + one page + next. Prev+next are disabled tone;
// the single page is on Accent.
func TestPaginationDrawSinglePage(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(1)
	theme := DefaultLight()
	p := NewPagination(1, 1)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Middle numeric slot must have an Accent fill.
	numX := PaginationBtnW + PaginationGap
	if pixelAt(buf, w, numX+2, 2) != theme.Accent {
		t.Fatalf("single-page fill = %+v, want Accent",
			pixelAt(buf, w, numX+2, 2))
	}
}

// Total == 3, Current == 2: prev enabled, next enabled, three page
// buttons, middle one on Accent.
func TestPaginationDrawSmallStrip(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultLight()
	p := NewPagination(2, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Middle numeric slot: prev(0) + slot0(1) + slot1(2 <- current).
	numX := (PaginationBtnW + PaginationGap) * 2
	if pixelAt(buf, w, numX+2, 2) != theme.Accent {
		t.Fatalf("current-page fill = %+v, want Accent",
			pixelAt(buf, w, numX+2, 2))
	}
}

// Total large + Current in middle: exercises the "middle" window
// branch — 1, ..., k-1, k, k+1, ..., Total.
func TestPaginationDrawLargeMiddleWindow(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(7)
	theme := DefaultLight()
	p := NewPagination(50, 100)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Current (page 50) lives at slot index 3 (0-based) of the 7-wide
	// numeric strip; overall button index 4 (prev + 4).
	currentX := (PaginationBtnW + PaginationGap) * 4
	if pixelAt(buf, w, currentX+2, 2) != theme.Accent {
		t.Fatalf("middle-window current fill = %+v, want Accent",
			pixelAt(buf, w, currentX+2, 2))
	}
}

// Total large + Current near start: exercises the "near-start" window
// branch — 1..5, ..., Total.
func TestPaginationDrawLargeNearStart(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(7)
	theme := DefaultLight()
	p := NewPagination(1, 100)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Slot 0 of numeric strip = page 1 (current). Overall button
	// index 1 (prev + 1 numeric slot preceded by nothing).
	currentX := (PaginationBtnW + PaginationGap) * 1
	if pixelAt(buf, w, currentX+2, 2) != theme.Accent {
		t.Fatalf("near-start current fill = %+v, want Accent",
			pixelAt(buf, w, currentX+2, 2))
	}
}

// Total large + Current near end: exercises the "near-end" window
// branch — 1, ..., Total-4..Total.
func TestPaginationDrawLargeNearEnd(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(7)
	theme := DefaultLight()
	p := NewPagination(100, 100)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Current = 100 = last numeric slot (index 6) + prev = overall 7.
	currentX := (PaginationBtnW + PaginationGap) * 7
	if pixelAt(buf, w, currentX+2, 2) != theme.Accent {
		t.Fatalf("near-end current fill = %+v, want Accent",
			pixelAt(buf, w, currentX+2, 2))
	}
}

// Prev disabled: label ink lands in Border (not OnSurface).
func TestPaginationDrawPrevDisabledInkIsBorder(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultLight()
	p := NewPagination(1, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// The "<" glyph centers around ~x = (PaginationBtnW - GlyphAdvance())/2.
	// Look for any Border-coloured ink in the prev-button interior
	// rows (excluding the outer stroke) to confirm disabled tone.
	found := false
	ty := (PaginationBtnH - GlyphHeight()) / 2
	for y := ty; y < ty+GlyphHeight() && !found; y++ {
		for x := 1; x < PaginationBtnW-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Border && y > 0 && y < h-1 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("prev-disabled: no Border-coloured glyph ink found")
	}
}

// Next disabled: label ink lands in Border on the far right button.
func TestPaginationDrawNextDisabledInkIsBorder(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultLight()
	p := NewPagination(3, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	nextX := (PaginationBtnW + PaginationGap) * 4 // prev + 3 nums = 4
	ty := (PaginationBtnH - GlyphHeight()) / 2
	found := false
	for y := ty; y < ty+GlyphHeight() && !found; y++ {
		for x := nextX + 1; x < nextX+PaginationBtnW-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Border && y > 0 && y < h-1 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("next-disabled: no Border-coloured glyph ink found")
	}
}

// Windowed strip: ellipsis slot draws in Border ink (its 'page == 0'
// tint). Exercises the ellipsis-specific draw branch.
func TestPaginationDrawEllipsisSlot(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(7)
	theme := DefaultLight()
	p := NewPagination(50, 100)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Left ellipsis is numeric slot 1 (0-based) → overall button
	// index 2 (prev + 1 numeric + this).
	ellX := (PaginationBtnW + PaginationGap) * 2
	ty := (PaginationBtnH - GlyphHeight()) / 2
	found := false
	for y := ty; y < ty+GlyphHeight() && !found; y++ {
		for x := ellX + 1; x < ellX+PaginationBtnW-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Border {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("ellipsis: no Border-coloured glyph ink found")
	}
}

// Dark theme + Extra["OnAccent"] override: the current-page label
// lands in the custom colour rather than the fallback Background.
func TestPaginationDrawWithOnAccentExtra(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultLight()
	custom := RGB(0x11, 0x22, 0x33)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	p := NewPagination(2, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Current = page 2 = numeric slot 1 → overall button index 2.
	numX := (PaginationBtnW + PaginationGap) * 2
	ty := (PaginationBtnH - GlyphHeight()) / 2
	found := false
	for y := ty; y < ty+GlyphHeight() && !found; y++ {
		for x := numX + 1; x < numX+PaginationBtnW-1; x++ {
			if pixelAt(buf, w, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("OnAccent Extra override: no custom-ink glyph pixel")
	}
}

// Extra map with no OnAccent key: accentInk falls through to
// theme.Background. Exercises the ok==false branch.
func TestPaginationDrawWithoutOnAccentExtraFallsBack(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{"other": RGB(1, 2, 3)}
	p := NewPagination(2, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	numX := (PaginationBtnW + PaginationGap) * 2
	ty := (PaginationBtnH - GlyphHeight()) / 2
	found := false
	for y := ty; y < ty+GlyphHeight() && !found; y++ {
		for x := numX + 1; x < numX+PaginationBtnW-1; x++ {
			if pixelAt(buf, w, x, y) == theme.Background {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent + no Background-ink glyph pixel found")
	}
}

// Dark theme sanity check.
func TestPaginationDrawDarkTheme(t *testing.T) {
	const h = PaginationBtnH
	w := paginationLayoutW(3)
	theme := DefaultDark()
	p := NewPagination(2, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("dark prev border top-left = %+v, want Border",
			pixelAt(buf, w, 0, 0))
	}
}

// Zero-width bounds must not panic.
func TestPaginationDrawZeroBounds(t *testing.T) {
	const w, h = 8, 8
	theme := DefaultLight()
	p := NewPagination(1, 3)
	p.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0).R != 0xC8 {
		t.Fatal("zero-bounds Pagination painted pixels")
	}
}

// --- OnEvent branches ----------------------------------------------------

func TestPaginationClickPrevEnabled(t *testing.T) {
	got := -1
	p := NewPagination(3, 10)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(7), H: PaginationBtnH})
	p.OnChange = func(page int) { got = page }
	// Prev button: idx 0, X in [0, PaginationBtnW).
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: 4})
	if p.Current != 2 || got != 2 {
		t.Fatalf("prev enabled: Current=%d got=%d", p.Current, got)
	}
}

func TestPaginationClickPrevDisabled(t *testing.T) {
	p := NewPagination(1, 10)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(7), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: 4})
	if p.Current != 1 || changes != 0 {
		t.Fatalf("prev disabled: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickNextEnabled(t *testing.T) {
	got := -1
	p := NewPagination(3, 10)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(7), H: PaginationBtnH})
	p.OnChange = func(page int) { got = page }
	// Next button lives at button index 8 (prev + 7 numeric slots).
	nextX := (PaginationBtnW+PaginationGap)*8 + 4
	p.OnEvent(Event{Kind: EventClick, X: nextX, Y: 4})
	if p.Current != 4 || got != 4 {
		t.Fatalf("next enabled: Current=%d got=%d", p.Current, got)
	}
}

func TestPaginationClickNextDisabled(t *testing.T) {
	p := NewPagination(10, 10)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(7), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	nextX := (PaginationBtnW+PaginationGap)*8 + 4
	p.OnEvent(Event{Kind: EventClick, X: nextX, Y: 4})
	if p.Current != 10 || changes != 0 {
		t.Fatalf("next disabled: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickPageNumber(t *testing.T) {
	got := -1
	p := NewPagination(2, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	p.OnChange = func(page int) { got = page }
	// Click page 4: numeric slot idx 3 → overall button idx 4.
	x := (PaginationBtnW+PaginationGap)*4 + 4
	p.OnEvent(Event{Kind: EventClick, X: x, Y: 4})
	if p.Current != 4 || got != 4 {
		t.Fatalf("click page 4: Current=%d got=%d", p.Current, got)
	}
}

func TestPaginationClickCurrentPageNoOp(t *testing.T) {
	changes := 0
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	p.OnChange = func(page int) { changes++ }
	// Numeric slot idx 2 = current page 3 → overall button idx 3.
	x := (PaginationBtnW+PaginationGap)*3 + 4
	p.OnEvent(Event{Kind: EventClick, X: x, Y: 4})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("click current: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickEllipsisNoOp(t *testing.T) {
	changes := 0
	p := NewPagination(50, 100)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(7), H: PaginationBtnH})
	p.OnChange = func(page int) { changes++ }
	// Left ellipsis lives at numeric slot idx 1 → overall button idx 2.
	x := (PaginationBtnW+PaginationGap)*2 + 4
	p.OnEvent(Event{Kind: EventClick, X: x, Y: 4})
	if p.Current != 50 || changes != 0 {
		t.Fatalf("click ellipsis: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickInGapNoOp(t *testing.T) {
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	// A click at x = PaginationBtnW lands in the gap between prev and
	// slot 0 (button width is PaginationBtnW, gap starts right after).
	p.OnEvent(Event{Kind: EventClick, X: PaginationBtnW, Y: 4})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("gap click: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickBelowRowNoOp(t *testing.T) {
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: PaginationBtnH + 5})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("below-row click: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickAboveRowNoOp(t *testing.T) {
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: -1})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("above-row click: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickShortBoundsNoOp(t *testing.T) {
	// Bounds H is smaller than PaginationBtnH — event Y is inside the
	// button height but past bounds; the guard rejects it.
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: 4})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: 8})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("short-bounds click: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationClickTotalZeroNoOp(t *testing.T) {
	p := NewPagination(1, 0)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(0), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: 4})
	if changes != 0 {
		t.Fatalf("total=0 click: changes=%d", changes)
	}
}

func TestPaginationIgnoresNonClick(t *testing.T) {
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	changes := 0
	p.OnChange = func(page int) { changes++ }
	p.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if p.Current != 3 || changes != 0 {
		t.Fatalf("non-click event: Current=%d changes=%d", p.Current, changes)
	}
}

func TestPaginationNilOnChangeNoPanic(t *testing.T) {
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	// Fire every mutation path; nil OnChange must be safe.
	p.OnEvent(Event{Kind: EventClick, X: 4, Y: 4})                                            // prev
	x := (PaginationBtnW+PaginationGap)*4 + 4
	p.OnEvent(Event{Kind: EventClick, X: x, Y: 4}) // page 4
	nextX := (PaginationBtnW+PaginationGap)*6 + 4
	p.OnEvent(Event{Kind: EventClick, X: nextX, Y: 4}) // next
}
