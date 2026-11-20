// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func (d *ide) double(t *testing.T, j int) {
	t.Helper()
	r, ok := d.s.HandleRect(j)
	if !ok {
		t.Fatalf("no divider %d", j)
	}
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 1, Y: 10, Code: "double"})
}

// A double-click on a divider whose panes are flexible puts both back to their
// Default weights, and tells OnResized what moved.
func TestSplitterDoubleClickResetsFlexiblePanesToTheirDefaults(t *testing.T) {
	d := newIDE(t, 1200)
	d.editorPane.Default, d.prevPane.Default = 55, 45
	d.drag(t, 1, 150)
	moved := d.sizes()
	d.resized = nil
	d.double(t, 1)
	got := d.sizes()
	if got == moved {
		t.Fatalf("the double-click changed nothing: %v", got)
	}
	if got[1]*45 < got[2]*55-55 || got[1]*45 > got[2]*55+55 {
		t.Fatalf("editor/preview = %d/%d, want 55:45", got[1], got[2])
	}
	if len(d.resized) != 1 {
		t.Fatalf("OnResized calls = %v, want one", d.resized)
	}
	// already at the default: nothing moves, nothing is reported
	d.resized = nil
	d.double(t, 1)
	if d.sizes() != got || len(d.resized) != 0 {
		t.Fatalf("a reset at the default moved %v or reported %v", d.sizes(), d.resized)
	}
}

// A fixed pane returns to its Default size; a pane with no Default is left as
// it is.
func TestSplitterDoubleClickResetsAFixedPaneAndLeavesTheOneWithoutDefault(t *testing.T) {
	d := newIDE(t, 1200)
	d.helpPane.Default = 300
	d.drag(t, 2, -60)
	if d.help.Get() == 300 {
		t.Fatal("the drag did not move Help")
	}
	prev := d.preview.Get()
	d.double(t, 2)
	if d.help.Get() != 300 {
		t.Fatalf("Help = %d, want 300", d.help.Get())
	}
	if d.helpPane.Size != d.help || d.preview.Get() == 0 || prev == 0 {
		t.Fatal("setup")
	}
}

// A divider next to a collapsible pane keeps toggling it, whatever the Defaults.
func TestSplitterDoubleClickStillCollapsesWhenThePaneCan(t *testing.T) {
	d := newIDE(t, 1200)
	d.filesPane.Default, d.editorPane.Default = 300, 1
	d.double(t, 0)
	if !d.s.Collapsed(0) {
		t.Fatal("a collapsible pane's divider must keep collapsing")
	}
}
