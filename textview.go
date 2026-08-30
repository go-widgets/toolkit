// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// TextSpan is a coloured run within a single line, in half-open rune
// coordinates [Start, End). A TextView.Highlighter returns a slice of
// these to paint syntax-highlighted source: any rune not covered by a
// span keeps the default ink. Spans may overlap — a later span in the
// slice wins for the runes it covers. Start/End are clamped to the
// line's rune length at paint time, so a highlighter need not worry
// about off-by-one bounds. Color reuses the toolkit's RGBA (a painter
// colour), so a highlighter composes theme colours directly.
type TextSpan struct {
	Start, End int
	Color      RGBA
}

// TextView is the multi-line cousin of Entry. Its reactive state is MVVM-only:
// the committed contents live on the Text() Observable (a host binds or
// subscribes to it, and every edit Sets it — there is no OnChange callback),
// and the caret line/col, the vertical scroll offset, the selection range and
// the focus flag are each exposed through their own Observable accessor so a
// host can bind or drive them without touching a field. The line buffer and the
// in-flight IME preview are internal editing state.
//
// This is the foundation a native wasmdesk editor builds on top of: syntax
// highlighting, search/replace and find can live above TextView without it
// growing those concerns.
type TextView struct {
	Base

	// Decorations are remote co-editors' carets + selections in a shared
	// buffer, each painted in its own colour with a name tag (see the
	// Decoration type). Empty for a solo editor; the host keeps it in sync
	// with the collaboration session. Painted on top of the local selection
	// so a co-editor's presence is always visible.
	Decorations []Decoration

	// Highlighter, when non-nil, turns each line into coloured runs at
	// paint time: Draw calls Highlighter(lineIndex, line) and paints
	// the returned TextSpans in their colours, with any uncovered runes
	// falling back to the default ink (theme.OnSurface). When nil (the
	// zero value) Draw paints every line in a single ink exactly as it
	// always has — a host adds syntax highlighting by setting this hook
	// without the widget growing a lexer of its own.
	Highlighter func(lineIndex int, line string) []TextSpan

	// ShowLineNumbers, when true, reserves a left gutter sized to the
	// widest line number and paints right-aligned 1-based line numbers
	// there; the text, caret and selection all shift right by the
	// gutter width. When false (the zero value) there is no gutter and
	// layout is byte-identical to before this field existed.
	ShowLineNumbers bool

	// GutterColor is the ink for the line-number gutter. The zero value
	// (a fully-transparent RGBA, A==0) means "unset": Draw then falls
	// back to a muted tone (dimInk) that reads on any theme.
	GutterColor RGBA

	// RowBackground, when non-nil, is consulted once per visible buffer
	// line to paint a full-width background band behind that line — over
	// the Surface fill, under the gutter number, the ink and the caret.
	// It returns (colour, true) to paint the band in colour, or
	// (_, false) to leave the row on the plain Surface. This is the seam
	// a CodeEditor uses for its current-line highlight, a search UI for
	// match rows, or a diff view for added / removed rows, without
	// TextView growing any of those concerns. When nil (the zero value)
	// no band is painted and rendering is byte-identical to before this
	// field existed.
	RowBackground func(lineIndex int) (RGBA, bool)

	// lines is the working buffer (one element per visible line). The Text()
	// Observable mirrors strings.Join(lines, "\n"); edits mutate lines then
	// sync(). It stays a plain slice (not an Observable) because slices are not
	// comparable and the editing code splices individual lines in place.
	lines []string
	// text / curLine / curCol / scroll / sel / focus are the reactive state,
	// each the single source of truth for its datum, reached through the
	// Text()/CursorLine()/CursorCol()/ScrollLine()/Selection()/Focused()
	// accessors. Lazily created so a bare &TextView{} works.
	text    *mvvm.Observable[string]
	curLine *mvvm.Observable[int]
	curCol  *mvvm.Observable[int]
	scroll  *mvvm.Observable[int]
	sel     *mvvm.Observable[Selection]
	focus   *mvvm.Observable[bool]
	// composition holds the in-progress IME preview string (dead-key output, CJK
	// candidate, …). Non-empty while an IME composition is active; cleared on
	// EventCompositionEnd. Draw paints it in a muted colour at the cursor; the
	// preview is NOT part of the buffer until the host commits via EventChar, so
	// Text() sees only committed input.
	composition string

	// caretHidden suppresses the insertion caret while it is true, even when the
	// view is focused — the seam a host uses to blink the caret (it toggles this on
	// a timer). Zero value false keeps the caret visible, so nothing changes for a
	// host that never touches it. The IME composition preview is unaffected.
	caretHidden bool

	// undo/redo hold point-in-time snapshots taken before each
	// mutating edit (see pushUndo). Ports the go-widgets/tui
	// TextEditor's undo model: one snapshot per mutation (no
	// coalescing of consecutive keystrokes -- simplest correct
	// behaviour, matching the sibling's documented choice).
	undo, redo []tvSnapshot

	// selAnchorLine/selAnchorCol remember where a mouse selection began: an
	// EventClick records the caret it placed as the anchor, and each
	// EventMouseDrag extends the selection from that anchor to the dragged-to
	// caret. Kept private -- callers drive programmatic selection through
	// SetSelection/SelectAll instead.
	selAnchorLine, selAnchorCol int

	// matchBands are search-match highlight ranges pushed by a host through the
	// CodeEditor match-highlight API (SetMatchHighlights / SetCurrentMatch): a
	// soft fill behind every match plus a stronger outline box for the current
	// one. Painted under the text like a selection band, on top of RowBackground
	// and the local/remote selections so a match on the caret line still reads.
	// Empty (nil) for an editor with no active search. See matchhighlight.go.
	matchBands []matchBand
}

