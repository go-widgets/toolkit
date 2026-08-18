// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- ListBox drag-to-reorder: Reorderable off (regression) ----------------

func TestListBoxNotReorderableDragDropIgnored(t *testing.T) {
	// Regression: with Reorderable false (the zero value), ListBox must not
	// participate in drag-and-drop at all -- DragData/AcceptsDrop are
	// inert and the drag event kinds are no-ops, exactly as before this
	// feature existed.
	l := NewListBox([]string{"a", "b", "c"})
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0}) // press row 0

	if got := l.DragData(); got != "" {
		t.Fatalf("DragData() = %q, want \"\" when not Reorderable", got)
	}
	if l.AcceptsDrop(ListRowDragPrefix + "0") {
		t.Fatal("AcceptsDrop must be false when not Reorderable, even for its own scheme")
	}

	reordered := false
	l.OnReorder = func(int, int) { reordered = true }

	l.OnEvent(Event{Kind: EventDragMove, Y: 30})
	if l.dropIndicator != -1 {
		t.Fatalf("dropIndicator = %d, want -1 (EventDragMove must be a no-op)", l.dropIndicator)
	}
	l.OnEvent(Event{Kind: EventDragLeave})
	if l.dropIndicator != -1 {
		t.Fatal("EventDragLeave must be a no-op when not Reorderable")
	}
	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "0", Y: 40})
	if reordered {
		t.Fatal("OnReorder must not fire when not Reorderable")
	}

	want := []string{"a", "b", "c"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want unchanged %v", l.Items, want)
		}
	}
}

func TestListBoxNotReorderableDrawUnchanged(t *testing.T) {
	// Regression: rendering with Reorderable false is byte-identical to a
	// ListBox with no drag-to-reorder support -- no insertion line, even
	// after an EventDragMove (which is a no-op above).
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.Selected().Set(1)
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	l.OnEvent(Event{Kind: EventDragMove, Y: 5})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 25, 5); got != theme.Surface {
		t.Fatalf("row 0 bg = %+v, want Surface", got)
	}
	if got := pixelAt(buf, w, 25, 25); got != theme.Accent {
		t.Fatalf("row 1 bg = %+v, want Accent", got)
	}
}

// --- ListBox drag-to-reorder: DragSource / DropTarget contract ------------

func TestListBoxReorderableDragData(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	if got := l.DragData(); got != "" {
		t.Fatalf("DragData() before any click = %q, want \"\"", got)
	}
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: l.RowHeight * 2}) // press row 2
	if got, want := l.DragData(), ListRowDragPrefix+"2"; got != want {
		t.Fatalf("DragData() = %q, want %q", got, want)
	}
}

func TestListBoxReorderableAcceptsDrop(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	l.Reorderable = true
	if !l.AcceptsDrop(ListRowDragPrefix + "0") {
		t.Fatal("AcceptsDrop must accept its own listrow: scheme")
	}
	if l.AcceptsDrop("/some/file/path") {
		t.Fatal("AcceptsDrop must reject a foreign payload")
	}
	if l.AcceptsDrop("") {
		t.Fatal("AcceptsDrop must reject an empty payload")
	}
}

// --- ListBox drag-to-reorder: indicator lifecycle --------------------------

func TestListBoxDragMoveSetsIndicatorAndDrawPaintsIt(t *testing.T) {
	const w, h = 64, 128
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.Reorderable = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // exactly 5 rows, no overflow

	l.OnEvent(Event{Kind: EventDragMove, Y: 55}) // bottom half of row 2 -> boundary 3
	if l.dropIndicator != 3 {
		t.Fatalf("dropIndicator = %d, want 3", l.dropIndicator)
	}

	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	found := false
	for y := 58; y <= 61; y++ {
		if pixelAt(buf, w, 25, y) == theme.Accent {
			found = true
		}
	}
	if !found {
		t.Fatal("drop indicator line not painted at the expected boundary")
	}
}

func TestListBoxReorderableNoIndicatorWhenNoneSet(t *testing.T) {
	// Reorderable alone (no EventDragMove yet) must not paint anything --
	// dropIndicator starts at -1.
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b"})
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 25, 0); got != theme.Surface {
		t.Fatalf("row 0 bg = %+v, want Surface (no indicator drawn)", got)
	}
}

func TestListBoxDropIndicatorOutsideWindowNotPainted(t *testing.T) {
	const w, h = 64, 128
	theme := DefaultLight()
	l := NewListBox(make([]string, 20))
	l.Reorderable = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100}) // 5 visible rows, window [0,5)
	l.dropIndicator = 15                         // way outside the visible window

	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme) // must not panic
	for i := 0; i < 5; i++ {
		y := 5 + i*20
		if got := pixelAt(buf, w, 25, y); got != theme.Surface {
			t.Fatalf("row %d bg = %+v, want Surface (out-of-window indicator must not paint)", i, got)
		}
	}
}

