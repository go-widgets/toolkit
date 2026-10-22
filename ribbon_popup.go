// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// popupKind is what an open ribbon popup holds.
type popupKind int

const (
	// popMenu is a split button's menu of actions.
	popMenu popupKind = iota
	// popList is a drop-down's list of options.
	popList
	// popGallery is a gallery's grid of all its items.
	popGallery
	// popGroup is a group folded into a button, shown open.
	popGroup
)

// galleryCols is how many columns a gallery's drop-down grid has at most.
const galleryCols = 4

// ribbonPopup is the one popup the ribbon can have open: a menu, a list, a
// gallery grid or a folded group, hanging under the control that opened it.
type ribbonPopup struct {
	kind   popupKind
	ctrl   *RibbonControl
	grp    *RibbonGroup
	anchor Rect
	// focus is the row (or gallery cell) the keyboard or the pointer is on.
	focus int
}

// popupLen is the number of rows (or cells) a menu, list or gallery popup lists.
// (A folded group has no rows and is never asked.)
func (pp *ribbonPopup) popupLen() int {
	if pp.kind == popList {
		return len(pp.ctrl.Options)
	}
	return len(pp.ctrl.Items)
}

// text is the label of a menu entry or a gallery cell.
func (it RibbonItem) text() string {
	if it.Action != nil {
		return it.Action.Label
	}
	return it.Label
}

// icon is the glyph of a menu entry or a gallery cell, nil when it has none.
func (it RibbonItem) icon() IconFunc {
	if it.Icon != nil {
		return it.Icon
	}
	if it.Action != nil && it.Action.Icon != nil {
		return IconFunc(it.Action.Icon)
	}
	return nil
}

func (it RibbonItem) enabled() bool { return it.Action == nil || it.Action.IsEnabled() }

// popupGeometry is the panel of a popup and the rectangle of each of its rows
// (for a gallery, each cell). A folded group has no rows: its controls are laid
// out like any others, over the panel.
func (r *Ribbon) popupGeometry(pp *ribbonPopup, m ribbonMetrics) (panel Rect, rows []Rect) {
	top := pp.anchor.Y + pp.anchor.H
	switch pp.kind {
	case popGroup:
		return Rect{X: pp.anchor.X, Y: top, W: r.groupWidth(pp.grp, RibbonFull, m), H: m.bodyH + m.labelH + 2}, nil
	case popGallery:
		n := len(pp.ctrl.Items)
		cols := min(galleryCols, n)
		cw, ch := m.iconL+2*m.padX, m.iconL+m.gh+3*m.gap
		for _, it := range pp.ctrl.Items {
			cw = max(cw, r.textWidth(it.text())+2*m.padX)
		}
		panel = Rect{X: pp.anchor.X, Y: top, W: cols*cw + 2, H: ((n+cols-1)/cols)*ch + 2}
		for i := 0; i < n; i++ {
			rows = append(rows, Rect{X: panel.X + 1 + (i%cols)*cw, Y: panel.Y + 1 + (i/cols)*ch, W: cw, H: ch})
		}
		return panel, rows
	}
	w := pp.anchor.W
	var labels []string
	if pp.kind == popList {
		labels = pp.ctrl.Options
	} else {
		for _, it := range pp.ctrl.Items {
			labels = append(labels, it.text())
			sc := r.shortcutFor(it.Action)
			w = max(w, 4*m.padX+m.iconS+r.textWidth(it.text())+m.padX+r.textWidth(sc))
		}
	}
	for _, l := range labels {
		w = max(w, 3*m.padX+m.iconS+r.textWidth(l))
	}
	panel = Rect{X: pp.anchor.X, Y: top, W: w, H: len(labels)*m.rowH + 2}
	for i := range labels {
		rows = append(rows, Rect{X: panel.X + 1, Y: panel.Y + 1 + i*m.rowH, W: panel.W - 2, H: m.rowH})
	}
	return panel, rows
}

// shortcutFor is the accelerator text shown for an Action: the live binding in
// the ribbon's Keymap when it has one, else the Action's own default.
func (r *Ribbon) shortcutFor(a *Action) string {
	if a == nil {
		return ""
	}
	if r.Keymap != nil {
		if ch, ok := r.Keymap.ShortcutFor(a.ID); ok {
			return ch.String()
		}
	}
	return a.Shortcut.String()
}

// groupPopupCells are the controls of a folded group, laid out in full over its
// panel.
func (r *Ribbon) groupPopupCells(pp *ribbonPopup, m ribbonMetrics) []ribbonCell {
	panel, _ := r.popupGeometry(pp, m)
	return r.layoutGroup(pp.grp, RibbonFull, panel.X+m.groupPad, panel.Y+1, m)
}

// cells are the controls the ribbon currently shows and answers to: the active
// tab's body (expanded, or collapsed and peeking), or a folded group's panel
// while that is open.
func (r *Ribbon) cells() []ribbonCell {
	if r.pop != nil && r.pop.kind == popGroup {
		return r.groupPopupCells(r.pop, r.metrics())
	}
	if !r.showsBody() {
		return nil
	}
	return r.body().cells
}

// cellAt is the index of the cell under the surface point (x, y), -1 for none.
func (r *Ribbon) cellAt(x, y int) int {
	for i, c := range r.cells() {
		if c.rect.Contains(x, y) {
			return i
		}
	}
	return -1
}

// selection is a drop-down's or gallery's chosen index, created on first use.
func (c *RibbonControl) selection() *mvvm.Observable[int] {
	if c.Selected == nil {
		c.Selected = mvvm.NewObservable(0)
	}
	return c.Selected
}

// check is a toggle's state, created on first use.
func (c *RibbonControl) check() *mvvm.Observable[bool] {
	if c.Checked == nil {
		c.Checked = mvvm.NewObservable(false)
	}
	return c.Checked
}
