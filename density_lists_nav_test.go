// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Axis-B density adoption for the LISTS + NAV widget family.
//
// Every case restores the compact/1x baseline on exit (restoreDensity, defined
// in density_test.go) so a failure never leaks a non-default density into the
// rest of the suite. Two guarantees are proven per widget:
//
//   - DensityCompact is BYTE-IDENTICAL to the pre-change toolkit: at the default
//     MetricScale of 1 every effective metric equals its historical raw value,
//     and TouchTarget is a pass-through so a row/hit rect equals its drawn band
//     exactly.
//   - DensityTouch grows spacing metrics by exactly x1.5 and clamps interactive
//     row/item hit heights UP to the finger floor -- exactly max(44xMetricScale,
//     round(base x MetricScale x 1.5)) -- asserted to the pixel, never merely
//     "bigger".
//
// The new per-widget seams (list.rowHeight, menu.rowH/barH, table.rowH, ...) are
// CONTROL-RUN against the already-proven primitives they compose: each is
// asserted equal to the same TouchTarget(scaled(base)) (or TouchTarget(sc(base)))
// expression the density foundation validated via Switch.HitRect, at both
// densities, before any behavioural claim rests on them.

// wantClamp is the control expression every touch-clamped row/hit seam must
// equal: the base logical metric scaled by MetricScale and the touch density,
// then clamped UP to the density hit floor. Recomputing it from the proven
// primitives (scaled + TouchTarget) is the control the widget seams are run
// against.
func wantClamp(base int) int { return TouchTarget(scaled(base)) }

// TestListBoxRowHeightDensity control-runs ListBox.rowHeight against the proven
// clamp and pins its compact (byte-identical) and touch (>=44) values, plus the
// zero-height sentinel and the MetricScale-2 floor.
func TestListBoxRowHeightDensity(t *testing.T) {
	defer restoreDensity()

	// Compact: rowHeight == the configured RowHeight, byte-for-byte (18 default).
	l := NewListBox(manyItems(3))
	if l.RowHeight != 18 {
		t.Fatalf("compact NewListBox RowHeight = %d, want 18", l.RowHeight)
	}
	if got := l.rowHeight(); got != 18 {
		t.Fatalf("compact rowHeight() = %d, want 18 (pass-through)", got)
	}

	// The zero/negative sentinel survives BOTH densities: it must stay 0 (no
	// rows fit), never be conjured into 44 by the touch floor.
	zero := &ListBox{RowHeight: 0}
	if got := zero.rowHeight(); got != 0 {
		t.Fatalf("compact rowHeight() with RowHeight 0 = %d, want 0", got)
	}
	SetDensity(DensityTouch)
	if got := zero.rowHeight(); got != 0 {
		t.Fatalf("touch rowHeight() with RowHeight 0 = %d, want 0 (sentinel kept)", got)
	}

	// Touch: a list built at touch has RowHeight scaled(18)=27, clamped to 44.
	lt := NewListBox(manyItems(3))
	if lt.rowHeight() != 44 || lt.rowHeight() != wantClamp(18) {
		t.Fatalf("touch rowHeight() = %d, want 44 == wantClamp(18)=%d", lt.rowHeight(), wantClamp(18))
	}

	// A short explicit RowHeight clamps straight to the floor; the floor scales
	// with MetricScale only: 44 at 1x, 88 at 2x.
	small := &ListBox{RowHeight: 10}
	if got := small.rowHeight(); got != 44 {
		t.Fatalf("touch rowHeight() RowHeight=10 = %d, want 44", got)
	}
	SetMetricScale(2)
	if got := small.rowHeight(); got != 88 {
		t.Fatalf("touch@2x rowHeight() RowHeight=10 = %d, want 88 (floor x MetricScale)", got)
	}
}

// TestListBoxHitTouch proves the CLICK path maps through the 44px touch rows: a
// list built at touch selects row 1 for a click at y in [44,88).
func TestListBoxHitTouch(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch)
	l := NewListBox(manyItems(5))
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 300})
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 44}) // first pixel of row 1
	if l.Selected().Get() != 1 {
		t.Fatalf("touch click at y=44 selected row %d, want 1 (44px rows)", l.Selected().Get())
	}
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 43}) // last pixel of row 0
	if l.Selected().Get() != 0 {
		t.Fatalf("touch click at y=43 selected row %d, want 0", l.Selected().Get())
	}
}

