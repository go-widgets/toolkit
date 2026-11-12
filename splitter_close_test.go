// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// closeIDE is the playground's shape: Files and Help fixed, an editor and a
// preview flexible and Essential, each in a ClosablePane bound to the splitter.
type closeIDE struct {
	s     *Splitter
	panes []*ClosablePane
	tree  *Entry
}

func newCloseIDE(t *testing.T) *closeIDE {
	t.Helper()
	d := &closeIDE{tree: NewEntry("")}
	names := []string{"Files", "Editor", "Preview", "Help"}
	sp := []*SplitPane{
		{Name: "Files", Size: mvvm.NewObservable(190), Min: 120, Max: 480, Collapsible: true},
		{Name: "Editor", Weight: mvvm.NewObservable(600), Min: 200, Essential: true},
		{Name: "Preview", Weight: mvvm.NewObservable(500), Min: 200, Essential: true},
		{Name: "Help", Size: mvvm.NewObservable(280), Min: 180, Max: 560},
	}
	for i, p := range sp {
		var body Widget = NewLabel(names[i])
		if i == 0 {
			body = d.tree
		}
		cp := NewClosablePane(names[i], body)
		cp.CloseLabel, cp.LastLabel = "Close "+names[i], "last pane"
		d.panes = append(d.panes, cp)
		p.Widget = cp
	}
	d.s = NewSplitter(false, sp...)
	for i, cp := range d.panes {
		cp.Bind(d.s, i)
	}
	d.s.SetBounds(Rect{W: 1200, H: 400})
	return d
}

func (d *closeIDE) sizes() [4]int {
	return [4]int{d.s.PaneSize(0), d.s.PaneSize(1), d.s.PaneSize(2), d.s.PaneSize(3)}
}

func TestSplitterClosePaneFillsAndRestores(t *testing.T) {
	d := newCloseIDE(t)
	before := d.sizes()
	if d.s.Panes() != 4 || d.s.PaneName(1) != "Editor" || !d.s.IsOpen(2) {
		t.Fatal("accessors")
	}
	if !d.s.Close(2) {
		t.Fatal("the preview refused to close beside an open editor")
	}
	if d.s.IsOpen(2) || d.s.PaneSize(2) != 0 || d.panes[2].Bounds() != (Rect{}) {
		t.Fatal("a closed pane keeps room")
	}
	// the editor takes what the preview left (the dividers count one fewer)
	sum := 0
	for _, v := range d.sizes() {
		sum += v
	}
	if sum+2*SplitterHandleW != 1200 {
		t.Fatalf("panes and dividers fill %d of 1200", sum+2*SplitterHandleW)
	}
	if d.sizes()[1] <= before[1] {
		t.Fatalf("editor %d did not grow from %d", d.sizes()[1], before[1])
	}
	if d.s.Close(2) {
		t.Fatal("closing a closed pane reports a change")
	}
	d.s.Reopen(2)
	if d.sizes() != before {
		t.Fatalf("reopened at %v, were %v", d.sizes(), before)
	}
}

func TestSplitterLastEssentialPaneRefuses(t *testing.T) {
	d := newCloseIDE(t)
	d.s.Close(1)
	if d.s.CanClose(2) || d.s.Close(2) {
		t.Fatal("the last editor/preview pane agreed to close")
	}
	if !d.s.IsOpen(2) {
		t.Fatal("refused, but closed")
	}
	// a non-essential pane closes freely, the last essential one does not
	if !d.s.Close(0) || !d.s.Close(3) {
		t.Fatal("a non-essential pane refused")
	}
	if !d.panes[2].Button.Disabled().Get() || d.panes[2].Tip() != "last pane" {
		t.Fatal("the last pane's button is not disabled with its reason")
	}
	d.s.Reopen(1)
	if d.panes[2].Button.Disabled().Get() || d.panes[2].Tip() != "Close Preview" {
		t.Fatal("the button did not come back")
	}
}

func TestSplitterRefusesWhoeverWritesTheObservable(t *testing.T) {
	d := newCloseIDE(t)
	d.s.Open(1).Set(false) // a menu toggle bound to the same Observable
	d.s.Open(2).Set(false)
	if !d.s.IsOpen(2) || d.s.IsOpen(1) {
		t.Fatalf("open: editor %v preview %v", d.s.IsOpen(1), d.s.IsOpen(2))
	}
	if d.s.PaneSize(2) == 0 {
		t.Fatal("the pane that stayed has no room")
	}
}