// Text is the committed contents as a shared [mvvm.Observable]: a host binds it
// two-way (or subscribes) instead of touching a field, and every edit Sets it —
// so a Set is the only way to change the text and there is no change callback.
// A host that Sets it directly (a VM→widget push) reloads the line buffer.
// Lazily created so a bare &TextView{} works.
func (t *TextView) Text() *mvvm.Observable[string] {
	if t.text == nil {
		t.text = mvvm.NewObservable(strings.Join(t.lines, "\n"))
		t.text.Subscribe(func(s string) {
			// Only a host-driven Set diverges from the buffer; sync()'s own
			// echo (join(lines) == s) is skipped so there is no loop.
			if strings.Join(t.lines, "\n") != s {
				t.adopt(s)
			}
		})
	}
	return t.text
}

// CursorLine is the caret's 0-based buffer line as a bindable Observable.
func (t *TextView) CursorLine() *mvvm.Observable[int] {
	if t.curLine == nil {
		t.curLine = mvvm.NewObservable(0)
	}
	return t.curLine
}

// CursorCol is the caret's 0-based rune column on its line, bindable.
func (t *TextView) CursorCol() *mvvm.Observable[int] {
	if t.curCol == nil {
		t.curCol = mvvm.NewObservable(0)
	}
	return t.curCol
}

// ScrollLine is the buffer line painted at the top of the viewport — the
// vertical scroll offset — as a bindable Observable. Draw windows from here,
// the wheel shifts it, and every cursor move scrolls it to keep the caret
// visible. Reads clamp on the fly (clampedScrollLine), so a stale value after
// the buffer shrank is harmless.
func (t *TextView) ScrollLine() *mvvm.Observable[int] {
	if t.scroll == nil {
		t.scroll = mvvm.NewObservable(0)
	}
	return t.scroll
}

// Selection is the (start, end) range the host paints highlighted + range-deletes
// via DeleteSelection / cut+paste via CopySelection / CutSelection / Paste, as a
// bindable Observable. An empty selection (Start == End) means "no selection";
// HasSelection() is the convenience predicate.
func (t *TextView) Selection() *mvvm.Observable[Selection] {
	if t.sel == nil {
		t.sel = mvvm.NewObservable(Selection{})
	}
	return t.sel
}

// Focused reports (and drives) keyboard focus as a bindable Observable: Draw
// paints the accent border + caret while it is true.
func (t *TextView) Focused() *mvvm.Observable[bool] {
	if t.focus == nil {
		t.focus = mvvm.NewObservable(false)
	}
	return t.focus
}