// TestSourceListRowHeightDensity control-runs SourceList.rowHeight, pins the
// unclamped header, and proves the laid-out item rect (draw AND hit) reaches 44.
func TestSourceListRowHeightDensity(t *testing.T) {
	defer restoreDensity()
	sl := NewSourceList(SourceSection{Title: "Places", Items: []SourceItem{{Label: "Home"}}})
	if got := sl.rowHeight(); got != 28 {
		t.Fatalf("compact rowHeight() = %d, want 28", got)
	}
	if got := scaled(slHeaderH); got != 24 {
		t.Fatalf("compact header = %d, want 24", got)
	}
	sl.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 240})
	if h := sl.rows[1].rect.H; h != 28 { // rows[0] is the header, rows[1] the item
		t.Fatalf("compact item rect.H = %d, want 28", h)
	}

	SetDensity(DensityTouch)
	if got := sl.rowHeight(); got != 44 || got != wantClamp(slRowH) {
		t.Fatalf("touch rowHeight() = %d, want 44", got)
	}
	if got := scaled(slHeaderH); got != 36 {
		t.Fatalf("touch header = %d, want 36 (x1.5, unclamped)", got)
	}
	slt := NewSourceList(SourceSection{Title: "Places", Items: []SourceItem{{Label: "Home"}, {Label: "Docs"}}})
	slt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 240})
	if h := slt.rows[1].rect.H; h != 44 {
		t.Fatalf("touch item rect.H = %d, want 44", h)
	}
	// The click hit-test uses the same rects. Layout: top pad scaled(10)=15,
	// header scaled(24)=36 -> header [15,51); item 0 [51,95); item 1 [95,139).
	// A click at y=100 lands squarely on the second item.
	if y := slt.rows[2].rect.Y; y != 95 {
		t.Fatalf("touch second item rect.Y = %d, want 95", y)
	}
	got := -1
	slt.OnSelect = func(sec, row int) { got = row }
	slt.OnEvent(Event{Kind: EventClick, X: 20, Y: 100})
	if got != 1 {
		t.Fatalf("touch click at y=100 selected row %d, want 1 (44px rows)", got)
	}
}

// TestTreeViewRowHeightDensity covers both branches of TreeView.rowHeight (the
// zero-default fallback and the explicit height) at both densities.
func TestTreeViewRowHeightDensity(t *testing.T) {
	defer restoreDensity()

	def := &TreeView{} // RowHeight 0 -> default branch
	if got := def.rowHeight(); got != 18 {
		t.Fatalf("compact default rowHeight() = %d, want 18", got)
	}
	tv := NewTreeView(&TreeNode{Label: "r"})
	if got := tv.rowHeight(); got != 18 {
		t.Fatalf("compact NewTreeView rowHeight() = %d, want 18", got)
	}

	SetDensity(DensityTouch)
	if got := def.rowHeight(); got != 44 || got != wantClamp(18) {
		t.Fatalf("touch default rowHeight() = %d, want 44", got)
	}
	tvt := NewTreeView(&TreeNode{Label: "r"})
	if got := tvt.rowHeight(); got != 44 {
		t.Fatalf("touch NewTreeView rowHeight() = %d, want 44", got)
	}
}

// TestTreeTableRowHeightDensity control-runs TreeTable.rowH and confirms the
// header band is NOT clamped (a non-interactive row grows only x1.5).
func TestTreeTableRowHeightDensity(t *testing.T) {
	defer restoreDensity()
	tt := &TreeTable{}
	if got := tt.rowH(); got != 22 {
		t.Fatalf("compact rowH() = %d, want 22", got)
	}
	if got := scaled(TreeTableHeaderHeight); got != 24 {
		t.Fatalf("compact header = %d, want 24", got)
	}
	SetDensity(DensityTouch)
	if got := tt.rowH(); got != 44 || got != wantClamp(TreeTableRowHeight) {
		t.Fatalf("touch rowH() = %d, want 44", got)
	}
	if got := scaled(TreeTableHeaderHeight); got != 36 {
		t.Fatalf("touch header = %d, want 36 (x1.5, NOT clamped)", got)
	}
}

