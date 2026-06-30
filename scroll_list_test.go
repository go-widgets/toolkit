// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- ScrollView ----------------------------------------------------------

func TestScrollViewClampNegativeOffsets(t *testing.T) {
	child := NewLabel("x")
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	sv.SetContentSize(200, 300)
	sv.Scroll(-50, -50) // both negative
	if sv.OffsetX != 0 || sv.OffsetY != 0 {
		t.Fatalf("negatives must clamp to 0; got OffsetX=%d OffsetY=%d", sv.OffsetX, sv.OffsetY)
	}
}

func TestScrollViewClampsToMax(t *testing.T) {
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	sv.SetContentSize(200, 300)
	sv.Scroll(10000, 10000)
	// maxY = 300 - 60 = 240. maxX = 200 - (100-8) = 108.
	if sv.OffsetY != 240 {
		t.Fatalf("OffsetY clamp: got %d, want 240", sv.OffsetY)
	}
	if sv.OffsetX != 108 {
		t.Fatalf("OffsetX clamp: got %d, want 108", sv.OffsetX)
	}
}

func TestScrollViewClampWhenContentSmallerThanViewport(t *testing.T) {
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	sv.SetContentSize(50, 30) // smaller than viewport
	sv.Scroll(100, 100)
	if sv.OffsetX != 0 || sv.OffsetY != 0 {
		t.Fatalf("offsets should stay 0 when content fits; got %d %d", sv.OffsetX, sv.OffsetY)
	}
}

func TestScrollViewDrawPaintsScrollbarTrack(t *testing.T) {
	const w, h = 64, 64
	theme := DefaultLight()
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	sv.SetContentSize(40, 200) // tall content -> scrollbar visible
	buf := makeSurface(w, h)
	sv.Draw(buf, w, theme)
	// Track pixel at right edge of bounds, below the thumb. With
	// contentH=200 + viewH=40 + OffsetY=0, thumbH=8 covering [0,8);
	// any row past 10 lands on bare track.
	if pixelAt(buf, w, 35, 20) != theme.SurfaceAlt {
		t.Fatalf("scrollbar track not painted; got %+v want SurfaceAlt", pixelAt(buf, w, 35, 20))
	}
}

func TestScrollViewDrawWithNilChildNoCrash(t *testing.T) {
	sv := NewScrollView(nil)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	sv.SetContentSize(20, 100)
	buf := makeSurface(32, 32)
	sv.Draw(buf, 32, DefaultLight())
}

func TestScrollViewDrawThumbPositionFollowsOffset(t *testing.T) {
	const w, h = 32, 64
	theme := DefaultLight()
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 40})
	sv.SetContentSize(24, 200)
	sv.Scroll(0, 100)
	buf := makeSurface(w, h)
	sv.Draw(buf, w, theme)
	// Somewhere in the middle of the track should be Accent (thumb).
	found := false
	for y := 0; y < 40; y++ {
		if pixelAt(buf, w, 20, y) == theme.Accent {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("thumb not painted on the track")
	}
}

func TestScrollViewDrawThumbMinSize(t *testing.T) {
	const w, h = 32, 32
	theme := DefaultLight()
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 16})
	sv.SetContentSize(24, 1_000_000) // huge -> thumb min 8 px
	buf := makeSurface(w, h)
	sv.Draw(buf, w, theme)
}

func TestScrollViewHitTest(t *testing.T) {
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 10, Y: 10, W: 20, H: 20})
	if !sv.HitTest(15, 15) {
		t.Fatal("inside bounds should hit")
	}
	if sv.HitTest(5, 5) {
		t.Fatal("outside bounds should miss")
	}
}

func TestScrollViewDrawZeroHeightThumbBranch(t *testing.T) {
	const w, h = 32, 32
	theme := DefaultLight()
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 0}) // r.H == 0
	sv.SetContentSize(24, 200)
	buf := makeSurface(w, h)
	sv.Draw(buf, w, theme)
	// Just must not panic; covers the r.H > 0 guard.
}

// --- ListBox -------------------------------------------------------------

func TestListBoxClickSelectsAndFires(t *testing.T) {
	got := -1
	l := NewListBox([]string{"a", "b", "c"})
	l.OnActivate = func(i int) { got = i }
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	// Row height 18 default; row 1 spans y in [18,36).
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 20})
	if l.Selected != 1 {
		t.Fatalf("Selected = %d, want 1", l.Selected)
	}
	if got != 1 {
		t.Fatalf("OnActivate fired with %d, want 1", got)
	}
}

func TestListBoxClickOutOfRangeIsNoOp(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 500})
	if l.Selected != -1 {
		t.Fatal("out-of-range click must not change Selected")
	}
}

func TestListBoxIgnoresNonClickEvents(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if l.Selected != -1 {
		t.Fatal("KeyDown must not select")
	}
}

func TestListBoxNilOnActivateNoPanic(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 30})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	// nil OnActivate must not panic.
}

func TestListBoxZeroRowHeightIsNoOp(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.RowHeight = 0
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if l.Selected != -1 {
		t.Fatal("zero RowHeight click must not select")
	}
}

func TestListBoxNegativeIndexNoSelect(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: -10})
	if l.Selected != -1 {
		t.Fatal("negative Y must not select")
	}
}

func TestListBoxDrawSelectedAndUnselected(t *testing.T) {
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.Selected = 1
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	buf := makeSurface(w, h)
	l.Draw(buf, w, theme)
	// Row 0 painted in Surface.
	if pixelAt(buf, w, 25, 5) != theme.Surface {
		t.Fatalf("row 0 bg = %+v, want Surface", pixelAt(buf, w, 25, 5))
	}
	// Row 1 (selected) painted in Accent.
	if pixelAt(buf, w, 25, 25) != theme.Accent {
		t.Fatalf("row 1 bg = %+v, want Accent", pixelAt(buf, w, 25, 25))
	}
}
