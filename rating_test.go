// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestRatingNewDefaultMax covers the max <= 0 -> 5 branch of NewRating.
func TestRatingNewDefaultMax(t *testing.T) {
	r := NewRating(2, 0)
	if r.Max != 5 {
		t.Fatalf("default Max = %d, want 5", r.Max)
	}
	r = NewRating(2, -3)
	if r.Max != 5 {
		t.Fatalf("negative Max = %d, want 5", r.Max)
	}
}

// TestRatingNewKeepsPositiveMax covers the positive-max half of the
// branch.
func TestRatingNewKeepsPositiveMax(t *testing.T) {
	r := NewRating(2, 7)
	if r.Max != 7 {
		t.Fatalf("Max = %d, want 7", r.Max)
	}
}

// TestRatingNewClampsValueNegative covers value < 0 -> 0.
func TestRatingNewClampsValueNegative(t *testing.T) {
	r := NewRating(-4, 5)
	if r.Value().Get() != 0 {
		t.Fatalf("negative Value clamped to %d, want 0", r.Value().Get())
	}
}

// TestRatingNewClampsValueAboveMax covers value > max -> max.
func TestRatingNewClampsValueAboveMax(t *testing.T) {
	r := NewRating(99, 5)
	if r.Value().Get() != 5 {
		t.Fatalf("Value clamped to %d, want 5", r.Value().Get())
	}
}

// TestRatingNewKeepsInRangeValue covers the in-range case (both
// clamp branches skipped).
func TestRatingNewKeepsInRangeValue(t *testing.T) {
	r := NewRating(3, 5)
	if r.Value().Get() != 3 {
		t.Fatalf("Value = %d, want 3 (unchanged)", r.Value().Get())
	}
}

// TestRatingDrawFilledAndEmpty verifies filled cells land in Accent
// and empty cells land in SurfaceAlt.
func TestRatingDrawFilledAndEmpty(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(2, 5)
	r.SetBounds(Rect{X: 0, Y: 0, W: 5 * (RatingStarW + RatingStarGap), H: RatingStarW})
	surfW := 5*(RatingStarW+RatingStarGap) + 4
	surf := makeSurface(surfW, RatingStarW+4)
	r.Draw(newP(surf, surfW), theme)

	// Cell 0 (index < Value) filled in Accent. Sample near cell centre,
	// away from the "*" glyph which occupies the middle rows.
	if got := pixelAt(surf, surfW, 1, 1); got != theme.Accent {
		t.Fatalf("cell 0 fill = %+v, want Accent", got)
	}
	// Cell 3 (index >= Value) filled in SurfaceAlt.
	x3 := 3*(RatingStarW+RatingStarGap) + 1
	if got := pixelAt(surf, surfW, x3, 1); got != theme.SurfaceAlt {
		t.Fatalf("cell 3 fill = %+v, want SurfaceAlt", got)
	}
}