func TestListBoxDropIndicatorAtTopBoundaryClamps(t *testing.T) {
	// dropIndicator == 0 would paint half-above the content rect; Draw
	// clamps the line to cr.Y instead of letting it bleed above the widget.
	const w, h = 64, 64
	theme := DefaultLight()
	l := NewListBox([]string{"a", "b", "c"})
	l.Reorderable = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60})
	l.dropIndicator = 0

	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 25, 0); got != theme.Accent {
		t.Fatalf("indicator at top boundary not painted at y=0; got %+v", got)
	}
}

func TestListBoxDragLeaveClearsIndicator(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60})
	l.OnEvent(Event{Kind: EventDragMove, Y: 10})
	if l.dropIndicator < 0 {
		t.Fatal("expected dropIndicator to be set by EventDragMove")
	}
	l.OnEvent(Event{Kind: EventDragLeave})
	if l.dropIndicator != -1 {
		t.Fatalf("dropIndicator = %d, want -1 after EventDragLeave", l.dropIndicator)
	}
}

// --- ListBox drag-to-reorder: rowInsertionIndex -----------------------------

func TestListBoxRowInsertionIndexZeroRowHeight(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	l.RowHeight = 0
	if got := l.rowInsertionIndex(50); got != 0 {
		t.Fatalf("rowInsertionIndex with RowHeight<=0 = %d, want 0", got)
	}
}

func TestListBoxRowInsertionIndexNegativeY(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.RowHeight = 20
	if got := l.rowInsertionIndex(-5); got != 0 {
		t.Fatalf("rowInsertionIndex(-5) = %d, want 0", got)
	}
}

func TestListBoxRowInsertionIndexClampsToLength(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 40})
	if got := l.rowInsertionIndex(10000); got != 2 {
		t.Fatalf("rowInsertionIndex(10000) = %d, want 2 (clamped to len(Items))", got)
	}
}

// --- ListBox drag-to-reorder: dropping reorders Items -----------------------

func TestListBoxDropReordersMoveDown(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.Reorderable = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	l.Selected().Set(0) // the row being dragged is selected

	gotFrom, gotTo := -1, -1
	l.OnReorder = func(from, to int) { gotFrom, gotTo = from, to }

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0}) // press row 0 ("a")
	payload := l.DragData()
	l.OnEvent(Event{Kind: EventDrop, Code: payload, Y: 55}) // boundary 3

	want := []string{"b", "c", "a", "d", "e"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want %v", l.Items, want)
		}
	}
	if gotFrom != 0 || gotTo != 2 {
		t.Fatalf("OnReorder(%d,%d), want (0,2)", gotFrom, gotTo)
	}
	if l.Selected().Get() != 2 {
		t.Fatalf("Selected = %d, want 2 (follows the moved row)", l.Selected().Get())
	}
	if l.dropIndicator != -1 {
		t.Fatalf("dropIndicator = %d, want -1 after drop", l.dropIndicator)
	}
}

func TestListBoxDropReordersMoveUp(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.Reorderable = true
	l.MultiSelect = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	l.SetSelection(2, 4) // "c" (idx 2, below "from") and "e" (idx 4)

	gotFrom, gotTo := -1, -1
	l.OnReorder = func(from, to int) { gotFrom, gotTo = from, to }

	// Move row 3 ("d") up to before row 1 -- payload built directly (no
	// click) so the MultiSelect click-collapses-selection path above
	// doesn't disturb the SetSelection(2, 4) set under test.
	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "3", Y: 15}) // boundary 1

	want := []string{"a", "d", "b", "c", "e"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want %v", l.Items, want)
		}
	}
	if gotFrom != 3 || gotTo != 1 {
		t.Fatalf("OnReorder(%d,%d), want (3,1)", gotFrom, gotTo)
	}
	// "c" was at 2 (< from=3): unaffected by removing "d" from 3, and the
	// insertion at 1 sits at/below it, so it shifts down to 3. "e" was at
	// 4 (> from=3): shifts down to 3 by the removal, then the insertion at
	// 1 pushes it further to 4.
	got := l.SelectedIndices()
	want2 := []int{3, 4}
	if len(got) != len(want2) || got[0] != want2[0] || got[1] != want2[1] {
		t.Fatalf("SelectedIndices = %v, want %v", got, want2)
	}
}

