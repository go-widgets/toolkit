// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Wave 3 gives the focus-managed form + navigation controls keyboard actions
// that reuse the exact mutate+callback paths a click already drives. Each test
// asserts three things per the Wave-3 contract: the key produces the same state
// change (and fires the same callback) as the equivalent click, the action
// clamps at its bounds, and a Disabled widget ignores the key.

// kd builds an EventKeyDown carrying code.
func kd(code string) Event { return Event{Kind: EventKeyDown, Code: code} }

// --- Button / IconButton / SplitButton -----------------------------------

func TestButtonKeyActivates(t *testing.T) {
	clicks := 0
	b := NewButton("OK", func() { clicks++ })
	b.OnEvent(kd("Enter"))
	b.OnEvent(kd(" "))
	b.OnEvent(kd("Space"))
	if clicks != 3 {
		t.Fatalf("Enter/Space activations = %d, want 3", clicks)
	}
	// Disabled ignores the key.
	b.Disabled = true
	b.OnEvent(kd("Enter"))
	if clicks != 3 {
		t.Fatalf("disabled button activated (clicks=%d)", clicks)
	}
	// Nil handler is safe.
	NewButton("x", nil).OnEvent(kd("Enter"))
}

func TestIconButtonKeyActivates(t *testing.T) {
	fired := 0
	ib := NewIconButton("+", func() { fired++ })
	ib.OnEvent(kd("Enter"))
	ib.OnEvent(kd(" "))
	if fired != 2 {
		t.Fatalf("activations = %d, want 2", fired)
	}
	ib.Disabled = true
	ib.OnEvent(kd(" "))
	if fired != 2 {
		t.Fatalf("disabled icon button activated (fired=%d)", fired)
	}
	// Nil handler is safe (covers activate's nil branch via the key path).
	NewIconButton("x", nil).OnEvent(kd("Enter"))
}

func TestSplitButtonKeyActivates(t *testing.T) {
	primary, arrow := 0, 0
	s := NewSplitButton("X", func() { primary++ })
	s.OnArrow = func() { arrow++ }
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(kd("Enter"))     // primary
	s.OnEvent(kd(" "))         // primary
	s.OnEvent(kd("ArrowDown")) // arrow/menu
	if primary != 2 || arrow != 1 {
		t.Fatalf("primary=%d arrow=%d, want 2/1", primary, arrow)
	}
	// ArrowDown with no arrow slot does nothing.
	s.Arrow = false
	s.OnEvent(kd("ArrowDown"))
	if arrow != 1 {
		t.Fatalf("arrow fired with Arrow=false (arrow=%d)", arrow)
	}
	// Disabled ignores keys.
	s.Arrow = true
	s.Disabled = true
	s.OnEvent(kd("Enter"))
	s.OnEvent(kd("ArrowDown"))
	if primary != 2 || arrow != 1 {
		t.Fatalf("disabled split button activated (primary=%d arrow=%d)", primary, arrow)
	}
	// Nil handlers are safe.
	n := NewSplitButton("y", nil)
	n.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	n.OnEvent(kd("Enter"))
	n.OnEvent(kd("ArrowDown"))
}

// --- CheckButton / Switch / ToggleButton ---------------------------------

func TestCheckButtonKeyToggles(t *testing.T) {
	toggles := 0
	last := false
	c := NewCheckButton("OK", false)
	c.Checked().Subscribe(func(v bool) { toggles++; last = v })
	c.OnEvent(kd(" "))
	if !c.Checked().Get() || toggles != 1 || !last {
		t.Fatalf("space: Checked=%v toggles=%d last=%v", c.Checked().Get(), toggles, last)
	}
	c.OnEvent(kd("Enter"))
	if c.Checked().Get() || toggles != 2 || last {
		t.Fatalf("enter: Checked=%v toggles=%d last=%v", c.Checked().Get(), toggles, last)
	}
	c.Disabled = true
	c.OnEvent(kd(" "))
	if c.Checked().Get() || toggles != 2 {
		t.Fatalf("disabled check toggled (Checked=%v toggles=%d)", c.Checked().Get(), toggles)
	}
	NewCheckButton("nil", false).OnEvent(kd(" ")) // no subscriber: toggle is safe
}

func TestSwitchKeyToggles(t *testing.T) {
	toggles := 0
	s := NewSwitch(false)
	s.On().Subscribe(func(on bool) { toggles++ })
	s.OnEvent(kd("Enter"))
	if !s.On().Get() || toggles != 1 {
		t.Fatalf("enter: On=%v toggles=%d", s.On().Get(), toggles)
	}
	s.OnEvent(kd(" "))
	if s.On().Get() || toggles != 2 {
		t.Fatalf("space: On=%v toggles=%d", s.On().Get(), toggles)
	}
	s.Disabled = true
	s.OnEvent(kd("Enter"))
	if s.On().Get() || toggles != 2 {
		t.Fatalf("disabled switch toggled (On=%v)", s.On().Get())
	}
	NewSwitch(false).OnEvent(kd(" ")) // no subscriber is safe
}