// TestTableRowHeightDensity control-runs Table.rowH and Table.cellPadX, and pins
// the header (unclamped) plus the 2x floor.
func TestTableRowHeightDensity(t *testing.T) {
	defer restoreDensity()
	tb := &Table{}
	if tb.rowH() != 22 || tb.cellPadX() != 4 {
		t.Fatalf("compact rowH=%d cellPadX=%d, want 22/4", tb.rowH(), tb.cellPadX())
	}
	if got := scaled(TableHeaderHeight); got != 24 {
		t.Fatalf("compact header = %d, want 24", got)
	}
	SetDensity(DensityTouch)
	if tb.rowH() != 44 || tb.rowH() != wantClamp(TableRowHeight) {
		t.Fatalf("touch rowH() = %d, want 44", tb.rowH())
	}
	if tb.cellPadX() != 6 {
		t.Fatalf("touch cellPadX() = %d, want 6 (x1.5)", tb.cellPadX())
	}
	if got := scaled(TableHeaderHeight); got != 36 {
		t.Fatalf("touch header = %d, want 36 (unclamped)", got)
	}
	SetMetricScale(2)
	if got := tb.rowH(); got != 88 {
		t.Fatalf("touch@2x rowH() = %d, want 88 (floor x MetricScale)", got)
	}
}

// TestTableHitTouch proves the row hit-test maps through the 44px touch rows.
func TestTableHitTouch(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch)
	tb := NewTable(
		[]TableColumn{{Title: "A"}},
		[][]string{{"r0"}, {"r1"}, {"r2"}},
	)
	tb.MultiSelect = true // a plain click only moves Selected in multi-select mode
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 300})
	// Header is scaled(24)=36 tall; body row 0 spans [36,80), row 1 [80,124).
	tb.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	if tb.Selected != 0 {
		t.Fatalf("touch click at y=50 selected %d, want row 0", tb.Selected)
	}
	tb.OnEvent(Event{Kind: EventClick, X: 5, Y: 90}) // 36 + 44 -> row 1
	if tb.Selected != 1 {
		t.Fatalf("touch click at y=90 selected %d, want row 1 (44px rows)", tb.Selected)
	}
}

// TestMenuRowHeightDensity control-runs Menu.sc (now density-aware) and Menu.rowH
// and pins the separator (unclamped) height.
func TestMenuRowHeightDensity(t *testing.T) {
	defer restoreDensity()
	m := NewMenu([]MenuItem{{Label: "a", Action: func() {}}})
	if m.sc(MenuRowH) != 22 || m.rowH() != 22 || m.sc(8) != 8 {
		t.Fatalf("compact sc(22)=%d rowH=%d sc(8)=%d, want 22/22/8", m.sc(MenuRowH), m.rowH(), m.sc(8))
	}
	SetDensity(DensityTouch)
	if m.sc(MenuRowH) != 33 {
		t.Fatalf("touch sc(MenuRowH) = %d, want 33 (x1.5)", m.sc(MenuRowH))
	}
	if m.rowH() != 44 || m.rowH() != TouchTarget(m.sc(MenuRowH)) {
		t.Fatalf("touch rowH() = %d, want 44", m.rowH())
	}
	if m.sc(8) != 12 {
		t.Fatalf("touch sc(8) = %d, want 12 (x1.5)", m.sc(8))
	}
	if m.sc(MenuSeparatorH) != 9 {
		t.Fatalf("touch sc(MenuSeparatorH) = %d, want 9 (x1.5, unclamped)", m.sc(MenuSeparatorH))
	}
}

// TestMenuHitTouch proves rowAt maps clicks through 44px touch rows.
func TestMenuHitTouch(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch)
	m := NewMenu([]MenuItem{{Label: "a", Action: func() {}}, {Label: "b", Action: func() {}}})
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	// Body inset sc(2)=3; row 0 spans [3,47), row 1 [47,91).
	if got := m.rowAt(3); got != 0 {
		t.Fatalf("touch rowAt(3) = %d, want 0", got)
	}
	if got := m.rowAt(47); got != 1 {
		t.Fatalf("touch rowAt(47) = %d, want 1 (44px rows)", got)
	}
}

// TestMenuBarDensity control-runs MenuBar.barH and NameWidth across the floor,
// out-of-range and clamp branches.
func TestMenuBarDensity(t *testing.T) {
	defer restoreDensity()
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	if got := b.barH(); got != 22 {
		t.Fatalf("compact barH() = %d, want 22", got)
	}
	// Out-of-range name -> the floor, byte-identical (60) at compact.
	if got := b.NameWidth(9); got != 60 {
		t.Fatalf("compact NameWidth(oob) = %d, want floor 60", got)
	}
	compactFile := b.NameWidth(0)
	if compactFile < 60 {
		t.Fatalf("compact NameWidth(File) = %d, want >= floor 60", compactFile)
	}

	SetDensity(DensityTouch)
	if got := b.barH(); got != 44 {
		t.Fatalf("touch barH() = %d, want 44", got)
	}
	// Floor scales x1.5 to 90 (already > 44, so the hit clamp is a no-op here):
	// the out-of-range width proves the floor branch exactly.
	if got := b.NameWidth(9); got != 90 {
		t.Fatalf("touch NameWidth(oob) = %d, want 90 (scaled floor)", got)
	}
	if got := b.NameWidth(0); got < 90 {
		t.Fatalf("touch NameWidth(File) = %d, want >= 90", got)
	}
}

