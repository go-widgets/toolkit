// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"

	"github.com/go-widgets/painter"
)

// AppDock geometry, in logical pixels (each routed through the metric scale so
// the bar tracks the display like every other widget).
const (
	// AppDockItemW / AppDockItemH are one item's resting width and height.
	AppDockItemW = 120
	AppDockItemH = 28
	// AppDockGap is the spacing between adjacent items (and the end padding).
	AppDockGap = 4
	// AppDockGlyphPx is the icon's side length inside a resting item.
	AppDockGlyphPx = 18
	// AppDockPadX is the inset from an item's left edge to its glyph.
	AppDockPadX = 8
	// AppDockLabelGap is the gap between the glyph and the label.
	AppDockLabelGap = 8
	// appDockRadius rounds an item face; appDockRunDot is the running-dot size.
	appDockRadius = 6
	appDockRunDot = 4
)

// appDockDefaultScale / appDockDefaultRadius are the built-in magnification
// feel: a 1.6x peak with a 1.5-item falloff — a lively bulge, not cartoonish.
const (
	appDockDefaultScale  = 1.6
	appDockDefaultRadius = 1.5
)

// AppDockItem is one entry in an AppDock: a leaf icon, an optional label, and
// the state flags that drive its face, running dot and attention badge.
type AppDockItem struct {
	// Id is the caller's opaque identifier (echoed nowhere by the widget; handy
	// for the host's OnActivate switch).
	Id string
	// Label is drawn to the right of the icon; "" makes the item icon-only.
	Label string
	// Icon is the host-supplied leaf painter (the same seam as Browser's toolbar
	// icons): it is called with the glyph box and the item's ink. Nil draws no
	// glyph, leaving the label (or an empty face).
	Icon func(p painter.Painter, r Rect, ink RGBA)
	// Running raises the "app is open" dot under the glyph.
	Running bool
	// Active fills the item face in the accent (the "current app") instead of the
	// resting surface.
	Active bool
	// Badge, when > 0, overlays an attention count as a Badge at the top-right.
	Badge int
}

// AppDock is a horizontal launcher bar (a macOS-style application dock) with
// optional hover magnification: the item under the pointer, and its neighbours
// with a smooth raised-cosine falloff, swell and the row reflows so swollen
// items never overlap and the point under the cursor stays put.
//
// It is composed entirely from toolkit primitives — a Backdrop ground and, per
// item, a rounded Backdrop face, the host's icon painter, a clipped label, a
// running dot and an attention Badge — so it carries the toolkit look under any
// theme with no hand-drawn chrome. ItemRects publishes the live geometry and
// HitTest maps a point to an item, so paint and hit-testing read one layout.
//
// (Not to be confused with Dock, the edge-docking LAYOUT container: AppDock is a
// visual launcher; Dock arranges bars around a body.)
type AppDock struct {
	Base
	// Items are the entries, left to right.
	Items []AppDockItem
	// Magnify enables the hover swell. MaxScale (default 1.6) is the peak factor
	// under the pointer; Radius (default 1.5) is the falloff reach in item
	// widths. A non-positive Radius or a MaxScale <= 1 disables the swell.
	Magnify  bool
	MaxScale float64
	Radius   float64
	// OnActivate fires with the clicked item's index.
	OnActivate func(i int)

	cursorX      int  // absolute x of the pointer while it hovers
	cursorInside bool // whether the pointer is over the dock
}

// NewAppDock builds a dock over items with magnification on at the default feel.
func NewAppDock(items ...AppDockItem) *AppDock {
	return &AppDock{Items: items, Magnify: true, MaxScale: appDockDefaultScale, Radius: appDockDefaultRadius}
}

// A11y describes the dock to a screen reader as a toolbar — a single node, the
// same way the other data-driven item widgets (ListBox) expose themselves.
func (d *AppDock) A11y() A11yInfo { return A11yInfo{Role: RoleToolbar} }

func (d *AppDock) itemW() int { return scaled(AppDockItemW) }
func (d *AppDock) itemH() int { return scaled(AppDockItemH) }
func (d *AppDock) gap() int   { return scaled(AppDockGap) }