// CaretVisible reports whether the insertion caret is currently drawn (when the
// view is focused). It is true by default.
func (t *TextView) CaretVisible() bool { return !t.caretHidden }

// SetCaretVisible shows or hides the insertion caret without disturbing focus,
// selection or the IME preview. A host blinks the caret by toggling this on a
// timer; setting it back to true (e.g. on a keystroke) makes the caret solid
// again so it never blinks off mid-typing.
func (t *TextView) SetCaretVisible(v bool) { t.caretHidden = !v }

// sync publishes the line buffer onto the Text() Observable, firing its
// subscribers. Called at the end of every mutating edit — the OnChange
// replacement. The Set is idempotent, so re-syncing an unchanged buffer is a
// no-op.
func (t *TextView) sync() { t.Text().Set(strings.Join(t.lines, "\n")) }

// adopt reloads the line buffer from a host-set Text() value and clamps the
// caret into the new bounds. It does NOT touch the Text() Observable (the caller
// is that Observable's subscriber), so there is no feedback loop.
func (t *TextView) adopt(s string) {
	if s == "" {
		t.lines = []string{""}
	} else {
		t.lines = strings.Split(s, "\n")
	}
	t.clampCursorToBuffer()
}

// clampCursorToBuffer pins the caret line into [0, len(lines)-1] and its column
// into the resulting line, after the buffer was replaced under it.
func (t *TextView) clampCursorToBuffer() {
	cl := t.CursorLine().Get()
	if cl > len(t.lines)-1 {
		cl = len(t.lines) - 1
	}
	if cl < 0 {
		cl = 0
	}
	t.CursorLine().Set(cl)
	t.clampCol()
}

// maxUndo caps the undo history so a long editing session can't grow
// the snapshot stack without bound.
const maxUndo = 200

// tvSnapshot is a point-in-time copy of the buffer + cursor +
// selection, used to restore a prior state on Undo/Redo.
type tvSnapshot struct {
	lines []string
	line  int
	col   int
	sel   Selection
}

// snapshot captures the current buffer + cursor + selection.
func (t *TextView) snapshot() tvSnapshot {
	cp := make([]string, len(t.lines))
	copy(cp, t.lines)
	return tvSnapshot{lines: cp, line: t.CursorLine().Get(), col: t.CursorCol().Get(), sel: t.Selection().Get()}
}

// restore replaces the buffer + cursor + selection with s. It does not publish;
// Undo/Redo sync() after restoring.
func (t *TextView) restore(s tvSnapshot) {
	t.lines = s.lines
	t.CursorLine().Set(s.line)
	t.CursorCol().Set(s.col)
	t.Selection().Set(s.sel)
}

// pushUndo records the current state before a mutation and drops any
// redo history -- a fresh edit invalidates the redo branch. Callers
// invoke this immediately before the mutation it should undo, and
// only when the operation will actually mutate the buffer (so purely
// no-op edits -- e.g. Backspace at the buffer start, an empty
// EventChar -- don't waste a stack slot).
func (t *TextView) pushUndo() {
	t.undo = append(t.undo, t.snapshot())
	if len(t.undo) > maxUndo {
		t.undo = t.undo[len(t.undo)-maxUndo:]
	}
	t.redo = nil
}

// Undo restores the buffer + cursor + selection to the state before
// the most recent mutation, pushing the current state onto the redo
// stack. No-op when there is nothing to undo.
func (t *TextView) Undo() {
	if len(t.undo) == 0 {
		return
	}
	t.redo = append(t.redo, t.snapshot())
	last := t.undo[len(t.undo)-1]
	t.undo = t.undo[:len(t.undo)-1]
	t.restore(last)
	t.sync()
}

// Redo re-applies the most recently undone mutation, pushing the
// current state back onto the undo stack. No-op when there is
// nothing to redo.
func (t *TextView) Redo() {
	if len(t.redo) == 0 {
		return
	}
	t.undo = append(t.undo, t.snapshot())
	last := t.redo[len(t.redo)-1]
	t.redo = t.redo[:len(t.redo)-1]
	t.restore(last)
	t.sync()
}

