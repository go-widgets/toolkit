// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit/textdiff"
)

// DiffView compares two texts line by line (textdiff: the shortest edit
// script), side by side or inline, the way a code review or an editor's
// "compare with" shows it:
//
//   - side by side: the old text on the left, the new one on the right, the
//     unchanged lines facing each other and each block of changes paired up;
//   - inline: one column, the removed lines (-) above the added ones (+);
//   - line numbers of both texts, and long runs of unchanged lines folded
//     into one "... N" row (Context lines kept around changes);
//   - an overview ruler at the right edge marks where the changes are in the
//     whole text and where the view is;
//   - keyboard: ↑/↓ a line, PageUp/PageDown a page, Home/End, F7 / Shift+F7
//     (and n / p) the next and previous change, which is outlined; the wheel
//     scrolls.
//
// Colours follow the theme: on a light Surface the rows are pale green and
// pale red under dark ink, on a dark Surface deep green and deep red under
// light ink. Every pairing keeps text at 4.5:1 or more and the change markers
// (the +/– gutter and the ruler) at 3:1 or more against what they sit on, in
// both DefaultLight and DefaultDark (diffview_test.go measures it). The +/–
// marks carry the meaning too, so the view reads without colour.
type DiffView struct {
	Base
	focusState

	// OldTitle and NewTitle head the two columns (side by side) or the view
	// ("old → new", inline). Empty: no header row.
	OldTitle, NewTitle string
	// SideBySide chooses the layout: two columns (true) or one (false).
	SideBySide bool
	// Context is how many unchanged lines stay shown around each change;
	// longer runs fold. Negative: never fold.
	Context int

	old, new []string
	edits    []textdiff.Edit
	rows     []dvRow
	changes  []int // the row index of the first row of each change block
	top      int   // first row shown
	current  int   // index into changes, -1 before the first move
}

// dvRow is one painted row. kind: dvEqual, dvDelete, dvInsert, dvChange
// (both sides, side by side), dvFold (n unchanged lines hidden), dvHeader.
type dvRow struct {
	kind dvKind
	a, b int // line indexes, -1 for none
	n    int // folded lines
}

type dvKind uint8

const (
	dvEqual dvKind = iota
	dvDelete
	dvInsert
	dvChange
	dvFold
)

// NewDiffView returns a side-by-side view comparing old with new, keeping 3
// lines of context.
func NewDiffView(old, new string) *DiffView {
	v := &DiffView{SideBySide: true, Context: 3}
	v.SetTexts(old, new)
	return v
}

// SetTexts compares two new texts; the view goes back to the top.
func (v *DiffView) SetTexts(old, new string) {
	v.old, v.new = textdiff.Split(old), textdiff.Split(new)
	v.edits = textdiff.Lines(v.old, v.new)
	v.top, v.current = 0, -1
	v.layout()
}

// SetLayout switches between side by side and inline, keeping the view on
// the same change.
func (v *DiffView) SetLayout(sideBySide bool) {
	if sideBySide == v.SideBySide {
		return
	}
	v.SideBySide = sideBySide
	cur := v.current
	v.layout()
	v.current = min(cur, len(v.changes)-1)
	if v.current >= 0 {
		v.top = v.changes[v.current]
	}
	v.clampTop()
}

// Stats counts the lines added and removed.
func (v *DiffView) Stats() (added, removed int) { return textdiff.Stats(v.edits) }

// Changes is the number of change blocks.
func (v *DiffView) Changes() int { return len(v.changes) }

// Current is the change the keyboard moved to (-1: none yet).
func (v *DiffView) Current() int { return v.current }

// Top is the first row shown.
func (v *DiffView) Top() int { return v.top }

// Rows is the number of rows the comparison takes (folds count as one).
func (v *DiffView) Rows() int { return len(v.rows) }

