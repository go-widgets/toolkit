// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
)

// rtQuickRibbon is the sample ribbon with its quick access bar, at width w.
func rtQuickRibbon(w int) (*Ribbon, *sampleRun) {
	run := &sampleRun{}
	r := run.withQuick(run.ribbon())
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: r.Height()})
	return r, run
}

// rtClickQuick presses the quick access command labelled label.
func rtClickQuick(t *testing.T, r *Ribbon, label string) {
	t.Helper()
	rc, ok := r.QuickRect(label)
	if !ok {
		t.Fatalf("the quick access bar is not showing %q", label)
	}
	x, y := rtCenter(rc)
	rtClick(r, x, y)
}

// TestTheQuickBarRunsACommandFromEveryTab is the whole reason the bar exists.
//
// ⛔ Save is on NO tab of the sample, so nothing but the bar can have run it.
// A test that put Save on a tab as well would pass with the bar deleted.
func TestTheQuickBarRunsACommandFromEveryTab(t *testing.T) {
	r, run := rtQuickRibbon(900)
	for i := range r.Tabs {
		r.ActiveTab().Set(i)
		run.ids = nil
		rtClickQuick(t, r, "Save")
		if len(run.ids) != 1 || run.ids[0] != "save" {
			t.Errorf("on the %s tab, pressing Save ran %v", r.Tabs[i].Label, run.ids)
		}
	}
}

// TestTheQuickBarSurvivesACollapsedRibbon is the case it is most needed in:
// the tab strip is all there is, and these are the only commands on it.
func TestTheQuickBarSurvivesACollapsedRibbon(t *testing.T) {
	r, run := rtQuickRibbon(900)
	r.Collapsed().Set(true)
	r.SetBounds(Rect{X: 0, Y: 0, W: 900, H: r.Height()})

	rc, ok := r.QuickRect("Save")
	if !ok {
		t.Fatal("a collapsed ribbon hides its quick access bar, which is the one row it still has")
	}
	if rc.Y+rc.H > r.Height() {
		t.Errorf("the bar is drawn from %d to %d and a collapsed ribbon is %d tall",
			rc.Y, rc.Y+rc.H, r.Height())
	}
	x, y := rtCenter(rc)
	rtClick(r, x, y)
	if len(run.ids) != 1 || run.ids[0] != "save" {
		t.Errorf("pressing Save on a collapsed ribbon ran %v", run.ids)
	}
}

func TestTheQuickBarIsBesideTheTabsAndNotOverThem(t *testing.T) {
	r, _ := rtQuickRibbon(900)
	m := r.metrics()
	tabs, chev := r.tabRects(m)
	quick := r.quickRects(m)
	if len(quick) != 3 {
		t.Fatalf("%d quick access cells for 3 commands", len(quick))
	}
	last := tabs[len(tabs)-1]
	if quick[0].X < last.X+last.W {
		t.Errorf("the bar starts at %d and the last tab ends at %d, so it is drawn over the tabs",
			quick[0].X, last.X+last.W)
	}
	if end := quick[len(quick)-1]; end.X+end.W > chev.X {
		t.Errorf("the bar ends at %d and the collapse chevron starts at %d",
			end.X+end.W, chev.X)
	}
	// One row: every cell on the same line as the tabs.
	for i, rc := range quick {
		if rc.Y != last.Y || rc.H != last.H {
			t.Errorf("quick cell %d is at y=%d h=%d and the tabs are at y=%d h=%d",
				i, rc.Y, rc.H, last.Y, last.H)
		}
	}
}

func TestAQuickCommandThatCannotRunIsGreyedAndTakesNoPress(t *testing.T) {
	r, run := rtQuickRibbon(900)
	r.Quick[0].Action.Enabled.Set(false)

	rc, _ := r.QuickRect("Save")
	x, y := rtCenter(rc)
	rtClick(r, x, y)
	if len(run.ids) != 0 {
		t.Errorf("a disabled quick access command ran anyway: %v", run.ids)
	}
	// And the pointer does not light it up, which is what says beforehand that
	// pressing would do nothing.
	rtMove(r, x, y)
	if r.hoverQuick >= 0 {
		t.Error("a disabled quick access command lights up under the pointer")
	}
}

func TestAQuickToggleFlipsAndRuns(t *testing.T) {
	r, run := rtQuickRibbon(900)
	on := r.Quick[2].Checked
	if on.Get() {
		t.Fatal("the sample's toggle starts on")
	}
	rtClickQuick(t, r, "Word wrap")
	if !on.Get() {
		t.Error("pressing the toggle did not turn it on")
	}
	if len(run.ids) != 1 || run.ids[0] != "quick.wrap" {
		t.Errorf("it ran %v", run.ids)
	}
	rtClickQuick(t, r, "Word wrap")
	if on.Get() {
		t.Error("pressing it again did not turn it off")
	}
}

