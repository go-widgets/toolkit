// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strings"

// partMain and partArrow say which part of a control an activation targets; a
// value of 0 or more is a gallery cell.
const (
	partMain  = -1
	partArrow = -2
)

// SetFocused records whether the ribbon holds keyboard focus. Losing it closes
// what the ribbon had open — a menu, a peeking body, the keytips — so nothing is
// left hanging over the page.
func (r *Ribbon) SetFocused(focused bool) {
	r.focusState.SetFocused(focused)
	if !focused {
		r.Dismiss()
	}
}

// Dismiss closes whatever the ribbon has open over the content: a menu, a drop-
// down, a folded group, a collapsed ribbon's body, the keytips. A host calls it
// when a click lands outside the ribbon and its popup (the ribbon cannot see
// that click), and the ribbon calls it itself after a command runs.
func (r *Ribbon) Dismiss() {
	r.pop, r.peek, r.keytips, r.tipBuf, r.hoverMs = nil, false, 0, "", 0
}

// HitTest is true over the ribbon and over what hangs from it — an open popup or
// a collapsed ribbon's peeking body — so a container routes clicks there.
func (r *Ribbon) HitTest(px, py int) bool {
	if r.Bounds().Contains(px, py) {
		return true
	}
	if r.pop != nil {
		panel, _ := r.popupGeometry(r.pop, r.metrics())
		return panel.Contains(px, py)
	}
	return r.peek && r.bodyRect().Contains(px, py)
}

// Tick advances the hover timer by dtMs milliseconds and reports whether the
// tooltip's visibility changed, i.e. whether the host should redraw. A host with
// a frame loop calls it every frame; without one, tooltips never show.
func (r *Ribbon) Tick(dtMs int) bool {
	if r.hoverCtl < 0 || r.pop != nil {
		return false
	}
	was := r.tooltipVisible()
	r.hoverMs += dtMs
	return was != r.tooltipVisible()
}

// tooltipVisible is whether the hovered control's tooltip is showing.
func (r *Ribbon) tooltipVisible() bool {
	d := r.TooltipDelayMs
	if d <= 0 {
		d = 500
	}
	return r.hoverCtl >= 0 && r.pop == nil && r.hoverMs >= d
}

// OnEvent drives the ribbon: the pointer (a move hovers, a click presses) and the
// keyboard. Coordinates are widget-local, as everywhere in the toolkit.
func (r *Ribbon) OnEvent(ev Event) {
	if r.Disabled().Get() {
		return
	}
	b := r.Bounds()
	x, y := b.X+ev.X, b.Y+ev.Y
	switch ev.Kind {
	case EventMouseMove:
		r.hover(x, y)
	case EventClick:
		r.click(x, y)
	case EventKeyDown:
		r.key(ev)
	}
}

// hover tracks what the pointer is over, restarting the tooltip timer when the
// control changes.
func (r *Ribbon) hover(x, y int) {
	m := r.metrics()
	tabs, chev := r.tabRects(m)
	r.hoverTab = -1
	for i, t := range tabs {
		if t.Contains(x, y) {
			r.hoverTab = i
		}
	}
	r.chevHot = chev.Contains(x, y)
	if r.pop != nil && r.pop.kind != popGroup {
		_, rows := r.popupGeometry(r.pop, m)
		for i, rc := range rows {
			if rc.Contains(x, y) {
				r.pop.focus = i
			}
		}
	}
	if idx := r.cellAt(x, y); idx != r.hoverCtl {
		r.hoverCtl, r.hoverMs = idx, 0
	}
}

