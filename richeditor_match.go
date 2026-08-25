// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// --- plain-text projection ------------------------------------------------
//
// A search runs over text, but the RichEditor edits a rich document (blocks of
// styled inline runs). BlockTexts projects that document to the exact flat form
// the caret is addressed in — one string per top-level block, each in the same
// rune coordinates a [DocPos].Off uses — so a match found in the projection maps
// straight back to a [DocSelection] the editor can highlight, with no coordinate
// drift. It is the rich-model counterpart of the `strings.Split(text, "\n")` a
// [CodeEditor] host feeds [FindMatches].

// BlockTexts is the editor's plain-text projection: one entry per top-level
// block, block i being the flattened editable content of Blocks[i] in the rune
// coordinates [DocPos].Off addresses (an inline atom — image, math, a hard break
// — occupies exactly one position, the object-replacement rune, so offsets stay
// aligned with the caret grid). A non-editable block (a table, a thematic break,
// a math/raw block) contributes an empty string, keeping every slice index equal
// to its block index.
//
// A host runs a search over it and maps the hits back to the rich model:
//
//	m, err := toolkit.FindMatches(e.BlockTexts(), query, opts)
//	e.SetMatchHighlights(toolkit.DocSelectionsFromMatches(m))
//
// because [FindMatches] reports each hit as a [Selection] whose line index is the
// block index and whose columns are offsets into that block — exactly the shape
// [DocSelectionsFromMatches] folds into [DocSelection] ranges.
func (e *RichEditor) BlockTexts() []string {
	blocks := e.docValue().Blocks
	out := make([]string, len(blocks))
	for i, b := range blocks {
		rs, _ := blockContent(b)
		out[i] = runesToText(rs)
	}
	return out
}

// DocSelectionFromMatch converts one [FindMatches] result — a [Selection] whose
// StartLine/EndLine are block indices and whose columns are offsets into
// [RichEditor.BlockTexts] — into the [DocSelection] that addresses the same rich
// content, so a match found over the projection highlights the cells it covers.
func DocSelectionFromMatch(m Selection) DocSelection {
	return DocSelection{
		Start: DocPos{Block: m.StartLine, Off: m.StartCol},
		End:   DocPos{Block: m.EndLine, Off: m.EndCol},
	}
}

// DocSelectionsFromMatches maps a whole [FindMatches] result set to the
// [DocSelection] ranges [RichEditor.SetMatchHighlights] takes, preserving order.
// It is the one call a find/replace host makes between running the search over
// [RichEditor.BlockTexts] and pushing the highlights back.
func DocSelectionsFromMatches(ms []Selection) []DocSelection {
	out := make([]DocSelection, len(ms))
	for i, m := range ms {
		out[i] = DocSelectionFromMatch(m)
	}
	return out
}

// --- match-highlight API --------------------------------------------------
//
// These mirror the CodeEditor match-highlight surface (see matchhighlight.go)
// adapted to the rich model's [DocSelection] ranges: a soft band behind every
// occurrence, a stronger boxed band for the current one, and a scroll that
// reveals a match. They are UI state a search host pushes (a find/replace panel
// runs the regexp via [FindMatches] over [RichEditor.BlockTexts] and calls
// these) — the editor itself never searches. The ranges are re-resolved to
// colours on every Draw, so a Set shows on the next paint with no explicit
// invalidation.

// SetMatchHighlights replaces the set of soft match highlights — the ranges of
// every occurrence, each painted with a faint accent-tinted band. Passing an
// empty (or nil) slice clears them. The current match (SetCurrentMatch) is
// independent and unaffected. Ranges may overlap and may repeat the current
// match; the bands simply stack (their tints are translucent), and an empty
// range in the set paints nothing.
func (e *RichEditor) SetMatchHighlights(ranges []DocSelection) {
	e.matchRanges = ranges
}

// MatchHighlights returns a copy of the current soft match ranges, in the order
// they were set, so a host mirrors the editor's highlight set without holding its
// own copy.
func (e *RichEditor) MatchHighlights() []DocSelection {
	return append([]DocSelection(nil), e.matchRanges...)
}

// SetCurrentMatch marks one range as the current match, painted with a stronger
// tint and an accent outline box on top of the soft highlights. An empty
// [DocSelection] (Start == End) clears the current-match emphasis without
// touching the soft highlight set.
func (e *RichEditor) SetCurrentMatch(sel DocSelection) {
	e.currentMatch = sel
}

// CurrentMatch returns the range currently emphasised, or the zero
// [DocSelection] when none is.
func (e *RichEditor) CurrentMatch() DocSelection { return e.currentMatch }

// ClearMatchHighlights removes both the soft highlight set and the current-match
// emphasis — the "search closed / query empty" reset.
func (e *RichEditor) ClearMatchHighlights() {
	e.matchRanges = nil
	e.currentMatch = DocSelection{}
}

