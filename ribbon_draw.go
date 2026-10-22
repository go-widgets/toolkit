// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ribbonMinContrast is the contrast the ribbon's boundaries and its focus ring
// keep against the surfaces they sit on (WCAG 2.2, success criterion 1.4.11).
const ribbonMinContrast = 3.0

// ribbonLine is the colour of every boundary the ribbon draws — the tab outline,
// the group dividers, the edge of a drop-down box and of a popup: the theme's own
// border when it already reaches 3:1 against both the page and the ribbon's
// body, else pulled toward the text colour until it does ([PopupBorder]).
func ribbonLine(theme *Theme) RGBA { return PopupBorder(theme) }

// ribbonAccent is the accent as the ribbon uses it for things that must be seen
// — the focus ring, a toggle's outline, the chosen item — pulled toward the text
// colour until it reaches 3:1 against both the tab strip and the body.
func ribbonAccent(theme *Theme) RGBA {
	c := ensureContrast(theme.Accent, theme.Surface, theme.OnSurface, ribbonMinContrast)
	return ensureContrast(c, theme.Background, theme.OnSurface, ribbonMinContrast)
}

// ribbonInk is the ink of a control: the text colour, muted when disabled.
func ribbonInk(theme *Theme, enabled bool) RGBA {
	if enabled {
		return theme.OnSurface
	}
	return mutedInk(theme)
}

// Draw paints the tab strip and, unless the ribbon is collapsed, the body of the
// active tab. Menus, a collapsed ribbon's body, tooltips and keytips are drawn by
// [Ribbon.DrawPopover], which a [PopoverHost] calls after the tree.
func (r *Ribbon) Draw(p painter.Painter, theme *Theme) {
	b := r.Bounds()
	m := r.metrics()
	line := ribbonLine(theme)
	collapsed := r.Collapsed().Get()
	fillRect(p, b.X, b.Y, b.W, m.tabH, theme.Background)
	fillRect(p, b.X, b.Y+m.tabH, b.W, 1, line)
	if !collapsed {
		r.drawBody(p, theme, m, r.body())
	}
	tabs, chev := r.tabRects(m)
	act := r.activeIndex()
	for i, t := range r.Tabs {
		r.drawTab(p, theme, m, t, tabs[i], line, i == act && !collapsed, i == r.hoverTab)
	}
	r.drawCollapse(p, theme, chev, collapsed)
	if r.Focused() && r.zone == ribbonZoneTabs && r.keytips == 0 && len(tabs) > 0 {
		f := tabs[min(r.focusTab, len(tabs)-1)]
		strokeRect(p, f.X+1, f.Y+1, f.W-2, f.H-2, ribbonAccent(theme))
	}
}

// drawTab paints one tab label; the active tab of an expanded ribbon is joined to
// the body below it and outlined on the other three sides.
func (r *Ribbon) drawTab(p painter.Painter, theme *Theme, m ribbonMetrics, t *RibbonTab, rc Rect, line RGBA, active, hot bool) {
	ink := theme.OnSurface
	if active {
		fillRect(p, rc.X, rc.Y, rc.W, rc.H+1, theme.Surface)
		fillRect(p, rc.X, rc.Y, rc.W, 1, line)
		fillRect(p, rc.X, rc.Y, 1, rc.H, line)
		fillRect(p, rc.X+rc.W-1, rc.Y, 1, rc.H, line)
	} else if hot {
		fillRoundRect(p, rc.X, rc.Y+scaled(2), rc.W, rc.H-scaled(2), scaled(3), theme.SurfaceAlt)
	}
	r.drawText(p, rc.X+(rc.W-r.textWidth(t.Label))/2, rc.Y+(rc.H-m.gh)/2, t.Label, ink)
}

// drawCollapse paints the button at the end of the tab strip that collapses and
// expands the ribbon: an up chevron when expanded, a down one when collapsed.
func (r *Ribbon) drawCollapse(p painter.Painter, theme *Theme, rc Rect, collapsed bool) {
	if r.chevHot {
		fillRoundRect(p, rc.X, rc.Y+scaled(2), rc.W, rc.H-scaled(2), scaled(3), theme.SurfaceAlt)
	}
	cx, cy := rc.X+rc.W/2, rc.Y+rc.H/2
	if collapsed {
		drawDropArrow(p, cx, cy, theme.OnSurface)
		return
	}
	s := max(1, scaled(1))
	for i := 0; i < 3; i++ {
		fillRect(p, cx-i*s, cy+(i-1)*s, (1+2*i)*s, s, theme.OnSurface)
	}
}

// drawDropArrow paints the small down-pointing triangle of a menu or drop-down,
// centred on (cx, cy).
func drawDropArrow(p painter.Painter, cx, cy int, ink RGBA) {
	s := max(1, scaled(1))
	for i := 0; i < 3; i++ {
		fillRect(p, cx-(2-i)*s, cy+(i-1)*s, (1+2*(2-i))*s, s, ink)
	}
}

