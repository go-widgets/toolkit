// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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

	// cursorCol / cursorRow are the block cursor's cell position (and the
	// next write position for Write / Put); cursorVisible gates whether Draw
	// paints the (inverted) block cursor. All three are reactive state behind
	// the CursorCol() / CursorRow() / CursorVisible() accessors — the emulator's
	// own write-head over its grid document, but exposed through MVVM so a host
	// reads/drives it the same way as any other widget.
	cursorCol, cursorRow *mvvm.Observable[int]
	cursorVisible        *mvvm.Observable[bool]

	// DefaultFG / DefaultBG are the pen colours Write / Put stamp into
	// new cells and the fallback Draw uses for cells whose own colour is
	// unset. When they too are unset (A==0) Draw falls back to the
	// theme's OnSurface / Surface.
	DefaultFG, DefaultBG RGBA

	// OnKey, when non-nil, receives every event delivered to OnEvent
	// (key down/up, char, click, …) so the host can feed a shell.
	OnKey func(ev Event)

	// CellW / CellH, when positive, pin an explicit per-cell pixel size
	// that OVERRIDES the font-derived metrics — the escape hatch a host
	// needs to lock a fixed cell geometry (e.g. a terminal that sizes its
	// grid to an exact character box) without swapping in a metrics-only
	// font shim. A zero value on an axis means "derive that axis from the
	// active font", so leaving both unset reproduces the original
	// font-metric behaviour exactly. Set them via SetCellSize (which
	// recomputes immediately when bounds are already established) or by
	// assigning the fields before the first SetBounds.
	CellW, CellH int

	// cellW / cellH are the resolved per-cell pixel size — the CellW /
	// CellH override when set, otherwise the active font's metrics —
	// recomputed each SetBounds (and each SetCellSize once bounded).
	// Exposed read-only via CellWidth / CellHeight for hosts that map
	// pixel coordinates back to cells.
	cellW, cellH int

	// bounded records that SetBounds has run at least once, so a later
	// SetCellSize can recompute the resolved size immediately while Draw
	// stays a no-op until the placement is known.
	bounded bool
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

// CursorCol is the block cursor's column (and the next write column) as a shared
// [mvvm.Observable]; Write/Put advance it and Draw reads it. Lazily created,
// defaulting to 0.
func (t *TerminalView) CursorCol() *mvvm.Observable[int] {
	if t.cursorCol == nil {
		t.cursorCol = mvvm.NewObservable(0)
	}
	return t.cursorCol
}

// CursorRow is the block cursor's row (and the next write row) as a shared
// [mvvm.Observable]. Lazily created, defaulting to 0.
func (t *TerminalView) CursorRow() *mvvm.Observable[int] {
	if t.cursorRow == nil {
		t.cursorRow = mvvm.NewObservable(0)
	}
	return t.cursorRow
}

// CursorVisible gates whether Draw paints the inverted block cursor, as a shared
// [mvvm.Observable]. Lazily created, defaulting to false.
func (t *TerminalView) CursorVisible() *mvvm.Observable[bool] {
	if t.cursorVisible == nil {
		t.cursorVisible = mvvm.NewObservable(false)
	}
	return t.cursorVisible
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
	if t.CursorCol().Get() >= t.Cols {
		t.CursorCol().Set(t.Cols - 1)
	}
	if t.CursorRow().Get() >= t.Rows {
		t.CursorRow().Set(t.Rows - 1)
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
			t.CursorCol().Set(0)
		case '\n':
			t.newline()
		default:
			t.Cells[t.CursorRow().Get()*t.Cols+t.CursorCol().Get()] = TermCell{Rune: ru, FG: t.DefaultFG, BG: t.DefaultBG}
			t.CursorCol().Set(t.CursorCol().Get() + 1)
			if t.CursorCol().Get() >= t.Cols {
				t.newline()
			}
		}
	}
}

// newline moves the cursor to column 0 of the next row, scrolling one
// row up when it would leave the bottom.
func (t *TerminalView) newline() {
	t.CursorCol().Set(0)
	t.CursorRow().Set(t.CursorRow().Get() + 1)
	if t.CursorRow().Get() >= t.Rows {
		t.ScrollUp(1)
		t.CursorRow().Set(t.Rows - 1)
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
// size: the CellW / CellH override on each axis where it is positive,
// otherwise the active font's metrics (advance × height), so the grid
// tracks a font change while honouring any pinned geometry.
func (t *TerminalView) SetBounds(r Rect) {
	t.Base.SetBounds(r)
	t.bounded = true
	t.cellW = t.resolveCellW()
	t.cellH = t.resolveCellH()
}

// SetCellSize pins an explicit per-cell pixel size, overriding the
// font-derived metrics. A non-positive value on an axis clears that
// axis's override, restoring derivation from the active font. When
// bounds are already established the resolved size updates immediately
// (so CellWidth / CellHeight and the next Draw reflect it at once);
// otherwise it takes effect at the first SetBounds.
func (t *TerminalView) SetCellSize(w, h int) {
	t.CellW = w
	t.CellH = h
	if t.bounded {
		t.cellW = t.resolveCellW()
		t.cellH = t.resolveCellH()
	}
}

// resolveCellW is the effective cell width: the CellW override when
// positive, else the active font's advance.
func (t *TerminalView) resolveCellW() int {
	if t.CellW > 0 {
		return t.CellW
	}
	return t.glyphAdvance()
}

// resolveCellH is the effective cell height: the CellH override when
// positive, else the active font's glyph-box height.
func (t *TerminalView) resolveCellH() int {
	if t.CellH > 0 {
		return t.CellH
	}
	return t.glyphHeight()
}

// CellWidth is the pixel width of one cell (0 before SetBounds).
func (t *TerminalView) CellWidth() int { return t.cellW }

// CellHeight is the pixel height of one cell (0 before SetBounds).
func (t *TerminalView) CellHeight() int { return t.cellH }

// Draw blits the grid: a single span-fill clears the whole bounds to
// the default background (covering every default-background cell and any
// margin the grid does not tile), then each cell whose resolved
// background DIFFERS from that clear colour is over-filled with a rect
// span and every non-blank glyph is stamped. When CursorVisible the
// cursor cell is drawn inverted (foreground/background swapped), which
// reads as a block cursor over both empty and occupied cells.
//
// This is one background pass, not two: the earlier form redundantly
// span-filled every cell on top of an already-painted whole-bounds
// clear, doubling the background cost for the common all-default grid.
// Skipping cells that match the clear colour is pixel-identical — those
// pixels already hold that exact colour — while non-default cells still
// get their own span so the final image is unchanged. Draw is a no-op
// until SetBounds has established a positive cell size.
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
			if t.CursorVisible().Get() && col == t.CursorCol().Get() && row == t.CursorRow().Get() {
				fg, bg = bg, fg
			}
			x := r.X + col*t.cellW
			y := r.Y + row*t.cellH
			if bg != defBG {
				// Only cells that diverge from the clear colour need
				// their own span; the rest are already painted.
				fillRect(p, x, y, t.cellW, t.cellH, bg)
			}
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
