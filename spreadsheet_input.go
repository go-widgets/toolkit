// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/toolkit/internal/formula"

// OnEvent drives selection, scrolling and inline editing.
//
// While an editor is open it owns the keyboard: characters and edit keys route
// to it, Enter commits and moves down, Tab commits and moves right, Escape
// cancels, and a click elsewhere commits first and then selects the clicked
// cell. With no editor open, arrow keys move the active cell, Enter/F2 opens an
// editor seeded with the cell's current text, a printable character opens one
// seeded with that character, the wheel scrolls, and a scrollbar press/drag
// scrolls; a grid click selects the cell under the pointer.
func (s *Spreadsheet) OnEvent(ev Event) {
	if s.editor != nil {
		switch ev.Kind {
		case EventChar:
			s.editor.OnEvent(ev)
			return
		case EventKeyDown:
			switch ev.Code {
			case "Escape":
				s.cancelEdit()
			case "Enter":
				s.commitEdit()
				s.moveCursor(0, 1)
			case "Tab":
				s.commitEdit()
				s.moveCursor(1, 0)
			default:
				s.editor.OnEvent(ev)
			}
			return
		case EventClick:
			s.commitEdit() // fall through to select the clicked cell
		}
	}

	switch ev.Kind {
	case EventScroll:
		s.ScrollBy(0, ev.Delta)
	case EventChar:
		s.beginEditWith(ev.Code)
	case EventKeyDown:
		s.handleKey(ev)
	case EventClick:
		if g, ok := s.vscrollGeom(); s.sbV.press(g, ok, ev, s.fullRows(), func(d int) { s.ScrollBy(0, d) }) {
			return
		}
		if g, ok := s.hscrollGeom(); s.sbH.press(g, ok, ev, s.fullCols(), func(d int) { s.ScrollBy(d, 0) }) {
			return
		}
		if ref, ok := s.cellAt(ev.X, ev.Y); ok {
			s.cur = ref
			s.ensureVisible()
		}
	case EventMouseDrag:
		gv, okv := s.vscrollGeom()
		s.sbV.drag(gv, okv, ev, s.scrollRowTo)
		gh, okh := s.hscrollGeom()
		s.sbH.drag(gh, okh, ev, s.scrollColTo)
	case EventMouseUp:
		s.sbV.release()
		s.sbH.release()
	}
}

// handleKey moves the active cell (arrows) or opens an editor (Enter / F2),
// reached only when no editor is open.
func (s *Spreadsheet) handleKey(ev Event) {
	switch ev.Code {
	case "ArrowUp":
		s.moveCursor(0, -1)
	case "ArrowDown":
		s.moveCursor(0, 1)
	case "ArrowLeft":
		s.moveCursor(-1, 0)
	case "ArrowRight":
		s.moveCursor(1, 0)
	case "Enter", "F2":
		s.BeginEdit()
	}
}

// cellAt maps a widget-local point to the cell under it. ok is false for a
// point on a header band, on a scrollbar gutter, or past the last cell.
func (s *Spreadsheet) cellAt(x, y int) (formula.Ref, bool) {
	if x < spRowHdrW() || y < spHeaderH() {
		return formula.Ref{}, false
	}
	g := s.gridRect()
	gx, gy := x-spRowHdrW(), y-spHeaderH()
	if gx >= g.W || gy >= g.H {
		return formula.Ref{}, false
	}
	ref := formula.Ref{Col: s.scrollCol + gx/spColW(), Row: s.scrollRow + gy/spRowH()}
	if !s.model.InBounds(ref) {
		return formula.Ref{}, false
	}
	return ref, true
}

// moveCursor shifts the active cell by (dCol, dRow), clamped inside the sheet,
// and scrolls to keep it visible.
func (s *Spreadsheet) moveCursor(dCol, dRow int) {
	col, row := s.cur.Col+dCol, s.cur.Row+dRow
	if col < 0 {
		col = 0
	}
	if col >= s.model.Cols() {
		col = s.model.Cols() - 1
	}
	if row < 0 {
		row = 0
	}
	if row >= s.model.Rows() {
		row = s.model.Rows() - 1
	}
	s.cur = formula.Ref{Col: col, Row: row}
	s.ensureVisible()
}

// BeginEdit opens an inline editor over the active cell, seeded with its current
// raw text — the command entry point Enter / F2 and a view model use.
func (s *Spreadsheet) BeginEdit() { s.openEditor(s.model.Raw(s.cur)) }

// beginEditWith opens an editor seeded with a single typed character, so typing
// over a cell replaces its contents (the spreadsheet type-to-edit gesture).
func (s *Spreadsheet) beginEditWith(seed string) { s.openEditor(seed) }

// openEditor is the shared editor-opening path. It is a no-op if the active
// cell is out of bounds (an empty 0-sized sheet).
func (s *Spreadsheet) openEditor(seed string) {
	if !s.model.InBounds(s.cur) {
		return
	}
	e := NewEntry(seed)
	e.Font = s.Font
	e.SetFocused(true)
	s.editor = e
}

// CommitEdit stores the open editor's text into the active cell (recomputing
// dependents), fires OnCellChange, and closes the editor. A no-op when no edit
// is open.
func (s *Spreadsheet) CommitEdit() { s.commitEdit() }

func (s *Spreadsheet) commitEdit() {
	if s.editor == nil {
		return
	}
	raw := s.editor.Text().Get()
	s.editor = nil
	s.model.SetCell(s.cur, raw)
	if s.OnCellChange != nil {
		s.OnCellChange(s.cur, raw)
	}
}

// CancelEdit discards the open editor without touching the cell. A no-op when no
// edit is open.
func (s *Spreadsheet) CancelEdit() { s.cancelEdit() }

func (s *Spreadsheet) cancelEdit() { s.editor = nil }
