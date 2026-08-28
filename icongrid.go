// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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

	// MinCellW is a floor on the cell width, in LOGICAL pixels, for a grid whose
	// LABEL is the identifying thing rather than the icon.
	//
	// A cell is otherwise as wide as its icon plus padding, which is right for a
	// grid of files -- the icon says what it is and the name is a caption. It is
	// wrong for a grid of devices: "VITURE Luma Ultra" under a 40-pixel icon is
	// elided to "VITURE ...", which is the one thing the tile exists to say.
	// Zero keeps the icon-derived width.
	MinCellW int

	// OnActivate fires when the already-selected cell is clicked again, with its
	// index. Nil-guarded.
	OnActivate func(index int)

	scroll int // vertical scroll offset in pixels
	// sel is the selected index (-1 = none) as a shared Observable: a click Sets
	// it (subscribers replace the old OnSelect callback) and a host binds it.
	sel *mvvm.Observable[int]
}

// IconCell is one cell of an IconGrid: a thumbnail/icon and a label. Raster marks
// the Image as a real raster thumbnail (a photo, a rendered preview) so the cell
// paints a light chip behind it; leave it false for a flat vector/symbol icon
// that needs no backing. Key is an opaque caller identity carried by DragData.
type IconCell struct {
	Image *Image
	// Icon is a VECTOR icon drawn into the cell's icon square, for a cell whose
	// picture is a shape rather than a photograph: one of the stock DrawIcon***
	// functions, or the caller's own.
	//
	// It exists because a cell used to accept an Image and nothing else, so an
	// application whose grid holds device classes -- pairs of glasses, printers,
	// drives -- had to rasterise artwork to put anything in one, or hand-draw
	// beside the widget. The toolkit already had [IconFunc] and a stock icon
	// family; the grid simply could not take one.
	//
	// Image wins when both are set, since a caller who supplied pixels meant
	// them.
	Icon   IconFunc
	Label  string
	Key    string
	Raster bool
}

// IconGrid cell metrics, in LOGICAL pixels: each one is routed through [scaled]
// at use, so a cell grows with HiDPI and touch density like every other box
// metric in the toolkit. They were raw device pixels, which left a magnified
// interface with cells whose padding, label band and selection field had stayed
// the size they were at 1x -- a large icon in a small cell with its label
// crushed against it.
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
	if v.Selected().Get() >= 0 && v.Selected().Get() < len(v.Cells) {
		label = v.Cells[v.Selected().Get()].Label
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

// Selected is the selected cell index (-1 = none) as a shared [mvvm.Observable]:
// a host binds it (or subscribes for the old OnSelect notification) instead of
// reading a field, and a click Sets it. Lazily created so a bare &IconGrid{}
// works (its zero selection is -1, "nothing selected").
func (v *IconGrid) Selected() *mvvm.Observable[int] {
	if v.sel == nil {
		v.sel = mvvm.NewObservable(-1)
	}
	return v.sel
}

// SetSelected selects cell index; an out-of-range index clears the selection.
func (v *IconGrid) SetSelected(index int) {
	if index >= 0 && index < len(v.Cells) {
		v.Selected().Set(index)
		return
	}
	v.Selected().Set(-1)
}

func (v *IconGrid) cellW() int {
	w := v.IconSize + 2*scaled(igPadX)
	if min := scaled(v.MinCellW); min > w {
		return min
	}
	return w
}
func (v *IconGrid) cellH() int {
	return scaled(igPadTop) + v.IconSize + scaled(igIconLabel) + scaled(igLabelH)
}

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
	selected := i == v.Selected().Get()
	iconBox := Rect{X: r.X + scaled(igPadX), Y: r.Y + scaled(igPadTop), W: v.IconSize, H: v.IconSize}

	if selected {
		field := Rect{
			X: iconBox.X - scaled(igSelField), Y: iconBox.Y - scaled(igSelField),
			W: iconBox.W + 2*scaled(igSelField), H: iconBox.H + 2*scaled(igSelField),
		}
		p.FillRoundRect(field, 10, withAlpha(theme.Accent, 0x33))
	}

	if cell.Raster {
		chip := Rect{
			X: iconBox.X - scaled(igChip), Y: iconBox.Y - scaled(igChip),
			W: iconBox.W + 2*scaled(igChip), H: iconBox.H + 2*scaled(igChip),
		}
		p.FillRoundRect(chip, 8, chipColor(theme))
		p.StrokeRoundRect(chip, 8, theme.Border, 1)
	}
	switch {
	case cell.Image != nil:
		cell.Image.SetBounds(iconBox)
		cell.Image.Draw(p, theme)
	case cell.Icon != nil:
		ink := theme.OnSurface
		if selected {
			ink = theme.Accent
		}
		cell.Icon(p, iconBox, ink)
	}

	label := cell.Label
	if v.textWidth(label) > r.W-scaled(igLabelPad) {
		label = ellipsize(v.EffectiveFont(), label, r.W-scaled(igLabelPad))
	}
	tw := v.textWidth(label)
	lx := r.X + (r.W-tw)/2
	ly := r.Y + scaled(igPadTop) + v.IconSize + scaled(igIconLabel)
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
	if v.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventScroll:
		v.scroll = v.clampScroll(v.scroll + ev.Delta*(v.cellH()/2))
	case EventClick:
		idx := v.IndexAt(ev.X, ev.Y)
		if idx < 0 {
			v.Selected().Set(-1)
			return
		}
		if idx == v.Selected().Get() {
			if v.OnActivate != nil {
				v.OnActivate(idx)
			}
			return
		}
		v.Selected().Set(idx)
	}
}

// DragData reports the selected cell's Key, or "" when nothing is selected. It
// makes the IconGrid a DragSource.
func (v *IconGrid) DragData() string {
	if v.Selected().Get() < 0 || v.Selected().Get() >= len(v.Cells) {
		return ""
	}
	return v.Cells[v.Selected().Get()].Key
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

// Measure reports the height the grid needs at this width: as many rows as its
// cells make at that width, each a cell tall.
//
// It makes an IconGrid usable as content -- an item in a column, or the child of
// a ScrollView -- instead of something a caller has to give a height to. A grid
// given a height that is not a whole number of rows shows a half row and scrolls
// for no reason, and a caller computing that height by hand is computing
// cellH() * ceil(n/cols) with the toolkit's own constants, which is exactly the
// arithmetic a widget should be asked for rather than reproduced.
//
// Width, not the current bounds, because the column count follows the width and
// a parent asks before it has committed to one.
func (v *IconGrid) Measure(width int) int {
	if len(v.Cells) == 0 {
		// The empty-state message, which is one line centred in whatever it is
		// given: a row's worth is enough for it and leaves the grid the same
		// height whether or not it has anything in it yet.
		return v.cellH()
	}
	cols := width / v.cellW()
	if cols < 1 {
		cols = 1
	}
	rows := (len(v.Cells) + cols - 1) / cols
	return rows * v.cellH()
}

// Columns is how many cells fit across the grid at its current width, at least
// one.
//
// It is exported for a host that drives the selection from the KEYBOARD, which
// this widget's own docs invite: moving up or down means adding or subtracting a
// row, and a row is this number. Without it a host has to guess the column count
// from the cell metrics — which are deliberately not exported, being scaled — and
// its arrows then disagree with the picture whenever the density changes.
func (v *IconGrid) Columns() int { return v.cols() }