// NewTextView builds a TextView pre-loaded with initial text (split
// on "\n"). Empty initial text creates a single empty line so the
// cursor always has a row to live on.
func NewTextView(initial string) *TextView {
	t := &TextView{}
	if initial == "" {
		t.lines = []string{""}
	} else {
		t.lines = strings.Split(initial, "\n")
	}
	return t
}

// SetText replaces the entire buffer + parks the cursor at (0,0). It goes
// through the Text() Observable, so bindings/subscribers fire.
func (t *TextView) SetText(s string) {
	if s == "" {
		t.lines = []string{""}
	} else {
		t.lines = strings.Split(s, "\n")
	}
	t.CursorLine().Set(0)
	t.CursorCol().Set(0)
	t.sync()
}

// Draw paints border + fill + every visible line + (when Focused) a
// 1-px vertical cursor stroke at the cursor's screen position.
//
// Lines that would render past the bottom of the bounds are
// painted-but-clipped by the raster helpers; wrap in a ScrollView
// for proper scrollable behaviour.
func (t *TextView) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	border := theme.Border
	if t.Focused().Get() {
		border = theme.Accent
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)
	lineH := t.glyphHeight() + 4 // 1-pixel-line font + 4 px line spacing
	textX := r.X + t.textLeftInset()
	gutterInk := t.GutterColor
	if gutterInk.A == 0 {
		gutterInk = dimInk(theme)
	}
	// Window from ScrollLine so a buffer taller than the bounds is reachable;
	// clip to the bounds so a partially-visible trailing line never bleeds past
	// the bottom edge into a neighbour. At ScrollLine == 0 with a buffer that
	// fits, start is 0 and every line is drawn exactly where it was before.
	start := t.clampedScrollLine()
	localSel := t.Selection().Get()
	withClip(p, r, func() {
		for i := start; i < len(t.lines); i++ {
			y := r.Y + 4 + (i-start)*lineH
			if y >= r.Y+r.H {
				break // fully below the viewport
			}
			line := t.lines[i]
			// Full-width row band (current-line highlight, search match, diff
			// row): painted first so the gutter number, ink and caret land on
			// top. The band brackets the glyph box (2 px above) and spans one
			// full line pitch, so consecutive bands tile without a seam.
			if t.RowBackground != nil {
				if bg, ok := t.RowBackground(i); ok {
					fillRect(p, r.X+1, y-2, r.W-2, lineH, bg)
				}
			}
			// Selection bands, painted under the text so the glyphs stay
			// legible: the local selection first (accent tint), then each
			// remote co-editor's selection in its own tint on top.
			t.paintSelectionBand(p, i, textX, y, lineH, r, localSel, tintBand(theme.Accent))
			for _, d := range t.Decorations {
				t.paintSelectionBand(p, i, textX, y, lineH, r, d.Selection, tintBand(d.Color))
			}
			// Search-match bands sit on top of the selection so a highlighted
			// match on a selected / caret line is still visible.
			t.paintMatchBands(p, i, textX, y, lineH, r)
			if t.ShowLineNumbers {
				num := strconv.Itoa(i + 1)
				// Right-justify within the numbers column: every number's right
				// edge lands at gutterPadL + numbersWidth, leaving gutterPadR of
				// air before the text.
				nx := r.X + t.gutterPadL() + t.numbersWidth() - t.textWidth(num)
				t.drawText(p, nx, y, num, gutterInk)
			}
			if t.Highlighter == nil {
				t.drawText(p, textX, y, line, theme.OnSurface)
				continue
			}
			t.drawSpans(p, textX, y, line, t.Highlighter(i, line), theme.OnSurface)
		}
	})
	if t.Focused().Get() {
		cx := textX + t.CursorCol().Get()*t.glyphAdvance()
		cy := r.Y + 4 + (t.CursorLine().Get()-start)*lineH
		if !t.caretHidden {
			fillRect(p, cx, cy-1, 1, t.glyphHeight()+2, theme.OnSurface)
		}
		// IME composition preview: render the pending string in the
		// muted SurfaceAlt tone starting at the cursor, so the user
		// sees dead-key / CJK candidates without them entering the
		// buffer. Underlined by a 1-px SurfaceAlt strip beneath.
		if t.composition != "" {
			cw := t.textWidth(t.composition)
			t.drawText(p, cx, cy, t.composition, theme.SurfaceAlt)
			fillRect(p, cx, cy+t.glyphHeight(), cw, 1, theme.SurfaceAlt)
		}
	}
	// Remote co-editor carets + name tags, on top of everything, each in the
	// co-editor's colour, so a collaborative session shows who is where.
	for _, d := range t.Decorations {
		if d.CursorLine < start || d.CursorLine >= len(t.lines) {
			continue
		}
		cx := textX + d.CursorCol*t.glyphAdvance()
		cy := r.Y + 4 + (d.CursorLine-start)*lineH
		if cx < r.X || cx > r.X+r.W {
			continue
		}
		fillRect(p, cx, cy-1, 2, t.glyphHeight()+2, d.Color)
		if d.Label != "" {
			tagW := t.textWidth(d.Label) + 4
			tagY := cy - t.glyphHeight() - 3
			if tagY < r.Y {
				tagY = cy // flip below the top edge
			}
			fillRect(p, cx, tagY, tagW, t.glyphHeight()+2, d.Color)
			t.drawText(p, cx+2, tagY+1, d.Label, tagInk(d.Color))
		}
	}
}

