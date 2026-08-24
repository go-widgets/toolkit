// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// twoSections is the standard fixture: two labelled sections of two items each,
// laid out (with 18px rows) as the visual sequence
//
//	v0 header "Favorites"
//	v1 item   "Home"   (flat 0)
//	v2 item   "Docs"   (flat 1)
//	v3 header "Devices"
//	v4 item   "Disk"   (flat 2)
//	v5 item   "USB"    (flat 3)
func twoSections() []ListSection {
	return []ListSection{
		{Title: "Favorites", Items: []string{"Home", "Docs"}},
		{Title: "Devices", Items: []string{"Disk", "USB"}},
	}
}

// rgbEq compares two colours ignoring the alpha channel.
func rgbEq(a, b RGBA) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

// --- construction + counts ----------------------------------------------

func TestSectionedCountsAndRows(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	if !lb.sectioned() {
		t.Fatal("NewSectionedListBox should be in sectioned mode")
	}
	if got := lb.itemCount(); got != 4 {
		t.Fatalf("itemCount = %d, want 4 (headers excluded)", got)
	}
	if got := lb.rowCount(); got != 6 {
		t.Fatalf("rowCount = %d, want 6 (4 items + 2 headers)", got)
	}
	rows := lb.sectionRows()
	want := []lbRow{
		{header: true, text: "Favorites", item: -1},
		{text: "Home", item: 0},
		{text: "Docs", item: 1},
		{header: true, text: "Devices", item: -1},
		{text: "Disk", item: 2},
		{text: "USB", item: 3},
	}
	if len(rows) != len(want) {
		t.Fatalf("sectionRows len = %d, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i] != w {
			t.Fatalf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
	if got := lb.flatItems(); len(got) != 4 || got[0] != "Home" || got[3] != "USB" {
		t.Fatalf("flatItems = %v, want [Home Docs Disk USB]", got)
	}
}

// A section with an empty Title contributes no header row; a section with a Title
// but no Items contributes a header-only row and no selectable items; a wholly
// empty section contributes nothing.
func TestSectionedEdgeShapes(t *testing.T) {
	lb := NewSectionedListBox(
		ListSection{Title: "", Items: []string{"x", "y"}}, // no header
		ListSection{Title: "Empty", Items: nil},           // header only
		ListSection{Title: "", Items: nil},                // nothing
	)
	if got := lb.itemCount(); got != 2 {
		t.Fatalf("itemCount = %d, want 2", got)
	}
	if got := lb.rowCount(); got != 3 {
		t.Fatalf("rowCount = %d, want 3 (2 items + 1 header)", got)
	}
	rows := lb.sectionRows()
	want := []lbRow{
		{text: "x", item: 0},
		{text: "y", item: 1},
		{header: true, text: "Empty", item: -1},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Fatalf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

// --- rendering: header Y positions + distinct fills ---------------------

func TestSectionedHeaderPositionsAndFills(t *testing.T) {
	theme := DefaultLight()
	lb := NewSectionedListBox(twoSections()...)
	lb.Selected().Set(2) // Disk selected (flat 2, visual row 4, Y=72)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	buf := makeSurface(120, 6*18)
	lb.Draw(newP(buf, 120), theme)

	// Header bands at Y=0 (Favorites) and Y=54 (Devices) use SurfaceAlt.
	for _, hy := range []int{0 + 5, 54 + 5} {
		if got := pixelAt(buf, 120, 118, hy); !rgbEq(got, theme.SurfaceAlt) {
			t.Fatalf("header pixel at y=%d = %+v, want SurfaceAlt %+v", hy, got, theme.SurfaceAlt)
		}
	}
	// Unselected item "Home" at Y=18 uses Surface.
	if got := pixelAt(buf, 120, 118, 18+5); !rgbEq(got, theme.Surface) {
		t.Fatalf("unselected item pixel = %+v, want Surface %+v", got, theme.Surface)
	}
	// Selected item "Disk" at Y=72 uses Accent.
	if got := pixelAt(buf, 120, 118, 72+5); !rgbEq(got, theme.Accent) {
		t.Fatalf("selected item pixel = %+v, want Accent %+v", got, theme.Accent)
	}
}

// The ItemRenderer content seam is honoured in sectioned mode, receiving only
// item rows (never headers) with their flat indices and selection state.
func TestSectionedItemRenderer(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.Selected().Set(0)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	var calls []rendererCall
	lb.ItemRenderer = func(p painter.Painter, th *Theme, rc Rect, index int, item string, selected bool, ink RGBA) {
		calls = append(calls, rendererCall{rc, index, item, selected, ink})
	}
	buf := makeSurface(120, 6*18)
	lb.Draw(newP(buf, 120), DefaultLight())

	if len(calls) != 4 {
		t.Fatalf("renderer called %d times, want 4 (one per item, headers excluded)", len(calls))
	}
	// First item is "Home" at visual row 1 (Y=18), flat index 0, selected.
	if calls[0].index != 0 || calls[0].item != "Home" || calls[0].rc.Y != 18 || !calls[0].selected {
		t.Fatalf("call[0] = %+v, want index 0 item Home Y=18 selected", calls[0])
	}
	// Third item is "Disk" at visual row 4 (Y=72), flat index 2.
	if calls[2].index != 2 || calls[2].item != "Disk" || calls[2].rc.Y != 72 {
		t.Fatalf("call[2] = %+v, want index 2 item Disk Y=72", calls[2])
	}
}

// A sectioned list shorter than its viewport clamps the draw window to the row
// count (exercising the end>len(rows) guard) and still paints every item once.
func TestSectionedShorterThanViewport(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 10 * 18}) // room for 10, only 6 rows
	var n int
	lb.ItemRenderer = func(p painter.Painter, th *Theme, rc Rect, index int, item string, selected bool, ink RGBA) {
		n++
	}
	buf := makeSurface(120, 10*18)
	lb.Draw(newP(buf, 120), DefaultLight())
	if n != 4 {
		t.Fatalf("renderer called %d times, want 4 (all items, window clamped)", n)
	}
}

// --- click selection: items select, headers do not ----------------------

func TestSectionedClickSelectsItemsNotHeaders(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	var activated []int
	lb.OnActivate = func(idx int) { activated = append(activated, idx) }

	// Click the "Favorites" header (Y=0): nothing selects, nothing activates.
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 0})
	if got := lb.Selected().Get(); got != -1 {
		t.Fatalf("after header click Selected = %d, want -1 (unchanged)", got)
	}
	// Click item "Docs" (visual row 2, Y=36 -> flat index 1).
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 36})
	if got := lb.Selected().Get(); got != 1 {
		t.Fatalf("after Docs click Selected = %d, want 1", got)
	}
	// Click the "Devices" header (Y=54): selection is untouched.
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 54})
	if got := lb.Selected().Get(); got != 1 {
		t.Fatalf("after Devices-header click Selected = %d, want 1 (unchanged)", got)
	}
	// Click item "USB" (visual row 5, Y=90 -> flat index 3).
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 90})
	if got := lb.Selected().Get(); got != 3 {
		t.Fatalf("after USB click Selected = %d, want 3", got)
	}
	// Click below the last row: nothing changes.
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 500})
	if got := lb.Selected().Get(); got != 3 {
		t.Fatalf("after empty-space click Selected = %d, want 3 (unchanged)", got)
	}
	// Only the two item clicks activated, with the item indices (headers skipped).
	if len(activated) != 2 || activated[0] != 1 || activated[1] != 3 {
		t.Fatalf("activated = %v, want [1 3]", activated)
	}
}