// TestTheQuickBarStopsAtTwenty is the reference's own number: the QAT "consists
// of a combination of up to 20 Commands".
func TestTheQuickBarStopsAtTwenty(t *testing.T) {
	r, run := rtQuickRibbon(2000)
	r.Quick = nil
	for i := 0; i < QuickMax+5; i++ {
		r.Quick = append(r.Quick, &RibbonControl{Kind: RibbonButton,
			Action: run.act("q"+string(rune('a'+i)), "Q"+string(rune('a'+i)), "star")})
	}
	if n := len(r.quickRects(r.metrics())); n != QuickMax {
		t.Errorf("%d cells for %d commands, want the %d the bar shows", n, len(r.Quick), QuickMax)
	}
	// ⛔ And the ones past the end say they are NOT there. Handing back a
	// rectangle for a command nobody can see would have a harness press an
	// empty patch of tab strip and report the command as broken.
	if _, ok := r.QuickRect("Q" + string(rune('a'+QuickMax))); ok {
		t.Error("a command past the twentieth is published as being somewhere")
	}
}

func TestEveryQuickCommandCarriesAMark(t *testing.T) {
	// ⛔ The row has no space for a caption, so a mark is the whole of what a
	// reader is given. A cell with none is a blank square that cannot be told
	// from the one beside it.
	r, _ := rtQuickRibbon(900)
	for i, c := range r.Quick {
		if c.icon() == nil {
			t.Errorf("quick access command %d (%q) carries no mark", i, c.label())
		}
	}
}

// TestAQuickCommandSaysWhatItDoes is the other half of being icon-only.
func TestAQuickCommandSaysWhatItDoes(t *testing.T) {
	r, _ := rtQuickRibbon(900)
	rc, _ := r.QuickRect("Save")
	x, y := rtCenter(rc)
	rtMove(r, x, y)
	r.Tick(10) // past the tooltip's own delay
	if !r.tooltipVisible() {
		t.Fatal("resting on a quick access mark shows no tooltip, so nothing ever says what it does")
	}
	lines := r.tooltipLines()
	if len(lines) != 2 || lines[0] != "Save" || lines[1] != "Write the deck to disk" {
		t.Errorf("the tooltip says %q", lines)
	}
}

// TestAQuickTooltipIsPulledBackInsideTheRibbon covers the edge the bar lives
// on: it is at the RIGHT end of the row, so a bubble hung off its left edge
// would otherwise run out past the window.
func TestAQuickTooltipIsPulledBackInsideTheRibbon(t *testing.T) {
	r, _ := rtQuickRibbon(900)
	rc, _ := r.QuickRect("Save")
	x, y := rtCenter(rc)
	rtMove(r, x, y)
	r.Tick(10)

	buf := make([]byte, 4*900*r.Height())
	p := painterOver(buf, 900, r.Height())
	r.DrawPopover(p, DefaultLight()) // would panic on a buffer it drew past
	m := r.metrics()
	w := 0
	for _, l := range r.tooltipLines() {
		w = max(w, r.textWidth(l))
	}
	if anchor := r.tooltipAnchor(m); anchor.X+w+2*m.padX <= r.Bounds().X+r.Bounds().W {
		t.Skip("the bubble fits at this width, so there is nothing to pull back")
	}
}

func TestTheArrowKeysReachTheQuickBarFromTheTabs(t *testing.T) {
	r, run := rtQuickRibbon(900)
	r.SetFocused(true)
	// Right off the LAST tab goes into the bar rather than wrapping round to
	// the first — the bar is what is physically to the right.
	r.ActiveTab().Set(len(r.Tabs) - 1)
	r.focusTab = len(r.Tabs) - 1
	rtKey(r, "ArrowRight")
	if r.zone != ribbonZoneQuick {
		t.Fatalf("Right off the last tab left the keyboard in zone %d", r.zone)
	}
	if r.focusQuick != 0 {
		t.Errorf("it landed on quick command %d", r.focusQuick)
	}
	rtKey(r, "ArrowRight")
	if r.focusQuick != 1 {
		t.Errorf("Right along the bar went to %d", r.focusQuick)
	}
	rtKey(r, "Enter")
	if len(run.ids) != 1 || run.ids[0] != "quick.undo" {
		t.Errorf("Enter on the bar ran %v", run.ids)
	}
	// Left off the START of the bar goes back to the tabs it shares a row
	// with, rather than wrapping to its own far end.
	r.focusQuick = 0
	rtKey(r, "ArrowLeft")
	if r.zone != ribbonZoneTabs {
		t.Errorf("Left off the start of the bar left the keyboard in zone %d", r.zone)
	}
}

func TestTheTabsStillWrapWithNoQuickBar(t *testing.T) {
	// ⛔ The control. Right off the last tab wraps when there is no bar to go
	// into, which is what it did before the bar existed.
	r, _ := rtRibbon(900)
	r.SetFocused(true)
	r.ActiveTab().Set(len(r.Tabs) - 1)
	r.focusTab = len(r.Tabs) - 1
	rtKey(r, "ArrowRight")
	if r.zone != ribbonZoneTabs || r.focusTab != 0 {
		t.Errorf("with no quick access bar, Right off the last tab went to zone %d tab %d",
			r.zone, r.focusTab)
	}
}