// appDockSlot is one laid-out item: its on-screen rectangle and magnification
// scale (1 at rest).
type appDockSlot struct {
	rect  Rect
	scale float64
}

// restingSlots lays the items out left to right at rest (scale 1), vertically
// centred in the dock's bounds.
func (d *AppDock) restingSlots() []appDockSlot {
	r := d.Bounds()
	iw, ih, g := d.itemW(), d.itemH(), d.gap()
	y := r.Y + (r.H-ih)/2
	out := make([]appDockSlot, len(d.Items))
	x := r.X + g
	for i := range d.Items {
		out[i] = appDockSlot{rect: Rect{X: x, Y: y, W: iw, H: ih}, scale: 1}
		x += iw + g
	}
	return out
}

// magnifyActive reports whether the next layout should swell the items: the
// effect is enabled, the cursor is over the bar, the falloff radius is positive
// and the peak scale exceeds 1.
func (d *AppDock) magnifyActive() bool {
	return d.Magnify && d.cursorInside && d.Radius > 0 && d.MaxScale > 1 && len(d.Items) > 0
}

// slots returns the items as they paint + hit-test right now. With magnification
// inactive that is exactly restingSlots. With it active each item's scale is a
// raised-cosine falloff of the cursor's horizontal distance to the item centre,
// the item swells in width and height, the row reflows left to right preserving
// the resting gaps, and the whole row is shifted so the point under the cursor
// stays fixed.
func (d *AppDock) slots() []appDockSlot {
	base := d.restingSlots()
	if !d.magnifyActive() {
		return base
	}
	top := d.Bounds().Y
	iw := d.itemW()
	radiusPx := d.Radius * float64(iw)
	n := len(base)
	origX := make([]int, n)
	for i := range base {
		origX[i] = base[i].rect.X
		center := float64(base[i].rect.X) + float64(iw)/2
		u := math.Abs(float64(d.cursorX)-center) / radiusPx
		base[i].scale = 1 + (d.MaxScale-1)*appDockBump(u)
	}
	cur := origX[0]
	for i := range base {
		nw := int(math.Round(float64(iw) * base[i].scale))
		origH := base[i].rect.H
		bottom := base[i].rect.Y + origH
		nh := int(math.Round(float64(origH) * base[i].scale))
		ny := bottom - nh
		if ny < top { // keep the swollen item inside the bar; it grows upward
			ny = top // from its pinned bottom, clamped at the dock's top edge
			nh = bottom - ny
		}
		base[i].rect.X = cur
		base[i].rect.W = nw
		base[i].rect.Y = ny
		base[i].rect.H = nh
		if i < n-1 {
			g := origX[i+1] - (origX[i] + iw)
			cur = cur + nw + g
		}
	}
	// Cursor-anchor the reflowed row so the hovered point stays put.
	for i := range base {
		if d.cursorX >= origX[i] && d.cursorX < origX[i]+iw {
			frac := (float64(d.cursorX) - float64(origX[i])) / float64(iw)
			newPointX := float64(base[i].rect.X) + frac*float64(base[i].rect.W)
			if shift := int(math.Round(float64(d.cursorX) - newPointX)); shift != 0 {
				for j := range base {
					base[j].rect.X += shift
				}
			}
			break
		}
	}
	return base
}

// appDockBump is the raised-cosine magnification falloff: 1 at the cursor
// (u<=0), easing smoothly to 0 at the falloff edge (u>=1) with a soft cosine
// shoulder.
func appDockBump(u float64) float64 {
	if u <= 0 {
		return 1
	}
	if u >= 1 {
		return 0
	}
	return 0.5 * (1 + math.Cos(math.Pi*u))
}

// ItemRects returns each item's current on-screen rectangle (magnified while
// hovered, resting otherwise), one per item in order — the geometry a host
// publishes and hit-tests against.
func (d *AppDock) ItemRects() []Rect {
	sl := d.slots()
	out := make([]Rect, len(sl))
	for i := range sl {
		out[i] = sl[i].rect
	}
	return out
}

// HitTest returns the index of the item at the absolute point (x,y), or -1 when
// the point hits no item.
func (d *AppDock) HitTest(x, y int) int {
	for i, r := range d.ItemRects() {
		if r.Contains(x, y) {
			return i
		}
	}
	return -1
}

