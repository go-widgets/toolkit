// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Wave 7 -- the last deferred overflow/scroll items: Accordion (forward the
// wheel to the expanded body under the pointer), Timeline (vertical event
// window), Gantt (vertical task-row window under a pinned header) and Notebook
// (vertical Left/Right tab strip). Each test proves the same three properties
// the earlier waves lock in: content past the visible window is reachable after
// scrolling (the offset changes and an item beyond the fold can be
// selected/activated), a wheel (EventScroll) moves the offset and clamps at
// both ends, and the hit-test maps correctly WITH the offset.

// --- Accordion: wheel forwarded to the body under the pointer --------------

func TestAccordionForwardsWheelToExpandedBody(t *testing.T) {
	body := &recordingWidget{}
	a := NewAccordion([]AccordionSection{{Title: "A", Body: body}})
	a.SetBounds(Rect{X: 30, Y: 20, W: 200, H: 100})
	a.Expanded = 0

	// Wheel over the expanded body forwards a translated EventScroll to it. The
	// body spans [ExpanderHeaderH, H) in widget-local space; the forwarded event
	// is translated past the header into the body's local frame (X preserved,
	// Y shifted up by the header height) with Delta carried through.
	a.OnEvent(Event{Kind: EventScroll, X: 5, Y: ExpanderHeaderH + 10, Delta: 3})
	if len(body.events) != 1 {
		t.Fatalf("wheel over body forwarded %d events, want 1", len(body.events))
	}
	got := body.events[0]
	if got.Kind != EventScroll || got.Delta != 3 {
		t.Fatalf("forwarded event = %+v, want EventScroll Delta 3", got)
	}
	if got.X != 5 || got.Y != 10 {
		t.Fatalf("forwarded coords = {%d,%d}, want {5,10}", got.X, got.Y)
	}

	// A wheel over the header row (above the body rect) matches no open body and
	// is dropped.
	a.OnEvent(Event{Kind: EventScroll, X: 5, Y: 2, Delta: 3})
	if len(body.events) != 1 {
		t.Fatalf("wheel over header forwarded to body (now %d events)", len(body.events))
	}
}

// A wheel over an expanded but nil-bodied section matches the body rect yet has
// nothing to forward to -- it must not panic (covers the sec.Body != nil guard).
func TestAccordionForwardsWheelNilBodyNoPanic(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A", Body: nil}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	a.Expanded = 0
	a.OnEvent(Event{Kind: EventScroll, X: 5, Y: ExpanderHeaderH + 5, Delta: 1})
}

// A wheel over a collapsed accordion (every body H == 0) matches no body and is
// a no-op (covers the br.H > 0 false arm and the fall-through return).
func TestAccordionForwardsWheelCollapsedNoBody(t *testing.T) {
	body := &recordingWidget{}
	a := NewAccordion([]AccordionSection{{Title: "A", Body: body}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	// Expanded stays -1: bodies[0].H == 0.
	a.OnEvent(Event{Kind: EventScroll, X: 5, Y: ExpanderHeaderH + 5, Delta: 1})
	if len(body.events) != 0 {
		t.Fatalf("collapsed accordion forwarded %d events, want 0", len(body.events))
	}
}

// --- Timeline: vertical event window ---------------------------------------

func timelineOverflowFixture() *Timeline {
	evs := make([]TimelineEvent, 10)
	for i := range evs {
		evs[i] = TimelineEvent{Title: "e" + itoa(i)}
	}
	tl := NewTimeline(evs)
	tl.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 60})
	return tl
}

func TestTimelineVerticalScrollsAndClamps(t *testing.T) {
	tl := timelineOverflowFixture()
	max := tl.maxScrollY()
	if max <= 0 {
		t.Fatalf("fixture does not overflow: maxScrollY=%d", max)
	}

	// Wheel down past the end clamps to maxScrollY; up past the start clamps to 0.
	tl.OnEvent(Event{Kind: EventScroll, Delta: 100})
	if tl.scrollY != max {
		t.Fatalf("wheel down clamp: scrollY=%d, want %d", tl.scrollY, max)
	}
	tl.OnEvent(Event{Kind: EventScroll, Delta: -100})
	if tl.scrollY != 0 {
		t.Fatalf("wheel up clamp: scrollY=%d, want 0", tl.scrollY)
	}

	// A non-scroll event is ignored (Timeline stays otherwise passive).
	tl.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if tl.scrollY != 0 {
		t.Fatalf("non-scroll event moved scrollY to %d", tl.scrollY)
	}

	// clampedScrollY floors a stale-negative and ceils a stale-over-max offset.
	tl.scrollY = -5
	if got := tl.clampedScrollY(); got != 0 {
		t.Fatalf("clampedScrollY(-5)=%d, want 0", got)
	}
	tl.scrollY = max + 999
	if got := tl.clampedScrollY(); got != max {
		t.Fatalf("clampedScrollY(over)=%d, want %d", got, max)
	}
	tl.scrollY = 0
}