// TestAltThenADigitRunsAQuickCommand is the bar's keyboard point: one
// keystroke after Alt, without choosing a tab first.
func TestAltThenADigitRunsAQuickCommand(t *testing.T) {
	r, run := rtQuickRibbon(900)
	r.SetFocused(true)
	rtKey(r, "Alt")
	if r.keytips != 1 {
		t.Fatalf("Alt did not show the keytips (keytips=%d)", r.keytips)
	}
	rtKey(r, "1")
	if len(run.ids) != 1 || run.ids[0] != "save" {
		t.Errorf("Alt then 1 ran %v, want the first quick access command", run.ids)
	}
	if r.keytips != 0 {
		t.Error("the keytips are still showing after a command ran")
	}
}

func TestTheQuickTipsAreDigitsAndTheTabTipsAreLetters(t *testing.T) {
	// ⛔ They are shown at the same time, so the two sets must not collide.
	r, _ := rtQuickRibbon(900)
	tabs := map[string]bool{}
	for _, tip := range r.tabTips() {
		tabs[tip] = true
	}
	for i, tip := range r.quickTips() {
		if i < 9 && tip == "" {
			t.Errorf("quick access command %d has no keytip", i)
		}
		if tabs[tip] {
			t.Errorf("quick access command %d and a tab both answer to %q", i, tip)
		}
	}
	// Past the ninth there is no tip: a two-digit one would need a second
	// keystroke to tell 1 from 10.
	r.Quick = nil
	run := &sampleRun{}
	for i := 0; i < 12; i++ {
		r.Quick = append(r.Quick, &RibbonControl{Kind: RibbonButton,
			Action: run.act("q", "Q", "star")})
	}
	tips := r.quickTips()
	for i := 9; i < len(tips); i++ {
		if tips[i] != "" {
			t.Errorf("quick access command %d is tipped %q", i, tips[i])
		}
	}
}

func TestAQuickCommandCanBeTheOnlyPlaceACommandIs(t *testing.T) {
	// The reference allows it outright: "the QAT can contain unique Commands
	// that are not available elsewhere in the ribbon UI."
	r, _ := rtQuickRibbon(900)
	for i := range r.Tabs {
		r.ActiveTab().Set(i)
		if _, shown := r.ControlRect("Save"); shown {
			t.Fatalf("Save is on the %s tab, so this test proves nothing about the bar", r.Tabs[i].Label)
		}
	}
	if _, ok := r.QuickRect("Save"); !ok {
		t.Error("Save is on no tab and not on the bar either, so nothing can reach it")
	}
}

func TestARibbonWithNoQuickBarIsUnchanged(t *testing.T) {
	plain, _ := rtRibbon(900)
	if n := len(plain.quickRects(plain.metrics())); n != 0 {
		t.Errorf("a ribbon with no Quick laid out %d cells", n)
	}
	if _, ok := plain.QuickRect("Save"); ok {
		t.Error("a ribbon with no Quick says it is showing one")
	}
	// And the tab row is the height it always was.
	withBar, _ := rtQuickRibbon(900)
	if plain.Height() != withBar.Height() {
		t.Errorf("the bar changed the ribbon's height: %d without, %d with",
			plain.Height(), withBar.Height())
	}
}

func TestTheQuickBarIsDrawn(t *testing.T) {
	// ⛔ A widget tree that compiles is not a widget tree that paints. This
	// counts the ink in the bar's own cells and nowhere else.
	r, _ := rtQuickRibbon(900)
	h := r.Height()
	buf := make([]byte, 4*900*h)
	theme := DefaultLight()
	for i := range buf {
		buf[i] = 0
	}
	r.Draw(painterOver(buf, 900, h), theme)

	rc, _ := r.QuickRect("Save")
	ink := 0
	for y := rc.Y; y < rc.Y+rc.H; y++ {
		for x := rc.X; x < rc.X+rc.W; x++ {
			if c := rtPixelAt(buf, 900, x, y); c != theme.Background {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Error("the Save cell is the bare background, so its mark was never painted")
	}
}

func TestAQuickToggleShowsItIsOn(t *testing.T) {
	r, _ := rtQuickRibbon(900)
	r.Quick[2].Checked = mvvm.NewObservable(true)
	h := r.Height()
	theme := DefaultLight()
	on := rtQuickInk(r, theme, h, "Word wrap")
	r.Quick[2].Checked.Set(false)
	off := rtQuickInk(r, theme, h, "Word wrap")
	if on <= off {
		t.Errorf("a toggle that is on has %d pixels of ink and one that is off %d, "+
			"so nothing on the bar says which it is", on, off)
	}
}

// rtQuickInk is how many pixels of the named quick cell are not the background.
func rtQuickInk(r *Ribbon, theme *Theme, h int, label string) int {
	buf := make([]byte, 4*900*h)
	r.Draw(painterOver(buf, 900, h), theme)
	rc, _ := r.QuickRect(label)
	n := 0
	for y := rc.Y; y < rc.Y+rc.H; y++ {
		for x := rc.X; x < rc.X+rc.W; x++ {
			if c := rtPixelAt(buf, 900, x, y); c != theme.Background {
				n++
			}
		}
	}
	return n
}
