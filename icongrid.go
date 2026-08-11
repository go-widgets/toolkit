// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// IconGrid is a selectable, size-driven grid of icon/thumbnail cells that
// reflows to the widget width. It generalizes a file-manager "icon view": unlike
// a bare reflowing grid it owns per-cell chrome — the icon is centred and fit in
// a subtle rounded frame, a real raster thumbnail sits on a light backing chip so
// a dark image stays visible on a dark theme, and the label is centred and elided
// to the cell width — plus selection and hit-testing. The cell footprint is
// driven by IconSize, so a host slider can resize every cell live.
//
// Layout: a body filled with Theme.Surface (or a centred empty-state message when
// there are no cells), then a reflowing grid of uniform cells centred within any
// left-over horizontal slack, scrolled vertically and clipped to the widget
// bounds. Each cell reserves padding above the icon, the icon square, a gap, and a
// label band; the selected cell paints a soft rounded field behind its icon and a
// rounded accent highlight behind its label.
//
// Selection + navigation: a click selects the cell under the pointer (firing
// OnSelect); a second click on the already-selected cell activates it (firing
// OnActivate). The wheel scrolls the grid. Selected / SetSelected read and drive
// the selection programmatically, and DragData makes a selected cell a
// DragSource carrying its Key.
type IconGrid struct {
	Base

	// Cells is the ordered grid content. Mutating it is reflected on the next
	// Draw; the scroll offset is clamped on the fly, so shrinking Cells never
	// scrolls past the end.
	Cells []IconCell

	// IconSize is the icon square side in pixels; it drives the whole cell
	// footprint. Set it via SetIconSize (which enforces a sane minimum).
	IconSize int

	// Empty is the message centred in an empty grid; a blank Empty falls back to
	// a generic default.
	Empty string

	// OnSelect fires when a click moves the selection to a new cell, with its
	// index. Nil-guarded.
	OnSelect func(index int)

	// OnActivate fires when the already-selected cell is clicked again, with its
	// index. Nil-guarded.
	OnActivate func(index int)

	scroll int // vertical scroll offset in pixels
	sel    int // selected index, or -1
}

// IconCell is one cell of an IconGrid: a thumbnail/icon and a label. Raster marks
// the Image as a real raster thumbnail (a photo, a rendered preview) so the cell
// paints a light chip behind it; leave it false for a flat vector/symbol icon
// that needs no backing. Key is an opaque caller identity carried by DragData.
type IconCell struct {
	Image  *Image
	Label  string
	Key    string
	Raster bool
}

// IconGrid cell metrics (pixels).
const (
	igPadX      = 14 // horizontal padding either side of the icon
	igPadTop    = 16 // padding above the icon
	igLabelH    = 20 // label band height below the icon
	igIconLabel = 8  // gap between icon and label
	igSelField  = 6  // inset of the soft selection field around the icon
	igChip      = 2  // inset of the light chip around a raster thumbnail
	igLabelPad  = 10 // horizontal slack subtracted before eliding the label
)

// NewIconGrid builds an IconGrid over cells with a default icon size. Nothing is
// selected initially.
func NewIconGrid(cells ...IconCell) *IconGrid {
	return &IconGrid{
		Cells:    cells,
		IconSize: 48,
		sel:      -1,
	}
}

// IconGrid is a DragSource for its selected cell and describes itself for
// accessibility.
var (
	_ DragSource = (*IconGrid)(nil)
	_ Accessible = (*IconGrid)(nil)
)

// A11y reports the IconGrid as a grid. Value is the selected cell's label, or
// empty when nothing is selected.
func (v *IconGrid) A11y() A11yInfo {
	label := ""
	if v.sel >= 0 && v.sel < len(v.Cells) {
		label = v.Cells[v.sel].Label
	}
	return A11yInfo{Role: RoleGrid, Value: label}
}

// SetIconSize sets the icon square side, clamped to a readable minimum, and
// resets the scroll so the reflow stays anchored at the top.
func (v *IconGrid) SetIconSize(px int) {
	if px < 24 {
		px = 24
	}
	v.IconSize = px
	v.scroll = 0
}

// Selected returns the selected cell index, or -1 when nothing is selected.
func (v *IconGrid) Selected() int { return v.sel }

// SetSelected selects cell index; an out-of-range index clears the selection.
func (v *IconGrid) SetSelected(index int) {
	if index >= 0 && index < len(v.Cells) {
		v.sel = index
		return
	}
	v.sel = -1
}

func (v *IconGrid) cellW() int { return v.IconSize + 2*igPadX }
func (v *IconGrid) cellH() int { return igPadTop + v.IconSize + igIconLabel + igLabelH }

// cols is the number of columns that fit in the current width (at least one).
func (v *IconGrid) cols() int {
	w := v.Bounds().W
	if w < v.cellW() {
		return 1
	}
	return w / v.cellW()
}

// Draw paints the visible cells (or the empty-state message), clipped to the
// widget bounds.
func (v *IconGrid) Draw(p painter.Painter, theme *Theme) {
	b := v.Bounds()
	fillRect(p, b.X, b.Y, b.W, b.H, theme.Surface)
	if len(v.Cells) == 0 {
		v.drawEmpty(p, theme)
		return
	}

	cols := v.cols()
	cw, ch := v.cellW(), v.cellH()
	gridW := cols * cw
	x0 := b.X + (b.W-gridW)/2
	if x0 < b.X {
		x0 = b.X
	}

	withClip(p, b, func() {
		for i := range v.Cells {
			col := i % cols
			top := b.Y + (i/cols)*ch - v.scroll
			if top+ch <= b.Y {
				continue // fully above the viewport
			}
			if top >= b.Y+b.H {
				break // fully below; every later cell is lower still
			}
			v.drawCell(p, theme, Rect{X: x0 + col*cw, Y: top, W: cw, H: ch}, i)
		}
	})
}

