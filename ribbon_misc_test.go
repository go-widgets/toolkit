// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestRibbonHitTestOutsideAndActivateOutOfRange(t *testing.T) {
	r, run := rtRibbon(800)
	if !r.HitTest(10, 10) {
		t.Error("a point inside the ribbon is a hit")
	}
	if r.HitTest(-5, -5) || r.HitTest(10, r.Height()+50) {
		t.Error("a point outside the ribbon with nothing open is not a hit")
	}
	// An open menu hangs below the ribbon: its panel is a hit, the air under it is not.
	r.activate(rtCell(r, "New slide"), partArrow)
	panel, _ := r.popupGeometry(r.pop, r.metrics())
	if panel.Y+panel.H <= r.Bounds().H {
		t.Fatal("the test needs a menu that hangs below the ribbon")
	}
	px, py := rtCenter(panel)
	if !r.HitTest(px, py) || r.HitTest(px, panel.Y+panel.H+20) {
		t.Error("an open menu's panel (and only it) is a hit below the ribbon")
	}
	r.Dismiss()
	r.activate(-1, partMain)
	r.activate(999, partMain)
	if len(run.ids) != 0 {
		t.Error("an out-of-range activation ran something")
	}
}

func TestRibbonEscapeClosesAFoldedGroupFirst(t *testing.T) {
	r, _ := rtRibbon(360)
	r.SetFocused(true)
	r.activate(rtCell(r, "Theme"), partMain)
	if r.pop == nil {
		t.Fatal("the group did not open")
	}
	rtKey(r, "Escape")
	if r.pop != nil || r.zone != ribbonZoneBody {
		t.Errorf("the first Escape closes the panel and keeps the focus in the body: pop=%v zone=%d", r.pop, r.zone)
	}
	rtKey(r, "Escape")
	if r.zone != ribbonZoneTabs {
		t.Error("the second Escape goes back to the tabs")
	}
}

func TestRibbonTooltipOfAFoldedGroup(t *testing.T) {
	r, _ := rtRibbon(360)
	i := rtCell(r, "Theme")
	x, y := rtCenterOf(r.cells()[i])
	rtMove(r, x, y)
	r.Tick(0.6)
	if lines := r.tooltipLines(); len(lines) != 1 || lines[0] != "Theme" {
		t.Errorf("a folded group's tooltip is its name: %q", lines)
	}
}

func TestRibbonItemIcons(t *testing.T) {
	if (RibbonItem{Label: "x"}).icon() != nil {
		t.Error("an item with no icon and no action has no icon")
	}
	if (RibbonItem{Action: NewAction("a", "A", nil)}).icon() != nil {
		t.Error("an item whose action has no icon has no icon")
	}
	a := NewAction("b", "B", nil)
	a.Icon = ico("page")
	if (RibbonItem{Action: a}).icon() == nil || (RibbonItem{Icon: ico("page"), Action: NewAction("c", "C", nil)}).icon() == nil {
		t.Error("an item takes its own icon, else its action's")
	}
	if (RibbonItem{Action: a}).text() != "B" || (RibbonItem{Label: "L"}).text() != "L" {
		t.Error("an item's text is its action's label, else its own")
	}
	if !(RibbonItem{}).enabled() {
		t.Error("an item with no action is enabled")
	}
}
