// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// This file covers the Wave 1 framework enablers: EventMouseMove routing
// through the containers, the hover-on-move faces the button family / menus /
// charts now set for themselves, and the shared Base.Disabled state (inert
// OnEvent + muted Draw) across the interactive widgets.

// --- EventMouseMove routing through containers ---------------------------

// twoButtons returns two buttons wired into a horizontal box-like layout so a
// move over one hovers it and clears the other (hover-enter + hover-leave).
func newHoverPair() (*Button, *Button) {
	return NewButton("A", nil), NewButton("B", nil)
}

func TestHBoxRoutesMouseMoveHoverEnterLeave(t *testing.T) {
	h := NewHBox()
	b1, b2 := newHoverPair()
	h.Append(b1)
	h.Append(b2)
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20}) // each ~48px, gap 4

	h.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10}) // over b1
	if !b1.hovered || b2.hovered {
		t.Fatalf("after move over b1: b1.hovered=%v b2.hovered=%v, want true/false", b1.hovered, b2.hovered)
	}
	h.OnEvent(Event{Kind: EventMouseMove, X: 90, Y: 10}) // over b2
	if b1.hovered || !b2.hovered {
		t.Fatalf("after move over b2: b1.hovered=%v b2.hovered=%v, want false/true (b1 cleared on move-away)", b1.hovered, b2.hovered)
	}
}

func TestVBoxRoutesMouseMove(t *testing.T) {
	v := NewVBox()
	b1, b2 := newHoverPair()
	v.Append(b1)
	v.Append(b2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 100})
	v.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10}) // over b1
	if !b1.hovered || b2.hovered {
		t.Fatalf("VBox move over b1: %v/%v", b1.hovered, b2.hovered)
	}
	v.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 90}) // over b2
	if b1.hovered || !b2.hovered {
		t.Fatalf("VBox move over b2: %v/%v", b1.hovered, b2.hovered)
	}
}

func TestGridRoutesMouseMove(t *testing.T) {
	g := NewGrid(2, 1)
	b1, b2 := newHoverPair()
	g.Attach(b1, 0, 0)
	g.Attach(b2, 1, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20}) // cols 50px each
	g.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10})
	if !b1.hovered || b2.hovered {
		t.Fatalf("Grid move over b1: %v/%v", b1.hovered, b2.hovered)
	}
	g.OnEvent(Event{Kind: EventMouseMove, X: 70, Y: 10})
	if b1.hovered || !b2.hovered {
		t.Fatalf("Grid move over b2: %v/%v", b1.hovered, b2.hovered)
	}
}

func TestContainerRoutesMouseMove(t *testing.T) {
	c := NewContainer(NewBoxLayout()) // horizontal box
	b1, b2 := newHoverPair()
	c.AddWidget(b1)
	c.AddWidget(b2)
	c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	c.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10})
	if !b1.hovered || b2.hovered {
		t.Fatalf("Container move over b1: %v/%v", b1.hovered, b2.hovered)
	}
	c.OnEvent(Event{Kind: EventMouseMove, X: 90, Y: 10})
	if b1.hovered || !b2.hovered {
		t.Fatalf("Container move over b2: %v/%v", b1.hovered, b2.hovered)
	}
	// A non-move event still routes to a single child (the click path).
	fired := 0
	b3 := NewButton("C", func() { fired++ })
	c2 := NewContainer(NewBoxLayout())
	c2.AddWidget(b3)
	c2.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	c2.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if fired != 1 {
		t.Fatalf("Container click routing broke: fired=%d", fired)
	}
}

func TestFrameForwardsMouseMoveAndClears(t *testing.T) {
	b := NewButton("X", nil)
	f := NewFrame(b)
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40}) // child inset by 5px
	f.OnEvent(Event{Kind: EventMouseMove, X: 20, Y: 20})
	if !b.hovered {
		t.Fatal("Frame should forward a move inside the child, setting hover")
	}
	// A move inside the frame but outside the child (the 5px inset) clears it.
	f.OnEvent(Event{Kind: EventMouseMove, X: 2, Y: 2})
	if b.hovered {
		t.Fatal("Frame move outside the child should clear the child's hover")
	}
}

