// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// fillWidget paints its whole bounds a solid colour — a probe for clip tests.
type fillWidget struct {
	Base
	color RGBA
}

func (f *fillWidget) Draw(p painter.Painter, _ *Theme) {
	r := f.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, f.color)
}

// noClipPainter wraps a Painter but hides any Clipper capability (embedding the
// Painter INTERFACE promotes only its methods), so ScrollView's clip type-
// assertion fails and the fallback path runs.
type noClipPainter struct{ painter.Painter }

// --- ScrollView ----------------------------------------------------------

func TestScrollViewClipsChildToViewport(t *testing.T) {
	// Regression: a child taller/wider than the viewport must be clipped to it,
	// not overdraw the surface below or the scrollbar column.
	const w, h = 64, 64
	red := RGBA{R: 255, A: 255}
	child := &fillWidget{color: red}
	child.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200}) // taller than the 40-px viewport
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	sv.SetContentSize(40, 200)
	buf := makeSurface(w, h)
	sv.Draw(newP(buf, w), DefaultLight())

	if pixelAt(buf, w, 10, 20) != red {
		t.Fatalf("inside viewport not painted: %+v", pixelAt(buf, w, 10, 20))
	}
	if pixelAt(buf, w, 10, 50) == red { // row 50 > viewport H 40
		t.Fatal("child painted below the viewport (not clipped)")
	}
	if pixelAt(buf, w, 35, 20) == red { // x=35 ≥ W−scrollbarWidth (28)
		t.Fatal("child painted into the scrollbar column (not clipped)")
	}
}

func TestScrollViewDrawWithoutClipperFallsBack(t *testing.T) {
	// A Painter that isn't a Clipper takes the fallback path (no clip) without
	// panicking — the child still draws (surface-edge clipped only).
	child := &fillWidget{color: RGBA{R: 1, A: 255}}
	child.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200})
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	sv.SetContentSize(40, 200)
	buf := makeSurface(64, 64)
	sv.Draw(noClipPainter{newP(buf, 64)}, DefaultLight())
}

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
	// Content is wider than the viewport (200 > 100-scrollGutter), so a
	// horizontal scrollbar reserves the bottom row and the viewport height shrinks
	// by the gutter. Express the clamps in terms of scrollGutter (track + the
	// normalized content gap) so they track the shared inset: viewport =
	// bounds - scrollGutter, clamp = content - viewport.
	viewW := 100 - scrollGutter()
	viewH := 60 - scrollGutter()
	wantY := 300 - viewH
	wantX := 200 - viewW
	if sv.OffsetY != wantY {
		t.Fatalf("OffsetY clamp: got %d, want %d", sv.OffsetY, wantY)
	}
	if sv.OffsetX != wantX {
		t.Fatalf("OffsetX clamp: got %d, want %d", sv.OffsetX, wantX)
	}
}

func TestScrollViewHorizontalScrollbar(t *testing.T) {
	const w, h = 100, 60
	acc := scrollbarThumbColor(DefaultLight())
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	sv.SetContentSize(200, 20) // wide (overflows) but short (no vertical overflow)
	buf := makeSurface(w, h)
	sv.Draw(newP(buf, w), DefaultLight())
	// A horizontal thumb (Accent) sits in the bottom scrollbar row.
	trackY := 40 - scrollbarWidth
	found := false
	for x := 0; x < 72 && !found; x++ {
		if pixelAt(buf, w, x, trackY+3) == acc {
			found = true
		}
	}
	if !found {
		t.Fatal("no horizontal scrollbar thumb drawn for wide content")
	}
	// Scrolling right moves the offset (clamped), i.e. horizontal scroll works.
	sv.Scroll(1000, 0)
	if sv.OffsetX == 0 {
		t.Fatal("horizontal scroll did not move OffsetX")
	}
}

