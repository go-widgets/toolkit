// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-richdoc/richdoc"

// BlockKind names the block types SetBlockType can convert the caret's block to:
// a paragraph, one of six heading levels, a code block or a block quote. The
// heading levels are contiguous so BlockH1+n is level n+1.
type BlockKind int

const (
	BlockParagraph BlockKind = iota
	BlockH1
	BlockH2
	BlockH3
	BlockH4
	BlockH5
	BlockH6
	BlockCodeKind
	BlockQuoteKind
)

// editBlocks returns a deep, independently-mutable copy of the current document's
// blocks, so an edit can splice values freely before committing a fresh document.
func (e *RichEditor) editBlocks() []richdoc.Block {
	return richdoc.Clone(e.docValue()).Blocks
}

// commit publishes a new document with the given blocks, clamps + parks the
// caret, and clears the selection. A fresh *richdoc.Document guarantees Doc()
// subscribers are notified.
func (e *RichEditor) commit(blocks []richdoc.Block, caret DocPos) {
	d := &richdoc.Document{Blocks: blocks, Meta: e.docValue().Meta}
	caret = clampPosIn(d, caret)
	e.Doc().Set(d)
	e.Caret().Set(caret)
	e.anchor = caret
	e.ClearSelection()
}

// InsertText inserts s at the caret, one rune at a time; a '\n' splits the block
// (or, inside a code block, inserts a literal newline). It is the programmatic
// counterpart of typing.
func (e *RichEditor) InsertText(s string) {
	if s == "" {
		return
	}
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if len(blocks) == 0 {
		// Typing into an empty document seeds the first paragraph.
		blocks = append(blocks, richdoc.Paragraph{})
		c = DocPos{}
	}
	if c.Block < 0 || c.Block >= len(blocks) {
		return
	}
	for _, r := range s {
		if r == '\n' && !isCodeBlock(blocks[c.Block]) {
			blocks, c = e.splitBlocksAt(blocks, c)
			continue
		}
		blocks, c = e.insertRuneAt(blocks, c, r)
	}
	e.commit(blocks, c)
}

// insertRuneAt inserts one rune at c, inheriting the caret's inline style (or the
// pending toggled style), and returns the updated blocks + advanced caret.
// Atomic blocks reject text (no-op).
func (e *RichEditor) insertRuneAt(blocks []richdoc.Block, c DocPos, r rune) ([]richdoc.Block, DocPos) {
	rs, editable := blockContent(blocks[c.Block])
	if !editable {
		return blocks, c
	}
	var cell styledRune
	if isCodeBlock(blocks[c.Block]) {
		cell = styledRune{r: r}
	} else {
		cell = styledRune{r: r, style: e.styleAt(rs, c.Off)}
	}
	rs = insertCell(rs, c.Off, cell)
	blocks[c.Block] = setBlockContent(blocks[c.Block], rs)
	return blocks, DocPos{c.Block, c.Off + 1}
}

// splitBlocksAt splits the caret's block at c. A code block gains a literal
// newline; a text block is cut in two — its left half keeps the block's kind, its
// right half becomes a new Paragraph — and an atomic block simply gains an empty
// Paragraph after it. The caret lands at the start of the new block.
func (e *RichEditor) splitBlocksAt(blocks []richdoc.Block, c DocPos) ([]richdoc.Block, DocPos) {
	b := blocks[c.Block]
	if isCodeBlock(b) {
		rs, _ := blockContent(b)
		rs = insertCell(rs, c.Off, styledRune{r: '\n'})
		blocks[c.Block] = setBlockContent(b, rs)
		return blocks, DocPos{c.Block, c.Off + 1}
	}
	rs, editable := blockContent(b)
	if !editable {
		return insertBlock(blocks, c.Block+1, richdoc.Paragraph{}), DocPos{c.Block + 1, 0}
	}
	off := reClamp(c.Off, 0, len(rs))
	left := append([]styledRune{}, rs[:off]...)
	right := append([]styledRune{}, rs[off:]...)
	blocks[c.Block] = setBlockContent(b, left)
	newPara := setBlockContent(richdoc.Paragraph{}, right)
	return insertBlock(blocks, c.Block+1, newPara), DocPos{c.Block + 1, 0}
}

// splitBlock is the Enter handler over the current caret.
func (e *RichEditor) splitBlock() {
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(blocks) {
		e.commit(append(blocks, richdoc.Paragraph{}), DocPos{})
		return
	}
	blocks, c = e.splitBlocksAt(blocks, c)
	e.commit(blocks, c)
}

