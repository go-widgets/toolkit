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
func (r *recordingWidget) OnEvent(ev Event)                 { r.events = append(r.events, ev) }

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
	n.Active().Subscribe(func(i int) { got = i })
	n.AddTab("A", &recordingWidget{})
	n.AddTab("B", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// Click at x=100 in the strip → tab idx = 100 / 80 = 1.
	n.OnEvent(Event{Kind: EventClick, X: 100, Y: 5})
	if n.Active().Get() != 1 || got != 1 {
		t.Fatalf("Active=%d got=%d", n.Active().Get(), got)
	}
}

func TestNotebookClickOutOfRangeTab(t *testing.T) {
	n := NewNotebook()
	n.AddTab("A", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 500, Y: 5})
	if n.Active().Get() != 0 {
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

func TestNotebookBodyClickTranslatedAtNonZeroBounds(t *testing.T) {
	// Regression: a body click must arrive in the page's local frame, i.e.
	// shifted up by NotebookTabStripH and by the Notebook's own origin.
	a := &recordingWidget{}
	n := NewNotebook()
	n.AddTab("A", a)
	n.SetBounds(Rect{X: 100, Y: 50, W: 200, H: 80})
	n.OnEvent(Event{Kind: EventClick, X: 50, Y: 40}) // notebook-local, in the body
	if len(a.events) != 1 {
		t.Fatalf("body click routed %d events, want 1", len(a.events))
	}
	// Page origin == Notebook origin in X (offset 0); Y is stripH below the top.
	if got := a.events[0]; got.X != 50 || got.Y != 40-NotebookTabStripH {
		t.Fatalf("page received %+v, want {50,%d}", got, 40-NotebookTabStripH)
	}
}

func TestNotebookTabSides(t *testing.T) {
	// Selection + body routing + the active-edge indicator for each non-default
	// side. Bounds 240×100, 3 tabs (tab width 80, strip thickness 24).
	const w, h = 240, 100
	acc := DefaultLight().Accent
	cases := []struct {
		name    string
		side    TabSide
		clickAt [2]int // widget-local click inside the middle (index 1) tab
		accAt   [2]int // a pixel on the active tab's accent edge after selecting 1
		bodyAt  [2]int // a widget-local click that lands in the body
	}{
		{"bottom", TabBottom, [2]int{120, 88}, [2]int{100, 76}, [2]int{10, 10}},
		{"left", TabLeft, [2]int{40, 36}, [2]int{78, 30}, [2]int{120, 10}},
		{"right", TabRight, [2]int{200, 36}, [2]int{160, 30}, [2]int{10, 10}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := &recordingWidget{}
			n := NewNotebook()
			n.TabSide = c.side
			n.AddTab("A", &recordingWidget{})
			n.AddTab("B", page)
			n.AddTab("C", &recordingWidget{})
			n.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

			// Click the middle tab → Active = 1.
			n.OnEvent(Event{Kind: EventClick, X: c.clickAt[0], Y: c.clickAt[1]})
			if n.Active().Get() != 1 {
				t.Fatalf("%s: click tab 1 → Active=%d", c.name, n.Active().Get())
			}
			// Draw: the active tab's edge carries the Accent indicator.
			buf := makeSurface(w, h)
			n.Draw(newP(buf, w), DefaultLight())
			if got := pixelAt(buf, w, c.accAt[0], c.accAt[1]); got != acc {
				t.Errorf("%s: active edge at %v = %+v, want Accent", c.name, c.accAt, got)
			}
			// A body click routes to the (now active) page.
			n.OnEvent(Event{Kind: EventClick, X: c.bodyAt[0], Y: c.bodyAt[1]})
			if len(page.events) != 1 {
				t.Errorf("%s: body click routed %d events, want 1", c.name, len(page.events))
			}
		})
	}
}

func TestNotebookClickNoSubscriberNoPanic(t *testing.T) {
	n := NewNotebook()
	n.AddTab("A", &recordingWidget{})
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	// No subscriber bound: a tab click Sets the Active Observable without panic.
	n.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
}

