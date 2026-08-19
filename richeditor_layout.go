// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-richdoc/richdoc"
)

// reLayout is a full formatted layout of the document: an ordered list of visual
// lines (each carrying its caret cells) plus the block chrome painted beneath the
// text (code boxes, quote rules, table grids, rules, image/math placeholders and
// list markers). It is recomputed from the model + bounds whenever the editor
// draws, hit-tests or moves the caret, so it always reflects the current
// document and width. Geometry is theme-independent; only run/chrome colours read
// the theme, so a geometry-only caller may pass any theme.
type reLayout struct {
	lines  []reLine
	chrome []reChrome
	height int // total content height in device pixels
}

// reLine is one visual line: a horizontal strip of styled runs. cellX holds the
// device x of every caret gap (len == number of caret cells + 1), so the caret
// position and hit-testing are exact and share the layout's own metrics. A line
// with hasStops == false is pure decoration (a non-primary list item, a table
// cell) that the caret skips.
type reLine struct {
	y, h     int
	textY    int
	blockIdx int
	hasStops bool
	startOff int
	cellX    []int
	runs     []reRun
}

// nCells is how many caret cells the line holds.
func (l reLine) nCells() int { return len(l.cellX) - 1 }

// reRun is one drawable run of uniform styling on a line.
type reRun struct {
	text      string
	x         int
	font      Font
	ink       RGBA
	underline bool
	strike    bool
}

// reChrome is a block-level paint under the text: a fill (stroke false, text
// ""), a stroke box (stroke true), or a text label (text != "", drawn at r.X,r.Y
// in font/ink) — used for code bands, quote rules, table grids, rules, image
// frames and list markers.
type reChrome struct {
	r      Rect
	c      RGBA
	stroke bool
	text   string
	font   Font
}

// fontKey identifies a resolved face by style bits and heading level, so a
// layout pass reuses one synthetic-bold/italic wrapper per style rather than
// allocating one per rune.
type fontKey struct {
	style styleBits
	level int
}

// layout constants, all metric-scaled so the editor stays crisp at any DPI.
func rePadX() int        { return scaled(6) }
func rePadTop() int      { return scaled(6) }
func reBlockGap() int    { return scaled(6) }
func reLineGap() int     { return scaled(4) }
func reListIndent() int  { return scaled(16) }
func reQuoteIndent() int { return scaled(14) }
func reQuoteRuleW() int  { return scaled(3) }
func reCodePad() int     { return scaled(6) }
func reBoxPad() int      { return scaled(3) }

// headingFactor is the size multiplier (num/den) applied to the base face for a
// heading of the given level: H1 is largest, H5/H6 fall back to body size (still
// bold), so the type scale reads as a hierarchy.
func headingFactor(level int) (num, den int) {
	switch level {
	case 1:
		return 2, 1
	case 2:
		return 7, 4
	case 3:
		return 3, 2
	case 4:
		return 5, 4
	default:
		return 1, 1
	}
}

// buildLayout lays the whole document out at the editor's current bounds width.
func (e *RichEditor) buildLayout(theme *Theme) reLayout {
	e.fontCache = map[fontKey]Font{}
	r := e.Bounds()
	b := &reBuilder{e: e, theme: theme}
	b.left = r.X + rePadX()
	b.right = r.X + r.W - rePadX() - e.scrollbarReserve()
	b.y = r.Y + rePadTop()
	doc := e.docValue()
	for i, blk := range doc.Blocks {
		b.layoutBlock(i, blk, b.left)
		b.y += reBlockGap()
	}
	if len(doc.Blocks) == 0 {
		// An empty document still offers a caret home on block 0, so a click or a
		// keystroke lands somewhere InsertText can seed a paragraph.
		b.emitLine(0, b.left, nil, 0, true, 0)
	}
	return reLayout{lines: b.lines, chrome: b.chrome, height: b.y - r.Y}
}

// reBuilder accumulates lines + chrome while walking the document top-to-bottom.
type reBuilder struct {
	e           *RichEditor
	theme       *Theme
	left, right int
	y           int
	lines       []reLine
	chrome      []reChrome
}