// The Selected Observable notifies subscribers on a sectioned click.
func TestSectionedSelectionObservableNotifies(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	var seen []int
	lb.Selected().Subscribe(func(v int) { seen = append(seen, v) })
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 72}) // "Disk" -> flat 2
	if len(seen) == 0 || seen[len(seen)-1] != 2 {
		t.Fatalf("observable saw %v, want last value 2", seen)
	}
}

// A non-positive RowHeight and a negative Y both make a sectioned click inert.
func TestSectionedClickGuards(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})
	lb.Selected().Set(1)

	lb.RowHeight = 0
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: 18})
	if got := lb.Selected().Get(); got != 1 {
		t.Fatalf("click with RowHeight=0 changed Selected to %d, want 1", got)
	}
	lb.RowHeight = 18
	lb.OnEvent(Event{Kind: EventClick, X: 5, Y: -1})
	if got := lb.Selected().Get(); got != 1 {
		t.Fatalf("click with Y<0 changed Selected to %d, want 1", got)
	}
}

// --- keyboard navigation skips headers ----------------------------------

func TestSectionedKeyboardSkipsHeaders(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})
	lb.Selected().Set(0) // "Home"

	var activated []int
	lb.OnActivate = func(idx int) { activated = append(activated, idx) }

	// Down moves Home(0) -> Docs(1). Down again crosses the "Devices" header in
	// the visual layout but lands on Disk(2) in the contiguous item space.
	lb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if got := lb.Selected().Get(); got != 1 {
		t.Fatalf("after 1st ArrowDown Selected = %d, want 1", got)
	}
	lb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if got := lb.Selected().Get(); got != 2 {
		t.Fatalf("after 2nd ArrowDown Selected = %d, want 2 (header skipped)", got)
	}
	// Enter activates the cursor row with its flat index.
	lb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(activated) != 1 || activated[0] != 2 {
		t.Fatalf("activated = %v, want [2]", activated)
	}
}