func TestTimelineScrollByHorizontalNoOp(t *testing.T) {
	tl := timelineOverflowFixture()
	tl.Horizontal = true
	tl.ScrollBy(5)
	if tl.scrollY != 0 {
		t.Fatalf("horizontal ScrollBy moved scrollY to %d, want 0", tl.scrollY)
	}
}

func TestTimelineEventAtWithOffset(t *testing.T) {
	tl := timelineOverflowFixture()
	const x = 20 // inside [0, W) and right of the marker column

	// At offset 0 the first event sits at the top padding band.
	if got := tl.EventAt(x, TimelinePadY+1); got != 0 {
		t.Fatalf("EventAt top (offset 0)=%d, want 0", got)
	}
	// A point below the last event's block resolves to -1.
	if got := tl.EventAt(x, tl.contentH()+50); got != -1 {
		t.Fatalf("EventAt below-last=%d, want -1", got)
	}
	// Out-of-width x resolves to -1 on both sides.
	if got := tl.EventAt(-1, TimelinePadY+1); got != -1 {
		t.Fatalf("EventAt x<0=%d, want -1", got)
	}
	if got := tl.EventAt(tl.Bounds().W, TimelinePadY+1); got != -1 {
		t.Fatalf("EventAt x>=W=%d, want -1", got)
	}

	// Scroll to the end: the last event (past the fold at offset 0) is now
	// on-screen and EventAt maps a visible point to it -- offset folded in.
	tl.OnEvent(Event{Kind: EventScroll, Delta: 100})
	last := len(tl.Events) - 1
	h := tl.eventBlockH(tl.Events[last])
	y := TimelinePadY - tl.clampedScrollY() + last*h
	if y < 0 || y >= tl.Bounds().H {
		t.Fatalf("last event screen y=%d not within visible window [0,%d)", y, tl.Bounds().H)
	}
	if got := tl.EventAt(x, y+1); got != last {
		t.Fatalf("EventAt scrolled=%d, want %d", got, last)
	}

	// A horizontal timeline never row-hit-tests.
	tl.Horizontal = true
	if got := tl.EventAt(x, TimelinePadY+1); got != -1 {
		t.Fatalf("horizontal EventAt=%d, want -1", got)
	}
}

// --- Gantt: vertical task-row window under a pinned header -----------------

func ganttScrollFixture() *Gantt {
	tasks := make([]GanttTask, 5)
	for i := range tasks {
		tasks[i] = GanttTask{Label: "t" + itoa(i), Start: 1, End: 4}
	}
	g := NewGantt(tasks)
	g.Units = 10
	// Two visible rows -> maxScroll = 5 - 2 = 3.
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: GanttHeaderH + 2*GanttRowH})
	return g
}

func TestGanttVerticalScrollsAndClamps(t *testing.T) {
	g := ganttScrollFixture()
	if got := g.maxScroll(); got != 3 {
		t.Fatalf("maxScroll=%d, want 3", got)
	}

	// Wheel down past the end clamps to maxScroll; up past the start clamps to 0.
	g.OnEvent(Event{Kind: EventScroll, Delta: 100})
	if g.scroll != 3 {
		t.Fatalf("wheel down clamp: scroll=%d, want 3", g.scroll)
	}
	g.OnEvent(Event{Kind: EventScroll, Delta: -100})
	if g.scroll != 0 {
		t.Fatalf("wheel up clamp: scroll=%d, want 0", g.scroll)
	}

	// clampedScroll floors a stale-negative and ceils a stale-over-max value.
	g.scroll = -5
	if got := g.clampedScroll(); got != 0 {
		t.Fatalf("clampedScroll(-5)=%d, want 0", got)
	}
	g.scroll = 99
	if got := g.clampedScroll(); got != 3 {
		t.Fatalf("clampedScroll(99)=%d, want 3", got)
	}
	g.scroll = 0

	// A short chart never scrolls: maxScroll floors at 0.
	short := NewGantt([]GanttTask{{Label: "only", Start: 0, End: 1}})
	short.SetBounds(Rect{X: 0, Y: 0, W: 400, H: GanttHeaderH + 3*GanttRowH})
	if got := short.maxScroll(); got != 0 {
		t.Fatalf("short-chart maxScroll=%d, want 0", got)
	}
}

