// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// SourceList is a macOS-style "source list" (an NSOutlineView sidebar): one or
// more labelled sections, each a run of rows carrying a leading icon and a
// label, with the selected row drawn as a rounded accent pill. A section can be
// marked Reorderable, in which case its rows can be dragged to reorder within
// that section (via the toolkit's DragSource/DropTarget contract). It
// generalizes the file-manager sidebar / mail-and-settings navigator: a flat
// ListBox cannot express section headers or per-section reorderability, which is
// exactly the gap SourceList fills.
//
// Layout: a thin panel filled with Theme.SurfaceAlt, a hairline Theme.Border on
// its right edge, then top-to-bottom a section header (drawn in muted ink, and
// only when the section has a non-empty Title) followed by its item rows. Each
// item row shows its icon (when non-nil) left-aligned, then the label elided to
// the remaining width; the selected row paints a Theme.Accent pill behind it and
// switches the ink to Theme.Background. All painting is clipped to the widget
// bounds, so a panel shorter than its content never bleeds a row below its edge.
//
// Selection + navigation: a click selects the row under the pointer and fires
// OnSelect(section, row). Selected / SetSelected read and drive the highlighted
// row programmatically.
//
// Drag-to-reorder: pressing a row in a Reorderable section arms a drag whose
// payload is SourceRowDragPrefix + "<section>:<row>" (see DragData); a host wires
// its native pointer gestures to the toolkit's EventDragMove / EventDragLeave /
// EventDrop, and the SourceList tracks the pressed row, paints an insertion line
// on EventDragMove, and reorders the section's items on EventDrop, firing
// OnReorder. A press on a non-reorderable section arms nothing, so those rows can
// be selected but never reordered.
type SourceList struct {
	Base

	// Sections is the ordered list of labelled groups. Mutating it and calling
	// SetBounds (or letting the next SetBounds run) re-lays the rows out.
	Sections []SourceSection

	// OnSelect fires after a click selects an item row, with the section index
	// and the row index within that section. Nil-guarded.
	OnSelect func(section, row int)

	// OnReorder fires after a successful drag-reorder within a section, with the
	// section index and the row's original + final indices. Nil-guarded.
	OnReorder func(section, from, to int)

	selSection int // selected section, or -1 when nothing is selected
	selRow     int // selected row within selSection, or -1

	rows []slRow // laid-out row rectangles, recomputed on SetBounds

	pressedSection int // section a reorderable press landed on, or -1
	pressedRow     int // row a reorderable press landed on, or -1
	dragging       bool
	dragY          int
}

// SourceItem is one row of a SourceList: an optional leading icon and a label.
// Key is an opaque caller-supplied identity (a path, a mailbox id, ...) the host
// can read back after OnSelect; the widget itself never interprets it.
type SourceItem struct {
	Icon  *Image
	Label string
	Key   string
}

// SourceSection is a labelled group of SourceItems. When Reorderable is true its
// rows can be dragged to reorder within the section; when false (the default) its
// rows are selectable but fixed in order.
type SourceSection struct {
	Title       string
	Items       []SourceItem
	Reorderable bool
}

// slRow is one laid-out SourceList entry: a section header (row < 0) or an item
// row (row >= 0). reorderable mirrors the owning section's flag so hit-testing
// can arm a drag without re-reading Sections.
type slRow struct {
	rect        Rect
	section     int
	row         int // -1 for a section header
	reorderable bool
}

// SourceList metrics (pixels).
const (
	slHeaderH    = 24
	slRowH       = 28
	slIconPx     = 17
	slLeftPad    = 14
	slIconGap    = 9
	slSectGap    = 8
	slTopPad     = 10
	slRowInset   = 6
	slPillRadius = 6
)

// SourceRowDragPrefix is the payload scheme a SourceList reorder drag carries:
// DragData returns this prefix followed by "<section>:<row>", and AcceptsDrop
// recognizes only payloads bearing it, so a foreign drag is never mistaken for a
// row reorder.
const SourceRowDragPrefix = "sourcerow:"

// NewSourceList builds a SourceList over sections. Nothing is selected initially
// (Selected returns -1, -1) and no press is armed; call SetBounds to lay the rows
// out before drawing.
func NewSourceList(sections ...SourceSection) *SourceList {
	return &SourceList{
		Sections:       sections,
		selSection:     -1,
		selRow:         -1,
		pressedSection: -1,
		pressedRow:     -1,
	}
}

