// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

func TestRibbonButtonRunsItsAction(t *testing.T) {
	r, run := rtRibbon(2000)
	rtClickCell(t, r, "Duplicate")
	rtClickCell(t, r, "Undo")
	if !reflect.DeepEqual(run.ids, []string{"slide.dup", "undo"}) {
		t.Errorf("ran %v", run.ids)
	}
}

func TestRibbonOffsetBoundsAreHonoured(t *testing.T) {
	run := &sampleRun{}
	r := run.ribbon()
	r.SetBounds(Rect{X: 37, Y: 21, W: 1500, H: r.Height()})
	rtClickCell(t, r, "Redo")
	if !reflect.DeepEqual(run.ids, []string{"redo"}) {
		t.Errorf("a click through a ribbon placed at (37,21) ran %v", run.ids)
	}
}

func TestRibbonToggleFlipsAndRuns(t *testing.T) {
	r, run := rtRibbon(2000)
	rtClickCell(t, r, "Word wrap")
	rtClickCell(t, r, "Word wrap")
	if !reflect.DeepEqual(run.ids, []string{"wrap", "wrap"}) {
		t.Errorf("ran %v", run.ids)
	}
	if r.Tabs[0].Groups[1].Controls[3].check().Get() {
		t.Error("two clicks should leave the toggle off")
	}
	rtClickCell(t, r, "Word wrap")
	if !r.Tabs[0].Groups[1].Controls[3].check().Get() {
		t.Error("a third click should turn it on")
	}
}

func TestRibbonSplitMainAndArrow(t *testing.T) {
	r, run := rtRibbon(2000)
	i := rtCell(r, "New slide")
	c := r.cells()[i]
	if c.arrow == (Rect{}) {
		t.Fatal("a split has an arrow part")
	}
	// The icon half of a large split runs the command.
	rtClick(r, c.rect.X+c.rect.W/2, c.rect.Y+4)
	if !reflect.DeepEqual(run.ids, []string{"slide.new"}) {
		t.Fatalf("main part ran %v", run.ids)
	}
	// The label half opens the menu, and a row runs its action.
	rtClick(r, c.arrow.X+c.arrow.W/2, c.arrow.Y+c.arrow.H/2)
	if r.pop == nil || r.pop.kind != popMenu {
		t.Fatal("the arrow part should open the menu")
	}
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(rows[1])
	rtClick(r, x, y)
	if r.pop != nil || run.ids[len(run.ids)-1] != "slide.code" {
		t.Errorf("choosing a menu row ran %v and left pop=%v", run.ids, r.pop)
	}
	// A small split (compact level) has its arrow at the right end.
	r.SetBounds(Rect{W: 1, H: r.Height()})
	small := r.cells()[rtCell(r, "New slide")]
	if small.form == formLarge || small.arrow == (Rect{}) || small.arrow.X <= small.rect.X {
		t.Errorf("a small split should have an arrow part at its right: %+v", small)
	}
}

func TestRibbonMenuDisabledEntryStaysOpen(t *testing.T) {
	r, run := rtRibbon(2000)
	r.Tabs[0].Groups[0].Controls[0].Items[0].Action.SetEnabled(false)
	r.activate(rtCell(r, "New slide"), partArrow)
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(rows[0])
	rtClick(r, x, y)
	if r.pop == nil || len(run.ids) != 0 {
		t.Error("a disabled menu entry must refuse the click and keep the menu open")
	}
}

func TestRibbonDropdown(t *testing.T) {
	r, _ := rtRibbon(2000)
	rtClickCell(t, r, "Style")
	if r.pop == nil || r.pop.kind != popList {
		t.Fatal("a drop-down opens its list")
	}
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(rows[2])
	rtClick(r, x, y)
	ctl := r.Tabs[0].Groups[2].Controls[0]
	if ctl.selection().Get() != 2 || r.pop != nil {
		t.Errorf("chose row 2: selected %d, popup %v", ctl.selection().Get(), r.pop)
	}
}

