// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Wave 7 (MVVM bindability): the change callbacks these tests exercise are what
// let a ViewModel observe each widget's state. Every callback must fire on the
// same interaction paths a user already drives, carry the new value, fire only
// when the value actually changes (so a two-way binding stays loop-free), and be
// nil-safe.

// --- Table.OnSelect -----------------------------------------------------------

func TestTableOnSelectFiresOnKeyboardMove(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	var got, calls int
	got = -99
	tb.OnSelect = func(row int) { got, calls = row, calls+1 }

	// ArrowDown from the -1 default lands on row 0 and reports it.
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if tb.Selected != 0 || got != 0 || calls != 1 {
		t.Fatalf("after ArrowDown: Selected=%d got=%d calls=%d, want 0/0/1", tb.Selected, got, calls)
	}
	// End jumps to the last row and reports it.
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if tb.Selected != 2 || got != 2 || calls != 2 {
		t.Fatalf("after End: Selected=%d got=%d calls=%d, want 2/2/2", tb.Selected, got, calls)
	}
	// ArrowDown at the last row is a no-op move -- Selected is unchanged, so
	// OnSelect must NOT fire again (the loop-free guard).
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if tb.Selected != 2 || calls != 2 {
		t.Fatalf("no-op move fired OnSelect: Selected=%d calls=%d, want 2/2", tb.Selected, calls)
	}
}

func TestTableOnSelectFiresOnMultiSelectClick(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}},
		[][]string{{"r0"}, {"r1"}, {"r2"}})
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	var got, calls int
	tb.OnSelect = func(row int) { got, calls = row, calls+1 }

	// A plain body-row click moves the anchor (Selected) to row 1 and reports it.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + TableRowHeight + 1})
	if tb.Selected != 1 || got != 1 || calls != 1 {
		t.Fatalf("after click row1: Selected=%d got=%d calls=%d, want 1/1/1", tb.Selected, got, calls)
	}
	// Re-clicking the same row does not change Selected, so OnSelect stays silent.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + TableRowHeight + 1})
	if calls != 1 {
		t.Fatalf("re-click same row fired OnSelect: calls=%d, want 1", calls)
	}
}

func TestTableOnSelectNilSafe(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A", Width: 100}}, [][]string{{"r0"}, {"r1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	// No OnSelect assigned: the keyboard move must not panic.
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if tb.Selected != 0 {
		t.Fatalf("nil-callback move: Selected=%d, want 0", tb.Selected)
	}
}

// --- Accordion.OnToggle -------------------------------------------------------

func TestAccordionOnToggleFiresExclusive(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "One"}, {Title: "Two"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	var gotI int
	var gotExp, called bool
	a.OnToggle = func(i int, expanded bool) { gotI, gotExp, called = i, expanded, true }

	// Click header 0 -> expands it, reporting (0, true).
	a.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !called || gotI != 0 || !gotExp {
		t.Fatalf("expand: called=%v i=%d exp=%v, want true/0/true", called, gotI, gotExp)
	}
	// Click header 0 again -> collapses it, reporting (0, false).
	a.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if gotI != 0 || gotExp {
		t.Fatalf("collapse: i=%d exp=%v, want 0/false", gotI, gotExp)
	}
}

func TestAccordionOnToggleFiresMultipleAndKeyboard(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "One"}, {Title: "Two"}})
	a.Multiple = true
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	var gotI int
	var gotExp bool
	calls := 0
	a.OnToggle = func(i int, expanded bool) { gotI, gotExp, calls = i, expanded, calls+1 }

	// Keyboard: focus starts on section 0; Enter toggles it open in Multiple mode.
	a.SetFocused(true)
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if gotI != 0 || !gotExp || calls != 1 {
		t.Fatalf("keyboard toggle: i=%d exp=%v calls=%d, want 0/true/1", gotI, gotExp, calls)
	}
	// Move focus down and toggle section 1 independently (Multiple mode).
	a.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	a.OnEvent(Event{Kind: EventKeyDown, Code: " "})
	if gotI != 1 || !gotExp || calls != 2 {
		t.Fatalf("second toggle: i=%d exp=%v calls=%d, want 1/true/2", gotI, gotExp, calls)
	}
}

// --- Carousel.OnChange --------------------------------------------------------

