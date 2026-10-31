// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// A11y describes the ribbon as a toolbar named "Ribbon" whose value is the
// active tab, the way the WAI-ARIA toolbar pattern labels one. Its [Ribbon.Children]
// carry the tablist and the controls, with their rectangles, for [WalkA11y].
func (r *Ribbon) A11y() A11yInfo {
	v := ""
	if t := r.activeTab(); t != nil {
		v = t.Label
	}
	return A11yInfo{Role: RoleToolbar, Name: Tr("ribbon.a11y", "Ribbon"), Value: v}
}

// ribbonNode is a synthetic, non-drawing stand-in for one part of the ribbon
// (the tablist, a tab, a control), materialised on demand so the generic
// accessibility walk reports each with its own role, name, state and rectangle.
// It draws nothing and handles no events: the ribbon paints and drives itself.
type ribbonNode struct {
	Base
	info A11yInfo
	kids []Widget
}

func (n *ribbonNode) A11y() A11yInfo     { return n.info }
func (n *ribbonNode) Children() []Widget { return n.kids }

var (
	_ Accessible = (*Ribbon)(nil)
	_ Accessible = (*ribbonNode)(nil)
)

// Children exposes the ribbon to [WalkA11y] as a tablist containing one tab per
// tab (Value "selected" on the active one, as aria-selected), followed — when the
// body is shown — by one node per control of the active tab: a button, a
// checkbox for a toggle (Value "checked", as aria-pressed), and for a control
// that opens a menu Value "collapsed" or, while its menu is open, "expanded"
// (aria-expanded). Nodes are rebuilt from the current geometry on each call.
func (r *Ribbon) Children() []Widget {
	m := r.metrics()
	tabs, _ := r.tabRects(m)
	list := &ribbonNode{info: A11yInfo{Role: RoleTablist, Name: Tr("ribbon.a11y_tabs", "Ribbon tabs")}}
	act := r.activeIndex()
	for i, t := range r.Tabs {
		n := &ribbonNode{info: A11yInfo{Role: RoleTab, Name: t.Label, Value: stateValue(i == act, "selected")}}
		n.SetBounds(tabs[i])
		list.kids = append(list.kids, n)
	}
	list.SetBounds(Rect{X: r.Bounds().X, Y: r.Bounds().Y, W: r.Bounds().W, H: m.tabH})
	out := []Widget{list}
	for _, c := range r.cells() {
		n := &ribbonNode{info: r.cellA11y(c)}
		n.SetBounds(c.rect)
		out = append(out, n)
	}
	return out
}

// cellA11y is the description of one control.
func (r *Ribbon) cellA11y(c ribbonCell) A11yInfo {
	if c.form == formPopup {
		// While a folded group is open its controls take the button's place in
		// the tree, so the button is only ever reported collapsed.
		return A11yInfo{Role: RoleButton, Name: c.grp.Label, Value: "collapsed"}
	}
	info := A11yInfo{Role: RoleButton, Name: c.ctrl.label()}
	switch {
	case c.ctrl.Kind == RibbonToggle:
		info.Role = RoleCheckbox
		info.Value = checkedValue(c.ctrl.check().Get())
	case c.ctrl.hasArrow():
		info.Value = r.expandedValue(c)
	}
	return info
}

// expandedValue is "expanded" while the cell's popup is open, else "collapsed".
func (r *Ribbon) expandedValue(c ribbonCell) string {
	if r.pop != nil && r.pop.anchor == c.rect {
		return "expanded"
	}
	return "collapsed"
}
