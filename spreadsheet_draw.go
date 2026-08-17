// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit/internal/formula"
)

// Draw paints the sheet: the cell grid (clipped to its viewport), the frozen
// column-letter and row-number header bands, the active-cell selection box, the
// scrollbars, and finally the inline editor overlay when a cell is being edited.
func (s *Spreadsheet) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	g := s.gridRect()

	// Cell body, clipped to the grid viewport so overflow and the active-cell
	// box never spill onto the header bands or the scrollbars.
	fillRect(p, g.X, g.Y, g.W, g.H, theme.Surface)
	withClip(p, g, func() {
		nc, nr := s.visibleCols(), s.visibleRows()
		for ci := 0; ci < nc; ci++ {
			for ri := 0; ri < nr; ri++ {
				s.drawCell(p, theme, s.scrollCol+ci, s.scrollRow+ri)
			}
		}
		cr := s.cellRect(s.cur.Col, s.cur.Row)
		strokeRect(p, cr.X, cr.Y, cr.W, cr.H, theme.Accent)
	})

	// Header bands (they never scroll on their cross axis).
	fillRect(p, r.X, r.Y, spRowHdrW(), spHeaderH(), theme.SurfaceAlt) // corner
	withClip(p, Rect{X: g.X, Y: r.Y, W: g.W, H: spHeaderH()}, func() {
		nc := s.visibleCols()
		for ci := 0; ci < nc; ci++ {
			col := s.scrollCol + ci
			cr := s.cellRect(col, 0)
			s.drawBandCell(p, theme, Rect{X: cr.X, Y: r.Y, W: spColW(), H: spHeaderH()},
				formula.ColumnName(col), col == s.cur.Col)
		}
	})
	withClip(p, Rect{X: r.X, Y: g.Y, W: spRowHdrW(), H: g.H}, func() {
		nr := s.visibleRows()
		for ri := 0; ri < nr; ri++ {
			row := s.scrollRow + ri
			cr := s.cellRect(0, row)
			s.drawBandCell(p, theme, Rect{X: r.X, Y: cr.Y, W: spRowHdrW(), H: spRowH()},
				strconv.Itoa(row+1), row == s.cur.Row)
		}
	})

	if s.vOverflow() {
		s.drawVScroll(p, theme)
	}
	if s.hOverflow() {
		s.drawHScroll(p, theme)
	}

	// Inline editor overlay, on top of everything, positioned over its cell.
	if s.editor != nil {
		s.editor.SetBounds(s.cellRect(s.cur.Col, s.cur.Row))
		s.editor.Draw(p, theme)
	}
	s.drawFocusRing(p, theme, r)
}

// drawCell paints one data cell: its bottom + right grid lines and its computed
// value. Numbers align right, text and errors align left; an error value is
// inked in the shared error red. The text is clipped to the cell so a wide
// value cannot bleed into its neighbour.
func (s *Spreadsheet) drawCell(p painter.Painter, theme *Theme, col, row int) {
	cr := s.cellRect(col, row)
	fillRect(p, cr.X, cr.Y+cr.H-1, cr.W, 1, theme.Border) // bottom grid line
	fillRect(p, cr.X+cr.W-1, cr.Y, 1, cr.H, theme.Border) // right grid line

	val := s.model.Get(formula.Ref{Col: col, Row: row})
	text := val.Display()
	if text == "" {
		return
	}
	ink := theme.OnSurface
	align := AlignLeft
	if val.Kind == formula.KindNumber {
		align = AlignRight
	}
	if val.IsError() {
		ink = spreadsheetErrorInk
	}
	tx := cellTextX(&s.Base, cr.X, cr.W, text, align)
	ty := cr.Y + (cr.H-s.glyphHeight())/2
	withClip(p, cr, func() { s.drawText(p, tx, ty, text, ink) })
}

// spreadsheetErrorInk is the red an errored cell (#DIV/0!, #REF!, ...) is inked
// in — the same brick red Table rings a rejected edit with, so errors read
// consistently across the toolkit.
var spreadsheetErrorInk = RGB(0xC0, 0x30, 0x30)