// TestRatingDrawGlyphsFilledUsesAccentInk verifies that a filled cell's
// glyph ink honours accentInk (OnAccent override present in Extra).
func TestRatingDrawGlyphsFilledUsesAccentInk(t *testing.T) {
	theme := DefaultLight()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	r := NewRating(1, 3)
	r.SetBounds(Rect{X: 0, Y: 0, W: 3 * (RatingStarW + RatingStarGap), H: RatingStarW})
	surfW := 3 * (RatingStarW + RatingStarGap)
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	found := false
	for y := 0; y < RatingStarW && !found; y++ {
		for x := 0; x < RatingStarW; x++ {
			if pixelAt(surf, surfW, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in filled cell 0")
	}
}

// TestRatingDrawGlyphsEmptyUsesOnSurface verifies that an empty cell's
// glyph ink is Theme.OnSurface (independent of any Extra override).
func TestRatingDrawGlyphsEmptyUsesOnSurface(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(0, 2)
	r.SetBounds(Rect{X: 0, Y: 0, W: 2 * (RatingStarW + RatingStarGap), H: RatingStarW})
	surfW := 2 * (RatingStarW + RatingStarGap)
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	found := false
	for y := 0; y < RatingStarW && !found; y++ {
		for x := 0; x < surfW; x++ {
			if pixelAt(surf, surfW, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnSurface-coloured glyph pixel found in empty cell")
	}
}

// TestRatingDrawValueEqualsMax covers the "all filled" edge — every
// cell is a filled cell.
func TestRatingDrawValueEqualsMax(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(5, 5)
	r.SetBounds(Rect{X: 0, Y: 0, W: 5 * (RatingStarW + RatingStarGap), H: RatingStarW})
	surfW := 5 * (RatingStarW + RatingStarGap)
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	// Last cell centre pixel outside the glyph must be Accent.
	xLast := 4*(RatingStarW+RatingStarGap) + 1
	if got := pixelAt(surf, surfW, xLast, 1); got != theme.Accent {
		t.Fatalf("last cell fill = %+v, want Accent", got)
	}
}

// TestRatingDrawValueZero covers the "all empty" edge.
func TestRatingDrawValueZero(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(0, 5)
	r.SetBounds(Rect{X: 0, Y: 0, W: 5 * (RatingStarW + RatingStarGap), H: RatingStarW})
	surfW := 5 * (RatingStarW + RatingStarGap)
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	// First cell fill = SurfaceAlt (no filled cells).
	if got := pixelAt(surf, surfW, 1, 1); got != theme.SurfaceAlt {
		t.Fatalf("cell 0 fill = %+v, want SurfaceAlt", got)
	}
}

// TestRatingValueObservable covers the zero-value lazy-init of the Value
// accessor and the host binding path: a Rating built as a bare struct (no
// NewRating) still yields a usable Observable, and Setting it from outside is
// reflected by the widget (there is no imperative Value field).
func TestRatingValueObservable(t *testing.T) {
	r := &Rating{Max: 5} // no NewRating → value Observable is nil until accessed
	if r.Value().Get() != 0 {
		t.Fatalf("zero-value Rating Value = %d, want 0", r.Value().Get())
	}
	seen := -1
	r.Value().Subscribe(func(v int) { seen = v })
	r.Value().Set(4) // a host drives the rating through the Observable
	if r.Value().Get() != 4 || seen != 4 {
		t.Fatalf("host Set: value=%d subscriber=%d, want 4/4", r.Value().Get(), seen)
	}
}

// TestRatingClickFillsToIndex verifies OnEvent turns a click at cell k
// into Value = k+1 and notifies the Value Observable.
func TestRatingClickFillsToIndex(t *testing.T) {
	got := -1
	r := NewRating(0, 5)
	r.Value().Subscribe(func(v int) { got = v })
	r.SetBounds(Rect{X: 0, Y: 0, W: 5 * (RatingStarW + RatingStarGap), H: RatingStarW})
	// Click at x = 2*(RatingStarW+RatingStarGap) + 3 -> cell index 2.
	r.OnEvent(Event{Kind: EventClick, X: 2*(RatingStarW+RatingStarGap) + 3, Y: RatingStarW / 2})
	if r.Value().Get() != 3 {
		t.Fatalf("after click on cell 2, Value = %d, want 3", r.Value().Get())
	}
	if got != 3 {
		t.Fatalf("Value subscriber got %d, want 3", got)
	}
}

// TestRatingClickFirstCell exercises the leftmost cell (index 0).
func TestRatingClickFirstCell(t *testing.T) {
	r := NewRating(4, 5)
	r.OnEvent(Event{Kind: EventClick, X: 3, Y: RatingStarW / 2})
	if r.Value().Get() != 1 {
		t.Fatalf("after click on cell 0, Value = %d, want 1", r.Value().Get())
	}
}

// TestRatingClickLastCell exercises the rightmost cell (index Max-1).
func TestRatingClickLastCell(t *testing.T) {
	r := NewRating(0, 5)
	r.OnEvent(Event{Kind: EventClick, X: 4*(RatingStarW+RatingStarGap) + 3, Y: RatingStarW / 2})
	if r.Value().Get() != 5 {
		t.Fatalf("after click on cell 4, Value = %d, want 5", r.Value().Get())
	}
}

// TestRatingClickOutsideStripIgnored covers the idx >= Max branch: a
// click to the right of the last cell must not change Value.
func TestRatingClickOutsideStripIgnored(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventClick, X: 6 * (RatingStarW + RatingStarGap), Y: 0})
	if r.Value().Get() != 2 {
		t.Fatalf("click past strip should be ignored: Value = %d, want 2", r.Value().Get())
	}
}

// TestRatingClickNegativeXIgnored covers the idx < 0 guard. Craft an X
// small enough that integer division wraps into a negative -- Go
// truncates toward zero so a negative X yields idx <= 0; but idx == 0
// is still a valid cell, so the branch is guarded specifically for
// negative X values that produce a negative index. A single-pixel
// negative X yields idx = 0 (0/(RatingStarW+RatingStarGap) == 0) —
// so we need X <= -(RatingStarW+RatingStarGap) to reach idx == -1.
func TestRatingClickNegativeXIgnored(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventClick, X: -(RatingStarW + RatingStarGap + 1), Y: 0})
	if r.Value().Get() != 2 {
		t.Fatalf("negative-X click should be ignored: Value = %d, want 2", r.Value().Get())
	}
}

// TestRatingIgnoresNonClick guards the early-return in OnEvent: any
// non-click event must leave Value unchanged.
func TestRatingIgnoresNonClick(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if r.Value().Get() != 2 {
		t.Fatalf("KeyDown should not change Value: got %d, want 2", r.Value().Get())
	}
}

// TestRatingClickNoSubscriber checks a click still updates Value with no
// subscriber attached (no panic).
func TestRatingClickNoSubscriber(t *testing.T) {
	r := NewRating(0, 5)
	r.OnEvent(Event{Kind: EventClick, X: 3, Y: 0})
	if r.Value().Get() != 1 {
		t.Fatal("click must update Value even without OnChange callback")
	}
}
