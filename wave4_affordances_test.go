// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Wave 4 wires pointer events into affordances that were drawn but inert: a
// drawn thumb/knob/crumb/badge whose click or drag was never handled. Each test
// asserts the specific mutate+callback path, at cell/coordinate precision.

// --- 1. Scale: draggable thumb --------------------------------------------

func TestScaleDragScrubsValue(t *testing.T) {
	got := []float64{}
	s := NewScale(0, 100, 0)
	s.Value().Subscribe(func(v float64) { got = append(got, v) })
	// Track the thumb centre travels is W-scaleThumbSize == 100.
	s.SetBounds(Rect{X: 0, Y: 0, W: 100 + scaleThumbSize, H: 20})

	// A drag whose x maps to pos 0.5 sets Value 50 (ev.X = thumbHalf + 50).
	s.OnEvent(Event{Kind: EventMouseDrag, X: scaleThumbSize/2 + 50, Y: 10})
	if s.Value().Get() != 50 {
		t.Fatalf("drag value = %v, want 50", s.Value().Get())
	}
	// A second drag further right keeps scrubbing the same thumb.
	s.OnEvent(Event{Kind: EventMouseDrag, X: scaleThumbSize/2 + 80, Y: 10})
	if s.Value().Get() != 80 {
		t.Fatalf("second drag value = %v, want 80", s.Value().Get())
	}
	if len(got) != 2 {
		t.Fatalf("Value subscriber fired %d times, want 2", len(got))
	}
}

func TestScaleVerticalDragScrubsValue(t *testing.T) {
	s := NewScale(0, 100, 0)
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 100 + scaleThumbSize})
	// Vertical is flipped (top = Max): pos = 1 - (y-half)/span. y = half+50 → 0.5.
	s.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: scaleThumbSize/2 + 50})
	if s.Value().Get() != 50 {
		t.Fatalf("vertical drag value = %v, want 50", s.Value().Get())
	}
}

// --- 3. Breadcrumbs: OnSelect on crumb click ------------------------------

func TestBreadcrumbsClickFiresOnSelect(t *testing.T) {
	b := NewBreadcrumbs([]string{"Home", "Docs", "Reference"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 20})
	got := -1
	b.OnSelect = func(i int) { got = i }

	w0 := b.textWidth("Home")
	step := b.textWidth(BreadcrumbSep) + 2*BreadcrumbGap // gap+sep+gap between crumbs

	// Click the middle of crumb 0.
	b.OnEvent(Event{Kind: EventClick, X: w0 / 2, Y: 10})
	if got != 0 {
		t.Fatalf("crumb 0 click: got %d, want 0", got)
	}
	// Click the middle of crumb 1.
	x1 := w0 + step
	b.OnEvent(Event{Kind: EventClick, X: x1 + b.textWidth("Docs")/2, Y: 10})
	if got != 1 {
		t.Fatalf("crumb 1 click: got %d, want 1", got)
	}
	// A click in the separator gap between crumbs 0 and 1 hits no crumb.
	got = -9
	b.OnEvent(Event{Kind: EventClick, X: w0 + BreadcrumbGap, Y: 10})
	if got != -9 {
		t.Fatalf("gap click fired OnSelect: got %d", got)
	}
}