func TestScrollViewTinyHorizontalThumbClamps(t *testing.T) {
	// Extremely wide content → the proportional horizontal thumb would be < 8px,
	// so it clamps to the 8px minimum (the thumbW<8 branch).
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	sv.SetContentSize(100000, 20)
	buf := makeSurface(64, 64)
	sv.Draw(newP(buf, 64), DefaultLight()) // must not panic; thumb floored to 8
	acc := scrollbarThumbColor(DefaultLight())
	trackY := 40 - scrollbarWidth
	found := false
	for x := 0; x < 32; x++ {
		if pixelAt(buf, 64, x, trackY+3) == acc {
			found = true
		}
	}
	if !found {
		t.Fatal("clamped horizontal thumb not drawn")
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
	sv.Draw(newP(buf, w), theme)
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
	sv.Draw(newP(buf, 32), DefaultLight())
}

func TestScrollViewDrawThumbPositionFollowsOffset(t *testing.T) {
	const w, h = 32, 64
	theme := DefaultLight()
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 40})
	sv.SetContentSize(24, 200)
	sv.Scroll(0, 100)
	buf := makeSurface(w, h)
	sv.Draw(newP(buf, w), theme)
	// Somewhere in the middle of the track should be the muted thumb.
	found := false
	for y := 0; y < 40; y++ {
		if pixelAt(buf, w, 20, y) == scrollbarThumbColor(theme) {
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
	sv.Draw(newP(buf, w), theme)
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
	sv.Draw(newP(buf, w), theme)
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
	if l.Selected().Get() != 1 {
		t.Fatalf("Selected = %d, want 1", l.Selected().Get())
	}
	if got != 1 {
		t.Fatalf("OnActivate fired with %d, want 1", got)
	}
}

func TestListBoxClickOutOfRangeIsNoOp(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 500})
	if l.Selected().Get() != -1 {
		t.Fatal("out-of-range click must not change Selected")
	}
}

func TestListBoxIgnoresNonClickEvents(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if l.Selected().Get() != -1 {
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
	if l.Selected().Get() != -1 {
		t.Fatal("zero RowHeight click must not select")
	}
}

func TestListBoxNegativeIndexNoSelect(t *testing.T) {
	l := NewListBox([]string{"a"})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: -10})
	if l.Selected().Get() != -1 {
		t.Fatal("negative Y must not select")
	}
}

func TestListBoxDrawSelectedAndUnselected(t *testing.T) {
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.Selected().Set(1)
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	// Row 0 painted in Surface.
	if pixelAt(buf, w, 25, 5) != theme.Surface {
		t.Fatalf("row 0 bg = %+v, want Surface", pixelAt(buf, w, 25, 5))
	}
	// Row 1 (selected) painted in Accent.
	if pixelAt(buf, w, 25, 25) != theme.Accent {
		t.Fatalf("row 1 bg = %+v, want Accent", pixelAt(buf, w, 25, 25))
	}
}

// --- ListBox multi-selection ---------------------------------------------

func TestListBoxMultiSelectDefaultOffPreservesSingleSelect(t *testing.T) {
	// Regression: with MultiSelect false (the zero value), Ctrl/Shift must
	// be completely ignored -- clicking behaves exactly like before this
	// feature existed, and the selection set never gets populated.
	got := -1
	l := NewListBox([]string{"a", "b", "c"})
	l.OnActivate = func(i int) { got = i }
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 20}) // row 1, no modifiers
	if l.Selected().Get() != 1 || got != 1 {
		t.Fatalf("Selected=%d got=%d, want 1,1", l.Selected().Get(), got)
	}

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 40, Ctrl: true}) // row 2, Ctrl
	if l.Selected().Get() != 2 {
		t.Fatalf("Ctrl-click without MultiSelect should still move Selected; got %d", l.Selected().Get())
	}
	if len(l.SelectedIndices()) != 0 {
		t.Fatalf("selection set must stay empty when MultiSelect is off; got %v", l.SelectedIndices())
	}

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0, Shift: true}) // row 0, Shift
	if l.Selected().Get() != 0 {
		t.Fatalf("Shift-click without MultiSelect should still move Selected; got %d", l.Selected().Get())
	}
	if len(l.SelectedIndices()) != 0 {
		t.Fatalf("selection set must stay empty when MultiSelect is off; got %v", l.SelectedIndices())
	}
}