// drawBody paints a laid-out body: the surface, then each group (its divider and
// label) and each control.
func (r *Ribbon) drawBody(p painter.Painter, theme *Theme, m ribbonMetrics, body ribbonBody) {
	b := r.Bounds()
	line := ribbonLine(theme)
	top := b.Y + m.tabH + 1
	fillRect(p, b.X, top, b.W, m.bodyH+m.labelH, theme.Surface)
	fillRect(p, b.X, top+m.bodyH+m.labelH, b.W, 1, line)
	labelInk := blendRGBA(theme.OnSurface, theme.Surface, 0.75)
	for _, g := range body.groups {
		if g.rect.X+g.rect.W+m.sepW <= b.X+b.W {
			fillRect(p, g.rect.X+g.rect.W, top+scaled(4), m.sepW, m.bodyH+m.labelH-scaled(8), line)
		}
		lw := r.textWidth(g.group.Label)
		r.drawText(p, g.rect.X+(g.rect.W-lw)/2, top+m.bodyH+(m.labelH-m.gh)/2, g.group.Label, labelInk)
	}
	for i, c := range body.cells {
		r.drawCell(p, theme, m, i, c)
	}
}

// drawCell paints one control in its form.
func (r *Ribbon) drawCell(p painter.Painter, theme *Theme, m ribbonMetrics, idx int, c ribbonCell) {
	rc := c.rect
	enabled := c.usable()
	ink := ribbonInk(theme, enabled)
	checked := c.ctrl != nil && c.ctrl.Kind == RibbonToggle && c.ctrl.check().Get()
	open := r.pop != nil && r.pop.anchor == rc
	hot := enabled && (idx == r.hoverCtl || open)
	switch {
	case checked:
		fillRoundRect(p, rc.X, rc.Y, rc.W, rc.H, scaled(3), blendRGBA(theme.Accent, theme.Surface, 0.25))
		strokeRoundRect(p, rc.X, rc.Y, rc.W, rc.H, scaled(3), ribbonAccent(theme))
	case hot:
		fillRoundRect(p, rc.X, rc.Y, rc.W, rc.H, scaled(3), theme.SurfaceAlt)
	}
	switch c.form {
	case formLarge:
		r.drawLarge(p, m, rc, c.ctrl.icon(), largeLines(c.ctrl.label()), c.ctrl.hasArrow(), ink)
	case formPopup:
		r.drawLarge(p, m, rc, c.grp.Icon, largeLines(c.grp.Label), true, ink)
	case formDropdown:
		r.drawDropdown(p, theme, m, rc, c.ctrl, ink)
	case formGallery:
		r.drawGallery(p, theme, m, c, ink)
	default:
		r.drawSmall(p, m, c, ink)
	}
	if r.Focused() && r.zone == ribbonZoneBody && idx == r.focusCtl && r.keytips == 0 {
		strokeRoundRect(p, rc.X, rc.Y, rc.W, rc.H, scaled(3), ribbonAccent(theme))
	}
}

// drawIcon paints an icon, if there is one, into rc.
func drawIcon(p painter.Painter, icon IconFunc, rc Rect, ink RGBA) {
	if icon != nil {
		icon(p, rc, ink)
	}
}

// drawLarge paints the large form: the icon above one or two lines of text, and
// for a control with a menu the arrow at the end of the last line (or on a line
// of its own under a one-line label).
func (r *Ribbon) drawLarge(p painter.Painter, m ribbonMetrics, rc Rect, icon IconFunc, lines []string, arrow bool, ink RGBA) {
	y := rc.Y + scaled(3)
	drawIcon(p, icon, Rect{X: rc.X + (rc.W-m.iconL)/2, Y: y, W: m.iconL, H: m.iconL}, ink)
	y += m.iconL + scaled(3)
	for i, ln := range lines {
		w := r.textWidth(ln)
		x := rc.X + (rc.W-w)/2
		if arrow && i == len(lines)-1 && len(lines) > 1 {
			x = rc.X + (rc.W-w-m.gap-m.arrowW)/2
			drawDropArrow(p, x+w+m.gap+m.arrowW/2, y+m.gh/2, ink)
		}
		r.drawText(p, x, y, ln, ink)
		y += m.gh
	}
	if arrow && len(lines) == 1 {
		drawDropArrow(p, rc.X+rc.W/2, y+m.gh/2, ink)
	}
}