func TestBreadcrumbsInertWhenNilOrNonClick(t *testing.T) {
	b := NewBreadcrumbs([]string{"A", "B"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	// Nil OnSelect: a click is a safe no-op.
	b.OnEvent(Event{Kind: EventClick, X: 1, Y: 5})
	// A non-click never fires even with OnSelect set.
	fired := false
	b.OnSelect = func(int) { fired = true }
	b.OnEvent(Event{Kind: EventMouseMove, X: 1, Y: 5})
	if fired {
		t.Fatal("non-click fired OnSelect")
	}
}

// --- 4. Steps: badge click Sets Current -----------------------------------

func TestStepsClickJumpsHorizontal(t *testing.T) {
	s := NewSteps([]string{"A", "B", "C"}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40}) // H > StepBoxH: badges vertically centred
	got := -1
	s.Current().Subscribe(func(i int) { got = i })

	// Badge i left = i*(StepBoxW+StepConnectorW); row centred at (40-16)/2 = 12.
	bx := 2 * (StepBoxW + StepConnectorW)
	s.OnEvent(Event{Kind: EventClick, X: bx + StepBoxW/2, Y: 12 + StepBoxH/2})
	if got != 2 || s.Current().Get() != 2 {
		t.Fatalf("badge 2 click: got %d Current %d, want 2/2", got, s.Current().Get())
	}
	// A click on the connector between badges hits nothing (Current unchanged).
	got = -9
	s.OnEvent(Event{Kind: EventClick, X: StepBoxW + StepConnectorW/2, Y: 12 + StepBoxH/2})
	if got != -9 || s.Current().Get() != 2 {
		t.Fatalf("connector click fired: got %d Current %d", got, s.Current().Get())
	}
}

func TestStepsClickJumpsVertical(t *testing.T) {
	s := NewSteps([]string{"A", "B"}, -1)
	s.Orientation = Vertical
	s.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	got := -1
	s.Current().Subscribe(func(i int) { got = i })
	// Badge 1 top = 1*(StepBoxH+StepConnectorW); column pinned at x in [0,StepBoxW).
	by := StepBoxH + StepConnectorW
	s.OnEvent(Event{Kind: EventClick, X: StepBoxW / 2, Y: by + StepBoxH/2})
	if got != 1 || s.Current().Get() != 1 {
		t.Fatalf("vertical badge 1 click: got %d Current %d", got, s.Current().Get())
	}
}

func TestStepsInertWhenNonClickOrShortBar(t *testing.T) {
	// Short bar (H <= StepBoxH): yOff branch stays 0; badge 0 still clickable.
	// Start at -1 so a jump to badge 0 is a real change the subscriber sees.
	s := NewSteps([]string{"A"}, -1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: StepBoxH})
	got := -9
	s.Current().Subscribe(func(i int) { got = i })
	s.OnEvent(Event{Kind: EventClick, X: StepBoxW / 2, Y: StepBoxH / 2})
	if got != 0 || s.Current().Get() != 0 {
		t.Fatalf("short-bar badge click: got %d Current %d", got, s.Current().Get())
	}
	// Non-click: ignored — the Current Observable is never Set.
	s2 := NewSteps([]string{"A"}, -1)
	s2.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	fired := false
	s2.Current().Subscribe(func(int) { fired = true })
	s2.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 15})
	if fired {
		t.Fatal("non-click Set Current")
	}
}

// --- 5. TextView: click-to-caret + drag-select ----------------------------

func TestTextViewClickPlacesCaretAndDragSelects(t *testing.T) {
	tv := NewTextView("hello\nworld\nfoobar")
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	lineH := tv.glyphHeight() + 4
	adv := tv.glyphAdvance()

	// Click line 1 ("world"), col 2.
	tv.OnEvent(Event{Kind: EventClick, X: 4 + 2*adv, Y: 4 + 1*lineH})
	if !tv.Focused().Get() {
		t.Fatal("click did not focus")
	}
	if tv.CursorLine().Get() != 1 || tv.CursorCol().Get() != 2 {
		t.Fatalf("caret = (%d,%d), want (1,2)", tv.CursorLine().Get(), tv.CursorCol().Get())
	}
	if !tv.Selection().Get().IsEmpty() {
		t.Fatal("click should collapse the selection at the caret")
	}
	// Drag to line 2 col 3 extends the selection from the click anchor.
	tv.OnEvent(Event{Kind: EventMouseDrag, X: 4 + 3*adv, Y: 4 + 2*lineH})
	if tv.CursorLine().Get() != 2 || tv.CursorCol().Get() != 3 {
		t.Fatalf("drag caret = (%d,%d), want (2,3)", tv.CursorLine().Get(), tv.CursorCol().Get())
	}
	if want := (SelectionRange(1, 2, 2, 3)); tv.Selection().Get() != want {
		t.Fatalf("selection = %+v, want %+v", tv.Selection().Get(), want)
	}
}

