// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// go-widgets/toolkit#560: the list of a ComboBox or the calendar of a DatePicker
// hangs out of the control's bounds, so a container routed a click on it to the
// widget that lies there. Both are PopoverOwners now: the host offers the open
// list the click first.

// listScene is a control with a spy right under it, which the open list covers.
func listScene(control Widget) (*PopoverHost, *spy) {
	watcher := &spy{}
	watcher.SetBounds(Rect{Y: 24, W: 300, H: 300})
	col := NewVBox()
	col.AddFixed(control, 24)
	col.AddFixed(watcher, 300)
	h := NewPopoverHost(col)
	h.SetBounds(Rect{W: 300, H: 324})
	return h, watcher
}

func TestAComboBoxListOutsideItsBoundsTakesTheClick(t *testing.T) {
	c := NewComboBox([]string{"alpha", "beta", "gamma"})
	h, under := listScene(c)
	c.Open().Set(true)
	if owners := PopoverOwners(h); len(owners) != 1 {
		t.Fatalf("owners %d", len(owners))
	}
	pb := c.PopoverBounds()
	if pb.Y < c.Bounds().Y+c.Bounds().H {
		t.Fatal("the list is not below the control")
	}
	row := pb.H / 3
	h.OnEvent(Event{Kind: EventClick, X: pb.X + 6, Y: pb.Y + 2*row + row/2})
	if c.Text().Get() != "gamma" || c.Open().Get() {
		t.Errorf("text %q open %v", c.Text().Get(), c.Open().Get())
	}
	if len(under.got) != 0 {
		t.Errorf("the widget under the list got the click: %v", under.got)
	}
	// a click elsewhere closes the list and is not delivered either
	c.Open().Set(true)
	h.OnEvent(Event{Kind: EventClick, X: 250, Y: 200})
	if c.Open().Get() || len(under.got) != 0 {
		t.Errorf("open %v, under got %v", c.Open().Get(), under.got)
	}
	// closed: the host falls through and the click reaches the control
	if c.PopoverClick(5, 5) {
		t.Error("a closed list takes no click")
	}
	if got := c.PopoverOpen(); got {
		t.Error("PopoverOpen")
	}
	c.DrawPopover(painterOver(make([]byte, 300*324*4), 300, 324), DefaultLight()) // closed: nothing
}

func TestADatePickerCalendarOutsideItsBoundsTakesTheClick(t *testing.T) {
	dp := NewDatePicker(2026, 11, 1)
	h, under := listScene(dp)
	dp.Open().Set(true)
	if owners := PopoverOwners(h); len(owners) != 1 {
		t.Fatalf("owners %d", len(owners))
	}
	pb := dp.PopoverBounds()
	_, _, d0 := dp.Date()
	// a day cell in the middle of the grid, well below the control
	h.OnEvent(Event{Kind: EventClick, X: pb.X + pb.W/2, Y: pb.Y + pb.H*2/3})
	if _, _, d := dp.Date(); d == d0 {
		t.Errorf("the day was not chosen (day %d)", d)
	}
	if dp.Open().Get() {
		t.Error("choosing a day left the calendar open")
	}
	if len(under.got) != 0 {
		t.Errorf("the widget under the calendar got the click: %v", under.got)
	}
	dp.Open().Set(true)
	h.OnEvent(Event{Kind: EventClick, X: 290, Y: 310})
	if dp.Open().Get() || len(under.got) != 0 {
		t.Errorf("open %v, under got %v", dp.Open().Get(), under.got)
	}
	if dp.PopoverClick(5, 5) || dp.PopoverOpen() {
		t.Error("a closed calendar takes no click")
	}
}