// --- Button family: hover-on-move faces ----------------------------------

func TestButtonHoverFaceOnMove(t *testing.T) {
	th := DefaultLight()
	b := NewButton("X", nil)
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	b.OnEvent(Event{Kind: EventMouseMove, X: 20, Y: 10}) // inside
	if !b.hovered {
		t.Fatal("move inside should set hovered")
	}
	buf := makeSurface(40, 20)
	b.Draw(newP(buf, 40), th)
	if px := pixelAt(buf, 40, 4, 10); px != th.SurfaceAlt {
		t.Fatalf("hover face = %+v, want SurfaceAlt", px)
	}
	b.OnEvent(Event{Kind: EventMouseMove, X: 100, Y: 10}) // outside
	if b.hovered {
		t.Fatal("move outside should clear hovered")
	}
}

func TestIconButtonHoverPressAndDisabled(t *testing.T) {
	th := DefaultLight()
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})

	// Hover face (SurfaceAlt).
	ib.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})
	if !ib.hovered {
		t.Fatal("icon move should set hover")
	}
	hov := makeSurface(30, 30)
	ib.Draw(newP(hov, 30), th)
	if px := pixelAt(hov, 30, 2, 2); px != th.SurfaceAlt {
		t.Fatalf("icon hover face = %+v, want SurfaceAlt", px)
	}
	// Press face (Accent) + fires OnClick, then release.
	clicked := 0
	ib.OnClick = func() { clicked++ }
	ib.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if !ib.pressed || clicked != 1 {
		t.Fatalf("icon click: pressed=%v clicked=%d", ib.pressed, clicked)
	}
	prs := makeSurface(30, 30)
	ib.Draw(newP(prs, 30), th)
	if px := pixelAt(prs, 30, 2, 2); px != th.Accent {
		t.Fatalf("icon press face = %+v, want Accent", px)
	}
	ib.OnEvent(Event{Kind: EventMouseUp})
	if ib.pressed {
		t.Fatal("mouseup should release icon button")
	}
	// Disabled: click is inert + face is muted.
	ib.Disabled = true
	ib.hovered, ib.pressed = false, false
	clicked = 0
	ib.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	ib.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})
	if clicked != 0 || ib.pressed || ib.hovered {
		t.Fatalf("disabled icon reacted: clicked=%d pressed=%v hovered=%v", clicked, ib.pressed, ib.hovered)
	}
	dis := makeSurface(30, 30)
	ib.Draw(newP(dis, 30), th)
	if px := pixelAt(dis, 30, 2, 2); px != mutedFace(th) {
		t.Fatalf("disabled icon face = %+v, want mutedFace %+v", px, mutedFace(th))
	}
}

func TestToggleButtonHoverAndDisabled(t *testing.T) {
	th := DefaultLight()
	tb := NewToggleButton("T", false)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	// Hover raises the unpressed face to SurfaceAlt.
	tb.OnEvent(Event{Kind: EventMouseMove, X: 20, Y: 10})
	if !tb.hovered {
		t.Fatal("toggle move should set hover")
	}
	hov := makeSurface(40, 20)
	tb.Draw(newP(hov, 40), th)
	if px := pixelAt(hov, 40, 4, 10); px != th.SurfaceAlt {
		t.Fatalf("toggle hover face = %+v, want SurfaceAlt", px)
	}
	// Click toggles Pressed (Accent face wins over hover).
	toggled := 0
	tb.Pressed().Subscribe(func(bool) { toggled++ })
	tb.OnEvent(Event{Kind: EventClick})
	if !tb.Pressed().Get() || toggled != 1 {
		t.Fatalf("toggle click: pressed=%v toggled=%d", tb.Pressed().Get(), toggled)
	}
	// Disabled: inert + muted.
	tb.Disabled = true
	tb.Pressed().Set(false)
	tb.hovered = false
	toggled = 0
	tb.OnEvent(Event{Kind: EventClick})
	tb.OnEvent(Event{Kind: EventMouseMove, X: 20, Y: 10})
	if tb.Pressed().Get() || toggled != 0 || tb.hovered {
		t.Fatalf("disabled toggle reacted: pressed=%v toggled=%d hovered=%v", tb.Pressed().Get(), toggled, tb.hovered)
	}
	dis := makeSurface(40, 20)
	tb.Draw(newP(dis, 40), th)
	if px := pixelAt(dis, 40, 4, 10); px != mutedFace(th) {
		t.Fatalf("disabled toggle face = %+v, want mutedFace", px)
	}
}