// --- accessibility: headings + list items -------------------------------

func TestSectionedA11yTree(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.Selected().Set(2) // "Disk"
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	nodes := WalkA11y(lb)
	type rn struct {
		role Role
		name string
		val  string
	}
	got := make([]rn, len(nodes))
	for i, n := range nodes {
		got[i] = rn{n.Role, n.Name, n.Value}
	}
	want := []rn{
		{RoleListbox, "", "Disk"}, // the widget itself, value = selected item
		{RoleHeading, "Favorites", ""},
		{RoleListItem, "Home", ""},
		{RoleListItem, "Docs", ""},
		{RoleHeading, "Devices", ""},
		{RoleListItem, "Disk", "selected"},
		{RoleListItem, "USB", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("a11y tree has %d nodes, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("a11y node %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	// Proxy bounds place each row at its visual Y.
	if r := nodes[1].Rect; r.Y != 0 || r.H != 18 {
		t.Fatalf("Favorites heading rect = %+v, want Y=0 H=18", r)
	}
	if r := nodes[5].Rect; r.Y != 72 {
		t.Fatalf("Disk item rect Y = %d, want 72", r.Y)
	}
}

// The single-select A11y value reports the selected item's text through the flat
// section item space.
func TestSectionedA11yValue(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.Selected().Set(3)
	if got := lb.A11y().Value; got != "USB" {
		t.Fatalf("A11y().Value = %q, want USB", got)
	}
	lb.Selected().Set(-1)
	if got := lb.A11y().Value; got != "" {
		t.Fatalf("A11y().Value with no selection = %q, want empty", got)
	}
}

// --- back-compat: a flat ListBox is unchanged ---------------------------

func TestFlatListBoxUnchangedBySections(t *testing.T) {
	lb := NewListBox([]string{"a", "b", "c"})
	if lb.sectioned() {
		t.Fatal("a flat ListBox must not be sectioned")
	}
	if lb.itemCount() != 3 || lb.rowCount() != 3 {
		t.Fatalf("flat counts = (%d,%d), want (3,3)", lb.itemCount(), lb.rowCount())
	}
	if got := lb.flatItems(); len(got) != 3 || got[1] != "b" {
		t.Fatalf("flat flatItems = %v", got)
	}
	// A flat ListBox exposes no synthetic a11y children, so its a11y tree is the
	// single listbox node it always was.
	if ch := lb.Children(); ch != nil {
		t.Fatalf("flat ListBox.Children = %v, want nil", ch)
	}
	nodes := WalkA11y(lb)
	if len(nodes) != 1 || nodes[0].Role != RoleListbox {
		t.Fatalf("flat a11y tree = %+v, want one listbox node", nodes)
	}
}

// NewSectionedListBox with no sections is just a flat, empty ListBox.
func TestNewSectionedListBoxNoSections(t *testing.T) {
	lb := NewSectionedListBox()
	if lb.sectioned() {
		t.Fatal("NewSectionedListBox() with no sections must be flat")
	}
	if lb.Children() != nil {
		t.Fatal("empty NewSectionedListBox should expose no a11y children")
	}
}

// --- IndexAt in sectioned mode ------------------------------------------

func TestSectionedIndexAt(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})

	if got := lb.IndexAt(0, 0); got != -1 { // header
		t.Fatalf("IndexAt header = %d, want -1", got)
	}
	if got := lb.IndexAt(0, 18); got != 0 { // Home
		t.Fatalf("IndexAt Home = %d, want 0", got)
	}
	if got := lb.IndexAt(0, 72); got != 2 { // Disk
		t.Fatalf("IndexAt Disk = %d, want 2", got)
	}
	if got := lb.IndexAt(0, 5000); got != -1 { // past the end
		t.Fatalf("IndexAt past-end = %d, want -1", got)
	}
}

// --- reorder is disabled in sectioned mode ------------------------------

func TestSectionedDisablesReorder(t *testing.T) {
	lb := NewSectionedListBox(twoSections()...)
	lb.Reorderable = true
	lb.pressedRow = 0 // even with a pressed row on record
	if got := lb.DragData(); got != "" {
		t.Fatalf("sectioned DragData = %q, want empty (reorder disabled)", got)
	}
	if lb.AcceptsDrop(ListRowDragPrefix + "0") {
		t.Fatal("sectioned AcceptsDrop should be false (reorder disabled)")
	}
}

// --- scrolling: overflow, windowing, scroll-into-view -------------------

func manySections() []ListSection {
	return []ListSection{
		{Title: "A", Items: []string{"a0", "a1", "a2", "a3"}},
		{Title: "B", Items: []string{"b0", "b1", "b2", "b3"}},
	}
	// 10 visual rows: hA,a0,a1,a2,a3,hB,b0,b1,b2,b3 ; 8 items (0..7).
}

func TestSectionedOverflowWindowAndScrollbar(t *testing.T) {
	theme := DefaultLight()
	lb := NewSectionedListBox(manySections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 4 * 18}) // 4 rows visible, 10 total
	lb.ScrollRow().Set(2)                             // window starts at visual row 2

	// clipping painter path.
	buf := makeSurface(100, 4*18)
	lb.Draw(newP(buf, 100), theme)
	// A scrollbar is drawn on the right gutter while overflowing.
	if got := pixelAt(buf, 100, 100-scrollbarTrack()/2, 2); rgbEq(got, RGBA{R: 0xC8, G: 0xC8, B: 0xC8}) {
		t.Fatal("expected a scrollbar in the right gutter, found the sentinel")
	}

	// non-clipping painter path must not panic and still paints.
	buf2 := makeSurface(100, 4*18)
	lb.Draw(noClipPainter{newP(buf2, 100)}, theme)
}

