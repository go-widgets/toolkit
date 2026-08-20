// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Compile-time proof that every interactive widget in the Wave-2 set satisfies
// Focusable (i.e. embeds focusState). A widget that stops embedding it — or a
// display widget that starts — breaks the build here.
var (
	_ Focusable = (*Button)(nil)
	_ Focusable = (*IconButton)(nil)
	_ Focusable = (*ToggleButton)(nil)
	_ Focusable = (*SplitButton)(nil)
	_ Focusable = (*CheckButton)(nil)
	_ Focusable = (*RadioButton)(nil)
	_ Focusable = (*Switch)(nil)
	_ Focusable = (*Scale)(nil)
	_ Focusable = (*RangeSlider)(nil)
	_ Focusable = (*SpinButton)(nil)
	_ Focusable = (*DropDown)(nil)
	_ Focusable = (*ComboBox)(nil)
	_ Focusable = (*Entry)(nil)
	_ Focusable = (*SearchEntry)(nil)
	_ Focusable = (*ListBox)(nil)
	_ Focusable = (*Table)(nil)
	_ Focusable = (*TreeView)(nil)
	_ Focusable = (*TreeTable)(nil)
	_ Focusable = (*Notebook)(nil)
	_ Focusable = (*ViewSwitcher)(nil)
	_ Focusable = (*Pagination)(nil)
	_ Focusable = (*Calendar)(nil)
	_ Focusable = (*TagField)(nil)
	_ Focusable = (*Rating)(nil)
	_ Focusable = (*CycleButton)(nil)
)

// Display / non-interactive widgets must NOT be focusable: they neither embed
// focusState nor get visited by container traversal. recordingWidget stands in
// for the whole non-interactive set.
func TestDisplayWidgetIsNotFocusable(t *testing.T) {
	var w Widget = &recordingWidget{}
	if _, ok := w.(Focusable); ok {
		t.Fatal("a display widget must not satisfy Focusable")
	}
}

// Every focusable widget toggles its flag through the Focusable methods.
func TestEachWidgetFocusToggles(t *testing.T) {
	widgets := []Focusable{
		&Button{}, &IconButton{}, &ToggleButton{}, &SplitButton{}, &CheckButton{},
		&RadioButton{}, &Switch{}, &Scale{}, &RangeSlider{}, &SpinButton{},
		&DropDown{}, &ComboBox{}, &Entry{}, &SearchEntry{}, &ListBox{},
		&Table{}, &TreeView{}, &TreeTable{}, &Notebook{}, &ViewSwitcher{},
		&Pagination{}, &Calendar{}, &TagField{}, &Rating{}, &CycleButton{},
	}
	for i, w := range widgets {
		if w.Focused() {
			t.Fatalf("widget %d focused before SetFocused", i)
		}
		w.SetFocused(true)
		if !w.Focused() {
			t.Fatalf("widget %d not focused after SetFocused(true)", i)
		}
		w.SetFocused(false)
		if w.Focused() {
			t.Fatalf("widget %d still focused after SetFocused(false)", i)
		}
	}
}

// The focus ring paints in the theme Accent only when the widget holds focus,
// leaving the unfocused render untouched.
func TestFocusRingDrawnOnlyWhenFocused(t *testing.T) {
	th := DefaultLight()
	const w, h = 120, 40
	b := NewButton("OK", nil)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	unfocused := makeSurface(w, h)
	b.Draw(newP(unfocused, w), th)
	if got := pixelAt(unfocused, w, w/2, 1); got == th.Accent {
		t.Fatalf("unfocused ring pixel = %+v, must not be Accent", got)
	}

	b.SetFocused(true)
	focused := makeSurface(w, h)
	b.Draw(newP(focused, w), th)
	if got := pixelAt(focused, w, w/2, 1); got != th.Accent {
		t.Fatalf("focused ring pixel = %+v, want Accent", got)
	}
}

// focusW is a focusable test leaf: it embeds focusState (so it is Focusable and
// draws a ring) and records the events routed to it, so container tests can
// assert exactly which descendant received a key/char.
type focusW struct {
	Base
	focusState
	events []Event
}

func (f *focusW) OnEvent(ev Event) { f.events = append(f.events, ev) }

func newFocusW(x, y, w, h int) *focusW {
	fw := &focusW{}
	fw.SetBounds(Rect{X: x, Y: y, W: w, H: h})
	return fw
}

// which returns the index of the single focused widget in ws, or -1, and fails
// if more than one is focused (the single-focus invariant).
func which(t *testing.T, ws ...*focusW) int {
	t.Helper()
	idx := -1
	for i, w := range ws {
		if w.Focused() {
			if idx != -1 {
				t.Fatalf("two widgets focused at once: %d and %d", idx, i)
			}
			idx = i
		}
	}
	return idx
}