func TestToggleButtonKeyToggles(t *testing.T) {
	toggles := 0
	tb := NewToggleButton("X", false)
	tb.Pressed().Subscribe(func(p bool) { toggles++ })
	tb.OnEvent(kd(" "))
	if !tb.Pressed().Get() || toggles != 1 {
		t.Fatalf("space: Pressed=%v toggles=%d", tb.Pressed().Get(), toggles)
	}
	tb.OnEvent(kd("Enter"))
	if tb.Pressed().Get() || toggles != 2 {
		t.Fatalf("enter: Pressed=%v toggles=%d", tb.Pressed().Get(), toggles)
	}
	tb.Disabled = true
	tb.OnEvent(kd(" "))
	if tb.Pressed().Get() || toggles != 2 {
		t.Fatalf("disabled toggle activated (Pressed=%v)", tb.Pressed().Get())
	}
	NewToggleButton("nil", false).OnEvent(kd("Enter")) // no subscriber safe
}

// --- RadioButton / RadioGroup --------------------------------------------

func TestRadioGroupArrowMovesChecked(t *testing.T) {
	g := NewRadioGroup()
	var members []*RadioButton
	for i := 0; i < 3; i++ {
		r := NewRadioButton("R")
		g.Add(r)
		members = append(members, r)
	}
	g.activate(0)
	members[0].SetFocused(true)
	// ArrowDown moves the checked member from 0 to 1 and follows focus.
	members[0].OnEvent(kd("ArrowDown"))
	if g.Active().Get() != 1 || !members[1].Checked().Get() || members[0].Checked().Get() {
		t.Fatalf("after ArrowDown: Active=%d checked=%v/%v", g.Active().Get(), members[0].Checked().Get(), members[1].Checked().Get())
	}
	if !members[1].Focused() || members[0].Focused() {
		t.Fatalf("focus did not follow to member 1 (0=%v 1=%v)", members[0].Focused(), members[1].Focused())
	}
	// ArrowUp from member 1 wraps? No -- goes to 0.
	members[1].OnEvent(kd("ArrowLeft"))
	if g.Active().Get() != 0 || !members[0].Checked().Get() {
		t.Fatalf("after ArrowLeft: Active=%d", g.Active().Get())
	}
	// ArrowUp from member 0 wraps to the last member (2).
	members[0].OnEvent(kd("ArrowUp"))
	if g.Active().Get() != 2 || !members[2].Checked().Get() {
		t.Fatalf("wrap ArrowUp: Active=%d", g.Active().Get())
	}
	// ArrowRight from 2 wraps to 0.
	members[2].OnEvent(kd("ArrowRight"))
	if g.Active().Get() != 0 {
		t.Fatalf("wrap ArrowRight: Active=%d", g.Active().Get())
	}
	// Disabled member ignores arrows.
	members[0].Disabled = true
	members[0].OnEvent(kd("ArrowDown"))
	if g.Active().Get() != 0 {
		t.Fatalf("disabled radio moved selection (Active=%d)", g.Active().Get())
	}
}

func TestRadioStandaloneKeyToggles(t *testing.T) {
	toggles := 0
	r := NewRadioButton("A")
	r.Checked().Subscribe(func(bool) { toggles++ })
	r.OnEvent(kd(" "))
	if !r.Checked().Get() || toggles != 1 {
		t.Fatalf("space: Checked=%v toggles=%d", r.Checked().Get(), toggles)
	}
	r.OnEvent(kd("Enter"))
	if r.Checked().Get() || toggles != 2 {
		t.Fatalf("enter: Checked=%v toggles=%d", r.Checked().Get(), toggles)
	}
	NewRadioButton("nil").OnEvent(kd(" ")) // no subscribers bound; safe
}

func TestRadioGroupMoveCheckedEmptyMembers(t *testing.T) {
	// Defensive n==0 guard: a radio whose group has no members must not panic.
	g := NewRadioGroup()
	r := &RadioButton{group: g}
	r.OnEvent(kd("ArrowDown"))
	if g.Active().Get() != -1 {
		t.Fatalf("empty group moved (Active=%d)", g.Active().Get())
	}
}

// --- CycleButton ---------------------------------------------------------

func TestCycleButtonKeySteps(t *testing.T) {
	c := NewCycleButton("List", "Grid", "Compact")
	var idx int
	c.Index().Subscribe(func(i int) { idx = i })
	c.OnEvent(kd("ArrowRight")) // 0 -> 1
	c.OnEvent(kd(" "))          // 1 -> 2
	c.OnEvent(kd("Enter"))      // 2 -> 0 (wrap)
	if c.Index().Get() != 0 || idx != 0 {
		t.Fatalf("advance wrap: Index=%d sub=%d", c.Index().Get(), idx)
	}
	c.OnEvent(kd("ArrowLeft")) // 0 -> 2 (wrap back)
	if c.Index().Get() != 2 || idx != 2 {
		t.Fatalf("ArrowLeft wrap: Index=%d sub=%d", c.Index().Get(), idx)
	}
	c.Disabled = true
	c.OnEvent(kd("ArrowRight"))
	if c.Index().Get() != 2 {
		t.Fatalf("disabled cycle advanced (Index=%d)", c.Index().Get())
	}
	NewCycleButton("a", "b").OnEvent(kd("ArrowRight")) // no subscriber safe
}

// --- Scale ---------------------------------------------------------------