// click presses whatever is at the surface point (x, y).
func (r *Ribbon) click(x, y int) {
	r.keytips, r.tipBuf = 0, ""
	if r.pop != nil && r.popupClick(x, y) {
		return
	}
	m := r.metrics()
	tabs, chev := r.tabRects(m)
	if chev.Contains(x, y) {
		r.toggleCollapsed()
		return
	}
	for i, t := range tabs {
		if t.Contains(x, y) {
			r.pickTab(i)
			return
		}
	}
	if idx := r.cellAt(x, y); idx >= 0 {
		r.zone, r.focusCtl = ribbonZoneBody, idx
		r.clickCell(idx, x, y)
		return
	}
	if r.peek && !r.bodyRect().Contains(x, y) {
		r.peek = false
	}
}

// popupClick handles a click while a popup is open and reports whether it was
// consumed: a row chooses it, the panel's border swallows the click, and a click
// anywhere else closes the popup and goes on to what it landed on. A folded
// group's controls are ordinary cells, handled by the caller.
func (r *Ribbon) popupClick(x, y int) bool {
	pp := r.pop
	panel, rows := r.popupGeometry(pp, r.metrics())
	if pp.kind == popGroup {
		if !panel.Contains(x, y) {
			r.pop = nil
		}
		return false
	}
	for i, rc := range rows {
		if rc.Contains(x, y) {
			r.choose(pp, i)
			return true
		}
	}
	if panel.Contains(x, y) {
		return true
	}
	r.pop = nil
	return false
}

// choose picks row i of a menu, list or gallery popup. A disabled entry is
// refused and the popup stays open.
func (r *Ribbon) choose(pp *ribbonPopup, i int) {
	switch pp.kind {
	case popMenu:
		it := pp.ctrl.Items[i]
		if !it.enabled() {
			return
		}
		if it.Action != nil {
			it.Action.Execute()
		}
	case popList:
		pp.ctrl.selection().Set(i)
	case popGallery:
		r.pickGallery(pp.ctrl, i)
		return
	}
	r.Dismiss()
}

// pickGallery selects cell i of a gallery and runs its command, if it has one.
func (r *Ribbon) pickGallery(c *RibbonControl, i int) {
	it := c.Items[i]
	if !it.enabled() {
		return
	}
	c.selection().Set(i)
	if it.Action != nil {
		it.Action.Execute()
	}
	r.Dismiss()
}

// pickTab shows tab i. On a collapsed ribbon that peeks at its body (and a
// second click on the same tab puts it away again).
func (r *Ribbon) pickTab(i int) {
	same := i == r.activeIndex()
	r.ActiveTab().Set(i)
	r.focusTab, r.zone, r.pop = i, ribbonZoneTabs, nil
	r.peek = r.Collapsed().Get() && !(same && r.peek)
}

// toggleCollapsed collapses an expanded ribbon and expands a collapsed one.
func (r *Ribbon) toggleCollapsed() {
	r.Collapsed().Set(!r.Collapsed().Get())
	r.pop, r.peek = nil, false
}

// clickCell presses cell idx at the surface point (x, y): a gallery cell picks
// it, an arrow opens the menu, anything else is the control's main action.
func (r *Ribbon) clickCell(idx, x, y int) {
	cell := r.cells()[idx]
	part := partMain
	for i, rc := range cell.cells {
		if rc.Contains(x, y) {
			part = i
		}
	}
	if part == partMain && cell.arrow != (Rect{}) && cell.arrow.Contains(x, y) {
		part = partArrow
	}
	r.activate(idx, part)
}