func vboxOf(ws ...*focusW) *VBox {
	v := NewVBox()
	for _, w := range ws {
		v.Append(w)
	}
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 90})
	return v
}

func TestTabTraversalForwardWraps(t *testing.T) {
	a, b, c := newFocusW(0, 0, 100, 30), newFocusW(0, 30, 100, 30), newFocusW(0, 60, 100, 30)
	v := vboxOf(a, b, c)
	// Nothing focused yet: Tab focuses the first.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if which(t, a, b, c) != 0 {
		t.Fatalf("first Tab should focus index 0, got %d", which(t, a, b, c))
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if which(t, a, b, c) != 2 {
		t.Fatalf("after three Tabs want index 2, got %d", which(t, a, b, c))
	}
	// Wrap last -> first.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if which(t, a, b, c) != 0 {
		t.Fatalf("Tab from last should wrap to 0, got %d", which(t, a, b, c))
	}
}

func TestShiftTabTraversalBackwardWraps(t *testing.T) {
	a, b, c := newFocusW(0, 0, 100, 30), newFocusW(0, 30, 100, 30), newFocusW(0, 60, 100, 30)
	v := vboxOf(a, b, c)
	// Nothing focused: Shift+Tab focuses the last.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Shift+Tab"})
	if which(t, a, b, c) != 2 {
		t.Fatalf("first Shift+Tab should focus last, got %d", which(t, a, b, c))
	}
	// Backward wrap last->...->first->last using the shifted-Tab spelling.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab", Shift: true})
	if which(t, a, b, c) != 1 {
		t.Fatalf("shifted Tab want index 1, got %d", which(t, a, b, c))
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab", Shift: true})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab", Shift: true})
	if which(t, a, b, c) != 2 {
		t.Fatalf("shifted Tab should wrap 0->last, got %d", which(t, a, b, c))
	}
}

func TestTabSkipsDisabledAndHidden(t *testing.T) {
	a := newFocusW(0, 0, 100, 30)
	b := newFocusW(0, 30, 100, 30)
	b.Disabled().Set(true)          // Wave-1 disabled: skipped by traversal
	hidden := newFocusW(0, 0, 0, 0) // empty bounds: skipped
	c := newFocusW(0, 60, 100, 30)
	// A nil-layout Container keeps each child's explicit bounds (no Arrange),
	// so `hidden` stays at zero size and `b` stays disabled.
	cont := NewContainer(nil)
	for _, w := range []*focusW{a, b, hidden, c} {
		cont.AddWidget(w)
	}
	cont.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 90})
	cont.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // -> a
	if which(t, a, b, hidden, c) != 0 {
		t.Fatalf("first Tab should focus a, got %d", which(t, a, b, hidden, c))
	}
	cont.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // skip b + hidden -> c
	if b.Focused() || hidden.Focused() {
		t.Fatal("disabled / hidden widget must never take focus")
	}
	if which(t, a, b, hidden, c) != 3 {
		t.Fatalf("second Tab should land on c (index 3), got %d", which(t, a, b, hidden, c))
	}
}

func TestKeyAndCharRouteToFocusedDescendant(t *testing.T) {
	a, b := newFocusW(0, 0, 100, 30), newFocusW(0, 30, 100, 30)
	v := vboxOf(a, b)
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // focus a
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // focus b
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	v.OnEvent(Event{Kind: EventChar, Code: "x"})
	if len(a.events) != 0 {
		t.Fatalf("unfocused widget received %d events", len(a.events))
	}
	if len(b.events) != 2 || b.events[0].Code != "Enter" || b.events[1].Code != "x" {
		t.Fatalf("focused widget should receive Enter then char, got %+v", b.events)
	}
}

func TestKeyWithNothingFocusedIsDropped(t *testing.T) {
	a, b := newFocusW(0, 0, 100, 30), newFocusW(0, 30, 100, 30)
	v := vboxOf(a, b)
	// No Tab first: nothing focused. Enter/char must reach no one and not panic.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	v.OnEvent(Event{Kind: EventChar, Code: "y"})
	if len(a.events)+len(b.events) != 0 {
		t.Fatal("keys must be dropped when nothing is focused")
	}
}

