// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"

	"github.com/go-widgets/mvvm"
)

// Part 3 of #578: the second entry points that bypass OnEvent, the keyboard path
// of the composites, and the state a gesture in flight leaves behind.

// honourSplitter is two equal panes side by side, laid out.
func honourSplitter() *Splitter {
	s := NewSplitter(false,
		&SplitPane{Widget: NewLabel("a"), Weight: mvvm.NewObservable(1), Min: 10},
		&SplitPane{Widget: NewLabel("b"), Weight: mvvm.NewObservable(1), Min: 10})
	s.SetBounds(Rect{W: 210, H: 100})
	return s
}

// keyed puts w in a VBox of the given height so the focus walk reaches it.
func keyed(w Widget, h int) *VBox {
	v := NewVBox()
	v.AddFixed(w, h)
	v.SetBounds(Rect{W: 400, H: h})
	return v
}

func tabAndType(w Widget, code string) {
	w.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	w.OnEvent(Event{Kind: EventChar, Code: code})
}

// A disabled Frame, Inspector and ForgePane must neither be walked into by the
// focus traversal nor take a key handed to them after they held focus.
func TestDisabledCompositesTakeNoKeys(t *testing.T) {
	t.Run("Frame", func(t *testing.T) {
		e := NewEntry("ab")
		f := NewFrame(e)
		v := keyed(f, 60)
		f.Disabled().Set(true)
		tabAndType(v, "z")
		if got := e.Text().Get(); got != "ab" {
			t.Fatalf("disabled Frame, keys through the walk: Entry became %q", got)
		}
		f.Disabled().Set(false)
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		f.Disabled().Set(true)
		f.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := e.Text().Get(); got != "ab" {
			t.Fatalf("disabled Frame, key sent directly: Entry became %q", got)
		}
		f.Disabled().Set(false)
		f.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := e.Text().Get(); got != "abz" {
			t.Fatalf("re-enabled Frame: Entry is %q, want abz", got)
		}
	})
	t.Run("Inspector", func(t *testing.T) {
		in := NewInspector([]InspectorRow{{ID: "name", Label: "Name", Kind: InspectorText}})
		in.Set("name", "ab")
		v := keyed(in, 60)
		in.Disabled().Set(true)
		tabAndType(v, "z")
		if got := in.Get("name"); got != "ab" {
			t.Fatalf("disabled Inspector, keys through the walk: %q", got)
		}
		in.Disabled().Set(false)
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		in.Disabled().Set(true)
		in.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := in.Get("name"); got != "ab" {
			t.Fatalf("disabled Inspector, key sent directly: %q", got)
		}
		in.Disabled().Set(false)
		in.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := in.Get("name"); got != "abz" {
			t.Fatalf("re-enabled Inspector: %q, want abz", got)
		}
	})
	t.Run("ForgePane", func(t *testing.T) {
		m := newFakeForge()
		f := laidOutForge(m, 600, 600)
		v := NewVBox()
		v.AddFlex(f, 1)
		v.SetBounds(Rect{W: 600, H: 600})
		f.client.repo.SetFocused(true)
		f.Disabled().Set(true)
		v.OnEvent(Event{Kind: EventChar, Code: "z"})
		f.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := m.repo.Get(); got != "a/b" {
			t.Fatalf("disabled ForgePane: repo became %q", got)
		}
		f.Disabled().Set(false)
		f.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := m.repo.Get(); got != "za/b" { // typed at the caret, which starts at 0
			t.Fatalf("re-enabled ForgePane: repo is %q, want za/b", got)
		}
	})
	t.Run("FontPicker", func(t *testing.T) {
		f := NewFontPickerFull([]string{"a", "b"}, FontSpec{Family: "a", Weight: 400, Size: 16})
		v := keyed(f, 24)
		size := func() int { return f.Spec().Get().Size }
		f.Disabled().Set(true)
		for range 3 {
			v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		}
		v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
		if size() != 16 || f.size.Focused() {
			t.Fatalf("disabled FontPicker: size %d, spinner focused %v", size(), f.size.Focused())
		}
		f.Disabled().Set(false)
		for range 3 {
			v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		}
		f.Disabled().Set(true)
		f.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
		if size() != 16 {
			t.Fatalf("disabled FontPicker, key sent directly: size %d", size())
		}
		f.Disabled().Set(false)
		f.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
		if size() != 17 {
			t.Fatalf("re-enabled FontPicker: size %d, want 17", size())
		}
	})
}

