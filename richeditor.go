// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-richdoc/richdoc"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// DocPos is a caret position in a richdoc document: the top-level Block index
// and a rune Off(set) into that block's editable content (its flattened inline
// text, or a code block's verbatim text). It is a comparable value so it can
// live directly on an [mvvm.Observable].
type DocPos struct {
	Block int
	Off   int
}

// DocSelection is a half-open range of caret positions. Start and End are stored
// as given (an anchor and a cursor); normalizeSel puts them in document order
// for painting and range edits. An empty selection (Start == End) means "no
// selection".
type DocSelection struct {
	Start, End DocPos
}

// IsEmpty reports whether the selection covers no cells.
func (s DocSelection) IsEmpty() bool { return s.Start == s.End }

// RichEditor is a WYSIWYG editing surface over a [richdoc.Document]: it lays the
// document out as formatted, wrapped content (headings, paragraphs, lists,
// quotes, code, tables, rules, math and inline emphasis) and edits it in place.
//
// Its reactive state is MVVM-only: the document lives on the Doc() Observable, and
// the caret, selection, focus flag and vertical scroll offset are each their own
// Observable accessor, so a host binds/subscribes instead of touching a field —
// a consumer re-serialises to Markdown/LaTeX/ODT/RTF by subscribing to Doc(). The
// formatting verbs (ToggleStrong, SetBlockType, ToggleList, ...) are methods that
// mutate the model and refresh; a visible toolbar is the consumer's job.
//
// Every edit produces a NEW *richdoc.Document (via richdoc.Clone), so Doc()
// subscribers always see an immutable snapshot and the pointer change guarantees
// notification.
type RichEditor struct {
	Base

	doc    *mvvm.Observable[*richdoc.Document]
	caret  *mvvm.Observable[DocPos]
	sel    *mvvm.Observable[DocSelection]
	focus  *mvvm.Observable[bool]
	scroll *mvvm.Observable[int]

	// pending holds the inline styles to apply to the next typed rune when the
	// caret is collapsed (a word-processor's "bold is on" with nothing selected).
	// pendingActive gates it so a fresh caret inherits its neighbour's style.
	pending       styleBits
	pendingActive bool

	// anchor is the fixed end of a keyboard/mouse selection; the caret is the
	// moving end. Kept private — callers drive programmatic selection through
	// Selection().
	anchor DocPos

	// fontCache memoises resolved faces for one layout pass; hbCache memoises the
	// per-level heading base faces, rebuilt when the base face changes.
	fontCache map[fontKey]Font
	hbCache   [7]Font
	hbBase    Font

	// supFace memoises the smaller face a footnote marker is drawn in (a
	// superscript), rebuilt when the base face changes (supBase tracks it).
	supBase Font
	supFace Font

	// lastTheme is the theme most recently drawn with, reused for geometry-only
	// layout passes (caret pixel, hit-testing) where colours are irrelevant.
	lastTheme *Theme
}

// NewRichEditor builds an editor over doc (a nil doc starts an empty document).
func NewRichEditor(doc *richdoc.Document) *RichEditor {
	e := &RichEditor{}
	e.SetDocument(doc)
	return e
}

// Doc is the edited document as a shared [mvvm.Observable]: a host binds it two-way
// or subscribes to re-serialise on every edit. Lazily created so a bare
// &RichEditor{} works.
func (e *RichEditor) Doc() *mvvm.Observable[*richdoc.Document] {
	if e.doc == nil {
		e.doc = mvvm.NewObservable(&richdoc.Document{})
	}
	return e.doc
}

// Caret is the caret position as a bindable Observable.
func (e *RichEditor) Caret() *mvvm.Observable[DocPos] {
	if e.caret == nil {
		e.caret = mvvm.NewObservable(DocPos{})
	}
	return e.caret
}

// Selection is the highlighted range as a bindable Observable; empty means none.
func (e *RichEditor) Selection() *mvvm.Observable[DocSelection] {
	if e.sel == nil {
		e.sel = mvvm.NewObservable(DocSelection{})
	}
	return e.sel
}

// Focused reports (and drives) keyboard focus; Draw paints the caret + accent
// border while it is true.
func (e *RichEditor) Focused() *mvvm.Observable[bool] {
	if e.focus == nil {
		e.focus = mvvm.NewObservable(false)
	}
	return e.focus
}