// layout builds the rows from the script.
func (v *DiffView) layout() {
	v.rows, v.changes = v.rows[:0], v.changes[:0]
	var rows []dvRow
	if v.SideBySide {
		for _, r := range textdiff.SideBySide(v.edits) {
			k := dvEqual
			switch {
			case r.Changed():
				k = dvChange
			case r.Op == textdiff.Delete:
				k = dvDelete
			case r.Op == textdiff.Insert:
				k = dvInsert
			}
			rows = append(rows, dvRow{kind: k, a: r.A, b: r.B})
		}
	} else {
		// the script has each run's removed lines before its added ones
		for _, e := range v.edits {
			switch e.Op {
			case textdiff.Equal:
				rows = append(rows, dvRow{kind: dvEqual, a: e.A, b: e.B})
			case textdiff.Delete:
				rows = append(rows, dvRow{kind: dvDelete, a: e.A, b: -1})
			case textdiff.Insert:
				rows = append(rows, dvRow{kind: dvInsert, a: -1, b: e.B})
			}
		}
	}
	// fold the long runs of unchanged rows
	for i := 0; i < len(rows); {
		if rows[i].kind != dvEqual {
			if i == 0 || rows[i-1].kind == dvEqual {
				v.changes = append(v.changes, len(v.rows))
			}
			v.rows = append(v.rows, rows[i])
			i++
			continue
		}
		j := i
		for j < len(rows) && rows[j].kind == dvEqual {
			j++
		}
		keepHead, keepTail := v.Context, v.Context
		if i == 0 {
			keepHead = 0 // nothing above to give context to
		}
		if j == len(rows) {
			keepTail = 0
		}
		if v.Context < 0 || j-i <= keepHead+keepTail+1 {
			v.rows = append(v.rows, rows[i:j]...)
		} else {
			v.rows = append(v.rows, rows[i:i+keepHead]...)
			v.rows = append(v.rows, dvRow{kind: dvFold, a: -1, b: -1, n: j - i - keepHead - keepTail})
			v.rows = append(v.rows, rows[j-keepTail:j]...)
		}
		i = j
	}
}

// rowH is a row's height.
func (v *DiffView) rowH() int { return v.glyphHeight() + scaled(4) }

func (v *DiffView) headerH() int {
	if v.OldTitle == "" && v.NewTitle == "" {
		return 0
	}
	return v.rowH()
}

// visible is the number of whole rows that fit.
func (v *DiffView) visible() int {
	return max((v.Bounds().H-v.headerH())/v.rowH(), 1)
}

func (v *DiffView) clampTop() {
	v.top = max(min(v.top, len(v.rows)-v.visible()), 0)
}

// ScrollBy moves the view n rows (negative: up).
func (v *DiffView) ScrollBy(n int) {
	v.top += n
	v.clampTop()
}

// NextChange moves to the next change block (dir 1) or the previous one
// (dir -1), wrapping around, and brings it into view. It reports whether
// there was one.
func (v *DiffView) NextChange(dir int) bool {
	if len(v.changes) == 0 {
		return false
	}
	switch {
	case v.current < 0 && dir < 0:
		v.current = len(v.changes) - 1
	case v.current < 0:
		v.current = 0
	default:
		v.current = (v.current + dir + len(v.changes)) % len(v.changes)
	}
	// a change a little below the top, so the context above it shows
	v.top = v.changes[v.current] - min(v.Context, v.visible()/3)
	v.clampTop()
	return true
}

// OnEvent scrolls and moves between changes.
func (v *DiffView) OnEvent(ev Event) {
	switch ev.Kind {
	case EventScroll:
		v.ScrollBy(ev.Delta)
	case EventClick:
		v.SetFocused(true)
	case EventKeyDown:
		switch ev.Code {
		case "ArrowDown":
			v.ScrollBy(1)
		case "ArrowUp":
			v.ScrollBy(-1)
		case "PageDown", " ", "Space":
			v.ScrollBy(v.visible() - 1)
		case "PageUp":
			v.ScrollBy(-(v.visible() - 1))
		case "Home":
			v.top = 0
		case "End":
			v.top = len(v.rows)
			v.clampTop()
		case "F7":
			if ev.Shift {
				v.NextChange(-1)
			} else {
				v.NextChange(1)
			}
		case "n", "N":
			v.NextChange(1)
		case "p", "P":
			v.NextChange(-1)
		}
	}
}

// The palette of the rows, by theme darkness. Light: pale fills under dark
// ink; dark: deep fills under light ink. Markers are the saturated tone.
type dvColors struct {
	addFill, addInk, addMark RGBA
	delFill, delInk, delMark RGBA
	foldInk, numInk          RGBA
}