func TestTextViewCaretClampsAndEmptyGuard(t *testing.T) {
	tv := NewTextView("hi\nlongerline")
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	adv := tv.glyphAdvance()
	// Click far below the last line clamps to the last line; far right clamps to
	// that line's length.
	tv.OnEvent(Event{Kind: EventClick, X: 4 + 999*adv, Y: 4000})
	if tv.CursorLine().Get() != 1 {
		t.Fatalf("below-buffer click line = %d, want 1 (clamped)", tv.CursorLine().Get())
	}
	if tv.CursorCol().Get() != len([]rune("longerline")) {
		t.Fatalf("far-right col = %d, want %d", tv.CursorCol().Get(), len([]rune("longerline")))
	}
	// A drag whose x maps left of column 0 clamps to col 0, and y < 4 → line 0.
	tv.OnEvent(Event{Kind: EventMouseDrag, X: -50, Y: 0})
	if tv.CursorLine().Get() != 0 || tv.CursorCol().Get() != 0 {
		t.Fatalf("clamp-left caret = (%d,%d), want (0,0)", tv.CursorLine().Get(), tv.CursorCol().Get())
	}
	// A zero-value TextView (no Lines): a click focuses but places no caret, and
	// a drag is a safe no-op.
	empty := &TextView{}
	empty.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if !empty.Focused().Get() {
		t.Fatal("empty click should still focus")
	}
	empty.OnEvent(Event{Kind: EventMouseDrag, X: 5, Y: 5}) // must not panic
}

// --- 6. HeaderBar: child event dispatch -----------------------------------

// recWidget records the OnEvent calls it receives so a container test can
// assert dispatch + coordinate translation.
type recWidget struct {
	Base
	events []Event
}

func (r *recWidget) OnEvent(ev Event) { r.events = append(r.events, ev) }

func TestHeaderBarForwardsEventsToChildren(t *testing.T) {
	h := NewHeaderBar("Title")
	a := &recWidget{}
	a.SetBounds(Rect{W: 30})
	e := &recWidget{}
	e.SetBounds(Rect{W: 24})
	h.Start = []Widget{a}
	h.End = []Widget{e}
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: HeaderBarHeight})

	// SetBounds positioned the children (bounds correct before first paint).
	if a.Bounds().X != HeaderBarPad {
		t.Fatalf("Start child not positioned by SetBounds: X=%d", a.Bounds().X)
	}
	// Click on the Start child (X in [8,38)); inner Y band is [4,36).
	h.OnEvent(Event{Kind: EventClick, X: 10, Y: 20})
	if len(a.events) != 1 {
		t.Fatalf("Start child got %d clicks, want 1", len(a.events))
	}
	// Click on the End child (X = 200-8-24 = 168).
	h.OnEvent(Event{Kind: EventClick, X: 170, Y: 20})
	if len(e.events) != 1 {
		t.Fatalf("End child got %d clicks, want 1", len(e.events))
	}
	// Click in the empty title strip hits no child.
	h.OnEvent(Event{Kind: EventClick, X: 100, Y: 20})
	if len(a.events) != 1 || len(e.events) != 1 {
		t.Fatalf("title-strip click leaked to a child: a=%d e=%d", len(a.events), len(e.events))
	}
	// EventMouseMove is forwarded to every child.
	h.OnEvent(Event{Kind: EventMouseMove, X: 100, Y: 20})
	if len(a.events) != 2 || len(e.events) != 2 {
		t.Fatalf("mouse-move not broadcast: a=%d e=%d", len(a.events), len(e.events))
	}
}

