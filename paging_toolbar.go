// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"

	"github.com/go-widgets/painter"
)

// PagingToolbar is a record-navigation toolbar: First (|<), Prev (<), a
// "Page N of M" indicator, Next (>), Last (>|) and an optional Refresh
// button. Navigation clamps to [1, PageCount] and fires OnChange only when the
// page actually changes; the extreme buttons render in a disabled tone and
// swallow clicks at the ends of the range. Unlike Pagination (a strip of
// numbered page buttons), this is the compact toolbar Ext's PagingToolbar
// provides for a data grid's footer.
type PagingToolbar struct {
	Base
	// Page is the 1-based current page.
	Page int
	// PageCount is the total number of pages.
	PageCount int
	// ShowRefresh adds a trailing reload button that fires OnRefresh.
	ShowRefresh bool
	// OnChange fires with the new page whenever navigation changes Page.
	OnChange func(page int)
	// OnRefresh fires when the Refresh button is clicked. Nil is safe.
	OnRefresh func()
}

// PagingBtnW is the pixel width of each toolbar button.
const PagingBtnW = 26

// PagingBtnH is the pixel height of the toolbar (and each button).
const PagingBtnH = 24

// PagingGap is the horizontal pixel gap between successive buttons.
const PagingGap = 2

// pagingInfoPad is the horizontal padding added around the "Page N of M"
// indicator text to size its slot.
const pagingInfoPad = 8

// NewPagingToolbar builds a toolbar at the given page/count. page is clamped
// into [1, count] and count to a floor of 1 (an empty grid still reads as
// "Page 1 of 1" with every nav button disabled).
func NewPagingToolbar(page, count int) *PagingToolbar {
	if count < 1 {
		count = 1
	}
	if page < 1 {
		page = 1
	}
	if page > count {
		page = count
	}
	return &PagingToolbar{Page: page, PageCount: count}
}

// info is the indicator text, "Page N of M".
func (pt *PagingToolbar) info() string {
	return "Page " + strconv.Itoa(pt.Page) + " of " + strconv.Itoa(pt.PageCount)
}

// pagingLayout holds the on-surface rectangles of every toolbar element, so
// Draw and OnEvent resolve positions through one shared computation.
type pagingLayout struct {
	first, prev, info, next, last, refresh Rect
	hasRefresh                             bool
}

// layout computes the element rectangles left-to-right from the widget origin.
func (pt *PagingToolbar) layout() pagingLayout {
	r := pt.Bounds()
	x, y := r.X, r.Y
	btn := func() Rect {
		rc := Rect{X: x, Y: y, W: PagingBtnW, H: PagingBtnH}
		x += PagingBtnW + PagingGap
		return rc
	}
	var l pagingLayout
	l.first = btn()
	l.prev = btn()
	iw := pt.textWidth(pt.info()) + 2*pagingInfoPad
	l.info = Rect{X: x, Y: y, W: iw, H: PagingBtnH}
	x += iw + PagingGap
	l.next = btn()
	l.last = btn()
	if pt.ShowRefresh {
		x += PagingGap // extra separation before the refresh button
		l.refresh = btn()
		l.hasRefresh = true
	}
	return l
}

// Draw paints the toolbar body, buttons (extreme ones dimmed at the range
// ends), the "Page N of M" indicator and the optional Refresh button.
func (pt *PagingToolbar) Draw(p painter.Painter, theme *Theme) {
	r := pt.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	l := pt.layout()
	atStart := pt.Page > 1
	atEnd := pt.Page < pt.PageCount
	pt.drawButton(p, theme, l.first, "|<", atStart)
	pt.drawButton(p, theme, l.prev, "<", atStart)

	info := pt.info()
	tx := l.info.X + (l.info.W-pt.textWidth(info))/2
	ty := l.info.Y + (l.info.H-pt.glyphHeight())/2
	pt.drawText(p, tx, ty, info, theme.OnSurface)

	pt.drawButton(p, theme, l.next, ">", atEnd)
	pt.drawButton(p, theme, l.last, ">|", atEnd)
	if l.hasRefresh {
		fillRect(p, l.refresh.X, l.refresh.Y, l.refresh.W, l.refresh.H, theme.SurfaceAlt)
		strokeRect(p, l.refresh.X, l.refresh.Y, l.refresh.W, l.refresh.H, theme.Border)
		drawReloadIcon(p, l.refresh, theme.OnSurface)
	}
}

