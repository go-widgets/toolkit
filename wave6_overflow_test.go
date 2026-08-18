// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Wave 6 — overflow / scroll for content that exceeds its bounds. Each test
// proves the same three properties for the widget it covers: content past the
// visible window is reachable after scrolling (the offset changes and an item
// beyond the fold can be selected / a hovered row scrolls into view), a wheel
// (EventScroll) moves the offset and clamps at both ends, and the click
// hit-test maps correctly WITH the offset (a click after scrolling selects the
// right item).

func wave6Options(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "o" + itoa(i)
	}
	return out
}

// --- DropDown popover ------------------------------------------------------

func TestDropDownPopoverScrolls(t *testing.T) {
	opts := wave6Options(15) // 15 > PopoverMaxRows (12): maxPopScroll = 3
	chosen := -1
	d := NewDropDown(opts, 0)
	d.Selected().Subscribe(func(i int) { chosen = i })
	d.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 22})

	// A click opens the popover; Selected 0 keeps the offset at the top.
	d.OnEvent(Event{Kind: EventClick})
	if !d.Open().Get() || d.popScroll != 0 {
		t.Fatalf("open: Open=%v popScroll=%d, want true/0", d.Open().Get(), d.popScroll)
	}
	// Wheel down past the end clamps to maxPopScroll (3).
	d.OnEvent(Event{Kind: EventScroll, Delta: 9})
	if d.popScroll != 3 {
		t.Fatalf("wheel down clamp: popScroll=%d, want 3", d.popScroll)
	}
	// Wheel up past the start clamps to 0.
	d.OnEvent(Event{Kind: EventScroll, Delta: -20})
	if d.popScroll != 0 {
		t.Fatalf("wheel up clamp: popScroll=%d, want 0", d.popScroll)
	}
	// Scroll so option 14 is in the window, then click the last visible row:
	// window row 11 maps to option popScroll(3)+11 = 14 — past the 12-row fold.
	d.OnEvent(Event{Kind: EventScroll, Delta: 3})
	pb := d.PopoverBounds()
	if !d.PopoverClick(pb.X+2, pb.Y+11*PopoverRowH+2) {
		t.Fatal("PopoverClick inside scrolled popover returned false")
	}
	if chosen != 14 || d.Selected().Get() != 14 {
		t.Fatalf("scrolled click: chosen=%d Selected=%d, want 14", chosen, d.Selected().Get())
	}

	// Reopening with Selected == 14 scrolls the window to reveal it from below.
	d.OnEvent(Event{Kind: EventClick})
	if d.popScroll != 3 {
		t.Fatalf("reopen reveal-below: popScroll=%d, want 3", d.popScroll)
	}
	// Arrow the selection up to the top: the highlight-follow reveals it from
	// above, pulling popScroll back to 0.
	for i := 0; i < 14; i++ {
		d.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if d.Selected().Get() != 0 || d.popScroll != 0 {
		t.Fatalf("arrow reveal-above: Selected=%d popScroll=%d, want 0/0", d.Selected().Get(), d.popScroll)
	}
	// ArrowDown to the last option follows the highlight down to the fold.
	for i := 0; i < 14; i++ {
		d.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if d.Selected().Get() != 14 || d.popScroll != 3 {
		t.Fatalf("arrow reveal-below: Selected=%d popScroll=%d, want 14/3", d.Selected().Get(), d.popScroll)
	}

	// A closed popover ignores the wheel.
	d.Open().Set(false)
	d.popScroll = 2
	d.OnEvent(Event{Kind: EventScroll, Delta: -5})
	if d.popScroll != 2 {
		t.Fatalf("closed wheel changed popScroll to %d, want 2 (ignored)", d.popScroll)
	}

	// A short list never scrolls: maxPopScroll floors at 0.
	small := NewDropDown(wave6Options(3), 0)
	small.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 22})
	small.Open().Set(true)
	small.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if small.popScroll != 0 {
		t.Fatalf("short-list popScroll=%d, want 0", small.popScroll)
	}
	// DrawPopover with a scroll offset paints without panicking.
	d.Open().Set(true)
	d.popScroll = 3
	d.DrawPopover(newP(makeSurface(160, 240), 160), DefaultLight())
}