// drawEmpty centres the empty-state message.
func (v *IconGrid) drawEmpty(p painter.Painter, theme *Theme) {
	msg := v.Empty
	if msg == "" {
		msg = "No items"
	}
	b := v.Bounds()
	tw := v.textWidth(msg)
	v.drawText(p, b.X+(b.W-tw)/2, b.Y+b.H/2-v.glyphHeight()/2, msg, mutedInk(theme))
}

// drawCell paints one cell: the selection field, the icon (on a light chip +
// hairline frame when it is a raster thumbnail), then the centred, elided label.
func (v *IconGrid) drawCell(p painter.Painter, theme *Theme, r Rect, i int) {
	cell := v.Cells[i]
	selected := i == v.sel
	iconBox := Rect{X: r.X + igPadX, Y: r.Y + igPadTop, W: v.IconSize, H: v.IconSize}

	if selected {
		field := Rect{
			X: iconBox.X - igSelField, Y: iconBox.Y - igSelField,
			W: iconBox.W + 2*igSelField, H: iconBox.H + 2*igSelField,
		}
		p.FillRoundRect(field, 10, withAlpha(theme.Accent, 0x33))
	}

	if cell.Raster {
		chip := Rect{
			X: iconBox.X - igChip, Y: iconBox.Y - igChip,
			W: iconBox.W + 2*igChip, H: iconBox.H + 2*igChip,
		}
		p.FillRoundRect(chip, 8, chipColor(theme))
		p.StrokeRoundRect(chip, 8, theme.Border, 1)
	}
	if cell.Image != nil {
		cell.Image.SetBounds(iconBox)
		cell.Image.Draw(p, theme)
	}

	label := cell.Label
	if v.textWidth(label) > r.W-igLabelPad {
		label = ellipsize(v.EffectiveFont(), label, r.W-igLabelPad)
	}
	tw := v.textWidth(label)
	lx := r.X + (r.W-tw)/2
	ly := r.Y + igPadTop + v.IconSize + igIconLabel
	ink := theme.OnSurface
	if selected {
		hl := Rect{X: lx - 6, Y: ly - 2, W: tw + 12, H: v.glyphHeight() + 4}
		p.FillRoundRect(hl, 6, theme.Accent)
		ink = theme.Background
	}
	v.drawText(p, lx, ly, label, ink)
}

// contentHeight is the total pixel height of all rows at the current width.
func (v *IconGrid) contentHeight() int {
	cols := v.cols()
	rows := (len(v.Cells) + cols - 1) / cols
	return rows * v.cellH()
}

// clampScroll keeps off within [0, max].
func (v *IconGrid) clampScroll(off int) int {
	max := v.contentHeight() - v.Bounds().H
	if max < 0 {
		max = 0
	}
	if off < 0 {
		return 0
	}
	if off > max {
		return max
	}
	return off
}

// IndexAt maps a widget-local point to a cell index, or -1 for empty space.
func (v *IconGrid) IndexAt(x, y int) int {
	b := v.Bounds()
	cols := v.cols()
	cw, ch := v.cellW(), v.cellH()
	gridW := cols * cw
	x0 := (b.W - gridW) / 2
	if x0 < 0 {
		x0 = 0
	}
	if x < x0 || x >= x0+gridW {
		return -1
	}
	col := (x - x0) / cw
	row := (y + v.scroll) / ch
	idx := row*cols + col
	if idx >= len(v.Cells) {
		return -1
	}
	return idx
}

// OnEvent scrolls on the wheel, selects on a click, activates on a second click
// of the selected cell, and is inert while Disabled.
func (v *IconGrid) OnEvent(ev Event) {
	if v.Disabled {
		return
	}
	switch ev.Kind {
	case EventScroll:
		v.scroll = v.clampScroll(v.scroll + ev.Delta*(v.cellH()/2))
	case EventClick:
		idx := v.IndexAt(ev.X, ev.Y)
		if idx < 0 {
			v.sel = -1
			return
		}
		if idx == v.sel {
			if v.OnActivate != nil {
				v.OnActivate(idx)
			}
			return
		}
		v.sel = idx
		if v.OnSelect != nil {
			v.OnSelect(idx)
		}
	}
}

// DragData reports the selected cell's Key, or "" when nothing is selected. It
// makes the IconGrid a DragSource.
func (v *IconGrid) DragData() string {
	if v.sel < 0 || v.sel >= len(v.Cells) {
		return ""
	}
	return v.Cells[v.sel].Key
}

// withAlpha returns c with its alpha channel replaced.
func withAlpha(c RGBA, a uint8) RGBA {
	c.A = a
	return c
}

// isDarkTheme reports whether the theme background is dark (luma-based).
func isDarkTheme(theme *Theme) bool {
	c := theme.Background
	luma := int(c.R)*299 + int(c.G)*587 + int(c.B)*114
	return luma < 128*1000
}

// chipColor is the light backing chip behind a raster thumbnail: near-white on a
// dark theme (so a dark photo reads), pure white on a light one.
func chipColor(theme *Theme) RGBA {
	if isDarkTheme(theme) {
		return RGB(0xEC, 0xEE, 0xF2)
	}
	return RGB(0xFF, 0xFF, 0xFF)
}