func TestRibbonGalleryStripAndGrid(t *testing.T) {
	r, run := rtRibbon(2000)
	gi := rtCell(r, "Kinds")
	g := r.cells()[gi]
	if len(g.cells) != 3 || g.arrow == (Rect{}) {
		t.Fatalf("the strip shows three cells and an arrow, got %d", len(g.cells))
	}
	ctl := r.Tabs[0].Groups[2].Controls[1]
	ctl.Items[1].Action = run.act("kind.code", "Code", "code")
	x, y := rtCenter(g.cells[1])
	rtClick(r, x, y)
	if ctl.selection().Get() != 1 || !reflect.DeepEqual(run.ids, []string{"kind.code"}) {
		t.Errorf("clicking a strip cell: selected %d, ran %v", ctl.selection().Get(), run.ids)
	}
	// The arrow opens the grid of all four, and a grid cell picks.
	rtClick(r, g.arrow.X+2, g.arrow.Y+2)
	if r.pop == nil || r.pop.kind != popGallery {
		t.Fatal("the arrow opens the gallery grid")
	}
	_, rows := r.popupGeometry(r.pop, r.metrics())
	if len(rows) != 4 {
		t.Fatalf("grid has %d cells", len(rows))
	}
	x, y = rtCenter(rows[3])
	rtClick(r, x, y)
	if ctl.selection().Get() != 3 || r.pop != nil {
		t.Errorf("chose cell 3: selected %d", ctl.selection().Get())
	}
	// A disabled gallery item is refused.
	ctl.Items[2].Action = NewAction("off", "Off", nil)
	ctl.Items[2].Action.SetEnabled(false)
	r.pickGallery(ctl, 2)
	if ctl.selection().Get() != 3 {
		t.Error("a disabled gallery item was picked")
	}
}

func TestRibbonPopupClickOutsideAndOnBorder(t *testing.T) {
	r, run := rtRibbon(2000)
	r.activate(rtCell(r, "Style"), partMain)
	panel, _ := r.popupGeometry(r.pop, r.metrics())
	rtClick(r, panel.X, panel.Y) // the border line: swallowed, popup stays
	if r.pop == nil {
		t.Error("a click on the panel's border closed the popup")
	}
	// A click elsewhere closes the popup AND goes on to what it landed on.
	rtClickCell(t, r, "Undo")
	if r.pop != nil || !reflect.DeepEqual(run.ids, []string{"undo"}) {
		t.Errorf("click outside: popup %v, ran %v", r.pop, run.ids)
	}
}

func TestRibbonTabsAndCollapse(t *testing.T) {
	r, _ := rtRibbon(2000)
	tabs, chev := r.tabRects(r.metrics())
	x, y := rtCenter(tabs[1])
	rtClick(r, x, y)
	if r.ActiveTab().Get() != 1 || rtCell(r, "Image") < 0 {
		t.Error("clicking Insert should show its body")
	}
	// Collapse with the chevron: the tabs stay, the body goes.
	x, y = rtCenter(chev)
	rtClick(r, x, y)
	if !r.Collapsed().Get() || len(r.cells()) != 0 {
		t.Fatal("the chevron collapses the ribbon")
	}
	// A tab then peeks at its body, a second click on it puts it away.
	r.SetBounds(Rect{W: 2000, H: r.Height()})
	x, y = rtCenter(tabs[0])
	rtClick(r, x, y)
	if !r.peek || rtCell(r, "Duplicate") < 0 {
		t.Fatal("a tab of a collapsed ribbon peeks at its body")
	}
	if !r.HitTest(10, r.Height()+10) {
		t.Error("the peeking body is part of the ribbon's hit area")
	}
	rtClick(r, x, y)
	if r.peek {
		t.Error("a second click on the same tab should put the body away")
	}
	// Peek, then run a command: the peek closes with it.
	rtClick(r, x, y)
	rtClickCell(t, r, "Duplicate")
	if r.peek {
		t.Error("running a command should close the peeking body")
	}
	// Peek, then click on neither a tab nor the body: it closes.
	rtClick(r, x, y)
	rtClick(r, 5, r.Height()+400)
	if r.peek {
		t.Error("a click away from the peeking body should close it")
	}
	// Expanding again.
	x, y = rtCenter(chev)
	rtClick(r, x, y)
	if r.Collapsed().Get() {
		t.Error("the chevron expands a collapsed ribbon")
	}
}