// ScrollOffset is the vertical scroll position in device pixels (0 == top),
// bindable. Reads clamp on the fly, so a stale value after the document shrank is
// harmless.
func (e *RichEditor) ScrollOffset() *mvvm.Observable[int] {
	if e.scroll == nil {
		e.scroll = mvvm.NewObservable(0)
	}
	return e.scroll
}

// A11y exposes the editor as a textbox whose value is the document's plain text,
// so an assistive technology reads the edited content.
func (e *RichEditor) A11y() A11yInfo {
	return A11yInfo{Role: RoleTextbox, Value: richdoc.PlainText(e.docValue())}
}

// SetDocument replaces the whole document and parks the caret at the start. It
// goes through Doc(), so bindings/subscribers fire.
func (e *RichEditor) SetDocument(d *richdoc.Document) {
	if d == nil {
		d = &richdoc.Document{}
	}
	e.Doc().Set(d)
	e.Caret().Set(DocPos{})
	e.anchor = DocPos{}
	e.ClearSelection()
	e.pendingActive = false
}

// Document returns an independent deep copy of the current document — the
// snapshot a consumer serialises. Mutating it never affects the editor.
func (e *RichEditor) Document() *richdoc.Document { return richdoc.Clone(e.docValue()) }

// docValue is the live document, never nil.
func (e *RichEditor) docValue() *richdoc.Document {
	d := e.Doc().Get()
	if d == nil {
		return &richdoc.Document{}
	}
	return d
}

// HasSelection reports whether the selection covers any cells.
func (e *RichEditor) HasSelection() bool { return !e.Selection().Get().IsEmpty() }

// ClearSelection collapses the selection onto the caret.
func (e *RichEditor) ClearSelection() {
	c := e.Caret().Get()
	e.Selection().Set(DocSelection{Start: c, End: c})
}

// theme returns the theme to lay out with when none is supplied (geometry-only
// passes): the one last drawn, or a default.
func (e *RichEditor) theme() *Theme {
	if e.lastTheme != nil {
		return e.lastTheme
	}
	return DefaultLight()
}

// scrollbarReserve is the always-reserved right gutter so wrapping accounts for
// the scrollbar whether or not it is currently live — the same convention
// ScrollView uses.
func (e *RichEditor) scrollbarReserve() int { return scaled(8) }

// baseFont is the editor's body face.
func (e *RichEditor) baseFont() Font { return e.EffectiveFont() }

// sizedFont is the face for a heading level (0 == body).
func (e *RichEditor) sizedFont(level int) Font {
	if level <= 0 {
		return e.baseFont()
	}
	return e.headingBase(level)
}

// headingBase returns (and memoises) the base face for a heading level, resized
// from the body face; the cache resets when the body face changes.
func (e *RichEditor) headingBase(level int) Font {
	base := e.baseFont()
	if e.hbBase != base {
		e.hbBase = base
		e.hbCache = [7]Font{}
	}
	if e.hbCache[level] == nil {
		num, den := headingFactor(level)
		e.hbCache[level] = resizeFont(base, num, den)
	}
	return e.hbCache[level]
}

// superscriptFont returns (and memoises) the smaller face a footnote reference
// marker is drawn in — two-thirds of the body face — so the marker reads as a
// superscript. The cache resets when the body face changes.
func (e *RichEditor) superscriptFont() Font {
	base := e.baseFont()
	if e.supBase != base {
		e.supBase = base
		e.supFace = resizeFont(base, 2, 3)
	}
	return e.supFace
}

// footnoteRise is how far a footnote marker is lifted above the baseline, a
// third of the body height, so the superscript sits high without clipping.
func (e *RichEditor) footnoteRise() int { return e.baseFont().Height() / 3 }

// fontFor resolves the DRAW face for a style at a heading level: the sized base,
// emboldened for bold or any heading, and sheared for italic. Layout geometry
// uses the plain base metrics (bold/italic are faux styles that keep the base
// advance), so a run's cells stay grid-aligned while its glyphs carry the weight.
func (e *RichEditor) fontFor(st styleBits, level int) Font {
	key := fontKey{st, level}
	if f, ok := e.fontCache[key]; ok {
		return f
	}
	f := e.sizedFont(level)
	if st&styBold != 0 || level > 0 {
		if bf, err := NewSyntheticBoldFont(f); err == nil {
			f = bf
		}
	}
	if st&styItalic != 0 {
		if itf, err := NewSyntheticItalicFont(f); err == nil {
			f = itf
		}
	}
	e.fontCache[key] = f
	return f
}