func TestScaleKeyMovesValue(t *testing.T) {
	var got float64
	s := NewScale(0, 100, 50)
	s.Value().Subscribe(func(v float64) { got = v })
	s.OnEvent(kd("ArrowRight")) // +1% of range = +1
	if s.Value().Get() != 51 || got != 51 {
		t.Fatalf("ArrowRight: Value=%v cb=%v", s.Value().Get(), got)
	}
	s.OnEvent(kd("ArrowLeft")) // -1
	if s.Value().Get() != 50 {
		t.Fatalf("ArrowLeft: Value=%v", s.Value().Get())
	}
	s.OnEvent(kd("PageUp")) // +10% = +10
	if s.Value().Get() != 60 {
		t.Fatalf("PageUp: Value=%v", s.Value().Get())
	}
	s.OnEvent(kd("PageDown")) // -10
	if s.Value().Get() != 50 {
		t.Fatalf("PageDown: Value=%v", s.Value().Get())
	}
	s.OnEvent(kd("End"))
	if s.Value().Get() != 100 {
		t.Fatalf("End: Value=%v", s.Value().Get())
	}
	s.OnEvent(kd("ArrowRight")) // clamp at Max
	if s.Value().Get() != 100 {
		t.Fatalf("clamp at Max: Value=%v", s.Value().Get())
	}
	s.OnEvent(kd("Home"))
	if s.Value().Get() != 0 {
		t.Fatalf("Home: Value=%v", s.Value().Get())
	}
	// Explicit Step overrides the 1% default.
	st := NewScale(0, 100, 50)
	st.Step = 5
	st.OnEvent(kd("ArrowRight"))
	if st.Value().Get() != 55 {
		t.Fatalf("Step=5 ArrowRight: Value=%v", st.Value().Get())
	}
	// Degenerate range: keys are a no-op.
	deg := NewScale(5, 5, 5)
	deg.OnEvent(kd("ArrowRight"))
	if deg.Value().Get() != 5 {
		t.Fatalf("degenerate range moved (Value=%v)", deg.Value().Get())
	}
	// Disabled ignores keys.
	s.Disabled = true
	s.OnEvent(kd("End"))
	if s.Value().Get() != 0 {
		t.Fatalf("disabled scale moved (Value=%v)", s.Value().Get())
	}
	// No subscriber is safe.
	NewScale(0, 10, 5).OnEvent(kd("Home"))
	// A non-click, non-keydown event is ignored.
	NewScale(0, 10, 5).OnEvent(Event{Kind: EventChar, Code: "a"})
}

// --- RangeSlider ---------------------------------------------------------

func TestRangeSliderKeyMovesHandles(t *testing.T) {
	var lo, hi float64
	s := NewRangeSlider(0, 100, 20, 80)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 20})
	s.Low().Subscribe(func(v float64) { lo = v })
	s.High().Subscribe(func(v float64) { hi = v })
	// Default focus is the low handle; ArrowRight nudges it +1% (=+1).
	s.OnEvent(kd("ArrowRight"))
	if s.Low().Get() != 21 || lo != 21 {
		t.Fatalf("low ArrowRight: Low=%v sub=%v", s.Low().Get(), lo)
	}
	s.OnEvent(kd("ArrowLeft"))
	if s.Low().Get() != 20 {
		t.Fatalf("low ArrowLeft: Low=%v", s.Low().Get())
	}
	// End selects the high handle; ArrowRight nudges High.
	s.OnEvent(kd("End"))
	s.OnEvent(kd("ArrowUp")) // +1
	if s.High().Get() != 81 || hi != 81 {
		t.Fatalf("high ArrowUp: High=%v sub=%v", s.High().Get(), hi)
	}
	// Home reselects the low handle.
	s.OnEvent(kd("Home"))
	// Cross-clamp: a big Step can't push Low past High.
	s.Step = 1000
	s.OnEvent(kd("ArrowRight"))
	if s.Low().Get() != s.High().Get() {
		t.Fatalf("low crossed high: Low=%v High=%v", s.Low().Get(), s.High().Get())
	}
	// Symmetric cross-clamp on the high handle.
	s2 := NewRangeSlider(0, 100, 40, 60)
	s2.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 20})
	s2.Step = 1000
	s2.OnEvent(kd("End"))       // focus high
	s2.OnEvent(kd("ArrowDown")) // -1000, clamped to Low
	if s2.High().Get() != s2.Low().Get() {
		t.Fatalf("high crossed low: Low=%v High=%v", s2.Low().Get(), s2.High().Get())
	}
	// Disabled ignores keys.
	s2.Disabled = true
	before := s2.Low().Get()
	s2.OnEvent(kd("ArrowRight"))
	if s2.Low().Get() != before {
		t.Fatalf("disabled range slider moved (Low=%v)", s2.Low().Get())
	}
	// No subscriber is safe (covers nudgeHandle's Set with no observers).
	sn := NewRangeSlider(0, 10, 2, 8)
	sn.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 20})
	sn.OnEvent(kd("ArrowRight"))
}

// --- SpinButton ----------------------------------------------------------