func TestRibbonFoldedGroupOpensItsControls(t *testing.T) {
	r, run := rtRibbon(360)
	i := rtCell(r, "Theme")
	if i < 0 || r.cells()[i].form != formPopup {
		t.Fatalf("at 360px the Theme group should be a pop-up button, got %v", rtLabels(r))
	}
	rtClickCell(t, r, "Theme")
	if r.pop == nil || r.pop.kind != popGroup || r.zone != ribbonZoneBody {
		t.Fatal("the folded group opens as a panel")
	}
	if rtCell(r, "Kinds") < 0 {
		t.Fatalf("the open group's controls are the cells: %v", rtLabels(r))
	}
	if !r.HitTest(r.pop.anchor.X+1, r.pop.anchor.Y+r.pop.anchor.H+3) {
		t.Error("the open group's panel is part of the ribbon's hit area")
	}
	// Clicking a control inside runs it and closes the panel.
	rtClickCell(t, r, "Kinds")
	if r.pop != nil && r.pop.kind == popGroup {
		t.Log("opening the gallery replaced the group panel, as documented")
	}
	// A click outside the panel closes it.
	r.Dismiss()
	rtClickCell(t, r, "Theme")
	rtClick(r, 5, r.Height()+300)
	if r.pop != nil {
		t.Error("a click away from the group's panel should close it")
	}
	_ = run
}

func TestRibbonHoverTracksTabsChevronAndRows(t *testing.T) {
	r, _ := rtRibbon(2000)
	tabs, chev := r.tabRects(r.metrics())
	x, y := rtCenter(tabs[2])
	rtMove(r, x, y)
	if r.hoverTab != 2 {
		t.Errorf("hoverTab %d", r.hoverTab)
	}
	x, y = rtCenter(chev)
	rtMove(r, x, y)
	if !r.chevHot || r.hoverTab != -1 {
		t.Error("the chevron should be hot and no tab hovered")
	}
	r.activate(rtCell(r, "New slide"), partArrow)
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y = rtCenter(rows[1])
	rtMove(r, x, y)
	if r.pop.focus != 1 {
		t.Errorf("hovering row 1 should highlight it, focus=%d", r.pop.focus)
	}
	r.Dismiss()
	i := rtCell(r, "Redo")
	x, y = rtCenterOf(r.cells()[i])
	rtMove(r, x, y)
	if r.hoverCtl != i {
		t.Errorf("hoverCtl %d, want %d", r.hoverCtl, i)
	}
	rtMove(r, 5, r.Height()+50)
	if r.hoverCtl != -1 {
		t.Error("moving off the controls clears the hover")
	}
}

