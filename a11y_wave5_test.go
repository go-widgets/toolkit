// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Button state ---------------------------------------------------

func TestA11yButtonState(t *testing.T) {
	resting := NewButton("OK", nil)
	if got := resting.A11y(); got != (A11yInfo{Role: RoleButton, Name: "OK"}) {
		t.Errorf("resting Button A11y() = %+v", got)
	}

	selected := NewButton("Bold", nil)
	selected.Selected().Set(true)
	if got := selected.A11y(); got != (A11yInfo{Role: RoleButton, Name: "Bold", Value: "selected"}) {
		t.Errorf("selected Button A11y() = %+v", got)
	}

	pressed := NewButton("Hold", nil)
	pressed.pressed = true
	if got := pressed.A11y(); got != (A11yInfo{Role: RoleButton, Name: "Hold", Value: "pressed"}) {
		t.Errorf("pressed Button A11y() = %+v", got)
	}

	// Selected wins over a concurrent transient press.
	both := NewButton("Tab", nil)
	both.Selected().Set(true)
	both.pressed = true
	if got := both.A11y().Value; got != "selected" {
		t.Errorf("selected+pressed Button Value = %q, want selected", got)
	}
}

// --- Range-valued controls (Min/Max/Now triple) --------------------------

func TestA11yRangeControls(t *testing.T) {
	scale := NewScale(0, 100, 42)
	if got := scale.A11y(); got != (A11yInfo{Role: RoleSlider, Value: "42", HasRange: true, Min: 0, Max: 100, Now: 42}) {
		t.Errorf("Scale A11y() = %+v", got)
	}

	rs := NewRangeSlider(0, 100, 20, 80)
	if got := rs.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "20..80", HasRange: true, Min: 0, Max: 100, Now: 20}) {
		t.Errorf("RangeSlider A11y() = %+v", got)
	}

	rating := NewRating(3, 5)
	if got := rating.A11y(); got != (A11yInfo{Role: RoleSlider, Value: "3/5", HasRange: true, Min: 0, Max: 5, Now: 3}) {
		t.Errorf("Rating A11y() = %+v", got)
	}

	spin := NewSpinButton(-2, 10, 4, 1)
	if got := spin.A11y(); got != (A11yInfo{Role: RoleSpinbutton, Value: "4", HasRange: true, Min: -2, Max: 10, Now: 4}) {
		t.Errorf("SpinButton A11y() = %+v", got)
	}

	pb := NewProgressBar()
	pb.SetFraction(0.42)
	if got := pb.A11y(); got != (A11yInfo{Role: RoleProgressbar, Value: "42%", HasRange: true, Min: 0, Max: 1, Now: 0.42}) {
		t.Errorf("ProgressBar A11y() = %+v", got)
	}

	lvl := NewLevelBar(10)
	lvl.Value().Set(4)
	if got := lvl.A11y(); got != (A11yInfo{Role: RoleMeter, Value: "4/10", HasRange: true, Min: 0, Max: 10, Now: 4}) {
		t.Errorf("LevelBar A11y() = %+v", got)
	}

	gauge := NewGauge(0, 200, 150)
	if got := gauge.A11y(); got != (A11yInfo{Role: RoleMeter, Value: "150", HasRange: true, Min: 0, Max: 200, Now: 150}) {
		t.Errorf("Gauge A11y() = %+v", got)
	}
}

// --- New interactive widgets --------------------------------------------

func TestA11yComboBoxTagFieldTimePicker(t *testing.T) {
	cb := NewComboBox([]string{"apple", "apricot"})
	cb.Text().Set("ap")
	if got := cb.A11y(); got != (A11yInfo{Role: RoleCombobox, Name: "ap"}) {
		t.Errorf("ComboBox A11y() = %+v", got)
	}

	tf := NewTagField("go", "rust")
	if got := tf.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "go, rust"}) {
		t.Errorf("TagField A11y() = %+v", got)
	}

	tp := NewTimePicker(9, 5)
	if got := tp.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "09:05"}) {
		t.Errorf("TimePicker A11y() = %+v", got)
	}
}

func TestA11yCycleButton(t *testing.T) {
	cb := NewCycleButton("Low", "Medium", "High")
	cb.Index().Set(2)
	if got := cb.A11y(); got != (A11yInfo{Role: RoleButton, Name: "High"}) {
		t.Errorf("CycleButton in-range A11y() = %+v", got)
	}

	empty := NewCycleButton()
	if got := empty.A11y(); got != (A11yInfo{Role: RoleButton, Name: ""}) {
		t.Errorf("CycleButton empty A11y() = %+v", got)
	}
}

func TestA11yTreeTable(t *testing.T) {
	root := &TreeTableNode{Cells: []string{"Root", "1"}}
	child := &TreeTableNode{Cells: []string{"Child", "2"}}
	root.Children = []*TreeTableNode{child}
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}, {Title: "N"}}, []*TreeTableNode{root})

	tt.Selected().Set(child)
	if got := tt.A11y(); got != (A11yInfo{Role: RoleTree, Value: "Child"}) {
		t.Errorf("TreeTable selected A11y() = %+v", got)
	}

	tt.Selected().Set(nil)
	if got := tt.A11y(); got != (A11yInfo{Role: RoleTree, Value: ""}) {
		t.Errorf("TreeTable no-selection A11y() = %+v", got)
	}

	// A selected node with no cells yields an empty value.
	tt.Selected().Set(&TreeTableNode{})
	if got := tt.A11y(); got != (A11yInfo{Role: RoleTree, Value: ""}) {
		t.Errorf("TreeTable empty-cells A11y() = %+v", got)
	}
}

