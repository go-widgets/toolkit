// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// QuickMax is how many commands the quick access bar shows. The Windows Ribbon
// framework says the QAT "consists of a combination of up to 20 Commands", and
// that is the number taken here; past it the rest are not drawn, and
// [Ribbon.QuickRect] says so rather than pretending they are somewhere.
const QuickMax = 20

// quickRects places the quick access bar along the tab row, from the right,
// ending before the collapse chevron.
//
// ⛔ ON THE TAB ROW, which is not where Microsoft puts it. The reference QAT
// lives in the window's TITLE BAR by default, with "show below the ribbon" as
// the alternative. A toolkit widget has no title bar to live in -- the ribbon
// begins at its own first pixel -- so the only row that is on screen whatever
// the ribbon is doing, and the only row with space going spare, is the tab row
// itself: tabs from the left, these from the right, the chevron at the end.
//
// That placement is also what makes the bar worth having, and the reason it
// exists at all: the commands a reader needs from every tab (Save, Undo) are on
// ONE tab in a ribbon, so saving means leaving the tab you are working in. The
// bar is the row that does not change when the tab does -- including when the
// ribbon is collapsed, when the tab strip is all there is.
func (r *Ribbon) quickRects(m ribbonMetrics) []Rect {
	n := r.quickCount()
	if n == 0 {
		return nil
	}
	b := r.Bounds()
	w := m.tabH // square cells, the height of the row they are on
	// Right to left from the chevron, then handed back left to right so the
	// index of a rect is the index of the control.
	out := make([]Rect, n)
	x := b.X + b.W - m.tabH - m.padX - m.gap
	for i := n - 1; i >= 0; i-- {
		x -= w
		out[i] = Rect{X: x, Y: b.Y, W: w, H: m.tabH}
	}
	return out
}

// quickCount is how many of Quick are shown: all of them, up to QuickMax.
func (r *Ribbon) quickCount() int { return min(len(r.Quick), QuickMax) }

// QuickRect is where the quick access command labelled label is, in surface
// coordinates; false when the bar is not showing one by that name — it has
// none, or it is past QuickMax.
//
// It is [Ribbon.ControlRect] for the quick access bar, and exists for the same
// reason: a canvas has no DOM, so a harness driving the surface has nothing to
// find by name and would otherwise press coordinates it guessed.
func (r *Ribbon) QuickRect(label string) (Rect, bool) {
	rects := r.quickRects(r.metrics())
	for i, c := range rects {
		if r.Quick[i].label() == label {
			return c, true
		}
	}
	return Rect{}, false
}

// quickAt is the index of the quick access command at the surface point, or -1.
// A command that cannot run is not "at" the point: it is drawn, greyed, and
// takes no press.
func (r *Ribbon) quickAt(x, y int) int {
	for i, rc := range r.quickRects(r.metrics()) {
		if rc.Contains(x, y) && r.Quick[i].enabled() {
			return i
		}
	}
	return -1
}

