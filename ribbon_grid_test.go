// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// gridRibbon is a ribbon with one grid control, "Table", whose OnGrid records the
// size it was called with.
func gridRibbon(cols, rows int) (*Ribbon, *RibbonControl, *[][2]int) {
	var got [][2]int
	c := &RibbonControl{Kind: RibbonGrid, Large: true, Action: NewAction("table", "Table", nil), Tip: "Insert a table.",
		GridCols: cols, GridRows: rows, OnGrid: func(c, r int) { got = append(got, [2]int{c, r}) }}
	r := NewRibbon(&RibbonTab{Label: "Insert", Groups: []*RibbonGroup{{Label: "Tables", Priority: 1, Controls: []*RibbonControl{c}}}})
	r.SetBounds(Rect{W: 800, H: r.Height()})
	return r, c, &got
}

func TestRibbonGridOpensAGridOfCells(t *testing.T) {
	r, c, got := gridRibbon(0, 0)
	if cols, rows := c.gridSize(); cols != 8 || rows != 6 {
		t.Fatalf("the default grid is 8 by 6: %d by %d", cols, rows)
	}
	i := rtCell(r, "Table")
	cell := r.cells()[i]
	if !c.hasArrow() || cell.arrow == (Rect{}) {
		t.Fatal("a grid control has an arrow")
	}
	// the icon half and the label half both open it
	rtClick(r, cell.rect.X+cell.rect.W/2, cell.rect.Y+4)
	if r.pop == nil || r.pop.kind != popGrid || r.pop.focus != -1 {
		t.Fatalf("a click should open the grid with no cell lit: %+v", r.pop)
	}
	panel, cells := r.popupGeometry(r.pop, r.metrics())
	if len(cells) != 48 || r.pop.popupLen() != 48 || r.pop.popupCols() != 8 {
		t.Fatalf("%d cells", len(cells))
	}
	if cells[9].X != cells[1].X || cells[9].Y != cells[1].Y+cells[1].H {
		t.Fatalf("cells run in rows of 8: %+v %+v", cells[1], cells[9])
	}
	if panel.W < cells[7].X+cells[7].W-panel.X || panel.Y != cell.rect.Y+cell.rect.H {
		t.Fatalf("the panel hangs under the control and holds every cell: %+v", panel)
	}
	if r.pop.gridCaption() != "Table" {
		t.Errorf("before a cell is lit the caption is the control's name: %q", r.pop.gridCaption())
	}
	// the pointer lights the cells from the corner to the one it is on
	x, y := rtCenter(cells[2*8+3]) // 4 columns, 3 rows
	r.hover(x, y)
	if r.pop.focus != 2*8+3 || r.pop.gridCaption() != "4 × 3" {
		t.Fatalf("hover: focus %d caption %q", r.pop.focus, r.pop.gridCaption())
	}
	rtClick(r, x, y)
	if len(*got) != 1 || (*got)[0] != [2]int{4, 3} || r.pop != nil {
		t.Fatalf("choosing a cell calls OnGrid(4, 3) and closes the grid: %v pop=%v", *got, r.pop)
	}
}

func TestRibbonGridSizeIsKept(t *testing.T) {
	r, c, got := gridRibbon(99, 3)
	if cols, rows := c.gridSize(); cols != 16 || rows != 3 {
		t.Fatalf("a grid is at most 16 wide: %d by %d", cols, rows)
	}
	r.activate(rtCell(r, "Table"), partMain)
	if r.pop.popupLen() != 48 {
		t.Fatalf("16 by 3: %d cells", r.pop.popupLen())
	}
	_, cells := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(cells[47])
	rtClick(r, x, y)
	if (*got)[0] != [2]int{16, 3} {
		t.Fatalf("last cell: %v", *got)
	}
}