func TestGanttTaskAtWithOffset(t *testing.T) {
	g := ganttScrollFixture()
	// At offset 0 the first visible slot is task 0.
	if got := g.TaskAt(GanttLabelW+2, GanttHeaderH+2); got != 0 {
		t.Fatalf("TaskAt slot0 (offset 0)=%d, want 0", got)
	}
	// Scroll by 3: the two visible slots now map to tasks 3 and 4 (past the
	// 2-row fold).
	g.ScrollBy(3)
	if got := g.TaskAt(GanttLabelW+2, GanttHeaderH+2); got != 3 {
		t.Fatalf("TaskAt slot0 (offset 3)=%d, want 3", got)
	}
	if got := g.TaskAt(GanttLabelW+2, GanttHeaderH+GanttRowH+2); got != 4 {
		t.Fatalf("TaskAt slot1 (offset 3)=%d, want 4", got)
	}
	// The header band and past-the-last-row space still resolve to -1.
	if got := g.TaskAt(GanttLabelW+2, GanttHeaderH-1); got != -1 {
		t.Fatalf("TaskAt header=%d, want -1", got)
	}
}

// A bar in a scrolled-in row still drags: the row is resolved through the
// offset, the horizontal bar math is offset-independent, and OnTaskChange fires
// for the task actually shown.
func TestGanttDragScrolledRow(t *testing.T) {
	g := ganttScrollFixture()
	changed := [3]int{-9, -9, -9}
	g.OnTaskChange = func(i, s, e int) { changed = [3]int{i, s, e} }
	g.ScrollBy(3) // rows 3,4 visible; slot 1 -> task 4

	rowY := GanttHeaderH + GanttRowH + 2 // second visible slot -> task 4
	midX := (g.barXLocal(1) + g.barXLocal(4)) / 2
	g.OnEvent(Event{Kind: EventClick, X: midX, Y: rowY})
	if g.Selected != 4 {
		t.Fatalf("scrolled click selected %d, want 4", g.Selected)
	}
	if !g.editing || g.editMode != ganttMove {
		t.Fatalf("grab: editing=%v mode=%d, want move", g.editing, g.editMode)
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: g.barXLocal(6), Y: rowY})
	g.OnEvent(Event{Kind: EventMouseUp, X: g.barXLocal(6), Y: rowY})
	if changed[0] != 4 {
		t.Fatalf("OnTaskChange index=%d, want 4", changed[0])
	}
	if g.Tasks[4].Start <= 1 {
		t.Fatalf("task 4 bar did not move: Start=%d", g.Tasks[4].Start)
	}
}

// --- Notebook: vertical Left/Right tab strip -------------------------------

func notebookVerticalFixture() *Notebook {
	n := NewNotebook()
	n.TabSide = TabLeft
	n.Tabs = make([]NotebookTab, 8)
	for i := range n.Tabs {
		n.Tabs[i] = NotebookTab{Label: "T" + itoa(i)}
	}
	// H = 72 -> visibleTabs = 3 -> maxTabScroll = 8 - 3 = 5.
	n.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 3 * NotebookTabStripH})
	return n
}

func TestNotebookVerticalStripScrollsAndClamps(t *testing.T) {
	n := notebookVerticalFixture()
	if got := n.maxTabScroll(); got != 5 {
		t.Fatalf("maxTabScroll=%d, want 5", got)
	}

	// Wheel over the strip scrolls; past either end it clamps.
	n.OnEvent(Event{Kind: EventScroll, X: 2, Y: 2, Delta: 100})
	if n.tabScroll != 5 {
		t.Fatalf("wheel down clamp: tabScroll=%d, want 5", n.tabScroll)
	}
	n.OnEvent(Event{Kind: EventScroll, X: 2, Y: 2, Delta: -100})
	if n.tabScroll != 0 {
		t.Fatalf("wheel up clamp: tabScroll=%d, want 0", n.tabScroll)
	}

	// A wheel over the body (right of the NotebookTabWidth strip) does not
	// scroll the strip.
	n.OnEvent(Event{Kind: EventScroll, X: NotebookTabWidth + 20, Y: 2, Delta: 3})
	if n.tabScroll != 0 {
		t.Fatalf("wheel over body scrolled strip to %d, want 0", n.tabScroll)
	}

	// clampedTabScroll floors a stale-negative and ceils a stale-over-max value.
	n.tabScroll = -5
	if got := n.clampedTabScroll(); got != 0 {
		t.Fatalf("clampedTabScroll(-5)=%d, want 0", got)
	}
	n.tabScroll = 99
	if got := n.clampedTabScroll(); got != 5 {
		t.Fatalf("clampedTabScroll(99)=%d, want 5", got)
	}
	n.tabScroll = 0
}