// activate runs control idx: its main action, or its menu when part is the arrow,
// or gallery cell part. Drop-downs and galleries open their popup whatever the
// part, folded groups open themselves.
func (r *Ribbon) activate(idx, part int) {
	cells := r.cells()
	if idx < 0 || idx >= len(cells) {
		return
	}
	cell := cells[idx]
	if cell.form == formPopup {
		r.openPopup(&ribbonPopup{kind: popGroup, grp: cell.grp, anchor: cell.rect})
		return
	}
	c := cell.ctrl
	if !c.enabled() {
		return
	}
	switch c.Kind {
	case RibbonToggle:
		c.check().Set(!c.check().Get())
		r.runAction(c.Action)
	case RibbonDropdown:
		r.openPopup(&ribbonPopup{kind: popList, ctrl: c, anchor: cell.rect, focus: c.selection().Get()})
	case RibbonGallery:
		if part >= 0 && part < len(c.Items) {
			r.pickGallery(c, part)
			return
		}
		r.openPopup(&ribbonPopup{kind: popGallery, ctrl: c, anchor: cell.rect, focus: c.selection().Get()})
	case RibbonSplit:
		if part == partArrow {
			r.openPopup(&ribbonPopup{kind: popMenu, ctrl: c, anchor: cell.rect})
			return
		}
		r.runAction(c.Action)
	default:
		r.runAction(c.Action)
	}
}

// openPopup shows pp; a folded group's panel takes the keyboard into its body.
func (r *Ribbon) openPopup(pp *ribbonPopup) {
	r.pop, r.hoverMs = pp, 0
	if pp.kind == popGroup {
		r.zone, r.focusCtl = ribbonZoneBody, 0
	}
}

// runAction runs a control's command, then closes whatever was open over the
// content: a command is the end of a gesture.
func (r *Ribbon) runAction(a *Action) {
	if a != nil {
		a.Execute()
	}
	r.Dismiss()
}

// key routes a key press.
func (r *Ribbon) key(ev Event) {
	switch {
	case ev.Ctrl && ev.Code == "F1":
		r.toggleCollapsed()
	case ev.Code == "Alt" && r.keytips > 0:
		r.keytips, r.tipBuf = 0, ""
	case ev.Code == "Alt":
		r.startKeyTips()
	case r.keytips > 0:
		r.keyTipKey(ev)
	case r.pop != nil && r.pop.kind != popGroup:
		r.popupKey(ev)
	case r.zone == ribbonZoneTabs:
		r.tabKey(ev)
	default:
		r.bodyKey(ev)
	}
}

// tabKey is the tablist: Left/Right (wrapping) move and show a tab at once
// ("automatic activation", which the WAI-ARIA practices recommend when the
// panel shows without delay), Home/End jump, and Down, Enter or Space go into
// the body (peeking at it first when the ribbon is collapsed).
func (r *Ribbon) tabKey(ev Event) {
	n := len(r.Tabs)
	if n == 0 {
		return
	}
	switch ev.Code {
	case "ArrowLeft":
		r.moveTab((r.focusTab + n - 1) % n)
	case "ArrowRight":
		r.moveTab((r.focusTab + 1) % n)
	case "Home":
		r.moveTab(0)
	case "End":
		r.moveTab(n - 1)
	case "ArrowDown", "Enter", " ", "Space":
		r.enterBody()
	case "Escape":
		r.peek = false
	}
}

// moveTab focuses and shows tab i.
func (r *Ribbon) moveTab(i int) {
	r.focusTab = i
	r.ActiveTab().Set(i)
	r.peek = false
}

// enterBody moves the focus from the tabs to the first usable control, peeking
// at the body of a collapsed ribbon.
func (r *Ribbon) enterBody() {
	if r.Collapsed().Get() {
		r.peek = true
	}
	cells := r.cells()
	for i, c := range cells {
		if c.usable() {
			r.zone, r.focusCtl = ribbonZoneBody, i
			return
		}
	}
}

// usable is whether keyboard focus can rest on the cell: a folded group's button
// always, a control when it is enabled.
func (c ribbonCell) usable() bool { return c.ctrl == nil || c.ctrl.enabled() }