func TestSplitButtonHoverAndDisabled(t *testing.T) {
	th := DefaultLight()
	s := NewSplitButton("Go", nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})
	if !s.hovered {
		t.Fatal("split move should set hover")
	}
	hov := makeSurface(100, 20)
	s.Draw(newP(hov, 100), th)
	if px := pixelAt(hov, 100, 5, 5); px != brighter(th.Accent) {
		t.Fatalf("split hover face = %+v, want brighter(Accent) %+v", px, brighter(th.Accent))
	}
	// Move off clears.
	s.OnEvent(Event{Kind: EventMouseMove, X: 500, Y: 5})
	if s.hovered {
		t.Fatal("split move off should clear hover")
	}
	// Disabled: inert + muted.
	clicked := 0
	s.OnClick = func() { clicked++ }
	s.Disabled = true
	s.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	s.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})
	if clicked != 0 || s.hovered {
		t.Fatalf("disabled split reacted: clicked=%d hovered=%v", clicked, s.hovered)
	}
	dis := makeSurface(100, 20)
	s.Draw(newP(dis, 100), th)
	if px := pixelAt(dis, 100, 5, 5); px != mutedFace(th) {
		t.Fatalf("disabled split face = %+v, want mutedFace", px)
	}
}

// --- Disabled-only interactive widgets -----------------------------------

func TestCycleButtonDisabled(t *testing.T) {
	th := DefaultLight()
	c := NewCycleButton("A", "B")
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	c.Disabled = true
	c.OnEvent(Event{Kind: EventClick})
	if c.Value() != "A" {
		t.Fatal("disabled cycle should not advance")
	}
	buf := makeSurface(80, 24)
	c.Draw(newP(buf, 80), th)
	if px := pixelAt(buf, 80, 12, 12); px != mutedFace(th) {
		t.Fatalf("disabled cycle face = %+v, want mutedFace", px)
	}
}

func TestCheckButtonDisabled(t *testing.T) {
	th := DefaultLight()
	c := NewCheckButton("L", false)
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	c.Disabled = true
	c.OnEvent(Event{Kind: EventClick})
	if c.Checked().Get() {
		t.Fatal("disabled checkbox should not toggle")
	}
	buf := makeSurface(60, 20)
	c.Draw(newP(buf, 60), th)
	if px := pixelAt(buf, 60, 2, 8); px != mutedFace(th) {
		t.Fatalf("disabled check box = %+v, want mutedFace", px)
	}
	// Disabled + checked exercises the muted checkmark path (Sized + fixed).
	c.Checked().Set(true)
	c.Draw(newP(makeSurface(60, 20), 60), th)
	c.Size = 16
	c.Draw(newP(makeSurface(60, 20), 60), th)
}

func TestRadioButtonDisabled(t *testing.T) {
	th := DefaultLight()
	r := NewRadioButton("R")
	r.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	r.Disabled = true
	r.OnEvent(Event{Kind: EventClick})
	if r.Checked().Get() {
		t.Fatal("disabled radio should not toggle")
	}
	r.Checked().Set(true) // exercise the muted dot path
	buf := makeSurface(60, 20)
	r.Draw(newP(buf, 60), th)
	if px := pixelAt(buf, 60, 1, 8); px != mutedFace(th) {
		t.Fatalf("disabled radio mark = %+v, want mutedFace", px)
	}
}

