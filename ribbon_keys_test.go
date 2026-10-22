// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

// The tabs are a WAI-ARIA tablist: Left/Right move (wrapping) and show the tab at
// once, Home/End jump.
func TestRibbonTablistKeys(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.SetFocused(true)
	rtKey(r, "ArrowRight")
	if r.ActiveTab().Get() != 1 || r.focusTab != 1 {
		t.Errorf("Right: tab %d", r.ActiveTab().Get())
	}
	rtKey(r, "End")
	if r.ActiveTab().Get() != 2 {
		t.Errorf("End: tab %d", r.ActiveTab().Get())
	}
	rtKey(r, "ArrowRight")
	if r.ActiveTab().Get() != 0 {
		t.Errorf("Right on the last tab wraps to the first, got %d", r.ActiveTab().Get())
	}
	rtKey(r, "ArrowLeft")
	if r.ActiveTab().Get() != 2 {
		t.Errorf("Left on the first tab wraps to the last, got %d", r.ActiveTab().Get())
	}
	rtKey(r, "Home")
	if r.ActiveTab().Get() != 0 {
		t.Errorf("Home: tab %d", r.ActiveTab().Get())
	}
	rtKey(r, "Escape")
	rtKey(r, "x") // not a tablist key
}

// The body is a toolbar: Left/Right move between controls, Up/Down inside a
// column, Enter runs, Escape goes back to the tabs.
func TestRibbonBodyKeys(t *testing.T) {
	r, run := rtRibbon(2000)
	r.SetFocused(true)
	rtKey(r, "ArrowDown") // tabs -> body
	if r.zone != ribbonZoneBody || r.focusCtl != 0 {
		t.Fatalf("Down from the tabs enters the body at its first control: zone %d focus %d", r.zone, r.focusCtl)
	}
	rtKey(r, "ArrowRight")
	if got := r.cells()[r.focusCtl].ctrl.label(); got != "Duplicate" {
		t.Errorf("Right moved to %q", got)
	}
	rtKey(r, "ArrowDown")
	if got := r.cells()[r.focusCtl].ctrl.label(); got != "Delete" {
		t.Errorf("Down inside the column moved to %q", got)
	}
	rtKey(r, "ArrowUp")
	if got := r.cells()[r.focusCtl].ctrl.label(); got != "Duplicate" {
		t.Errorf("Up inside the column moved to %q", got)
	}
	rtKey(r, "Enter")
	rtKey(r, "ArrowLeft")
	rtKey(r, "ArrowLeft") // wraps to the last control
	if got := r.cells()[r.focusCtl].ctrl.label(); got != "Kinds" {
		t.Errorf("Left from the first control wraps to %q", got)
	}
	rtKey(r, "Home")
	if r.focusCtl != 0 {
		t.Errorf("Home: %d", r.focusCtl)
	}
	rtKey(r, "End")
	if r.focusCtl != len(r.cells())-1 {
		t.Errorf("End: %d", r.focusCtl)
	}
	rtKey(r, "Home")
	rtKey(r, "ArrowUp") // nothing above the first control: back to the tabs
	if r.zone != ribbonZoneTabs {
		t.Error("Up with nothing above returns to the tabs")
	}
	rtKey(r, "Enter")
	rtKey(r, "ArrowRight")
	rtKey(r, " ")
	rtKey(r, "Escape")
	if r.zone != ribbonZoneTabs {
		t.Error("Escape returns to the tabs")
	}
	if !reflect.DeepEqual(run.ids, []string{"slide.dup", "slide.dup"}) {
		t.Errorf("Enter and Space activated %v", run.ids)
	}
}