// backspace deletes the cell before the caret, or merges the caret's block into
// the previous one when the caret is at the block start.
func (e *RichEditor) backspace() {
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(blocks) {
		return
	}
	if c.Off > 0 {
		rs, _ := blockContent(blocks[c.Block])
		rs = removeCell(rs, c.Off-1)
		blocks[c.Block] = setBlockContent(blocks[c.Block], rs)
		e.commit(blocks, DocPos{c.Block, c.Off - 1})
		return
	}
	if c.Block == 0 {
		return
	}
	prev, cur := blocks[c.Block-1], blocks[c.Block]
	prevRs, prevEd := blockContent(prev)
	curRs, curEd := blockContent(cur)
	switch {
	case prevEd && curEd:
		merged := append(append([]styledRune{}, prevRs...), curRs...)
		blocks[c.Block-1] = setBlockContent(prev, merged)
		blocks = removeBlock(blocks, c.Block)
		e.commit(blocks, DocPos{c.Block - 1, len(prevRs)})
	case !prevEd:
		blocks = removeBlock(blocks, c.Block-1)
		e.commit(blocks, DocPos{c.Block - 1, 0})
	default:
		blocks = removeBlock(blocks, c.Block)
		e.commit(blocks, DocPos{c.Block - 1, len(prevRs)})
	}
}

// deleteForward deletes the cell at the caret, or merges the next block into the
// caret's block when the caret is at the block end.
func (e *RichEditor) deleteForward() {
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(blocks) {
		return
	}
	rs, editable := blockContent(blocks[c.Block])
	if editable && c.Off < len(rs) {
		rs = removeCell(rs, c.Off)
		blocks[c.Block] = setBlockContent(blocks[c.Block], rs)
		e.commit(blocks, c)
		return
	}
	if c.Block >= len(blocks)-1 {
		return
	}
	next := blocks[c.Block+1]
	nextRs, nextEd := blockContent(next)
	switch {
	case editable && nextEd:
		merged := append(append([]styledRune{}, rs...), nextRs...)
		blocks[c.Block] = setBlockContent(blocks[c.Block], merged)
		blocks = removeBlock(blocks, c.Block+1)
		e.commit(blocks, c)
	case !nextEd:
		blocks = removeBlock(blocks, c.Block+1)
		e.commit(blocks, c)
	default:
		blocks = removeBlock(blocks, c.Block)
		e.commit(blocks, DocPos{c.Block, 0})
	}
}

// DeleteSelection removes the selected range and parks the caret at its start.
// No-op on an empty selection.
func (e *RichEditor) DeleteSelection() {
	sel := normalizeSel(e.Selection().Get())
	if sel.IsEmpty() {
		return
	}
	blocks := e.editBlocks()
	s, en := sel.Start, sel.End
	if s.Block == en.Block {
		if rs, editable := blockContent(blocks[s.Block]); editable {
			a := reClamp(s.Off, 0, len(rs))
			b := reClamp(en.Off, 0, len(rs))
			rs = append(append([]styledRune{}, rs[:a]...), rs[b:]...)
			blocks[s.Block] = setBlockContent(blocks[s.Block], rs)
		}
		e.commit(blocks, s)
		return
	}
	startRs, se := blockContent(blocks[s.Block])
	endRs, ee := blockContent(blocks[en.Block])
	switch {
	case se && ee:
		a := reClamp(s.Off, 0, len(startRs))
		b := reClamp(en.Off, 0, len(endRs))
		merged := append(append([]styledRune{}, startRs[:a]...), endRs[b:]...)
		blocks[s.Block] = setBlockContent(blocks[s.Block], merged)
	case se:
		a := reClamp(s.Off, 0, len(startRs))
		blocks[s.Block] = setBlockContent(blocks[s.Block], startRs[:a])
	}
	blocks = append(blocks[:s.Block+1], blocks[en.Block+1:]...)
	e.commit(blocks, s)
}

// --- inline formatting verbs ---------------------------------------------

// ToggleStrong toggles bold over the selection, or arms bold for the next typed
// rune when the caret is collapsed.
func (e *RichEditor) ToggleStrong() { e.toggleInlineStyle(styBold) }

// ToggleEmph toggles italic (emphasis).
func (e *RichEditor) ToggleEmph() { e.toggleInlineStyle(styItalic) }

// ToggleStrikethrough toggles a strike-out.
func (e *RichEditor) ToggleStrikethrough() { e.toggleInlineStyle(styStrike) }

// ToggleCode toggles an inline code span (exclusive of other styles on rebuild).
func (e *RichEditor) ToggleCode() { e.toggleInlineStyle(styCode) }