func TestSwitchDisabled(t *testing.T) {
	th := DefaultLight()
	s := NewSwitch(false)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	s.Disabled = true
	s.OnEvent(Event{Kind: EventClick})
	if s.On().Get() {
		t.Fatal("disabled switch should not flip")
	}
	buf := makeSurface(40, 20)
	s.Draw(newP(buf, 40), th)
	if px := pixelAt(buf, 40, 30, 10); px != mutedFace(th) {
		t.Fatalf("disabled switch track = %+v, want mutedFace", px)
	}
}

func TestScaleDisabled(t *testing.T) {
	th := DefaultLight()
	s := NewScale(0, 100, 50)
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	s.Disabled = true
	s.OnEvent(Event{Kind: EventClick, X: 90, Y: 10})
	if s.Value().Get() != 50 {
		t.Fatalf("disabled scale moved to %v", s.Value().Get())
	}
	buf := makeSurface(100, 20)
	s.Draw(newP(buf, 100), th)
	if px := pixelAt(buf, 100, 90, 10); px != mutedFace(th) {
		t.Fatalf("disabled scale track = %+v, want mutedFace", px)
	}
	// Vertical disabled path too.
	sv := NewScale(0, 100, 50)
	sv.Orientation = Vertical
	sv.Disabled = true
	sv.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 100})
	sv.Draw(newP(makeSurface(20, 100), 20), th)
}

func TestSpinButtonDisabled(t *testing.T) {
	th := DefaultLight()
	s := NewSpinButton(0, 10, 5, 1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	s.Disabled = true
	s.OnEvent(Event{Kind: EventClick, X: 55, Y: 5}) // upper button
	if s.Value().Get() != 5 {
		t.Fatalf("disabled spin changed to %d", s.Value().Get())
	}
	buf := makeSurface(60, 20)
	s.Draw(newP(buf, 60), th)
	if px := pixelAt(buf, 60, 30, 3); px != mutedFace(th) {
		t.Fatalf("disabled spin body = %+v, want mutedFace", px)
	}
}

func TestDropDownDisabled(t *testing.T) {
	th := DefaultLight()
	d := NewDropDown([]string{"a", "b"}, 0)
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 20})
	d.Disabled = true
	d.OnEvent(Event{Kind: EventClick})
	if d.Open().Get() {
		t.Fatal("disabled dropdown should not open")
	}
	buf := makeSurface(80, 20)
	d.Draw(newP(buf, 80), th)
	if px := pixelAt(buf, 80, 2, 3); px != mutedFace(th) {
		t.Fatalf("disabled dropdown body = %+v, want mutedFace", px)
	}
}

func TestComboBoxDisabled(t *testing.T) {
	th := DefaultLight()
	c := NewComboBox([]string{"a", "b"})
	c.Placeholder = "pick"
	c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 24})
	c.Disabled = true
	c.OnEvent(Event{Kind: EventChar, Code: "z"})
	c.OnEvent(Event{Kind: EventClick})
	if c.Text().Get() != "" || c.Open().Get() {
		t.Fatalf("disabled combobox reacted: text=%q open=%v", c.Text().Get(), c.Open().Get())
	}
	buf := makeSurface(100, 24)
	c.Draw(newP(buf, 100), th)
	if px := pixelAt(buf, 100, 2, 12); px != mutedFace(th) {
		t.Fatalf("disabled combobox body = %+v, want mutedFace", px)
	}
}

// --- Menus: hover-on-move ------------------------------------------------