func TestHeaderBarOnEventReLaysOutLateChild(t *testing.T) {
	// A child added AFTER SetBounds still gets positioned by OnEvent's layout(),
	// so the click lands on it.
	h := NewHeaderBar("")
	h.SetBounds(Rect{X: 0, Y: 0, W: 200, H: HeaderBarHeight})
	a := &recWidget{}
	a.SetBounds(Rect{W: 40})
	h.Start = []Widget{a}
	h.OnEvent(Event{Kind: EventClick, X: 10, Y: 20})
	if len(a.events) != 1 {
		t.Fatalf("late-added child got %d clicks, want 1", len(a.events))
	}
}

// --- 7. ColorChooser: drag scrub ------------------------------------------

func TestColorChooserDragScrubsChannel(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 2*ColorChooserChannelPadY + 3*ColorChooserChannelH})
	var last RGBA
	cc.Color().Subscribe(func(c RGBA) { last = c })

	trackX := ColorChooserPadX + 12
	channelW := 200 - 2*ColorChooserPadX
	trackW := channelW - 12
	rY := ColorChooserChannelPadY // row 0 (R) top

	// Press on channel R, far right → R == 255, channel grabbed (active == 1).
	cc.OnEvent(Event{Kind: EventClick, X: trackX + trackW, Y: rY + 2})
	if cc.Color().Get().R != 255 || cc.active != 1 {
		t.Fatalf("press: R=%d active=%d, want 255/1", cc.Color().Get().R, cc.active)
	}
	// Drag left of the track (y now irrelevant) → R == 0, still channel R.
	cc.OnEvent(Event{Kind: EventMouseDrag, X: trackX - 5, Y: 999})
	if cc.Color().Get().R != 0 {
		t.Fatalf("drag-left R=%d, want 0", cc.Color().Get().R)
	}
	// Drag to the track middle → R ~ 127.
	cc.OnEvent(Event{Kind: EventMouseDrag, X: trackX + trackW/2, Y: 999})
	if r := cc.Color().Get().R; r < 120 || r > 135 {
		t.Fatalf("drag-mid R=%d, want ~127", r)
	}
	if last != cc.Color().Get() {
		t.Fatal("Subscribe did not report the latest colour")
	}
	// Release, then a stray drag is ignored.
	cc.OnEvent(Event{Kind: EventMouseUp})
	if cc.active != 0 {
		t.Fatalf("mouse-up did not release: active=%d", cc.active)
	}
	before := cc.Color().Get()
	cc.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 5, Y: rY + 2})
	if cc.Color().Get() != before {
		t.Fatal("drag after release changed the colour")
	}
}

func TestColorChooserClickMissesGrabNothing(t *testing.T) {
	cc := NewColorChooser(RGBA{A: 0xFF})
	h := 2*ColorChooserChannelPadY + 3*ColorChooserChannelH
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: h})
	// y above the widget, below it, and in the gap past the last channel row all
	// grab nothing (channelAt returns -1 for each).
	for _, y := range []int{-1, h + 5, ColorChooserChannelPadY + 3*ColorChooserChannelH} {
		cc.active = 7 // sentinel
		cc.OnEvent(Event{Kind: EventClick, X: 30, Y: y})
		if cc.active != 0 {
			t.Fatalf("click at y=%d grabbed a channel: active=%d", y, cc.active)
		}
	}
}

// --- 8. Scrollbar: grabbable thumb + track paging -------------------------