func TestRibbonGridKeyboard(t *testing.T) {
	r, _, got := gridRibbon(4, 3)
	r.byKey = true
	r.activate(rtCell(r, "Table"), partMain)
	r.byKey = false
	if r.pop == nil || r.pop.focus != 0 {
		t.Fatalf("opened by the keyboard the corner cell is lit: %+v", r.pop)
	}
	for _, k := range []string{"ArrowLeft", "ArrowUp", "ArrowRight", "ArrowRight", "ArrowDown"} {
		r.key(Event{Kind: EventKeyDown, Code: k})
	}
	if r.pop.focus != 6 || r.pop.gridCaption() != "3 × 2" { // column 3, row 2 of 4 columns
		t.Fatalf("arrows move over the grid: focus %d %q", r.pop.focus, r.pop.gridCaption())
	}
	r.key(Event{Kind: EventKeyDown, Code: "End"})
	if r.pop.gridCaption() != "4 × 3" {
		t.Fatalf("End: %q", r.pop.gridCaption())
	}
	r.key(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // already on the last row
	r.key(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	r.key(Event{Kind: EventKeyDown, Code: "Home"})
	if r.pop.focus != 0 {
		t.Fatalf("Home: %d", r.pop.focus)
	}
	r.key(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	r.key(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	r.key(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*got) != 1 || (*got)[0] != [2]int{2, 2} || r.pop != nil {
		t.Fatalf("Enter chooses the lit cell: %v", *got)
	}
	// a grid opened with the pointer and then driven by keys starts at the corner
	r.activate(rtCell(r, "Table"), partMain)
	r.key(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if r.pop.focus < 0 {
		t.Fatalf("an arrow lights a cell: %d", r.pop.focus)
	}
	r.key(Event{Kind: EventKeyDown, Code: "Escape"})
	if r.pop != nil {
		t.Error("Escape closes the grid")
	}
}

func TestRibbonGridDisabledAndWithoutACallback(t *testing.T) {
	r, c, _ := gridRibbon(3, 3)
	c.Action.SetEnabled(false)
	r.activate(rtCell(r, "Table"), partMain)
	if r.pop != nil {
		t.Error("a disabled grid does not open")
	}
	c.Action.SetEnabled(true)
	c.OnGrid = nil
	r.activate(rtCell(r, "Table"), partMain)
	_, cells := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(cells[4])
	rtClick(r, x, y)
	if r.pop != nil {
		t.Error("choosing a cell closes the grid, callback or not")
	}
	// the open grid is "expanded" for assistive technology
	r.activate(rtCell(r, "Table"), partMain)
	var found bool
	for _, n := range r.Children() {
		if info := n.(Accessible).A11y(); info.Name == "Table" {
			found = info.Role == RoleButton && info.Value == "expanded"
		}
	}
	if !found {
		t.Error("an open grid reports expanded")
	}
	// a caption for a highlight outside the grid falls back to the name
	r.pop.focus = 999
	if r.pop.gridCaption() != "Table" {
		t.Errorf("caption %q", r.pop.gridCaption())
	}
}

func TestRibbonGridIsDrawnInBothThemes(t *testing.T) {
	for _, th := range []*Theme{DefaultLight(), DefaultDark()} {
		r, _, _ := gridRibbon(5, 4)
		r.activate(rtCell(r, "Table"), partMain)
		w, h := 800, 300
		plain := make([]byte, w*h*4)
		r.DrawPopover(painter.NewPixelPainter(plain, w, h), th)
		_, cells := r.popupGeometry(r.pop, r.metrics())
		x, y := rtCenter(cells[1*5+2])
		r.hover(x, y)
		lit := make([]byte, w*h*4)
		r.DrawPopover(painter.NewPixelPainter(lit, w, h), th)
		if string(plain) == string(lit) {
			t.Error("lighting cells changes the picture")
		}
		// the lit cells use the accent, the others the line colour
		in := cells[0]
		px := ((in.Y+in.H/2)*w + in.X + in.W/2) * 4
		if lit[px] == plain[px] && lit[px+1] == plain[px+1] && lit[px+2] == plain[px+2] {
			t.Error("a lit cell is filled")
		}
	}
}
