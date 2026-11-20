package toolkit

import "testing"

// A body shorter than one row still shows (a sliver of) one row at a time, and
// one with no height at all scrolls nowhere.
func TestTableWholeRowsFloorAtOneWhileBodyHasHeight(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, tableManyRows(20))
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: scaled(TableHeaderHeight) + 5})
	if got := tb.maxScrollRow(); got != 19 {
		t.Fatalf("maxScrollRow with a 5px body = %d, want 19", got)
	}
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: scaled(TableHeaderHeight)})
	if got := tb.maxScrollRow(); got != 20 || tb.bodyOverflows() {
		t.Fatalf("no body: maxScrollRow=%d overflows=%v, want 20 and false", got, tb.bodyOverflows())
	}
}

// TestTableLastRowFullyVisibleAtBottom: scrolled to its end, the last row's
// bottom edge must lie within the viewport -- above the horizontal scrollbar
// when one is shown -- whatever the height (a multiple of the row height or
// not) and with the horizontal scrollbar shown or hidden.
func TestTableLastRowFullyVisibleAtBottom(t *testing.T) {
	rh := TouchTarget(scaled(TableRowHeight))
	for _, wide := range []bool{false, true} {
		// from a height where one row still fits above the horizontal scrollbar
		for body := rh + scrollbarWidth; body <= 6*rh+scrollbarWidth+5; body += 3 {
			w := 120
			cols := []TableColumn{{Title: "A", Width: 100}}
			if wide {
				cols = []TableColumn{{Title: "A", Width: 400}}
			}
			const n = 20
			tb := NewTable(cols, tableManyRows(n))
			tb.SetBounds(Rect{X: 0, Y: 0, W: w, H: scaled(TableHeaderHeight) + body})
			tb.ScrollTo(1 << 20)
			viewBottom := tb.Bounds().H
			if tb.hScrollable() {
				viewBottom -= scrollbarWidth
			}
			if wide != tb.hScrollable() {
				t.Fatalf("wide=%v but hScrollable=%v", wide, tb.hScrollable())
			}
			top := scaled(TableHeaderHeight) + (n-1-tb.ScrollRow().Get())*rh
			if bottom := top + rh; bottom > viewBottom {
				t.Fatalf("wide=%v body=%d: last row bottom %d > viewport bottom %d (scroll %d)", wide, body, bottom, viewBottom, tb.ScrollRow().Get())
			}
		}
	}
}
