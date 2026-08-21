// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-richdoc/richdoc"

// InlineStyles reports which inline styles are in force at the caret (or across
// the whole selection). A style is Active when a collapsed caret sits in a run
// carrying it — or, with the toggle armed, when the next typed rune would carry
// it — and, for a non-empty selection, only when EVERY selected cell carries it
// (the same "all set" test [RichEditor.toggleInlineStyle] uses to decide whether
// a toggle adds or removes the style). It is the query a formatting toolbar reads
// to light its Bold / Italic / Strikethrough / Code buttons.
type InlineStyles struct {
	Strong        bool
	Emph          bool
	Strikethrough bool
	Code          bool
}

// ActiveInlineStyles returns the inline styles active at the caret or across the
// selection — see [InlineStyles]. It is read-only: it never mutates the document.
func (e *RichEditor) ActiveInlineStyles() InlineStyles {
	b := e.activeStyleBits()
	return InlineStyles{
		Strong:        b&styBold != 0,
		Emph:          b&styItalic != 0,
		Strikethrough: b&styStrike != 0,
		Code:          b&styCode != 0,
	}
}

// activeStyleBits is the style mask active at the caret/selection: for a
// non-empty selection the bits set on every selected cell (their intersection,
// or 0 when the selection covers no editable cell); for a collapsed caret the
// armed pending style if a toggle is pending, else the neighbouring cell's style.
func (e *RichEditor) activeStyleBits() styleBits {
	if e.HasSelection() {
		all := styBold | styItalic | styStrike | styCode
		count := 0
		e.forEachSelectedCell(func(sr styledRune) {
			all &= sr.style
			count++
		})
		if count == 0 {
			return 0
		}
		return all
	}
	if e.pendingActive {
		return e.pending
	}
	return e.styleAtCaret()
}

// forEachSelectedCell calls fn for each styled cell the (normalised) selection
// covers, reading the live document without mutating it — the read-only
// counterpart of [RichEditor.forEachSelectedRange].
func (e *RichEditor) forEachSelectedCell(fn func(sr styledRune)) {
	sel := normalizeSel(e.Selection().Get())
	blocks := e.docValue().Blocks
	for bi := sel.Start.Block; bi <= sel.End.Block && bi < len(blocks); bi++ {
		if bi < 0 {
			continue
		}
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
		for i := a; i < b; i++ {
			fn(rs[i])
		}
	}
}

// CurrentBlockKind reports the [BlockKind] of the caret's top-level block, so a
// toolbar can light the matching Paragraph / Heading / Quote / Code-block button.
// A heading maps to BlockH1..BlockH6 by its level (clamped to that range); a
// code block to BlockCodeKind, a block quote to BlockQuoteKind; everything else
// — an ordinary paragraph, a list (whose editable content is paragraph-like), an
// atomic block, or an out-of-range caret — reports BlockParagraph.
func (e *RichEditor) CurrentBlockKind() BlockKind {
	d := e.docValue()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(d.Blocks) {
		return BlockParagraph
	}
	return blockKindOf(d.Blocks[c.Block])
}

// blockKindOf classifies a block into the [BlockKind] SetBlockType would produce
// for it (a List and any other block read as BlockParagraph).
func blockKindOf(b richdoc.Block) BlockKind {
	switch n := b.(type) {
	case richdoc.Heading:
		lvl := reClamp(n.Level, 1, 6)
		return BlockH1 + BlockKind(lvl-1)
	case richdoc.CodeBlock:
		return BlockCodeKind
	case richdoc.BlockQuote:
		return BlockQuoteKind
	default:
		return BlockParagraph
	}
}

// CurrentListOrdered reports whether the caret's top-level block is a list, and
// if so whether it is ordered (numbered). isList is false — and ordered is then
// meaningless (false) — when the caret is not directly on a List block or the
// caret is out of range. A toolbar lights its bullet-list button when
// isList && !ordered and its numbered-list button when isList && ordered.
func (e *RichEditor) CurrentListOrdered() (ordered, isList bool) {
	d := e.docValue()
	c := e.Caret().Get()
	if c.Block < 0 || c.Block >= len(d.Blocks) {
		return false, false
	}
	if l, ok := d.Blocks[c.Block].(richdoc.List); ok {
		return l.Ordered, true
	}
	return false, false
}