func TestScrollbarThumbDragAndTrackPage(t *testing.T) {
	sb := NewScrollbar()
	sb.Total, sb.Viewport = 200, 100 // thumb half the 100px track (H=50)
	sb.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	var got []int
	sb.OnScroll = func(o int) { got = append(got, o) }

	// Grab the thumb (top half) and drag to the bottom → Offset clamps to 100.
	sb.OnEvent(Event{Kind: EventClick, X: 4, Y: 10})
	if !sb.drag.active {
		t.Fatal("press on thumb did not start a drag")
	}
	sb.OnEvent(Event{Kind: EventMouseDrag, X: 4, Y: 60})
	if sb.Offset != 100 {
		t.Fatalf("thumb drag Offset = %d, want 100", sb.Offset)
	}
	// A further drag past the end stays pinned (off == Offset → no extra callback).
	n := len(got)
	sb.OnEvent(Event{Kind: EventMouseDrag, X: 4, Y: 200})
	if sb.Offset != 100 || len(got) != n {
		t.Fatalf("over-drag changed state: Offset=%d callbacks=%d", sb.Offset, len(got))
	}
	sb.OnEvent(Event{Kind: EventMouseUp})
	if sb.drag.active {
		t.Fatal("mouse-up did not release the drag")
	}

	// Track paging: from the top, a click below the thumb pages down one viewport.
	sb.Offset = 0
	sb.OnEvent(Event{Kind: EventClick, X: 4, Y: 70})
	if sb.Offset != 100 { // 0 + Viewport(100), clamped to max
		t.Fatalf("page-down Offset = %d, want 100", sb.Offset)
	}
	// From the bottom, a click above the thumb pages up.
	sb.OnEvent(Event{Kind: EventClick, X: 4, Y: 10})
	if sb.Offset != 0 {
		t.Fatalf("page-up Offset = %d, want 0", sb.Offset)
	}
}

func TestScrollbarDisabledZeroBoundsAndSilent(t *testing.T) {
	// Disabled: input ignored.
	d := NewScrollbar()
	d.Total, d.Viewport = 200, 100
	d.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	d.Disabled = true
	d.OnEvent(Event{Kind: EventClick, X: 4, Y: 10})
	if d.drag.active {
		t.Fatal("disabled scrollbar started a drag")
	}
	// Zero bounds: geom is not live, so a press is a safe no-op.
	z := NewScrollbar()
	z.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	z.OnEvent(Event{Kind: EventMouseDrag, X: 0, Y: 0})
	z.OnEvent(Event{Kind: EventMouseUp})
	// No OnScroll wired: it still moves its own Offset but reports nothing.
	s := NewScrollbar()
	s.Total, s.Viewport = 200, 100
	s.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	s.OnEvent(Event{Kind: EventClick, X: 4, Y: 70}) // page down
	if s.Offset != 100 {
		t.Fatalf("silent scrollbar Offset = %d, want 100", s.Offset)
	}
}

// --- 9. Frame: 1px collapse-band boundary ---------------------------------

func TestFrameCollapseBandBoundary(t *testing.T) {
	f := NewFrame(nil)
	f.Collapsible = true
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	hh := FrameTitleH // band is widget-local Y in [1, 1+hh)

	// Y == 0 is the top border, above the band → no toggle.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 0})
	if f.Collapsed().Get() {
		t.Fatal("click on the top border (Y=0) toggled")
	}
	// Y == 1 is the band's first row → toggle.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 1})
	if !f.Collapsed().Get() {
		t.Fatal("click at band start (Y=1) did not toggle")
	}
	f.Collapsed().Set(false)
	// Y == hh is the band's last row (hh < 1+hh) → toggle.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: hh})
	if !f.Collapsed().Get() {
		t.Fatalf("click at band end (Y=%d) did not toggle", hh)
	}
	f.Collapsed().Set(false)
	// Y == 1+hh is the first content row, just past the band → no toggle.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 1 + hh})
	if f.Collapsed().Get() {
		t.Fatalf("click below the band (Y=%d) toggled", 1+hh)
	}
}

// --- 10. RangeSlider: Low==High tie-break by drag direction ---------------

