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
//
// The v0.2.0 reference inlines add three more atoms. A [richdoc.Footnote] is
// always an atom — its body is block-level and is not edited inline in v1, so
// the flat model shows a superscript reference marker in its place. An
// inline-less (point) [richdoc.Anchor] and an inline-less [richdoc.CrossRef]
// are atoms too, because they carry no visible runes of their own; an Anchor or
// CrossRef that DOES wrap visible inlines is folded into styled cells (see
// [flattenInlines]) so the caret can enter them, exactly as a Link is.
type inlAtom uint8

const (
	atomNone      inlAtom = iota // an ordinary text rune
	atomImage                    // richdoc.Image
	atomMath                     // richdoc.Math
	atomLineBreak                // richdoc.LineBreak
	atomRaw                      // richdoc.RawInline
	atomFootnote                 // richdoc.Footnote (superscript reference marker)
	atomAnchor                   // point richdoc.Anchor (no inlines) — invisible target
	atomXRef                     // richdoc.CrossRef with no inlines — resolved label/cite
)

// refSpan is the identity of an enclosing [richdoc.CrossRef]: the target key it
// resolves to and whether it is a label reference or a citation. It is a
// comparable value so a run of cells inside one CrossRef groups by it on
// rebuild, the way a link span groups by URL/title.
type refSpan struct {
	target string
	kind   richdoc.RefKind
}

// objReplacement is the sentinel rune a non-text atom occupies in the flat rune
// stream, so offset arithmetic treats an image or a hard break as one position.
const objReplacement = '￼'

// styledRune is one caret cell of a block's editable content: a rune plus the
// inline styling in force at that position. For a text rune atom is atomNone and
// r is the character; for a non-text inline atom names its kind, r is
// [objReplacement] and payload is the exact inline to re-emit.
//
// A cell also records the reference spans enclosing it: anchor is the ID of an
// enclosing [richdoc.Anchor] (empty when none), and ref/refActive the identity
// of an enclosing [richdoc.CrossRef]. These mirror link/title and let a run of
// cells rebuild into the same Anchor/CrossRef wrapper it came from, so an Anchor
// or CrossRef with visible inlines survives an edit round-trip while its caret
// cells stay ordinary. fnNum is the 1-based footnote number assigned to an
// atomFootnote cell at layout time (0 in the editing model, where numbering is
// irrelevant).
type styledRune struct {
	r         rune
	style     styleBits
	link      string
	title     string
	anchor    string
	ref       refSpan
	refActive bool
	fnNum     int
	atom      inlAtom
	payload   richdoc.Inline
}

// flattenInlines turns a rich inline tree into the flat per-rune model. Nested
// Strong/Emph/Strikethrough fold into the style mask, a Link folds into the
// link/title fields, an Anchor/CrossRef with visible inlines folds into the
// anchor/ref span fields, and Code / non-text inlines (plus point Anchors,
// inline-less CrossRefs and Footnotes) become styled atoms. The inverse is
// [rebuildInlines].
func flattenInlines(inlines []richdoc.Inline) []styledRune {
	var out []styledRune
	var rec func(inls []richdoc.Inline, sc spanCtx)
	rec = func(inls []richdoc.Inline, sc spanCtx) {
		for _, in := range inls {
			switch n := in.(type) {
			case richdoc.Text:
				for _, r := range n.Value {
					out = append(out, sc.cell(r, 0))
				}
			case richdoc.Code:
				for _, r := range n.Value {
					out = append(out, sc.cell(r, styCode))
				}
			case richdoc.Strong:
				rec(n.Inlines, sc.withStyle(styBold))
			case richdoc.Emph:
				rec(n.Inlines, sc.withStyle(styItalic))
			case richdoc.Strikethrough:
				rec(n.Inlines, sc.withStyle(styStrike))
			case richdoc.Link:
				rec(n.Inlines, sc.withLink(n.URL, n.Title))
			case richdoc.Anchor:
				if len(n.Inlines) == 0 {
					out = append(out, sc.atom(n, atomAnchor))
				} else {
					rec(n.Inlines, sc.withAnchor(n.ID))
				}
			case richdoc.CrossRef:
				if len(n.Inlines) == 0 {
					out = append(out, sc.atom(n, atomXRef))
				} else {
					rec(n.Inlines, sc.withRef(refSpan{n.Target, n.Kind}))
				}
			case richdoc.Footnote:
				out = append(out, sc.atom(n, atomFootnote))
			case richdoc.Image:
				out = append(out, sc.atom(n, atomImage))
			case richdoc.Math:
				out = append(out, sc.atom(n, atomMath))
			case richdoc.LineBreak:
				out = append(out, sc.atom(n, atomLineBreak))
			case richdoc.RawInline:
				out = append(out, sc.atom(n, atomRaw))
			}
		}
	}
	rec(inlines, spanCtx{})
	return out
}