// drawReloadIcon paints a circular "reload" glyph centred in rc: most of a
// ring with a gap at the top, plus a small arrowhead at the gap so it reads as
// a refresh rather than a plain circle.
func drawReloadIcon(p painter.Painter, rc Rect, ink RGBA) {
	cx := float64(rc.X) + float64(rc.W-1)/2
	cy := float64(rc.Y) + float64(rc.H-1)/2
	radius := float64(rc.W)
	if rc.H < rc.W {
		radius = float64(rc.H)
	}
	radius = radius/2 - 4
	// Ring, leaving a gap at the top (near -90°, i.e. 270°) for the arrowhead.
	for deg := -60; deg <= 210; deg += 6 {
		a := float64(deg) * math.Pi / 180
		fillRect(p, int(cx+radius*math.Cos(a)), int(cy+radius*math.Sin(a)), 1, 1, ink)
	}
	// Arrowhead at the ring's leading end (top, deg == -60): a small triangle
	// pointing up-and-left so the ring reads as turning clockwise.
	ax := int(cx + radius*math.Cos(-60*math.Pi/180))
	ay := int(cy + radius*math.Sin(-60*math.Pi/180))
	for i := 0; i < 4; i++ {
		fillRect(p, ax-i, ay, 1, 1, ink)   // horizontal leg (leftward)
		fillRect(p, ax, ay-i, 1, 1, ink)   // vertical leg (upward)
	}
}

// drawButton paints one nav button; enabled=false renders the label in the
// disabled (Border) tone.
func (pt *PagingToolbar) drawButton(p painter.Painter, theme *Theme, rc Rect, label string, enabled bool) {
	fillRect(p, rc.X, rc.Y, rc.W, rc.H, theme.SurfaceAlt)
	strokeRect(p, rc.X, rc.Y, rc.W, rc.H, theme.Border)
	ink := theme.OnSurface
	if !enabled {
		ink = theme.Border
	}
	tx := rc.X + (rc.W-pt.textWidth(label))/2
	ty := rc.Y + (rc.H-pt.glyphHeight())/2
	pt.drawText(p, tx, ty, label, ink)
}

// OnEvent routes an EventClick to whichever element contains it: First/Last
// jump to the ends, Prev/Next step by one (all clamped, firing OnChange only
// on an actual change), Refresh fires OnRefresh. The indicator + gaps are
// inert.
func (pt *PagingToolbar) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := pt.Bounds()
	sx, sy := ev.X+r.X, ev.Y+r.Y // events are widget-local; lift to surface coords
	l := pt.layout()
	switch {
	case l.first.Contains(sx, sy):
		pt.goTo(1)
	case l.prev.Contains(sx, sy):
		pt.goTo(pt.Page - 1)
	case l.next.Contains(sx, sy):
		pt.goTo(pt.Page + 1)
	case l.last.Contains(sx, sy):
		pt.goTo(pt.PageCount)
	case l.hasRefresh && l.refresh.Contains(sx, sy):
		if pt.OnRefresh != nil {
			pt.OnRefresh()
		}
	}
}

// goTo clamps page into [1, PageCount] and, when it differs from the current
// Page, updates Page and fires OnChange.
func (pt *PagingToolbar) goTo(page int) {
	if page > pt.PageCount {
		page = pt.PageCount
	}
	if page < 1 {
		page = 1
	}
	if page == pt.Page {
		return
	}
	pt.Page = page
	if pt.OnChange != nil {
		pt.OnChange(pt.Page)
	}
}
