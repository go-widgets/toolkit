// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// ---- recordingWidget -----------------------------------------------------

// recordingWidget is a Widget stub that records every Draw / OnEvent
// call. Used by structural tests to assert routing.
type recordingWidget struct {
	Base
	draws  int
	events []Event
}

func (r *recordingWidget) Draw(_ painter.Painter, _ *Theme) { r.draws++ }
func (r *recordingWidget) OnEvent(ev Event)               { r.events = append(r.events, ev) }

// --- Stack ---------------------------------------------------------------

func TestStackAddPageAutoVisible(t *testing.T) {
	s := NewStack()
	w1 := &recordingWidget{}
	s.AddPage("main", w1)
	if s.Visible != "main" {
		t.Fatalf("Visible after first AddPage = %q, want main", s.Visible)
	}
}

func TestStackAddSecondPageKeepsFirstVisible(t *testing.T) {
	s := NewStack()
	s.AddPage("a", &recordingWidget{})
	s.AddPage("b", &recordingWidget{})
	if s.Visible != "a" {
		t.Fatalf("Visible after 2nd AddPage = %q, want a", s.Visible)
	}
}

func TestStackSetVisibleKnownAndUnknown(t *testing.T) {
	s := NewStack()
	s.AddPage("a", &recordingWidget{})
	s.AddPage("b", &recordingWidget{})
	s.SetVisible("b")
	if s.Visible != "b" {
		t.Fatalf("after SetVisible(b): Visible = %q", s.Visible)
	}
	s.SetVisible("ghost") // unknown — must NOT change Visible
	if s.Visible != "b" {
		t.Fatalf("after SetVisible(ghost): Visible changed to %q", s.Visible)
	}
}

func TestStackDrawAndEventGoToVisibleOnly(t *testing.T) {
	s := NewStack()
	a := &recordingWidget{}
	b := &recordingWidget{}
	s.AddPage("a", a)
	s.AddPage("b", b)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	s.Draw(newP(make([]byte, 100*100*4), 100), DefaultLight())
	if a.draws != 1 || b.draws != 0 {
		t.Fatalf("draws after first Draw: a=%d b=%d", a.draws, b.draws)
	}
	s.SetVisible("b")
	s.Draw(newP(make([]byte, 100*100*4), 100), DefaultLight())
	if a.draws != 1 || b.draws != 1 {
		t.Fatalf("draws after switch: a=%d b=%d", a.draws, b.draws)
	}
	s.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if len(a.events) != 0 || len(b.events) != 1 {
		t.Fatalf("events: a=%d b=%d", len(a.events), len(b.events))
	}
}

func TestStackDrawWithNoPagesNoOp(t *testing.T) {
	s := NewStack()
	s.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	s.Draw(newP(make([]byte, 10*10*4), 10), DefaultLight())
	s.OnEvent(Event{Kind: EventClick})
}

// --- Notebook ------------------------------------------------------------

func TestNotebookAddTabAndDraw(t *testing.T) {
	n := NewNotebook()
	a := &recordingWidget{}
	b := &recordingWidget{}
	n.AddTab("A", a)
	n.AddTab("B", b)
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.Draw(newP(make([]byte, 200*80*4), 200), DefaultLight())
	if a.draws != 1 {
		t.Fatalf("active page drawn %d times, want 1", a.draws)
	}
	if b.draws != 0 {
		t.Fatal("inactive page must not draw")
	}
}

func TestNotebookClickSelectsTab(t *testing.T) {
	got := -1
	n := NewNotebook()
	n.OnTabChanged = func(i int) { got = i }
	n.AddTab("A", &recordingWidget{})
	n.AddTab("B", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// Click at x=100 in the strip → tab idx = 100 / 80 = 1.
	n.OnEvent(Event{Kind: EventClick, X: 100, Y: 5})
	if n.Active != 1 || got != 1 {
		t.Fatalf("Active=%d got=%d", n.Active, got)
	}
}

func TestNotebookClickOutOfRangeTab(t *testing.T) {
	n := NewNotebook()
	n.AddTab("A", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 500, Y: 5})
	if n.Active != 0 {
		t.Fatal("out-of-range tab click must not select")
	}
}