// --- ComboBox popover ------------------------------------------------------

func TestComboBoxPopoverScrolls(t *testing.T) {
	opts := wave6Options(15) // all contain "o": empty query matches every one
	c := NewComboBox(opts)
	var selected string
	c.Text().Subscribe(func(s string) { selected = s })
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	c.Open().Set(true)

	// Height stays clamped to PopoverMaxRows even with 15 matches.
	if got := c.PopoverBounds().H; got != PopoverMaxRows*comboRowH {
		t.Fatalf("clamped popover height = %d, want %d", got, PopoverMaxRows*comboRowH)
	}
	// Wheel down past the end clamps to maxPopScroll (3); up clamps to 0.
	c.OnEvent(Event{Kind: EventScroll, Delta: 9})
	if c.popScroll != 3 {
		t.Fatalf("wheel down clamp: popScroll=%d, want 3", c.popScroll)
	}
	c.OnEvent(Event{Kind: EventScroll, Delta: -20})
	if c.popScroll != 0 {
		t.Fatalf("wheel up clamp: popScroll=%d, want 0", c.popScroll)
	}
	// ArrowDown to the last match follows the highlight to the fold.
	for i := 0; i < 14; i++ {
		c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if c.highlightRow() != 14 || c.popScroll != 3 {
		t.Fatalf("arrow reveal-below: highlight=%d popScroll=%d, want 14/3", c.highlightRow(), c.popScroll)
	}
	// A click on the last visible row (window row 11) selects the absolute
	// option popScroll(3)+11 = "o14" — past the fold, with the offset applied.
	pb := c.PopoverBounds()
	ly := 11*comboRowH + comboRowH/2
	c.OnEvent(Event{Kind: EventClick, X: 10, Y: (pb.Y - 0) + ly})
	if selected != "o14" || c.Text().Get() != "o14" {
		t.Fatalf("scrolled click: selected=%q Text=%q, want o14", selected, c.Text().Get())
	}

	// ArrowUp back to the top reveals the highlight from above.
	c.Open().Set(true)
	c.highlight, c.popScroll = 14, 3
	for i := 0; i < 14; i++ {
		c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if c.highlightRow() != 0 || c.popScroll != 0 {
		t.Fatalf("arrow reveal-above: highlight=%d popScroll=%d, want 0/0", c.highlightRow(), c.popScroll)
	}
	// Typing resets the scroll offset with the highlight.
	c.popScroll = 3
	c.OnEvent(Event{Kind: EventChar, Code: "o"})
	if c.popScroll != 0 {
		t.Fatalf("typing did not reset popScroll (got %d)", c.popScroll)
	}
	// Backspace also resets it.
	c.popScroll = 3
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c.popScroll != 0 {
		t.Fatalf("backspace did not reset popScroll (got %d)", c.popScroll)
	}
	// A closed popover ignores the wheel.
	c.Open().Set(false)
	c.popScroll = 1
	c.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if c.popScroll != 1 {
		t.Fatalf("closed wheel changed popScroll to %d, want 1", c.popScroll)
	}
	// A short list never scrolls.
	short := NewComboBox(wave6Options(3))
	short.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	short.Open().Set(true)
	short.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if short.popScroll != 0 {
		t.Fatalf("short-list popScroll=%d, want 0", short.popScroll)
	}
	// Draw a scrolled popover (exercises the offset highlight branch).
	c.Open().Set(true)
	c.popScroll, c.highlight = 3, 14
	c.Draw(newP(makeSurface(200, 320), 200), DefaultLight())
}

// --- CommandPalette --------------------------------------------------------

func wave6Palette(n int) *CommandPalette {
	cmds := make([]PaletteCommand, n)
	for i := range cmds {
		cmds[i] = PaletteCommand{Label: "cmd" + itoa(i)}
	}
	return NewCommandPalette(cmds)
}

func TestCommandPaletteScrolls(t *testing.T) {
	ran := -1
	cp := wave6Palette(15)
	for i := range cp.Commands {
		i := i
		cp.Commands[i].Action = func() { ran = i }
	}
	cp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	cp.Open()

	// Broad (empty) query: the panel caps its result rows at PaletteMaxRows
	// instead of growing past the surface.
	wantH := (1+PaletteMaxRows)*PaletteRowH + 2*palettePadY
	if got := cp.panelBounds().H; got != wantH {
		t.Fatalf("panel height = %d, want %d (clamped)", got, wantH)
	}
	// Wheel down past the end clamps to maxScroll (3); up clamps to 0.
	cp.OnEvent(Event{Kind: EventScroll, Delta: 9})
	if cp.scroll != 3 {
		t.Fatalf("wheel down clamp: scroll=%d, want 3", cp.scroll)
	}
	cp.OnEvent(Event{Kind: EventScroll, Delta: -20})
	if cp.scroll != 0 {
		t.Fatalf("wheel up clamp: scroll=%d, want 0", cp.scroll)
	}
	// ArrowDown to the last result follows the selection to the fold.
	for i := 0; i < 14; i++ {
		cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if cp.Selected() != 14 || cp.scroll != 3 {
		t.Fatalf("arrow reveal-below: selected=%d scroll=%d, want 14/3", cp.Selected(), cp.scroll)
	}
	// A click on the last visible result row (window row 11) runs command
	// scroll(3)+11 = 14 — past the fold.
	pb := cp.panelBounds()
	ry := pb.Y + palettePadY + (11+1)*PaletteRowH + PaletteRowH/2
	cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: ry})
	if ran != 14 {
		t.Fatalf("scrolled click ran command %d, want 14", ran)
	}

	// Reopen; ArrowUp from a scrolled position reveals the selection above.
	cp.Open()
	cp.selected, cp.scroll = 14, 3
	for i := 0; i < 14; i++ {
		cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if cp.Selected() != 0 || cp.scroll != 0 {
		t.Fatalf("arrow reveal-above: selected=%d scroll=%d, want 0/0", cp.Selected(), cp.scroll)
	}
	// A query that filters to a short list floors maxScroll at 0.
	cp.SetQuery("cmd1") // matches cmd1 + cmd10..cmd14 (< PaletteMaxRows)
	cp.scroll = 5
	cp.clampScroll()
	if cp.scroll != 0 {
		t.Fatalf("short filtered scroll=%d, want 0", cp.scroll)
	}
	// A click below the last result row is ignored (no panic, nothing runs).
	ran = -1
	cp.SetQuery("")
	cp.scroll = 3
	pb = cp.panelBounds()
	tooFar := pb.Y + palettePadY + (PaletteMaxRows+1)*PaletteRowH + PaletteRowH/2
	if tooFar < pb.Y+pb.H {
		cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: tooFar})
	}
	// Draw a scrolled panel.
	cp.Draw(newP(makeSurface(400, 400), 400), DefaultLight())
}

