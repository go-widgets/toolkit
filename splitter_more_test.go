// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// recorder is a pane that keeps the events it was given.
type recorder struct {
	Base
	got []Event
}

func (r *recorder) OnEvent(ev Event) { r.got = append(r.got, ev) }

func TestSplitterForwardsEventsToThePaneUnderThePointer(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	s := NewSplitter(false,
		&SplitPane{Widget: a, Weight: mvvm.NewObservable(1), Min: 10},
		&SplitPane{Widget: b, Weight: mvvm.NewObservable(1), Min: 10})
	s.SetBounds(Rect{X: 20, Y: 30, W: 210, H: 100})
	// a press in the second pane arrives in the pane's own coordinates
	bx := b.Bounds().X
	s.OnEvent(Event{Kind: EventClick, X: bx - 20 + 15, Y: 40})
	if len(b.got) != 1 || b.got[0].X != 15 || b.got[0].Y != 40 || len(a.got) != 0 {
		t.Fatalf("pane events: a=%v b=%v", a.got, b.got)
	}
	// a move goes to every pane, and hovers the divider it is over
	hx, _ := s.HandleRect(0)
	s.OnEvent(Event{Kind: EventMouseMove, X: hx.X - 20 + 2, Y: 5})
	if len(a.got) != 1 || len(b.got) != 2 {
		t.Fatalf("a move reaches every pane: a=%d b=%d", len(a.got), len(b.got))
	}
	if s.hover != 0 {
		t.Fatalf("the divider under the pointer is hovered: %d", s.hover)
	}
	s.OnEvent(Event{Kind: EventMouseMove, X: 3, Y: 5})
	if s.hover != -1 {
		t.Fatal("moving off the divider clears the hover")
	}
	// a release nobody grabbed, and a drag with no divider held, go to the pane
	s.OnEvent(Event{Kind: EventMouseUp, X: 3, Y: 5})
	s.OnEvent(Event{Kind: EventMouseDrag, X: 3, Y: 5})
	if len(a.got) != 4 { // two moves, a release and a drag
		t.Fatalf("a release and a drag outside a drag are the pane's: %d", len(a.got))
	}
	// the press that missed every pane and divider is dropped
	s.OnEvent(Event{Kind: EventClick, X: 5000, Y: 5})
}

func TestSplitterOnlyFixedPanesAndAFlexiblePaneAtZero(t *testing.T) {
	a, b := NewLabel("a"), NewLabel("b")
	sa, sb := mvvm.NewObservable(100), mvvm.NewObservable(100)
	s := NewSplitter(false,
		&SplitPane{Widget: a, Size: sa, Min: 20},
		&SplitPane{Widget: b, Size: sb, Min: 20})
	s.SetBounds(Rect{W: 400, H: 50})
	if s.PaneSize(0) != 100 || s.PaneSize(1) != 100 {
		t.Fatalf("fixed panes keep their size: %d %d", s.PaneSize(0), s.PaneSize(1))
	}
	// two fixed panes: the divider moves one and takes from the other
	r, _ := s.HandleRect(0)
	s.OnEvent(Event{Kind: EventClick, X: r.X + 2, Y: 5})
	s.OnEvent(Event{Kind: EventMouseDrag, X: r.X + 2 + 30, Y: 5})
	s.OnEvent(Event{Kind: EventMouseUp, X: r.X + 2 + 30, Y: 5})
	if sa.Get() != 130 || sb.Get() != 70 {
		t.Fatalf("a divider between two fixed panes: %d and %d", sa.Get(), sb.Get())
	}

	// a flexible pane with no Min can be dragged to nothing
	left, right := NewLabel("l"), NewLabel("r")
	size := mvvm.NewObservable(100)
	s2 := NewSplitter(false,
		&SplitPane{Widget: left, Size: size, Min: 20},
		&SplitPane{Widget: right, Weight: mvvm.NewObservable(1)})
	s2.SetBounds(Rect{W: 300, H: 50})
	s2.FocusHandle(0)
	s2.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if s2.PaneSize(1) != 0 || size.Get() != 290 {
		t.Fatalf("End leaves nothing for a pane without a Min: %d, %d", s2.PaneSize(1), size.Get())
	}
}