// A horizontal (Top) strip never scrolls: maxTabScroll/clampedTabScroll are 0,
// ScrollTabsBy is a no-op, and a wheel is not consumed by the strip branch.
func TestNotebookHorizontalStripNeverScrolls(t *testing.T) {
	n := NewNotebook()
	n.Tabs = make([]NotebookTab, 8)
	for i := range n.Tabs {
		n.Tabs[i] = NotebookTab{Label: "T" + itoa(i)}
	}
	n.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 120})
	if got := n.maxTabScroll(); got != 0 {
		t.Fatalf("horizontal maxTabScroll=%d, want 0", got)
	}
	if got := n.clampedTabScroll(); got != 0 {
		t.Fatalf("horizontal clampedTabScroll=%d, want 0", got)
	}
	n.ScrollTabsBy(3)
	if n.tabScroll != 0 {
		t.Fatalf("horizontal ScrollTabsBy moved tabScroll to %d, want 0", n.tabScroll)
	}
	// A wheel over the (top) strip is not consumed by the vertical-strip branch.
	n.OnEvent(Event{Kind: EventScroll, X: 2, Y: 2, Delta: 3})
	if n.tabScroll != 0 {
		t.Fatalf("horizontal wheel moved tabScroll to %d, want 0", n.tabScroll)
	}
}

// A vertical strip whose window can hold every tab never scrolls (maxTabScroll
// floors at 0).
func TestNotebookVerticalShortStripNoScroll(t *testing.T) {
	n := NewNotebook()
	n.TabSide = TabRight
	n.Tabs = []NotebookTab{{Label: "A"}, {Label: "B"}}
	n.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 8 * NotebookTabStripH})
	if got := n.maxTabScroll(); got != 0 {
		t.Fatalf("short vertical strip maxTabScroll=%d, want 0", got)
	}
	n.ScrollTabsBy(5)
	if n.tabScroll != 0 {
		t.Fatalf("short vertical strip scrolled to %d, want 0", n.tabScroll)
	}
}

// A degenerate (non-positive height) vertical strip shows no tabs and never
// scrolls the active tab into view (covers visibleTabs h<0 and vis<=0 guards).
func TestNotebookVerticalZeroWindow(t *testing.T) {
	n := NewNotebook()
	n.TabSide = TabLeft
	n.Tabs = make([]NotebookTab, 4)
	n.SetBounds(Rect{X: 0, Y: 0, W: 300, H: -10})
	if got := n.visibleTabs(); got != 0 {
		t.Fatalf("negative-height visibleTabs=%d, want 0", got)
	}
	n.setActive(3) // scrollActiveIntoView must bail on vis<=0
	if n.tabScroll != 0 {
		t.Fatalf("zero-window setActive moved tabScroll to %d, want 0", n.tabScroll)
	}
}

// Keyboard tab switching keeps the active tab visible: arrowing onto an
// off-screen tab scrolls the strip both down (reveal-below) and back up
// (reveal-above).
func TestNotebookVerticalKeyboardScrollsActiveIntoView(t *testing.T) {
	n := notebookVerticalFixture()

	// From tab 0, ArrowUp wraps to the last tab (7): reveal-below pulls
	// tabScroll to 7 - 3 + 1 = 5.
	n.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if n.Active().Get() != 7 || n.tabScroll != 5 {
		t.Fatalf("wrap up: Active=%d tabScroll=%d, want 7/5", n.Active().Get(), n.tabScroll)
	}
	// ArrowDown wraps back to tab 0: reveal-above pulls tabScroll to 0.
	n.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if n.Active().Get() != 0 || n.tabScroll != 0 {
		t.Fatalf("wrap down: Active=%d tabScroll=%d, want 0/0", n.Active().Get(), n.tabScroll)
	}

	// A setActive to a tab below the window scrolls it into view from below.
	n.setActive(4)
	if n.tabScroll != 2 {
		t.Fatalf("reveal-below setActive(4): tabScroll=%d, want 2", n.tabScroll)
	}
	// A setActive to a tab above the window scrolls it into view from above.
	n.setActive(1)
	if n.tabScroll != 1 {
		t.Fatalf("reveal-above setActive(1): tabScroll=%d, want 1", n.tabScroll)
	}
}

// A click hit-tests through the scroll offset: after scrolling, clicking the top
// strip slot activates the scrolled-in tab, not the tab that used to sit there.
func TestNotebookVerticalClickWithOffset(t *testing.T) {
	n := notebookVerticalFixture()
	activated := -1
	n.Active().Subscribe(func(i int) { activated = i })

	n.ScrollTabsBy(5) // tab 5 now sits in the top slot (i-scroll == 0)
	n.OnEvent(Event{Kind: EventClick, X: 2, Y: 2})
	if n.Active().Get() != 5 || activated != 5 {
		t.Fatalf("scrolled click: Active=%d activated=%d, want 5/5", n.Active().Get(), activated)
	}
}