func TestSpinButtonKeySteps(t *testing.T) {
	var got int
	s := NewSpinButton(0, 100, 50, 2)
	s.Value().Subscribe(func(v int) { got = v })
	s.OnEvent(kd("ArrowUp"))
	if s.Value().Get() != 52 || got != 52 {
		t.Fatalf("ArrowUp: Value=%d cb=%d", s.Value().Get(), got)
	}
	s.OnEvent(kd("ArrowDown"))
	if s.Value().Get() != 50 {
		t.Fatalf("ArrowDown: Value=%d", s.Value().Get())
	}
	s.OnEvent(kd("PageUp")) // +10*Step = +20
	if s.Value().Get() != 70 {
		t.Fatalf("PageUp: Value=%d", s.Value().Get())
	}
	s.OnEvent(kd("PageDown")) // -20
	if s.Value().Get() != 50 {
		t.Fatalf("PageDown: Value=%d", s.Value().Get())
	}
	s.OnEvent(kd("End"))
	if s.Value().Get() != 100 {
		t.Fatalf("End: Value=%d", s.Value().Get())
	}
	s.OnEvent(kd("ArrowUp")) // clamp at Max
	if s.Value().Get() != 100 {
		t.Fatalf("clamp at Max: Value=%d", s.Value().Get())
	}
	s.OnEvent(kd("Home"))
	if s.Value().Get() != 0 {
		t.Fatalf("Home: Value=%d", s.Value().Get())
	}
	s.Disabled = true
	s.OnEvent(kd("End"))
	if s.Value().Get() != 0 {
		t.Fatalf("disabled spinbutton moved (Value=%d)", s.Value().Get())
	}
	NewSpinButton(0, 10, 5, 1).OnEvent(kd("ArrowUp")) // no subscriber: safe
	// A non-click, non-keydown event is ignored.
	NewSpinButton(0, 10, 5, 1).OnEvent(Event{Kind: EventChar, Code: "a"})
}

// --- Rating --------------------------------------------------------------

func TestRatingKeyAdjusts(t *testing.T) {
	var got int
	r := NewRating(2, 5)
	r.Value().Subscribe(func(v int) { got = v })
	r.OnEvent(kd("ArrowRight"))
	if r.Value().Get() != 3 || got != 3 {
		t.Fatalf("ArrowRight: Value=%d cb=%d", r.Value().Get(), got)
	}
	r.OnEvent(kd("ArrowLeft"))
	if r.Value().Get() != 2 {
		t.Fatalf("ArrowLeft: Value=%d", r.Value().Get())
	}
	r.OnEvent(kd("End"))
	if r.Value().Get() != 5 {
		t.Fatalf("End: Value=%d", r.Value().Get())
	}
	r.OnEvent(kd("ArrowRight")) // clamp at Max
	if r.Value().Get() != 5 {
		t.Fatalf("clamp at Max: Value=%d", r.Value().Get())
	}
	r.OnEvent(kd("Home"))
	if r.Value().Get() != 0 {
		t.Fatalf("Home: Value=%d", r.Value().Get())
	}
	r.OnEvent(kd("ArrowLeft")) // clamp at 0
	if r.Value().Get() != 0 {
		t.Fatalf("clamp at 0: Value=%d", r.Value().Get())
	}
	r.Disabled = true
	r.OnEvent(kd("End"))
	if r.Value().Get() != 0 {
		t.Fatalf("disabled rating moved (Value=%d)", r.Value().Get())
	}
	NewRating(1, 5).OnEvent(kd("ArrowRight")) // nil OnChange safe
	// A non-click, non-keydown event is ignored.
	NewRating(2, 5).OnEvent(Event{Kind: EventChar, Code: "a"})
}

// --- DropDown ------------------------------------------------------------

func TestDropDownKeyboard(t *testing.T) {
	opts := []string{"a", "b", "c"}
	d := NewDropDown(opts, 1)
	// Closed: an unrelated key does not open.
	d.OnEvent(kd("ArrowUp"))
	if d.Open().Get() {
		t.Fatal("ArrowUp opened a closed dropdown")
	}
	// Space opens and remembers the current selection.
	d.OnEvent(kd(" "))
	if !d.Open().Get() || d.savedSelected != 1 {
		t.Fatalf("space open: Open=%v saved=%d", d.Open().Get(), d.savedSelected)
	}
	// ArrowDown moves the highlight, clamped at the last option.
	d.OnEvent(kd("ArrowDown")) // 1 -> 2
	d.OnEvent(kd("ArrowDown")) // clamp at 2
	if d.Selected().Get() != 2 {
		t.Fatalf("ArrowDown clamp: Selected=%d", d.Selected().Get())
	}
	// ArrowUp moves up, clamped at 0.
	d.OnEvent(kd("ArrowUp")) // 2 -> 1
	d.OnEvent(kd("ArrowUp")) // 1 -> 0
	d.OnEvent(kd("ArrowUp")) // clamp at 0
	if d.Selected().Get() != 0 {
		t.Fatalf("ArrowUp clamp: Selected=%d", d.Selected().Get())
	}
	// Escape restores the pre-open selection and closes.
	d.OnEvent(kd("Escape"))
	if d.Open().Get() || d.Selected().Get() != 1 {
		t.Fatalf("escape: Open=%v Selected=%d (want closed, 1)", d.Open().Get(), d.Selected().Get())
	}

	// ArrowDown opens (from closed), then Enter commits the highlighted option.
	sel := -1
	d2 := NewDropDown(opts, 0)
	d2.Selected().Subscribe(func(i int) { sel = i })
	d2.OnEvent(kd("ArrowDown")) // closed -> open, saved=0
	d2.OnEvent(kd("ArrowDown")) // 0 -> 1
	d2.OnEvent(kd("Enter"))     // commit 1
	if d2.Open().Get() || d2.Selected().Get() != 1 || sel != 1 {
		t.Fatalf("enter commit: Open=%v Selected=%d cb=%d", d2.Open().Get(), d2.Selected().Get(), sel)
	}

	// Disabled ignores keys.
	d3 := NewDropDown(opts, 0)
	d3.Disabled = true
	d3.OnEvent(kd(" "))
	if d3.Open().Get() {
		t.Fatal("disabled dropdown opened")
	}
}

