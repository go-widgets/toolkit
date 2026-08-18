// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestDropDownPopoverHelpers covers the self-contained DrawPopover / PopoverClick
// the host uses instead of hand-rolling the popover.
func TestDropDownPopoverHelpers(t *testing.T) {
	chosen := -1
	d := NewDropDown([]string{"UTF-8", "Latin-1", "Shift-JIS"}, 0)
	d.Selected().Subscribe(func(i int) { chosen = i })
	d.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 22})

	// Closed: DrawPopover paints nothing, PopoverClick declines.
	const w, h = 140, 120
	buf := makeSurface(w, h)
	d.DrawPopover(newP(buf, w), DefaultLight())
	if _, _, mx, _ := nbPaintedBBox(buf, w, h); mx >= 0 {
		t.Fatal("closed DropDown.DrawPopover painted something")
	}
	if d.PopoverClick(20, 40) {
		t.Fatal("closed DropDown.PopoverClick should return false")
	}

	// Open: DrawPopover paints the list.
	d.Open().Set(true)
	buf2 := makeSurface(w, h)
	d.DrawPopover(newP(buf2, w), DefaultLight())
	if _, _, mx, _ := nbPaintedBBox(buf2, w, h); mx < 0 {
		t.Fatal("open DropDown.DrawPopover painted nothing")
	}
	// Click option row 1 → Sets Selected (notifying the subscriber) + closes.
	pb := d.PopoverBounds()
	if !d.PopoverClick(pb.X+5, pb.Y+PopoverRowH+2) {
		t.Fatal("PopoverClick inside the popover should return true")
	}
	if d.Selected().Get() != 1 || chosen != 1 || d.Open().Get() {
		t.Fatalf("after row click: Selected=%d chosen=%d Open=%v", d.Selected().Get(), chosen, d.Open().Get())
	}
	// Open again, click outside → just closes.
	d.Open().Set(true)
	if !d.PopoverClick(pb.X-100, pb.Y) || d.Open().Get() {
		t.Fatalf("outside click should close: Open=%v", d.Open().Get())
	}
}

// TestDropDownBareAccessors exercises the nil→init lazy path of both accessors
// on a zero-value &DropDown{} (no constructor), and that a host binding via
// Subscribe on each observable observes the widget's own mutations.
func TestDropDownBareAccessors(t *testing.T) {
	d := &DropDown{}
	// Nil observables initialise to the zero value on first access.
	if got := d.Selected().Get(); got != 0 {
		t.Fatalf("bare Selected().Get() = %d, want 0", got)
	}
	if d.Open().Get() {
		t.Fatal("bare Open().Get() = true, want false")
	}
	// Host binds both; the widget's own Set paths notify.
	var gotSel int
	var gotOpen bool
	d.Selected().Subscribe(func(v int) { gotSel = v })
	d.Open().Subscribe(func(v bool) { gotOpen = v })
	d.Options = []string{"a", "b", "c"}
	d.Select(2)
	if d.Selected().Get() != 2 || gotSel != 2 {
		t.Fatalf("after Select(2): Selected=%d host=%d", d.Selected().Get(), gotSel)
	}
	if d.Open().Get() {
		t.Fatal("Select should close the popover")
	}
	d.Open().Set(true)
	if !gotOpen {
		t.Fatal("host did not observe Open().Set(true)")
	}
}