func TestRangeSliderTieBreakByDirection(t *testing.T) {
	mk := func() *RangeSlider {
		s := NewRangeSlider(0, 100, 50, 50) // Low == High: both thumbs stacked
		s.SetBounds(Rect{X: 0, Y: 0, W: 100 + scaleThumbSize, H: 20})
		return s
	}
	// The stacked thumbs' shared centre coordinate (widget-local).
	ref := mk()
	centre := ref.thumbPos(ref.Low().Get()) - 0 + scaleThumbSize/2

	// A press to the RIGHT of the stack grabs High and pulls it open rightward.
	right := mk()
	right.OnEvent(Event{Kind: EventClick, X: centre + 12, Y: 10})
	if right.active != 2 {
		t.Fatalf("right press grabbed handle %d, want 2 (High)", right.active)
	}
	if !(right.High().Get() > right.Low().Get()) {
		t.Fatalf("right press did not open High: Low=%v High=%v", right.Low().Get(), right.High().Get())
	}

	// A press to the LEFT of the stack grabs Low and pulls it open leftward.
	left := mk()
	left.OnEvent(Event{Kind: EventClick, X: centre - 12, Y: 10})
	if left.active != 1 {
		t.Fatalf("left press grabbed handle %d, want 1 (Low)", left.active)
	}
	if !(left.Low().Get() < left.High().Get()) {
		t.Fatalf("left press did not open Low: Low=%v High=%v", left.Low().Get(), left.High().Get())
	}
}

// --- 2. Menu: reachable submenus ------------------------------------------

// submenuFixture builds a parent Menu whose row 2 ("More") opens a child Menu
// (row 0 = separator sits between "File" and "More", exercising rowTop's
// separator advance). The child's single item bumps *inner when activated.
func submenuFixture() (parent, child *Menu, inner *int) {
	n := 0
	child = NewMenu([]MenuItem{{Label: "Inner", Action: func() { n++ }}})
	parent = NewMenu([]MenuItem{
		{Label: "File", Action: func() {}},
		{Separator: true},
		{Label: "More", Submenu: child},
	})
	parent.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90})
	return parent, child, &n
}

func TestMenuSubmenuOpensOnClickAndRoutes(t *testing.T) {
	p, child, inner := submenuFixture()
	closed := 0
	p.OnClose = func() { closed++ }

	// Click the "More" row (index 2): it opens the submenu (not fires an action).
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: p.rowTop(2) + 2})
	if p.openSub != 2 {
		t.Fatalf("click on submenu parent: openSub=%d, want 2", p.openSub)
	}
	if child.OnClose == nil {
		t.Fatal("openSubAt did not wire the child's OnClose to the parent's")
	}
	// A click inside the child routes into it: Inner fires + the chain closes.
	_, cb, ok := p.openSubmenu()
	if !ok {
		t.Fatal("openSubmenu reported no open child")
	}
	p.OnEvent(Event{Kind: EventClick, X: cb.X + 10, Y: cb.Y + 4})
	if *inner != 1 || closed != 1 {
		t.Fatalf("submenu item: inner=%d closed=%d, want 1/1", *inner, closed)
	}
}

func TestMenuSubmenuClickOutsideChildFallsThrough(t *testing.T) {
	p, _, _ := submenuFixture()
	fileFired := 0
	p.Items[0].Action = func() { fileFired++ }
	p.openSubAt(2) // submenu open

	// A click NOT on the child but on parent row 0 falls through to activate it.
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: p.rowTop(0) + 2})
	if fileFired != 1 {
		t.Fatalf("click on parent row while submenu open did not activate it: %d", fileFired)
	}
}

func TestMenuSubmenuHoverOpensAndCloses(t *testing.T) {
	p, child, _ := submenuFixture()

	// Hovering the submenu-parent row opens it.
	p.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: p.rowTop(2) + 2})
	if p.openSub != 2 {
		t.Fatalf("hover parent: openSub=%d, want 2", p.openSub)
	}
	// A move over the open child routes in (the child highlights its row).
	_, cb, _ := p.openSubmenu()
	p.OnEvent(Event{Kind: EventMouseMove, X: cb.X + 10, Y: cb.Y + 4})
	if child.Hover().Get() != 0 {
		t.Fatalf("move into child: child.Hover=%d, want 0", child.Hover().Get())
	}
	// Hovering a non-submenu row closes the child.
	p.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: p.rowTop(0) + 2})
	if p.openSub != -1 {
		t.Fatalf("hover non-parent: openSub=%d, want -1", p.openSub)
	}
	// Re-open, then a move off the body clears Hover (child stays as-is).
	p.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: p.rowTop(2) + 2})
	p.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: -5})
	if p.Hover().Get() != -1 {
		t.Fatalf("move off body: Hover=%d, want -1", p.Hover().Get())
	}
}