func TestListBoxMultiSelectPlainClickCollapses(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d"})
	l.MultiSelect = true
	l.SetSelection(0, 1, 2)
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 3}) // plain click row 3
	if got := l.SelectedIndices(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("plain click must collapse selection to clicked row; got %v", got)
	}
	if l.Selected().Get() != 3 {
		t.Fatalf("Selected (anchor) = %d, want 3", l.Selected().Get())
	}
	if !l.IsSelected(3) || l.IsSelected(0) || l.IsSelected(1) || l.IsSelected(2) {
		t.Fatal("IsSelected disagrees with SelectedIndices after plain click")
	}
}

func TestListBoxMultiSelectCtrlToggle(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.MultiSelect = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0})                           // plain: select row 0
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 2, Ctrl: true}) // Ctrl: add row 2

	got := l.SelectedIndices()
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("Ctrl-click should add to selection; got %v", got)
	}
	if l.Selected().Get() != 2 {
		t.Fatalf("Ctrl-click should move the anchor; Selected=%d, want 2", l.Selected().Get())
	}

	// Ctrl-click an already-selected row toggles it OFF.
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0, Ctrl: true})
	got = l.SelectedIndices()
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("Ctrl-click on selected row should remove it; got %v", got)
	}
}

func TestListBoxMultiSelectShiftRangeForward(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.MultiSelect = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 1})              // anchor row 1
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 3, Shift: true}) // extend to row 3

	got := l.SelectedIndices()
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("SelectedIndices = %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("SelectedIndices = %v, want %v", got, want)
		}
	}
	if l.Selected().Get() != 1 {
		t.Fatalf("Shift-click must not move the anchor; Selected=%d, want 1", l.Selected().Get())
	}
}

func TestListBoxMultiSelectShiftRangeBackward(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.MultiSelect = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 3})              // anchor row 3
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 1, Shift: true}) // extend up to row 1

	got := l.SelectedIndices()
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("SelectedIndices = %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("SelectedIndices = %v, want %v", got, want)
		}
	}
	if l.Selected().Get() != 3 {
		t.Fatalf("Shift-click must not move the anchor; Selected=%d, want 3", l.Selected().Get())
	}
}

func TestListBoxSelectionSetAPI(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})

	// SetSelection replaces wholesale + drops out-of-range indices.
	l.SetSelection(3, 1, -1, 99)
	if got := l.SelectedIndices(); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("SetSelection = %v, want [1 3]", got)
	}
	if !l.IsSelected(1) || !l.IsSelected(3) || l.IsSelected(0) || l.IsSelected(2) {
		t.Fatal("IsSelected mismatch after SetSelection")
	}

	// ToggleSelect flips membership, guards out-of-range.
	l.ToggleSelect(0)
	if !l.IsSelected(0) {
		t.Fatal("ToggleSelect(0) should have added row 0")
	}
	l.ToggleSelect(0)
	if l.IsSelected(0) {
		t.Fatal("ToggleSelect(0) again should have removed row 0")
	}
	l.ToggleSelect(-1)
	l.ToggleSelect(99)
	if got := l.SelectedIndices(); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("out-of-range ToggleSelect must be a no-op; got %v", got)
	}

	// SelectRange, either order, clamped to bounds.
	l.SelectRange(4, 2)
	if got := l.SelectedIndices(); len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Fatalf("SelectRange(4,2) = %v, want [2 3 4]", got)
	}
	l.SelectRange(-5, 1)
	if got := l.SelectedIndices(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("SelectRange(-5,1) clamp = %v, want [0 1]", got)
	}
	l.SelectRange(3, 500)
	if got := l.SelectedIndices(); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("SelectRange(3,500) clamp = %v, want [3 4]", got)
	}

	// ClearSelection empties the set without touching Selected.
	l.Selected().Set(2)
	l.ClearSelection()
	if got := l.SelectedIndices(); len(got) != 0 {
		t.Fatalf("ClearSelection should leave an empty set; got %v", got)
	}
	if l.Selected().Get() != 2 {
		t.Fatalf("ClearSelection must not touch Selected; got %d", l.Selected().Get())
	}
}