func TestSplitterWithoutEssentialKeepsAnyPane(t *testing.T) {
	a := &SplitPane{Widget: NewLabel("a"), Name: "a", Weight: mvvm.NewObservable(1)}
	b := &SplitPane{Widget: NewLabel("b"), Name: "b", Weight: mvvm.NewObservable(1)}
	s := NewSplitter(false, a, b)
	s.SetBounds(Rect{W: 400, H: 100})
	if !s.Close(0) || s.Close(1) || s.CanClose(1) {
		t.Fatal("the last of two panes must stay")
	}
	if s.CanClose(0) {
		t.Fatal("a closed pane cannot be closed")
	}
}

func TestSplitterClosedPaneNeverKeepsFocus(t *testing.T) {
	d := newCloseIDE(t)
	d.tree.SetFocused(true)
	d.panes[0].Button.SetFocused(true)
	d.s.FocusHandle(0)
	d.s.Close(0)
	if d.tree.Focused() || d.panes[0].Button.Focused() || d.s.FocusedHandle() != -1 {
		t.Fatal("the closed pane or its divider kept the focus")
	}
	d.s.Reopen(0)
	if d.tree.Focused() || d.s.FocusedHandle() != -1 {
		t.Fatal("a stale focus came back with the pane")
	}
	// Tab never lands in a closed pane
	d.s.Close(3)
	for range 8 {
		d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		if d.panes[3].Button.Focused() {
			t.Fatal("Tab reached a closed pane")
		}
	}
}

func TestSplitterRefusedCloseKeepsFocus(t *testing.T) {
	d := newCloseIDE(t)
	d.s.Close(1)
	d.tree.SetFocused(true)
	d.s.Open(2).Set(false) // refused: nothing may be released
	if !d.tree.Focused() {
		t.Fatal("a refused close dropped the focus elsewhere")
	}
}

func TestClosablePaneButtonClosesAndTabReachesIt(t *testing.T) {
	d := newCloseIDE(t)
	// Tab walks: the first focusable is Files' close button
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if !d.panes[0].Button.Focused() {
		t.Fatal("Tab did not reach the first close button")
	}
	if got := d.panes[0].Button.A11y(); got.Role != RoleButton || got.Name != "Close Files" {
		t.Fatalf("a11y %+v", got)
	}
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if d.s.IsOpen(0) {
		t.Fatal("Enter on the focused button did not close the pane")
	}
	// a click on another pane's button
	b := d.panes[3].Button.Bounds()
	d.s.OnEvent(Event{Kind: EventClick, X: b.X + b.W/2, Y: b.Y + b.H/2})
	if d.s.IsOpen(3) {
		t.Fatal("a click on the button did not close the pane")
	}
	if d.panes[3].Button.pressed || d.panes[3].Button.hovered {
		t.Fatal("the closed pane's button stays pressed: it comes back drawn so")
	}
	d.s.Reopen(3)
	if d.panes[3].Button.pressed || d.panes[3].Button.hovered || d.panes[3].hover {
		t.Fatal("the reopened pane's button is still pressed")
	}
	// a click in a body goes to the body
	d.s.Reopen(0)
	bb := d.panes[0].Body.Bounds()
	d.s.OnEvent(Event{Kind: EventClick, X: bb.X + 5, Y: bb.Y + 5})
	if !d.tree.Focused() {
		t.Fatal("a click in the body did not reach it")
	}
	// a click in the header, off the button, reaches nothing
	d.s.OnEvent(Event{Kind: EventClick, X: 3, Y: 3})
}