func TestNotebookClickBodyRoutesToActivePage(t *testing.T) {
	a := &recordingWidget{}
	n := NewNotebook()
	n.AddTab("A", a)
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
	if len(a.events) != 1 {
		t.Fatalf("body click should route to active page, got %d events", len(a.events))
	}
}

func TestNotebookNilOnTabChangedNoPanic(t *testing.T) {
	n := NewNotebook()
	n.AddTab("A", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
}

func TestNotebookNilPageDrawNoPanic(t *testing.T) {
	n := NewNotebook()
	n.AddTab("X", nil)
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.Draw(newP(make([]byte, 200*80*4), 200), DefaultLight())
}

func TestNotebookNilPageEventNoPanic(t *testing.T) {
	n := NewNotebook()
	n.AddTab("X", nil)
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
}

func TestNotebookEmptyDrawAndEvent(t *testing.T) {
	n := NewNotebook()
	n.Active = 5 // out of range to exercise Draw/OnEvent guards
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.Draw(newP(make([]byte, 200*80*4), 200), DefaultLight())
	n.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
}

func TestNotebookForwardsKeyEventsToActivePage(t *testing.T) {
	// KeyDown / Char must reach the active page so an Entry / focused
	// widget inside a tab gets its keystrokes. Only strip-area clicks
	// are intercepted by the Notebook itself.
	n := NewNotebook()
	a := &recordingWidget{}
	n.AddTab("A", a)
	n.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(a.events) != 1 {
		t.Fatalf("KeyDown should forward to active page, got %d events", len(a.events))
	}
}

// --- Paned ---------------------------------------------------------------

func TestPanedHorizontalLayout(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewHPaned(a, b)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// Default Position = 100 (centre).
	if p.Position != 100 {
		t.Fatalf("default position = %d, want 100", p.Position)
	}
	ab := a.Bounds()
	bb := b.Bounds()
	if ab.W != 100 || bb.X != 106 || bb.W != 94 {
		t.Fatalf("layout: a=%+v b=%+v", ab, bb)
	}
}

func TestPanedVerticalLayout(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewVPaned(a, b)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	if p.Position != 40 {
		t.Fatalf("default position = %d, want 40", p.Position)
	}
	bb := b.Bounds()
	if bb.Y != 46 || bb.H != 34 {
		t.Fatalf("second bounds = %+v", bb)
	}
}

func TestPanedMoveHandleClamps(t *testing.T) {
	p := NewHPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.MoveHandle(5)
	if p.Position != 10 {
		t.Fatalf("clamp low: Position = %d, want 10", p.Position)
	}
	p.MoveHandle(500)
	if p.Position != 190 {
		t.Fatalf("clamp high: Position = %d, want 190", p.Position)
	}
}

func TestPanedMoveHandleVerticalClamps(t *testing.T) {
	p := NewVPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.MoveHandle(500)
	if p.Position != 70 {
		t.Fatalf("vertical high clamp = %d, want 70", p.Position)
	}
}

func TestPanedMoveHandleFiresOnPositionChanged(t *testing.T) {
	got := 0
	p := NewHPaned(&recordingWidget{}, &recordingWidget{})
	p.OnPositionChanged = func(pos int) { got = pos }
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.MoveHandle(50)
	if got != 50 {
		t.Fatalf("OnPositionChanged got %d", got)
	}
}

func TestPanedDrawHorizontal(t *testing.T) {
	const w, h = 64, 32
	theme := DefaultLight()
	p := NewHPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Handle at x=30..36 painted in SurfaceAlt.
	if pixelAt(buf, w, 32, 15) != theme.SurfaceAlt {
		t.Fatalf("horizontal handle = %+v", pixelAt(buf, w, 32, 15))
	}
}

func TestPanedDrawVertical(t *testing.T) {
	const w, h = 64, 60
	theme := DefaultLight()
	p := NewVPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Handle at y=30..36 painted in SurfaceAlt.
	if pixelAt(buf, w, 30, 32) != theme.SurfaceAlt {
		t.Fatalf("vertical handle = %+v", pixelAt(buf, w, 30, 32))
	}
}

func TestPanedDrawNilChildrenNoPanic(t *testing.T) {
	p := NewHPaned(nil, nil)
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	p.Draw(newP(make([]byte, 60*30*4), 60), DefaultLight())
}

func TestPanedEventRoutingHorizontal(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewHPaned(a, b)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.OnEvent(Event{Kind: EventClick, X: 20, Y: 10}) // left of handle
	if len(a.events) != 1 || len(b.events) != 0 {
		t.Fatalf("left click: a=%d b=%d", len(a.events), len(b.events))
	}
	p.OnEvent(Event{Kind: EventClick, X: 180, Y: 10}) // right of handle
	if len(b.events) != 1 {
		t.Fatalf("right click: b=%d", len(b.events))
	}
}

func TestPanedEventRoutingVertical(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewVPaned(a, b)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if len(a.events) != 1 {
		t.Fatalf("top click: a=%d", len(a.events))
	}
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 70})
	if len(b.events) != 1 {
		t.Fatalf("bottom click: b=%d", len(b.events))
	}
}