// The divider handles are widgets a host can drive without the Splitter's
// OnEvent: Handle(j).OnEvent for a key, FocusHandle for focus, Cursor for the
// resize cursor.
func TestDisabledSplitterHandlesTakeNoInput(t *testing.T) {
	s := honourSplitter()
	h := s.Handle(0)
	before := s.PaneSize(0)
	s.Disabled().Set(true)
	h.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if s.PaneSize(0) != before {
		t.Fatalf("disabled: a key on the handle moved the divider to %d", s.PaneSize(0))
	}
	if s.FocusHandle(0) || s.FocusedHandle() != -1 {
		t.Fatal("disabled: a divider took the keyboard")
	}
	hr, _ := s.HandleRect(0)
	x, y := centre(hr)
	if c := s.Cursor(x, y); c != "" {
		t.Fatalf("disabled: cursor %q over a divider, want none", c)
	}
	if !h.(*dividerHandle).A11y().Disabled {
		t.Fatal("disabled: the divider's A11y does not say so")
	}
	s.Disabled().Set(false)
	if !s.FocusHandle(0) {
		t.Fatal("re-enabled: the divider refuses the keyboard")
	}
	h.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if s.PaneSize(0) == before {
		t.Fatal("re-enabled: a key on the handle moved nothing")
	}
	hr, _ = s.HandleRect(0) // the key moved the divider
	x, y = centre(hr)
	if s.Cursor(x, y) == "" {
		t.Fatal("re-enabled: no resize cursor over the divider")
	}
	if h.(*dividerHandle).A11y().Disabled {
		t.Fatal("re-enabled: the divider still reads Disabled")
	}
}

// Tabbing to a divider and pressing a key through the Splitter's walk.
func TestDisabledSplitterWalkTakesNoKeys(t *testing.T) {
	s := honourSplitter()
	v := keyed(s, 100)
	before := s.PaneSize(0)
	s.Disabled().Set(true)
	for range 2 {
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if s.PaneSize(0) != before || s.FocusedHandle() != -1 {
		t.Fatalf("disabled: pane %d (was %d), focused handle %d", s.PaneSize(0), before, s.FocusedHandle())
	}
}

// A divider held when the splitter is disabled is let go: the release that
// follows must not move it, and re-enabling must not resume the drag.
func TestDisabledSplitterReleasesAGrabInFlight(t *testing.T) {
	s := honourSplitter()
	hr, _ := s.HandleRect(0)
	x, y := centre(hr)
	s.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if !s.Dragging().Get() {
		t.Fatal("setup: the press did not grab the divider")
	}
	before := s.PaneSize(0)
	s.Disabled().Set(true)
	s.OnEvent(Event{Kind: EventMouseDrag, X: x + 30, Y: y})
	if s.Dragging().Get() || s.PaneSize(0) != before {
		t.Fatalf("disabled: dragging %v, pane %d (was %d)", s.Dragging().Get(), s.PaneSize(0), before)
	}
	s.Disabled().Set(false)
	s.OnEvent(Event{Kind: EventMouseDrag, X: x + 30, Y: y})
	if s.PaneSize(0) != before {
		t.Fatalf("re-enabled: the old grab resumed, pane %d (was %d)", s.PaneSize(0), before)
	}
}

// Border.ResizeSplit is what an application calls for a drag it tracks itself.
func TestDisabledBorderRefusesResize(t *testing.T) {
	n := 0
	b := NewBorder()
	b.West, b.Center = NewLabel("w"), NewLabel("c")
	b.WestSize, b.WestSplit = 40, true
	b.OnResize = func(DockSide, int) { n++ }
	b.SetBounds(Rect{W: 200, H: 100})
	b.Disabled().Set(true)
	b.ResizeSplit(DockLeft, 80)
	if b.WestSize != 40 || n != 0 {
		t.Fatalf("disabled: WestSize %d, %d OnResize calls", b.WestSize, n)
	}
	// a handle grabbed before it was disabled is let go
	b.Disabled().Set(false)
	h, _ := b.SplitHandleAt(40+BorderSplitW/2, 5)
	b.OnEvent(Event{Kind: EventClick, X: 40 + BorderSplitW/2, Y: 5})
	if !b.resizing {
		t.Fatalf("setup: the press on side %v did not grab the handle", h)
	}
	b.Disabled().Set(true)
	b.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 5})
	if b.resizing || b.WestSize != 40 {
		t.Fatalf("disabled mid-drag: resizing %v, WestSize %d", b.resizing, b.WestSize)
	}
	b.Disabled().Set(false)
	b.ResizeSplit(DockLeft, 80)
	if b.WestSize != 80 || n != 1 {
		t.Fatalf("re-enabled: WestSize %d, %d OnResize calls, want 80 and 1", b.WestSize, n)
	}
}

