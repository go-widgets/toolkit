// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func disabledOverflowList() *ListBox {
	l := NewListBox([]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	l.RowHeight = 20
	l.Reorderable = true
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	return l
}

// A disabled ListBox drops every pointer event, not only keys: the a11y walk
// marks its rows disabled (a11yDisablesSubtree), so a click that still selected
// or activated one would contradict what a screen reader was told.
func TestDisabledListBoxIgnoresClick(t *testing.T) {
	l := disabledOverflowList()
	activated := 0
	l.OnActivate = func(int) { activated++ }
	l.Disabled().Set(true)
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 25})
	if got := l.Selected().Get(); got != -1 || activated != 0 {
		t.Fatalf("a click on a disabled list selected %d, activated %d times", got, activated)
	}
	l.Disabled().Set(false)
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 25})
	if l.Selected().Get() != 1 || activated != 1 {
		t.Fatalf("enabled again, a click selected %d, activated %d times; want 1 and 1", l.Selected().Get(), activated)
	}
}

func TestDisabledListBoxIgnoresClickSectioned(t *testing.T) {
	l := NewSectionedListBox(twoSections()...)
	l.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 6 * 18})
	activated := 0
	l.OnActivate = func(int) { activated++ }
	l.Disabled().Set(true)
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 36})
	if l.Selected().Get() != -1 || activated != 0 {
		t.Fatalf("a click on a disabled sectioned list selected %d, activated %d times", l.Selected().Get(), activated)
	}
}

func TestDisabledListBoxIgnoresScroll(t *testing.T) {
	l := disabledOverflowList()
	l.Disabled().Set(true)
	l.OnEvent(Event{Kind: EventScroll, Delta: 2})
	// the scrollbar track, below the thumb, pages the list
	l.OnEvent(Event{Kind: EventClick, X: 98, Y: 58})
	if got := l.ScrollRow().Get(); got != 0 {
		t.Fatalf("a disabled list scrolled to row %d", got)
	}
	l.Disabled().Set(false)
	l.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if got := l.ScrollRow().Get(); got != 2 {
		t.Fatalf("enabled again, the wheel scrolled to row %d, want 2", got)
	}
}

// A thumb grabbed before the list was disabled is let go: later drags and the
// release move nothing.
func TestDisabledListBoxLetsGoOfTheScrollbarThumb(t *testing.T) {
	l := disabledOverflowList()
	l.OnEvent(Event{Kind: EventClick, X: 98, Y: 2}) // grab the thumb
	l.Disabled().Set(true)
	l.OnEvent(Event{Kind: EventMouseDrag, X: 98, Y: 58})
	if got := l.ScrollRow().Get(); got != 0 {
		t.Fatalf("a drag on a disabled list scrolled to row %d", got)
	}
	l.Disabled().Set(false)
	l.OnEvent(Event{Kind: EventMouseDrag, X: 98, Y: 58})
	if got := l.ScrollRow().Get(); got != 0 {
		t.Fatalf("the grab survived the disabled spell: scrolled to row %d", got)
	}
}

func TestDisabledListBoxIgnoresDragAndDrop(t *testing.T) {
	l := disabledOverflowList()
	l.OnEvent(Event{Kind: EventClick, X: 5, Y: 0}) // press row 0 while enabled
	l.Selected().Set(0)                            // the row being dragged is selected
	payload := l.DragData()
	if !l.AcceptsDrop(payload) {
		t.Fatal("an enabled reorderable list must accept its own payload")
	}
	reordered := false
	l.OnReorder = func(int, int) { reordered = true }
	// control: the same drop on the enabled list does reorder
	ctl := disabledOverflowList()
	ctl.OnEvent(Event{Kind: EventClick, X: 5, Y: 0})
	ctl.Selected().Set(0)
	ctl.OnEvent(Event{Kind: EventDrop, Code: ctl.DragData(), Y: 55})
	if ctl.Items[0] != "b" {
		t.Fatalf("control drop did not reorder: %v", ctl.Items)
	}
	l.Disabled().Set(true)
	if l.AcceptsDrop(payload) {
		t.Error("a disabled list accepts a drop")
	}
	l.OnEvent(Event{Kind: EventDrop, Code: payload, Y: 55})
	if reordered || l.Items[0] != "a" {
		t.Errorf("a drop reordered a disabled list: %v", l.Items)
	}
	l.OnEvent(Event{Kind: EventDragMove, Y: 30})
	if l.dropIndicator != -1 {
		t.Errorf("a disabled list shows a drop indicator at %d", l.dropIndicator)
	}
}

func TestDisabledIsoDiagramRefusesDrops(t *testing.T) {
	d := NewIsoDiagram(nil)
	p := EncodeIsoIconPayload("server")
	d.Disabled().Set(true)
	if d.AcceptsDrop(p) {
		t.Fatal("a disabled diagram accepts an icon drop")
	}
	d.Disabled().Set(false)
	if !d.AcceptsDrop(p) {
		t.Fatal("an enabled diagram must accept it again")
	}
}
