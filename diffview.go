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
//   - in a changed line, the words that changed are marked (a diff of the
//     line pair's words, textdiff again): a one-word change reads at once;
//   - long lines wrap, both sides of a row kept level (the taller side sets
//     the row's height): nothing is cut at the column's edge, so a change at
//     column 80 of a narrow column shows as well as one at column 1;
//   - line numbers of both texts, and long runs of unchanged lines folded
//     into one "... N" row (Context lines kept around changes);
//   - an overview ruler at the right edge marks where the changes are in the
//     whole text and where the view is;
//   - keyboard: ↑/↓ a row, PageUp/PageDown a page, Home/End, F7 / Shift+F7
//     (and n / p) the next and previous change, which is outlined; the wheel
//     scrolls.
//
// Wrapping rather than scrolling sideways: a side scroll hides what is past
// the edge until someone looks for it, and a reader cannot look for a change
// they do not know is there; wrapped, every character of every shown row is
// on screen, and the rows still face each other.
//
// Colours follow the theme: on a light Surface the rows are pale green and
// pale red under dark ink, on a dark Surface deep green and deep red under
// light ink, the changed words on a stronger tone of the same. Every pairing
// keeps text at 4.5:1 or more and the change markers (the +/- gutter and the
// ruler) at 3:1 or more against what they sit on, in both DefaultLight and
// DefaultDark (diffview_test.go measures it). The +/- marks carry the
// meaning too, so the view reads without colour.
type DiffView struct {
	Base
	focusState

	// OldTitle and NewTitle head the two columns (side by side) or the view
	// (inline: over the old and the new line numbers). Empty: no header row.
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

	// the rows' heights in lines, for the text width they were wrapped to
	wrapW   int
	heights []int
	runeW   map[rune]int
}