func TestClickMovesFocusSingleFocus(t *testing.T) {
	a, b := newFocusW(0, 0, 100, 40), newFocusW(0, 40, 100, 40)
	v := vboxOf(a, b)
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	// Click inside a.
	v.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if which(t, a, b) != 0 {
		t.Fatalf("click in a should focus a, got %d", which(t, a, b))
	}
	// Click inside b: focus moves, a is defocused.
	v.OnEvent(Event{Kind: EventClick, X: 10, Y: 50})
	if which(t, a, b) != 1 {
		t.Fatalf("click in b should focus b, got %d", which(t, a, b))
	}
}

func TestClickOutsideAnyFocusableLeavesFocus(t *testing.T) {
	a := newFocusW(0, 0, 40, 40)
	pad := &recordingWidget{} // non-focusable display widget filling the rest
	v := NewVBox()
	v.AddFixed(a, 40)
	v.Append(pad)
	v.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 100})
	v.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // focus a
	if !a.Focused() {
		t.Fatal("click in a should focus it")
	}
	// Click over the non-focusable pad: focus stays on a (no focusable hit).
	v.OnEvent(Event{Kind: EventClick, X: 5, Y: 80})
	if !a.Focused() {
		t.Fatal("click on a non-focusable must leave focus unchanged")
	}
	if len(pad.events) == 0 {
		t.Fatal("the click should still reach the display widget")
	}
}

// Traversal descends into nested containers in visual order.
func TestNestedContainerTraversal(t *testing.T) {
	a := newFocusW(0, 0, 100, 20)
	innerL := newFocusW(0, 20, 50, 20)
	innerR := newFocusW(50, 20, 50, 20)
	inner := NewHBox()
	inner.Append(innerL)
	inner.Append(innerR)
	c := newFocusW(0, 40, 100, 20)

	outer := NewVBox()
	outer.AddFixed(a, 20)
	outer.AddFixed(inner, 20)
	outer.AddFixed(c, 20)
	outer.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	order := []*focusW{a, innerL, innerR, c}
	for want := range order {
		outer.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		got := -1
		for i, w := range order {
			if w.Focused() {
				got = i
			}
		}
		if got != want {
			t.Fatalf("Tab %d landed on %d, want %d", want, got, want)
		}
	}
}

// Each of the five focus-managing containers traverses, routes keys, and
// focuses on click through the shared helpers.
func TestAllContainersManageFocus(t *testing.T) {
	build := map[string]func(a, b *focusW) Widget{
		"Container": func(a, b *focusW) Widget {
			c := NewContainer(&BoxLayout{Vertical: true})
			c.AddWidget(a)
			c.AddWidget(b)
			c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
			return c
		},
		"HBox": func(a, b *focusW) Widget {
			h := NewHBox()
			h.Append(a)
			h.Append(b)
			h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
			return h
		},
		"VBox": func(a, b *focusW) Widget {
			return vboxOf(a, b)
		},
		"Grid": func(a, b *focusW) Widget {
			g := NewGrid(1, 2)
			g.Attach(a, 0, 0)
			g.Attach(b, 0, 1)
			g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
			return g
		},
		"Frame": func(a, b *focusW) Widget {
			// Frame holds one child; nest a VBox so it has two focusables.
			inner := NewVBox()
			inner.Append(a)
			inner.Append(b)
			f := NewFrame(inner)
			f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
			return f
		},
	}
	for name, mk := range build {
		a, b := newFocusW(0, 0, 100, 40), newFocusW(0, 40, 100, 40)
		w := mk(a, b)
		w.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		if !a.Focused() {
			t.Fatalf("%s: first Tab should focus a", name)
		}
		w.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		if !b.Focused() || a.Focused() {
			t.Fatalf("%s: second Tab should move focus to b", name)
		}
		w.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
		if len(b.events) != 1 || b.events[0].Code != "Enter" {
			t.Fatalf("%s: Enter should route to focused b, got %+v", name, b.events)
		}
	}
}

// A Frame with no child exposes no focusable children and drops keys safely.
func TestFrameNilChildFocusNoOp(t *testing.T) {
	f := NewFrame(nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // no panic, nothing to focus
	f.OnEvent(Event{Kind: EventChar, Code: "z"})      // no focused descendant
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

// A Container carrying a nil-widget item (only possible with a nil layout, so no
// layout call dereferences it) must tolerate the nil during focus enumeration.
func TestContainerNilItemFocusSafe(t *testing.T) {
	c := NewContainer(nil) // nil layout => Add does not Arrange the nil widget
	c.Add(Item{Widget: nil})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"}) // gatherFocusables skips nil
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Shift+Tab"})
}

// An empty focus-managing container drops Tab without moving anything.
func TestEmptyContainerTabNoOp(t *testing.T) {
	h := NewHBox()
	h.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	h.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	h.OnEvent(Event{Kind: EventKeyDown, Code: "Shift+Tab"})
}
