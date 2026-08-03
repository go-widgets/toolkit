// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// IconFunc paints a vector/stock icon into rect r using ink as its stroke
// colour — the same shape the toolkit's own DrawIcon*** helpers (DrawIconNew,
// DrawIconSettings, ...) and Table's RowIcon already use. A StatusIcon takes
// one so a caller can hang any of the stock icons, or a hand-drawn one, in a
// tray slot without shipping a bitmap.
type IconFunc func(p painter.Painter, r Rect, ink RGBA)

// StatusIconSize is the default square dimension in pixels when Bounds() is
// zero-sized. 18 px sits in the 16-22 px band a desktop status area (the
// GNOME top-bar tray, the macOS menu-bar extras) gives each indicator, so a
// StatusIcon drops next to a clock or a Label without extra layout maths.
const StatusIconSize = 18

// StatusIconSecondary is the Event.Code a host tags a secondary (right / menu)
// click with so a StatusIcon can route it to OnRightClick. A primary click
// carries any other Code (typically ""). This mirrors how other widgets read
// Event.Code to distinguish an activation variant without a dedicated
// EventKind — the compositor sets it when the secondary mouse button (or a
// long-press) produced the click.
const StatusIconSecondary = "right"

// StatusIcon is a small tray/status-area indicator: it paints one icon (an
// [IconFunc] vector glyph or an RGBA image) at a fixed cell, optionally
// overlays a [Badge] in the top-right corner, carries a Tooltip string the
// host shows on hover, and fires OnClick / OnRightClick when activated.
//
// The cell has no background of its own, so whatever the tray sits on (a panel
// fill, a Wallpaper) shows through around the glyph — the least-surprising look
// for a status-area icon. A caller wanting a chip behind the glyph draws it
// under the StatusIcon.
//
// Image vs icon: when Pixels is a valid RGBA buffer it is drawn (aspect-
// preserving, centred — a [ScaleFit] Image) and Icon is ignored; otherwise the
// Icon func is called with Ink (falling back to Theme.OnSurface). Either may be
// absent, in which case only the optional Badge paints.
//
// Auto-sizing: if Bounds().W is zero the first Draw() resizes the widget to
// StatusIconSize x StatusIconSize (H preserved when already non-zero). A
// pre-sized Bounds is honoured verbatim so a fixed tray column doesn't shift.
type StatusIcon struct {
	Base
	// Icon paints the glyph when Pixels is not a valid image. May be nil.
	Icon IconFunc
	// Pixels is an optional RGBA image (IW*IH*4 bytes). When valid it is drawn
	// instead of Icon, aspect-preserved and centred in the cell.
	Pixels []byte
	IW, IH int
	// Ink overrides the icon colour; the zero RGBA (A==0) falls back to
	// Theme.OnSurface.
	Ink RGBA
	// Badge, when non-nil, is painted in the top-right corner (an unread count,
	// a status dot). It is auto-sized + positioned by Draw.
	Badge *Badge
	// Tooltip is the hover text the host surfaces (via a Tooltip widget). The
	// StatusIcon only stores it; it does not pop the bubble itself.
	Tooltip string
	// OnClick fires on a primary EventClick; OnRightClick on a secondary one
	// (Event.Code == StatusIconSecondary). Both are nil-safe.
	OnClick      func()
	OnRightClick func()
}

// NewStatusIcon builds a StatusIcon that paints the given vector icon. Bounds
// default to zero so the first Draw() auto-sizes the cell.
func NewStatusIcon(icon IconFunc) *StatusIcon { return &StatusIcon{Icon: icon} }

// NewStatusIconImage builds a StatusIcon that paints the given RGBA image
// (length must equal w*h*4). The image is drawn aspect-preserved + centred.
func NewStatusIconImage(pixels []byte, w, h int) *StatusIcon {
	return &StatusIcon{Pixels: pixels, IW: w, IH: h}
}

// hasImage reports whether Pixels is a usable RGBA buffer for the source dims.
func (s *StatusIcon) hasImage() bool {
	return s.IW > 0 && s.IH > 0 && len(s.Pixels) >= s.IW*s.IH*4
}

