// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-richdoc/richdoc"
)

// styleBits is the set of inline styles a single rune can carry. It is a bitmask
// so the editor can toggle one axis (bold, italic, ...) without disturbing the
// others, and so the flat editing model stays a simple []styledRune.
type styleBits uint8

const (
	styBold   styleBits = 1 << iota // richdoc.Strong
	styItalic                       // richdoc.Emph
	styStrike                       // richdoc.Strikethrough
	styCode                         // richdoc.Code (exclusive: see wrapStyle)
)

// inlAtom classifies a non-text inline that occupies exactly one caret cell:
// images, inline math, hard line breaks and raw passthroughs have no editable
// runes, so the flat model represents each as a single styledRune carrying the
// original inline in payload for a loss-free rebuild.
type inlAtom uint8

const (
	atomNone      inlAtom = iota // an ordinary text rune
	atomImage                    // richdoc.Image
	atomMath                     // richdoc.Math
	atomLineBreak                // richdoc.LineBreak
	atomRaw                      // richdoc.RawInline
)

// objReplacement is the sentinel rune a non-text atom occupies in the flat rune
// stream, so offset arithmetic treats an image or a hard break as one position.
const objReplacement = '￼'

// styledRune is one caret cell of a block's editable content: a rune plus the
// inline styling in force at that position. For a text rune atom is atomNone and
// r is the character; for a non-text inline atom names its kind, r is
// [objReplacement] and payload is the exact inline to re-emit.
type styledRune struct {
	r       rune
	style   styleBits
	link    string
	title   string
	atom    inlAtom
	payload richdoc.Inline
}

// flattenInlines turns a rich inline tree into the flat per-rune model. Nested
// Strong/Emph/Strikethrough fold into the style mask, a Link folds into the
// link/title fields, and Code / non-text inlines become styled atoms. The
// inverse is [rebuildInlines].
func flattenInlines(inlines []richdoc.Inline) []styledRune {
	var out []styledRune
	var rec func(inls []richdoc.Inline, st styleBits, link, title string)
	rec = func(inls []richdoc.Inline, st styleBits, link, title string) {
		for _, in := range inls {
			switch n := in.(type) {
			case richdoc.Text:
				for _, r := range n.Value {
					out = append(out, styledRune{r: r, style: st, link: link, title: title})
				}
			case richdoc.Code:
				for _, r := range n.Value {
					out = append(out, styledRune{r: r, style: st | styCode, link: link, title: title})
				}
			case richdoc.Strong:
				rec(n.Inlines, st|styBold, link, title)
			case richdoc.Emph:
				rec(n.Inlines, st|styItalic, link, title)
			case richdoc.Strikethrough:
				rec(n.Inlines, st|styStrike, link, title)
			case richdoc.Link:
				rec(n.Inlines, st, n.URL, n.Title)
			case richdoc.Image:
				out = append(out, styledRune{r: objReplacement, style: st, link: link, title: title, atom: atomImage, payload: n})
			case richdoc.Math:
				out = append(out, styledRune{r: objReplacement, style: st, link: link, title: title, atom: atomMath, payload: n})
			case richdoc.LineBreak:
				out = append(out, styledRune{r: objReplacement, style: st, link: link, title: title, atom: atomLineBreak, payload: n})
			case richdoc.RawInline:
				out = append(out, styledRune{r: objReplacement, style: st, link: link, title: title, atom: atomRaw, payload: n})
			}
		}
	}
	rec(inlines, 0, "", "")
	return out
}

// rebuildInlines is the inverse of [flattenInlines]: it coalesces the flat runes
// back into a canonical inline tree. Runs are first grouped by link (a non-empty
// link wraps its span in a richdoc.Link), then by style within each link span.
func rebuildInlines(rs []styledRune) []richdoc.Inline {
	var out []richdoc.Inline
	for i := 0; i < len(rs); {
		link, title := rs[i].link, rs[i].title
		j := i
		for j < len(rs) && rs[j].link == link && rs[j].title == title {
			j++
		}
		seg := rebuildStyledSpan(rs[i:j])
		if link != "" {
			out = append(out, richdoc.Link{URL: link, Title: title, Inlines: seg})
		} else {
			out = append(out, seg...)
		}
		i = j
	}
	return out
}

// rebuildStyledSpan rebuilds a link-uniform run of styled runes into inlines,
// grouping maximal runs of identical style (and emitting each atom verbatim).
func rebuildStyledSpan(rs []styledRune) []richdoc.Inline {
	var out []richdoc.Inline
	for i := 0; i < len(rs); {
		if rs[i].atom != atomNone {
			out = append(out, rs[i].payload)
			i++
			continue
		}
		st := rs[i].style
		var sb strings.Builder
		j := i
		for j < len(rs) && rs[j].atom == atomNone && rs[j].style == st {
			sb.WriteRune(rs[j].r)
			j++
		}
		out = append(out, wrapStyle(sb.String(), st))
		i = j
	}
	return out
}

// wrapStyle wraps literal text s in the inline nodes named by st, in the
// canonical nesting order Strong(Emph(Strikethrough(Text))). styCode is
// exclusive — richdoc's Code span carries no nested styling — so a code run
// becomes a bare richdoc.Code and any other bits on it are dropped.
func wrapStyle(s string, st styleBits) richdoc.Inline {
	if st&styCode != 0 {
		return richdoc.Code{Value: s}
	}
	var inl richdoc.Inline = richdoc.Text{Value: s}
	if st&styStrike != 0 {
		inl = richdoc.Strikethrough{Inlines: []richdoc.Inline{inl}}
	}
	if st&styItalic != 0 {
		inl = richdoc.Emph{Inlines: []richdoc.Inline{inl}}
	}
	if st&styBold != 0 {
		inl = richdoc.Strong{Inlines: []richdoc.Inline{inl}}
	}
	return inl
}