func TestListBoxToggleSelectOnFreshList(t *testing.T) {
	// Regression: ToggleSelect must lazily initialise the selection set
	// when it has never been touched (selected == nil).
	l := NewListBox([]string{"a", "b"})
	l.ToggleSelect(0)
	if !l.IsSelected(0) {
		t.Fatal("ToggleSelect on a fresh ListBox should add the row")
	}
}

func TestListBoxSelectRangeEmptyList(t *testing.T) {
	l := NewListBox(nil)
	l.SelectRange(0, 3) // must not panic; nothing to select
	if got := l.SelectedIndices(); len(got) != 0 {
		t.Fatalf("SelectRange on empty list = %v, want []", got)
	}
}

func TestListBoxMultiSelectDrawHighlightsAllSelectedRows(t *testing.T) {
	const w, h = 64, 128
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b", "c", "d"})
	l.MultiSelect = true
	l.SetSelection(0, 2) // two NON-adjacent rows
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 80})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)

	if pixelAt(buf, w, 25, 5) != theme.Accent {
		t.Fatalf("row 0 bg = %+v, want Accent (selected)", pixelAt(buf, w, 25, 5))
	}
	if pixelAt(buf, w, 25, 25) != theme.Surface {
		t.Fatalf("row 1 bg = %+v, want Surface (unselected)", pixelAt(buf, w, 25, 25))
	}
	if pixelAt(buf, w, 25, 45) != theme.Accent {
		t.Fatalf("row 2 bg = %+v, want Accent (selected)", pixelAt(buf, w, 25, 45))
	}
	if pixelAt(buf, w, 25, 65) != theme.Surface {
		t.Fatalf("row 3 bg = %+v, want Surface (unselected)", pixelAt(buf, w, 25, 65))
	}
}

// --- ListBox virtual scrolling --------------------------------------------

// listSentinel is makeSurface's fill colour -- a pixel still holding it was
// never painted by the widget.
var listSentinel = RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}

func TestListBoxVisibleRowsCeilsPartialRow(t *testing.T) {
	l := NewListBox(make([]string, 10))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 45}) // 45/20 = 2 rem 5 -> ceil 3
	if got := l.visibleRows(); got != 3 {
		t.Fatalf("visibleRows() = %d, want 3", got)
	}
}

func TestListBoxVisibleRowsZeroRowHeight(t *testing.T) {
	l := NewListBox(make([]string, 10))
	l.RowHeight = 0
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	if got := l.visibleRows(); got != 0 {
		t.Fatalf("visibleRows() with RowHeight<=0 = %d, want 0", got)
	}
}

func TestListBoxVisibleRowsZeroHeightBounds(t *testing.T) {
	l := NewListBox(make([]string, 10))
	l.RowHeight = 20
	// SetBounds never called -> Bounds().H == 0.
	if got := l.visibleRows(); got != 0 {
		t.Fatalf("visibleRows() with H<=0 = %d, want 0", got)
	}
}

func TestListBoxSmallListDrawUnchangedNoScrollbar(t *testing.T) {
	// Regression: a list that fits entirely renders exactly like a
	// non-scrolling ListBox -- full-width rows, no scrollbar column.
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // plenty of room
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 46, 5); got != theme.Surface {
		t.Fatalf("row background should extend the full width (no scrollbar); got %+v, want Surface", got)
	}
	if got := pixelAt(buf, w, 46, 25); got != theme.Surface {
		t.Fatalf("row 1 background should extend the full width; got %+v, want Surface", got)
	}
}