// paintSelectionBand fills the highlight for selection sel on buffer line i, in
// colour c, under the text. Nothing is painted when the line is outside sel or
// sel is empty. A line that the selection spans through (not the last line)
// extends to the right edge, so the selected newline reads as selected.
func (t *TextView) paintSelectionBand(p painter.Painter, i, textX, y, lineH int, r Rect, sel Selection, c RGBA) {
	if sel.IsEmpty() || i < sel.StartLine || i > sel.EndLine {
		return
	}
	x0, x1 := t.bandXRange(i, textX, r, sel)
	fillRect(p, x0, y-2, x1-x0, lineH, c)
}

// bandXRange is the horizontal extent of the band for range sel on buffer line
// i, in absolute pixels: it starts at sel.StartCol on the start line (else at
// the text origin) and ends at sel.EndCol on the end line (else at the widget's
// right edge, so a range spanning through the line reads as covering its
// newline). Shared by paintSelectionBand and paintMatchBands so the selection
// highlight and the search-match highlight place their bands identically.
func (t *TextView) bandXRange(i, textX int, r Rect, sel Selection) (x0, x1 int) {
	adv := t.glyphAdvance()
	sc := 0
	if i == sel.StartLine {
		sc = sel.StartCol
	}
	x0 = textX + sc*adv
	if i < sel.EndLine {
		x1 = r.X + r.W - 1 // range continues onto the next line
	} else {
		x1 = textX + sel.EndCol*adv
	}
	return x0, x1
}

// tintBand returns c at the selection-band alpha, so the highlight tints the
// row without hiding the glyphs painted on top of it.
func tintBand(c RGBA) RGBA { return RGBA{R: c.R, G: c.G, B: c.B, A: 0x55} }

// tagInk returns a legible ink (black or white) for text on a co-editor tag of
// colour c, chosen by c's perceived luminance.
func tagInk(c RGBA) RGBA {
	if int(c.R)*299+int(c.G)*587+int(c.B)*114 > 128*1000 {
		return RGBA{A: 0xFF}
	}
	return RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
}