// SetCursor updates the pointer position that drives magnification, in absolute
// coordinates; inside is whether the pointer is currently over the dock. A host
// that does not route EventMouseMove can drive the swell through this instead.
func (d *AppDock) SetCursor(x int, inside bool) {
	d.cursorX, d.cursorInside = x, inside
}

// Draw paints the ground bar then every item.
func (d *AppDock) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	ground := &Backdrop{Fill: theme.SurfaceAlt}
	ground.SetBounds(r)
	ground.Draw(p, theme)
	sl := d.slots()
	for i := range d.Items {
		d.drawItem(p, theme, d.Items[i], sl[i])
	}
}

// drawItem paints one item: a rounded face (accent when Active), the swelling
// glyph, the clipped label, the running dot and the attention badge.
func (d *AppDock) drawItem(p painter.Painter, theme *Theme, it AppDockItem, sl appDockSlot) {
	r := sl.rect
	face, ink := theme.Surface, theme.OnSurface
	if it.Active {
		face, ink = theme.Accent, accentInk(theme)
	}
	fc := &Backdrop{Fill: face, Radius: scaled(appDockRadius)}
	fc.SetBounds(r)
	fc.Draw(p, theme)

	// Glyph, swelling with the item but clamped inside the face.
	gsz := scaled(AppDockGlyphPx)
	if sl.scale > 1 {
		gsz = int(float64(gsz)*sl.scale + 0.5)
	}
	if lim := r.H - 2*scaled(2); gsz > lim {
		gsz = lim
	}
	gx := r.X + scaled(AppDockPadX)
	if it.Icon != nil {
		it.Icon(p, Rect{X: gx, Y: r.Y + (r.H-gsz)/2, W: gsz, H: gsz}, ink)
	}

	// Label to the right of the glyph, clipped to the item's remaining width.
	if it.Label != "" {
		tx := gx + gsz + scaled(AppDockLabelGap)
		maxW := r.X + r.W - tx - scaled(2)
		if label := d.clip(it.Label, maxW); label != "" {
			d.drawText(p, tx, r.Y+(r.H-d.glyphHeight())/2, label, ink)
		}
	}

	// Running dot centred under the glyph.
	if it.Running {
		dot := scaled(appDockRunDot)
		rd := &Backdrop{Fill: ink, Radius: dot / 2}
		rd.SetBounds(Rect{X: gx + (gsz-dot)/2, Y: r.Y + r.H - dot - scaled(1), W: dot, H: dot})
		rd.Draw(p, theme)
	}

	// Attention badge at the top-right.
	if it.Badge > 0 {
		text := strconv.Itoa(it.Badge)
		bw := d.textWidth(text) + 2*scaled(BadgePadX)
		bh := d.glyphHeight() + 2*scaled(BadgePadY)
		b := &Badge{Text: text}
		b.Font = d.Font
		b.SetBounds(Rect{X: r.X + r.W - bw - scaled(2), Y: r.Y + scaled(1), W: bw, H: bh})
		b.Draw(p, theme)
	}
}

// clip truncates s (with a trailing ellipsis) to at most maxW pixels in the
// dock's effective font, returning "" when even the ellipsis will not fit.
func (d *AppDock) clip(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if d.textWidth(s) <= maxW {
		return s
	}
	const ell = "…"
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		if d.textWidth(string(rs)+ell) <= maxW {
			return string(rs) + ell
		}
	}
	return ""
}

// OnEvent drives magnification from EventMouseMove (the pointer position sets
// the swell; a move that leaves the bounds flattens the row) and activation from
// EventClick (the hit item fires OnActivate). Coordinates are widget-local.
func (d *AppDock) OnEvent(ev Event) {
	switch ev.Kind {
	case EventMouseMove:
		d.cursorInside = d.localInBounds(ev.X, ev.Y)
		d.cursorX = d.Bounds().X + ev.X
	case EventClick:
		if d.OnActivate == nil {
			return
		}
		b := d.Bounds()
		if i := d.HitTest(b.X+ev.X, b.Y+ev.Y); i >= 0 {
			d.OnActivate(i)
		}
	}
}