// SourceList participates in drag-and-drop as both source and target, and
// describes itself for accessibility.
var (
	_ DragSource = (*SourceList)(nil)
	_ DropTarget = (*SourceList)(nil)
	_ Accessible = (*SourceList)(nil)
)

// A11y reports the SourceList as navigation. Value is the selected item's label,
// or empty when nothing is selected.
func (s *SourceList) A11y() A11yInfo {
	v := ""
	if s.selSection >= 0 && s.selSection < len(s.Sections) &&
		s.selRow >= 0 && s.selRow < len(s.Sections[s.selSection].Items) {
		v = s.Sections[s.selSection].Items[s.selRow].Label
	}
	return A11yInfo{Role: RoleNavigation, Value: v}
}

// Selected returns the highlighted item as (section, row), or (-1, -1) when
// nothing is selected.
func (s *SourceList) Selected() (section, row int) { return s.selSection, s.selRow }

// SetSelected highlights the item at (section, row). An out-of-range pair clears
// the selection to (-1, -1).
func (s *SourceList) SetSelected(section, row int) {
	if section >= 0 && section < len(s.Sections) &&
		row >= 0 && row < len(s.Sections[section].Items) {
		s.selSection, s.selRow = section, row
		return
	}
	s.selSection, s.selRow = -1, -1
}

// SetBounds records the widget bounds and recomputes the row layout.
func (s *SourceList) SetBounds(r Rect) {
	s.Base.SetBounds(r)
	s.layout()
}

// layout recomputes every row rectangle top to bottom: a header (only for a
// section with a non-empty Title) then the section's item rows.
func (s *SourceList) layout() {
	b := s.Bounds()
	y := b.Y + slTopPad
	s.rows = s.rows[:0]
	for si := range s.Sections {
		sec := &s.Sections[si]
		if sec.Title != "" {
			s.rows = append(s.rows, slRow{
				rect:    Rect{X: b.X, Y: y, W: b.W, H: slHeaderH},
				section: si,
				row:     -1,
			})
			y += slHeaderH
		}
		for ri := range sec.Items {
			s.rows = append(s.rows, slRow{
				rect:        Rect{X: b.X, Y: y, W: b.W, H: slRowH},
				section:     si,
				row:         ri,
				reorderable: sec.Reorderable,
			})
			y += slRowH
		}
		y += slSectGap
	}
}

// Draw paints the panel, its sections and rows, and (while dragging) the
// reorder insertion line, clipped to the widget bounds.
func (s *SourceList) Draw(p painter.Painter, theme *Theme) {
	b := s.Bounds()
	fillRect(p, b.X, b.Y, b.W, b.H, theme.SurfaceAlt)
	fillRect(p, b.X+b.W-1, b.Y, 1, b.H, theme.Border)

	withClip(p, b, func() {
		headerInk := mutedInk(theme)
		for _, row := range s.rows {
			if row.row < 0 {
				ty := row.rect.Y + (row.rect.H-s.glyphHeight())/2
				s.drawText(p, row.rect.X+slLeftPad, ty, s.Sections[row.section].Title, headerInk)
				continue
			}
			s.drawItem(p, theme, row)
		}
		if s.dragging {
			if _, lineY, ok := s.dropTarget(s.dragY); ok {
				fillRect(p, b.X+slRowInset, lineY-1, b.W-2*slRowInset, 2, theme.Accent)
			}
		}
	})
}

// drawItem paints one item row: the selection pill (when selected), the leading
// icon (when non-nil) and the label elided to the remaining width.
func (s *SourceList) drawItem(p painter.Painter, theme *Theme, row slRow) {
	r := row.rect
	item := s.Sections[row.section].Items[row.row]
	ink := theme.OnSurface
	if row.section == s.selSection && row.row == s.selRow {
		hl := Rect{X: r.X + slRowInset, Y: r.Y + 2, W: r.W - 2*slRowInset, H: r.H - 4}
		p.FillRoundRect(hl, slPillRadius, theme.Accent)
		ink = theme.Background
	}

	tx := r.X + slLeftPad
	if item.Icon != nil {
		iy := r.Y + (r.H-slIconPx)/2
		item.Icon.SetBounds(Rect{X: r.X + slLeftPad, Y: iy, W: slIconPx, H: slIconPx})
		item.Icon.Draw(p, theme)
		tx += slIconPx + slIconGap
	}

	ty := r.Y + (r.H-s.glyphHeight())/2
	avail := r.X + r.W - slRowInset - tx
	label := item.Label
	if s.textWidth(label) > avail {
		label = ellipsize(s.EffectiveFont(), label, avail)
	}
	s.drawText(p, tx, ty, label, ink)
}