func TestA11yKanban(t *testing.T) {
	cols := []KanbanColumn{
		{Title: "Todo", Cards: []KanbanCard{{Title: "Write tests"}, {Title: "Ship it"}}},
		{Title: "Done"},
	}
	k := NewKanban(cols)

	// Seeded with -1/-1: no selection.
	if got := k.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("Kanban no-selection A11y() = %+v", got)
	}

	k.SelectedCol().Set(0)
	k.SelectedCard().Set(1)
	if got := k.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "Ship it"}) {
		t.Errorf("Kanban selected A11y() = %+v", got)
	}

	// Out-of-range card index collapses to no highlight.
	k.SelectedCol().Set(0)
	k.SelectedCard().Set(9)
	if got := k.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("Kanban out-of-range A11y() = %+v", got)
	}
}

func TestA11yPropertyGrid(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("Name", "widget")
	pg.Add("Size", "88")

	// No row selected yet (Table seeds Selected = -1).
	if got := pg.A11y(); got != (A11yInfo{Role: RoleGrid, Value: ""}) {
		t.Errorf("PropertyGrid no-selection A11y() = %+v", got)
	}

	pg.Table().Selected().Set(1)
	if got := pg.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "Size"}) {
		t.Errorf("PropertyGrid selected A11y() = %+v", got)
	}

	// A zero-value grid (nil backing table) is still safe.
	if got := (&PropertyGrid{}).A11y(); got != (A11yInfo{Role: RoleGrid, Value: ""}) {
		t.Errorf("PropertyGrid nil-table A11y() = %+v", got)
	}
}

func TestA11yMarkdownEditor(t *testing.T) {
	me := NewMarkdownEditor("# Title\nbody")
	if got := me.A11y(); got != (A11yInfo{Role: RoleTextbox, Value: "# Title\nbody"}) {
		t.Errorf("MarkdownEditor A11y() = %+v", got)
	}

	// A nil source pane yields an empty value.
	if got := (&MarkdownEditor{}).A11y(); got != (A11yInfo{Role: RoleTextbox, Value: ""}) {
		t.Errorf("MarkdownEditor nil-source A11y() = %+v", got)
	}
}

func TestA11yTerminalView(t *testing.T) {
	term := NewTerminalView(3, 2)
	term.Put(0, 0, 'h')
	term.Put(1, 0, 'i')
	// Row 0 = "hi" (trailing blank trimmed); row 1 blank and dropped.
	if got := term.A11y(); got != (A11yInfo{Role: RoleTextbox, Value: "hi"}) {
		t.Errorf("TerminalView A11y() = %+v", got)
	}

	// A wholly-blank sized grid trims to the empty string.
	blank := NewTerminalView(2, 2)
	if got := blank.A11y(); got != (A11yInfo{Role: RoleTextbox, Value: ""}) {
		t.Errorf("TerminalView blank A11y() = %+v", got)
	}

	// A zero-value / under-allocated grid is safe and yields "".
	if got := (&TerminalView{}).A11y(); got != (A11yInfo{Role: RoleTextbox, Value: ""}) {
		t.Errorf("TerminalView zero-value A11y() = %+v", got)
	}
}

func TestA11yAgenda(t *testing.T) {
	// Explicit Year/Month resolves the focus directly.
	a := NewAgenda(nil)
	a.Year, a.Month = 2026, 8
	if got := a.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "2026-08"}) {
		t.Errorf("Agenda explicit-focus A11y() = %+v", got)
	}

	// Focus derived from the first dated event.
	fromEvents := NewAgenda([]AgendaEvent{{Y: 2025, M: 12, Title: "Party"}})
	if got := fromEvents.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "2025-12"}) {
		t.Errorf("Agenda event-focus A11y() = %+v", got)
	}

	// No fields, no dated events: empty value.
	empty := NewAgenda(nil)
	if got := empty.A11y(); got != (A11yInfo{Role: RoleGrid, Value: ""}) {
		t.Errorf("Agenda empty A11y() = %+v", got)
	}
}

// --- Charts ---------------------------------------------------

func TestA11yWave5Charts(t *testing.T) {
	checkA11yCases(t, []a11yCase{
		{"areachart", NewAreaChart([][]float64{{1, 2}, {3, 4}}), A11yInfo{Role: RoleImg, Value: "2 series"}},
		{"scatterchart", &ScatterChart{Series: [][]ScatterPoint{{{X: 1, Y: 2}}}}, A11yInfo{Role: RoleImg, Value: "1 series"}},
		{"radarchart", &RadarChart{Axes: []string{"a", "b", "c"}}, A11yInfo{Role: RoleImg, Value: "3 axes"}},
		{"sparkline", &Sparkline{Values: []float64{1, 2, 3, 4, 5}}, A11yInfo{Role: RoleImg, Value: "5 points"}},
	})
}

// --- Collection sanity ---------------------------------------------------

// TestA11yWave5CollectA11y checks CollectA11y surfaces a Wave-5 widget.
func TestA11yWave5CollectA11y(t *testing.T) {
	widgets := []Widget{
		NewComboBox([]string{"x"}),
		&HBox{}, // pure layout container: skipped
		NewGauge(0, 10, 5),
	}
	got := CollectA11y(widgets)
	if len(got) != 2 {
		t.Fatalf("collected %d, want 2 (HBox skipped)", len(got))
	}
	if got[0].Role != RoleCombobox || got[1].Role != RoleMeter {
		t.Errorf("collected = %+v", got)
	}
}