func TestCarouselOnChangeFiresOnStepAndDot(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	var got, calls int
	c.OnChange = func(i int) { got, calls = i, calls+1 }

	c.Next() // 0 -> 1
	if c.Current != 1 || got != 1 || calls != 1 {
		t.Fatalf("Next: Current=%d got=%d calls=%d, want 1/1/1", c.Current, got, calls)
	}
	c.Prev() // 1 -> 0
	if c.Current != 0 || got != 0 || calls != 2 {
		t.Fatalf("Prev: Current=%d got=%d calls=%d, want 0/0/2", c.Current, got, calls)
	}
	// Click the dot for slide 2 (dots strip is at the bottom of the bounds).
	dr := c.dotRect(2)
	c.OnEvent(Event{Kind: EventClick, X: dr.X + dr.W/2, Y: dr.Y + dr.H/2})
	if c.Current != 2 || got != 2 || calls != 3 {
		t.Fatalf("dot click: Current=%d got=%d calls=%d, want 2/2/3", c.Current, got, calls)
	}
	// Clicking the already-active dot is a no-op move: OnChange stays silent.
	dr2 := c.dotRect(2)
	c.OnEvent(Event{Kind: EventClick, X: dr2.X + dr2.W/2, Y: dr2.Y + dr2.H/2})
	if calls != 3 {
		t.Fatalf("active-dot click fired OnChange: calls=%d, want 3", calls)
	}
}

func TestCarouselOnChangeClampNoFireAndNilSafe(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b")})
	c.Current = 1 // at the last slide, Wrap is false
	calls := 0
	c.OnChange = func(int) { calls++ }
	c.Next() // clamped: no move, no fire
	if c.Current != 1 || calls != 0 {
		t.Fatalf("clamped Next: Current=%d calls=%d, want 1/0", c.Current, calls)
	}
	// Nil callback: a real move must not panic.
	c.OnChange = nil
	c.Prev()
	if c.Current != 0 {
		t.Fatalf("nil-callback Prev: Current=%d, want 0", c.Current)
	}
}

// --- CycleButton.OnChangeIndex ------------------------------------------------

func TestCycleButtonOnChangeIndexFires(t *testing.T) {
	c := NewCycleButton("List", "Grid", "Compact")
	var got, calls int
	got = -1
	c.OnChangeIndex = func(i int) { got, calls = i, calls+1 }
	c.OnEvent(Event{Kind: EventClick}) // List -> Grid
	if c.Index != 1 || got != 1 || calls != 1 {
		t.Fatalf("after click: Index=%d got=%d calls=%d, want 1/1/1", c.Index, got, calls)
	}
	// Both slots fire together; ArrowLeft steps back and reports index 0.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if got != 0 || calls != 2 {
		t.Fatalf("after ArrowLeft: got=%d calls=%d, want 0/2", got, calls)
	}
}

// --- RadioGroup.OnChange ------------------------------------------------------

func TestRadioGroupOnChangeFiresOnClickAndKey(t *testing.T) {
	g := NewRadioGroup()
	r0, r1 := NewRadioButton("a"), NewRadioButton("b")
	g.Add(r0)
	g.Add(r1)
	var got, calls int
	got = -1
	g.OnChange = func(active int) { got, calls = active, calls+1 }

	// Click member 1 -> group Active becomes 1.
	r1.OnEvent(Event{Kind: EventClick})
	if g.Active != 1 || got != 1 || calls != 1 {
		t.Fatalf("click member1: Active=%d got=%d calls=%d, want 1/1/1", g.Active, got, calls)
	}
	// ArrowDown from member 1 wraps the checked member to member 0.
	r1.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if g.Active != 0 || got != 0 || calls != 2 {
		t.Fatalf("ArrowDown: Active=%d got=%d calls=%d, want 0/0/2", g.Active, got, calls)
	}
}

// --- Menu.OnItemToggle --------------------------------------------------------

func TestMenuOnItemToggleFiresForCheckAndRadio(t *testing.T) {
	noop := func() {}
	m := &Menu{Items: []MenuItem{
		{Label: "Wrap", Checkable: true, Action: noop},
		{Label: "Left", RadioGroup: 1, Action: noop},
		{Label: "Right", RadioGroup: 1, Action: noop},
	}}
	var gotI int
	var gotChecked, called bool
	m.OnItemToggle = func(i int, checked bool) { gotI, gotChecked, called = i, checked, true }

	// Activate the checkable row: it flips to checked and reports (0, true).
	m.activate(0)
	if !called || gotI != 0 || !gotChecked {
		t.Fatalf("checkable: called=%v i=%d checked=%v, want true/0/true", called, gotI, gotChecked)
	}
	// Activating it again flips it back and reports (0, false).
	m.activate(0)
	if gotI != 0 || gotChecked {
		t.Fatalf("checkable off: i=%d checked=%v, want 0/false", gotI, gotChecked)
	}
	// Activate a radio member: the just-selected member reports checked=true.
	m.activate(2)
	if gotI != 2 || !gotChecked {
		t.Fatalf("radio: i=%d checked=%v, want 2/true", gotI, gotChecked)
	}
}