// TestNotebookStripDensity control-runs Notebook.stripH.
func TestNotebookStripDensity(t *testing.T) {
	defer restoreDensity()
	n := NewNotebook()
	if got := n.stripH(); got != 24 {
		t.Fatalf("compact stripH() = %d, want 24", got)
	}
	SetDensity(DensityTouch)
	if got := n.stripH(); got != 44 || got != wantClamp(NotebookTabStripH) {
		t.Fatalf("touch stripH() = %d, want 44", got)
	}
}

// TestNotebookTabHitTouch proves a Top-strip tab click lands with a 44px strip.
func TestNotebookTabHitTouch(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch)
	n := NewNotebook()
	n.AddTab("one", nil)
	n.AddTab("two", nil)
	n.SetBounds(Rect{X: 0, Y: 0, W: 320, H: 200})
	// Seed the active tab to 1 so a click that selects tab 0 is an observable
	// change, letting the subscriber prove the click landed on the tab row.
	n.Active().Set(1)
	got := -1
	n.Active().Subscribe(func(i int) { got = i })
	// Top strip is 44 tall; a click at y=40 is still on the tab row.
	n.OnEvent(Event{Kind: EventClick, X: 10, Y: 40})
	if n.Active().Get() != 0 || got != 0 {
		t.Fatalf("touch tab click Active=%d cb=%d, want 0/0 (44px strip)", n.Active().Get(), got)
	}
}

// TestExpanderHeaderDensity control-runs ExpanderHeaderHeight and proves the
// header hit band grows with it.
func TestExpanderHeaderDensity(t *testing.T) {
	defer restoreDensity()
	if got := ExpanderHeaderHeight(); got != 24 {
		t.Fatalf("compact ExpanderHeaderHeight() = %d, want 24", got)
	}
	SetDensity(DensityTouch)
	if got := ExpanderHeaderHeight(); got != 44 || got != wantClamp(ExpanderHeaderH) {
		t.Fatalf("touch ExpanderHeaderHeight() = %d, want 44", got)
	}
	// A click at y=40 (inside the 44px header, past the old 24px one) toggles.
	e := NewExpander("x", nil)
	got := false
	e.Expanded().Subscribe(func(b bool) { got = b })
	e.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 40})
	if !e.Expanded().Get() || !got {
		t.Fatalf("touch header click at y=40 did not toggle (Expanded=%v)", e.Expanded().Get())
	}
}

// TestViewSwitcherHeightDensity covers ViewSwitcherHeight, including the
// no-clamp path: 32x1.5 = 48 already clears the 44 floor.
func TestViewSwitcherHeightDensity(t *testing.T) {
	defer restoreDensity()
	if got := ViewSwitcherHeight(); got != 32 {
		t.Fatalf("compact ViewSwitcherHeight() = %d, want 32", got)
	}
	SetDensity(DensityTouch)
	if got := ViewSwitcherHeight(); got != 48 {
		t.Fatalf("touch ViewSwitcherHeight() = %d, want 48 (x1.5, above floor -> no clamp)", got)
	}
	if got := ViewSwitcherHeight(); got != wantClamp(ViewSwitcherH) {
		t.Fatalf("touch ViewSwitcherHeight() = %d, want == wantClamp(32)=%d", got, wantClamp(ViewSwitcherH))
	}
}

// TestPaginationButtonDensity control-runs the pagination button seams; both
// axes clamp to the floor at touch (28x1.5=42 and 24x1.5=36 are below 44).
func TestPaginationButtonDensity(t *testing.T) {
	defer restoreDensity()
	pg := NewPagination(1, 5)
	if pg.btnW() != 28 || pg.btnH() != 24 || pg.gap() != 2 {
		t.Fatalf("compact btnW=%d btnH=%d gap=%d, want 28/24/2", pg.btnW(), pg.btnH(), pg.gap())
	}
	SetDensity(DensityTouch)
	if pg.btnW() != 44 || pg.btnW() != wantClamp(PaginationBtnW) {
		t.Fatalf("touch btnW() = %d, want 44", pg.btnW())
	}
	if pg.btnH() != 44 || pg.btnH() != wantClamp(PaginationBtnH) {
		t.Fatalf("touch btnH() = %d, want 44", pg.btnH())
	}
	if pg.gap() != 3 {
		t.Fatalf("touch gap() = %d, want 3 (x1.5, spacer unclamped)", pg.gap())
	}
}

