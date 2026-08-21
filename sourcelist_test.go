// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// solidIcon builds a 1×1 image of colour c; stretched over any bounds it fills
// them with c, so a test can assert an icon painted at a known cell.
func solidIcon(c RGBA) *Image {
	return NewImage([]byte{c.R, c.G, c.B, c.A}, 1, 1)
}

// sampleSourceList builds a three-section list: a reorderable "Favoris" (with
// icons, one row iconless), a fixed "Places" (one very long label to force
// elision), and a title-less trailing section (no header row).
func sampleSourceList() *SourceList {
	ic := solidIcon(RGB(0x11, 0x22, 0x33))
	return NewSourceList(
		SourceSection{Title: "Favoris", Reorderable: true, Items: []SourceItem{
			{Label: "Alpha", Icon: ic, Key: "a"},
			{Label: "Beta", Icon: ic, Key: "b"},
			{Label: "Gamma", Key: "c"},
		}},
		SourceSection{Title: "Places", Items: []SourceItem{
			{Label: "Home", Key: "h"},
			{Label: "A location whose label is far too long to fit the narrow sidebar", Key: "x"},
		}},
		SourceSection{Items: []SourceItem{
			{Label: "Untitled", Key: "u"},
		}},
	)
}

func TestSourceListLayoutRects(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})

	if got := len(sl.rows); got != 8 {
		t.Fatalf("row count = %d, want 8", got)
	}
	// Header 0 at topPad; first item just below it.
	if sl.rows[0].row != -1 || sl.rows[0].rect != (Rect{X: 0, Y: slTopPad, W: 180, H: slHeaderH}) {
		t.Fatalf("header0 = %+v", sl.rows[0])
	}
	if want := (Rect{X: 0, Y: 34, W: 180, H: slRowH}); sl.rows[1].rect != want {
		t.Fatalf("Alpha row rect = %+v, want %+v", sl.rows[1].rect, want)
	}
	if !sl.rows[1].reorderable {
		t.Fatalf("Alpha row should be reorderable")
	}
	// Places header after Favoris (3 rows + sect gap).
	if want := (Rect{X: 0, Y: 126, W: 180, H: slHeaderH}); sl.rows[4].rect != want || sl.rows[4].row != -1 {
		t.Fatalf("Places header = %+v, want rect %+v", sl.rows[4], want)
	}
	// Title-less section contributes NO header row: its item lands directly.
	last := sl.rows[7]
	if last.section != 2 || last.row != 0 || last.reorderable {
		t.Fatalf("trailing item = %+v", last)
	}
	if want := (Rect{X: 0, Y: 214, W: 180, H: slRowH}); last.rect != want {
		t.Fatalf("trailing item rect = %+v, want %+v", last.rect, want)
	}
}

