// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// filler paints its whole rectangle one colour, so a test can say which widget
// owns a pixel.
type filler struct {
	Base
	ink RGBA
}

func (f *filler) Draw(p painter.Painter, _ *Theme) {
	r := f.Bounds()
	p.FillRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, f.ink)
}

// spy records the events it is given.
type spy struct {
	Base
	got []Event
}

func (s *spy) OnEvent(ev Event) { s.got = append(s.got, ev) }

// popoverScene is a form with a DropDown at the top and, right underneath it, a
// widget the open option list must cover: the arrangement that shows whether a
// popover is drawn after the whole tree or merely after its own subtree.
func popoverScene(open bool) (*PopoverHost, *DropDown, *filler, *spy) {
	dd := NewDropDown([]string{"three", "six", "nine", "twelve"}, 0)
	dd.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})
	behind := &filler{ink: RGB(0, 0xFF, 0)}
	behind.SetBounds(Rect{X: 0, Y: 24, W: 200, H: 200})
	watcher := &spy{}
	watcher.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 224})

	box := NewVBox()
	box.Append(dd)
	box.Append(behind)
	box.Append(watcher)
	if open {
		dd.Open().Set(true)
	}
	h := NewPopoverHost(box)
	return h, dd, behind, watcher
}

// renderTree draws a widget into a fresh buffer of the given size.
func renderTree(w Widget, width, height int) []byte {
	buf := make([]byte, width*height*4)
	p := painter.NewPixelPainter(buf, width, height)
	w.Draw(p, DefaultDark())
	return buf
}

// TestPopoverHostDrawsTheOpenListOverTheWidgetsBehindIt is the defect this type
// exists for, with the negative control that proves it was one: the same tree
// without the host paints the form over the option list, so the list is
// invisible and nothing about the widget is at fault.
func TestPopoverHostDrawsTheOpenListOverTheWidgetsBehindIt(t *testing.T) {
	h, dd, behind, _ := popoverScene(true)
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 224})
	// SetBounds re-arranged the box; put the scene back to fixed rows so the
	// popover's expected position is known.
	dd.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})
	behind.SetBounds(Rect{X: 0, Y: 24, W: 200, H: 200})

	pb := dd.PopoverBounds()
	if pb.H <= 0 {
		t.Fatalf("an open dropdown has no popover: %+v", pb)
	}
	mid := pb.Y + pb.H/2
	green := RGB(0, 0xFF, 0)

	withHost := renderTree(h, 200, 224)
	if got := pixelAt(withHost, 200, 4, mid); got.R == green.R && got.G == green.G && got.B == green.B {
		t.Errorf("the widget behind is still showing at y=%d: %v", mid, got)
	}

	// Without the host: the box draws the dropdown, then paints the filler over
	// the very rows the option list needs.
	box := h.Content
	alone := renderTree(box, 200, 224)
	if got := pixelAt(alone, 200, 4, mid); got.R != green.R || got.G != green.G || got.B != green.B {
		t.Errorf("negative control: without the host the widget behind should own "+
			"y=%d, got %v -- the test no longer proves anything", mid, got)
	}
}

// TestPopoverHostRoutesAClickIntoTheOpenList: a click on the third row selects
// the third option. The negative control sends the identical click to the tree
// with no host, where it selects nothing -- which is what a person in front of
// the settings window found.
func TestPopoverHostRoutesAClickIntoTheOpenList(t *testing.T) {
	h, dd, _, _ := popoverScene(true)
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 224})
	dd.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})

	pb := dd.PopoverBounds()
	row := pb.H / len(dd.Options)
	h.OnEvent(Event{Kind: EventClick, X: 10, Y: pb.Y + 2*row + row/2})

	if got := dd.Selected().Get(); got != 2 {
		t.Errorf("clicking the third row selected %d", got)
	}
	if dd.Open().Get() {
		t.Error("the list stayed open after a choice")
	}

	h2, dd2, _, _ := popoverScene(true)
	pb2 := dd2.PopoverBounds()
	h2.Content.OnEvent(Event{Kind: EventClick, X: 10, Y: pb2.Y + 2*row + row/2})
	if got := dd2.Selected().Get(); got == 2 {
		t.Error("negative control: the tree routed the click on its own, so the " +
			"host is not what makes the list clickable")
	}
}

// TestPopoverHostClosesTheListOnAClickElsewhere: a click outside an open list
// dismisses it and does NOT reach the widget it landed on, which is the whole
// meaning of a popover being on top.
func TestPopoverHostClosesTheListOnAClickElsewhere(t *testing.T) {
	h, dd, _, watcher := popoverScene(true)
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 224})
	dd.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})

	h.OnEvent(Event{Kind: EventClick, X: 10, Y: 220})
	if dd.Open().Get() {
		t.Error("a click away from the list left it open")
	}
	if len(watcher.got) != 0 {
		t.Errorf("the click reached the tree behind the open list: %+v", watcher.got)
	}
	if got := dd.Selected().Get(); got != 0 {
		t.Errorf("a click outside the list changed the selection to %d", got)
	}
}