// --- TextView --------------------------------------------------------------

func wave6TextView(lines int) *TextView {
	t := &TextView{Lines: make([]string, lines)}
	for i := range t.Lines {
		t.Lines[i] = "line" + itoa(i)
	}
	return t
}

func TestTextViewScrolls(t *testing.T) {
	tv := wave6TextView(30)
	tv.Focused = true
	// lineH = glyphHeight(7)+4 = 11; H=40 → visibleLines = (40-4)/11 = 3.
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	if got := tv.visibleLines(); got != 3 {
		t.Fatalf("visibleLines = %d, want 3", got)
	}
	// Wheel down past the end clamps to maxScrollLine = 30-3 = 27.
	tv.OnEvent(Event{Kind: EventScroll, Delta: 40})
	if tv.ScrollLine != 27 {
		t.Fatalf("wheel down clamp: ScrollLine=%d, want 27", tv.ScrollLine)
	}
	// Wheel up past the start clamps to 0.
	tv.OnEvent(Event{Kind: EventScroll, Delta: -40})
	if tv.ScrollLine != 0 {
		t.Fatalf("wheel up clamp: ScrollLine=%d, want 0", tv.ScrollLine)
	}
	// ArrowDown past the visible window scrolls to keep the caret visible.
	for i := 0; i < 20; i++ {
		tv.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if tv.CursorLine != 20 {
		t.Fatalf("cursor line = %d, want 20", tv.CursorLine)
	}
	if tv.CursorLine < tv.ScrollLine || tv.CursorLine >= tv.ScrollLine+tv.visibleLines() {
		t.Fatalf("caret line %d out of window [%d,%d)", tv.CursorLine, tv.ScrollLine, tv.ScrollLine+tv.visibleLines())
	}
	// A click after scrolling maps to the absolute line through the offset:
	// viewport row 1 at ScrollLine s selects line s+1.
	s := tv.ScrollLine
	lineH := tv.glyphHeight() + 4
	tv.OnEvent(Event{Kind: EventClick, X: 4, Y: 4 + 1*lineH + 1})
	if tv.CursorLine != s+1 {
		t.Fatalf("scrolled click: CursorLine=%d, want %d", tv.CursorLine, s+1)
	}
	// ArrowUp back to the top scrolls the viewport up with the caret.
	for i := 0; i < 30; i++ {
		tv.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if tv.CursorLine != 0 || tv.ScrollLine != 0 {
		t.Fatalf("back to top: CursorLine=%d ScrollLine=%d, want 0/0", tv.CursorLine, tv.ScrollLine)
	}
	// Typing enough newlines pushes the caret down and follows it.
	tv.CursorLine, tv.CursorCol, tv.ScrollLine = 0, 0, 0
	for i := 0; i < 10; i++ {
		tv.OnEvent(Event{Kind: EventChar, Code: "\n"})
	}
	if tv.ScrollLine == 0 {
		t.Fatalf("typing newlines did not scroll (ScrollLine=%d)", tv.ScrollLine)
	}
	// Draw a scrolled buffer (exercises the window + break branch).
	tv.ScrollLine = 20
	tv.Draw(newP(makeSurface(200, 40), 200), DefaultLight())

	// A viewport too short for even one line collapses visibleLines to 0 and
	// makes the caret-follow a no-op.
	tiny := wave6TextView(5)
	tiny.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 3}) // H-4 < 0 → 0
	if tiny.visibleLines() != 0 {
		t.Fatalf("tiny visibleLines = %d, want 0", tiny.visibleLines())
	}
	tiny.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // scrollCaretIntoView no-op
	// A one-line-floor viewport still shows the caret's line.
	floor := wave6TextView(5)
	floor.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10}) // (10-4)/11 = 0 → floored to 1
	if floor.visibleLines() != 1 {
		t.Fatalf("floor visibleLines = %d, want 1", floor.visibleLines())
	}
}

