// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TermCell is one character cell in a TerminalView grid: a single
// rune plus its foreground and background colours. A zero Rune (or a
// space) paints no glyph — only the background fill. A colour left at
// its zero value (RGBA with A==0) is "unset" and falls back at paint
// time to the view's DefaultFG / DefaultBG (and, failing those, to the
// theme). Keeping colour per cell is what a console needs for coloured
// prompts, error text and ANSI-styled output.
type TermCell struct {
	Rune   rune
	FG, BG RGBA
}

// TerminalView is a fixed Cols×Rows grid of character cells — the
// reusable building block a terminal, console or REPL renders into. It
// owns the cell buffer, a block cursor and the scroll/wrap bookkeeping
// a shell needs; it does NOT parse escape sequences or run a PTY (that
// belongs to the host). SetBounds derives the cell size from the
// active font's metrics, Draw blits each cell (background fill + glyph
// in the foreground), and OnEvent forwards input to OnKey so the host
// can drive a shell.
type TerminalView struct {
	Base

	// Cols and Rows are the grid dimensions. Cells is row-major: the
	// cell at (col, row) lives at Cells[row*Cols+col].
	Cols, Rows int
	Cells      []TermCell

	// CursorCol / CursorRow are the block cursor's cell position, and
	// the next write position for Write / Put. CursorVisible gates
	// whether Draw paints the (inverted) block cursor.
	CursorCol, CursorRow int
	CursorVisible        bool

	// DefaultFG / DefaultBG are the pen colours Write / Put stamp into
	// new cells and the fallback Draw uses for cells whose own colour is
	// unset. When they too are unset (A==0) Draw falls back to the
	// theme's OnSurface / Surface.
	DefaultFG, DefaultBG RGBA

	// OnKey, when non-nil, receives every event delivered to OnEvent
	// (key down/up, char, click, …) so the host can feed a shell.
	OnKey func(ev Event)

	// cellW / cellH are the per-cell pixel size, recomputed from the
	// active font each SetBounds. Exposed read-only via CellWidth /
	// CellHeight for hosts that map pixel coordinates back to cells.
	cellW, cellH int
}

// NewTerminalView allocates a blank cols×rows grid with the cursor
// homed at (0, 0). A non-positive dimension panics: a zero-sized grid
// cannot render usefully and a silent fallback would hide the bug.
func NewTerminalView(cols, rows int) *TerminalView {
	if cols <= 0 || rows <= 0 {
		panic("toolkit: NewTerminalView requires positive cols + rows")
	}
	return &TerminalView{
		Cols:  cols,
		Rows:  rows,
		Cells: make([]TermCell, cols*rows),
	}
}

// Resize reshapes the grid to newCols×newRows, preserving the top-left
// rectangle that still fits and blanking any newly exposed cells. The
// cursor is clamped into the new bounds. A non-positive dimension
// panics, matching NewTerminalView.
func (t *TerminalView) Resize(newCols, newRows int) {
	if newCols <= 0 || newRows <= 0 {
		panic("toolkit: Resize requires positive cols + rows")
	}
	out := make([]TermCell, newCols*newRows)
	copyCols := newCols
	if t.Cols < copyCols {
		copyCols = t.Cols
	}
	copyRows := newRows
	if t.Rows < copyRows {
		copyRows = t.Rows
	}
	for r := 0; r < copyRows; r++ {
		for c := 0; c < copyCols; c++ {
			out[r*newCols+c] = t.Cells[r*t.Cols+c]
		}
	}
	t.Cells = out
	t.Cols = newCols
	t.Rows = newRows
	if t.CursorCol >= t.Cols {
		t.CursorCol = t.Cols - 1
	}
	if t.CursorRow >= t.Rows {
		t.CursorRow = t.Rows - 1
	}
}

// inRange reports whether (col, row) addresses a real cell.
func (t *TerminalView) inRange(col, row int) bool {
	return col >= 0 && col < t.Cols && row >= 0 && row < t.Rows
}

// SetCell writes ru with explicit fg/bg at (col, row). An out-of-range
// position is a no-op, so callers need not bounds-check every write.
func (t *TerminalView) SetCell(col, row int, ru rune, fg, bg RGBA) {
	if !t.inRange(col, row) {
		return
	}
	t.Cells[row*t.Cols+col] = TermCell{Rune: ru, FG: fg, BG: bg}
}