// layoutBlock dispatches one top-level block to its specific layout routine.
func (b *reBuilder) layoutBlock(idx int, blk richdoc.Block, indentX int) {
	switch n := blk.(type) {
	case richdoc.Heading:
		b.layoutInlines(idx, flattenInlines(n.Inlines), clampLevel(n.Level), indentX, true)
	case richdoc.Paragraph:
		b.layoutInlines(idx, flattenInlines(n.Inlines), 0, indentX, true)
	case richdoc.CodeBlock:
		b.layoutCode(idx, n, indentX)
	case richdoc.List:
		b.layoutList(idx, n, indentX)
	case richdoc.BlockQuote:
		b.layoutQuote(idx, n, indentX)
	case richdoc.Table:
		b.layoutTable(idx, n, indentX)
	case richdoc.ThematicBreak:
		b.layoutThematic(idx, indentX)
	case richdoc.MathBlock:
		b.layoutBox(idx, n.TeX, indentX, b.theme.Accent)
	case richdoc.RawBlock:
		b.layoutBox(idx, n.Text, indentX, b.dim())
	}
}

// clampLevel pins a heading level to the 1..6 render range (0 is body).
func clampLevel(l int) int {
	if l < 1 {
		return 1
	}
	if l > 6 {
		return 6
	}
	return l
}

// layoutInlines wraps a run of styled cells to the available width and emits one
// reLine per visual line, tagging them with blockIdx/offsets when hasStops.
func (b *reBuilder) layoutInlines(idx int, rs []styledRune, level, indentX int, hasStops bool) {
	avail := b.right - indentX
	segs := b.wrap(rs, level, avail)
	for _, seg := range segs {
		b.emitLine(idx, indentX, rs[seg[0]:seg[1]], seg[0], hasStops, level)
	}
	if len(rs) == 0 {
		b.emitLine(idx, indentX, nil, 0, hasStops, level)
	}
}

// wrap breaks rs into [start,end) visual-line spans that each fit avail pixels,
// breaking at the last space and honouring hard LineBreak atoms. A single cell
// wider than avail overflows on its own line rather than looping forever.
func (b *reBuilder) wrap(rs []styledRune, level, avail int) [][2]int {
	var out [][2]int
	i := 0
	for i < len(rs) {
		start := i
		curW := 0
		lastSpace := -1
		j := i
		for j < len(rs) {
			sr := rs[j]
			if sr.atom == atomLineBreak {
				j++
				break
			}
			w := b.e.cellWidth(sr, level)
			if curW+w > avail && j > start {
				if lastSpace >= start {
					j = lastSpace + 1
				}
				break
			}
			if sr.atom == atomNone && sr.r == ' ' {
				lastSpace = j
			}
			curW += w
			j++
		}
		out = append(out, [2]int{start, j})
		i = j
	}
	return out
}

// emitLine positions cells [rs] starting at indentX, builds their draw runs and
// caret cellX table, advances the vertical cursor and records the line.
func (b *reBuilder) emitLine(idx, indentX int, rs []styledRune, startOff int, hasStops bool, level int) {
	h := b.e.sizedFont(level).Height() + reLineGap()
	line := reLine{
		y: b.y, h: h, textY: b.y + reLineGap()/2,
		blockIdx: idx, hasStops: hasStops, startOff: startOff,
	}
	x := indentX
	cellX := []int{x}
	i := 0
	for i < len(rs) {
		sr := rs[i]
		if sr.atom != atomNone {
			w := b.e.cellWidth(sr, level)
			b.emitAtom(&line, sr, x, w, level)
			x += w
			cellX = append(cellX, x)
			i++
			continue
		}
		st, link := sr.style, sr.link
		var sb strings.Builder
		runX := x
		for i < len(rs) && rs[i].atom == atomNone && rs[i].style == st && rs[i].link == link {
			sb.WriteRune(rs[i].r)
			x += b.e.cellWidth(rs[i], level)
			cellX = append(cellX, x)
			i++
		}
		line.runs = append(line.runs, reRun{
			text: sb.String(), x: runX, font: b.e.fontFor(st, level),
			ink: b.inkFor(st, link), underline: link != "", strike: st&styStrike != 0,
		})
	}
	line.cellX = cellX
	b.lines = append(b.lines, line)
	b.y += h
}

