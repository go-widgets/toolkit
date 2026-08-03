// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// summaryTable builds an ungrouped Table with a numeric column (Sum) and a
// text column (no aggregate), tall enough that every line fits.
func summaryTable() *Table {
	tb := NewTable([]TableColumn{
		{Title: "Item", Width: 60},
		{Title: "Qty", Width: 60, Align: AlignRight, Aggregate: AggregateSum},
	}, [][]string{
		{"a", "1"},
		{"b", "2"},
		{"c", "4"},
	})
	tb.ShowSummary = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	return tb
}

// --- aggregate: every kind ----------------------------------------------

func TestTableAggregateKinds(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "n"}}, [][]string{
		{"2"}, {"4"}, {"x"}, {"1"}, // "x" is skipped by numeric kinds
	})
	cases := []struct {
		agg  TableAggregate
		want string
	}{
		{AggregateNone, ""},
		{AggregateCount, "4"}, // counts rows, including the non-numeric one
		{AggregateSum, "7"},   // 2+4+1, integral -> no decimals
		{AggregateAvg, "2.33"},
		{AggregateMin, "1"},
		{AggregateMax, "4"},
	}
	for _, c := range cases {
		if got := tb.aggregate(c.agg, 0, 0, len(tb.Rows)); got != c.want {
			t.Fatalf("aggregate(%d) = %q, want %q", c.agg, got, c.want)
		}
	}
}

// TestTableAggregateAvgDecimal exercises the fractional formatAggregate branch.
func TestTableAggregateAvgDecimal(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "n"}}, [][]string{{"1"}, {"2"}})
	if got := tb.aggregate(AggregateAvg, 0, 0, 2); got != "1.50" {
		t.Fatalf("avg(1,2) = %q, want 1.50", got)
	}
}

// TestTableAggregateAllNonNumeric: a numeric aggregate over an all-text range
// yields "" (blank), not "0".
func TestTableAggregateAllNonNumeric(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "n"}}, [][]string{{"foo"}, {"bar"}})
	for _, agg := range []TableAggregate{AggregateSum, AggregateAvg, AggregateMin, AggregateMax} {
		if got := tb.aggregate(agg, 0, 0, 2); got != "" {
			t.Fatalf("aggregate(%d) over text = %q, want empty", agg, got)
		}
	}
	// Count still counts the rows regardless of content.
	if got := tb.aggregate(AggregateCount, 0, 0, 2); got != "2" {
		t.Fatalf("count over text = %q, want 2", got)
	}
}

// TestTableAggregateRaggedRow: a row too short to have the column's cell is
// skipped by the numeric reducer (exercises the col >= len(row) guard).
func TestTableAggregateRaggedRow(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "a"}, {Title: "b"}}, [][]string{
		{"x", "10"},
		{"y"}, // no column-1 cell
		{"z", "20"},
	})
	if got := tb.aggregate(AggregateSum, 1, 0, 3); got != "30" {
		t.Fatalf("sum over ragged col = %q, want 30", got)
	}
}

// TestTableAggregateMinMaxOrdering forces both the v<min and v>max updates by
// feeding values that dip below then rise above the first sample.
func TestTableAggregateMinMaxOrdering(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "n"}}, [][]string{
		{"5"}, {"2"}, {"9"}, {"5"}, // first=5, then 2 (<min), then 9 (>max)
	})
	if got := tb.aggregate(AggregateMin, 0, 0, 4); got != "2" {
		t.Fatalf("min = %q, want 2", got)
	}
	if got := tb.aggregate(AggregateMax, 0, 0, 4); got != "9" {
		t.Fatalf("max = %q, want 9", got)
	}
}

// --- line model: ungrouped + summary ------------------------------------

func TestTableSummaryLinesUngrouped(t *testing.T) {
	tb := summaryTable()
	if !tb.useLineModel() {
		t.Fatal("useLineModel must be true with ShowSummary")
	}
	lines := tb.lines()
	// 3 data lines + 1 grand-total footer.
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
	for i := 0; i < 3; i++ {
		if lines[i].summary || lines[i].header || lines[i].dataRow != i {
			t.Fatalf("line %d = %+v, want data row %d", i, lines[i], i)
		}
	}
	f := lines[3]
	if !f.summary || f.sumStart != 0 || f.sumEnd != 3 {
		t.Fatalf("footer line = %+v, want summary [0,3)", f)
	}
	if tb.lineCount() != 4 {
		t.Fatalf("lineCount = %d, want 4", tb.lineCount())
	}
}