func TestSourceListDrawSelectionAndChrome(t *testing.T) {
	sl := sampleSourceList()
	th := DefaultLight()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.SetSelected(0, 0) // Alpha

	w, h := 180, 400
	buf := makeSurface(w, h)
	sl.Draw(newP(buf, w), th)

	// Right-edge hairline is the border colour.
	if got := pixelAt(buf, w, w-1, 5); got != th.Border {
		t.Fatalf("right hairline = %+v, want Border %+v", got, th.Border)
	}
	// Panel body away from any row is SurfaceAlt.
	if got := pixelAt(buf, w, 90, 122); got != th.SurfaceAlt {
		t.Fatalf("panel body = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// Selected Alpha row paints an Accent pill at its centre.
	if got := pixelAt(buf, w, 90, 48); got != th.Accent {
		t.Fatalf("selection pill centre = %+v, want Accent %+v", got, th.Accent)
	}
	// Beta row (index 2, unselected) shows its leading icon at the icon cell.
	iconC := RGB(0x11, 0x22, 0x33)
	if got := pixelAt(buf, w, slLeftPad+8, 62+5+8); got != iconC {
		t.Fatalf("Beta icon pixel = %+v, want %+v", got, iconC)
	}
}

func TestSourceListClickSelectsAndCallsBack(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	var gotSec, gotRow int = -9, -9
	sl.OnSelect = func(section, row int) { gotSec, gotRow = section, row }

	// Click Home (section 1, row 0) at Y in [150,178): a non-reorderable row.
	sl.OnEvent(Event{Kind: EventClick, Y: 160})
	if gotSec != 1 || gotRow != 0 {
		t.Fatalf("OnSelect got (%d,%d), want (1,0)", gotSec, gotRow)
	}
	if s, r := sl.Selected(); s != 1 || r != 0 {
		t.Fatalf("Selected() = (%d,%d), want (1,0)", s, r)
	}
	// A non-reorderable row arms no drag payload.
	if got := sl.DragData(); got != "" {
		t.Fatalf("DragData after non-reorderable click = %q, want empty", got)
	}

	// Click on a header row (Y in the Favoris header band) selects nothing new.
	sl.OnEvent(Event{Kind: EventClick, Y: 12})
	if s, r := sl.Selected(); s != 1 || r != 0 {
		t.Fatalf("Selected() after header click = (%d,%d), want unchanged (1,0)", s, r)
	}
	// Click below every row is a miss too.
	sl.OnEvent(Event{Kind: EventClick, Y: 390})
	if s, _ := sl.Selected(); s != 1 {
		t.Fatalf("Selected() after empty-space click changed unexpectedly")
	}
}

func TestSourceListClickNilCallbackNoPanic(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.OnEvent(Event{Kind: EventClick, Y: 40}) // Alpha, OnSelect nil
	if s, r := sl.Selected(); s != 0 || r != 0 {
		t.Fatalf("Selected() = (%d,%d), want (0,0)", s, r)
	}
}

func TestSourceListSetSelectedValidation(t *testing.T) {
	sl := sampleSourceList()
	sl.SetSelected(1, 1)
	if s, r := sl.Selected(); s != 1 || r != 1 {
		t.Fatalf("valid SetSelected -> (%d,%d), want (1,1)", s, r)
	}
	sl.SetSelected(9, 0) // section out of range clears selection
	if s, r := sl.Selected(); s != -1 || r != -1 {
		t.Fatalf("invalid SetSelected -> (%d,%d), want (-1,-1)", s, r)
	}
}

func TestSourceListDisabledIsInert(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.Disabled().Set(true)
	sl.OnEvent(Event{Kind: EventClick, Y: 40})
	if s, r := sl.Selected(); s != -1 || r != -1 {
		t.Fatalf("disabled list selected (%d,%d), want (-1,-1)", s, r)
	}
}

func TestSourceListReorderDownWithinSection(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	var from, to = -9, -9
	sl.OnReorder = func(section, f, tgt int) {
		if section != 0 {
			t.Fatalf("reorder section = %d, want 0", section)
		}
		from, to = f, tgt
	}
	th := DefaultLight()

	// Press Alpha (row 0), drag toward Gamma, and confirm the insertion line.
	sl.OnEvent(Event{Kind: EventClick, Y: 40})
	if got := sl.DragData(); got != SourceRowDragPrefix+"0:0" {
		t.Fatalf("DragData = %q, want %q", got, SourceRowDragPrefix+"0:0")
	}
	sl.OnEvent(Event{Kind: EventDragMove, Y: 95})
	if !sl.dragging {
		t.Fatalf("drag not armed after EventDragMove")
	}
	w, h := 180, 400
	buf := makeSurface(w, h)
	sl.Draw(newP(buf, w), th)
	// Insertion line sits at Gamma's top edge (Y=90), 2px thick from Y=89.
	if got := pixelAt(buf, w, 90, 89); got != th.Accent {
		t.Fatalf("insertion line = %+v, want Accent %+v", got, th.Accent)
	}

	sl.OnEvent(Event{Kind: EventDrop, Y: 95})
	if from != 0 || to != 1 {
		t.Fatalf("OnReorder(from,to) = (%d,%d), want (0,1)", from, to)
	}
	names := labelsOf(sl.Sections[0].Items)
	if want := []string{"Beta", "Alpha", "Gamma"}; !equalStrings(names, want) {
		t.Fatalf("order after down-reorder = %v, want %v", names, want)
	}
	if sl.dragging {
		t.Fatalf("drag flag should clear after drop")
	}
	if got := sl.DragData(); got != "" {
		t.Fatalf("DragData after drop = %q, want empty", got)
	}
}

func TestSourceListReorderUpWithNilCallback(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400}) // OnReorder nil
	sl.OnEvent(Event{Kind: EventClick, Y: 100})    // press Gamma (row 2)
	sl.OnEvent(Event{Kind: EventDragMove, Y: 36})
	sl.OnEvent(Event{Kind: EventDrop, Y: 36}) // drop before Alpha
	names := labelsOf(sl.Sections[0].Items)
	if want := []string{"Gamma", "Alpha", "Beta"}; !equalStrings(names, want) {
		t.Fatalf("order after up-reorder = %v, want %v", names, want)
	}
}