// cellWidth is the device width one caret cell occupies. Text cells measure in
// the sized base face (style-independent, so bold/italic never shift the caret
// grid); atoms measure their placeholder label.
func (e *RichEditor) cellWidth(sr styledRune, level int) int {
	switch sr.atom {
	case atomLineBreak:
		return 0
	case atomImage:
		return e.baseFont().Measure(imageLabel(sr.payload.(richdoc.Image))) + 2*reBoxPad()
	case atomMath:
		return e.baseFont().Measure(sr.payload.(richdoc.Math).TeX)
	case atomRaw:
		return e.baseFont().Measure(sr.payload.(richdoc.RawInline).Text)
	case atomFootnote:
		return e.superscriptFont().Measure(footnoteMark(sr))
	case atomXRef:
		return e.baseFont().Measure(xrefText(sr.payload.(richdoc.CrossRef)))
	case atomAnchor:
		return strokeWidth() + scaled(2)
	}
	return e.sizedFont(level).Measure(string(sr.r))
}

// resizeFont returns base scaled by num/den: a larger bitmap scale for the
// built-in face, a re-instantiated TrueType face at the scaled em size for a
// vector face, or base unchanged for a face that exposes neither (a documented
// limitation, like synthetic bold).
func resizeFont(base Font, num, den int) Font {
	if num == den {
		return base
	}
	if bm, ok := base.(*bitmapFont); ok {
		ns := (bm.Scale*num + den/2) / den
		if ns < 1 {
			ns = 1
		}
		return NewBitmapFont(ns)
	}
	if fd, ok := base.(interface {
		FontData() []byte
		SizePx() int
	}); ok {
		data, sz := fd.FontData(), fd.SizePx()
		if len(data) > 0 && sz > 0 {
			ns := (sz*num + den/2) / den
			if ns < 1 {
				ns = 1
			}
			if f, err := NewTrueTypeFont(data, ns); err == nil {
				return f
			}
		}
	}
	return base
}

// --- caret geometry -------------------------------------------------------

// caretLineFor finds the visual line + in-line cell index for a document
// position, preferring a start-of-line match at a soft-wrap boundary so the caret
// is deterministic. It returns ok == false when the block has no caret line.
func (e *RichEditor) caretLineFor(lay reLayout, pos DocPos) (reLine, int, bool) {
	var best reLine
	bestCell := 0
	found := false
	for _, ln := range lay.lines {
		if !ln.hasStops || ln.blockIdx != pos.Block {
			continue
		}
		n := ln.nCells()
		if pos.Off == ln.startOff {
			return ln, 0, true
		}
		if pos.Off > ln.startOff && pos.Off <= ln.startOff+n {
			best, bestCell, found = ln, pos.Off-ln.startOff, true
		}
	}
	if found {
		return best, bestCell, true
	}
	for _, ln := range lay.lines {
		if ln.hasStops && ln.blockIdx == pos.Block {
			return ln, reClamp(pos.Off-ln.startOff, 0, ln.nCells()), true
		}
	}
	return reLine{}, 0, false
}

// CaretPixel returns the top-left device pixel of the caret for pos, in the same
// surface coordinates Draw paints in (scroll already applied), so a host or a
// test can place/probe the caret without duplicating the layout math.
func (e *RichEditor) CaretPixel(pos DocPos) (x, y int) {
	lay := e.buildLayout(e.theme())
	ln, cell, ok := e.caretLineFor(lay, pos)
	if !ok {
		r := e.Bounds()
		return r.X + rePadX(), r.Y + rePadTop()
	}
	return ln.cellX[cell], ln.textY - e.clampedScroll(lay)
}

