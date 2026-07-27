// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

var (
	segRed   = RGBA{R: 255, A: 255}
	segGreen = RGBA{G: 255, A: 255}
	segBlue  = RGBA{B: 255, A: 255}
)

func TestNewSegmentedBar(t *testing.T) {
	segs := []BarSegment{{Value: 1, Fill: segRed}, {Value: 2, Fill: segGreen}}
	s := NewSegmentedBar(segs)
	if len(s.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(s.Segments))
	}
	if s.Orientation != Horizontal {
		t.Fatalf("default Orientation = %v, want Horizontal", s.Orientation)
	}
}

func TestSegmentedBarTotal(t *testing.T) {
	s := NewSegmentedBar([]BarSegment{{Value: 1}, {Value: 2.5}, {Value: 0.5}})
	if got := s.Total(); got != 4 {
		t.Fatalf("Total() = %v, want 4", got)
	}
	if got := (&SegmentedBar{}).Total(); got != 0 {
		t.Fatalf("Total() of empty bar = %v, want 0", got)
	}
}

func TestSegmentedBarHorizontalProportional(t *testing.T) {
	// Three equal segments in a bar whose width divides evenly by 3
	// (90px -> 30px each), so the layout is exact with no rounding
	// leftover -- a clean check of each band's pixel extent.
	const w, h = 90, 20
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{
		{Value: 1, Fill: segRed},
		{Value: 1, Fill: segGreen},
		{Value: 1, Fill: segBlue},
	})
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)

	const y = h / 2
	// Segment 0 spans [0,30): check both ends of its extent.
	if got := pixelAt(buf, w, 1, y); got != segRed {
		t.Errorf("seg0 start (x=1) = %+v, want red", got)
	}
	if got := pixelAt(buf, w, 29, y); got != segRed {
		t.Errorf("seg0 end (x=29) = %+v, want red", got)
	}
	// Segment 1 spans [30,60).
	if got := pixelAt(buf, w, 31, y); got != segGreen {
		t.Errorf("seg1 start (x=31) = %+v, want green", got)
	}
	if got := pixelAt(buf, w, 59, y); got != segGreen {
		t.Errorf("seg1 end (x=59) = %+v, want green", got)
	}
	// Segment 2 spans [60,90). x=89 is the bar's own right edge (part of
	// the outer border stroke), so sample x=88 instead.
	if got := pixelAt(buf, w, 61, y); got != segBlue {
		t.Errorf("seg2 start (x=61) = %+v, want blue", got)
	}
	if got := pixelAt(buf, w, 88, y); got != segBlue {
		t.Errorf("seg2 end (x=88) = %+v, want blue", got)
	}
	// A separator line sits at the segment 0/1 boundary.
	if got := pixelAt(buf, w, 30, y); got != theme.Border {
		t.Errorf("boundary (x=30) = %+v, want Border", got)
	}
}

func TestSegmentedBarVerticalBottomUp(t *testing.T) {
	// Bottom-up: the first segment (Value=1) sits at the bottom, the
	// last (Value=1) at the top; equal shares in a 90px-tall bar give
	// exact 30px bands, same reasoning as the horizontal test.
	const w, h = 20, 90
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{
		{Value: 1, Fill: segRed},
		{Value: 1, Fill: segGreen},
		{Value: 1, Fill: segBlue},
	})
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)

	const x = w / 2
	// Bottom band (rows [60,89]) is the first segment: red. Row h-1=89
	// is the outer border stroke, so sample an interior row instead.
	if got := pixelAt(buf, w, x, 75); got != segRed {
		t.Errorf("bottom (y=75) = %+v, want red", got)
	}
	// Middle band (rows [30,59]) is the second segment: green. Row 59
	// is the seam against the red band below (drawn in Border), so
	// sample an interior row instead.
	if got := pixelAt(buf, w, x, 45); got != segGreen {
		t.Errorf("middle (y=45) = %+v, want green", got)
	}
	// Top band (rows [0,29]) is the third segment: blue. Row 0 is the
	// outer border stroke and row 29 is the seam against green, so
	// sample an interior row instead.
	if got := pixelAt(buf, w, x, 10); got != segBlue {
		t.Errorf("top (y=10) = %+v, want blue", got)
	}
}