// OnEvent dispatches the editing operations.
func (t *TextView) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		t.Focused().Set(true)
		if len(t.lines) == 0 {
			return
		}
		// Place the caret under the click (mapping ev.X/ev.Y back through
		// Draw's line/col layout) and start a fresh, collapsed selection
		// anchored there so a following drag can extend it.
		cl, cc := t.caretAt(ev.X, ev.Y)
		t.CursorLine().Set(cl)
		t.CursorCol().Set(cc)
		t.selAnchorLine, t.selAnchorCol = cl, cc
		t.ClearSelection()
		t.scrollCaretIntoView()
	case EventScroll:
		// Native wheel scroll: shift the viewport by Delta lines (clamped at
		// both ends). Independent of the caret, exactly like every sibling
		// content widget that gained native scroll in v0.98.0.
		t.scrollBy(ev.Delta)
	case EventMouseDrag:
		if len(t.lines) == 0 {
			return
		}
		// Extend the selection from the click anchor to the caret under the
		// dragged pointer, moving the cursor with it.
		cl, cc := t.caretAt(ev.X, ev.Y)
		t.CursorLine().Set(cl)
		t.CursorCol().Set(cc)
		t.Selection().Set(SelectionRange(t.selAnchorLine, t.selAnchorCol, cl, cc))
		t.scrollCaretIntoView()
	case EventKeyDown:
		t.handleKey(ev.Code)
		t.scrollCaretIntoView()
	case EventChar:
		// If an IME composition was in flight, the incoming char is
		// the commit result — clear the preview BEFORE inserting so
		// the buffer + display stay consistent.
		t.composition = ""
		if ev.Code != "" {
			t.pushUndo()
		}
		t.insertText(ev.Code)
		t.scrollCaretIntoView()
	case EventCompositionStart, EventCompositionUpdate:
		// Preview only — do NOT touch the buffer. Repaint responsibility
		// lies with the host, who typically calls the widget's Draw
		// method after each composition event.
		t.composition = ev.Code
	case EventCompositionEnd:
		// Cancel / commit-without-follow-up: drop the preview. When
		// the host follows up with EventChar (commit path), the
		// EventChar arm above will re-clear + insert.
		t.composition = ""
	}
}

// handleKey runs the per-key navigation + delete operations.
func (t *TextView) handleKey(code string) {
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	switch code {
	case "Backspace":
		if cc > 0 || cl > 0 {
			t.pushUndo()
		}
		t.backspace()
	case "Enter":
		t.pushUndo()
		t.splitLine()
	case "Ctrl+Z":
		t.Undo()
	case "Ctrl+Y", "Ctrl+Shift+Z":
		t.Redo()
	case "Ctrl+C":
		t.CopySelection()
	case "Ctrl+X":
		t.CutSelection()
	case "Ctrl+V":
		t.Paste(ClipboardText())
	case "ArrowLeft":
		t.cursorLeft()
	case "ArrowRight":
		t.cursorRight()
	case "ArrowUp":
		if cl > 0 {
			t.CursorLine().Set(cl - 1)
			t.clampCol()
		}
	case "ArrowDown":
		if cl < len(t.lines)-1 {
			t.CursorLine().Set(cl + 1)
			t.clampCol()
		}
	case "Home":
		t.CursorCol().Set(0)
	case "End":
		t.CursorCol().Set(len([]rune(t.lines[cl])))
	}
}

// insertText inserts s at the cursor; "\n" inside s splits lines.
func (t *TextView) insertText(s string) {
	if s == "" {
		return
	}
	for _, ch := range s {
		if ch == '\n' {
			t.splitLine()
			continue
		}
		cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
		runes := []rune(t.lines[cl])
		runes = append(runes[:cc], append([]rune{ch}, runes[cc:]...)...)
		t.lines[cl] = string(runes)
		t.CursorCol().Set(cc + 1)
	}
	t.sync()
}

// splitLine breaks the current line at the cursor + moves the
// cursor to col 0 of the new line.
func (t *TextView) splitLine() {
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	cur := []rune(t.lines[cl])
	left := string(cur[:cc])
	right := string(cur[cc:])
	t.lines[cl] = left
	t.lines = append(t.lines[:cl+1], append([]string{right}, t.lines[cl+1:]...)...)
	t.CursorLine().Set(cl + 1)
	t.CursorCol().Set(0)
	t.sync()
}

// backspace removes the char before the cursor (or merges lines).
func (t *TextView) backspace() {
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	if cc > 0 {
		runes := []rune(t.lines[cl])
		t.lines[cl] = string(append(runes[:cc-1], runes[cc:]...))
		t.CursorCol().Set(cc - 1)
		t.sync()
		return
	}
	if cl == 0 {
		return // at buffer start; nothing to delete
	}
	prev := []rune(t.lines[cl-1])
	t.CursorCol().Set(len(prev))
	t.lines[cl-1] = string(prev) + t.lines[cl]
	t.lines = append(t.lines[:cl], t.lines[cl+1:]...)
	t.CursorLine().Set(cl - 1)
	t.sync()
}