// posAtLayout maps a layout-space point to the nearest caret position: the line
// whose box contains layY (else the vertically nearest stop line), then the
// nearest caret gap to layX.
func (e *RichEditor) posAtLayout(lay reLayout, layX, layY int) DocPos {
	var chosen *reLine
	for i := range lay.lines {
		ln := &lay.lines[i]
		if ln.hasStops && layY >= ln.y && layY < ln.y+ln.h {
			chosen = ln
			break
		}
	}
	if chosen == nil {
		bestDy := 1 << 30
		for i := range lay.lines {
			ln := &lay.lines[i]
			if !ln.hasStops {
				continue
			}
			dy := reAbsInt(layY - (ln.y + ln.h/2))
			if dy < bestDy {
				bestDy, chosen = dy, ln
			}
		}
	}
	if chosen == nil {
		return DocPos{}
	}
	cell, bestDx := 0, 1<<30
	for k, gx := range chosen.cellX {
		if dx := reAbsInt(layX - gx); dx < bestDx {
			bestDx, cell = dx, k
		}
	}
	return DocPos{Block: chosen.blockIdx, Off: chosen.startOff + cell}
}

// posAtLocal maps a widget-local event point to a document position.
func (e *RichEditor) posAtLocal(lay reLayout, lx, ly int) DocPos {
	r := e.Bounds()
	return e.posAtLayout(lay, r.X+lx, r.Y+ly+e.clampedScroll(lay))
}

// --- scrolling ------------------------------------------------------------

// maxScroll is the furthest the content can scroll and still fill the viewport.
func (e *RichEditor) maxScroll(lay reLayout) int {
	m := lay.height - e.Bounds().H
	if m < 0 {
		return 0
	}
	return m
}

// clampedScroll reads ScrollOffset clamped to [0, maxScroll] without mutating it.
func (e *RichEditor) clampedScroll(lay reLayout) int {
	return reClamp(e.ScrollOffset().Get(), 0, e.maxScroll(lay))
}

// scrollBy shifts the offset by dy device pixels, clamped, and writes it back.
func (e *RichEditor) scrollBy(dy int) {
	lay := e.buildLayout(e.theme())
	e.ScrollOffset().Set(reClamp(e.ScrollOffset().Get()+dy, 0, e.maxScroll(lay)))
}

// scrollCaretIntoView nudges the offset so the caret's line stays visible.
func (e *RichEditor) scrollCaretIntoView() {
	lay := e.buildLayout(e.theme())
	ln, _, ok := e.caretLineFor(lay, e.Caret().Get())
	if !ok {
		return
	}
	r := e.Bounds()
	top := ln.y - r.Y // content-space top of the caret line
	bot := top + ln.h // content-space bottom
	off := e.clampedScroll(lay)
	if top < off {
		off = top
	} else if bot > off+r.H {
		off = bot - r.H
	}
	e.ScrollOffset().Set(reClamp(off, 0, e.maxScroll(lay)))
}

// --- drawing --------------------------------------------------------------

// Draw paints the border + surface, the block chrome, selection bands, the
// formatted runs and (when focused) the caret, windowed by the scroll offset, and
// a scrollbar when the content overflows.
func (e *RichEditor) Draw(p painter.Painter, theme *Theme) {
	e.lastTheme = theme
	r := e.Bounds()
	border := theme.Border
	if e.Focused().Get() {
		border = theme.Accent
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)

	lay := e.buildLayout(theme)
	scroll := e.clampedScroll(lay)
	sel := normalizeSel(e.Selection().Get())
	caret := e.Caret().Get()

	withClip(p, r, func() {
		for _, c := range lay.chrome {
			e.drawChrome(p, c, r, scroll)
		}
		if !sel.IsEmpty() {
			for _, ln := range lay.lines {
				e.drawSelectionBand(p, theme, ln, sel, r, scroll)
			}
		}
		for _, ln := range lay.lines {
			y := ln.textY - scroll
			if y+ln.h < r.Y || y > r.Y+r.H {
				continue
			}
			for _, run := range ln.runs {
				e.drawRun(p, run, y)
			}
		}
		if e.Focused().Get() {
			if ln, cell, ok := e.caretLineFor(lay, caret); ok {
				cx := ln.cellX[cell]
				cy := ln.textY - scroll
				gh := ln.h - reLineGap()
				fillRect(p, cx, cy-1, strokeWidth(), gh+2, theme.OnSurface)
			}
		}
	})
	e.drawScrollbar(p, theme, lay, scroll)
}