// Kanban.MoveCard is what a host calls from a menu action; a card in flight when
// the board is disabled is dropped, not kept for a later release.
func TestDisabledKanbanRefusesMovesAndDropsAGrab(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3
	k.Disabled().Set(true)
	k.MoveCard(0, 0, 2, 0)
	if len(k.Columns[0].Cards) != 2 || len(k.Columns[2].Cards) != 1 {
		t.Fatalf("disabled: MoveCard moved a card: %d / %d", len(k.Columns[0].Cards), len(k.Columns[2].Cards))
	}
	k.Disabled().Set(false)
	from, to := k.cardLocalRect(0, 0, colW), k.cardLocalRect(2, 0, colW)
	k.OnEvent(Event{Kind: EventClick, X: from.X + 4, Y: from.Y + 4})
	k.OnEvent(Event{Kind: EventMouseDrag, X: to.X + 4, Y: to.Y + 4})
	k.Disabled().Set(true)
	k.OnEvent(Event{Kind: EventMouseUp, X: to.X + 4, Y: to.Y + 4})
	k.Disabled().Set(false)
	k.OnEvent(Event{Kind: EventMouseUp, X: to.X + 4, Y: to.Y + 4})
	if len(k.Columns[0].Cards) != 2 || len(k.Columns[2].Cards) != 1 {
		t.Fatalf("a grab in flight outlived the disabled state: %d / %d", len(k.Columns[0].Cards), len(k.Columns[2].Cards))
	}
	k.MoveCard(0, 0, 2, 0)
	if len(k.Columns[2].Cards) != 2 {
		t.Fatal("re-enabled: MoveCard moves nothing")
	}
}

// TimePicker's three steppers are public and a host binds keys to them.
func TestDisabledTimePickerRefusesSteps(t *testing.T) {
	tp := NewTimePicker(9, 30)
	tp.Disabled().Set(true)
	tp.StepHour(1)
	tp.StepMinute(1)
	tp.ToggleAmPm()
	if got := tp.String(); got != NewTimePicker(9, 30).String() {
		t.Fatalf("disabled: time became %s", got)
	}
	tp.Disabled().Set(false)
	tp.StepHour(1)
	tp.StepMinute(1)
	tp.ToggleAmPm()
	if tp.Hour().Get() != 22 || tp.Minute().Get() != 31 {
		t.Fatalf("re-enabled: %02d:%02d, want 22:31", tp.Hour().Get(), tp.Minute().Get())
	}
}

// Wizard.Next and Back are public (a host binds keys to them); Next on the last
// step is Finish. A button held when the wizard is disabled is let go.
func TestDisabledWizardRefusesNavigation(t *testing.T) {
	finished := 0
	w := NewWizard([]WizardStep{{Title: "a"}, {Title: "b"}})
	w.OnFinish = func() { finished++ }
	w.SetBounds(Rect{W: 300, H: 200})
	w.Disabled().Set(true)
	w.Next()
	if w.Current().Get() != 0 {
		t.Fatalf("disabled: Next moved to step %d", w.Current().Get())
	}
	w.Disabled().Set(false)
	nx, ny := centre(w.nextRect())
	w.OnEvent(Event{Kind: EventClick, X: nx, Y: ny})
	if w.Current().Get() != 1 || w.pressedBtn != wizBtnNext {
		t.Fatalf("setup: step %d, pressed %d", w.Current().Get(), w.pressedBtn)
	}
	w.Disabled().Set(true)
	w.Back()
	w.Next() // the last step: Finish
	if w.Current().Get() != 1 || finished != 0 {
		t.Fatalf("disabled: step %d, %d finishes", w.Current().Get(), finished)
	}
	w.OnEvent(Event{Kind: EventMouseUp})
	if w.pressedBtn != wizBtnNone {
		t.Fatal("disabled: a held button stayed pressed")
	}
	w.Disabled().Set(false)
	w.Back()
	if w.Current().Get() != 0 {
		t.Fatalf("re-enabled: Back left step %d", w.Current().Get())
	}
	w.Next()
	w.Next()
	if finished != 1 {
		t.Fatalf("re-enabled: %d finishes, want 1", finished)
	}
}