// cursorLeft handles the wrap-to-previous-line case.
func (t *TextView) cursorLeft() {
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	if cc > 0 {
		t.CursorCol().Set(cc - 1)
		return
	}
	if cl > 0 {
		t.CursorLine().Set(cl - 1)
		t.CursorCol().Set(len([]rune(t.lines[cl-1])))
	}
}

// cursorRight handles the wrap-to-next-line case.
func (t *TextView) cursorRight() {
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	line := []rune(t.lines[cl])
	if cc < len(line) {
		t.CursorCol().Set(cc + 1)
		return
	}
	if cl < len(t.lines)-1 {
		t.CursorLine().Set(cl + 1)
		t.CursorCol().Set(0)
	}
}

// visibleLines is how many whole buffer lines fit vertically within the bounds
// (minus the 4px top inset) at the current line height. Floored at 1 so a very
// short widget still shows the caret's line and scrolling stays well-defined. A
// non-positive height or line height both collapse to 0 — no lines fit.
func (t *TextView) visibleLines() int {
	lineH := t.glyphHeight() + 4 // > 0: glyphHeight is always positive
	h := t.Bounds().H - 4
	if h <= 0 {
		return 0
	}
	n := h / lineH
	if n < 1 {
		n = 1
	}
	return n
}

// maxScrollLine is the highest ScrollLine that still leaves a full window of
// lines on screen: len(lines) - visibleLines(), floored at 0 so a buffer that
// already fits never scrolls.
func (t *TextView) maxScrollLine() int {
	m := len(t.lines) - t.visibleLines()
	if m < 0 {
		m = 0
	}
	return m
}

// clampedScrollLine returns ScrollLine clamped to [0, maxScrollLine()] WITHOUT
// mutating the Observable, so an out-of-range value (set directly, or left stale
// after the buffer shrank) never paints or hit-tests outside the valid window.
func (t *TextView) clampedScrollLine() int {
	s := t.ScrollLine().Get()
	if s < 0 {
		s = 0
	}
	if m := t.maxScrollLine(); s > m {
		s = m
	}
	return s
}

// scrollBy shifts ScrollLine by delta lines (negative scrolls up), clamped to
// [0, maxScrollLine()] and written back immediately.
func (t *TextView) scrollBy(delta int) {
	t.ScrollLine().Set(t.ScrollLine().Get() + delta)
	t.ScrollLine().Set(t.clampedScrollLine())
}

// scrollCaretIntoView nudges ScrollLine so the caret's line (CursorLine) stays
// within the visible window: up if it sits above the top line, down if at or
// past the last visible line. Called after every cursor move so typing or
// arrowing off the visible region follows the caret (Wave 4 keep-caret-visible).
func (t *TextView) scrollCaretIntoView() {
	vis := t.visibleLines()
	if vis <= 0 {
		return
	}
	cl := t.CursorLine().Get()
	s := t.ScrollLine().Get()
	if cl < s {
		t.ScrollLine().Set(cl)
	} else if cl >= s+vis {
		t.ScrollLine().Set(cl - vis + 1)
	}
	t.ScrollLine().Set(t.clampedScrollLine())
}

// gutterPadL is the breathing space from the widget's left edge to the line
// numbers; gutterPadR the space between the numbers and the text. Both are
// metric-scaled so the gutter stays airy at any resolution.
func (t *TextView) gutterPadL() int { return scaled(10) }
func (t *TextView) gutterPadR() int { return scaled(12) }

// numbersWidth is the width of the widest line number (the last line's), which
// every line number is right-justified within.
func (t *TextView) numbersWidth() int { return t.textWidth(strconv.Itoa(len(t.lines))) }