// bodyKey is the toolbar: Left/Right (wrapping) between controls, Up/Down inside
// a column, Home/End, Enter or Space to activate, Alt+Down (or Down on a control
// with nothing below it) to open a menu, Escape back to the tabs.
func (r *Ribbon) bodyKey(ev Event) {
	cells := r.cells()
	if len(cells) == 0 {
		r.zone = ribbonZoneTabs
		return
	}
	cur := min(max(r.focusCtl, 0), len(cells)-1)
	switch ev.Code {
	case "ArrowRight":
		r.focusCtl = stepUsable(cells, cur, 1)
	case "ArrowLeft":
		r.focusCtl = stepUsable(cells, cur, -1)
	case "Home":
		r.focusCtl = stepUsable(cells, len(cells)-1, 1)
	case "End":
		r.focusCtl = stepUsable(cells, 0, -1)
	case "ArrowUp":
		if n := verticalNeighbour(cells, cur, -1); n >= 0 {
			r.focusCtl = n
		} else {
			r.zone = ribbonZoneTabs
		}
	case "ArrowDown":
		if n := verticalNeighbour(cells, cur, 1); n >= 0 && !ev.Alt {
			r.focusCtl = n
		} else {
			r.openFocused(cells, cur)
		}
	case "Enter", " ", "Space":
		r.activate(cur, partMain)
	case "Escape":
		r.escapeBody()
	}
}

// escapeBody leaves the body for the tabs, closing a folded group's panel or a
// collapsed ribbon's peek.
func (r *Ribbon) escapeBody() {
	if r.pop != nil {
		r.pop = nil
		return
	}
	r.zone, r.peek = ribbonZoneTabs, false
}

// openFocused opens the menu, list or gallery of the focused control, when it
// has one.
func (r *Ribbon) openFocused(cells []ribbonCell, cur int) {
	c := cells[cur].ctrl
	switch {
	case c == nil || !c.enabled():
	case c.Kind == RibbonSplit:
		r.activate(cur, partArrow)
	case c.Kind == RibbonDropdown || c.Kind == RibbonGallery:
		r.activate(cur, partMain)
	}
}

// stepUsable is the next usable cell after from, going dir (+1 or -1) and
// wrapping; from itself when none else is usable.
func stepUsable(cells []ribbonCell, from, dir int) int {
	n := len(cells)
	for i := 1; i <= n; i++ {
		j := ((from+dir*i)%n + n) % n
		if cells[j].usable() {
			return j
		}
	}
	return from
}