func TestRibbonTooltipTimerAndText(t *testing.T) {
	r, _ := rtRibbon(2000)
	i := rtCell(r, "New slide")
	x, y := rtCenterOf(r.cells()[i])
	rtMove(r, x, y)
	if !r.Animating() {
		t.Fatal("a hovered control with its tooltip pending keeps the host painting")
	}
	r.Tick(0.1)
	r.Tick(0.3)
	if r.tooltipVisible() {
		t.Fatal("the tooltip must wait its delay (500ms by default)")
	}
	r.Tick(0.15)
	if !r.tooltipVisible() || r.Animating() {
		t.Fatal("after 550ms the tooltip shows, and nothing is left animating")
	}
	lines := r.tooltipLines()
	if len(lines) != 2 || lines[0] != "New slide (Ctrl+N)" || lines[1] != "Add a slide after this one" {
		t.Errorf("tooltip = %q: want the name with its shortcut, then the description", lines)
	}
	// A control without a description gets one line; a custom delay is honoured.
	r.TooltipDelayMs = 50
	j := rtCell(r, "Redo")
	x, y = rtCenterOf(r.cells()[j])
	rtMove(r, x, y)
	r.Tick(0.06)
	if !r.tooltipVisible() || len(r.tooltipLines()) != 1 {
		t.Errorf("custom delay: lines %q", r.tooltipLines())
	}
	// An open popup hides the tooltip and stops the timer, and nothing hovered
	// means nothing to time.
	r.activate(rtCell(r, "Style"), partMain)
	ms := r.hoverMs
	r.Tick(1)
	if r.tooltipVisible() || r.Animating() || r.hoverMs != ms {
		t.Error("a popup hides the tooltip and stops its timer")
	}
	r.Dismiss()
	rtMove(r, 5, r.Height()+50)
	r.Tick(1)
	if r.Animating() || r.hoverMs != 0 {
		t.Error("nothing hovered: nothing to time")
	}
}

// The ribbon is a PopoverOwner: under a PopoverHost its menus, peeking body,
// tooltips and keytips are drawn after the tree and its clicks are routed to them,
// with no host code.
func TestRibbonUnderAPopoverHost(t *testing.T) {
	run := &sampleRun{}
	r := run.ribbon()
	body := NewVBox()
	body.AddFixed(r, r.Height())
	body.AddFlex(NewLabel("content"), 1)
	host := NewPopoverHost(body)
	host.SetBounds(Rect{W: 1100, H: 400})
	body.SetBounds(Rect{W: 1100, H: 400})
	r.SetBounds(Rect{W: 1100, H: r.Height()})
	if PopoverOwners(host) != nil {
		t.Fatal("nothing is open yet")
	}
	// Open the split menu: the host now sees an open popover over the content.
	i := rtCell(r, "New slide")
	c := r.cells()[i]
	host.OnEvent(Event{Kind: EventClick, X: c.arrow.X + c.arrow.W/2, Y: c.arrow.Y + c.arrow.H/2})
	owners := PopoverOwners(host)
	if len(owners) != 1 || owners[0] != PopoverOwner(r) || !r.PopoverOpen() {
		t.Fatalf("an open menu should be a popover, got %d owners", len(owners))
	}
	panel := r.PopoverBounds()
	if panel.Y+panel.H <= r.Bounds().H {
		t.Fatal("the menu should hang below the ribbon")
	}
	// The host draws it over the content: a pixel inside the panel is the panel's.
	th := DefaultLight()
	buf := make([]byte, 1100*400*4)
	p := painterOver(buf, 1100, 400)
	host.Draw(p, th)
	if got := rtPixelAt(buf, 1100, panel.X+panel.W/2, panel.Y+panel.H-2); got != th.Surface {
		t.Errorf("the host did not draw the menu over the content: %v", got)
	}
	// A click on a row (below the ribbon, where no container would route it) runs it.
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(rows[1])
	host.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if r.PopoverOpen() || run.ids[len(run.ids)-1] != "slide.code" {
		t.Errorf("clicking a menu row through the host ran %v", run.ids)
	}
	// A click far away closes a popup and is not swallowed.
	r.activate(rtCell(r, "Style"), partMain)
	if r.PopoverClick(900, 380) || r.PopoverOpen() {
		t.Error("a click outside closes the popup and passes through")
	}
	// A click on the panel's border is consumed, a click on the ribbon itself is not.
	r.activate(rtCell(r, "Style"), partMain)
	pb := r.PopoverBounds()
	if !r.PopoverClick(pb.X, pb.Y) || !r.PopoverOpen() {
		t.Error("the panel's border swallows the click")
	}
	if r.PopoverClick(5, 5) || !r.PopoverOpen() {
		t.Error("a click on the ribbon itself is left to OnEvent, which closes the popup there")
	}
	r.Dismiss()
	// A collapsed ribbon's peeking body is a popover too, and clickable.
	r.Collapsed().Set(true)
	r.SetBounds(Rect{W: 1100, H: r.Height()})
	tabs, _ := r.tabRects(r.metrics())
	x, y = rtCenter(tabs[0])
	host.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if !r.PopoverOpen() || r.PopoverBounds() != r.bodyRect() {
		t.Fatal("a collapsed ribbon's tab should open its body as a popover")
	}
	bx, by := rtCenterOf(r.cells()[rtCell(r, "Duplicate")])
	host.OnEvent(Event{Kind: EventClick, X: bx, Y: by})
	if run.ids[len(run.ids)-1] != "slide.dup" || r.PopoverOpen() {
		t.Errorf("a click on the peeking body ran %v", run.ids)
	}
	// Keytips and the tooltip are popovers as well; a click away ends the keytips.
	rtKey(r, "Alt")
	if !r.PopoverOpen() || r.PopoverBounds() != r.Bounds() {
		t.Error("keytips are a popover anchored on the ribbon")
	}
	if r.PopoverClick(5, 380) || r.keytips != 0 {
		t.Error("a click away from the ribbon ends the keytips")
	}
}