// TestPopoverHostSendsAWheelNotchToTheOpenList, and to the tree when the notch
// is anywhere else.
func TestPopoverHostSendsAWheelNotchToTheOpenList(t *testing.T) {
	dd := NewDropDown([]string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14",
	}, 0)
	watcher := &spy{}
	box := NewVBox()
	box.Append(dd)
	box.Append(watcher)
	h := NewPopoverHost(box)
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})
	dd.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})
	dd.Open().Set(true)
	watcher.SetBounds(Rect{X: 0, Y: 380, W: 200, H: 20})

	pb := dd.PopoverBounds()
	h.OnEvent(Event{Kind: EventScroll, X: 10, Y: pb.Y + pb.H/2, Delta: 2})
	if len(watcher.got) != 0 {
		t.Errorf("the notch over the open list reached the tree behind it: %+v",
			watcher.got)
	}
	if dd.clampedPopScroll() == 0 {
		t.Error("the open list did not scroll")
	}

	// A notch away from the list belongs to whatever is under the pointer.
	h.OnEvent(Event{Kind: EventScroll, X: 10, Y: 390, Delta: 1})
	if len(watcher.got) == 0 {
		t.Error("a notch outside the list never reached the tree")
	}
}

// TestPopoverHostIsTransparentWhenNothingIsOpen: every event reaches the tree,
// and the walks look straight through the wrapper.
func TestPopoverHostIsTransparentWhenNothingIsOpen(t *testing.T) {
	// The spy IS the content, so anything the host forwards is visible: with a
	// box in between, routing to a child is the box's business and not this
	// test's.
	watcher := &spy{}
	h := NewPopoverHost(watcher)
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 224})

	for _, ev := range []Event{
		{Kind: EventClick, X: 5, Y: 5},
		{Kind: EventScroll, X: 5, Y: 5, Delta: 1},
		{Kind: EventKeyDown, Code: "Tab"},
	} {
		h.OnEvent(ev)
	}
	if len(watcher.got) == 0 {
		t.Error("with no popover open the tree received nothing at all")
	}
	if kids := h.Children(); len(kids) != 1 || kids[0] != h.Content {
		t.Errorf("Children yielded %v, want the content", kids)
	}
	if kids := h.focusableChildren(); len(kids) != 1 {
		t.Errorf("focusableChildren yielded %v, want the content", kids)
	}
	if got := h.A11y().Role; got != RolePresentation {
		t.Errorf("the host reports itself as %v, not presentational", got)
	}
	if !h.HitTest(1, 1) || h.HitTest(999, 999) {
		t.Error("HitTest does not cover exactly the host's rectangle")
	}
}

// TestPopoverOwnersFindsTheOpenOnesInTreeOrder, and only those.
func TestPopoverOwnersFindsTheOpenOnesInTreeOrder(t *testing.T) {
	shut := NewDropDown([]string{"a", "b"}, 0)
	first := NewDropDown([]string{"a", "b"}, 0)
	second := NewDropDown([]string{"a", "b"}, 0)
	first.Open().Set(true)
	second.Open().Set(true)

	inner := NewVBox()
	inner.Append(second)
	outer := NewVBox()
	outer.Append(shut)
	outer.Append(first)
	outer.Append(inner)

	got := PopoverOwners(outer)
	if len(got) != 2 {
		t.Fatalf("found %d open lists, want 2", len(got))
	}
	if got[0] != PopoverOwner(first) || got[1] != PopoverOwner(second) {
		t.Error("the open lists came back out of tree order, so the topmost one " +
			"is not the last")
	}
	if owners := PopoverOwners(nil); owners != nil {
		t.Errorf("a nil tree yielded %v", owners)
	}
	if owners := PopoverOwners(shut); owners != nil {
		t.Errorf("a closed dropdown was reported as open: %v", owners)
	}
}

// TestPopoverHostWithNothingInIt does not panic: a window built incrementally
// has an empty root for a frame or two.
func TestPopoverHostWithNothingInIt(t *testing.T) {
	h := NewPopoverHost(nil)
	h.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	h.Draw(painter.NewPixelPainter(make([]byte, 400), 10, 10), DefaultDark())
	h.OnEvent(Event{Kind: EventClick})
	if kids := h.Children(); len(kids) != 0 {
		t.Errorf("an empty host has children: %v", kids)
	}
}

// TestAnOpenListIsDrawnWhereTheHostWasPut checks the coordinate frame: a host
// that is not at the origin must still route and draw in surface coordinates,
// which is what PopoverBounds is expressed in.
func TestAnOpenListIsDrawnWhereTheHostWasPut(t *testing.T) {
	dd := NewDropDown([]string{"a", "b", "c"}, 0)
	box := NewVBox()
	box.Append(dd)
	h := NewPopoverHost(box)
	h.SetBounds(Rect{X: 40, Y: 30, W: 120, H: 90})
	dd.Open().Set(true)

	pb := dd.PopoverBounds()
	if pb.X != dd.Bounds().X {
		t.Fatalf("the popover is at x=%d, the control at x=%d", pb.X, dd.Bounds().X)
	}
	// The event arrives host-local; the host adds its own origin before asking.
	row := pb.H / len(dd.Options)
	h.OnEvent(Event{
		Kind: EventClick,
		X:    pb.X - h.Bounds().X + 5,
		Y:    pb.Y - h.Bounds().Y + row + row/2,
	})
	if got := dd.Selected().Get(); got != 1 {
		t.Errorf("a click on the second row of a displaced host selected %d", got)
	}
}