// drawBandCell paints one header-band cell (a column letter or a row number):
// an Accent face when it heads the active cell's column/row, a SurfaceAlt face
// otherwise, with the label centred and a separating border on its trailing
// edges.
func (s *Spreadsheet) drawBandCell(p painter.Painter, theme *Theme, cell Rect, label string, active bool) {
	face := theme.SurfaceAlt
	ink := theme.OnSurface
	if active {
		face = theme.Accent
		ink = accentInk(theme)
	}
	fillRect(p, cell.X, cell.Y, cell.W, cell.H, face)
	fillRect(p, cell.X, cell.Y+cell.H-1, cell.W, 1, theme.Border)
	fillRect(p, cell.X+cell.W-1, cell.Y, 1, cell.H, theme.Border)
	tx := cellTextX(&s.Base, cell.X, cell.W, label, AlignCenter)
	ty := cell.Y + (cell.H-s.glyphHeight())/2
	s.drawText(p, tx, ty, label, ink)
}

// vscrollGeom returns the vertical scrollbar's widget-local geometry and whether
// it is live (the rows overflow). The thumb is sized + placed in row-pixel space
// exactly as Table's scrollbar is, and the scroll value it maps to is scrollRow,
// clamped to maxScrollRow. vOverflow guarantees contentH > trackH > 0, so the
// travel denominator is positive.
func (s *Spreadsheet) vscrollGeom() (sbGeom, bool) {
	if !s.vOverflow() {
		return sbGeom{}, false
	}
	r := s.Bounds()
	trackH := s.gridRect().H
	contentH := s.model.Rows() * spRowH()
	thumbH := trackH * trackH / contentH
	if thumbH < spreadsheetThumbMin {
		thumbH = spreadsheetThumbMin
	}
	return sbGeom{
		cross0:     r.W - scrollbarWidth,
		crossW:     scrollbarWidth,
		trackStart: spHeaderH(),
		trackLen:   trackH,
		thumbStart: spHeaderH() + s.scrollRow*spRowH()*(trackH-thumbH)/(contentH-trackH),
		thumbLen:   thumbH,
		travelNum:  spRowH() * (trackH - thumbH),
		travelDen:  contentH - trackH,
		maxScroll:  s.maxScrollRow(),
	}, true
}

// hscrollGeom is the horizontal counterpart of vscrollGeom, in column-pixel
// space along the bottom edge.
func (s *Spreadsheet) hscrollGeom() (sbGeom, bool) {
	if !s.hOverflow() {
		return sbGeom{}, false
	}
	r := s.Bounds()
	trackW := s.gridRect().W
	contentW := s.model.Cols() * spColW()
	thumbW := trackW * trackW / contentW
	if thumbW < spreadsheetThumbMin {
		thumbW = spreadsheetThumbMin
	}
	return sbGeom{
		horizontal: true,
		cross0:     r.H - scrollbarWidth,
		crossW:     scrollbarWidth,
		trackStart: spRowHdrW(),
		trackLen:   trackW,
		thumbStart: spRowHdrW() + s.scrollCol*spColW()*(trackW-thumbW)/(contentW-trackW),
		thumbLen:   thumbW,
		travelNum:  spColW() * (trackW - thumbW),
		travelDen:  contentW - trackW,
		maxScroll:  s.maxScrollCol(),
	}, true
}

// drawVScroll paints the right-edge vertical scrollbar; caller guards on
// vOverflow so vscrollGeom is live here.
func (s *Spreadsheet) drawVScroll(p painter.Painter, theme *Theme) {
	g, _ := s.vscrollGeom()
	r := s.Bounds()
	paintScrollTrack(p, theme, r.X+g.cross0, r.Y+g.trackStart, scrollbarWidth, g.trackLen)
	paintScrollThumb(p, theme, r.X+g.cross0, r.Y+g.thumbStart, scrollbarWidth, g.thumbLen)
}

// drawHScroll paints the bottom-edge horizontal scrollbar; caller guards on
// hOverflow so hscrollGeom is live here.
func (s *Spreadsheet) drawHScroll(p painter.Painter, theme *Theme) {
	g, _ := s.hscrollGeom()
	r := s.Bounds()
	paintScrollTrack(p, theme, r.X+g.trackStart, r.Y+g.cross0, g.trackLen, scrollbarWidth)
	paintScrollThumb(p, theme, r.X+g.thumbStart, r.Y+g.cross0, g.thumbLen, scrollbarWidth)
}