// --- AgendaSidebar ---------------------------------------------------------

func wave6Sidebar(n int) *AgendaSidebar {
	cals := make([]AgendaCalendar, n)
	for i := range cals {
		cals[i] = AgendaCalendar{Name: "cal" + itoa(i)}
	}
	s := NewAgendaSidebar(cals)
	s.Title = "" // no header: rows start at the top
	return s
}

func TestAgendaSidebarScrolls(t *testing.T) {
	toggled := -1
	s := wave6Sidebar(20)
	s.OnToggle = func(i int) { toggled = i }
	// No header, H = 3*24 = 72 → visibleRows = 3; maxScroll = 20-3 = 17.
	s.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 3 * AgendaSidebarRowH})
	if got := s.visibleRows(); got != 3 {
		t.Fatalf("visibleRows = %d, want 3", got)
	}
	// Wheel down past the end clamps to 17; up clamps to 0.
	s.OnEvent(Event{Kind: EventScroll, Delta: 40})
	if s.scroll != 17 {
		t.Fatalf("wheel down clamp: scroll=%d, want 17", s.scroll)
	}
	s.OnEvent(Event{Kind: EventScroll, Delta: -40})
	if s.scroll != 0 {
		t.Fatalf("wheel up clamp: scroll=%d, want 0", s.scroll)
	}
	// A click after scrolling toggles the calendar the pointer visually sits on:
	// viewport row 0 at scroll s is calendar s.
	s.ScrollBy(17)
	if s.scroll != 17 {
		t.Fatalf("ScrollBy: scroll=%d, want 17", s.scroll)
	}
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: AgendaSidebarRowH / 2})
	if toggled != 17 || !s.Calendars[17].Hidden {
		t.Fatalf("scrolled click toggled=%d Hidden=%v, want 17/true", toggled, s.Calendars[17].Hidden)
	}
	// Draw a scrolled rail (windowed rows clipped to the body).
	s.Draw(newP(makeSurface(160, 72), 160), DefaultLight())

	// A short list never scrolls, and a zero-height rail shows no rows.
	short := wave6Sidebar(2)
	short.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 3 * AgendaSidebarRowH})
	short.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if short.scroll != 0 {
		t.Fatalf("short-list scroll=%d, want 0", short.scroll)
	}
	flat := wave6Sidebar(5)
	flat.Title = "Calendars" // header taller than bounds → body height <= 0
	flat.SetBounds(Rect{X: 0, Y: 0, W: 160, H: AgendaHeaderH})
	if flat.visibleRows() != 0 {
		t.Fatalf("flat visibleRows = %d, want 0", flat.visibleRows())
	}
}