// TestTableSummaryEmptyRows: an empty Table emits no summary line (the
// placeholder shows instead).
func TestTableSummaryEmptyRows(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "a", Aggregate: AggregateSum}}, nil)
	tb.ShowSummary = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if got := tb.lines(); len(got) != 0 {
		t.Fatalf("empty+summary lines = %v, want none", got)
	}
	if tb.lineCount() != 0 {
		t.Fatalf("empty lineCount = %d, want 0", tb.lineCount())
	}
	// Draw must not panic and must show the placeholder path.
	buf := makeTableSurface(100, 100)
	tb.Draw(newP(buf, 100), DefaultLight())
}

// TestTableSummaryRowAtAndVisualIndex: a click on the footer resolves to no
// row, data-row clicks still map, and visualIndex skips the summary line.
func TestTableSummaryRowAtAndVisualIndex(t *testing.T) {
	tb := summaryTable()
	yOf := func(vi int) int { return TableHeaderHeight + vi*TableRowHeight + 2 }
	if got := tb.rowAt(yOf(0)); got != 0 {
		t.Fatalf("rowAt(line0) = %d, want 0", got)
	}
	if got := tb.rowAt(yOf(3)); got != -1 { // footer line
		t.Fatalf("rowAt(footer) = %d, want -1", got)
	}
	if got := tb.visualIndex(2); got != 2 {
		t.Fatalf("visualIndex(2) = %d, want 2", got)
	}
	if got := tb.visualIndex(9); got != -1 { // no such row
		t.Fatalf("visualIndex(9) = %d, want -1", got)
	}
}

// --- line model: grouped + summary --------------------------------------

func TestTableSummaryLinesGrouped(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Grp", Width: 50},
		{Title: "Val", Width: 50, Aggregate: AggregateSum},
	}, [][]string{{"g1", "1"}, {"g1", "2"}, {"g2", "4"}})
	tb.GroupBy = 0
	tb.ShowSummary = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	// [hdr g1][r0][r1][sum g1][hdr g2][r2][sum g2][grand] = 8
	lines := tb.lines()
	if len(lines) != 8 {
		t.Fatalf("grouped+summary lines = %d, want 8: %+v", len(lines), lines)
	}
	if !lines[3].summary || lines[3].sumStart != 0 || lines[3].sumEnd != 2 {
		t.Fatalf("g1 summary line = %+v, want [0,2)", lines[3])
	}
	if !lines[6].summary || lines[6].sumStart != 2 || lines[6].sumEnd != 3 {
		t.Fatalf("g2 summary line = %+v, want [2,3)", lines[6])
	}
	if !lines[7].summary || lines[7].sumStart != 0 || lines[7].sumEnd != 3 {
		t.Fatalf("grand total line = %+v, want [0,3)", lines[7])
	}
	// The per-group Sum aggregates only that group's rows.
	if got := tb.aggregate(AggregateSum, 1, lines[3].sumStart, lines[3].sumEnd); got != "3" {
		t.Fatalf("g1 sum = %q, want 3", got)
	}
}

// TestTableSummaryGroupedCollapsedHidesSummary: a collapsed group emits neither
// its rows nor its per-group summary.
func TestTableSummaryGroupedCollapsedHidesSummary(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Grp", Width: 50},
		{Title: "Val", Width: 50, Aggregate: AggregateSum},
	}, [][]string{{"g1", "1"}, {"g1", "2"}, {"g2", "4"}})
	tb.GroupBy = 0
	tb.ShowSummary = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	tb.toggleGroup("g1")
	// [hdr g1 collapsed][hdr g2][r2][sum g2][grand] = 5
	lines := tb.lines()
	if len(lines) != 5 {
		t.Fatalf("collapsed grouped+summary lines = %d, want 5: %+v", len(lines), lines)
	}
	for _, ln := range lines {
		if ln.summary && ln.sumStart == 0 && ln.sumEnd == 2 {
			t.Fatal("collapsed g1 must not emit its per-group summary")
		}
	}
}