// Inspector.ToggleColor opens a colour row's picker; PropertyGrid.RemoveAt
// deletes a property: both are host-callable actions.
func TestDisabledInspectorAndPropertyGridRefuseHostActions(t *testing.T) {
	in, _ := testInspector()
	in.Disabled().Set(true)
	in.ToggleColor("bg")
	if in.byID["bg"].open {
		t.Fatal("disabled Inspector: ToggleColor opened a picker")
	}
	in.Disabled().Set(false)
	in.ToggleColor("bg")
	if !in.byID["bg"].open {
		t.Fatal("re-enabled Inspector: ToggleColor opened nothing")
	}

	pg := NewPropertyGrid()
	pg.Add("a", "1")
	pg.Disabled().Set(true)
	pg.RemoveAt(0)
	if pg.Value("a") != "1" {
		t.Fatal("disabled PropertyGrid: RemoveAt removed a property")
	}
	pg.Disabled().Set(false)
	pg.RemoveAt(0)
	if pg.Value("a") != "" {
		t.Fatal("re-enabled PropertyGrid: RemoveAt removed nothing")
	}
}

// PopoverHost asks every open PopoverOwner in the tree for the click. A composite
// cannot reach into the open list of a drop-down it holds, so a disabled
// composite must show none of its descendants' popups.
func TestDisabledCompositeShowsNoChildPopup(t *testing.T) {
	cases := map[string]func() (Widget, *mvvm.Observable[bool], func()){
		"FontPicker": func() (Widget, *mvvm.Observable[bool], func()) {
			f := NewFontPicker([]string{"a", "b", "c"})
			return f, f.Disabled(), func() { f.family.Open().Set(true) }
		},
		"Inspector": func() (Widget, *mvvm.Observable[bool], func()) {
			in, _ := testInspector()
			return in, in.Disabled(), func() { in.byID["pos"].enum.Open().Set(true) }
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			w, off, open := build()
			v := keyed(w, 400)
			open()
			if n := len(PopoverOwners(v)); n != 1 {
				t.Fatalf("setup: %d popovers showing, want 1", n)
			}
			off.Set(true)
			if n := len(PopoverOwners(v)); n != 0 {
				t.Errorf("disabled: %d popovers showing, want 0", n)
			}
			off.Set(false)
			if n := len(PopoverOwners(v)); n != 1 {
				t.Errorf("re-enabled: %d popovers showing, want 1", n)
			}
		})
	}
}

// MarkdownEditor, RichEditorToolbar and the rest hold widgets but are dead ends
// for the focus walk, so a key reaches their children only through their own
// OnEvent, which the flag gates. If one of them ever becomes a focus enumerator,
// this fails and the gate must be re-checked.
func TestBatch3CompositesAreFocusDeadEnds(t *testing.T) {
	for name, w := range map[string]Widget{
		"MarkdownEditor":    NewMarkdownEditor("a"),
		"RichEditorToolbar": NewRichEditorToolbar(nil),
		"PropertyGrid":      NewPropertyGrid(),
		"LogView":           NewLogView(),
		"Wizard":            NewWizard(nil),
		"Kanban":            NewKanban(nil),
		"Border":            NewBorder(),
	} {
		if _, ok := w.(focusEnumerator); ok {
			t.Errorf("%s is a focusEnumerator: keys can reach its children past its gate", name)
		}
	}
}

// A MarkdownEditor's Source takes keys only through the editor's OnEvent.
func TestDisabledMarkdownEditorTakesNoKeys(t *testing.T) {
	m := NewMarkdownEditor("hello")
	m.SetBounds(Rect{W: 100, H: 60})
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	m.Disabled().Set(true)
	m.OnEvent(Event{Kind: EventChar, Code: "x"})
	if got := m.Text(); got != "hello" {
		t.Fatalf("disabled: text became %q", got)
	}
	m.Disabled().Set(false)
	m.OnEvent(Event{Kind: EventChar, Code: "x"})
	if got := fmt.Sprint(m.Text()); got == "hello" {
		t.Fatal("re-enabled: the key changed nothing")
	}
}