func TestListBoxWindowedDrawOnlyPaintsVisibleRows(t *testing.T) {
	const w, h = 64, 128
	theme := DefaultLight()
	items := make([]string, 20)
	for i := range items {
		items[i] = "row"
	}
	l := NewListBox(items)
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // exactly 5 rows visible
	l.ScrollRow().Set(3)                         // window = rows [3,8)
	l.Selected().Set(5)                          // in-window
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)

	// Row 8 would land at local slot 5 (y in [100,120)) if drawn -- it
	// isn't, so that band must stay the untouched sentinel.
	if got := pixelAt(buf, w, 20, 105); got != listSentinel {
		t.Fatalf("off-window row 8 appears painted at slot 5; got %+v", got)
	}
	// Row 5 is in-window (local slot 2, y in [40,60)) and is Selected,
	// so it must paint Accent.
	if got := pixelAt(buf, w, 20, 45); got != theme.Accent {
		t.Fatalf("in-window selected row 5 bg = %+v, want Accent", got)
	}
	// Row 3 (first visible, local slot 0) paints Surface (unselected).
	if got := pixelAt(buf, w, 20, 5); got != theme.Surface {
		t.Fatalf("in-window row 3 bg = %+v, want Surface", got)
	}
}

func TestListBoxWindowedDrawWithoutClipperFallsBack(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	buf := makeSurface(64, 64)
	l.Draw(noClipPainter{newP(buf, 64)}, DefaultLight()) // must not panic
}

func TestListBoxDrawZeroHeightBoundsNoPanic(t *testing.T) {
	l := NewListBox(make([]string, 5))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 0})
	buf := makeSurface(64, 64)
	l.Draw(newP(buf, 64), DefaultLight()) // must not panic
}

func TestListBoxScrollbarZeroRowHeightNoPanic(t *testing.T) {
	l := NewListBox(make([]string, 5))
	l.RowHeight = 0
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	buf := makeSurface(64, 64)
	l.Draw(newP(buf, 64), DefaultLight()) // must not panic; no thumb (contentH=0)
}

func TestListBoxScrollbarThumbRendersOnOverflow(t *testing.T) {
	const w, h = 64, 128
	theme := DefaultLight()
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // 5 of 20 rows visible -> overflow
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)

	trackX := 50 - scrollbarWidth // 38
	foundTrack := false
	for y := 90; y < 100; y++ {
		if pixelAt(buf, w, trackX+3, y) == theme.SurfaceAlt {
			foundTrack = true
		}
	}
	if !foundTrack {
		t.Fatal("scrollbar track not painted")
	}
	foundThumb := false
	for y := 0; y < 20; y++ {
		if pixelAt(buf, w, trackX+scrollbarWidth/2, y) == scrollbarThumbColor(theme) {
			foundThumb = true
		}
	}
	if !foundThumb {
		t.Fatal("scrollbar thumb not painted")
	}
}

func TestListBoxScrollbarTinyThumbClamps(t *testing.T) {
	// Extremely long content -> the proportional thumb would be < 8px,
	// so it clamps to the 8px minimum (the thumbH<8 branch).
	const w, h = 64, 128
	theme := DefaultLight()
	l := NewListBox(make([]string, 100000))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme) // must not panic; thumb floored to 8

	trackX := 50 - scrollbarWidth
	found := false
	for y := 0; y < 8; y++ {
		if pixelAt(buf, w, trackX+scrollbarWidth/2, y) == scrollbarThumbColor(theme) {
			found = true
		}
	}
	if !found {
		t.Fatal("clamped scrollbar thumb not drawn")
	}
}