func TestMenuHoverOnMove(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "A", Action: func() {}}, {Label: "B", Action: func() {}}})
	m.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	m.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10}) // row 0
	if got := m.Hover().Get(); got != 0 {
		t.Fatalf("menu hover = %d, want 0", got)
	}
	m.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5}) // off the menu
	if got := m.Hover().Get(); got != -1 {
		t.Fatalf("menu hover after move-off = %d, want -1", got)
	}
}

func TestMenuBarHoverOnMove(t *testing.T) {
	th := DefaultLight()
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	b.AddMenu("Edit", NewMenu(nil))
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: MenuBarH})
	b.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5}) // over "File"
	if b.hoverName != 1 {
		t.Fatalf("menubar hoverName = %d, want 1", b.hoverName)
	}
	// Draw paints the hover highlight (Surface) on the hovered, non-active name.
	buf := makeSurface(200, MenuBarH)
	b.Draw(newP(buf, 200), th)
	if px := pixelAt(buf, 200, 1, 1); px != th.Surface {
		t.Fatalf("menubar hover highlight = %+v, want Surface", px)
	}
	// Move below the strip clears the hover.
	b.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: MenuBarH + 5})
	if b.hoverName != 0 {
		t.Fatalf("menubar hoverName after off-strip = %d, want 0", b.hoverName)
	}
	// Move past every name (no match) also leaves it clear.
	b.OnEvent(Event{Kind: EventMouseMove, X: 10000, Y: 5})
	if b.hoverName != 0 {
		t.Fatalf("menubar hoverName past names = %d, want 0", b.hoverName)
	}
}

func TestContextMenuForwardsMove(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "A", Action: func() {}}})
	cm := NewContextMenu(m)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	cm.Popup(10, 10)
	cm.OnEvent(Event{Kind: EventMouseMove, X: 15, Y: 15}) // inside the menu
	if got := m.Hover().Get(); got != 0 {
		t.Fatalf("contextmenu did not forward move: menu.Hover=%d", got)
	}
	// A non-click, non-move event is ignored (not dismissed).
	cm.OnEvent(Event{Kind: EventKeyDown, Code: "x"})
	if !cm.Open {
		t.Fatal("keydown should not dismiss the context menu")
	}
	// Closed: a move is a no-op.
	cm.Close()
	cm.OnEvent(Event{Kind: EventMouseMove, X: 15, Y: 15})
}

// --- Charts: hover-on-move ------------------------------------------------