func TestSourceListDropFarAwayDoesNotReorder(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.OnEvent(Event{Kind: EventClick, Y: 40}) // press Alpha
	sl.OnEvent(Event{Kind: EventDragMove, Y: 40})
	sl.OnEvent(Event{Kind: EventDrop, Y: 5000}) // way past the band -> no-op
	names := labelsOf(sl.Sections[0].Items)
	if want := []string{"Alpha", "Beta", "Gamma"}; !equalStrings(names, want) {
		t.Fatalf("order should be unchanged, got %v", names)
	}
}

func TestSourceListDragLeaveClears(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.OnEvent(Event{Kind: EventClick, Y: 40})
	sl.OnEvent(Event{Kind: EventDragMove, Y: 60})
	sl.OnEvent(Event{Kind: EventDragLeave})
	if sl.dragging {
		t.Fatalf("EventDragLeave should clear the drag flag")
	}
}

func TestSourceListDragMoveWithoutPressIsNoop(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	sl.OnEvent(Event{Kind: EventDragMove, Y: 60}) // no prior press
	if sl.dragging {
		t.Fatalf("drag armed without a press")
	}
}

func TestSourceListDrawInsertionAtBandEnd(t *testing.T) {
	sl := sampleSourceList()
	th := DefaultLight()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	// Hand-arm a drag whose Y is past every row midpoint but within the band.
	sl.pressedSection, sl.pressedRow = 0, 0
	sl.dragging = true
	sl.dragY = 115 // below Gamma's midpoint, inside its row
	w, h := 180, 400
	buf := makeSurface(w, h)
	sl.Draw(newP(buf, w), th)
	// Line at the band's bottom edge: Gamma bottom = 90+28 = 118, drawn from 117.
	if got := pixelAt(buf, w, 90, 117); got != th.Accent {
		t.Fatalf("end-of-band insertion line = %+v, want Accent %+v", got, th.Accent)
	}
}

func TestSourceListDropTargetEmptyBand(t *testing.T) {
	// A reorderable section with no items: pressing is impossible through the UI,
	// so drive the drop-target math directly to cover the empty-band branch.
	sl := NewSourceList(SourceSection{Title: "Empty", Reorderable: true})
	sl.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	sl.pressedSection, sl.pressedRow = 0, 0
	if _, _, ok := sl.dropTargetIndex(20); ok {
		t.Fatalf("dropTargetIndex over an empty band should be ok=false")
	}
	if _, _, ok := sl.dropTarget(20); ok {
		t.Fatalf("dropTarget over an empty band should be ok=false")
	}
}

func TestSourceListMoveItemOutOfRange(t *testing.T) {
	sl := sampleSourceList()
	sl.SetBounds(Rect{X: 0, Y: 0, W: 180, H: 400})
	before := labelsOf(sl.Sections[0].Items)
	sl.moveItem(0, 99, 0) // from out of range: no-op
	if after := labelsOf(sl.Sections[0].Items); !equalStrings(before, after) {
		t.Fatalf("out-of-range moveItem mutated items: %v", after)
	}
}

func TestSourceListAcceptsDrop(t *testing.T) {
	sl := sampleSourceList()
	if !sl.AcceptsDrop(SourceRowDragPrefix + "0:1") {
		t.Fatalf("own payload should be accepted")
	}
	if sl.AcceptsDrop("listrow:2") {
		t.Fatalf("foreign payload should be rejected")
	}
}

func TestSourceListA11y(t *testing.T) {
	sl := sampleSourceList()
	if got := sl.A11y(); got.Role != RoleNavigation || got.Value != "" {
		t.Fatalf("unselected A11y = %+v, want navigation with empty value", got)
	}
	sl.SetSelected(1, 0) // Home
	if got := sl.A11y(); got.Role != RoleNavigation || got.Value != "Home" {
		t.Fatalf("selected A11y = %+v, want navigation/Home", got)
	}
}

func labelsOf(items []SourceItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ExampleSourceList builds a two-section sidebar, selects a row and reports it.
func ExampleSourceList() {
	sl := NewSourceList(
		SourceSection{Title: "Favourites", Reorderable: true, Items: []SourceItem{
			{Label: "Documents", Key: "docs"},
			{Label: "Downloads", Key: "dl"},
		}},
		SourceSection{Title: "Locations", Items: []SourceItem{
			{Label: "Home", Key: "home"},
		}},
	)
	sl.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	sl.Draw(newP(makeSurface(200, 300), 200), DefaultLight())
	sl.OnEvent(Event{Kind: EventClick, Y: 40}) // select the first favourite
	sec, row := sl.Selected()
	fmt.Printf("selected section %d row %d\n", sec, row)
	// Output: selected section 0 row 0
}