// drawChrome paints one block-level decoration, windowed by scroll and culled to
// the bounds.
func (e *RichEditor) drawChrome(p painter.Painter, c reChrome, r Rect, scroll int) {
	y := c.r.Y - scroll
	if y+c.r.H < r.Y || y > r.Y+r.H {
		if c.text == "" {
			return
		}
	}
	if c.text != "" {
		c.font.Draw(p, c.r.X, y, c.text, c.c)
		return
	}
	if c.stroke {
		strokeRect(p, c.r.X, y, c.r.W, c.r.H, c.c)
		return
	}
	fillRect(p, c.r.X, y, c.r.W, c.r.H, c.c)
}

// drawRun paints one styled run at screen y (offset by the run's dy, so a
// superscript marker rides above the baseline), with link underline / strike
// rules.
func (e *RichEditor) drawRun(p painter.Painter, run reRun, y int) {
	y += run.dy
	run.font.Draw(p, run.x, y, run.text, run.ink)
	if !run.underline && !run.strike {
		return
	}
	w := run.font.Measure(run.text)
	gh := run.font.Height()
	if run.underline {
		fillRect(p, run.x, y+gh, w, strokeWidth(), run.ink)
	}
	if run.strike {
		fillRect(p, run.x, y+gh/2, w, strokeWidth(), run.ink)
	}
}

// drawSelectionBand paints the highlight for a single line under the text.
func (e *RichEditor) drawSelectionBand(p painter.Painter, theme *Theme, ln reLine, sel DocSelection, r Rect, scroll int) {
	if !ln.hasStops || ln.blockIdx < sel.Start.Block || ln.blockIdx > sel.End.Block {
		return
	}
	n := ln.nCells()
	lo := 0
	if ln.blockIdx == sel.Start.Block {
		lo = sel.Start.Off - ln.startOff
	} else {
		lo = -(1 << 30)
	}
	hiAbs := 1 << 30
	if ln.blockIdx == sel.End.Block {
		hiAbs = sel.End.Off - ln.startOff
	}
	c0 := reClamp(lo, 0, n)
	c1 := reClamp(hiAbs, 0, n)
	continues := hiAbs > n
	if c1 <= c0 && !continues {
		return
	}
	x0 := ln.cellX[c0]
	x1 := ln.cellX[c1]
	if continues {
		x1 = r.X + r.W - rePadX() - e.scrollbarReserve()
	}
	y := ln.y - scroll
	if x1 > x0 {
		fillRect(p, x0, y, x1-x0, ln.h, tintBand(theme.Accent))
	}
}

// drawScrollbar paints a track + proportional thumb on the right when the content
// overflows the viewport.
func (e *RichEditor) drawScrollbar(p painter.Painter, theme *Theme, lay reLayout, scroll int) {
	r := e.Bounds()
	if lay.height <= r.H {
		return
	}
	track := e.scrollbarReserve()
	tx := r.X + r.W - track
	fillRect(p, tx, r.Y, track, r.H, theme.SurfaceAlt)
	thumbH := r.H * r.H / lay.height
	if thumbH < scaled(12) {
		thumbH = scaled(12)
	}
	maxOff := lay.height - r.H
	thumbY := r.Y + scroll*(r.H-thumbH)/maxOff
	fillRect(p, tx, thumbY, track, thumbH, theme.Accent)
}

// --- events ---------------------------------------------------------------

// OnEvent dispatches clicks (caret placement + drag selection), the wheel and
// keyboard navigation/editing.
func (e *RichEditor) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		e.Focused().Set(true)
		lay := e.buildLayout(e.theme())
		pos := e.posAtLocal(lay, ev.X, ev.Y)
		e.Caret().Set(pos)
		e.anchor = pos
		e.ClearSelection()
		e.pendingActive = false
	case EventMouseDrag:
		lay := e.buildLayout(e.theme())
		pos := e.posAtLocal(lay, ev.X, ev.Y)
		e.Caret().Set(pos)
		e.Selection().Set(normalizeSel(DocSelection{Start: e.anchor, End: pos}))
	case EventScroll:
		e.scrollBy(ev.Delta * (e.baseFont().Height() + reLineGap()))
	case EventKeyDown:
		e.handleKey(ev)
	case EventChar:
		if ev.Code == "" {
			return
		}
		if e.HasSelection() {
			e.DeleteSelection()
		}
		e.InsertText(ev.Code)
		e.scrollCaretIntoView()
	}
}