// --- ComboBox ------------------------------------------------------------

func TestComboBoxKeyboardHighlight(t *testing.T) {
	c := NewComboBox([]string{"Apple", "Apricot", "Banana"})
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 20})
	// ArrowDown opens the popover and moves the highlight, clamped at the last.
	c.OnEvent(kd("ArrowDown")) // 0 -> 1
	if !c.Open || c.highlight != 1 {
		t.Fatalf("ArrowDown: Open=%v highlight=%d", c.Open, c.highlight)
	}
	c.OnEvent(kd("ArrowDown")) // 1 -> 2
	c.OnEvent(kd("ArrowDown")) // clamp at 2
	if c.highlight != 2 {
		t.Fatalf("ArrowDown clamp: highlight=%d", c.highlight)
	}
	c.OnEvent(kd("ArrowUp")) // 2 -> 1
	c.OnEvent(kd("ArrowUp")) // 1 -> 0
	c.OnEvent(kd("ArrowUp")) // clamp at 0
	if c.highlight != 0 {
		t.Fatalf("ArrowUp clamp: highlight=%d", c.highlight)
	}
	// Escape closes.
	c.OnEvent(kd("Escape"))
	if c.Open {
		t.Fatal("escape did not close combobox")
	}

	// Enter selects the highlighted (not just the first) option.
	sel := ""
	c2 := NewComboBox([]string{"Apple", "Apricot", "Banana"})
	c2.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 20})
	c2.OnSelect = func(s string) { sel = s }
	c2.OnEvent(kd("ArrowDown")) // highlight 1
	c2.OnEvent(kd("Enter"))
	if sel != "Apricot" || c2.Text != "Apricot" || c2.Open {
		t.Fatalf("enter select: sel=%q Text=%q Open=%v", sel, c2.Text, c2.Open)
	}

	// Typing resets the highlight to the first match; Backspace too.
	c3 := NewComboBox([]string{"Apple", "Apricot", "Banana"})
	c3.OnEvent(kd("ArrowDown")) // highlight 1
	c3.OnEvent(Event{Kind: EventChar, Code: "A"})
	if c3.highlight != 0 {
		t.Fatalf("char did not reset highlight (=%d)", c3.highlight)
	}
	c3.OnEvent(kd("ArrowDown")) // highlight 1 again
	c3.OnEvent(kd("Backspace"))
	if c3.highlight != 0 {
		t.Fatalf("backspace did not reset highlight (=%d)", c3.highlight)
	}

	// Enter with no matches is a no-op.
	c4 := NewComboBox([]string{"Apple"})
	c4.Text = "zzz"
	c4.OnEvent(kd("Enter"))
	if c4.Text != "zzz" {
		t.Fatalf("enter with no matches selected something: Text=%q", c4.Text)
	}

	// Disabled ignores keys.
	c5 := NewComboBox([]string{"Apple"})
	c5.Disabled = true
	c5.OnEvent(kd("ArrowDown"))
	if c5.Open {
		t.Fatal("disabled combobox opened")
	}
}

func TestComboBoxHighlightRowClamps(t *testing.T) {
	c := NewComboBox([]string{"a", "b"})
	// Above range -> last row.
	c.highlight = 9
	if got := c.highlightRow(); got != 1 {
		t.Fatalf("above-range highlightRow=%d, want 1", got)
	}
	// Below range -> first row.
	c.highlight = -3
	if got := c.highlightRow(); got != 0 {
		t.Fatalf("below-range highlightRow=%d, want 0", got)
	}
	// No visible rows -> 0.
	c.Text = "zzz"
	if got := c.highlightRow(); got != 0 {
		t.Fatalf("empty highlightRow=%d, want 0", got)
	}
}

func TestComboBoxDrawOpenHighlightRow(t *testing.T) {
	c := NewComboBox([]string{"a", "b", "c"})
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 18})
	c.Open = true
	c.highlight = 1
	surf := makeSurface(120, 200)
	c.Draw(newP(surf, 120), DefaultLight())
	pb := c.PopoverBounds()
	// The highlighted (second) row carries a SurfaceAlt band.
	rowY := pb.Y + comboRowH + comboRowH/2
	if got := pixelAt(surf, 120, pb.X+2, rowY); got != DefaultLight().SurfaceAlt {
		t.Fatalf("highlight band = %+v, want SurfaceAlt", got)
	}
	// The popover corner stays Border (stroked after the highlight fill).
	if got := pixelAt(surf, 120, pb.X, pb.Y); got != DefaultLight().Border {
		t.Fatalf("popover corner = %+v, want Border", got)
	}
}