// toggleInlineStyle flips one style bit over the selection (adding it unless the
// whole selection already carries it, in which case it is removed), preserving
// the selection so verbs compose. With no selection it arms the pending style for
// the next typed rune.
func (e *RichEditor) toggleInlineStyle(bit styleBits) {
	if !e.HasSelection() {
		e.pending = e.styleAtCaret() ^ bit
		e.pendingActive = true
		return
	}
	sel := normalizeSel(e.Selection().Get())
	blocks := e.editBlocks()
	allSet := true
	e.forEachSelectedRange(blocks, sel, func(rs []styledRune, a, b int) {
		for i := a; i < b; i++ {
			if rs[i].style&bit == 0 {
				allSet = false
			}
		}
	})
	e.forEachSelectedRange(blocks, sel, func(rs []styledRune, a, b int) {
		for i := a; i < b; i++ {
			if allSet {
				rs[i].style &^= bit
			} else {
				rs[i].style |= bit
			}
		}
	})
	d := &richdoc.Document{Blocks: blocks, Meta: e.docValue().Meta}
	e.Doc().Set(d)
	e.Caret().Set(clampPosIn(d, e.Caret().Get()))
	e.Selection().Set(DocSelection{Start: clampPosIn(d, sel.Start), End: clampPosIn(d, sel.End)})
}

// forEachSelectedRange calls fn with the editable content slice + [a,b) cell
// range of each block the (normalised) selection spans, rebuilding the block from
// the possibly-mutated slice after fn returns.
func (e *RichEditor) forEachSelectedRange(blocks []richdoc.Block, sel DocSelection, fn func(rs []styledRune, a, b int)) {
	for bi := sel.Start.Block; bi <= sel.End.Block && bi < len(blocks); bi++ {
		rs, editable := blockContent(blocks[bi])
		if !editable {
			continue
		}
		a, b := 0, len(rs)
		if bi == sel.Start.Block {
			a = reClamp(sel.Start.Off, 0, len(rs))
		}
		if bi == sel.End.Block {
			b = reClamp(sel.End.Off, 0, len(rs))
		}
		fn(rs, a, b)
		blocks[bi] = setBlockContent(blocks[bi], rs)
	}
}

// styleAt is the style a rune typed at off should carry: the pending toggled
// style if one is armed, else the neighbouring cell's style.
func (e *RichEditor) styleAt(rs []styledRune, off int) styleBits {
	if e.pendingActive {
		return e.pending
	}
	return styleOfNeighbor(rs, off)
}

// styleAtCaret is the style in force at the collapsed caret.
func (e *RichEditor) styleAtCaret() styleBits {
	d := e.docValue()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(d.Blocks) {
		return 0
	}
	rs, _ := blockContent(d.Blocks[c.Block])
	return styleOfNeighbor(rs, c.Off)
}

// styleOfNeighbor returns the style of the cell left of off (or right of it when
// off is at the block start), or 0 when the block is empty.
func styleOfNeighbor(rs []styledRune, off int) styleBits {
	if off > 0 && off-1 < len(rs) {
		return rs[off-1].style
	}
	if off >= 0 && off < len(rs) {
		return rs[off].style
	}
	return 0
}

// --- block verbs ----------------------------------------------------------

// SetBlockType converts the caret's top-level block to kind, preserving its
// inline content (a conversion to code flattens styling to plain text).
func (e *RichEditor) SetBlockType(kind BlockKind) {
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(blocks) {
		return
	}
	inlines := e.blockInlines(blocks[c.Block])
	var nb richdoc.Block
	switch {
	case kind == BlockParagraph:
		nb = richdoc.Paragraph{Inlines: inlines}
	case kind >= BlockH1 && kind <= BlockH6:
		nb = richdoc.Heading{Level: int(kind-BlockH1) + 1, Inlines: inlines}
	case kind == BlockCodeKind:
		nb = richdoc.CodeBlock{Text: inlinesPlain(inlines)}
	case kind == BlockQuoteKind:
		nb = richdoc.BlockQuote{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: inlines}}}
	default:
		return
	}
	blocks[c.Block] = nb
	e.commit(blocks, DocPos{c.Block, reClamp(c.Off, 0, blockLen(nb))})
}

// blockInlines is the caret block's editable content rebuilt into inlines.
func (e *RichEditor) blockInlines(b richdoc.Block) []richdoc.Inline {
	rs, _ := blockContent(b)
	return rebuildInlines(rs)
}