// TestPaginationHitTouch proves the click stride follows the touch button size.
func TestPaginationHitTouch(t *testing.T) {
	defer restoreDensity()
	SetDensity(DensityTouch)
	pg := NewPagination(1, 5)
	pg.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 60})
	got := -1
	pg.Current().Subscribe(func(page int) { got = page })
	// stride = 44 + 3 = 47. Slot 0 is prev, slots 1.. are pages: page "1" is
	// idx 1 (already current), page "2" is idx 2 at x in [94, 138).
	pg.OnEvent(Event{Kind: EventClick, X: 94, Y: 10})
	if got != 2 {
		t.Fatalf("touch click on second button changed to page %d, want 2", got)
	}
}

// TestStepsBadgeHitDensity control-runs the Steps drawn-badge scaling and the
// clamped, centred tap box (the Switch.HitRect pattern) WITHOUT changing what is
// drawn.
func TestStepsBadgeHitDensity(t *testing.T) {
	defer restoreDensity()
	// Compact: the hit box equals the drawn 16x16 badge byte-for-byte, so a
	// click at x=15 (inside) hits and x=16 (outside) misses.
	// Start at -1 so a jump to badge 0 is a real change the subscriber sees;
	// a miss leaves Current untouched so hit stays at the sentinel.
	s := NewSteps([]string{"a", "b"}, -1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 40})
	hit := -1
	s.Current().Subscribe(func(i int) { hit = i })
	yMid := (40 - scaled(StepBoxH)) / 2 // vertical centre offset used by Draw/OnEvent
	s.OnEvent(Event{Kind: EventClick, X: 15, Y: yMid})
	if hit != 0 {
		t.Fatalf("compact click at badge edge x=15 hit %d, want 0", hit)
	}
	hit = -1
	s.OnEvent(Event{Kind: EventClick, X: 16, Y: yMid})
	if hit != -1 {
		t.Fatalf("compact click just past badge x=16 hit %d, want miss", hit)
	}

	// Touch: badge drawn 24x24, hit box clamped to 44 centred over it, so the
	// first badge's tap box spans x in [-10, 34) and y in [yc-10, yc+34).
	SetDensity(DensityTouch)
	st := NewSteps([]string{"a", "b"}, -1)
	st.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 60})
	sthit := -1
	st.Current().Subscribe(func(i int) { sthit = i })
	yc := (60 - scaled(StepBoxH)) / 2
	// x=33 is inside the 44-wide hit box but well past the 24-wide drawn badge.
	st.OnEvent(Event{Kind: EventClick, X: 33, Y: yc})
	if sthit != 0 {
		t.Fatalf("touch click at x=33 (inside 44px hit box) hit %d, want 0", sthit)
	}
}

// TestBreadcrumbGapDensity pins the crumb gap: byte-identical at compact, x1.5 at
// touch. The gap drives both the drawn separator position and the hit walk, so
// asserting it is asserting both stay aligned.
func TestBreadcrumbGapDensity(t *testing.T) {
	defer restoreDensity()
	if got := scaled(BreadcrumbGap); got != 4 {
		t.Fatalf("compact scaled(BreadcrumbGap) = %d, want 4", got)
	}
	SetDensity(DensityTouch)
	if got := scaled(BreadcrumbGap); got != 6 {
		t.Fatalf("touch scaled(BreadcrumbGap) = %d, want 6 (x1.5)", got)
	}
}

// TestContextMenuInheritsTouch proves ContextMenu needs no metrics of its own:
// its measured popup height follows the wrapped Menu straight into the touch
// profile (the rows it sizes to are Menu.rowH-tall).
func TestContextMenuInheritsTouch(t *testing.T) {
	defer restoreDensity()
	menu := NewMenu([]MenuItem{{Label: "a", Action: func() {}}, {Label: "b", Action: func() {}}})
	cm := NewContextMenu(menu)
	_, hCompact := cm.menuSize()
	SetDensity(DensityTouch)
	_, hTouch := cm.menuSize()
	// Two 44px rows + sc(4)=6 inset at touch vs two 22px rows + 4 at compact.
	if hCompact != 2*22+4 {
		t.Fatalf("compact ContextMenu height = %d, want %d", hCompact, 2*22+4)
	}
	if hTouch != 2*44+6 {
		t.Fatalf("touch ContextMenu height = %d, want %d (inherits Menu.rowH)", hTouch, 2*44+6)
	}
}