// drawSmall paints the small form (icon, text, arrow) or its icon-only variant.
func (r *Ribbon) drawSmall(p painter.Painter, m ribbonMetrics, c ribbonCell, ink RGBA) {
	rc := c.rect
	icon := c.ctrl.icon()
	ix := rc.X + m.padX
	if c.form == formIcon {
		w := rc.W
		if c.ctrl.hasArrow() {
			w -= m.arrowW
		}
		ix = rc.X + (w-m.iconS)/2
	}
	drawIcon(p, icon, Rect{X: ix, Y: rc.Y + (rc.H-m.iconS)/2, W: m.iconS, H: m.iconS}, ink)
	if c.form == formSmall {
		r.drawText(p, ix+m.iconS+m.gap, rc.Y+(rc.H-m.gh)/2, c.ctrl.label(), ink)
	}
	if c.ctrl.hasArrow() {
		drawDropArrow(p, rc.X+rc.W-m.padX-m.arrowW/2, rc.Y+rc.H/2, ink)
	}
}

// drawDropdown paints a drop-down's box: its outline, the chosen option (or its
// label while there is no choice) and the arrow.
func (r *Ribbon) drawDropdown(p painter.Painter, theme *Theme, m ribbonMetrics, rc Rect, c *RibbonControl, ink RGBA) {
	box := Rect{X: rc.X + scaled(1), Y: rc.Y + scaled(2), W: rc.W - 2*scaled(1), H: rc.H - 2*scaled(2)}
	fillRoundRect(p, box.X, box.Y, box.W, box.H, scaled(3), theme.Background)
	strokeRoundRect(p, box.X, box.Y, box.W, box.H, scaled(3), ribbonLine(theme))
	text := c.label()
	if i := c.selection().Get(); i >= 0 && i < len(c.Options) {
		text = c.Options[i]
	}
	r.drawText(p, box.X+m.padX, box.Y+(box.H-m.gh)/2, text, ink)
	drawDropArrow(p, box.X+box.W-m.padX-m.arrowW/2, box.Y+box.H/2, ink)
}

// drawGallery paints the in-ribbon strip: the first cells of the gallery with the
// chosen one outlined, then the arrow that opens the rest.
func (r *Ribbon) drawGallery(p painter.Painter, theme *Theme, m ribbonMetrics, c ribbonCell, ink RGBA) {
	sel := c.ctrl.selection().Get()
	for i, rc := range c.cells {
		it := c.ctrl.Items[i]
		drawIcon(p, it.icon(), Rect{X: rc.X + (rc.W-m.iconL)/2, Y: rc.Y + (rc.H-m.iconL)/2, W: m.iconL, H: m.iconL}, ink)
		if i == sel {
			strokeRoundRect(p, rc.X, rc.Y+scaled(1), rc.W, rc.H-2*scaled(1), scaled(3), ribbonAccent(theme))
		}
	}
	drawDropArrow(p, c.arrow.X+c.arrow.W/2, c.arrow.Y+c.arrow.H/2, ink)
}

// DrawPopover paints what hangs over the page: a collapsed ribbon's body while it
// peeks, the open menu, list, gallery or folded group, the tooltip of the hovered
// control and the keytips. A [PopoverHost] calls it after the whole tree is drawn
// (the ribbon is a [PopoverOwner]); a host without one calls it last itself.
func (r *Ribbon) DrawPopover(p painter.Painter, theme *Theme) {
	m := r.metrics()
	if r.Collapsed().Get() && r.peek {
		r.drawBody(p, theme, m, r.body())
	}
	if r.pop != nil {
		r.drawPopupPanel(p, theme, m)
	}
	if r.tooltipVisible() {
		r.drawTooltip(p, theme, m)
	}
	if r.keytips > 0 {
		r.drawKeyTips(p, theme, m)
	}
}