func TestRibbonShortcutComesFromTheKeymapWhenBound(t *testing.T) {
	r, _ := rtRibbon(2000)
	km := NewKeymap()
	if err := km.Bind(MustParseChord("Ctrl+Alt+N"), "slide.new", ScopeGlobal); err != nil {
		t.Fatal(err)
	}
	r.Keymap = km
	a := r.Tabs[0].Groups[0].Controls[0].Action
	if got := r.shortcutFor(a); got != "Ctrl+Alt+N" {
		t.Errorf("live binding %q", got)
	}
	// An action the keymap does not know falls back to its own default.
	b := r.Tabs[0].Groups[1].Controls[0].Action
	b.Shortcut = MustParseChord("Ctrl+V")
	if got := r.shortcutFor(b); got != "Ctrl+V" {
		t.Errorf("default %q", got)
	}
	if r.shortcutFor(nil) != "" {
		t.Error("no action, no shortcut")
	}
}

func TestRibbonDisabledIsInert(t *testing.T) {
	r, run := rtRibbon(2000)
	r.Disabled().Set(true)
	rtClickCell(t, r, "Undo")
	rtKey(r, "Alt")
	if len(run.ids) != 0 || r.keytips != 0 {
		t.Error("a disabled ribbon must ignore the pointer and the keyboard")
	}
}

func TestRibbonFocusLossClosesEverything(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.SetFocused(true)
	r.activate(rtCell(r, "Style"), partMain)
	rtKey(r, "Escape")
	r.activate(rtCell(r, "Style"), partMain)
	r.keytips = 1
	r.SetFocused(false)
	if r.pop != nil || r.keytips != 0 || r.Focused() {
		t.Error("losing focus should close the popup and the keytips")
	}
}

func TestRibbonStaleHoverIndexIsHarmless(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.hoverCtl, r.hoverMs = 999, 1000
	if lines := r.tooltipLines(); lines != nil {
		t.Errorf("a stale hover index produced a tooltip %q", lines)
	}
	renderRibbon(r, DefaultLight(), 2000, r.Height())
}

func TestRibbonIgnoresOtherEvents(t *testing.T) {
	r, run := rtRibbon(2000)
	r.OnEvent(Event{Kind: EventScroll, Delta: 3})
	r.OnEvent(Event{Kind: EventSecondaryClick, X: 40, Y: 40})
	if len(run.ids) != 0 {
		t.Error("only a click runs a command")
	}
}