func TestMenuSubmenuKeyboard(t *testing.T) {
	p, child, inner := submenuFixture()
	closed := 0
	p.OnClose = func() { closed++ }

	// ArrowRight on the hovered submenu parent opens it + seeds the child's first row.
	p.Hover().Set(2)
	p.OnEvent(kd3b("ArrowRight"))
	if p.openSub != 2 || child.Hover().Get() != 0 {
		t.Fatalf("ArrowRight: openSub=%d childHover=%d, want 2/0", p.openSub, child.Hover().Get())
	}
	// A non-Left/Escape key routes into the child (ArrowDown keeps Hover on 0).
	p.OnEvent(kd3b("ArrowDown"))
	if child.Hover().Get() != 0 {
		t.Fatalf("routed ArrowDown moved child.Hover to %d", child.Hover().Get())
	}
	// Enter inside the open child fires the item + closes the chain.
	p.OnEvent(kd3b("Enter"))
	if *inner != 1 || closed != 1 {
		t.Fatalf("Enter in child: inner=%d closed=%d, want 1/1", *inner, closed)
	}

	// ArrowLeft closes the submenu; Escape also closes it.
	p2, _, _ := submenuFixture()
	p2.Hover().Set(2)
	p2.OnEvent(kd3b("ArrowRight"))
	p2.OnEvent(kd3b("ArrowLeft"))
	if p2.openSub != -1 {
		t.Fatalf("ArrowLeft did not close submenu: openSub=%d", p2.openSub)
	}
	p2.OnEvent(kd3b("ArrowRight"))
	p2.OnEvent(kd3b("Escape"))
	if p2.openSub != -1 {
		t.Fatalf("Escape did not close submenu: openSub=%d", p2.openSub)
	}
	// A disabled menu ignores keys even with a submenu open.
	p2.OnEvent(kd3b("ArrowRight")) // reopen (Hover still 2)
	p2.Disabled = true
	p2.OnEvent(kd3b("ArrowLeft"))
	if p2.openSub != 2 {
		t.Fatalf("disabled menu processed a key: openSub=%d", p2.openSub)
	}
}

func TestMenuEnterAndArrowRightEdgeCases(t *testing.T) {
	// Enter on a hovered submenu parent (no submenu open yet) opens it.
	p, _, _ := submenuFixture()
	p.Hover().Set(2)
	p.OnEvent(kd3b("Enter"))
	if p.openSub != 2 {
		t.Fatalf("Enter on submenu parent: openSub=%d, want 2", p.openSub)
	}
	// ArrowRight on a NON-submenu hovered row opens nothing.
	p2, _, _ := submenuFixture()
	p2.Hover().Set(0)
	p2.OnEvent(kd3b("ArrowRight"))
	if p2.openSub != -1 {
		t.Fatalf("ArrowRight on non-parent opened: openSub=%d", p2.openSub)
	}
}

func TestMenuOpenSubmenuGuardsAndNilOnClose(t *testing.T) {
	// openSub out of range and pointing at a non-submenu row both report closed.
	p, _, _ := submenuFixture()
	p.openSub = 99
	if _, _, ok := p.openSubmenu(); ok {
		t.Fatal("out-of-range openSub reported open")
	}
	p.openSub = 0 // row 0 has no submenu
	if _, _, ok := p.openSubmenu(); ok {
		t.Fatal("non-submenu openSub reported open")
	}
	// openSubAt on a menu with no OnClose leaves the child's OnClose unset.
	p2, child2, _ := submenuFixture() // OnClose nil
	p2.openSubAt(2)
	if child2.OnClose != nil {
		t.Fatal("openSubAt wired an OnClose when the parent had none")
	}
}

