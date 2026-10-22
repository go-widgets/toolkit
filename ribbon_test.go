// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
)

// rtRibbon is a sample ribbon sized to width w, with its run log.
func rtRibbon(w int) (*Ribbon, *sampleRun) {
	run := &sampleRun{}
	r := run.ribbon()
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: r.Height()})
	return r, run
}

// rtCell is the index of the cell whose control (or folded group) is labelled
// label, -1 when none is shown.
func rtCell(r *Ribbon, label string) int {
	for i, c := range r.cells() {
		l := ""
		if c.form == formPopup {
			l = c.grp.Label
		} else {
			l = c.ctrl.label()
		}
		if l == label {
			return i
		}
	}
	return -1
}

// rtCenter is the centre of a rectangle.
func rtCenter(rc Rect) (int, int) { return rc.X + rc.W/2, rc.Y + rc.H/2 }

// rtClick delivers a click at the surface point (x, y), as a container would.
func rtClick(r *Ribbon, x, y int) {
	r.OnEvent(Event{Kind: EventClick, X: x - r.Bounds().X, Y: y - r.Bounds().Y})
}

// rtMove delivers a pointer move at the surface point (x, y).
func rtMove(r *Ribbon, x, y int) {
	r.OnEvent(Event{Kind: EventMouseMove, X: x - r.Bounds().X, Y: y - r.Bounds().Y})
}

// rtClickCell clicks the centre of the cell labelled label.
func rtClickCell(t *testing.T, r *Ribbon, label string) {
	t.Helper()
	i := rtCell(r, label)
	if i < 0 {
		t.Fatalf("no cell %q among %v", label, rtLabels(r))
	}
	x, y := rtCenterOf(r.cells()[i])
	rtClick(r, x, y)
}

func rtCenterOf(c ribbonCell) (int, int) { return rtCenter(c.rect) }

func rtLabels(r *Ribbon) []string {
	var out []string
	for _, c := range r.cells() {
		if c.form == formPopup {
			out = append(out, "["+c.grp.Label+"]")
		} else {
			out = append(out, c.ctrl.label())
		}
	}
	return out
}

// rtKey presses a key.
func rtKey(r *Ribbon, code string) { r.OnEvent(Event{Kind: EventKeyDown, Code: code}) }

// fullWidth is the width the active tab needs with every group at full size.
func fullWidth(r *Ribbon) int {
	m := r.metrics()
	w := 0
	for _, g := range r.activeTab().Groups {
		w += r.groupWidth(g, RibbonFull, m) + m.sepW
	}
	return w
}

func TestRibbonFitsAtFullWidth(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.SetBounds(Rect{W: fullWidth(r), H: r.Height()})
	for i, l := range r.Levels() {
		if l != RibbonFull {
			t.Errorf("group %d is at level %d at full width", i, l)
		}
	}
	r.SetBounds(Rect{W: fullWidth(r) - 1, H: r.Height()})
	lv := r.Levels()
	if lv[2] == RibbonFull || lv[0] != RibbonFull || lv[1] != RibbonFull {
		t.Errorf("one pixel short, only the lowest-priority group (Theme) should step down: %v", lv)
	}
}

// The ribbon "shows the largest valid combination that fits": as the width
// shrinks no group ever grows back, the total never exceeds the width while a
// group can still shrink, and groups give up their large form in priority order.
func TestRibbonScalesDownMonotonically(t *testing.T) {
	r, _ := rtRibbon(2000)
	m := r.metrics()
	tab := r.activeTab()
	prev := make([]RibbonLevel, len(tab.Groups))
	leftFull := make([]int, len(tab.Groups))
	for w := fullWidth(r) + 40; w >= 20; w-- {
		r.SetBounds(Rect{W: w, H: r.Height()})
		lv := r.Levels()
		total, canShrink := 0, false
		for i, g := range tab.Groups {
			if lv[i] < prev[i] {
				t.Fatalf("width %d: group %d grew back from %d to %d", w, i, prev[i], lv[i])
			}
			if lv[i] != RibbonFull && leftFull[i] == 0 {
				leftFull[i] = w
			}
			total += r.groupWidth(g, lv[i], m) + m.sepW
			if _, ok := r.nextLevel(g, lv[i], m); ok {
				canShrink = true
			}
		}
		if total > w && canShrink {
			t.Fatalf("width %d: total %d does not fit though a group can still shrink: %v", w, total, lv)
		}
		copy(prev, lv)
	}
	// Theme (priority 1) leaves full size first, then Edit (2), then Slides (3).
	if !(leftFull[2] > leftFull[1] && leftFull[1] > leftFull[0] && leftFull[0] > 0) {
		t.Errorf("groups left full size at widths %v, want Theme > Edit > Slides", leftFull)
	}
	// At 20px every group is as narrow as it can get: a level is only taken when
	// it is narrower than the one before, so folding into a (wider) pop-up button
	// is skipped when icon-only already beats it.
	for i, l := range prev {
		if _, ok := r.nextLevel(tab.Groups[i], l, m); ok {
			t.Errorf("at 20px group %d is at level %d and could still shrink", i, l)
		}
	}
}