// dvRow is one painted row. kind: dvEqual, dvDelete, dvInsert, dvChange
// (both sides, side by side) or dvFold (n unchanged lines hidden). spanA and
// spanB are the byte ranges of the old and new line that changed within a
// changed pair (nil: none marked).
type dvRow struct {
	kind         dvKind
	a, b         int // line indexes, -1 for none
	n            int // folded lines
	spanA, spanB [][2]int
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
	v.rows, v.changes, v.wrapW = v.rows[:0], v.changes[:0], -1
	var rows []dvRow
	if v.SideBySide {
		for _, r := range textdiff.SideBySide(v.edits) {
			row := dvRow{kind: dvEqual, a: r.A, b: r.B}
			switch {
			case r.Changed():
				row.kind = dvChange
				row.spanA, row.spanB = WordChanges(v.old[r.A], v.new[r.B])
			case r.Op == textdiff.Delete:
				row.kind = dvDelete
			case r.Op == textdiff.Insert:
				row.kind = dvInsert
			}
			rows = append(rows, row)
		}
	} else {
		// the script has each run's removed lines before its added ones; the
		// k-th removed line of a run is paired with its k-th added line for
		// the words that changed, as side by side pairs them
		for i := 0; i < len(v.edits); {
			e := v.edits[i]
			if e.Op == textdiff.Equal {
				rows = append(rows, dvRow{kind: dvEqual, a: e.A, b: e.B})
				i++
				continue
			}
			var dels, ins []int
			for i < len(v.edits) && v.edits[i].Op != textdiff.Equal {
				if v.edits[i].Op == textdiff.Delete {
					dels = append(dels, v.edits[i].A)
				} else {
					ins = append(ins, v.edits[i].B)
				}
				i++
			}
			spans := make([][2][][2]int, min(len(dels), len(ins)))
			for k := range spans {
				spans[k][0], spans[k][1] = WordChanges(v.old[dels[k]], v.new[ins[k]])
			}
			for k, a := range dels {
				row := dvRow{kind: dvDelete, a: a, b: -1}
				if k < len(spans) {
					row.spanA = spans[k][0]
				}
				rows = append(rows, row)
			}
			for k, b := range ins {
				row := dvRow{kind: dvInsert, a: -1, b: b}
				if k < len(spans) {
					row.spanB = spans[k][1]
				}
				rows = append(rows, row)
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

// WordChanges marks what changed between two versions of a line: the byte
// ranges of a and of b that a diff of their words (textdiff over runs of
// letters and digits, runs of spaces, and single other characters) finds
// deleted and inserted, adjacent ranges joined. Lines over 4096 bytes are not
// split (nil, nil): the whole line is the change.
func WordChanges(a, b string) (inA, inB [][2]int) {
	if len(a) > 4096 || len(b) > 4096 {
		return nil, nil
	}
	ta, tb := words(a), words(b)
	sa, sb := make([]string, len(ta)), make([]string, len(tb))
	for i, t := range ta {
		sa[i] = a[t[0]:t[1]]
	}
	for i, t := range tb {
		sb[i] = b[t[0]:t[1]]
	}
	add := func(out [][2]int, r [2]int) [][2]int {
		if n := len(out); n > 0 && out[n-1][1] == r[0] {
			out[n-1][1] = r[1]
			return out
		}
		return append(out, r)
	}
	for _, e := range textdiff.Lines(sa, sb) {
		switch e.Op {
		case textdiff.Delete:
			inA = add(inA, ta[e.A])
		case textdiff.Insert:
			inB = add(inB, tb[e.B])
		}
	}
	return inA, inB
}

// words cuts a line into tokens: runs of letters and digits, runs of
// spaces, and every other character alone.
func words(s string) [][2]int {
	var out [][2]int
	class := func(r rune) int {
		switch {
		case r == ' ' || r == '\t':
			return 1
		case r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= 0x80:
			return 2
		}
		return 0
	}
	start, prev := 0, -1
	for i, r := range s {
		c := class(r)
		if i > start && (c != prev || c == 0) {
			out = append(out, [2]int{start, i})
			start = i
		}
		prev = c
	}
	if start < len(s) {
		out = append(out, [2]int{start, len(s)})
	}
	return out
}

// rowH is the height of one line of a row.
func (v *DiffView) rowH() int { return v.glyphHeight() + scaled(4) }

func (v *DiffView) headerH() int {
	if v.OldTitle == "" && v.NewTitle == "" {
		return 0
	}
	return v.rowH()
}

// numW is the line-number column's width: the widest number (digits are not
// all as wide as the advance in a proportional face: measure).
func (v *DiffView) numW() int {
	return v.textWidth(strings.Repeat("8", len(strconv.Itoa(max(len(v.old), len(v.new), 1))))) + scaled(10)
}

// geometry is where the text of a row starts in its cell, and how wide it
// may run, for the bounds as they are.
func (v *DiffView) geometry() (textX, textW int) {
	body := v.Bounds().W - scaled(6)
	if v.SideBySide {
		textX = v.numW() + scaled(2) + v.glyphAdvance()*2
		return textX, max(body/2-textX-scaled(4), v.glyphAdvance()*4)
	}
	textX = 2*v.numW() + scaled(2) + v.glyphAdvance()*2
	return textX, max(body-textX-scaled(4), v.glyphAdvance()*4)
}

// width measures s rune by rune (cached): what the wrap and the marks use.
func (v *DiffView) width(s string) int {
	if v.runeW == nil {
		v.runeW = map[rune]int{}
	}
	n := 0
	for _, r := range s {
		w, ok := v.runeW[r]
		if !ok {
			w = v.textWidth(string(r))
			v.runeW[r] = w
		}
		n += w
	}
	return n
}

// wrap cuts s into pieces no wider than w, at a space when one is near the
// end of the piece, else anywhere: byte ranges, at least one.
func (v *DiffView) wrap(s string, w int) [][2]int {
	var out [][2]int
	start, cur, lastSpace := 0, 0, -1
	for i, r := range s {
		rw := v.width(string(r))
		if cur+rw > w && i > start {
			cut := i
			if lastSpace > start {
				cut = lastSpace
			}
			out = append(out, [2]int{start, cut})
			start, cur, lastSpace = cut, v.width(s[cut:i]), -1
		}
		cur += rw
		if r == ' ' {
			lastSpace = i + 1
		}
	}
	return append(out, [2]int{start, len(s)})
}

// pieces are a side's wrapped lines (none for a blank side).
func (v *DiffView) pieces(row dvRow, left bool, w int) [][2]int {
	switch {
	case row.kind == dvFold:
		return [][2]int{{0, 0}}
	case left && row.a >= 0:
		return v.wrap(v.old[row.a], w)
	case !left && row.b >= 0:
		return v.wrap(v.new[row.b], w)
	}
	return nil
}

// ensureHeights wraps every row for the current width.
func (v *DiffView) ensureHeights() {
	_, w := v.geometry()
	if w == v.wrapW && len(v.heights) == len(v.rows) {
		return
	}
	v.wrapW = w
	v.heights = make([]int, len(v.rows))
	for i, row := range v.rows {
		if v.SideBySide {
			v.heights[i] = max(len(v.pieces(row, true, w)), len(v.pieces(row, false, w)), 1)
			continue
		}
		left := row.kind == dvDelete || (row.kind == dvEqual && row.a >= 0)
		v.heights[i] = max(len(v.pieces(row, left, w)), 1)
	}
}

// visible is the number of rows from the top that fit (at least one).
func (v *DiffView) visible() int {
	v.ensureHeights()
	room := (v.Bounds().H - v.headerH()) / v.rowH()
	n := 0
	for i := v.top; i < len(v.rows) && room >= v.heights[i]; i++ {
		room -= v.heights[i]
		n++
	}
	return max(n, 1)
}

// lastTop is the largest top that still fills the view.
func (v *DiffView) lastTop() int {
	v.ensureHeights()
	room := (v.Bounds().H - v.headerH()) / v.rowH()
	i := len(v.rows)
	for i > 0 && room >= v.heights[i-1] {
		room -= v.heights[i-1]
		i--
	}
	return min(i, max(len(v.rows)-1, 0))
}

func (v *DiffView) clampTop() {
	v.top = max(min(v.top, v.lastTop()), 0)
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
	v.top = v.changes[v.current]
	v.top -= min(v.Context, v.visible()/3)
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
			v.ScrollBy(max(v.visible()-1, 1))
		case "PageUp":
			v.ScrollBy(-max(v.visible()-1, 1))
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
// ink; dark: deep fills under light ink. Markers are the saturated tone; the
// changed words sit on a stronger fill of the row's colour.
type dvColors struct {
	addFill, addInk, addMark, addWord RGBA
	delFill, delInk, delMark, delWord RGBA
	foldInk, numInk                   RGBA
}

func diffColors(theme *Theme) dvColors {
	if isDarkTheme(theme) {
		return dvColors{
			addFill: RGB(0x16, 0x3A, 0x24), addInk: RGB(0xC8, 0xF0, 0xD2), addMark: RGB(0x4C, 0xC3, 0x6E), addWord: RGB(0x23, 0x6B, 0x3C),
			delFill: RGB(0x4A, 0x1E, 0x22), delInk: RGB(0xF6, 0xD0, 0xD3), delMark: RGB(0xF0, 0x6A, 0x74), delWord: RGB(0x86, 0x2C, 0x35),
			foldInk: RGB(0xA8, 0xAD, 0xB8), numInk: RGB(0x9A, 0xA0, 0xAC),
		}
	}
	return dvColors{
		addFill: RGB(0xD9, 0xF5, 0xDF), addInk: RGB(0x0E, 0x3B, 0x1A), addMark: RGB(0x1A, 0x7F, 0x37), addWord: RGB(0x9C, 0xE3, 0xAE),
		delFill: RGB(0xFB, 0xDF, 0xE1), delInk: RGB(0x5C, 0x0F, 0x16), delMark: RGB(0xB4, 0x23, 0x2F), delWord: RGB(0xF4, 0xA9, 0xB0),
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
	lh := v.rowH()
	numW := v.numW()
	v.ensureHeights()
	withClip(p, body, func() {
		y := r.Y
		if h := v.headerH(); h > 0 {
			fillRect(p, body.X, y, body.W, h, theme.SurfaceAlt)
			// the old title over the left half, the new over the right, in
			// both layouts (inline: where the old and the new line numbers are),
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
		cur := -1
		if v.current >= 0 {
			cur = v.changes[v.current]
		}
		for i := v.top; i < len(v.rows) && y < body.Y+body.H; i++ {
			row, h := v.rows[i], v.heights[i]*lh
			if v.SideBySide {
				// each side clipped to its column
				half := body.W / 2
				lc, rc := Rect{X: body.X, Y: y, W: half, H: h}, Rect{X: body.X + half, Y: y, W: body.W - half, H: h}
				withClip(p, lc, func() { v.drawSide(p, theme, c, lc, row, true, numW) })
				withClip(p, rc, func() { v.drawSide(p, theme, c, rc, row, false, numW) })
				fillRect(p, body.X+half, y, 1, h, theme.Border)
			} else {
				v.drawInline(p, theme, c, Rect{X: body.X, Y: y, W: body.W, H: h}, row, numW)
			}
			if i == cur {
				// the change the keyboard moved to
				bh := 0
				for k := i; k < i+v.blockRows(i); k++ {
					bh += v.heights[k] * lh
				}
				strokeRect(p, body.X+1, y, body.W-2, bh, theme.Accent)
			}
			y += h
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

// drawLine paints a line's wrapped pieces from (x, y), one line height
// apart, with the changed spans on word behind them.
func (v *DiffView) drawLine(p painter.Painter, x, y int, text string, spans [][2]int, ink, word RGBA) {
	lh := v.rowH()
	_, w := v.geometry()
	for k, pc := range v.wrap(text, w) {
		ly := y + k*lh
		for _, s := range spans {
			a, b := max(s[0], pc[0]), min(s[1], pc[1])
			if a < b {
				fillRect(p, x+v.width(text[pc[0]:a]), ly+scaled(1), max(v.width(text[a:b]), scaled(2)), lh-scaled(2), word)
			}
		}
		v.drawText(p, x, ly+(lh-v.glyphHeight())/2, text[pc[0]:pc[1]], ink)
	}
}

// drawSide paints one column of a side-by-side row.
func (v *DiffView) drawSide(p painter.Painter, theme *Theme, c dvColors, cell Rect, row dvRow, left bool, numW int) {
	lh := v.rowH()
	ty := cell.Y + (lh-v.glyphHeight())/2
	if row.kind == dvFold {
		v.drawText(p, cell.X+numW+scaled(4), ty, "... "+strconv.Itoa(row.n), c.foldInk)
		return
	}
	line, spans := row.b, row.spanB
	if left {
		line, spans = row.a, row.spanA
	}
	if line < 0 {
		// a blank facing a change: hatch it lightly so it reads as absent
		fillRect(p, cell.X, cell.Y, cell.W, cell.H, theme.SurfaceAlt)
		return
	}
	fill, ink, mark, word, sign := theme.Surface, theme.OnSurface, RGBA{}, RGBA{}, ""
	if row.kind != dvEqual {
		if left {
			fill, ink, mark, word, sign = c.delFill, c.delInk, c.delMark, c.delWord, "-"
		} else {
			fill, ink, mark, word, sign = c.addFill, c.addInk, c.addMark, c.addWord, "+"
		}
	}
	text := v.new[line]
	if left {
		text = v.old[line]
	}
	fillRect(p, cell.X, cell.Y, cell.W, cell.H, fill)
	n := strconv.Itoa(line + 1)
	v.drawText(p, cell.X+numW-scaled(4)-v.textWidth(n), ty, n, c.numInk)
	if sign != "" {
		fillRect(p, cell.X+numW-scaled(2), cell.Y, scaled(2), cell.H, mark)
		v.drawText(p, cell.X+numW+scaled(2), ty, sign, mark)
	}
	textX, _ := v.geometry()
	v.drawLine(p, cell.X+textX, cell.Y, text, spans, ink, word)
}

// drawInline paints a row of the one-column layout: both line numbers, the
// sign, the text.
func (v *DiffView) drawInline(p painter.Painter, theme *Theme, c dvColors, cell Rect, row dvRow, numW int) {
	lh := v.rowH()
	ty := cell.Y + (lh-v.glyphHeight())/2
	gutter := 2 * numW
	if row.kind == dvFold {
		v.drawText(p, cell.X+gutter+scaled(4), ty, "... "+strconv.Itoa(row.n), c.foldInk)
		return
	}
	fill, ink, mark, word, sign, text, spans := theme.Surface, theme.OnSurface, RGBA{}, RGBA{}, "", "", [][2]int(nil)
	switch row.kind {
	case dvDelete:
		fill, ink, mark, word, sign, text, spans = c.delFill, c.delInk, c.delMark, c.delWord, "-", v.old[row.a], row.spanA
	case dvInsert:
		fill, ink, mark, word, sign, text, spans = c.addFill, c.addInk, c.addMark, c.addWord, "+", v.new[row.b], row.spanB
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
	textX, _ := v.geometry()
	v.drawLine(p, cell.X+textX, cell.Y, text, spans, ink, word)
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
