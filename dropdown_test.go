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
	d.OnSelect = func(i int) { chosen = i }
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
	d.Open = true
	buf2 := makeSurface(w, h)
	d.DrawPopover(newP(buf2, w), DefaultLight())
	if _, _, mx, _ := nbPaintedBBox(buf2, w, h); mx < 0 {
		t.Fatal("open DropDown.DrawPopover painted nothing")
	}
	// Click option row 1 → selects + fires OnSelect + closes.
	pb := d.PopoverBounds()
	if !d.PopoverClick(pb.X+5, pb.Y+PopoverRowH+2) {
		t.Fatal("PopoverClick inside the popover should return true")
	}
	if d.Selected != 1 || chosen != 1 || d.Open {
		t.Fatalf("after row click: Selected=%d chosen=%d Open=%v", d.Selected, chosen, d.Open)
	}
	// Open again, click outside → just closes.
	d.Open = true
	if !d.PopoverClick(pb.X-100, pb.Y) || d.Open {
		t.Fatalf("outside click should close: Open=%v", d.Open)
	}
}