// ScrollToMatch scrolls the editor vertically so sel's start line is visible,
// centring it in the viewport when it is currently out of view. A match already
// fully on screen is left where it is (no jump). It only moves the vertical
// scroll — the caret and the highlight set are the host's to set — and is a
// no-op when the range addresses no caret line (an out-of-range block) or the
// viewport has no room (a zero-height editor).
func (e *RichEditor) ScrollToMatch(sel DocSelection) {
	sel = normalizeSel(sel)
	lay := e.buildLayout(e.theme())
	ln, _, ok := e.caretLineFor(lay, sel.Start)
	if !ok {
		return
	}
	r := e.Bounds()
	if r.H <= 0 {
		return
	}
	top := ln.y - r.Y // content-space top of the match line
	bot := top + ln.h
	off := e.clampedScroll(lay)
	if top >= off && bot <= off+r.H {
		return // already fully visible
	}
	ns := top - (r.H-ln.h)/2 // centre the line in the viewport
	e.ScrollOffset().Set(reClamp(ns, 0, e.maxScroll(lay)))
}

// --- band geometry + painting ---------------------------------------------

// bandXRange is the horizontal extent [x0, x1] of the band covering visual line
// ln for the (normalised) document range sel, in absolute device pixels: it
// starts at sel.Start.Off on the start line (else at the line's left edge) and
// ends at sel.End.Off on the end line (else at the editor's right text edge, so a
// range spanning through the line reads as covering its wrap). ok is false when
// ln is a decoration line (no caret cells) or lies outside [Start.Block,
// End.Block], or when the range covers no cell on it. It is shared by
// drawSelectionBand and the match bands so the active selection and the search
// matches place their bands identically.
func (e *RichEditor) bandXRange(ln reLine, sel DocSelection, r Rect) (x0, x1 int, ok bool) {
	if !ln.hasStops || ln.blockIdx < sel.Start.Block || ln.blockIdx > sel.End.Block {
		return 0, 0, false
	}
	n := ln.nCells()
	lo := -(1 << 30)
	if ln.blockIdx == sel.Start.Block {
		lo = sel.Start.Off - ln.startOff
	}
	hiAbs := 1 << 30
	if ln.blockIdx == sel.End.Block {
		hiAbs = sel.End.Off - ln.startOff
	}
	c0 := reClamp(lo, 0, n)
	c1 := reClamp(hiAbs, 0, n)
	continues := hiAbs > n
	if c1 <= c0 && !continues {
		return 0, 0, false
	}
	x0 = ln.cellX[c0]
	x1 = ln.cellX[c1]
	if continues {
		x1 = r.X + r.W - rePadX() - e.scrollbarReserve()
	}
	return x0, x1, true
}

// hasMatchHighlights reports whether anything search-related is set, so Draw
// skips the per-line band pass entirely when no search is active.
func (e *RichEditor) hasMatchHighlights() bool {
	return len(e.matchRanges) > 0 || !e.currentMatch.IsEmpty()
}

// drawMatchBands paints, for one visual line, every soft match band the line
// covers plus (last, so it reads on top) the current-match band with its accent
// outline box. An empty range in the soft set is skipped, so overlapping and
// empty entries are safe to hand in.
func (e *RichEditor) drawMatchBands(p painter.Painter, theme *Theme, ln reLine, r Rect, scroll int) {
	soft := e.matchTint(theme)
	for _, m := range e.matchRanges {
		if m.IsEmpty() {
			continue
		}
		e.paintMatchBand(p, ln, normalizeSel(m), r, scroll, soft, RGBA{})
	}
	if !e.currentMatch.IsEmpty() {
		e.paintMatchBand(p, ln, normalizeSel(e.currentMatch), r, scroll, e.currentMatchTint(theme), theme.Accent)
	}
}

// paintMatchBand fills one match band on line ln with fill and, when outline's
// alpha is non-zero, draws a 1-px box around it — the emphasis that distinguishes
// the current match. A range that covers no cell on the line paints nothing.
func (e *RichEditor) paintMatchBand(p painter.Painter, ln reLine, sel DocSelection, r Rect, scroll int, fill, outline RGBA) {
	x0, x1, ok := e.bandXRange(ln, sel, r)
	if !ok {
		return
	}
	y := ln.y - scroll
	fillRect(p, x0, y, x1-x0, ln.h, fill)
	if outline.A != 0 {
		strokeRect(p, x0, y, x1-x0, ln.h, outline)
	}
}

// matchTint is the soft highlight colour behind every match: MatchColor when the
// host set one, else a faint wash of the theme accent (lighter than, and distinct
// from, the selection band so the two read apart on the same line).
func (e *RichEditor) matchTint(theme *Theme) RGBA {
	if e.MatchColor.A != 0 {
		return e.MatchColor
	}
	a := theme.Accent
	return RGBA{R: a.R, G: a.G, B: a.B, A: 0x3C}
}

// currentMatchTint is the fill behind the current match: CurrentMatchColor when
// the host set one, else a stronger wash of the theme accent, under the accent
// outline box drawMatchBands adds.
func (e *RichEditor) currentMatchTint(theme *Theme) RGBA {
	if e.CurrentMatchColor.A != 0 {
		return e.CurrentMatchColor
	}
	a := theme.Accent
	return RGBA{R: a.R, G: a.G, B: a.B, A: 0x80}
}