// --- Notebook / ViewSwitcher (tablists) ----------------------------------

func TestNotebookKeyMovesTab(t *testing.T) {
	changed := -1
	n := &Notebook{Tabs: []NotebookTab{{Label: "A"}, {Label: "B"}, {Label: "C"}}}
	n.Active().Subscribe(func(i int) { changed = i })
	n.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 100})
	n.OnEvent(kd("ArrowRight")) // 0 -> 1
	if n.Active().Get() != 1 || changed != 1 {
		t.Fatalf("ArrowRight: Active=%d cb=%d", n.Active().Get(), changed)
	}
	n.OnEvent(kd("ArrowLeft")) // 1 -> 0
	n.OnEvent(kd("ArrowLeft")) // 0 -> 2 (wrap)
	if n.Active().Get() != 2 {
		t.Fatalf("ArrowLeft wrap: Active=%d", n.Active().Get())
	}
	n.OnEvent(kd("ArrowDown")) // 2 -> 0 (wrap), axis-agnostic
	if n.Active().Get() != 0 {
		t.Fatalf("ArrowDown wrap: Active=%d", n.Active().Get())
	}
	// Disabled ignores the tab-move key.
	n.Disabled = true
	n.OnEvent(kd("ArrowRight"))
	if n.Active().Get() != 0 {
		t.Fatalf("disabled notebook moved tab (Active=%d)", n.Active().Get())
	}
	// Out-of-range active index is treated as 0 by stepTab; empty tabs are a no-op.
	n.Disabled = false
	n.Active().Set(99)
	n.OnEvent(kd("ArrowRight"))
	if n.Active().Get() != 1 {
		t.Fatalf("out-of-range Active step: Active=%d, want 1", n.Active().Get())
	}
	empty := &Notebook{}
	empty.OnEvent(kd("ArrowRight")) // no panic
	// A notebook with no subscriber bound is safe.
	nn := &Notebook{Tabs: []NotebookTab{{Label: "A"}, {Label: "B"}}}
	nn.OnEvent(kd("ArrowRight"))
}

func TestViewSwitcherKeyMovesSegment(t *testing.T) {
	changed := -1
	v := &ViewSwitcher{Views: []string{"A", "B", "C"}}
	v.Current().Subscribe(func(i int) { changed = i })
	v.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 30})
	v.OnEvent(kd("ArrowRight")) // 0 -> 1
	if v.Current().Get() != 1 || changed != 1 {
		t.Fatalf("ArrowRight: Current=%d cb=%d", v.Current().Get(), changed)
	}
	v.OnEvent(kd("ArrowLeft")) // 1 -> 0
	v.OnEvent(kd("ArrowLeft")) // 0 -> 2 (wrap)
	if v.Current().Get() != 2 {
		t.Fatalf("ArrowLeft wrap: Current=%d", v.Current().Get())
	}
	v.Disabled = true
	v.OnEvent(kd("ArrowRight"))
	if v.Current().Get() != 2 {
		t.Fatalf("disabled view switcher moved (Current=%d)", v.Current().Get())
	}
	v.Disabled = false
	v.Current().Set(99) // out of range -> treated as 0
	v.OnEvent(kd("ArrowRight"))
	if v.Current().Get() != 1 {
		t.Fatalf("out-of-range Current step: Current=%d", v.Current().Get())
	}
	(&ViewSwitcher{}).OnEvent(kd("ArrowRight")) // empty, no panic
	vn := &ViewSwitcher{Views: []string{"A", "B"}}
	vn.OnEvent(kd("ArrowRight"))                  // no subscriber safe
	vn.OnEvent(Event{Kind: EventChar, Code: "a"}) // non-click/keydown ignored
}

// --- Pagination ----------------------------------------------------------