func TestSegmentedBarZeroTotalDrawsEmptyTrack(t *testing.T) {
	const w, h = 40, 20
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{{Value: 0, Fill: segRed}, {Value: 0, Fill: segGreen}})
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, w/2, h/2); got != theme.SurfaceAlt {
		t.Errorf("zero-total interior = %+v, want SurfaceAlt", got)
	}
	if got := pixelAt(buf, w, 0, h/2); got != theme.Border {
		t.Errorf("zero-total left edge = %+v, want Border", got)
	}
}

func TestSegmentedBarEmptySegmentsNoOp(t *testing.T) {
	const w, h = 40, 20
	theme := DefaultLight()
	s := NewSegmentedBar(nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, w/2, h/2); got != theme.SurfaceAlt {
		t.Errorf("empty-segments interior = %+v, want SurfaceAlt", got)
	}
	if got := pixelAt(buf, w, 0, h/2); got != theme.Border {
		t.Errorf("empty-segments left edge = %+v, want Border", got)
	}
}

func TestSegmentedBarSingleSegmentFillsFully(t *testing.T) {
	const w, h = 50, 20
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{{Value: 7, Fill: segRed}})
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 1, h/2); got != segRed {
		t.Errorf("single-segment near-left = %+v, want red", got)
	}
	if got := pixelAt(buf, w, w-2, h/2); got != segRed {
		t.Errorf("single-segment near-right = %+v, want red", got)
	}
}

func TestSegmentedBarRoundingRemainderOnLastSegment(t *testing.T) {
	// Three equal-value segments in a 10px bar: 10/3 floors to 3px each
	// (sum 9), so the leftover 1px is pushed onto the last segment,
	// giving extents [3, 3, 4].
	const w, h = 10, 10
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{
		{Value: 1, Fill: segRed},
		{Value: 1, Fill: segGreen},
		{Value: 1, Fill: segBlue},
	})
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)

	const y = h / 2
	// Segment 0: x in [0,3) -> x=1,2 unambiguous (x=0 is the outer border).
	if got := pixelAt(buf, w, 2, y); got != segRed {
		t.Errorf("x=2 = %+v, want red", got)
	}
	// Segment 1: x in [3,6) -> x=4,5.
	if got := pixelAt(buf, w, 5, y); got != segGreen {
		t.Errorf("x=5 = %+v, want green", got)
	}
	// Segment 2 absorbed the rounding leftover: x in [6,10) -> x=7,8,9
	// all fall inside it (4px wide, vs. 3px for the other two). x=9 is
	// the bar's own right edge (outer border stroke), so sample x=8.
	if got := pixelAt(buf, w, 7, y); got != segBlue {
		t.Errorf("x=7 = %+v, want blue", got)
	}
	if got := pixelAt(buf, w, 8, y); got != segBlue {
		t.Errorf("x=8 (last interior pixel) = %+v, want blue", got)
	}
}

func TestSegmentedBarVerticalZeroValueSegmentSkipped(t *testing.T) {
	// A zero-Value segment sandwiched between two real ones rounds down
	// to a zero-height extent; Draw must skip painting/advancing for it
	// (covers the vertical loop's e<=0 continue branch) without
	// disturbing the segments on either side.
	const w, h = 20, 40
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{
		{Value: 1, Fill: segRed},
		{Value: 0, Fill: segGreen},
		{Value: 1, Fill: segBlue},
	})
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)

	const x = w / 2
	if got := pixelAt(buf, w, x, h-10); got != segRed {
		t.Errorf("bottom = %+v, want red", got)
	}
	if got := pixelAt(buf, w, x, 10); got != segBlue {
		t.Errorf("top = %+v, want blue", got)
	}
}

func TestSegmentedBarNegativeValueTreatedAsZero(t *testing.T) {
	// A defensively-negative Value contributes nothing to the layout
	// instead of corrupting it (covers the clamp inside Draw).
	const w, h = 30, 10
	theme := DefaultLight()
	s := NewSegmentedBar([]BarSegment{
		{Value: -5, Fill: segRed},
		{Value: 1, Fill: segGreen},
	})
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, w/2, h/2); got != segGreen {
		t.Errorf("negative-value segment should not paint = %+v, want green", got)
	}
}