func diffColors(theme *Theme) dvColors {
	if isDarkTheme(theme) {
		return dvColors{
			addFill: RGB(0x16, 0x3A, 0x24), addInk: RGB(0xC8, 0xF0, 0xD2), addMark: RGB(0x4C, 0xC3, 0x6E),
			delFill: RGB(0x4A, 0x1E, 0x22), delInk: RGB(0xF6, 0xD0, 0xD3), delMark: RGB(0xF0, 0x6A, 0x74),
			foldInk: RGB(0xA8, 0xAD, 0xB8), numInk: RGB(0x9A, 0xA0, 0xAC),
		}
	}
	return dvColors{
		addFill: RGB(0xD9, 0xF5, 0xDF), addInk: RGB(0x0E, 0x3B, 0x1A), addMark: RGB(0x1A, 0x7F, 0x37),
		delFill: RGB(0xFB, 0xDF, 0xE1), delInk: RGB(0x5C, 0x0F, 0x16), delMark: RGB(0xB4, 0x23, 0x2F),
		foldInk: RGB(0x4A, 0x4F, 0x57), numInk: RGB(0x55, 0x5B, 0x63),
	}
}

// Draw paints the header, the rows, the ruler and the focus ring.
func (v *DiffView) Draw(p painter.Painter, theme *Theme) {
	r := v.Bounds()
	c := diffColors(theme)
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	ruler := scaled(6)
	body := Rect{X: r.X, Y: r.Y, W: r.W - ruler, H: r.H}
	rh := v.rowH()
	// the line-number column fits the widest number (digits are not all as
	// wide as the advance in a proportional face: measure)
	numW := v.textWidth(strings.Repeat("8", len(strconv.Itoa(max(len(v.old), len(v.new), 1))))) + scaled(10)
	withClip(p, body, func() {
		y := r.Y
		if h := v.headerH(); h > 0 {
			fillRect(p, body.X, y, body.W, h, theme.SurfaceAlt)
			// the old title over the left half, the new over the right, in
			// both layouts (inline: where the old and the new line numbers are)
			// each clipped to its half: a long title does not run under the other
			ty := y + (h-v.glyphHeight())/2
			half := body.W / 2
			withClip(p, Rect{X: body.X, Y: y, W: half - scaled(4), H: h}, func() {
				v.drawText(p, body.X+scaled(4), ty, v.OldTitle, theme.OnSurface)
			})
			withClip(p, Rect{X: body.X + half, Y: y, W: body.W - half, H: h}, func() {
				v.drawText(p, body.X+half+scaled(4), ty, v.NewTitle, theme.OnSurface)
			})
			y += h
		}
		end := min(v.top+v.visible()+1, len(v.rows))
		cur := -1
		if v.current >= 0 {
			cur = v.changes[v.current]
		}
		for i := v.top; i < end; i++ {
			row := v.rows[i]
			if v.SideBySide {
				// each side clipped to its column: a long line stops at the middle
				half := body.W / 2
				lc, rc := Rect{X: body.X, Y: y, W: half, H: rh}, Rect{X: body.X + half, Y: y, W: body.W - half, H: rh}
				withClip(p, lc, func() { v.drawSide(p, theme, c, lc, row, true, numW) })
				withClip(p, rc, func() { v.drawSide(p, theme, c, rc, row, false, numW) })
				fillRect(p, body.X+half, y, 1, rh, theme.Border)
			} else {
				v.drawInline(p, theme, c, Rect{X: body.X, Y: y, W: body.W, H: rh}, row, numW)
			}
			if i == cur {
				// the change the keyboard moved to
				strokeRect(p, body.X+1, y, body.W-2, v.blockRows(i)*rh, theme.Accent)
			}
			y += rh
		}
	})
	v.drawRuler(p, theme, c, Rect{X: r.X + r.W - ruler, Y: r.Y, W: ruler, H: r.H})
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	v.drawFocusRing(p, theme, r)
}

// blockRows is how many rows the change block starting at row i takes.
func (v *DiffView) blockRows(i int) int {
	n := 0
	for i+n < len(v.rows) && v.rows[i+n].kind != dvEqual && v.rows[i+n].kind != dvFold {
		n++
	}
	return max(n, 1)
}