// emitAtom draws a non-text inline as a placeholder: an image gets a framed box
// with its alt label, math its TeX, a raw inline its verbatim text; a hard break
// consumes its cell silently.
func (b *reBuilder) emitAtom(line *reLine, sr styledRune, x, w, level int) {
	switch sr.atom {
	case atomLineBreak:
		return
	case atomImage:
		img := sr.payload.(richdoc.Image)
		bh := b.e.sizedFont(level).Height() + 2*reBoxPad()
		b.chrome = append(b.chrome, reChrome{r: Rect{X: x, Y: line.textY - reBoxPad(), W: w, H: bh}, c: b.theme.Border, stroke: true})
		line.runs = append(line.runs, reRun{text: imageLabel(img), x: x + reBoxPad(), font: b.e.baseFont(), ink: b.theme.Accent})
	case atomMath:
		line.runs = append(line.runs, reRun{text: sr.payload.(richdoc.Math).TeX, x: x, font: b.e.baseFont(), ink: b.theme.Accent})
	case atomRaw:
		line.runs = append(line.runs, reRun{text: sr.payload.(richdoc.RawInline).Text, x: x, font: b.e.baseFont(), ink: b.dim()})
	}
}

// layoutCode lays a CodeBlock out as a monospace band, one visual line per source
// line, all caret-addressable (newlines are cells between lines).
func (b *reBuilder) layoutCode(idx int, cb richdoc.CodeBlock, indentX int) {
	textLines := strings.Split(cb.Text, "\n")
	top := b.y
	off := 0
	for _, tl := range textLines {
		b.emitLine(idx, indentX+reCodePad(), textToRunes(tl), off, true, 0)
		off += len([]rune(tl)) + 1
	}
	band := Rect{X: indentX, Y: top - reBoxPad(), W: b.right - indentX, H: b.y - top + reBoxPad()}
	b.chrome = append(b.chrome, reChrome{r: band, c: b.theme.SurfaceAlt})
}

// layoutList renders each item indented under a marker; only the primary
// paragraph (first block of the first item) is caret-addressable in v1.
func (b *reBuilder) layoutList(idx int, l richdoc.List, indentX int) {
	itemX := indentX + reListIndent()
	start := l.Start
	if start < 1 {
		start = 1
	}
	for ii, item := range l.Items {
		marker := "•"
		if l.Ordered {
			marker = strconv.Itoa(start+ii) + "."
		}
		markerY := b.y
		fpi := firstParagraphIndex(item.Blocks)
		for bi, blk := range item.Blocks {
			primary := ii == 0 && bi == fpi
			if p, ok := blk.(richdoc.Paragraph); ok {
				b.layoutInlines(idx, flattenInlines(p.Inlines), 0, itemX, primary)
			} else if sub, ok := blk.(richdoc.List); ok {
				b.layoutList(idx, sub, itemX)
			} else {
				b.layoutBlock(idx, blk, itemX)
			}
		}
		if len(item.Blocks) == 0 {
			b.emitLine(idx, itemX, nil, 0, ii == 0, 0)
		}
		b.chrome = append(b.chrome, reChrome{r: Rect{X: indentX, Y: markerY + reLineGap()/2}, c: b.theme.OnSurface, text: marker, font: b.e.baseFont()})
	}
}

// layoutQuote renders a blockquote's inner blocks indented behind a left rule;
// the first inner paragraph is caret-addressable.
func (b *reBuilder) layoutQuote(idx int, q richdoc.BlockQuote, indentX int) {
	inX := indentX + reQuoteIndent()
	top := b.y
	fpi := firstParagraphIndex(q.Blocks)
	for bi, blk := range q.Blocks {
		primary := bi == fpi
		if p, ok := blk.(richdoc.Paragraph); ok {
			b.layoutInlines(idx, flattenInlines(p.Inlines), 0, inX, primary)
		} else {
			b.layoutBlock(idx, blk, inX)
		}
	}
	if len(q.Blocks) == 0 {
		b.emitLine(idx, inX, nil, 0, true, 0)
	}
	b.chrome = append(b.chrome, reChrome{r: Rect{X: indentX, Y: top, W: reQuoteRuleW(), H: b.y - top}, c: b.theme.Accent})
}