func TestMenuDrawsOpenSubmenuAndHoveredParent(t *testing.T) {
	p, _, _ := submenuFixture()
	p.Hover().Set(2) // submenu parent highlighted (Accent fill + inverted ink)
	p.openSubAt(2)   // child painted beside the row
	surf := makeSurface(320, 120)
	p.Draw(newP(surf, 320), DefaultLight())
	// The child paints its Border frame somewhere to the right of the parent.
	if countInk(surf, 320, 120, DefaultLight().Border) == 0 {
		t.Fatal("open submenu drew no border")
	}
}

func TestMenuPreferredSizeMeasuresEveryRowKind(t *testing.T) {
	// A rich menu exercises every branch of preferredSize: check-gutter reserve,
	// separator height, submenu chevron, shortcut width, and the widest-row bump.
	nested := NewMenu(nil)
	m := NewMenu([]MenuItem{
		{Label: "Toggle", Checkable: true},                      // forces the check gutter
		{Separator: true},                                       // separator height branch
		{Label: "Deep", Submenu: nested},                        // submenu chevron branch
		{Label: "Save", Shortcut: "Ctrl+S", Action: func() {}},  // shortcut branch
		{Label: "A very wide command label", Action: func() {}}, // rowW > w bump
	})
	w, h := m.preferredSize()
	if w <= MenuMinW {
		t.Fatalf("preferredSize width = %d, want > floor %d (the wide label)", w, MenuMinW)
	}
	// Height = 4 inset + 4 MenuRowH rows + 1 separator.
	if want := 4 + 4*MenuRowH + MenuSeparatorH; h != want {
		t.Fatalf("preferredSize height = %d, want %d", h, want)
	}
}

func TestScrollbarSetOffsetClamps(t *testing.T) {
	s := NewScrollbar()
	s.Total, s.Viewport = 200, 100 // maxOff = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	s.Offset = 50
	s.scrollBy(-999) // underflow clamps to 0
	if s.Offset != 0 {
		t.Fatalf("underflow Offset = %d, want 0", s.Offset)
	}
	s.scrollBy(999) // overflow clamps to maxOff
	if s.Offset != 100 {
		t.Fatalf("overflow Offset = %d, want 100", s.Offset)
	}
	s.scrollTo(100) // unchanged: early return, no work
	if s.Offset != 100 {
		t.Fatalf("no-op scrollTo changed Offset to %d", s.Offset)
	}

	// Everything fits (Viewport >= Total): maxOff is negative and clamps to 0, so
	// any stray Offset collapses back to the top.
	fits := NewScrollbar()
	fits.Total, fits.Viewport = 50, 100
	fits.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	fits.Offset = 5
	fits.scrollBy(1)
	if fits.Offset != 0 {
		t.Fatalf("fits-case Offset = %d, want 0", fits.Offset)
	}
}

func TestContextMenuSubmenuRoutes(t *testing.T) {
	inner := 0
	child := NewMenu([]MenuItem{{Label: "Inner", Action: func() { inner++ }}})
	menu := NewMenu([]MenuItem{{Label: "More", Submenu: child}})
	cm := NewContextMenu(menu)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	cm.Popup(10, 10)
	mb := cm.MenuBounds()

	// Open the submenu by clicking the "More" row (row 0).
	cm.OnEvent(Event{Kind: EventClick, X: mb.X + 10, Y: mb.Y + 2 + MenuRowH/2})
	if menu.openSub != 0 {
		t.Fatalf("context-menu submenu did not open: openSub=%d", menu.openSub)
	}
	// A click inside the submenu (outside the parent body) routes in rather than
	// dismissing the popup: Inner fires.
	_, cb, ok := menu.openSubmenu()
	if !ok {
		t.Fatal("no open submenu to click")
	}
	cm.OnEvent(Event{Kind: EventClick, X: cb.X + 10, Y: cb.Y + 4})
	if inner != 1 {
		t.Fatalf("submenu click via context menu did not fire: inner=%d", inner)
	}
}