// spanCtx is the set of enclosing spans in force while flattening: the style
// mask, the current link/title, the current Anchor ID and CrossRef identity. It
// threads immutably through the recursion so each cell records exactly the
// wrappers it must rebuild into.
type spanCtx struct {
	style     styleBits
	link      string
	title     string
	anchor    string
	ref       refSpan
	refActive bool
}

// cell builds a text cell for rune r, OR-ing extra style bits onto the context.
func (sc spanCtx) cell(r rune, extra styleBits) styledRune {
	return styledRune{r: r, style: sc.style | extra, link: sc.link, title: sc.title, anchor: sc.anchor, ref: sc.ref, refActive: sc.refActive}
}

// atom builds a single-cell atom carrying inline in for a loss-free rebuild,
// tagged with the enclosing spans so it re-emits inside them.
func (sc spanCtx) atom(in richdoc.Inline, kind inlAtom) styledRune {
	return styledRune{r: objReplacement, style: sc.style, link: sc.link, title: sc.title, anchor: sc.anchor, ref: sc.ref, refActive: sc.refActive, atom: kind, payload: in}
}

// withStyle / withLink / withAnchor / withRef return the context extended with
// one more enclosing span.
func (sc spanCtx) withStyle(bit styleBits) spanCtx { sc.style |= bit; return sc }
func (sc spanCtx) withLink(url, title string) spanCtx {
	sc.link, sc.title = url, title
	return sc
}
func (sc spanCtx) withAnchor(id string) spanCtx { sc.anchor = id; return sc }
func (sc spanCtx) withRef(ref refSpan) spanCtx {
	sc.ref, sc.refActive = ref, true
	return sc
}

// rebuildInlines is the inverse of [flattenInlines]: it coalesces the flat runes
// back into a canonical inline tree. Cells are grouped from the outside in —
// first by anchor (a non-empty anchor wraps its span in a richdoc.Anchor), then
// by CrossRef identity, then by link, then by style — so every wrapper the
// flatten pass recorded is reconstructed in a canonical nesting order.
func rebuildInlines(rs []styledRune) []richdoc.Inline {
	return rebuildAnchorSpans(rs)
}

// rebuildAnchorSpans groups by the enclosing Anchor ID, wrapping each non-empty
// run in a richdoc.Anchor and delegating its content to [rebuildRefSpans].
func rebuildAnchorSpans(rs []styledRune) []richdoc.Inline {
	var out []richdoc.Inline
	for i := 0; i < len(rs); {
		anchor := rs[i].anchor
		j := i
		for j < len(rs) && rs[j].anchor == anchor {
			j++
		}
		seg := rebuildRefSpans(rs[i:j])
		if anchor != "" {
			out = append(out, richdoc.Anchor{ID: anchor, Inlines: seg})
		} else {
			out = append(out, seg...)
		}
		i = j
	}
	return out
}

// rebuildRefSpans groups by the enclosing CrossRef identity, wrapping each active
// run in a richdoc.CrossRef and delegating its content to [rebuildLinkSpans].
func rebuildRefSpans(rs []styledRune) []richdoc.Inline {
	var out []richdoc.Inline
	for i := 0; i < len(rs); {
		ref, active := rs[i].ref, rs[i].refActive
		j := i
		for j < len(rs) && rs[j].refActive == active && rs[j].ref == ref {
			j++
		}
		seg := rebuildLinkSpans(rs[i:j])
		if active {
			out = append(out, richdoc.CrossRef{Target: ref.target, Kind: ref.kind, Inlines: seg})
		} else {
			out = append(out, seg...)
		}
		i = j
	}
	return out
}

// rebuildLinkSpans groups by link (a non-empty link wraps its span in a
// richdoc.Link), then by style within each link span via [rebuildStyledSpan].
func rebuildLinkSpans(rs []styledRune) []richdoc.Inline {
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