func TestClosablePaneTooltip(t *testing.T) {
	d := newCloseIDE(t)
	cp := d.panes[1]
	b := cp.Button.Bounds()
	if cp.PopoverOpen() || cp.Animating() {
		t.Fatal("a tip before any hover")
	}
	cp.OnEvent(Event{Kind: EventMouseMove, X: b.X + b.W/2 - cp.Bounds().X, Y: b.Y + b.H/2})
	if !cp.Animating() || cp.PopoverOpen() {
		t.Fatal("the tip must wait for its delay")
	}
	cp.Tick(0.2)
	if cp.PopoverOpen() {
		t.Fatal("tip too early")
	}
	cp.Tick(0.3)
	if !cp.PopoverOpen() || cp.Animating() || cp.PopoverClick(0, 0) {
		t.Fatal("the tip should show and take no click")
	}
	if box := cp.PopoverBounds(); box.W <= 0 || box.H <= 0 {
		t.Fatalf("tip box %+v", box)
	}
	buf := make([]byte, 1200*400*4)
	pp := painter.NewPixelPainter(buf, 1200, 400)
	cp.DrawPopover(pp, DefaultLight())
	// moving within the button keeps the timer, moving off clears it
	cp.OnEvent(Event{Kind: EventMouseMove, X: b.X + b.W/2 - cp.Bounds().X, Y: b.Y + b.H/2})
	if !cp.PopoverOpen() {
		t.Fatal("a move on the button restarted the timer")
	}
	cp.OnEvent(Event{Kind: EventMouseMove, X: 2, Y: 2})
	if cp.PopoverOpen() {
		t.Fatal("tip stays after the pointer left")
	}
	// keyboard: a focused button shows its tip at once
	cp.Button.SetFocused(true)
	if !cp.PopoverOpen() {
		t.Fatal("a focused button shows no tip")
	}
	// no words, no tip
	cp.CloseLabel = ""
	if cp.PopoverOpen() {
		t.Fatal("an empty tip showed")
	}
	cp.Refresh()
	if cp.Button.Icon != "" {
		t.Fatal("Refresh did not take the label")
	}
	// the reason shows on a disabled button
	d.s.Close(2)
	cp.CloseLabel = "Close Editor"
	cp.Refresh()
	if cp.Tip() != "last pane" || cp.Button.Focused() {
		t.Fatalf("tip %q, focused %v: a disabled button keeps neither", cp.Tip(), cp.Button.Focused())
	}
	// keys reach the pane's own focus walk
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
}

func TestClosablePaneDrawAndLayout(t *testing.T) {
	d := newCloseIDE(t)
	buf := make([]byte, 1200*400*4)
	d.s.Draw(painter.NewPixelPainter(buf, 1200, 400), DefaultLight())
	cp := d.panes[1]
	r := cp.Bounds()
	if cb := cp.Body.Bounds(); cb.Y != r.Y+scaled(ClosablePaneHeaderH) || cb.H != r.H-scaled(ClosablePaneHeaderH) {
		t.Fatalf("body %+v in %+v", cb, r)
	}
	if bb := cp.Button.Bounds(); bb.X+bb.W > r.X+r.W || bb.Y < r.Y || bb.Y+bb.H > r.Y+scaled(ClosablePaneHeaderH) {
		t.Fatalf("button %+v outside the header of %+v", bb, r)
	}
	if got := cp.A11y(); got.Role != RoleGroup || got.Name != "Editor" {
		t.Fatalf("a11y %+v", got)
	}
	if len(cp.Children()) != 2 || len(cp.focusableChildren()) != 2 {
		t.Fatal("children")
	}
	// shorter than its header, and empty
	cp.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	cp.Draw(painter.NewPixelPainter(buf, 1200, 400), DefaultLight())
	cp.SetBounds(Rect{})
	cp.Draw(painter.NewPixelPainter(buf, 1200, 400), DefaultLight())
	if cp.Button.Bounds() != (Rect{}) || cp.Body.Bounds() != (Rect{}) {
		t.Fatal("an empty pane keeps its parts in reach")
	}
	// a pane without a body, never bound
	nb := NewClosablePane("x", nil)
	nb.SetBounds(Rect{W: 100, H: 100})
	nb.Draw(painter.NewPixelPainter(buf, 1200, 400), DefaultLight())
	nb.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 50})
	nb.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	nb.Tick(1)
	nb.SetBounds(Rect{})
	nb.Refresh()
	if len(nb.Children()) != 1 {
		t.Fatal("a bodiless pane has one child")
	}
	nb.Button.OnClick = nil
	nb.Button.OnEvent(Event{Kind: EventClick})
}

func TestBlurTreeSkipsNil(t *testing.T) {
	blurTree(NewLabel("x"))
	v := NewVBox()
	in := NewEntry("")
	in.SetFocused(true)
	v.AddFixed(in, 10)
	blurTree(v)
	if in.Focused() {
		t.Fatal("blurTree left a focus")
	}
}
