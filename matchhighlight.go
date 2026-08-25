// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// matchBand is one search-match highlight range painted under a TextView's
// text. color is the fill tint (translucent, so the glyphs on top stay legible);
// outline, when its alpha is non-zero, is a 1-px box drawn around the band on
// each line it covers — the emphasis that distinguishes the current match from
// the rest. It reuses Selection for the range so it shares the toolkit's
// (line, col) coordinate model and the selection-band geometry.
type matchBand struct {
	sel     Selection
	color   RGBA
	outline RGBA
}

// paintMatchBands paints every search-match band that covers buffer line i,
// using the SAME horizontal geometry as a selection band (bandXRange) so a match
// aligns exactly with the glyphs it spans. Each band gets its translucent fill;
// the current match additionally gets an outline box on each of its lines.
// Empty (nil) matchBands paints nothing — the no-active-search case — and an
// empty range within the set is skipped, so overlapping and zero-width entries
// are safe to hand in.
func (t *TextView) paintMatchBands(p painter.Painter, i, textX, y, lineH int, r Rect) {
	for _, b := range t.matchBands {
		if b.sel.IsEmpty() || i < b.sel.StartLine || i > b.sel.EndLine {
			continue
		}
		x0, x1 := t.bandXRange(i, textX, r, b.sel)
		fillRect(p, x0, y-2, x1-x0, lineH, b.color)
		if b.outline.A != 0 {
			strokeRect(p, x0, y-2, x1-x0, lineH, b.outline)
		}
	}
}

// --- CodeEditor match-highlight API ---------------------------------------
//
// These extend the editor's decoration/overlay layer with search-match
// highlighting: a soft band behind every match, a stronger boxed band for the
// current one, and a scroll that reveals a match. They are UI state a search
// host pushes (the FindReplace widget's consumer runs the regexp via
// FindMatches and calls these) — the editor itself never searches, exactly as it
// never lexes. The ranges are re-resolved to colours on every Draw, so a Set is
// reflected on the next paint with no explicit invalidation.

// SetMatchHighlights replaces the set of soft match highlights — the ranges of
// every occurrence, each painted with a faint accent-tinted band. Passing an
// empty (or nil) slice clears them. The current match (SetCurrentMatch) is
// independent and unaffected. Ranges may overlap and may repeat the current
// match; the bands simply stack (their tints are translucent).
func (c *CodeEditor) SetMatchHighlights(ranges []Selection) {
	c.matchRanges = ranges
}

// MatchHighlights returns a copy of the current soft match ranges, in the order
// they were set — a host reads it back to mirror the editor's highlight set
// without holding its own copy.
func (c *CodeEditor) MatchHighlights() []Selection {
	return append([]Selection(nil), c.matchRanges...)
}

// SetCurrentMatch marks one range as the current match, painted with a stronger
// tint and an outline box on top of the soft highlights. An empty Selection
// (Start == End) clears the current-match emphasis without touching the soft
// highlight set.
func (c *CodeEditor) SetCurrentMatch(sel Selection) {
	c.currentMatch = sel
}

// CurrentMatch returns the range currently emphasised, or the zero Selection
// when none is.
func (c *CodeEditor) CurrentMatch() Selection { return c.currentMatch }

// ClearMatchHighlights removes both the soft highlight set and the current-match
// emphasis — the "search closed / query empty" reset.
func (c *CodeEditor) ClearMatchHighlights() {
	c.matchRanges = nil
	c.currentMatch = Selection{}
}

// ScrollToMatch scrolls the editor vertically so sel's start line is visible,
// centring it in the viewport when it is currently out of view. A match already
// on screen is left where it is (no jump). It only moves the vertical scroll —
// the caret and the highlight set are the host's to set — and is a no-op when
// the viewport has no room for even one line (a zero-height editor).
func (c *CodeEditor) ScrollToMatch(sel Selection) {
	line := sel.StartLine
	if line < 0 {
		line = 0
	}
	vis := c.visibleLines()
	if vis <= 0 {
		return
	}
	top := c.clampedScrollLine()
	if line >= top && line < top+vis {
		return // already visible
	}
	ns := line - vis/2 // centre it
	if ns < 0 {
		ns = 0
	}
	c.ScrollLine().Set(ns)
	c.ScrollLine().Set(c.clampedScrollLine())
}

// buildMatchBands resolves the editor's match ranges + current match into the
// TextView's paint-time band list, colouring them against theme. Called from
// Draw so a colour follows a theme swap and a range change shows on the next
// frame. With nothing to highlight it clears the band list.
func (c *CodeEditor) buildMatchBands(theme *Theme) {
	if len(c.matchRanges) == 0 && c.currentMatch.IsEmpty() {
		c.TextView.matchBands = nil
		return
	}
	soft := c.matchTint(theme)
	bands := make([]matchBand, 0, len(c.matchRanges)+1)
	for _, s := range c.matchRanges {
		if s.IsEmpty() {
			continue
		}
		bands = append(bands, matchBand{sel: s, color: soft})
	}
	if !c.currentMatch.IsEmpty() {
		bands = append(bands, matchBand{
			sel:     c.currentMatch,
			color:   c.currentMatchTint(theme),
			outline: theme.Accent,
		})
	}
	c.TextView.matchBands = bands
}

// matchTint is the soft highlight colour behind every match: MatchColor when the
// host set one, else a faint wash of the theme accent (distinct from, and
// lighter than, the selection band so the two read apart on the same line).
func (c *CodeEditor) matchTint(theme *Theme) RGBA {
	if c.MatchColor.A != 0 {
		return c.MatchColor
	}
	a := theme.Accent
	return RGBA{R: a.R, G: a.G, B: a.B, A: 0x3C}
}

// currentMatchTint is the fill behind the current match: CurrentMatchColor when
// the host set one, else a stronger wash of the theme accent, under the accent
// outline box buildMatchBands adds.
func (c *CodeEditor) currentMatchTint(theme *Theme) RGBA {
	if c.CurrentMatchColor.A != 0 {
		return c.CurrentMatchColor
	}
	a := theme.Accent
	return RGBA{R: a.R, G: a.G, B: a.B, A: 0x80}
}