// runesToText joins the flat runes into a plain string (used for CodeBlock text,
// whose newlines live as ordinary runes in the flat model).
func runesToText(rs []styledRune) string {
	var sb strings.Builder
	for _, r := range rs {
		sb.WriteRune(r.r)
	}
	return sb.String()
}

// blockContent returns block b's editable content as flat runes, and whether the
// block accepts inline text editing. Atomic blocks (Table, ThematicBreak,
// MathBlock, RawBlock) return (nil, false): the caret may rest on them for
// block-level commands, but typing is a no-op. A List or BlockQuote delegates to
// its first item's / first block's paragraph — the primary paragraph — which is
// the one caret cell the v1 editor exposes for those containers.
func blockContent(b richdoc.Block) (rs []styledRune, editable bool) {
	switch n := b.(type) {
	case richdoc.Paragraph:
		return flattenInlines(n.Inlines), true
	case richdoc.Heading:
		return flattenInlines(n.Inlines), true
	case richdoc.CodeBlock:
		return textToRunes(n.Text), true
	case richdoc.List:
		if p, ok := primaryParagraph(n); ok {
			return flattenInlines(p.Inlines), true
		}
		return nil, false
	case richdoc.BlockQuote:
		if p, ok := firstParagraph(n.Blocks); ok {
			return flattenInlines(p.Inlines), true
		}
		return nil, false
	}
	return nil, false
}

// isCodeBlock reports whether b is a CodeBlock, whose editable content is raw
// text (newlines are ordinary runes) rather than styled inlines.
func isCodeBlock(b richdoc.Block) bool {
	_, ok := b.(richdoc.CodeBlock)
	return ok
}

// textToRunes maps a plain string to atomless, unstyled runes — the flat form of
// a CodeBlock's verbatim text.
func textToRunes(s string) []styledRune {
	out := make([]styledRune, 0, len(s))
	for _, r := range s {
		out = append(out, styledRune{r: r})
	}
	return out
}

// setBlockContent returns a copy of b whose editable content is rs, preserving
// everything the flat model does not carry (heading level, code language, the
// non-primary items/blocks of a list/quote). Atomic blocks are returned
// unchanged.
func setBlockContent(b richdoc.Block, rs []styledRune) richdoc.Block {
	switch n := b.(type) {
	case richdoc.Paragraph:
		return richdoc.Paragraph{Inlines: rebuildInlines(rs)}
	case richdoc.Heading:
		return richdoc.Heading{Level: n.Level, Inlines: rebuildInlines(rs)}
	case richdoc.CodeBlock:
		return richdoc.CodeBlock{Language: n.Language, Text: runesToText(rs)}
	case richdoc.List:
		return setPrimaryParagraph(n, rebuildInlines(rs))
	case richdoc.BlockQuote:
		return setFirstParagraph(n, rebuildInlines(rs))
	}
	return b
}

// primaryParagraph returns the paragraph the caret edits inside a list: the
// first block of the first item, when that block is a Paragraph.
func primaryParagraph(l richdoc.List) (richdoc.Paragraph, bool) {
	if len(l.Items) == 0 {
		return richdoc.Paragraph{}, false
	}
	return firstParagraph(l.Items[0].Blocks)
}

// setPrimaryParagraph returns l with its primary paragraph's inlines replaced.
func setPrimaryParagraph(l richdoc.List, inlines []richdoc.Inline) richdoc.List {
	if len(l.Items) == 0 {
		l.Items = []richdoc.ListItem{{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: inlines}}}}
		return l
	}
	items := make([]richdoc.ListItem, len(l.Items))
	copy(items, l.Items)
	items[0].Blocks = setFirstParagraphSlice(items[0].Blocks, inlines)
	l.Items = items
	return l
}

// firstParagraph returns the first Paragraph in blocks.
func firstParagraph(blocks []richdoc.Block) (richdoc.Paragraph, bool) {
	for _, b := range blocks {
		if p, ok := b.(richdoc.Paragraph); ok {
			return p, true
		}
	}
	return richdoc.Paragraph{}, false
}

// setFirstParagraph returns a BlockQuote with its first paragraph replaced.
func setFirstParagraph(q richdoc.BlockQuote, inlines []richdoc.Inline) richdoc.BlockQuote {
	q.Blocks = setFirstParagraphSlice(q.Blocks, inlines)
	return q
}

// setFirstParagraphSlice replaces the inlines of the first Paragraph in blocks,
// or prepends one when there is none, returning a fresh slice.
func setFirstParagraphSlice(blocks []richdoc.Block, inlines []richdoc.Inline) []richdoc.Block {
	out := make([]richdoc.Block, len(blocks))
	copy(out, blocks)
	for i, b := range out {
		if _, ok := b.(richdoc.Paragraph); ok {
			out[i] = richdoc.Paragraph{Inlines: inlines}
			return out
		}
	}
	return append([]richdoc.Block{richdoc.Paragraph{Inlines: inlines}}, out...)
}