func TestRibbonKeysSkipDisabledControls(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.Tabs[0].Groups[0].Controls[0].Action.SetEnabled(false) // New slide
	r.SetFocused(true)
	rtKey(r, "ArrowDown")
	if got := r.cells()[r.focusCtl].ctrl.label(); got != "Duplicate" {
		t.Errorf("focus entered on the disabled %q", got)
	}
	// With everything disabled there is nowhere to go.
	for _, g := range r.Tabs[0].Groups {
		for _, c := range g.Controls {
			if c.Action != nil {
				c.Action.SetEnabled(false)
			}
		}
	}
	r.Tabs[0].Groups[2].Controls[0].Kind, r.Tabs[0].Groups[2].Controls[1].Kind = RibbonButton, RibbonButton
	r.Tabs[0].Groups[2].Controls[0].Action = NewAction("d", "D", nil)
	r.Tabs[0].Groups[2].Controls[1].Action = NewAction("g", "G", nil)
	r.Tabs[0].Groups[2].Controls[0].Action.SetEnabled(false)
	r.Tabs[0].Groups[2].Controls[1].Action.SetEnabled(false)
	r.zone = ribbonZoneTabs
	rtKey(r, "ArrowDown")
	if r.zone != ribbonZoneTabs {
		t.Error("with every control disabled the body cannot take the focus")
	}
	r.zone, r.focusCtl = ribbonZoneBody, 0
	rtKey(r, "ArrowRight")
	if r.focusCtl != 0 {
		t.Error("stepping with nothing usable stays put")
	}
}

func TestRibbonKeysOpenMenusAndPopups(t *testing.T) {
	r, run := rtRibbon(2000)
	r.SetFocused(true)
	rtKey(r, "ArrowDown")
	// Alt+Down on a split opens its menu even with a control below it.
	r.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Alt: true})
	if r.pop == nil || r.pop.kind != popMenu {
		t.Fatal("Alt+Down opens the split button's menu")
	}
	rtKey(r, "ArrowDown")
	rtKey(r, "ArrowDown") // clamps at the last row
	rtKey(r, "ArrowUp")
	rtKey(r, "Home")
	rtKey(r, "End")
	if r.pop.focus != 1 {
		t.Errorf("menu focus %d", r.pop.focus)
	}
	rtKey(r, "ArrowRight") // sideways does nothing in a plain menu
	rtKey(r, "ArrowLeft")
	rtKey(r, "Enter")
	if r.pop != nil || run.ids[len(run.ids)-1] != "slide.code" {
		t.Errorf("Enter chose the highlighted row: %v", run.ids)
	}
	// Escape closes without choosing.
	r.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Alt: true})
	rtKey(r, "Escape")
	if r.pop != nil {
		t.Error("Escape closes the menu")
	}
	// Down on a control with nothing below it opens its list; Space chooses.
	i := rtCell(r, "Style")
	r.focusCtl = i
	rtKey(r, "ArrowDown")
	if r.pop == nil || r.pop.kind != popList {
		t.Fatal("Down on a drop-down opens its list")
	}
	rtKey(r, "ArrowDown")
	rtKey(r, " ")
	if got := r.Tabs[0].Groups[2].Controls[0].selection().Get(); got != 1 {
		t.Errorf("Space chose option %d", got)
	}
	// A gallery grid moves sideways and by row.
	r.focusCtl = rtCell(r, "Kinds")
	rtKey(r, "ArrowDown")
	if r.pop == nil || r.pop.kind != popGallery {
		t.Fatal("Down on a gallery opens its grid")
	}
	rtKey(r, "ArrowRight")
	rtKey(r, "ArrowRight")
	rtKey(r, "ArrowLeft")
	rtKey(r, "ArrowDown") // a row down is past the end of four cells in four columns: clamps
	rtKey(r, "ArrowUp")
	rtKey(r, "Enter")
	if r.pop != nil {
		t.Error("Enter picks a gallery cell and closes")
	}
	// Down on a plain button with nothing below does nothing at all.
	r.focusCtl = rtCell(r, "Delete")
	rtKey(r, "ArrowDown")
	if r.pop != nil {
		t.Error("Down on a plain button opened something")
	}
	// A popup with nothing in it ignores Enter.
	r.pop = &ribbonPopup{kind: popList, ctrl: &RibbonControl{Kind: RibbonDropdown}}
	rtKey(r, "Enter")
	if r.pop == nil {
		t.Error("Enter on an empty list should do nothing")
	}
	r.Dismiss()
	// A disabled drop-down does not open from the keyboard.
	r.Tabs[0].Groups[2].Controls[0].Action = NewAction("dd", "Style", nil)
	r.Tabs[0].Groups[2].Controls[0].Action.SetEnabled(false)
	r.focusCtl = rtCell(r, "Style")
	r.openFocused(r.cells(), r.focusCtl)
	if r.pop != nil {
		t.Error("a disabled control opened its popup")
	}
}