func TestSectionedScrollToSelected(t *testing.T) {
	lb := NewSectionedListBox(manySections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 4 * 18}) // 4 rows visible

	// Selection cleared: scrollToSelected does nothing.
	lb.ScrollRow().Set(3)
	lb.Selected().Set(-1)
	lb.scrollToSelected()
	if got := lb.ScrollRow().Get(); got != 3 {
		t.Fatalf("cleared-selection scroll = %d, want 3 (unchanged)", got)
	}

	// Selected item above the window scrolls up to it. Item 0 (a0) is visual row 1.
	lb.ScrollRow().Set(6)
	lb.Selected().Set(0)
	lb.scrollToSelected()
	if got := lb.ScrollRow().Get(); got != 1 {
		t.Fatalf("scroll-up = %d, want 1 (visual row of item 0)", got)
	}

	// Selected item below the window scrolls down so it is the last visible row.
	// Item 7 (b3) is visual row 9; with 4 visible rows the top becomes 9-4+1=6.
	lb.ScrollRow().Set(0)
	lb.Selected().Set(7)
	lb.scrollToSelected()
	if got := lb.ScrollRow().Get(); got != 6 {
		t.Fatalf("scroll-down = %d, want 6", got)
	}

	// Already visible: no change. Item 7 is visual row 9, top 6 keeps it visible.
	lb.scrollToSelected()
	if got := lb.ScrollRow().Get(); got != 6 {
		t.Fatalf("already-visible scroll = %d, want 6 (unchanged)", got)
	}
}

// A zero-height sectioned list (no rows fit) leaves scroll-into-view a no-op via
// the visibleRows()<=0 guard, without dividing by zero.
func TestSectionedScrollToSelectedZeroHeight(t *testing.T) {
	lb := NewSectionedListBox(manySections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 0}) // no rows fit
	lb.ScrollRow().Set(0)
	lb.Selected().Set(5) // visual row 7, >= scrollRow, forces the vr<=0 branch
	lb.scrollToSelected()
	if got := lb.ScrollRow().Get(); got != 0 {
		t.Fatalf("zero-height scroll = %d, want 0 (no-op)", got)
	}
}

// Keyboard ArrowDown auto-scrolls a sectioned list to keep the moving cursor
// visible, mapping the flat item index through the header rows.
func TestSectionedKeyboardAutoScroll(t *testing.T) {
	lb := NewSectionedListBox(manySections()...)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 4 * 18}) // 4 rows visible
	lb.Selected().Set(2)                              // a2, visual row 3 (last visible from top 0)
	lb.ScrollRow().Set(0)

	lb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // -> a3, visual row 4
	if lb.Selected().Get() != 3 {
		t.Fatalf("cursor = %d, want 3", lb.Selected().Get())
	}
	if got := lb.ScrollRow().Get(); got != 1 {
		t.Fatalf("auto-scroll top = %d, want 1 (kept a3 visible)", got)
	}
}