func TestRibbonEqualPrioritiesShrinkRightToLeft(t *testing.T) {
	run := &sampleRun{}
	mk := func(label string) *RibbonGroup {
		return &RibbonGroup{Label: label, Priority: 1, Controls: []*RibbonControl{
			{Kind: RibbonButton, Action: run.act(label+"1", label+" one", "page"), Large: true},
			{Kind: RibbonButton, Action: run.act(label+"2", label+" two", "page")}}}
	}
	r := NewRibbon(&RibbonTab{Label: "T", Groups: []*RibbonGroup{mk("A"), mk("B"), mk("C")}})
	r.SetBounds(Rect{W: fullWidth(r) - 1, H: r.Height()})
	lv := r.Levels()
	if lv[2] == RibbonFull || lv[1] != RibbonFull || lv[0] != RibbonFull {
		t.Errorf("equal priorities should shrink the rightmost group first, got %v", lv)
	}
}

func TestRibbonSingleControlGroupNeverFoldsIntoAPopup(t *testing.T) {
	run := &sampleRun{}
	one := &RibbonGroup{Label: "One", Priority: 0, Controls: []*RibbonControl{
		{Kind: RibbonButton, Action: run.act("a", "Alpha button", "page"), Large: true}}}
	r := NewRibbon(&RibbonTab{Label: "T", Groups: []*RibbonGroup{one}})
	r.SetBounds(Rect{W: 10, H: r.Height()})
	if l := r.Levels()[0]; l == RibbonPopup {
		t.Error("a group of one control folded into a pop-up; the guidelines say to leave it a single button")
	}
	if l := r.Levels()[0]; l != RibbonIconOnly {
		t.Errorf("a single control with an icon should end at icon-only, got %d", l)
	}
}

func TestRibbonLevelsChangeTheFormOfControls(t *testing.T) {
	r, _ := rtRibbon(2000)
	m := r.metrics()
	g := r.activeTab().Groups[0]
	form := func(l RibbonLevel) ribbonForm {
		return r.layoutGroup(g, l, 0, 0, m)[0].form
	}
	if form(RibbonFull) != formLarge || form(RibbonCompact) != formSmall || form(RibbonIconOnly) != formIcon || form(RibbonPopup) != formPopup {
		t.Error("the first control of Slides should be large, small, icon-only, then the group's pop-up button")
	}
	// A control with no icon keeps its text at icon-only, or it would vanish.
	bare := &RibbonControl{Kind: RibbonButton, Action: NewAction("x", "Bare", nil)}
	if bare.formAt(RibbonIconOnly) != formSmall {
		t.Error("an iconless control must keep its label at icon-only")
	}
	// A drop-down is a drop-down at every level; a gallery is a strip only at full size.
	dd := &RibbonControl{Kind: RibbonDropdown, Label: "D"}
	ga := &RibbonControl{Kind: RibbonGallery, Label: "G"}
	if dd.formAt(RibbonIconOnly) != formDropdown || ga.formAt(RibbonFull) != formGallery || ga.formAt(RibbonCompact) != formSmall {
		t.Error("drop-down and gallery forms are wrong")
	}
}

func TestRibbonHeightCollapsed(t *testing.T) {
	r, _ := rtRibbon(800)
	m := r.metrics()
	full := r.Height()
	if full != m.tabH+1+m.bodyH+m.labelH+1 {
		t.Errorf("expanded height %d", full)
	}
	r.Collapsed().Set(true)
	if r.Height() != m.tabH+1 {
		t.Errorf("collapsed height %d, want the tab strip and its rule", r.Height())
	}
	if len(r.cells()) != 0 {
		t.Error("a collapsed ribbon shows no controls")
	}
}