// gutterWidth is the pixel width reserved for the line-number gutter, or 0 when
// ShowLineNumbers is false. It sizes to the widest number (the last line's) plus
// padding, so the number column never jitters as the cursor moves between rows
// of different index widths.
func (t *TextView) gutterWidth() int {
	if !t.ShowLineNumbers {
		return 0
	}
	return t.gutterPadL() + t.numbersWidth() + t.gutterPadR()
}

// textLeftInset is where the text (and caret) begins, from the widget's left
// edge: right after the gutter when line numbers show (the gutter already
// carries its right padding), else a small base inset so text isn't flush to
// the border. Draw and caretAt share this so click-to-caret cannot drift.
func (t *TextView) textLeftInset() int {
	if t.ShowLineNumbers {
		return t.gutterWidth()
	}
	return scaled(4)
}

// drawSpans paints line at (x, y) as coloured runs: every rune starts
// at the base ink, spans overwrite the runes they cover (later spans
// win), and consecutive same-colour runes are coalesced into a single
// drawText so proportional fonts stay correctly kerned within a run.
// Out-of-range span bounds are clamped, so a highlighter can be sloppy
// about edges without corrupting the paint.
func (t *TextView) drawSpans(p painter.Painter, x, y int, line string, spans []TextSpan, base RGBA) {
	runes := []rune(line)
	if len(runes) == 0 {
		return
	}
	colors := make([]RGBA, len(runes))
	for i := range colors {
		colors[i] = base
	}
	for _, sp := range spans {
		s, e := sp.Start, sp.End
		if s < 0 {
			s = 0
		}
		if e > len(runes) {
			e = len(runes)
		}
		for j := s; j < e; j++ {
			colors[j] = sp.Color
		}
	}
	cx := x
	runStart := 0
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || colors[i] != colors[runStart] {
			seg := string(runes[runStart:i])
			t.drawText(p, cx, y, seg, colors[runStart])
			cx += t.textWidth(seg)
			runStart = i
		}
	}
}

// caretAt maps a widget-local (x, y) to a (line, col) caret position -- the
// inverse of Draw's placement, where line i sits at local y == 4+i*lineH and
// column c at local x == 4+gutterW+c*glyphAdvance. The line is clamped to the
// buffer and the column to that line's rune length; the column rounds to the
// nearest gap so a click on a glyph's right half lands after it. Callers
// guarantee len(lines) > 0 before calling.
func (t *TextView) caretAt(x, y int) (line, col int) {
	lineH := t.glyphHeight() + 4 // > 0: glyphHeight is always positive
	if y >= 4 {
		line = (y - 4) / lineH
	}
	// Map the viewport row back to an absolute buffer line through the scroll
	// offset, so a click after scrolling lands on the line the user sees.
	line += t.clampedScrollLine()
	if line > len(t.lines)-1 {
		line = len(t.lines) - 1
	}
	adv := t.glyphAdvance() // > 0 for every real font (Draw assumes the same)
	col = (x - t.textLeftInset() + adv/2) / adv
	if col < 0 {
		col = 0
	}
	if maxCol := len([]rune(t.lines[line])); col > maxCol {
		col = maxCol
	}
	return line, col
}

// CaretPixel returns the top-left device pixel of the caret cell for a 0-based
// (line, col) position, using the SAME layout Draw and caretAt share
// (textLeftInset + the gutter, and the line pitch) — so a host or a test harness
// can place/probe the caret without duplicating the gutter/advance math (which
// drifts the moment the gutter padding or font changes). It is the inverse of
// caretAt: caretAt(CaretPixel(l,c)) == (l,c) for an in-range cell.
func (t *TextView) CaretPixel(line, col int) (x, y int) {
	r := t.Bounds()
	x = r.X + t.textLeftInset() + col*t.glyphAdvance()
	y = r.Y + 4 + (line-t.clampedScrollLine())*(t.glyphHeight()+4)
	return x, y
}

// clampCol clamps CursorCol to the current line's rune length, used
// after ArrowUp / ArrowDown lands on a shorter line.
func (t *TextView) clampCol() {
	cl := t.CursorLine().Get()
	maxCol := len([]rune(t.lines[cl]))
	if t.CursorCol().Get() > maxCol {
		t.CursorCol().Set(maxCol)
	}
}