func TestPanedEventOnHandleIgnored(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewHPaned(a, b)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// Click ON the handle (Position=100, handle=100..106).
	p.OnEvent(Event{Kind: EventClick, X: 103, Y: 10})
	if len(a.events) != 0 || len(b.events) != 0 {
		t.Fatal("click on handle should not propagate")
	}
}

func TestPanedEventIgnoresNonClick(t *testing.T) {
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewHPaned(a, b)
	p.OnEvent(Event{Kind: EventKeyDown, Code: "x"})
	if len(a.events) != 0 || len(b.events) != 0 {
		t.Fatal("KeyDown must not route through Paned")
	}
}

func TestPanedNilFirstSecondLayoutNoOp(t *testing.T) {
	p := NewHPaned(nil, nil)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// layout() short-circuits; nothing to assert beyond no-panic.
}

func TestPanedNilFirstEventRoutingNoCrash(t *testing.T) {
	b := &recordingWidget{}
	p := &Paned{First: nil, Second: b, Orientation: PanedHorizontal, Position: 50}
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.OnEvent(Event{Kind: EventClick, X: 20, Y: 10})  // left -> nil first
	p.OnEvent(Event{Kind: EventClick, X: 180, Y: 10}) // right -> Second
	if len(b.events) != 1 {
		t.Fatalf("right click should reach Second; got %d", len(b.events))
	}
}

func TestPanedNilSecondVerticalNoCrash(t *testing.T) {
	a := &recordingWidget{}
	p := &Paned{First: a, Second: nil, Orientation: PanedVertical, Position: 30}
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 70}) // would go to nil second
	if len(a.events) != 1 {
		t.Fatalf("top click should reach First; got %d", len(a.events))
	}
}

// --- Expander ------------------------------------------------------------

func TestExpanderClickHeaderTogglesAndFires(t *testing.T) {
	expanded := false
	e := NewExpander("Settings", &recordingWidget{})
	e.OnExpand = func(v bool) { expanded = v }
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !e.Expanded || !expanded {
		t.Fatalf("after header click: Expanded=%v expanded=%v", e.Expanded, expanded)
	}
	e.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if e.Expanded || expanded {
		t.Fatal("second click should collapse")
	}
}

func TestExpanderClickBodyRoutesWhenExpanded(t *testing.T) {
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.Expanded = true
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	if len(body.events) != 1 {
		t.Fatalf("body click: got %d events", len(body.events))
	}
}

func TestExpanderClickBodyIgnoredWhenCollapsed(t *testing.T) {
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	if len(body.events) != 0 {
		t.Fatal("collapsed body shouldn't get events")
	}
}

func TestExpanderNilContentNoPanic(t *testing.T) {
	e := NewExpander("S", nil)
	e.Expanded = true
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.Draw(newP(make([]byte, 200*100*4), 200), DefaultLight())
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
}

func TestExpanderIgnoresNonClick(t *testing.T) {
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if e.Expanded || len(body.events) != 0 {
		t.Fatal("KeyDown must not toggle or propagate")
	}
}

func TestExpanderNilOnExpandNoPanic(t *testing.T) {
	e := NewExpander("S", nil)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
}

func TestExpanderDrawCollapsedAndExpanded(t *testing.T) {
	const w, h = 200, 100
	theme := DefaultLight()
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.Draw(newP(makeSurface(w, h), w), theme)
	if body.draws != 0 {
		t.Fatal("collapsed Draw must not render body")
	}
	e.Expanded = true
	e.Draw(newP(makeSurface(w, h), w), theme)
	if body.draws != 1 {
		t.Fatal("expanded Draw must render body")
	}
}