func TestChartsHoverOnMove(t *testing.T) {
	bounds := Rect{X: 10, Y: 10, W: 130, H: 100}
	inside := Event{Kind: EventMouseMove, X: 60, Y: 40}
	// emptyBounded builds a chart with real bounds but no data, so an in-bounds
	// move resolves ok=false and exercises the "clear Hover" else-branch.
	emptyBounded := func(w Widget) {
		w.SetBounds(bounds)
		w.OnEvent(inside)
		w.OnEvent(Event{Kind: EventClick, X: 60, Y: 40}) // non-move ignored
	}

	lc := NewLineChart([]float64{1, 4, 2, 5})
	lc.SetBounds(bounds)
	lc.OnEvent(inside)
	if !lc.Hover {
		t.Fatal("line move should set Hover")
	}
	lc.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5})
	if lc.Hover {
		t.Fatal("line move off should clear Hover")
	}
	emptyBounded(NewLineChart(nil))

	ac := NewAreaChart([][]float64{{1, 4, 2, 5}})
	ac.SetBounds(bounds)
	ac.OnEvent(inside)
	if !ac.Hover {
		t.Fatal("area move should set Hover")
	}
	ac.OnEvent(Event{Kind: EventMouseMove, X: 500, Y: 40})
	if ac.Hover {
		t.Fatal("area move off should clear Hover")
	}
	emptyBounded(NewAreaChart(nil))

	bc := NewBarChart([]float64{4, 7, 2})
	bc.SetBounds(bounds)
	bc.OnEvent(inside)
	if !bc.Hover {
		t.Fatal("bar move should set Hover")
	}
	bc.OnEvent(Event{Kind: EventMouseMove, X: -1, Y: 40})
	if bc.Hover {
		t.Fatal("bar move off should clear Hover")
	}
	emptyBounded(NewBarChart(nil))

	sl := NewSparkline([]float64{3, 7, 2, 8})
	sl.SetBounds(bounds)
	sl.OnEvent(inside)
	if !sl.Hover().Get() {
		t.Fatal("sparkline move should set Hover")
	}
	sl.OnEvent(Event{Kind: EventMouseMove, X: 60, Y: -5})
	if sl.Hover().Get() {
		t.Fatal("sparkline move off should clear Hover")
	}
	emptyBounded(NewSparkline(nil))

	sc := NewScatterChart([][]ScatterPoint{{{X: 1, Y: 2}, {X: 3, Y: 5}}})
	sc.SetBounds(bounds)
	xr, yr, _ := sc.ranges()
	px, py := sc.project(sc.Series[0][1], xr, yr)
	r := sc.Bounds()
	sc.OnEvent(Event{Kind: EventMouseMove, X: px - r.X, Y: py - r.Y})
	if !sc.Hover {
		t.Fatal("scatter move should set Hover")
	}
	sc.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5})
	if sc.Hover {
		t.Fatal("scatter move off should clear Hover")
	}
	emptyBounded(NewScatterChart(nil))

	pc := NewPieChart([]float64{3, 5, 2})
	pc.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 100})
	pc.OnEvent(Event{Kind: EventMouseMove, X: 52, Y: 30}) // just clockwise of 12 o'clock
	if !pc.Hover {
		t.Fatal("pie move should set Hover")
	}
	pc.OnEvent(Event{Kind: EventMouseMove, X: 0, Y: 0}) // corner: inside bounds but outside the disc
	if pc.Hover {
		t.Fatal("pie move off the disc should clear Hover")
	}
	pc.HoverIndex, pc.Hover = 1, true
	pc.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5}) // off the widget entirely
	if pc.Hover {
		t.Fatal("pie move off the widget should clear Hover")
	}
	emptyBounded(NewPieChart(nil))

	rc := NewRadarChart([]string{"A", "B", "C"}, [][]float64{{8, 6, 7}})
	rc.SetBounds(bounds)
	rc.OnEvent(Event{Kind: EventMouseMove, X: 65, Y: 50}) // near centre → axis 0
	if !rc.Hover {
		t.Fatal("radar move should set Hover")
	}
	rc.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5})
	if rc.Hover {
		t.Fatal("radar move off should clear Hover")
	}
	emptyBounded(NewRadarChart(nil, nil))
}

// --- Button disabled + Base helpers --------------------------------------

func TestButtonDisabled(t *testing.T) {
	th := DefaultLight()
	fired := 0
	b := NewButton("X", func() { fired++ })
	b.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	b.Disabled = true
	b.OnEvent(Event{Kind: EventClick})
	b.OnEvent(Event{Kind: EventMouseMove, X: 30, Y: 10})
	if fired != 0 || b.pressed || b.hovered {
		t.Fatalf("disabled button reacted: fired=%d pressed=%v hovered=%v", fired, b.pressed, b.hovered)
	}
	buf := makeSurface(60, 20)
	b.Draw(newP(buf, 60), th)
	if px := pixelAt(buf, 60, 4, 10); px != mutedFace(th) {
		t.Fatalf("disabled button face = %+v, want mutedFace %+v", px, mutedFace(th))
	}
}

func TestMutedHelpersDifferFromBase(t *testing.T) {
	th := DefaultLight()
	if mutedFace(th) == th.SurfaceAlt || mutedInk(th) == th.OnSurface {
		t.Fatal("muted helpers should differ from the un-muted tones")
	}
	var base Base
	base.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	if !base.localInBounds(5, 5) || base.localInBounds(-1, 5) || base.localInBounds(5, 20) {
		t.Fatal("localInBounds bounds check wrong")
	}
}