func TestListBoxScrollRowClampBothEnds(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // 5 visible -> maxScrollRow = 15
	l.ScrollRow().Set(-5)
	if got := l.clampedScrollRow(); got != 0 {
		t.Fatalf("negative ScrollRow should clamp to 0; got %d", got)
	}
	l.ScrollRow().Set(1000)
	if got := l.clampedScrollRow(); got != 15 {
		t.Fatalf("ScrollRow should clamp to maxScrollRow=15; got %d", got)
	}
}

func TestListBoxClickWithScrollRowSelectsCorrectRow(t *testing.T) {
	got := -1
	l := NewListBox(make([]string, 20))
	l.OnActivate = func(i int) { got = i }
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	l.ScrollRow().Set(3)
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 25}) // local slot 1 -> abs row 4
	if l.Selected().Get() != 4 {
		t.Fatalf("Selected = %d, want 4", l.Selected().Get())
	}
	if got != 4 {
		t.Fatalf("OnActivate fired with %d, want 4", got)
	}
}

func TestListBoxScrollToAndScrollByClamp(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // maxScrollRow = 15

	l.ScrollTo(1000)
	if l.ScrollRow().Get() != 15 {
		t.Fatalf("ScrollTo(1000) = %d, want 15", l.ScrollRow().Get())
	}
	l.ScrollTo(-1000)
	if l.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollTo(-1000) = %d, want 0", l.ScrollRow().Get())
	}
	l.ScrollTo(5)
	l.ScrollBy(3)
	if l.ScrollRow().Get() != 8 {
		t.Fatalf("ScrollBy(3) from 5 = %d, want 8", l.ScrollRow().Get())
	}
	l.ScrollBy(-100)
	if l.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollBy(-100) should clamp to 0; got %d", l.ScrollRow().Get())
	}
}

func TestListBoxScrollToSelectedNoSelectionIsNoOp(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	l.ScrollRow().Set(3)
	// Selected stays at -1 (NewListBox default).
	l.scrollToSelected()
	if l.ScrollRow().Get() != 3 {
		t.Fatalf("scrollToSelected with Selected=-1 must be a no-op; ScrollRow=%d, want 3", l.ScrollRow().Get())
	}
}

func TestListBoxScrollToSelectedScrollsUp(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // 5 visible
	l.ScrollRow().Set(10)
	l.Selected().Set(2) // above the window
	l.scrollToSelected()
	if l.ScrollRow().Get() != 2 {
		t.Fatalf("ScrollRow = %d, want 2 (scrolled up to Selected)", l.ScrollRow().Get())
	}
}

func TestListBoxScrollToSelectedScrollsDown(t *testing.T) {
	l := NewListBox(make([]string, 20))
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // 5 visible, window starts [0,5)
	l.Selected().Set(9)                          // below the window
	l.scrollToSelected()
	if l.ScrollRow().Get() != 5 { // Selected - vr + 1 = 9-5+1
		t.Fatalf("ScrollRow = %d, want 5", l.ScrollRow().Get())
	}
}

func TestListBoxScrollToSelectedZeroVisibleRowsIsNoOp(t *testing.T) {
	l := NewListBox(make([]string, 5))
	l.RowHeight = 0 // -> visibleRows() == 0
	l.Selected().Set(2)
	l.ScrollRow().Set(0)
	l.scrollToSelected()
	if l.ScrollRow().Get() != 0 {
		t.Fatalf("vr<=0 branch must be a no-op; ScrollRow=%d, want 0", l.ScrollRow().Get())
	}
}

func TestListBoxMultiSelectDrawIgnoresSelectedWhenSetEmpty(t *testing.T) {
	// Selected (the anchor) alone must NOT drive highlighting once
	// MultiSelect is on -- only the selection set does.
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.MultiSelect = true
	l.Selected().Set(1) // anchor points at row 1, but nothing is in the set
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 25, 25) != theme.Surface {
		t.Fatalf("row 1 bg = %+v, want Surface (set empty)", pixelAt(buf, w, 25, 25))
	}
}