// --- Menu ------------------------------------------------------------------

func wave6Menu(n int) *Menu {
	items := make([]MenuItem, n)
	for i := range items {
		items[i] = MenuItem{Label: "item" + itoa(i), Action: func() {}}
	}
	return NewMenu(items)
}

func TestMenuScrolls(t *testing.T) {
	fired := -1
	items := make([]MenuItem, 20)
	for i := range items {
		i := i
		items[i] = MenuItem{Label: "item" + itoa(i), Action: func() { fired = i }}
	}
	m := NewMenu(items)
	// rowsHeight = 20*22 = 440; body H = 5*22 = 110 → maxScroll = 440+4-110 = 334.
	m.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 5 * MenuRowH})
	if got := m.maxScroll(); got != 334 {
		t.Fatalf("maxScroll = %d, want 334", got)
	}
	// Wheel down moves by whole rows; a huge delta clamps to maxScroll.
	m.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if m.scroll != 2*MenuRowH {
		t.Fatalf("wheel: scroll=%d, want %d", m.scroll, 2*MenuRowH)
	}
	m.OnEvent(Event{Kind: EventScroll, Delta: 100})
	if m.scroll != 334 {
		t.Fatalf("wheel down clamp: scroll=%d, want 334", m.scroll)
	}
	m.OnEvent(Event{Kind: EventScroll, Delta: -100})
	if m.scroll != 0 {
		t.Fatalf("wheel up clamp: scroll=%d, want 0", m.scroll)
	}
	// Keyboard navigation to the last item scrolls it into view from below.
	for i := 0; i < 20; i++ {
		m.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if m.Hover().Get() != 19 {
		t.Fatalf("hover = %d, want 19", m.Hover().Get())
	}
	top := m.rowTop(19)
	if top-m.scroll < 0 || (top+MenuRowH)-m.scroll > m.Bounds().H {
		t.Fatalf("hovered row 19 not fully visible: top=%d scroll=%d H=%d", top, m.scroll, m.Bounds().H)
	}
	// A click while scrolled activates the row the pointer visually sits on:
	// the hovered last row sits near the bottom of the viewport.
	rowLocalY := (m.rowTop(19) - m.scroll) + MenuRowH/2
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: rowLocalY})
	if fired != 19 {
		t.Fatalf("scrolled click fired %d, want 19", fired)
	}
	// Navigating back to the top reveals the first item from above (19 steps
	// from row 19 reach row 0; the reveal pulls it fully into view).
	for i := 0; i < 19; i++ {
		m.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if m.Hover().Get() != 0 || m.scroll > m.rowTop(0) {
		t.Fatalf("back to top: Hover=%d scroll=%d, want Hover 0 with row 0 revealed", m.Hover().Get(), m.scroll)
	}
	// scrollHoverIntoView is a no-op with no hovered row.
	m.Hover().Set(-1)
	m.scroll = 50
	m.scrollHoverIntoView()
	if m.scroll != 50 {
		t.Fatalf("no-hover scrollHoverIntoView changed scroll to %d, want 50", m.scroll)
	}
	// Draw a scrolled menu (clipped body).
	m.scroll = 100
	m.Draw(newP(makeSurface(140, 110), 140), DefaultLight())

	// A menu that fits never scrolls (maxScroll floors at 0).
	fits := wave6Menu(3)
	fits.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 200})
	if fits.maxScroll() != 0 {
		t.Fatalf("fitting menu maxScroll = %d, want 0", fits.maxScroll())
	}
	fits.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if fits.scroll != 0 {
		t.Fatalf("fitting menu scroll = %d, want 0", fits.scroll)
	}
}

