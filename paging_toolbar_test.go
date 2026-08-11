// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// ptClick clicks the centre of rc on pt (bounds pinned at origin, so
// surface == local coordinates).
func ptClick(pt *PagingToolbar, rc Rect) {
	pt.OnEvent(Event{Kind: EventClick, X: rc.X + rc.W/2, Y: rc.Y + rc.H/2})
}

// newBoundPagingToolbar builds a toolbar at page/count with bounds pinned at
// the origin so layout() rects are directly clickable.
func newBoundPagingToolbar(page, count int) *PagingToolbar {
	pt := NewPagingToolbar(page, count)
	pt.SetBounds(Rect{X: 0, Y: 0, W: 260, H: PagingBtnH})
	return pt
}

// TestPagingToolbarNewClamps covers the New clamps.
func TestPagingToolbarNewClamps(t *testing.T) {
	if got := NewPagingToolbar(9, 5).Page; got != 5 {
		t.Fatalf("page over count clamps to %d, want 5", got)
	}
	if got := NewPagingToolbar(0, 5).Page; got != 1 {
		t.Fatalf("page under 1 clamps to %d, want 1", got)
	}
	e := NewPagingToolbar(3, 0)
	if e.PageCount != 1 || e.Page != 1 {
		t.Fatalf("empty grid = Page %d of %d, want 1 of 1", e.Page, e.PageCount)
	}
	if e.info() != "Page 1 of 1" {
		t.Fatalf("info = %q, want %q", e.info(), "Page 1 of 1")
	}
}

// TestPagingToolbarNavigation covers First/Prev/Next/Last stepping + OnChange.
func TestPagingToolbarNavigation(t *testing.T) {
	pt := newBoundPagingToolbar(3, 5)
	var pages []int
	pt.OnChange = func(p int) { pages = append(pages, p) }
	l := pt.layout()

	ptClick(pt, l.prev)  // 3 → 2
	ptClick(pt, l.next)  // 2 → 3
	ptClick(pt, l.first) // 3 → 1
	ptClick(pt, l.last)  // 1 → 5
	if pt.Page != 5 {
		t.Fatalf("final Page = %d, want 5", pt.Page)
	}
	want := []int{2, 3, 1, 5}
	if len(pages) != len(want) {
		t.Fatalf("OnChange pages = %v, want %v", pages, want)
	}
	for i := range want {
		if pages[i] != want[i] {
			t.Fatalf("OnChange[%d] = %d, want %d", i, pages[i], want[i])
		}
	}
}

// TestPagingToolbarDisabledEnds: First/Prev at page 1 and Next/Last at the
// last page are no-ops (no OnChange).
func TestPagingToolbarDisabledEnds(t *testing.T) {
	fired := 0
	// At page 1: First/Prev inert.
	lo := newBoundPagingToolbar(1, 5)
	lo.OnChange = func(int) { fired++ }
	l := lo.layout()
	ptClick(lo, l.first)
	ptClick(lo, l.prev)
	if lo.Page != 1 || fired != 0 {
		t.Fatalf("page-1 First/Prev changed state: Page %d, fired %d", lo.Page, fired)
	}
	// At last page: Next/Last inert.
	hi := newBoundPagingToolbar(5, 5)
	hi.OnChange = func(int) { fired++ }
	l = hi.layout()
	ptClick(hi, l.next)
	ptClick(hi, l.last)
	if hi.Page != 5 || fired != 0 {
		t.Fatalf("last-page Next/Last changed state: Page %d, fired %d", hi.Page, fired)
	}
}

// TestPagingToolbarRefresh covers the Refresh button (present, clicked; and
// absent by default).
func TestPagingToolbarRefresh(t *testing.T) {
	pt := newBoundPagingToolbar(2, 5)
	pt.ShowRefresh = true
	refreshed := 0
	pt.OnRefresh = func() { refreshed++ }
	l := pt.layout()
	if !l.hasRefresh {
		t.Fatal("layout should include a refresh slot when ShowRefresh")
	}
	ptClick(pt, l.refresh)
	if refreshed != 1 {
		t.Fatalf("OnRefresh calls = %d, want 1", refreshed)
	}

	// Without ShowRefresh the refresh slot is absent, so a click there does
	// nothing (and OnRefresh nil is safe even if somehow reached).
	pt.ShowRefresh = false
	l2 := pt.layout()
	if l2.hasRefresh {
		t.Fatal("no refresh slot expected when ShowRefresh is false")
	}
}

// TestPagingToolbarRefreshNilSafe: clicking refresh with OnRefresh unset is a
// no-op (no panic).
func TestPagingToolbarRefreshNilSafe(t *testing.T) {
	pt := newBoundPagingToolbar(2, 5)
	pt.ShowRefresh = true // OnRefresh nil
	ptClick(pt, pt.layout().refresh)
}

// TestPagingToolbarInertClicks: a non-click event, a click on the indicator,
// and a click outside every element are all no-ops.
func TestPagingToolbarInertClicks(t *testing.T) {
	pt := newBoundPagingToolbar(3, 5)
	fired := false
	pt.OnChange = func(int) { fired = true }

	pt.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})  // not a click
	ptClick(pt, pt.layout().info)                         // indicator, inert
	pt.OnEvent(Event{Kind: EventClick, X: 5000, Y: 5000}) // far outside
	if fired || pt.Page != 3 {
		t.Fatalf("inert clicks changed state: fired %v Page %d", fired, pt.Page)
	}
}

// TestPagingToolbarDraw renders the toolbar (enabled + disabled tones, with
// and without refresh) and asserts it paints.
func TestPagingToolbarDraw(t *testing.T) {
	const w, h = 260, PagingBtnH
	pt := newBoundPagingToolbar(1, 5) // page 1 → First/Prev disabled tone
	pt.ShowRefresh = true
	buf := makeSurface(w, h)
	pt.Draw(newP(buf, w), DefaultLight())
	if !anyPainted(buf, w, 0, 0, w, h) {
		t.Fatal("PagingToolbar painted nothing")
	}

	// Empty bounds → no-op.
	empty := NewPagingToolbar(1, 3)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 0, H: h})
	buf2 := makeSurface(w, h)
	empty.Draw(newP(buf2, w), DefaultLight())
	if anyPainted(buf2, w, 0, 0, w, h) {
		t.Fatal("empty-bounds PagingToolbar must paint nothing")
	}
}