// Draw paints the icon (image or vector) then the optional Badge overlay. If
// Bounds().W is zero the widget resizes itself to StatusIconSize square (H
// preserved when non-zero) before painting.
func (s *StatusIcon) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W == 0 {
		r.W = StatusIconSize
		if r.H == 0 {
			r.H = StatusIconSize
		}
		s.SetBounds(r)
	}
	if s.hasImage() {
		img := Image{Pixels: s.Pixels, W: s.IW, H: s.IH, Scale: ScaleFit}
		img.SetBounds(r)
		img.Draw(p, theme)
	} else if s.Icon != nil {
		ink := s.Ink
		if ink.A == 0 {
			ink = theme.OnSurface
		}
		s.Icon(p, r, ink)
	}
	if s.Badge != nil {
		bw := s.Badge.textWidth(s.Badge.Text) + 2*BadgePadX
		bh := s.Badge.glyphHeight() + 2*BadgePadY
		s.Badge.SetBounds(Rect{X: r.X + r.W - bw, Y: r.Y, W: bw, H: bh})
		s.Badge.Draw(p, theme)
	}
}

// OnEvent fires OnRightClick on a secondary EventClick (Code ==
// StatusIconSecondary) and OnClick on any other EventClick; other event kinds
// are ignored. Both callbacks are nil-safe.
func (s *StatusIcon) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if ev.Code == StatusIconSecondary {
		if s.OnRightClick != nil {
			s.OnRightClick()
		}
		return
	}
	if s.OnClick != nil {
		s.OnClick()
	}
}

// StatusAreaGap is the default horizontal spacing in pixels between the icons
// in a StatusArea when Gap is left at its zero value.
const StatusAreaGap = 4

// StatusArea is a tray container: it lays out N StatusIcons in a left-to-right
// row, each in a square IconSize cell centred vertically in the area, with Gap
// pixels between them — a mini Dock/HBox specialised for status indicators. It
// routes a pointer event to the icon whose cell contains it (in the icon's
// local space), so each StatusIcon's OnClick/OnRightClick fires correctly.
type StatusArea struct {
	Base
	// Icons are laid out in order. Mutate through Add or set directly then call
	// SetBounds to re-flow.
	Icons []*StatusIcon
	// Gap is the spacing between cells; zero selects StatusAreaGap. A negative
	// value is clamped to 0 (flush icons).
	Gap int
	// IconSize is each cell's square dimension; zero selects StatusIconSize.
	IconSize int
}

// NewStatusArea builds a StatusArea over the given icons (any number, including
// none). Call SetBounds to lay them out.
func NewStatusArea(icons ...*StatusIcon) *StatusArea { return &StatusArea{Icons: icons} }

// Add appends an icon and re-flows the row against the current Bounds.
func (a *StatusArea) Add(ic *StatusIcon) {
	a.Icons = append(a.Icons, ic)
	a.SetBounds(a.Bounds())
}

// cellSize and gap resolve the effective per-icon dimension + spacing.
func (a *StatusArea) cellSize() int {
	if a.IconSize > 0 {
		return a.IconSize
	}
	return StatusIconSize
}

func (a *StatusArea) gap() int {
	if a.Gap > 0 {
		return a.Gap
	}
	if a.Gap < 0 {
		return 0
	}
	return StatusAreaGap
}

// SetBounds places each icon in a square cell along the row, vertically centred
// in the area's height.
func (a *StatusArea) SetBounds(r Rect) {
	a.Base.SetBounds(r)
	sz := a.cellSize()
	g := a.gap()
	x := r.X
	y := r.Y + (r.H-sz)/2
	for _, ic := range a.Icons {
		ic.SetBounds(Rect{X: x, Y: y, W: sz, H: sz})
		x += sz + g
	}
}

// Draw paints every icon in insertion order.
func (a *StatusArea) Draw(p painter.Painter, theme *Theme) {
	for _, ic := range a.Icons {
		ic.Draw(p, theme)
	}
}

// OnEvent forwards to the first icon whose cell contains the point, translating
// the event into that icon's local space.
func (a *StatusArea) OnEvent(ev Event) {
	pr := a.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, ic := range a.Icons {
		if ic.Bounds().Contains(sx, sy) {
			ic.OnEvent(translateEvent(ev, pr, ic.Bounds()))
			return
		}
	}
}