// --- Draw: footer band paints -------------------------------------------

func TestTableSummaryDrawFooterBand(t *testing.T) {
	tb := summaryTable()
	theme := DefaultLight()
	buf := makeTableSurface(120, 200)
	tb.Draw(newP(buf, 120), theme)
	// Footer is visual line 3: its band is a SurfaceAlt fill.
	fy := TableHeaderHeight + 3*TableRowHeight + TableRowHeight/2
	// Sample the left cell centre (away from separators) for the band colour.
	if got := pixelAt(buf, 120, 30, fy); got != theme.SurfaceAlt {
		t.Fatalf("footer band pixel = %+v, want SurfaceAlt %+v", got, theme.SurfaceAlt)
	}
	// A data row above must still be Surface (not the footer band).
	dy := TableHeaderHeight + TableRowHeight/2
	if got := pixelAt(buf, 120, 30, dy); got != theme.Surface {
		t.Fatalf("row-0 pixel = %+v, want Surface %+v", got, theme.Surface)
	}
	// The Qty aggregate "7" must paint OnSurface ink somewhere in the footer's
	// right column band.
	found := false
	for y := TableHeaderHeight + 3*TableRowHeight; y < TableHeaderHeight+4*TableRowHeight && !found; y++ {
		for x := 60; x < 120; x++ {
			if pixelAt(buf, 120, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no aggregate ink found in footer band")
	}
}

// TestTableSummaryDrawWithIconGutter exercises the column-0 gutter branch of
// drawSummaryRow (RowIcon reserves a leading gutter that the summary honours).
func TestTableSummaryDrawWithIconGutter(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Item", Width: 80, Aggregate: AggregateCount},
		{Title: "Qty", Width: 60, Aggregate: AggregateSum},
	}, [][]string{{"a", "1"}, {"b", "2"}})
	tb.ShowSummary = true
	tb.RowIcon = func(int) (TableIconFunc, bool) { return DrawIconOpen, true }
	tb.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 200})
	buf := makeTableSurface(140, 200)
	tb.Draw(newP(buf, 140), DefaultLight())
	// Footer (line 2) count "2" must paint ink somewhere.
	theme := DefaultLight()
	found := false
	for y := TableHeaderHeight + 2*TableRowHeight; y < TableHeaderHeight+3*TableRowHeight && !found; y++ {
		for x := 0; x < 80; x++ {
			if pixelAt(buf, 140, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no count ink found in gutter-summary footer")
	}
}

// TestTableSummaryDrawFrozenScroll exercises drawSummaryRow's frozen/scrolled
// geometry: fixed columns wide enough to overflow, with a frozen leading
// column and a horizontal scroll offset.
func TestTableSummaryDrawFrozenScroll(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 80, Aggregate: AggregateCount},
		{Title: "B", Width: 80, Aggregate: AggregateSum},
		{Title: "C", Width: 80, Aggregate: AggregateSum},
	}, [][]string{{"a", "1", "10"}, {"b", "2", "20"}})
	tb.ShowSummary = true
	tb.FrozenColumns = 1
	tb.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 200}) // 240 natural > 140 viewport
	if !tb.hScrollable() {
		t.Fatal("table should be horizontally scrollable")
	}
	tb.ScrollXTo(40)
	buf := makeTableSurface(140, 200)
	tb.Draw(newP(buf, 140), DefaultLight())
	// Footer band (line 2) paints SurfaceAlt across the frozen region.
	theme := DefaultLight()
	fy := TableHeaderHeight + 2*TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 140, 10, fy); got != theme.SurfaceAlt {
		t.Fatalf("frozen footer band = %+v, want SurfaceAlt", got)
	}
}

// TestTableSummaryClickInert: a click on the footer line neither selects a row
// nor toggles anything, even under MultiSelect.
func TestTableSummaryClickInert(t *testing.T) {
	tb := summaryTable()
	tb.MultiSelect = true
	tb.OnEvent(Event{Kind: EventClick, X: 30, Y: TableHeaderHeight + 3*TableRowHeight + 2})
	if len(tb.SelectedRows()) != 0 || tb.Selected != -1 {
		t.Fatalf("footer click changed selection: rows=%v sel=%d", tb.SelectedRows(), tb.Selected)
	}
}