// verticalNeighbour is the usable cell nearest above (dir -1) or below (dir +1)
// the cell cur in its column, -1 when there is none.
func verticalNeighbour(cells []ribbonCell, cur, dir int) int {
	best, bestD := -1, 1<<30
	c := cells[cur].rect
	for i, o := range cells {
		if i == cur || !o.usable() || o.rect.X >= c.X+c.W || o.rect.X+o.rect.W <= c.X {
			continue
		}
		if d := (o.rect.Y - c.Y) * dir; d > 0 && d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// popupKey drives an open menu, list or gallery: arrows move the highlight
// (a gallery grid also moves sideways), Home/End jump, Enter or Space choose,
// Escape closes it, leaving the focus on the control that opened it.
func (r *Ribbon) popupKey(ev Event) {
	pp := r.pop
	n := pp.popupLen()
	step := 1
	if pp.kind == popGallery {
		step = min(galleryCols, n)
	}
	switch ev.Code {
	case "ArrowDown":
		pp.focus = min(pp.focus+step, n-1)
	case "ArrowUp":
		pp.focus = max(pp.focus-step, 0)
	case "ArrowRight":
		if pp.kind == popGallery {
			pp.focus = min(pp.focus+1, n-1)
		}
	case "ArrowLeft":
		if pp.kind == popGallery {
			pp.focus = max(pp.focus-1, 0)
		}
	case "Home":
		pp.focus = 0
	case "End":
		pp.focus = n - 1
	case "Enter", " ", "Space":
		if n > 0 {
			r.choose(pp, pp.focus)
		}
	case "Escape":
		r.pop = nil
	}
}

// startKeyTips shows a keytip over each tab, the way a press of Alt does in
// Office.
func (r *Ribbon) startKeyTips() {
	r.keytips, r.tipBuf = 1, ""
	r.zone, r.focusTab, r.pop = ribbonZoneTabs, r.activeIndex(), nil
}

// keyTipKey handles a key while keytips show: a letter or digit narrows the tip
// being typed (a wrong one starts it over), Escape goes back one level.
func (r *Ribbon) keyTipKey(ev Event) {
	if ev.Code == "Escape" {
		if r.keytips == 2 {
			r.keytips, r.peek = 1, false
		} else {
			r.keytips = 0
		}
		r.tipBuf = ""
		return
	}
	if len(ev.Code) != 1 {
		return
	}
	buf := r.tipBuf + strings.ToUpper(ev.Code)
	var tips []string
	if r.keytips == 1 {
		tips = r.tabTips()
	} else {
		tips = r.cellTips(r.cells())
	}
	exact, prefix := matchTip(tips, buf)
	switch {
	case exact >= 0 && r.keytips == 1:
		r.moveTab(exact)
		r.keytips, r.tipBuf = 2, ""
		r.peek = r.Collapsed().Get()
	case exact >= 0:
		r.keytips, r.tipBuf = 0, ""
		r.zone, r.focusCtl = ribbonZoneBody, exact
		r.activate(exact, partMain)
	case prefix:
		r.tipBuf = buf
	default:
		r.tipBuf = ""
	}
}

// matchTip finds buf among tips: the index of an exact match (-1 for none), and
// whether buf is the beginning of a longer tip.
func matchTip(tips []string, buf string) (exact int, prefix bool) {
	exact = -1
	for i, t := range tips {
		switch {
		case t == buf:
			exact = i
		case t != "" && strings.HasPrefix(t, buf):
			prefix = true
		}
	}
	return exact, prefix && exact < 0
}

// tabTips are the keytips of the tabs: the explicit ones, else the first free
// letter of each label.
func (r *Ribbon) tabTips() []string {
	explicit := make([]string, len(r.Tabs))
	labels := make([]string, len(r.Tabs))
	for i, t := range r.Tabs {
		explicit[i], labels[i] = t.KeyTip, t.Label
	}
	return assignTips(explicit, labels, "", "")
}

// cellTips are the keytips of the shown controls; a folded group's button gets a
// two-letter tip starting with Z, as the Windows guidelines have it, and no other
// control starts with Z.
func (r *Ribbon) cellTips(cells []ribbonCell) []string {
	var nIdx, fIdx []int
	var nExp, nLab, fLab []string
	for i, c := range cells {
		if c.form == formPopup {
			fIdx, fLab = append(fIdx, i), append(fLab, c.grp.Label)
		} else {
			nIdx, nExp, nLab = append(nIdx, i), append(nExp, c.ctrl.KeyTip), append(nLab, c.ctrl.label())
		}
	}
	out := make([]string, len(cells))
	for k, tip := range assignTips(nExp, nLab, "", "Z") {
		out[nIdx[k]] = tip
	}
	for k, tip := range assignTips(make([]string, len(fLab)), fLab, "Z", "") {
		out[fIdx[k]] = tip
	}
	return out
}

// assignTips gives each label a keytip: its explicit one, else prefix plus the
// first letter of the label (then of the digits 1-9) not yet taken and not in
// skip. A label with nothing left gets no tip.
func assignTips(explicit, labels []string, prefix, skip string) []string {
	out := make([]string, len(labels))
	used := map[string]bool{}
	for i, e := range explicit {
		if e != "" {
			out[i] = strings.ToUpper(e)
			used[out[i]] = true
		}
	}
	for i, l := range labels {
		if out[i] != "" {
			continue
		}
		for _, ch := range strings.ToUpper(l) + "123456789" {
			t := prefix + string(ch)
			isKey := ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
			if isKey && !used[t] && !strings.ContainsRune(skip, ch) {
				out[i], used[t] = t, true
				break
			}
		}
	}
	return out
}