// drawPopupPanel paints the open popup: the panel, then its rows or its group.
func (r *Ribbon) drawPopupPanel(p painter.Painter, theme *Theme, m ribbonMetrics) {
	pp := r.pop
	panel, rows := r.popupGeometry(pp, m)
	fillRect(p, panel.X, panel.Y, panel.W, panel.H, theme.Surface)
	strokePopupBorder(p, panel, theme)
	if pp.kind == popGroup {
		for i, c := range r.groupPopupCells(pp, m) {
			r.drawCell(p, theme, m, i, c)
		}
		lw := r.textWidth(pp.grp.Label)
		r.drawText(p, panel.X+(panel.W-lw)/2, panel.Y+1+m.bodyH+(m.labelH-m.gh)/2, pp.grp.Label, blendRGBA(theme.OnSurface, theme.Surface, 0.75))
		return
	}
	accent := ribbonAccent(theme)
	for i, rc := range rows {
		if i == pp.focus {
			fillRect(p, rc.X, rc.Y, rc.W, rc.H, theme.SurfaceAlt)
		}
		switch pp.kind {
		case popGallery:
			it := pp.ctrl.Items[i]
			drawIcon(p, it.icon(), Rect{X: rc.X + (rc.W-m.iconL)/2, Y: rc.Y + m.gap, W: m.iconL, H: m.iconL}, ribbonInk(theme, it.enabled()))
			lw := r.textWidth(it.text())
			r.drawText(p, rc.X+(rc.W-lw)/2, rc.Y+m.gap+m.iconL+m.gap, it.text(), ribbonInk(theme, it.enabled()))
			if i == pp.ctrl.selection().Get() {
				strokeRect(p, rc.X, rc.Y, rc.W, rc.H, accent)
			}
		case popList:
			if i == pp.ctrl.selection().Get() {
				fillRect(p, rc.X, rc.Y, scaled(3), rc.H, accent)
			}
			r.drawText(p, rc.X+m.padX+m.iconS/2, rc.Y+(rc.H-m.gh)/2, pp.ctrl.Options[i], theme.OnSurface)
		default:
			it := pp.ctrl.Items[i]
			ink := ribbonInk(theme, it.enabled())
			drawIcon(p, it.icon(), Rect{X: rc.X + m.padX, Y: rc.Y + (rc.H-m.iconS)/2, W: m.iconS, H: m.iconS}, ink)
			r.drawText(p, rc.X+2*m.padX+m.iconS, rc.Y+(rc.H-m.gh)/2, it.text(), ink)
			if sc := r.shortcutFor(it.Action); sc != "" {
				r.drawText(p, rc.X+rc.W-m.padX-r.textWidth(sc), rc.Y+(rc.H-m.gh)/2, sc, mutedInk(theme))
			}
		}
	}
}

// tooltipLines is the text of the hovered control's tooltip: its name with its
// shortcut, then its description when it has one. The Windows guidelines ask for
// exactly this and for tooltips that "avoid simply restating the label".
func (r *Ribbon) tooltipLines() []string {
	cells := r.cells()
	if r.hoverCtl < 0 || r.hoverCtl >= len(cells) {
		return nil
	}
	c := cells[r.hoverCtl]
	if c.form == formPopup {
		return []string{c.grp.Label}
	}
	head := c.ctrl.label()
	if sc := r.shortcutFor(c.ctrl.Action); sc != "" {
		head += " (" + sc + ")"
	}
	if c.ctrl.Tip == "" {
		return []string{head}
	}
	return []string{head, c.ctrl.Tip}
}

// drawTooltip paints the tooltip bubble under the hovered control.
func (r *Ribbon) drawTooltip(p painter.Painter, theme *Theme, m ribbonMetrics) {
	lines := r.tooltipLines()
	if len(lines) == 0 {
		return
	}
	w := 0
	for _, l := range lines {
		w = max(w, r.textWidth(l))
	}
	anchor := r.cells()[r.hoverCtl].rect
	box := Rect{X: anchor.X, Y: anchor.Y + anchor.H + scaled(2), W: w + 2*m.padX, H: len(lines)*m.gh + 2*m.gap}
	fillRect(p, box.X, box.Y, box.W, box.H, theme.OnSurface)
	strokeRect(p, box.X, box.Y, box.W, box.H, ribbonLine(theme))
	for i, l := range lines {
		r.drawText(p, box.X+m.padX, box.Y+m.gap+i*m.gh, l, theme.Background)
	}
}

// drawKeyTips paints the keytip badges: a letter over each tab while choosing a
// tab, a tip over each control of the shown tab while choosing a control. Tips
// that no longer match what has been typed are left out.
func (r *Ribbon) drawKeyTips(p painter.Painter, theme *Theme, m ribbonMetrics) {
	if r.keytips == 1 {
		tabs, _ := r.tabRects(m)
		for i, tip := range r.tabTips() {
			r.drawBadge(p, theme, m, tip, tabs[i].X+tabs[i].W/2, tabs[i].Y+tabs[i].H-m.gh/2)
		}
		return
	}
	cells := r.cells()
	for i, tip := range r.cellTips(cells) {
		rc := cells[i].rect
		r.drawBadge(p, theme, m, tip, rc.X+rc.W/2, rc.Y+rc.H-m.gh)
	}
}

// drawBadge paints one keytip, centred on (cx, y), unless it is empty or does
// not start with what has been typed so far.
func (r *Ribbon) drawBadge(p painter.Painter, theme *Theme, m ribbonMetrics, tip string, cx, y int) {
	if tip == "" || len(tip) < len(r.tipBuf) || tip[:len(r.tipBuf)] != r.tipBuf {
		return
	}
	w := r.textWidth(tip) + 2*m.gap
	box := Rect{X: cx - w/2, Y: y, W: w, H: m.gh + m.gap}
	fillRoundRect(p, box.X, box.Y, box.W, box.H, scaled(3), theme.OnSurface)
	r.drawText(p, box.X+m.gap, box.Y+m.gap/2, tip, theme.Background)
}