func TestSplitterWithoutCollapsiblePaneDoubleClickIsInert(t *testing.T) {
	a, b := NewLabel("a"), NewLabel("b")
	s := NewSplitter(false,
		&SplitPane{Widget: a, Weight: mvvm.NewObservable(1), Min: 10},
		&SplitPane{Widget: b, Weight: mvvm.NewObservable(1), Min: 10})
	s.SetBounds(Rect{W: 200, H: 50})
	before := s.PaneSize(0)
	s.FocusHandle(0)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // nothing to collapse
	if s.PaneSize(0) != before {
		t.Fatal("Enter on a divider between flexible panes does nothing")
	}
}

func TestSplitterBackFromZeroUsesTheMin(t *testing.T) {
	d := newIDE(t, 1200)
	d.files.Set(0) // stored collapsed: no size to come back to
	if !d.s.Collapsed(0) {
		t.Fatal("Size 0 on a collapsible pane is collapsed")
	}
	d.s.FocusHandle(0)
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if d.files.Get() != 120 {
		t.Fatalf("with nothing to restore, Enter gives the pane its Min: %d", d.files.Get())
	}
}

func TestSplitterTheDividerHeldWhileItsPaneGoesAway(t *testing.T) {
	d := newIDE(t, 1200)
	r, _ := d.s.HandleRect(0)
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 2, Y: 5})
	d.filesOn.Set(false)
	d.s.OnEvent(Event{Kind: EventMouseDrag, X: r.X + 40, Y: 5}) // nothing to move
	if d.files.Get() != 190 {
		t.Fatalf("a drag of a divider that went away moves nothing: %d", d.files.Get())
	}
	d.s.OnEvent(Event{Kind: EventMouseUp, X: r.X + 40, Y: 5})
	if d.s.Dragging().Get() {
		t.Fatal("the release ends the drag")
	}
}

func TestSplitterAccessibilityOfAnUnusedDividerAndOfTheSplitter(t *testing.T) {
	d := newIDE(t, 1200)
	d.filesOn.Set(false)
	off := d.s.Handle(2).(Accessible).A11y()
	if off.Role != RoleSeparator || off.HasRange {
		t.Fatalf("an unused divider has a role and no value: %+v", off)
	}
	d.s.Handle(2).OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // ignored
	flex := d.s.Handle(0).(Accessible).A11y()
	if flex.Name != "Resize Editor" || flex.Min != 200 || flex.Max < 900 {
		t.Fatalf("a pane with no Max reports the whole length: %+v", flex)
	}
	if d.s.A11y().Role != RolePresentation || d.s.Handles() != 3 {
		t.Fatal("the splitter is a plain container with three dividers")
	}
}

func TestSplitterFocusHandleRefusesWhatIsNotThere(t *testing.T) {
	d := newIDE(t, 1200)
	d.filesOn.Set(false)
	if d.s.FocusHandle(-1) || d.s.FocusHandle(2) || d.s.FocusHandle(9) {
		t.Fatal("no divider there to focus")
	}
}

func TestSplitterThicknessAndStepAreSettable(t *testing.T) {
	d := newIDE(t, 1200)
	d.s.Thickness, d.s.Step = 14, 8
	d.s.SetBounds(d.s.Bounds())
	if r, _ := d.s.HandleRect(0); r.W != 14 {
		t.Fatalf("Thickness: %d", r.W)
	}
	d.s.FocusHandle(0)
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if d.files.Get() != 198 {
		t.Fatalf("Step: %d", d.files.Get())
	}
}

func TestSplitterEmptyBoundsLeaveNothing(t *testing.T) {
	d := newIDE(t, 1200)
	d.s.SetBounds(Rect{})
	if d.fw.Bounds() != (Rect{}) || d.ew.Bounds() != (Rect{}) || d.s.PaneSize(1) != 0 {
		t.Fatal("an empty splitter empties its panes")
	}
	if _, ok := d.s.HandleRect(0); ok {
		t.Fatal("and has no divider")
	}
}