// handleKey runs keyboard navigation, selection extension and editing.
func (e *RichEditor) handleKey(ev Event) {
	switch ev.Code {
	case "ArrowLeft":
		e.moveCaret(e.posLeft(), ev.Shift)
	case "ArrowRight":
		e.moveCaret(e.posRight(), ev.Shift)
	case "ArrowUp":
		e.moveCaret(e.posVert(-1), ev.Shift)
	case "ArrowDown":
		e.moveCaret(e.posVert(1), ev.Shift)
	case "Home":
		e.moveCaret(e.posHome(), ev.Shift)
	case "End":
		e.moveCaret(e.posEnd(), ev.Shift)
	case "Backspace":
		if e.HasSelection() {
			e.DeleteSelection()
		} else {
			e.backspace()
		}
		e.scrollCaretIntoView()
	case "Delete":
		if e.HasSelection() {
			e.DeleteSelection()
		} else {
			e.deleteForward()
		}
		e.scrollCaretIntoView()
	case "Enter":
		if e.HasSelection() {
			e.DeleteSelection()
		}
		e.splitBlock()
		e.scrollCaretIntoView()
	}
	if ev.Ctrl {
		switch ev.Code {
		case "b", "B":
			e.ToggleStrong()
		case "i", "I":
			e.ToggleEmph()
		}
	}
}

// moveCaret sets the caret to pos, either extending the selection from the anchor
// (shift held) or collapsing it and re-anchoring.
func (e *RichEditor) moveCaret(pos DocPos, shift bool) {
	if shift {
		e.Caret().Set(pos)
		e.Selection().Set(normalizeSel(DocSelection{Start: e.anchor, End: pos}))
	} else {
		e.Caret().Set(pos)
		e.anchor = pos
		e.ClearSelection()
	}
	e.pendingActive = false
	e.scrollCaretIntoView()
}

// posHome / posEnd move to the start / end of the caret's current visual line.
func (e *RichEditor) posHome() DocPos {
	lay := e.buildLayout(e.theme())
	if ln, _, ok := e.caretLineFor(lay, e.Caret().Get()); ok {
		return DocPos{ln.blockIdx, ln.startOff}
	}
	return e.Caret().Get()
}

func (e *RichEditor) posEnd() DocPos {
	lay := e.buildLayout(e.theme())
	if ln, _, ok := e.caretLineFor(lay, e.Caret().Get()); ok {
		return DocPos{ln.blockIdx, ln.startOff + ln.nCells()}
	}
	return e.Caret().Get()
}

// posVert moves the caret one caret-bearing visual line up (dir -1) or down
// (dir +1), keeping its horizontal pixel where possible. It steps through the
// ordered list of stop lines so inter-block gaps never trap the caret on its own
// line; at the first/last stop line it stays put.
func (e *RichEditor) posVert(dir int) DocPos {
	lay := e.buildLayout(e.theme())
	ln, cell, ok := e.caretLineFor(lay, e.Caret().Get())
	if !ok {
		return e.Caret().Get()
	}
	var stops []reLine
	idx := -1
	for _, s := range lay.lines {
		if !s.hasStops {
			continue
		}
		if s.y == ln.y && s.blockIdx == ln.blockIdx && s.startOff == ln.startOff {
			idx = len(stops)
		}
		stops = append(stops, s)
	}
	ni := idx + dir
	if idx < 0 || ni < 0 || ni >= len(stops) {
		return e.Caret().Get()
	}
	target := stops[ni]
	x := ln.cellX[cell]
	best, bestDx := 0, 1<<30
	for k, gx := range target.cellX {
		if dx := reAbsInt(x - gx); dx < bestDx {
			bestDx, best = dx, k
		}
	}
	return DocPos{target.blockIdx, target.startOff + best}
}

// reClamp constrains v to [lo, hi].
func reClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// reAbsInt is the absolute value of v.
func reAbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// normalizeSel returns sel with Start <= End in document order.
func normalizeSel(sel DocSelection) DocSelection {
	if posLess(sel.End, sel.Start) {
		sel.Start, sel.End = sel.End, sel.Start
	}
	return sel
}

// posLess reports whether a precedes b in document order.
func posLess(a, b DocPos) bool {
	if a.Block != b.Block {
		return a.Block < b.Block
	}
	return a.Off < b.Off
}