// drawQuick paints the bar: one square icon cell per command, hot under the
// pointer or the keyboard, greyed when it cannot run.
//
// ⛔ ICONS ONLY, and not because it is less work. The row is one line of text
// tall, so a caption beside each mark would either push the tabs off the left
// or set the type smaller than the tabs beside it — and the reference bar is
// icons too. A command here therefore MUST carry an Icon; the test that says so
// is TestEveryQuickCommandCarriesAMark, because a cell with no mark is a blank
// square that cannot be told from the one beside it.
func (r *Ribbon) drawQuick(p painter.Painter, theme *Theme, m ribbonMetrics) {
	rects := r.quickRects(m)
	for i, rc := range rects {
		c := r.Quick[i]
		on := c.enabled()
		hot := on && (i == r.hoverQuick ||
			(r.Focused() && r.zone == ribbonZoneQuick && i == r.focusQuick && r.keytips == 0))
		if hot {
			fillRoundRect(p, rc.X, rc.Y+scaled(2), rc.W, rc.H-scaled(2), scaled(3), theme.SurfaceAlt)
		}
		if c.Kind == RibbonToggle && c.Checked != nil && c.Checked.Get() {
			strokeRect(p, rc.X, rc.Y+scaled(2), rc.W, rc.H-scaled(2), ribbonAccent(theme))
		}
		if icon := c.icon(); icon != nil {
			s := m.iconS
			icon(p, Rect{X: rc.X + (rc.W-s)/2, Y: rc.Y + (rc.H-s)/2, W: s, H: s}, ribbonInk(theme, on))
		}
	}
	if r.Focused() && r.zone == ribbonZoneQuick && r.keytips == 0 && len(rects) > 0 {
		f := rects[min(r.focusQuick, len(rects)-1)]
		strokeRect(p, f.X+1, f.Y+1, f.W-2, f.H-2, ribbonAccent(theme))
	}
}

// runQuick presses quick access command i.
//
// A toggle flips before it runs, as a toggle on the ribbon itself does, so the
// two surfaces of one command cannot disagree about whether it is on.
func (r *Ribbon) runQuick(i int) {
	c := r.Quick[i]
	if !c.enabled() {
		return
	}
	if c.Kind == RibbonToggle && c.Checked != nil {
		c.Checked.Set(!c.Checked.Get())
	}
	if c.Action != nil {
		c.Action.Execute()
	}
	if r.byKey {
		r.leave()
	}
}

// quickTips are the keytips of the quick access commands: 1 to 9, as the
// reference numbers them, and nothing past the ninth — a two-digit tip would
// need a second keystroke to tell 1 from 10.
func (r *Ribbon) quickTips() []string {
	out := make([]string, r.quickCount())
	for i := range out {
		if i < 9 {
			out[i] = string(rune('1' + i))
		}
	}
	return out
}

// quickKey is the bar as a toolbar: Left and Right move along it, Left off its
// start goes back to the tabs it shares a row with, Enter or Space runs, and
// Escape hands the keyboard back.
func (r *Ribbon) quickKey(ev Event) {
	n := r.quickCount()
	if n == 0 {
		r.zone = ribbonZoneTabs
		return
	}
	switch ev.Code {
	case "ArrowRight":
		r.focusQuick = stepUsableQuick(r, r.focusQuick, 1)
	case "ArrowLeft":
		// ⛔ It does NOT wrap, and the tabs do. One row, read left to right:
		// wrapping from the first mark round to the last would walk away from
		// the tabs that are physically to its left, which is the one direction
		// somebody pressing Left is asking for.
		if next := stepUsableQuick(r, r.focusQuick, -1); next < r.focusQuick {
			r.focusQuick = next
		} else {
			r.zone, r.focusTab = ribbonZoneTabs, r.activeIndex()
		}
	case "Home":
		r.focusQuick = stepUsableQuick(r, n-1, 1)
	case "End":
		r.focusQuick = stepUsableQuick(r, 0, -1)
	case "Enter", " ", "Space":
		r.runQuick(r.focusQuick)
	case "ArrowDown":
		r.enterBody()
	case "Escape":
		r.leave()
	}
}

// stepUsableQuick is the next command in direction d that can be run, or where
// it started when none can.
func stepUsableQuick(r *Ribbon, from, d int) int {
	n := r.quickCount()
	for i, at := 0, from; i < n; i++ {
		at = (at + d + n) % n
		if r.Quick[at].enabled() {
			return at
		}
	}
	return from
}

// enterQuick moves the keyboard to the bar, on the first command that can run.
func (r *Ribbon) enterQuick() bool {
	for i := 0; i < r.quickCount(); i++ {
		if r.Quick[i].enabled() {
			r.zone, r.focusQuick = ribbonZoneQuick, i
			return true
		}
	}
	return false
}