func TestSplitterDrawsHotAndVerticalDividers(t *testing.T) {
	th := DefaultDark()
	d := newIDE(t, 1200)
	paint := func(s *Splitter, w, h int) []byte {
		buf := make([]byte, w*h*4)
		s.Draw(painter.NewPixelPainter(buf, w, h), th)
		return buf
	}
	rest := paint(d.s, 1200, 400)
	r, _ := d.s.HandleRect(0)
	d.s.OnEvent(Event{Kind: EventMouseMove, X: r.X + 2, Y: 30})
	hot := paint(d.s, 1200, 400)
	_, active := SplitterColors(th)
	px := (30*1200 + r.X + r.W/2) * 4
	if hot[px] != active.R || hot[px+1] != active.G || hot[px+2] != active.B {
		t.Fatalf("a hovered divider paints in the accent: %v want %v", hot[px:px+3], active)
	}
	if string(rest) == string(hot) {
		t.Fatal("hover changes the picture")
	}

	top, bottom := NewLabel("t"), NewLabel("b")
	v := NewSplitter(true,
		&SplitPane{Widget: top, Weight: mvvm.NewObservable(1), Min: 10},
		&SplitPane{Widget: bottom, Weight: mvvm.NewObservable(1), Min: 10})
	v.SetBounds(Rect{W: 100, H: 100})
	v.FocusHandle(0)
	vr, _ := v.HandleRect(0)
	img := paint(v, 100, 100)
	ink, _ := SplitterColors(th)
	vpx := ((vr.Y+vr.H/2)*100 + 10) * 4
	if img[vpx] != ink.R || img[vpx+1] != ink.G || img[vpx+2] != ink.B {
		t.Fatalf("a horizontal bar paints its line across: %v want %v", img[vpx:vpx+3], ink)
	}
}

func TestSplitterColorsFallBackToTheInkWhenNothingReachesThreeToOne(t *testing.T) {
	grey := RGB(0x80, 0x80, 0x80)
	th := &Theme{Background: grey, Surface: grey, OnBackground: grey, Border: grey, Accent: grey}
	ink, active := SplitterColors(th)
	if ink != grey || active != grey {
		t.Fatalf("with no contrast to be had the ink is returned: %v %v", ink, active)
	}
}

func TestSplitterWeightsWithNoFlexiblePaneLeftToWrite(t *testing.T) {
	left, right := NewLabel("l"), NewLabel("r")
	size := mvvm.NewObservable(100)
	w := mvvm.NewObservable(1)
	s := NewSplitter(false,
		&SplitPane{Widget: left, Size: size, Min: 20},
		&SplitPane{Widget: right, Weight: w, Min: 0})
	s.SetBounds(Rect{W: 300, H: 50})
	// both flexible weights are written together, so a pane at zero is allowed
	s.FocusHandle(0)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if s.PaneSize(0) != 20 || w.Get() < 1 {
		t.Fatalf("Home gives the fixed pane its Min: %d, weight %d", s.PaneSize(0), w.Get())
	}
}

func TestSplitterHandleAt(t *testing.T) {
	d := newIDE(t, 1200)
	r, _ := d.s.HandleRect(1)
	if d.s.HandleAt(r.X+1, 5) != 1 || d.s.HandleAt(r.X-20, 5) != -1 {
		t.Fatal("HandleAt finds the divider under a surface point")
	}
}

func TestSplitterCollapsesAPaneAfterItsDivider(t *testing.T) {
	d := newIDE(t, 1200)
	d.helpPane.Collapsible = true
	d.s.FocusHandle(2)
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !d.s.Collapsed(3) || d.help.Get() != 0 || d.s.PaneSize(3) != 0 {
		t.Fatalf("Enter collapses the pane after the divider: %d", d.help.Get())
	}
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if d.s.Collapsed(3) || d.help.Get() != 280 {
		t.Fatalf("and brings it back: %d", d.help.Get())
	}
	// a divider that is off draws nothing
	d.filesOn.Set(false)
	buf := make([]byte, 1200*400*4)
	d.s.Draw(painter.NewPixelPainter(buf, 1200, 400), DefaultLight())
}