// ToggleList wraps the caret's block into a single-item list, converts an
// existing list between bullet/numbered, or (when the ordered-ness already
// matches) unwraps the list back into its items' blocks.
func (e *RichEditor) ToggleList(ordered bool) {
	blocks := e.editBlocks()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(blocks) {
		return
	}
	if l, ok := blocks[c.Block].(richdoc.List); ok {
		if l.Ordered == ordered {
			var flat []richdoc.Block
			for _, it := range l.Items {
				if len(it.Blocks) == 0 {
					flat = append(flat, richdoc.Paragraph{})
				} else {
					flat = append(flat, it.Blocks...)
				}
			}
			if len(flat) == 0 {
				flat = []richdoc.Block{richdoc.Paragraph{}}
			}
			out := append([]richdoc.Block{}, blocks[:c.Block]...)
			out = append(out, flat...)
			out = append(out, blocks[c.Block+1:]...)
			e.commit(out, DocPos{c.Block, 0})
			return
		}
		l.Ordered = ordered
		blocks[c.Block] = l
		e.commit(blocks, c)
		return
	}
	item := richdoc.ListItem{Blocks: []richdoc.Block{blocks[c.Block]}}
	blocks[c.Block] = richdoc.List{Ordered: ordered, Start: 1, Items: []richdoc.ListItem{item}}
	e.commit(blocks, DocPos{c.Block, reClamp(c.Off, 0, blockLen(blocks[c.Block]))})
}

// --- caret model helpers --------------------------------------------------

// posRight is the caret one cell right, crossing into the next block at a block
// end.
func (e *RichEditor) posRight() DocPos {
	d := e.docValue()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(d.Blocks) {
		return c
	}
	if c.Off < blockLen(d.Blocks[c.Block]) {
		return DocPos{c.Block, c.Off + 1}
	}
	if c.Block < len(d.Blocks)-1 {
		return DocPos{c.Block + 1, 0}
	}
	return c
}

// posLeft is the caret one cell left, crossing into the previous block at a block
// start.
func (e *RichEditor) posLeft() DocPos {
	d := e.docValue()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(d.Blocks) {
		return c
	}
	if c.Off > 0 {
		return DocPos{c.Block, c.Off - 1}
	}
	if c.Block > 0 {
		return DocPos{c.Block - 1, blockLen(d.Blocks[c.Block-1])}
	}
	return c
}

// --- small slice/model helpers -------------------------------------------

// blockLen is the number of editable caret cells in b (0 for atomic blocks).
func blockLen(b richdoc.Block) int {
	rs, _ := blockContent(b)
	return len(rs)
}

// clampPosIn constrains pos to a valid cell of d.
func clampPosIn(d *richdoc.Document, pos DocPos) DocPos {
	if len(d.Blocks) == 0 {
		return DocPos{}
	}
	bi := reClamp(pos.Block, 0, len(d.Blocks)-1)
	return DocPos{bi, reClamp(pos.Off, 0, blockLen(d.Blocks[bi]))}
}

// insertCell splices c into rs at off.
func insertCell(rs []styledRune, off int, c styledRune) []styledRune {
	off = reClamp(off, 0, len(rs))
	out := make([]styledRune, 0, len(rs)+1)
	out = append(out, rs[:off]...)
	out = append(out, c)
	out = append(out, rs[off:]...)
	return out
}

// removeCell drops the cell at off (a no-op when out of range).
func removeCell(rs []styledRune, off int) []styledRune {
	if off < 0 || off >= len(rs) {
		return rs
	}
	out := make([]styledRune, 0, len(rs)-1)
	out = append(out, rs[:off]...)
	out = append(out, rs[off+1:]...)
	return out
}

// insertBlock splices blk into blocks at idx.
func insertBlock(blocks []richdoc.Block, idx int, blk richdoc.Block) []richdoc.Block {
	idx = reClamp(idx, 0, len(blocks))
	out := make([]richdoc.Block, 0, len(blocks)+1)
	out = append(out, blocks[:idx]...)
	out = append(out, blk)
	out = append(out, blocks[idx:]...)
	return out
}

// removeBlock drops the block at idx.
func removeBlock(blocks []richdoc.Block, idx int) []richdoc.Block {
	if idx < 0 || idx >= len(blocks) {
		return blocks
	}
	out := make([]richdoc.Block, 0, len(blocks)-1)
	out = append(out, blocks[:idx]...)
	out = append(out, blocks[idx+1:]...)
	return out
}

// inlinesPlain is the plain-text of inlines (used when flattening to a code
// block).
func inlinesPlain(inlines []richdoc.Inline) string {
	return richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: inlines}}})
}
