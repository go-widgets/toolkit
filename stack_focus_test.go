// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// stackScene is a form whose lower half is a Stack of two pages, under a field
// that lives outside the Stack: the arrangement of an application window with
// a header above its screens (#480).
//
//	y   0..30   outside   Entry, not in the Stack
//	y  30..200  stack
//	              "main":     VBox{ a }
//	              "settings": VBox{ b, dd }
type stackScene struct {
	root            *VBox
	stack           *Stack
	outside, a, b   *Entry
	dd              *DropDown
	mainPg, setPage *VBox
}

func newStackScene() *stackScene {
	s := &stackScene{
		outside: NewEntry(""), a: NewEntry(""), b: NewEntry(""),
		dd: NewDropDown([]string{"one", "two"}, 0),
	}
	s.mainPg = NewVBox()
	s.mainPg.Append(s.a)
	s.setPage = NewVBox()
	s.setPage.Append(s.b)
	s.setPage.Append(s.dd)
	s.stack = NewStack()
	s.stack.AddPage("main", s.mainPg)
	s.stack.AddPage("settings", s.setPage)
	s.root = NewVBox()
	s.root.AddFixed(s.outside, 30)
	s.root.Append(s.stack)
	s.root.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	return s
}

// clickOn sends root a click at the centre of w, in root-local coordinates.
func clickOn(root Widget, w Widget) {
	b, r := w.Bounds(), root.Bounds()
	root.OnEvent(Event{Kind: EventClick, X: b.X + b.W/2 - r.X, Y: b.Y + b.H/2 - r.Y})
}

// An Entry on a Stack page that the user clicked receives the keys typed next.
// Before the fix the click focused it inside the page, but the window's own
// focus list stopped at the Stack, so every key went to a focus list that did
// not contain it and was dropped.
func TestStackEntryReceivesTypedKeys(t *testing.T) {
	s := newStackScene()
	clickOn(s.root, s.a)
	s.root.OnEvent(Event{Kind: EventChar, Code: "x"})
	if got := s.a.Value(); got != "x" {
		t.Fatalf("the Entry on the visible page holds %q after typing x, want \"x\"", got)
	}
	if s.outside.Value() != "" {
		t.Fatalf("the key went to the field outside the Stack: %q", s.outside.Value())
	}
}

// Tab walks from the field outside the Stack into the visible page and back,
// and never into the hidden page — whose widgets keep the bounds they last had
// and would otherwise look reachable.
func TestStackTakesPartInTabTraversal(t *testing.T) {
	s := newStackScene()
	// Bring "settings" forward once so its widgets acquire real bounds, then
	// hide it again: a hidden page with stale, non-empty bounds is the case a
	// bounds check alone cannot exclude.
	s.stack.SetVisible("settings")
	s.root.SetBounds(s.root.Bounds())
	s.stack.SetVisible("main")
	s.root.SetBounds(s.root.Bounds())

	tab := Event{Kind: EventKeyDown, Code: "Tab"}
	s.root.OnEvent(tab)
	if !s.outside.Focused() {
		t.Fatal("first Tab should focus the field outside the Stack")
	}
	s.root.OnEvent(tab)
	if !s.a.Focused() || s.outside.Focused() {
		t.Fatalf("second Tab should move focus to the visible page's Entry (a=%v outside=%v)",
			s.a.Focused(), s.outside.Focused())
	}
	for range 4 {
		s.root.OnEvent(tab)
		if s.b.Focused() || s.dd.Focused() {
			t.Fatal("Tab reached a widget on the hidden page")
		}
	}

	s.stack.SetVisible("settings")
	s.root.SetBounds(s.root.Bounds())
	s.outside.SetFocused(true)
	s.a.SetFocused(false)
	s.root.OnEvent(tab)
	if !s.b.Focused() {
		t.Fatal("after SetVisible, Tab should reach the newly visible page")
	}
	s.root.OnEvent(Event{Kind: EventChar, Code: "y"})
	if s.b.Value() != "y" {
		t.Fatalf("the newly visible page's Entry holds %q, want \"y\"", s.b.Value())
	}
}

// A DropDown on a Stack page opens a popover the host can see: PopoverHost
// finds open popovers through the Children walk, and the Stack used to end it.
// The DropDown on a hidden page is never reported, open or not.
func TestStackExposesItsVisiblePagePopover(t *testing.T) {
	s := newStackScene()
	s.stack.SetVisible("settings")
	host := NewPopoverHost(s.root)
	host.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickOn(host, s.dd)
	if !s.dd.Open().Get() {
		t.Fatal("the click did not open the DropDown")
	}
	owners := PopoverOwners(host)
	if len(owners) != 1 || owners[0] != PopoverOwner(s.dd) {
		t.Fatalf("PopoverOwners = %v, want the open DropDown on the visible page", owners)
	}

	s.stack.SetVisible("main")
	if got := PopoverOwners(host); len(got) != 0 {
		t.Fatalf("a popover on the hidden page is still reported: %v", got)
	}
}

// A Stack used as the root, with a lone Entry as its page, routes focus itself:
// a click focuses the Entry and the next key reaches it.
func TestStackAsRootRoutesFocus(t *testing.T) {
	e := NewEntry("")
	st := NewStack()
	st.AddPage("only", e)
	st.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 30})

	st.OnEvent(Event{Kind: EventChar, Code: "q"})
	if e.Value() != "" {
		t.Fatal("a key reached an Entry that does not hold focus")
	}
	st.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	st.OnEvent(Event{Kind: EventChar, Code: "z"})
	if e.Value() != "z" {
		t.Fatalf("Entry holds %q after click + z, want \"z\"", e.Value())
	}
}

// Children and focusableChildren yield the visible page only, and nothing for a
// Stack with no page; an empty Stack ignores every event.
func TestStackChildrenAreTheVisiblePage(t *testing.T) {
	st := NewStack()
	if got := st.Children(); len(got) != 0 {
		t.Fatalf("empty Stack Children = %v", got)
	}
	st.OnEvent(Event{Kind: EventClick, X: 1, Y: 1}) // no page: a no-op
	a, b := NewButton("a", nil), NewButton("b", nil)
	st.AddPage("a", a)
	st.AddPage("b", b)
	if got := st.Children(); len(got) != 1 || got[0] != Widget(a) {
		t.Fatalf("Children = %v, want just the visible page a", got)
	}
	st.SetVisible("b")
	if got := st.focusableChildren(); len(got) != 1 || got[0] != Widget(b) {
		t.Fatalf("focusableChildren = %v, want just the visible page b", got)
	}
}
