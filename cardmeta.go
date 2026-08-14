// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// CardMeta is a horizontal strip of small metadata for a content card: an
// author, a relative time, a score and a comment count, laid out left to right
// as "author · time · ▲score · 💬comments" and elided to its width. It is the
// reusable footer/byline the MediaCard, ArticleCard and LinkCard all share, so
// the byline of a feed reads the same whatever the card type.
//
// A field is shown only when it carries a value: Author / Time when non-empty,
// Score / Comments when NON-NEGATIVE. Set Score or Comments to a negative value
// (the sentinel −1 reads well) to hide that count entirely — a story with no
// score, an item with comments disabled. An all-hidden strip measures and
// paints as nothing, so a card can carry an empty CardMeta without reserving
// space for it.
//
// CardMeta is passive content: it never reads input. Colour is the theme's
// dim-label tone (see dimInk) so the strip reads as subordinate to the title
// above it.
type CardMeta struct {
	Base
	// Author is the byline (a user / source name); empty hides it.
	Author string
	// Time is a pre-formatted relative or absolute time ("3h", "2026-08-14");
	// empty hides it. CardMeta does not format time — the caller passes a string.
	Time string
	// Score is an up-vote / points count; negative hides it (use −1).
	Score int
	// Comments is a reply count; negative hides it (use −1).
	Comments int
}

// cardMetaSep joins the strip's segments. The middot reads as a neutral
// separator; the score / comment glyphs mark their counts.
const (
	cardMetaSep      = " · "
	cardMetaScore    = "▲"
	cardMetaComments = "💬"
)

// NewCardMeta builds a meta strip. Pass −1 for score or comments to hide that
// count; pass "" for author or time to hide those.
func NewCardMeta(author, time string, score, comments int) *CardMeta {
	return &CardMeta{Author: author, Time: time, Score: score, Comments: comments}
}

// segments is the strip's visible pieces in left-to-right order, each present
// field contributing one. An empty result means the whole strip is hidden.
func (m *CardMeta) segments() []string {
	segs := make([]string, 0, 4)
	if m.Author != "" {
		segs = append(segs, m.Author)
	}
	if m.Time != "" {
		segs = append(segs, m.Time)
	}
	if m.Score >= 0 {
		segs = append(segs, cardMetaScore+strconv.Itoa(m.Score))
	}
	if m.Comments >= 0 {
		segs = append(segs, cardMetaComments+strconv.Itoa(m.Comments))
	}
	return segs
}

// line is the fully-joined strip text (segments separated by cardMetaSep), or
// "" when nothing is shown.
func (m *CardMeta) line() string { return strings.Join(m.segments(), cardMetaSep) }

// Measure reports the strip's height at the given width — one glyph row when
// any field is shown, zero when the strip is entirely hidden. Width does not
// change the height (the strip is a single elided row); it is accepted for the
// uniform Measure(width) signature the card family shares.
func (m *CardMeta) Measure(width int) int {
	_ = width
	if len(m.segments()) == 0 {
		return 0
	}
	return m.glyphHeight()
}

// Draw paints the strip within Bounds, vertically centred when the bounds are
// taller than one glyph row, and ellipsised to the bounds width. A hidden
// (empty) strip paints nothing.
func (m *CardMeta) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	line := m.line()
	if line == "" {
		return
	}
	f := m.EffectiveFont()
	if f.Measure(line) > r.W {
		line = ellipsize(f, line, r.W)
	}
	ty := r.Y
	if r.H > f.Height() {
		ty = r.Y + (r.H-f.Height())/2
	}
	m.drawText(p, r.X, ty, line, dimInk(theme))
}