func TestListBoxDropRemapsMultiSelectSet(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c", "d", "e"})
	l.Reorderable = true
	l.MultiSelect = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
	l.SetSelection(1, 3) // "b" and "d"

	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "0", Y: 55}) // move "a" to boundary 3

	want := []string{"b", "c", "a", "d", "e"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want %v", l.Items, want)
		}
	}
	got := l.SelectedIndices()
	wantSel := []int{0, 3} // "b" now at 0, "d" stays at 3
	if len(got) != len(wantSel) || got[0] != wantSel[0] || got[1] != wantSel[1] {
		t.Fatalf("SelectedIndices = %v, want %v", got, wantSel)
	}
}

func TestListBoxDragReorderRespectsScrollRow(t *testing.T) {
	items := make([]string, 10)
	for i := range items {
		items[i] = string(rune('0' + i))
	}
	l := NewListBox(items)
	l.Reorderable = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60}) // 3 visible rows -> overflow
	l.ScrollRow().Set(5)                        // window = rows [5,8)

	// Local Y=5 is in the top half of local slot 0 -> absolute row 5, not
	// row 0 -- proves rowInsertionIndex reads through ScrollRow.
	if got := l.rowInsertionIndex(5); got != 5 {
		t.Fatalf("rowInsertionIndex(5) with ScrollRow=5 = %d, want 5", got)
	}

	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0}) // press local slot 0 -> absolute row 5
	payload := l.DragData()
	if payload != ListRowDragPrefix+"5" {
		t.Fatalf("DragData() = %q, want %s5 (scrolled click hits absolute row 5)", payload, ListRowDragPrefix)
	}
	l.OnEvent(Event{Kind: EventDrop, Code: payload, Y: 59}) // bottom of local slot 2 -> boundary 8

	want := []string{"0", "1", "2", "3", "4", "6", "7", "5", "8", "9"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want %v", l.Items, want)
		}
	}
}

// --- ListBox drag-to-reorder: rejecting bad drops ---------------------------

func TestListBoxDropForeignPayloadIgnored(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60})
	fired := false
	l.OnReorder = func(int, int) { fired = true }

	l.OnEvent(Event{Kind: EventDrop, Code: "/some/file", Y: 10})
	if fired {
		t.Fatal("OnReorder must not fire for a foreign payload")
	}
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want unchanged %v", l.Items, want)
		}
	}
}

func TestListBoxDropGarbageIndexIgnored(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60})
	fired := false
	l.OnReorder = func(int, int) { fired = true }

	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "notanumber", Y: 10})
	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "99", Y: 10}) // out of range
	l.OnEvent(Event{Kind: EventDrop, Code: ListRowDragPrefix + "-1", Y: 10}) // negative

	if fired {
		t.Fatal("OnReorder must not fire for a garbage/out-of-range source index")
	}
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if l.Items[i] != w {
			t.Fatalf("Items = %v, want unchanged %v", l.Items, want)
		}
	}
}

// --- internal helpers -------------------------------------------------------

func TestRemapReorderedIndexBranches(t *testing.T) {
	cases := []struct {
		name          string
		idx, from, to int
		want          int
	}{
		{"moved item itself", 0, 0, 3, 2},                 // idx == from
		{"after from, lands before target", 1, 0, 3, 0},   // idx>from, j<target
		{"after from, lands at/after target", 3, 0, 3, 3}, // idx>from, j>=target
		{"before from, stays below target", 0, 3, 1, 0},   // idx<from, j<target
		{"before from, pushed by insertion", 2, 3, 1, 3},  // idx<from, j>=target
	}
	for _, c := range cases {
		if got := remapReorderedIndex(c.idx, c.from, c.to); got != c.want {
			t.Errorf("%s: remapReorderedIndex(%d,%d,%d) = %d, want %d", c.name, c.idx, c.from, c.to, got, c.want)
		}
	}
}

func TestListBoxMoveItemOutOfRangeFromIsNoOp(t *testing.T) {
	l := NewListBox([]string{"a", "b"})
	if got := l.moveItem(5, 0); got != 5 {
		t.Fatalf("moveItem(5,0) = %d, want 5 (out-of-range from is unchanged)", got)
	}
	if l.Items[0] != "a" || l.Items[1] != "b" {
		t.Fatal("moveItem with out-of-range from must not mutate Items")
	}
}

func TestListBoxMoveItemClampsOutOfRangeTarget(t *testing.T) {
	l := NewListBox([]string{"a", "b", "c"})
	if got := l.moveItem(0, -5); got != 0 {
		t.Fatalf("moveItem(0,-5) = %d, want 0 (target clamps to 0)", got)
	}
	l2 := NewListBox([]string{"a", "b", "c"})
	if got := l2.moveItem(0, 1000); got != 2 {
		t.Fatalf("moveItem(0,1000) = %d, want 2 (target clamps to len(rest))", got)
	}
}