// --- ContextMenu -----------------------------------------------------------

func TestContextMenuClampsAndScrolls(t *testing.T) {
	m := wave6Menu(20) // 20*22 + 4 = 444px tall
	cm := NewContextMenu(m)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 120}) // surface shorter than the menu
	cm.Popup(10, 10)

	mb := cm.MenuBounds()
	if mb.H != 120 {
		t.Fatalf("MenuBounds height = %d, want 120 (clamped to surface)", mb.H)
	}
	// Forwarding the wheel scrolls the wrapped Menu's rows.
	cm.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if m.scroll != 3*MenuRowH {
		t.Fatalf("forwarded wheel: menu scroll=%d, want %d", m.scroll, 3*MenuRowH)
	}
	// A menu that fits keeps its exact measured height.
	small := wave6Menu(2)
	scm := NewContextMenu(small)
	scm.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	scm.Popup(5, 5)
	if got := scm.MenuBounds().H; got != 2*MenuRowH+4 {
		t.Fatalf("small MenuBounds height = %d, want %d", got, 2*MenuRowH+4)
	}
}

// --- Kanban ----------------------------------------------------------------

func wave6Kanban(cols, cards int) *Kanban {
	cs := make([]KanbanColumn, cols)
	for i := range cs {
		cc := make([]KanbanCard, cards)
		for j := range cc {
			cc[j] = KanbanCard{Title: "c" + itoa(i) + "_" + itoa(j)}
		}
		cs[i] = KanbanColumn{Title: "col" + itoa(i), Cards: cc}
	}
	return NewKanban(cs)
}