// drawSide paints one column of a side-by-side row.
func (v *DiffView) drawSide(p painter.Painter, theme *Theme, c dvColors, cell Rect, row dvRow, left bool, numW int) {
	ty := cell.Y + (cell.H-v.glyphHeight())/2
	if row.kind == dvFold {
		v.drawText(p, cell.X+numW+scaled(4), ty, "... "+strconv.Itoa(row.n), c.foldInk)
		return
	}
	line, text := row.b, ""
	if left {
		line = row.a
	}
	if line < 0 {
		// a blank facing a change: hatch it lightly so it reads as absent
		fillRect(p, cell.X, cell.Y, cell.W, cell.H, theme.SurfaceAlt)
		return
	}
	fill, ink, mark, sign := theme.Surface, theme.OnSurface, RGBA{}, ""
	if row.kind != dvEqual {
		if left {
			fill, ink, mark, sign = c.delFill, c.delInk, c.delMark, "-"
		} else {
			fill, ink, mark, sign = c.addFill, c.addInk, c.addMark, "+"
		}
	}
	if left {
		text = v.old[line]
	} else {
		text = v.new[line]
	}
	fillRect(p, cell.X, cell.Y, cell.W, cell.H, fill)
	n := strconv.Itoa(line + 1)
	v.drawText(p, cell.X+numW-scaled(4)-v.textWidth(n), ty, n, c.numInk)
	if sign != "" {
		fillRect(p, cell.X+numW-scaled(2), cell.Y, scaled(2), cell.H, mark)
		v.drawText(p, cell.X+numW+scaled(2), ty, sign, mark)
	}
	v.drawText(p, cell.X+numW+scaled(2)+v.glyphAdvance()*2, ty, text, ink)
}

// drawInline paints a row of the one-column layout: both line numbers, the
// sign, the text.
func (v *DiffView) drawInline(p painter.Painter, theme *Theme, c dvColors, cell Rect, row dvRow, numW int) {
	ty := cell.Y + (cell.H-v.glyphHeight())/2
	gutter := 2 * numW
	if row.kind == dvFold {
		v.drawText(p, cell.X+gutter+scaled(4), ty, "... "+strconv.Itoa(row.n), c.foldInk)
		return
	}
	fill, ink, mark, sign, text := theme.Surface, theme.OnSurface, RGBA{}, "", ""
	switch row.kind {
	case dvDelete:
		fill, ink, mark, sign, text = c.delFill, c.delInk, c.delMark, "-", v.old[row.a]
	case dvInsert:
		fill, ink, mark, sign, text = c.addFill, c.addInk, c.addMark, "+", v.new[row.b]
	default:
		text = v.old[row.a]
	}
	fillRect(p, cell.X, cell.Y, cell.W, cell.H, fill)
	if row.a >= 0 {
		n := strconv.Itoa(row.a + 1)
		v.drawText(p, cell.X+numW-scaled(4)-v.textWidth(n), ty, n, c.numInk)
	}
	if row.b >= 0 {
		n := strconv.Itoa(row.b + 1)
		v.drawText(p, cell.X+gutter-scaled(4)-v.textWidth(n), ty, n, c.numInk)
	}
	if sign != "" {
		fillRect(p, cell.X+gutter-scaled(2), cell.Y, scaled(2), cell.H, mark)
		v.drawText(p, cell.X+gutter+scaled(2), ty, sign, mark)
	}
	v.drawText(p, cell.X+gutter+scaled(2)+v.glyphAdvance()*2, ty, text, ink)
}

// drawRuler marks the changes along the whole text, and the rows in view.
func (v *DiffView) drawRuler(p painter.Painter, theme *Theme, c dvColors, r Rect) {
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	n := len(v.rows)
	if n == 0 {
		return
	}
	at := func(i int) int { return r.Y + i*r.H/n }
	for i, row := range v.rows {
		var col RGBA
		switch row.kind {
		case dvDelete:
			col = c.delMark
		case dvInsert:
			col = c.addMark
		case dvChange:
			col = c.addMark
			fillRect(p, r.X, at(i), r.W/2, max(at(i+1)-at(i), 2), c.delMark)
			fillRect(p, r.X+r.W/2, at(i), r.W-r.W/2, max(at(i+1)-at(i), 2), col)
			continue
		default:
			continue
		}
		fillRect(p, r.X, at(i), r.W, max(at(i+1)-at(i), 2), col)
	}
	// the window: a frame over the rows in view
	strokeRect(p, r.X, at(v.top), r.W, max(at(min(v.top+v.visible(), n))-at(v.top), 2), theme.OnSurface)
}

// A11y describes the view: a document, named by its titles, valued by the
// counts.
func (v *DiffView) A11y() A11yInfo {
	add, del := v.Stats()
	return A11yInfo{Role: RoleDocument, Name: v.OldTitle + " → " + v.NewTitle,
		Value: "+" + strconv.Itoa(add) + " −" + strconv.Itoa(del) + ", " + strconv.Itoa(len(v.changes)) + " changes"}
}