func TestPaginationKeyNavigates(t *testing.T) {
	changes := 0
	p := NewPagination(3, 5)
	p.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(5), H: PaginationBtnH})
	p.Current().Subscribe(func(int) { changes++ })
	p.OnEvent(kd("ArrowRight")) // 3 -> 4
	if p.Current().Get() != 4 || changes != 1 {
		t.Fatalf("ArrowRight: Current=%d changes=%d", p.Current().Get(), changes)
	}
	p.OnEvent(kd("ArrowLeft")) // 4 -> 3
	if p.Current().Get() != 3 {
		t.Fatalf("ArrowLeft: Current=%d", p.Current().Get())
	}
	p.OnEvent(kd("End")) // -> 5
	if p.Current().Get() != 5 {
		t.Fatalf("End: Current=%d", p.Current().Get())
	}
	p.OnEvent(kd("ArrowRight")) // clamp at Total (no change, no fire)
	if p.Current().Get() != 5 {
		t.Fatalf("clamp at Total: Current=%d", p.Current().Get())
	}
	p.OnEvent(kd("Home")) // -> 1
	if p.Current().Get() != 1 {
		t.Fatalf("Home: Current=%d", p.Current().Get())
	}
	before := changes
	p.OnEvent(kd("Home")) // already at 1: no change, no extra fire
	if p.Current().Get() != 1 || changes != before {
		t.Fatalf("Home no-op fired (changes=%d)", changes)
	}
	p.OnEvent(kd("ArrowLeft")) // goTo(0) clamps up to 1: no change
	if p.Current().Get() != 1 || changes != before {
		t.Fatalf("ArrowLeft below 1 fired (Current=%d changes=%d)", p.Current().Get(), changes)
	}
	p.Disabled = true
	p.OnEvent(kd("End"))
	if p.Current().Get() != 1 {
		t.Fatalf("disabled pagination moved (Current=%d)", p.Current().Get())
	}
	// Total<=0 ignores keys.
	empty := &Pagination{Total: 0}
	empty.OnEvent(kd("ArrowRight"))
	if empty.Current().Get() != 0 {
		t.Fatalf("empty pagination moved (Current=%d)", empty.Current().Get())
	}
	// Nil OnChange is safe.
	pn := NewPagination(1, 3)
	pn.SetBounds(Rect{X: 0, Y: 0, W: paginationLayoutW(3), H: PaginationBtnH})
	pn.OnEvent(kd("ArrowRight"))
	// A non-click, non-keydown event is ignored.
	pn.OnEvent(Event{Kind: EventChar, Code: "a"})
}

// --- Calendar ------------------------------------------------------------

func TestCalendarKeyboard(t *testing.T) {
	// August 2026 has 31 days.
	c := NewCalendar(2026, 8, 15)
	sel := [3]int{}
	c.OnSelect = func(y, m, d int) { sel = [3]int{y, m, d} }
	monthChanges := 0
	c.OnMonthChange = func(y, m int) { monthChanges++ }
	c.OnEvent(kd("ArrowRight")) // 15 -> 16, no OnSelect
	if c.Day != 16 || sel != [3]int{} {
		t.Fatalf("ArrowRight: Day=%d sel=%v (should not select)", c.Day, sel)
	}
	c.OnEvent(kd("ArrowLeft")) // 16 -> 15
	c.OnEvent(kd("ArrowDown")) // +7 -> 22
	if c.Day != 22 {
		t.Fatalf("ArrowDown: Day=%d", c.Day)
	}
	c.OnEvent(kd("ArrowUp")) // -7 -> 15
	if c.Day != 15 {
		t.Fatalf("ArrowUp: Day=%d", c.Day)
	}
	c.OnEvent(kd("End")) // last of month -> 31
	if c.Day != 31 {
		t.Fatalf("End: Day=%d", c.Day)
	}
	c.OnEvent(kd("ArrowDown")) // +7 clamps at 31
	if c.Day != 31 {
		t.Fatalf("ArrowDown clamp: Day=%d", c.Day)
	}
	c.OnEvent(kd("Home")) // first of month -> 1
	if c.Day != 1 {
		t.Fatalf("Home: Day=%d", c.Day)
	}
	c.OnEvent(kd("ArrowLeft")) // clamp at 1
	if c.Day != 1 {
		t.Fatalf("ArrowLeft clamp: Day=%d", c.Day)
	}
	// PageUp / PageDown step the month, reusing Prev/NextMonth (OnMonthChange).
	c.OnEvent(kd("PageDown")) // -> September
	if c.Month != 9 || monthChanges != 1 {
		t.Fatalf("PageDown: Month=%d changes=%d", c.Month, monthChanges)
	}
	c.OnEvent(kd("PageUp")) // -> August
	if c.Month != 8 || monthChanges != 2 {
		t.Fatalf("PageUp: Month=%d changes=%d", c.Month, monthChanges)
	}
	// Enter selects the focused day.
	c.Day = 10
	c.OnEvent(kd("Enter"))
	if sel != [3]int{2026, 8, 10} {
		t.Fatalf("Enter select: sel=%v", sel)
	}
	// Space also selects.
	c.Day = 11
	c.OnEvent(kd(" "))
	if sel != [3]int{2026, 8, 11} {
		t.Fatalf("Space select: sel=%v", sel)
	}
	// Disabled ignores keys.
	c.Disabled = true
	c.OnEvent(kd("ArrowRight"))
	if c.Day != 11 {
		t.Fatalf("disabled calendar moved (Day=%d)", c.Day)
	}
	// Nil OnSelect is safe.
	NewCalendar(2026, 8, 1).OnEvent(kd("Enter"))
	// A non-click, non-keydown event is ignored.
	NewCalendar(2026, 8, 1).OnEvent(Event{Kind: EventChar, Code: "a"})
}

// --- Accordion / Expander ------------------------------------------------