// TestNotebookBareAccessorInitAndBind proves the Active accessor lazy-inits on a
// bare &Notebook{} (no constructor) and that a host can bind it: a Subscribe on
// the freshly-initialised Observable observes a keyboard tab move.
func TestNotebookBareAccessorInitAndBind(t *testing.T) {
	n := &Notebook{Tabs: []NotebookTab{{Label: "A"}, {Label: "B"}}}
	// Accessor lazy-inits to 0 without a prior constructor call.
	if got := n.Active().Get(); got != 0 {
		t.Fatalf("bare &Notebook{} Active().Get() = %d, want 0", got)
	}
	seen := -1
	n.Active().Subscribe(func(i int) { seen = i })
	n.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	n.OnEvent(kd("ArrowRight")) // 0 -> 1
	if n.Active().Get() != 1 || seen != 1 {
		t.Fatalf("bound accessor: Active=%d seen=%d, want 1/1", n.Active().Get(), seen)
	}
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
	n.Active().Set(5) // out of range to exercise Draw/OnEvent guards
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
	if p.Position().Get() != 100 {
		t.Fatalf("default position = %d, want 100", p.Position().Get())
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
	if p.Position().Get() != 40 {
		t.Fatalf("default position = %d, want 40", p.Position().Get())
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
	if p.Position().Get() != 10 {
		t.Fatalf("clamp low: Position = %d, want 10", p.Position().Get())
	}
	p.MoveHandle(500)
	if p.Position().Get() != 190 {
		t.Fatalf("clamp high: Position = %d, want 190", p.Position().Get())
	}
}

func TestPanedMoveHandleVerticalClamps(t *testing.T) {
	p := NewVPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.MoveHandle(500)
	if p.Position().Get() != 70 {
		t.Fatalf("vertical high clamp = %d, want 70", p.Position().Get())
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
	// Handle at x=30..36 painted with the distinct splitter tone (interior
	// fill point (32,4), away from the centre grip + the Border edges).
	want := blendRGBA(theme.SurfaceAlt, theme.Border, 0.45)
	if got := pixelAt(buf, w, 32, 4); got != want {
		t.Fatalf("horizontal handle fill = %+v, want splitter tone %+v", got, want)
	}
	// Left long edge is the Border delineator.
	if got := pixelAt(buf, w, 30, 4); got != theme.Border {
		t.Fatalf("horizontal handle left edge = %+v, want Border", got)
	}
}

func TestPanedDrawVertical(t *testing.T) {
	const w, h = 64, 60
	theme := DefaultLight()
	p := NewVPaned(&recordingWidget{}, &recordingWidget{})
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	buf := makeSurface(w, h)
	p.Draw(newP(buf, w), theme)
	// Handle at y=30..36 painted with the distinct splitter tone (interior
	// fill point (10,32), away from the centre grip + the Border edges).
	want := blendRGBA(theme.SurfaceAlt, theme.Border, 0.45)
	if got := pixelAt(buf, w, 10, 32); got != want {
		t.Fatalf("vertical handle fill = %+v, want splitter tone %+v", got, want)
	}
	// Top long edge is the Border delineator.
	if got := pixelAt(buf, w, 10, 30); got != theme.Border {
		t.Fatalf("vertical handle top edge = %+v, want Border", got)
	}
	// Centre grip dot is OnSurface.
	if got := pixelAt(buf, w, 30, 33); got != theme.OnSurface {
		t.Fatalf("vertical handle centre grip = %+v, want OnSurface", got)
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

func TestPanedEventRoutingTranslatesAtNonZeroBounds(t *testing.T) {
	// The regression the old tests missed: with the Paned NOT at the origin, the
	// forwarded event must be translated into each child's local frame.
	a := &recordingWidget{}
	b := &recordingWidget{}
	p := NewHPaned(a, b)
	p.SetBounds(Rect{X: 100, Y: 50, W: 200, H: 80}) // Position defaults to 100
	// A Paned-local click at X=120 lands in the Second pane (>Position+handle).
	p.OnEvent(Event{Kind: EventClick, X: 120, Y: 10})
	if len(b.events) != 1 {
		t.Fatalf("second pane got %d events, want 1", len(b.events))
	}
	// Second pane's absolute X = 100+100+6 = 206; child-local X = 120+100-206 = 14.
	if got := b.events[0]; got.X != 14 || got.Y != 10 {
		t.Fatalf("second pane received %+v, want local {14,10}", got)
	}
	// First pane (offset 0) is unchanged by translation.
	p.OnEvent(Event{Kind: EventClick, X: 20, Y: 10})
	if len(a.events) != 1 || a.events[0].X != 20 || a.events[0].Y != 10 {
		t.Fatalf("first pane received %+v, want {20,10}", a.events[0])
	}

	// Vertical: same translation on Y.
	c := &recordingWidget{}
	d := &recordingWidget{}
	vp := NewVPaned(c, d)
	vp.SetBounds(Rect{X: 100, Y: 50, W: 200, H: 80}) // Position defaults to 40
	vp.OnEvent(Event{Kind: EventClick, X: 5, Y: 50}) // >Position+handle → Second
	if len(d.events) != 1 {
		t.Fatalf("vertical second got %d events, want 1", len(d.events))
	}
	// Second abs Y = 50+40+6 = 96; child-local Y = 50+50-96 = 4.
	if got := d.events[0]; got.X != 5 || got.Y != 4 {
		t.Fatalf("vertical second received %+v, want local {5,4}", got)
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
	p := &Paned{First: nil, Second: b, Orientation: PanedHorizontal}
	p.Position().Set(50)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	p.OnEvent(Event{Kind: EventClick, X: 20, Y: 10})  // left -> nil first
	p.OnEvent(Event{Kind: EventClick, X: 180, Y: 10}) // right -> Second
	if len(b.events) != 1 {
		t.Fatalf("right click should reach Second; got %d", len(b.events))
	}
}

func TestPanedNilSecondVerticalNoCrash(t *testing.T) {
	a := &recordingWidget{}
	p := &Paned{First: a, Second: nil, Orientation: PanedVertical}
	p.Position().Set(30)
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
	e.Expanded().Subscribe(func(v bool) { expanded = v })
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !e.Expanded().Get() || !expanded {
		t.Fatalf("after header click: Expanded=%v expanded=%v", e.Expanded().Get(), expanded)
	}
	e.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if e.Expanded().Get() || expanded {
		t.Fatal("second click should collapse")
	}
}

func TestExpanderClickBodyRoutesWhenExpanded(t *testing.T) {
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.Expanded().Set(true)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	if len(body.events) != 1 {
		t.Fatalf("body click: got %d events", len(body.events))
	}
}

func TestExpanderBodyClickTranslatedAtNonZeroBounds(t *testing.T) {
	// Regression: expanded-body clicks must arrive in the content's local frame
	// (shifted up by ExpanderHeaderH and by the Expander's own origin).
	body := &recordingWidget{}
	e := NewExpander("S", body)
	e.Expanded().Set(true)
	e.SetBounds(Rect{X: 30, Y: 20, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 40}) // expander-local, in the body
	if len(body.events) != 1 {
		t.Fatalf("body click routed %d events, want 1", len(body.events))
	}
	// Content origin: X == Expander origin (offset 0), Y is ExpanderHeaderH below.
	if got := body.events[0]; got.X != 5 || got.Y != 40-ExpanderHeaderH {
		t.Fatalf("content received %+v, want {5,%d}", got, 40-ExpanderHeaderH)
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
	e.Expanded().Set(true)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.Draw(newP(make([]byte, 200*100*4), 200), DefaultLight())
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
}

func TestExpanderIgnoresNonClick(t *testing.T) {
	body := &recordingWidget{}
	e := NewExpander("S", body)
	// Enter/Space toggle the header as of Wave 3; an unrelated key (Tab) must
	// neither toggle nor propagate to the body.
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if e.Expanded().Get() || len(body.events) != 0 {
		t.Fatal("KeyDown must not toggle or propagate")
	}
}

// TestExpanderBareAccessorInitsAndBinds proves a zero-value &Expander{} (built
// without the constructor) lazily allocates its expanded Observable on the first
// Expanded() call — defaulting collapsed — and that a header click drives the
// same Observable a host binds via Subscribe.
func TestExpanderBareAccessorInitsAndBinds(t *testing.T) {
	e := &Expander{}
	if e.Expanded().Get() {
		t.Fatal("bare Expander should start collapsed")
	}
	seen := false
	e.Expanded().Subscribe(func(v bool) { seen = v })
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // header click toggles
	if !e.Expanded().Get() || !seen {
		t.Fatalf("host bind: Expanded=%v subscriber=%v, want true/true", e.Expanded().Get(), seen)
	}
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
	e.Expanded().Set(true)
	e.Draw(newP(makeSurface(w, h), w), theme)
	if body.draws != 1 {
		t.Fatal("expanded Draw must render body")
	}
}