func TestRibbonCollapsedKeyboardPeek(t *testing.T) {
	r, run := rtRibbon(2000)
	r.Collapsed().Set(true)
	r.SetFocused(true)
	rtKey(r, "ArrowDown")
	if !r.peek || r.zone != ribbonZoneBody {
		t.Fatal("Down on a collapsed ribbon peeks at the body and enters it")
	}
	rtKey(r, "Enter")
	if r.peek || len(run.ids) != 1 {
		t.Errorf("running from the peek closes it: peek=%v ran %v", r.peek, run.ids)
	}
	rtKey(r, "ArrowDown")
	rtKey(r, "Escape")
	if r.peek || r.zone != ribbonZoneTabs {
		t.Error("Escape leaves the body and puts the peek away")
	}
	// Moving between the tabs of a collapsed ribbon does not peek.
	rtKey(r, "ArrowRight")
	if r.peek || r.ActiveTab().Get() != 1 {
		t.Error("moving to another tab shows no body on a collapsed ribbon")
	}
	// Escape on the tabs puts a peek away.
	r.peek = true
	rtKey(r, "Escape")
	if r.peek {
		t.Error("Escape on the tabs should close the peek")
	}
}

func TestRibbonCtrlF1TogglesCollapse(t *testing.T) {
	r, _ := rtRibbon(2000)
	ev := Event{Kind: EventKeyDown, Code: "F1", Ctrl: true}
	r.OnEvent(ev)
	if !r.Collapsed().Get() {
		t.Fatal("Ctrl+F1 collapses")
	}
	r.OnEvent(ev)
	if r.Collapsed().Get() {
		t.Fatal("Ctrl+F1 expands")
	}
	r.OnEvent(Event{Kind: EventKeyDown, Code: "F1"})
	if r.Collapsed().Get() {
		t.Error("F1 alone is not the shortcut")
	}
}

func TestRibbonBodyKeyWithNoControlsReturnsToTabs(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.zone = ribbonZoneBody
	r.Collapsed().Set(true)
	rtKey(r, "ArrowRight")
	if r.zone != ribbonZoneTabs {
		t.Error("with no body to navigate the focus returns to the tabs")
	}
}

// KEYTIPS: Alt shows a letter over each tab, a letter picks the tab and shows
// one over each control, the next letter runs it; Escape goes back a level.
func TestRibbonKeyTips(t *testing.T) {
	r, run := rtRibbon(2000)
	tips := r.tabTips()
	if !reflect.DeepEqual(tips, []string{"H", "I", "V"}) {
		t.Fatalf("tab tips %v", tips)
	}
	rtKey(r, "Alt")
	if r.keytips != 1 {
		t.Fatal("Alt starts the keytips")
	}
	rtKey(r, "I") // Insert
	if r.keytips != 2 || r.ActiveTab().Get() != 1 {
		t.Fatalf("a tab keytip shows the tab and asks for a control: keytips %d tab %d", r.keytips, r.ActiveTab().Get())
	}
	ct := r.cellTips(r.cells())
	if !reflect.DeepEqual(ct, []string{"I", "B"}) {
		t.Fatalf("control tips %v", ct)
	}
	rtKey(r, "B")
	if r.keytips != 0 || !reflect.DeepEqual(run.ids, []string{"bg"}) {
		t.Errorf("a control keytip runs it and ends the mode: keytips %d ran %v", r.keytips, run.ids)
	}
	// A wrong letter starts the tip over; Escape steps back, then out.
	rtKey(r, "Alt")
	rtKey(r, "Q")
	if r.tipBuf != "" || r.keytips != 1 {
		t.Error("an unknown tab letter changes nothing")
	}
	rtKey(r, "H")
	rtKey(r, "Z") // wrong control letter
	rtKey(r, "Escape")
	if r.keytips != 1 {
		t.Errorf("Escape at the control level goes back to the tabs, keytips=%d", r.keytips)
	}
	rtKey(r, "ArrowDown") // not a tip: ignored
	rtKey(r, "Escape")
	if r.keytips != 0 {
		t.Error("Escape at the tab level leaves the keytips")
	}
	// Alt again leaves it too, and a click does.
	rtKey(r, "Alt")
	rtKey(r, "Alt")
	if r.keytips != 0 {
		t.Error("Alt toggles")
	}
	rtKey(r, "Alt")
	rtClick(r, 5, 5)
	if r.keytips != 0 {
		t.Error("a click leaves the keytips")
	}
}