func TestAccordionKeyboard(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 300})
	// Enter toggles the focused (initially first) section.
	a.OnEvent(kd("Enter"))
	if a.Expanded().Get() != 0 {
		t.Fatalf("Enter toggle: Expanded=%d", a.Expanded().Get())
	}
	a.OnEvent(kd("Enter")) // collapse
	if a.Expanded().Get() != -1 {
		t.Fatalf("Enter collapse: Expanded=%d", a.Expanded().Get())
	}
	// ArrowDown moves header focus; Space toggles the now-focused section.
	a.OnEvent(kd("ArrowDown")) // focus 1
	a.OnEvent(kd(" "))
	if a.Expanded().Get() != 1 {
		t.Fatalf("focus+space: Expanded=%d", a.Expanded().Get())
	}
	// ArrowDown clamps at the last section.
	a.OnEvent(kd("ArrowDown")) // focus 2
	a.OnEvent(kd("ArrowDown")) // clamp at 2
	a.OnEvent(kd(" "))
	if a.Expanded().Get() != 2 {
		t.Fatalf("clamp last: Expanded=%d", a.Expanded().Get())
	}
	// ArrowUp clamps at the first section.
	a.OnEvent(kd("ArrowUp"))
	a.OnEvent(kd("ArrowUp"))
	a.OnEvent(kd("ArrowUp")) // clamp at 0
	a.OnEvent(kd("Enter"))
	if a.Expanded().Get() != 0 {
		t.Fatalf("clamp first: Expanded=%d", a.Expanded().Get())
	}
	// Disabled ignores keys.
	a.Disabled = true
	a.OnEvent(kd("ArrowDown"))
	a.OnEvent(kd("Enter"))
	if a.Expanded().Get() != 0 {
		t.Fatalf("disabled accordion changed (Expanded=%d)", a.Expanded().Get())
	}
	// Empty accordion is a no-op.
	NewAccordion(nil).OnEvent(kd("Enter"))
	// A non-click, non-keydown event is ignored.
	a.Disabled = false
	a.OnEvent(Event{Kind: EventChar, Code: "a"})
}

func TestAccordionFocusIdxClamps(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}})
	a.focusedSection = -5
	if got := a.focusIdx(); got != 0 {
		t.Fatalf("negative focusIdx=%d, want 0", got)
	}
	a.focusedSection = 9
	if got := a.focusIdx(); got != 1 {
		t.Fatalf("over-range focusIdx=%d, want 1", got)
	}
}

func TestAccordionDrawsFocusRingWhenFocused(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	a.SetFocused(true)
	surf := makeSurface(60, 60)
	a.Draw(newP(surf, 60), DefaultLight())
	// The focus ring is stroked one pixel in from the focused header's corner.
	if got := pixelAt(surf, 60, 1, 1); got != DefaultLight().Accent {
		t.Fatalf("focus ring pixel = %+v, want Accent", got)
	}
}

func TestExpanderKeyToggles(t *testing.T) {
	expands := 0
	e := NewExpander("S", nil)
	e.Expanded().Subscribe(func(bool) { expands++ })
	e.OnEvent(kd("Enter"))
	if !e.Expanded().Get() || expands != 1 {
		t.Fatalf("Enter: Expanded=%v expands=%d", e.Expanded().Get(), expands)
	}
	e.OnEvent(kd(" "))
	if e.Expanded().Get() || expands != 2 {
		t.Fatalf("Space: Expanded=%v expands=%d", e.Expanded().Get(), expands)
	}
	e.Disabled = true
	e.OnEvent(kd("Enter"))
	if e.Expanded().Get() || expands != 2 {
		t.Fatalf("disabled expander toggled (Expanded=%v)", e.Expanded().Get())
	}
	NewExpander("nil", nil).OnEvent(kd("Enter")) // unbound observable safe
	// A non-click, non-keydown event is ignored.
	NewExpander("S", nil).OnEvent(Event{Kind: EventChar, Code: "a"})
}

func TestExpanderDrawsFocusRingWhenFocused(t *testing.T) {
	e := NewExpander("S", nil)
	e.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 60})
	e.SetFocused(true)
	surf := makeSurface(80, 60)
	e.Draw(newP(surf, 80), DefaultLight())
	if got := pixelAt(surf, 80, 1, 1); got != DefaultLight().Accent {
		t.Fatalf("focus ring pixel = %+v, want Accent", got)
	}
}

// --- Carousel ------------------------------------------------------------

func TestCarouselKeyNavigates(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	c.OnEvent(kd("ArrowRight")) // 0 -> 1
	if c.Current != 1 {
		t.Fatalf("ArrowRight: Current=%d", c.Current)
	}
	c.OnEvent(kd("ArrowLeft")) // 1 -> 0
	if c.Current != 0 {
		t.Fatalf("ArrowLeft: Current=%d", c.Current)
	}
	c.OnEvent(kd("ArrowLeft")) // clamp at 0 (Wrap off)
	if c.Current != 0 {
		t.Fatalf("clamp at 0: Current=%d", c.Current)
	}
	// Disabled ignores keys.
	c.Disabled = true
	c.OnEvent(kd("ArrowRight"))
	if c.Current != 0 {
		t.Fatalf("disabled carousel moved (Current=%d)", c.Current)
	}
	// Empty carousel is a no-op.
	NewCarousel(nil).OnEvent(kd("ArrowRight"))
}

func TestCarouselDrawsFocusRingWhenFocused(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a")})
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	c.SetFocused(true)
	surf := makeSurface(200, 120)
	c.Draw(newP(surf, 200), DefaultLight())
	if got := pixelAt(surf, 200, 1, 1); got != DefaultLight().Accent {
		t.Fatalf("focus ring pixel = %+v, want Accent", got)
	}
}