func TestRibbonLargeLines(t *testing.T) {
	cases := map[string][]string{
		"Paste":         {"Paste"},
		"New slide":     {"New", "slide"},
		"Paste special": {"Paste", "special"},
		"Save as image": {"Save as", "image"},
		"":              {""},
	}
	for in, want := range cases {
		got := largeLines(in)
		if len(got) != len(want) || got[0] != want[0] || (len(want) == 2 && got[1] != want[1]) {
			t.Errorf("largeLines(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRibbonHiddenAndDisabledControls(t *testing.T) {
	r, run := rtRibbon(2000)
	dup := rtCell(r, "Duplicate")
	if dup < 0 {
		t.Fatal("Duplicate not shown")
	}
	c := r.cells()[dup].ctrl
	c.Action.SetVisible(false)
	if rtCell(r, "Duplicate") >= 0 {
		t.Error("an invisible action's control is still laid out")
	}
	c.Action.SetVisible(true)
	c.Action.SetEnabled(false)
	rtClickCell(t, r, "Duplicate")
	if len(run.ids) != 0 {
		t.Errorf("a disabled control ran %v", run.ids)
	}
	if c.enabled() || !c.visible() {
		t.Error("enabled/visible do not follow the action")
	}
	// A control with no Action at all is always enabled and visible.
	if !(&RibbonControl{}).enabled() || !(&RibbonControl{}).visible() {
		t.Error("an action-less control should be enabled and visible")
	}
}

func TestRibbonNoTabs(t *testing.T) {
	r := NewRibbon()
	r.SetBounds(Rect{W: 300, H: 40})
	if r.activeTab() != nil || r.Levels() != nil || len(r.cells()) != 0 || len(r.body().cells) != 0 {
		t.Error("a ribbon without tabs has nothing to show")
	}
	if a := r.A11y(); a.Role != RoleToolbar || a.Value != "" {
		t.Errorf("A11y of an empty ribbon = %+v", a)
	}
	rtKey(r, "ArrowRight")
	rtKey(r, "ArrowDown")
	// Drawing an empty ribbon must not panic.
	r.SetFocused(true)
	renderRibbon(r, DefaultLight(), 300, 40)
}

func TestRibbonActiveTabIsClamped(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.ActiveTab().Set(99)
	if r.activeIndex() != len(r.Tabs)-1 {
		t.Errorf("active %d, want the last tab", r.activeIndex())
	}
	r.ActiveTab().Set(-5)
	if r.activeIndex() != 0 {
		t.Errorf("active %d, want the first tab", r.activeIndex())
	}
}

// A toggle's checked state and a drop-down's selection are shared Observables:
// two views of the same state stay in step, with no copying.
func TestRibbonStateIsShared(t *testing.T) {
	r, _ := rtRibbon(2000)
	shared := mvvm.NewObservable(false)
	wrap := r.Tabs[0].Groups[1].Controls[3]
	wrap.Checked = shared
	rtClickCell(t, r, "Word wrap")
	if !shared.Get() {
		t.Error("clicking the toggle did not set the shared observable")
	}
	shared.Set(false)
	if wrap.check().Get() {
		t.Error("the shared observable did not reach the toggle")
	}
	// ActiveTab and Collapsed are the host's to bind.
	var seen []int
	r.ActiveTab().Subscribe(func(i int) { seen = append(seen, i) })
	cx, cy := rtCenter(mustTabRect(t, r, 1))
	rtClick(r, cx, cy)
	if len(seen) != 1 || seen[0] != 1 {
		t.Errorf("tab subscribers saw %v", seen)
	}
}

func mustTabRect(t *testing.T, r *Ribbon, i int) Rect {
	t.Helper()
	tabs, _ := r.tabRects(r.metrics())
	return tabs[i]
}

func TestNewCommandAction(t *testing.T) {
	ran := 0
	can := true
	cmd := mvvm.NewCommand(func() { ran++ }, func() bool { return can })
	a := NewCommandAction("c", "Cmd", cmd)
	if !a.IsEnabled() || !a.Execute() || ran != 1 {
		t.Fatalf("a command that can execute should run through its action (ran %d)", ran)
	}
	can = false
	cmd.RaiseCanExecuteChanged()
	if a.IsEnabled() || a.Execute() || ran != 1 {
		t.Error("the action did not follow CanExecute off")
	}
	can = true
	cmd.RaiseCanExecuteChanged()
	if !a.IsEnabled() {
		t.Error("the action did not follow CanExecute back on")
	}
	// Disabled from the start.
	can = false
	b := NewCommandAction("d", "D", cmd)
	if b.IsEnabled() {
		t.Error("an action made over a command that cannot run starts disabled")
	}
}