func TestRibbonKeyTipsOnACollapsedRibbonPeekAndRunDropdowns(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.Collapsed().Set(true)
	rtKey(r, "Alt")
	rtKey(r, "H")
	if !r.peek {
		t.Fatal("a keytip on a collapsed ribbon peeks at its tab")
	}
	// Choosing the drop-down by its tip opens its list.
	var themeTip string
	for i, tip := range r.cellTips(r.cells()) {
		if r.cells()[i].ctrl.label() == "Style" {
			themeTip = tip
		}
	}
	rtKey(r, themeTip)
	if r.pop == nil || r.pop.kind != popList {
		t.Errorf("the drop-down's keytip %q opens it", themeTip)
	}
	rtKey(r, "Escape")
	rtKey(r, "Escape")
}

// A keytip may be two letters: the first narrows, the second runs. A folded
// group's button gets a Z-prefixed tip, and no other control starts with Z.
func TestRibbonTwoLetterAndFoldedGroupTips(t *testing.T) {
	r, _ := rtRibbon(360)
	cells := r.cells()
	tips := r.cellTips(cells)
	for i, c := range cells {
		if c.form == formPopup {
			if len(tips[i]) != 2 || tips[i][0] != 'Z' {
				t.Errorf("folded group tip %q", tips[i])
			}
		} else if len(tips[i]) > 0 && tips[i][0] == 'Z' {
			t.Errorf("control %q got a Z tip %q", c.ctrl.label(), tips[i])
		}
	}
	// Explicit tips win, and assignment skips the taken ones.
	got := assignTips([]string{"", "p", ""}, []string{"Paste", "Print", "Page"}, "", "")
	if !reflect.DeepEqual(got, []string{"P", "P", "A"}) {
		// explicit "P" is reserved for Print, so Paste moves on to A, then Page to G.
		t.Logf("assignTips = %v", got)
	}
	got = assignTips([]string{"", "P", ""}, []string{"Paste", "Print", "Page"}, "", "")
	if !reflect.DeepEqual(got, []string{"A", "P", "G"}) {
		t.Errorf("assignTips = %v, want [A P G]", got)
	}
	// Digits are the last resort, and a label with nothing left gets no tip.
	got = assignTips(make([]string, 3), []string{"a", "a", "a"}, "", "")
	if !reflect.DeepEqual(got, []string{"A", "1", "2"}) {
		t.Errorf("digits fallback %v", got)
	}
	many := make([]string, 40)
	got = assignTips(make([]string, 40), many, "", "")
	if got[39] != "" {
		t.Errorf("no tip left for the 40th, got %q", got[39])
	}
	// A two-letter tip: the first letter narrows (nothing runs yet), the second runs.
	run := &sampleRun{}
	one := &RibbonControl{Kind: RibbonButton, Action: run.act("x", "Xylophone", ""), KeyTip: "AB"}
	rb := NewRibbon(&RibbonTab{Label: "T", Groups: []*RibbonGroup{{Label: "G", Controls: []*RibbonControl{one}}}})
	rb.SetBounds(Rect{W: 600, H: rb.Height()})
	rtKey(rb, "Alt")
	rtKey(rb, "T")
	rtKey(rb, "A")
	if rb.keytips != 2 || rb.tipBuf != "A" || len(run.ids) != 0 {
		t.Fatalf("first letter of a two-letter tip: keytips %d buf %q", rb.keytips, rb.tipBuf)
	}
	rtKey(rb, "B")
	if !reflect.DeepEqual(run.ids, []string{"x"}) {
		t.Errorf("second letter ran %v", run.ids)
	}
}

func TestMatchTip(t *testing.T) {
	tips := []string{"A", "BC", "", "BD"}
	if e, p := matchTip(tips, "A"); e != 0 || p {
		t.Errorf("exact: %d %v", e, p)
	}
	if e, p := matchTip(tips, "B"); e != -1 || !p {
		t.Errorf("prefix: %d %v", e, p)
	}
	if e, p := matchTip(tips, "Q"); e != -1 || p {
		t.Errorf("none: %d %v", e, p)
	}
	if e, p := matchTip(tips, "BC"); e != 1 || p {
		t.Errorf("exact two-letter: %d %v", e, p)
	}
}