// Put writes ru at (col, row) using the view's pen colours
// (DefaultFG / DefaultBG). It is the convenience form of SetCell for
// output that shares one colour. Out-of-range is a no-op.
func (t *TerminalView) Put(col, row int, ru rune) {
	t.SetCell(col, row, ru, t.DefaultFG, t.DefaultBG)
}

// Cell returns the cell at (col, row), or a zero TermCell when the
// position is out of range.
func (t *TerminalView) Cell(col, row int) TermCell {
	if !t.inRange(col, row) {
		return TermCell{}
	}
	return t.Cells[row*t.Cols+col]
}

// Write stamps s at the cursor using the pen colours, advancing the
// cursor cell by cell. '\n' returns the cursor to column 0 of the next
// row (scrolling when it overflows the bottom); '\r' returns it to
// column 0 of the current row; every other rune is written and the
// cursor advances, wrapping to the next row at end-of-line. It is the
// terminal-style output helper a shell's stdout feeds.
func (t *TerminalView) Write(s string) {
	for _, ru := range s {
		switch ru {
		case '\r':
			t.CursorCol = 0
		case '\n':
			t.newline()
		default:
			t.Cells[t.CursorRow*t.Cols+t.CursorCol] = TermCell{Rune: ru, FG: t.DefaultFG, BG: t.DefaultBG}
			t.CursorCol++
			if t.CursorCol >= t.Cols {
				t.newline()
			}
		}
	}
}

// newline moves the cursor to column 0 of the next row, scrolling one
// row up when it would leave the bottom.
func (t *TerminalView) newline() {
	t.CursorCol = 0
	t.CursorRow++
	if t.CursorRow >= t.Rows {
		t.ScrollUp(1)
		t.CursorRow = t.Rows - 1
	}
}

// ScrollUp shifts the grid up by n rows: the top n rows are discarded,
// the rest move up, and the bottom n rows are blanked. n<=0 is a
// no-op; n>=Rows clears the whole grid.
func (t *TerminalView) ScrollUp(n int) {
	if n <= 0 {
		return
	}
	if n >= t.Rows {
		for i := range t.Cells {
			t.Cells[i] = TermCell{}
		}
		return
	}
	copy(t.Cells, t.Cells[n*t.Cols:])
	tail := t.Cells[(t.Rows-n)*t.Cols:]
	for i := range tail {
		tail[i] = TermCell{}
	}
}

// SetBounds records the placement and recomputes the per-cell pixel
// size from the active font's metrics (advance × height), so the grid
// tracks a font change.
func (t *TerminalView) SetBounds(r Rect) {
	t.Base.SetBounds(r)
	t.cellW = t.glyphAdvance()
	t.cellH = t.glyphHeight()
}

// CellWidth is the pixel width of one cell (0 before SetBounds).
func (t *TerminalView) CellWidth() int { return t.cellW }

// CellHeight is the pixel height of one cell (0 before SetBounds).
func (t *TerminalView) CellHeight() int { return t.cellH }

// Draw blits the grid: it fills the bounds with the default background,
// then paints each cell's background and glyph. When CursorVisible the
// cursor cell is drawn inverted (foreground/background swapped), which
// reads as a block cursor over both empty and occupied cells. Draw is
// a no-op until SetBounds has established a positive cell size.
func (t *TerminalView) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if t.cellW <= 0 || t.cellH <= 0 {
		return
	}
	defFG := t.DefaultFG
	if defFG.A == 0 {
		defFG = theme.OnSurface
	}
	defBG := t.DefaultBG
	if defBG.A == 0 {
		defBG = theme.Surface
	}
	fillRect(p, r.X, r.Y, r.W, r.H, defBG)
	for row := 0; row < t.Rows; row++ {
		for col := 0; col < t.Cols; col++ {
			cell := t.Cells[row*t.Cols+col]
			fg := cell.FG
			if fg.A == 0 {
				fg = defFG
			}
			bg := cell.BG
			if bg.A == 0 {
				bg = defBG
			}
			if t.CursorVisible && col == t.CursorCol && row == t.CursorRow {
				fg, bg = bg, fg
			}
			x := r.X + col*t.cellW
			y := r.Y + row*t.cellH
			fillRect(p, x, y, t.cellW, t.cellH, bg)
			if cell.Rune != 0 && cell.Rune != ' ' {
				t.drawText(p, x, y, string(cell.Rune), fg)
			}
		}
	}
}

// OnEvent forwards the event to OnKey when one is set, letting the host
// drive a shell from the grid's key and character input.
func (t *TerminalView) OnEvent(ev Event) {
	if t.OnKey != nil {
		t.OnKey(ev)
	}
}