// layoutTable renders a grid of cells with a header row; atomic (no inline caret
// cells) in v1 — cell editing is a documented gap. The first grid line becomes
// the block's caret rest.
func (b *reBuilder) layoutTable(idx int, t richdoc.Table, indentX int) {
	cols := tableCols(t)
	if cols == 0 {
		b.emitLine(idx, indentX, nil, 0, true, 0)
		return
	}
	colW := (b.right - indentX) / cols
	f := b.e.baseFont()
	rowH := f.Height() + 2*reLineGap()
	first := true
	drawRow := func(cells []richdoc.Cell, header bool) {
		for c := 0; c < cols; c++ {
			cx := indentX + c*colW
			ink := b.theme.OnSurface
			if header {
				ink = b.theme.Accent
			}
			var txt string
			if c < len(cells) {
				txt = richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: cells[c].Inlines}}})
			}
			line := reLine{y: b.y, h: rowH, textY: b.y + reLineGap(), blockIdx: idx, startOff: 0, cellX: []int{cx + reBoxPad()}}
			if first {
				line.hasStops = true
				first = false
			}
			line.runs = append(line.runs, reRun{text: txt, x: cx + reBoxPad(), font: f, ink: ink})
			b.lines = append(b.lines, line)
			b.chrome = append(b.chrome, reChrome{r: Rect{X: cx, Y: b.y, W: colW, H: rowH}, c: b.theme.Border, stroke: true})
		}
		b.y += rowH
	}
	if len(t.Header) > 0 {
		drawRow(t.Header, true)
	}
	for _, row := range t.Rows {
		drawRow(row, false)
	}
}

// layoutThematic renders a horizontal rule and gives the block a single caret
// rest at its left.
func (b *reBuilder) layoutThematic(idx, indentX int) {
	h := b.e.baseFont().Height() + reLineGap()
	midY := b.y + h/2
	b.chrome = append(b.chrome, reChrome{r: Rect{X: indentX, Y: midY, W: b.right - indentX, H: strokeWidth()}, c: b.theme.Border})
	b.lines = append(b.lines, reLine{y: b.y, h: h, textY: b.y, blockIdx: idx, hasStops: true, startOff: 0, cellX: []int{indentX}})
	b.y += h
}

// layoutBox renders a MathBlock or RawBlock in a framed monospace band (a
// documented placeholder for real math rendering) with a single caret rest.
func (b *reBuilder) layoutBox(idx int, text string, indentX int, ink RGBA) {
	f := b.e.baseFont()
	textLines := strings.Split(text, "\n")
	top := b.y
	for k, tl := range textLines {
		h := f.Height() + reLineGap()
		line := reLine{y: b.y, h: h, textY: b.y + reLineGap()/2, blockIdx: idx, startOff: 0, cellX: []int{indentX + reCodePad()}}
		if k == 0 {
			line.hasStops = true
		}
		line.runs = append(line.runs, reRun{text: tl, x: indentX + reCodePad(), font: f, ink: ink})
		b.lines = append(b.lines, line)
		b.y += h
	}
	band := Rect{X: indentX, Y: top - reBoxPad(), W: b.right - indentX, H: b.y - top + reBoxPad()}
	b.chrome = append(b.chrome, reChrome{r: band, c: b.theme.SurfaceAlt})
}

// inkFor is the text colour for a run: links in the accent, inline code dimmed to
// read as monospace, everything else the surface ink.
func (b *reBuilder) inkFor(st styleBits, link string) RGBA {
	if link != "" {
		return b.theme.Accent
	}
	if st&styCode != 0 {
		return b.dim()
	}
	return b.theme.OnSurface
}

// dim is the muted ink shared by code spans and raw passthroughs.
func (b *reBuilder) dim() RGBA { return dimInk(b.theme) }

// imageLabel is the placeholder caption for an image atom: its alt text, or a
// generic tag when the alt is empty.
func imageLabel(img richdoc.Image) string {
	if strings.TrimSpace(img.Alt) != "" {
		return "[" + img.Alt + "]"
	}
	return "[image]"
}

// firstParagraphIndex returns the index of the first Paragraph in blocks, or -1.
func firstParagraphIndex(blocks []richdoc.Block) int {
	for i, b := range blocks {
		if _, ok := b.(richdoc.Paragraph); ok {
			return i
		}
	}
	return -1
}

// tableCols is the column count of t: the widest of its header and rows.
func tableCols(t richdoc.Table) int {
	n := len(t.Header)
	for _, row := range t.Rows {
		if len(row) > n {
			n = len(row)
		}
	}
	return n
}
