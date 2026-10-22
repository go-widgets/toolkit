// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestRibbonTabRectAndControlRect(t *testing.T) {
	r, run := rtRibbon(2000)
	tab, ok := r.TabRect(1)
	if !ok || !tab.Contains(tab.X+1, tab.Y+1) {
		t.Fatalf("TabRect(1) = %v %v", tab, ok)
	}
	if _, ok := r.TabRect(-1); ok {
		t.Error("a negative tab has no rect")
	}
	if _, ok := r.TabRect(99); ok {
		t.Error("a tab past the end has no rect")
	}
	// A control's rect is where a click runs it.
	rc, ok := r.ControlRect("Undo")
	if !ok {
		t.Fatal("Undo is shown")
	}
	x, y := rtCenter(rc)
	rtClick(r, x, y)
	if len(run.ids) != 1 || run.ids[0] != "undo" {
		t.Errorf("a click at ControlRect ran %v", run.ids)
	}
	// Not shown: another tab's control, and a folded group's own control.
	if _, ok := r.ControlRect("Image"); ok {
		t.Error("Image is on the Insert tab")
	}
	small, _ := rtRibbon(360)
	if _, ok := small.ControlRect("Kinds"); ok {
		t.Error("Kinds is folded into the Theme group's pop-up at 360px")
	}
	if rc, ok := small.ControlRect("Theme"); !ok || rc.W == 0 {
		t.Error("the folded group's button is shown under its label")
	}
}