func TestKanbanColumnScrolls(t *testing.T) {
	clicked := -1
	k := wave6Kanban(2, 12) // 12 cards/column
	k.OnCardClick = func(col, card int) { clicked = card }
	// H below header+divider: body = H - 29. Pick H so a few cards fit.
	k.SetBounds(Rect{X: 0, Y: 0, W: 300, H: KanbanHeaderH + 1 + 3*cardSlot})
	// content = gap + 12*slot; body = 3*slot → maxScroll = gap + 9*slot.
	wantMax := KanbanCardGap + 9*cardSlot
	if got := k.colMaxScroll(0); got != wantMax {
		t.Fatalf("colMaxScroll(0) = %d, want %d", got, wantMax)
	}
	// Wheel over column 0 scrolls only it; a huge delta clamps to the max.
	k.OnEvent(Event{Kind: EventScroll, X: 10, Delta: 100})
	if k.colScrollAt(0) != wantMax {
		t.Fatalf("col0 wheel clamp = %d, want %d", k.colScrollAt(0), wantMax)
	}
	if k.colScrollAt(1) != 0 {
		t.Fatalf("col1 must stay unscrolled, got %d", k.colScrollAt(1))
	}
	// Wheel up past the start clamps to 0.
	k.OnEvent(Event{Kind: EventScroll, X: 10, Delta: -100})
	if k.colScrollAt(0) != 0 {
		t.Fatalf("col0 wheel up clamp = %d, want 0", k.colScrollAt(0))
	}
	// Scroll column 0 by two card slots, then click the first visible card:
	// its local Y maps back (through the offset) to card index 2.
	k.scrollColumn(0, 2*cardSlot)
	colW := k.colWidth()
	lr := k.cardLocalRect(0, 2, colW) // card 2 is now the top visible card
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 2, Y: lr.Y + 2})
	if clicked != 2 || k.SelectedCard().Get() != 2 {
		t.Fatalf("scrolled click: clicked=%d SelectedCard=%d, want 2", clicked, k.SelectedCard().Get())
	}
	// Draw a scrolled board.
	k.Draw(newP(makeSurface(300, KanbanHeaderH+1+3*cardSlot), 300), DefaultLight())

	// A drag drop while a column is scrolled targets the visually-correct slot.
	k.scrollColumn(1, 2*cardSlot)
	toIdx := k.dropIndexAt(1, KanbanHeaderH+KanbanCardGap+1) // top of the visible body
	if toIdx != 2 {
		t.Fatalf("scrolled dropIndexAt = %d, want 2", toIdx)
	}
	// A drag overlay renders while a column is scrolled.
	k.dragging, k.moved, k.dragCol, k.dragCard = true, true, 1, 0
	k.dragX, k.dragY = colW+20, KanbanHeaderH+10
	k.Draw(newP(makeSurface(300, KanbanHeaderH+1+3*cardSlot), 300), DefaultLight())
	k.dragging, k.moved = false, false

	// Out-of-range and edge helpers: colScrollAt clamps a stale value, an
	// out-of-range column is a no-op, and an empty board ignores the wheel.
	k.colScroll[0] = 1 << 20 // stale, past the max
	if k.colScrollAt(0) != wantMax {
		t.Fatalf("stale colScrollAt clamp = %d, want %d", k.colScrollAt(0), wantMax)
	}
	k.colScroll[0] = -5
	if k.colScrollAt(0) != 0 {
		t.Fatalf("negative colScrollAt clamp = %d, want 0", k.colScrollAt(0))
	}
	if k.colScrollAt(99) != 0 {
		t.Fatal("out-of-range colScrollAt must be 0")
	}
	k.scrollColumn(99, 5) // ignored
	// A column with no cards has zero content height.
	empty := wave6Kanban(1, 0)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	if empty.colContentH(0) != 0 || empty.colMaxScroll(0) != 0 {
		t.Fatalf("empty column content=%d max=%d, want 0/0", empty.colContentH(0), empty.colMaxScroll(0))
	}
	// A board whose bounds are shorter than a header floors the body at 0.
	squished := wave6Kanban(1, 3)
	squished.SetBounds(Rect{X: 0, Y: 0, W: 200, H: KanbanHeaderH - 5})
	if squished.colBodyH() != 0 {
		t.Fatalf("squished colBodyH = %d, want 0", squished.colBodyH())
	}
	var none Kanban
	none.OnEvent(Event{Kind: EventScroll, Delta: 3}) // empty board: no panic
}