// itemRowAt returns the item row whose rectangle contains y (headers ignored),
// and ok=false when y falls on a header or outside every row.
func (s *SourceList) itemRowAt(y int) (slRow, bool) {
	for _, row := range s.rows {
		if row.row >= 0 && y >= row.rect.Y && y < row.rect.Y+row.rect.H {
			return row, true
		}
	}
	return slRow{}, false
}

// OnEvent selects on a click, arms/drives a reorder drag on a Reorderable
// section, and is inert while Disabled.
func (s *SourceList) OnEvent(ev Event) {
	if s.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick:
		s.pressedSection, s.pressedRow = -1, -1
		row, ok := s.itemRowAt(ev.Y)
		if !ok {
			return
		}
		if row.reorderable {
			s.pressedSection, s.pressedRow = row.section, row.row
		}
		s.selSection, s.selRow = row.section, row.row
		if s.OnSelect != nil {
			s.OnSelect(row.section, row.row)
		}
	case EventDragMove:
		if s.pressedRow >= 0 {
			s.dragging = true
			s.dragY = ev.Y
		}
	case EventDragLeave:
		s.dragging = false
	case EventDrop:
		if s.dragging && s.pressedRow >= 0 {
			if sec, idx, ok := s.dropTargetIndex(ev.Y); ok && sec == s.pressedSection {
				s.moveItem(sec, s.pressedRow, idx)
			}
		}
		s.dragging = false
		s.pressedSection, s.pressedRow = -1, -1
	}
}

// sectionBand returns the item rows of the pressed section, in order.
func (s *SourceList) sectionBand() []slRow {
	var band []slRow
	for _, row := range s.rows {
		if row.section == s.pressedSection && row.row >= 0 {
			band = append(band, row)
		}
	}
	return band
}

// dropTargetIndex maps a pointer y to the insertion index within the pressed
// section's band, returning the section, the index in [0, len(band)] and ok. It
// is ok=false when the pressed section has no item rows or y falls well outside
// the band.
func (s *SourceList) dropTargetIndex(y int) (section, index int, ok bool) {
	band := s.sectionBand()
	if len(band) == 0 {
		return s.pressedSection, 0, false
	}
	first := band[0].rect.Y
	last := band[len(band)-1].rect
	if y < first-slRowH || y > last.Y+last.H+slRowH {
		return s.pressedSection, 0, false
	}
	for i, row := range band {
		if y < row.rect.Y+row.rect.H/2 {
			return s.pressedSection, i, true
		}
	}
	return s.pressedSection, len(band), true
}

// dropTarget maps a pointer y to the pixel Y of the insertion line and ok — the
// Draw-time counterpart of dropTargetIndex.
func (s *SourceList) dropTarget(y int) (section, lineY int, ok bool) {
	sec, idx, ok := s.dropTargetIndex(y)
	if !ok {
		return sec, 0, false
	}
	band := s.sectionBand()
	if idx >= len(band) {
		last := band[len(band)-1].rect
		return sec, last.Y + last.H, true
	}
	return sec, band[idx].rect.Y, true
}

// moveItem relocates item "from" to insertion index "to" within section sec,
// preserving every other row's order, then re-lays out and fires OnReorder with
// the row's final index. An out-of-range "from" is ignored.
func (s *SourceList) moveItem(sec, from, to int) {
	items := s.Sections[sec].Items
	if from < 0 || from >= len(items) {
		return
	}
	it := items[from]
	items = append(items[:from], items[from+1:]...)
	if to > from {
		to--
	}
	items = append(items, SourceItem{})
	copy(items[to+1:], items[to:])
	items[to] = it
	s.Sections[sec].Items = items
	s.layout()
	if s.OnReorder != nil {
		s.OnReorder(sec, from, to)
	}
}

// DragData reports the reorder payload for the pressed row (a Reorderable
// section's row), or "" when no reorderable press is armed. It makes the
// SourceList a DragSource.
func (s *SourceList) DragData() string {
	if s.pressedRow < 0 {
		return ""
	}
	return SourceRowDragPrefix + strconv.Itoa(s.pressedSection) + ":" + strconv.Itoa(s.pressedRow)
}

// AcceptsDrop reports whether payload is one of this widget's own reorder
// payloads. It makes the SourceList a DropTarget for its own rows.
func (s *SourceList) AcceptsDrop(payload string) bool {
	return strings.HasPrefix(payload, SourceRowDragPrefix)
}
