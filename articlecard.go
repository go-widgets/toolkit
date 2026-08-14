// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ArticleCard is a text-led content card: a wrapped headline, a summary body
// wrapped and truncated to at most a few lines, and an optional CardMeta strip.
// It is the row a news / blog / discussion feed is built from, where the words
// carry the item and there is no lead image.
//
// Layout (top to bottom, inside the CardPadX/Y inset):
//
//	┌──────────────────────────┐
//	│ Wrapped headline over as  │  ← Title, wrapped to the content width
//	│ many lines as it needs    │
//	│ A summary that wraps and  │  ← Body, wrapped then clamped to BodyLines,
//	│ is cut to BodyLines with…  │    the last kept line ellipsised on overflow
//	│ author · 3h · ▲12 · 💬4   │  ← Meta (optional)
//	└──────────────────────────┘
//
// The body is wrapped to the content width and then truncated to at most
// BodyLines lines; when the summary is longer, the last kept line ends in an
// ellipsis so the cut reads as deliberate. ArticleCard is passive content — no
// hover, no selection.
type ArticleCard struct {
	Base
	// Title is the headline, wrapped to the content width over as many lines as
	// it needs. Empty draws no title.
	Title string
	// Body is the summary, wrapped to the content width and then clamped to at
	// most BodyLines lines. Empty draws no body.
	Body string
	// BodyLines caps the wrapped body. Zero or negative selects the default
	// (DefaultArticleBodyLines) so a caller can leave it unset.
	BodyLines int
	// Meta is the optional byline strip drawn under the body; nil (or an
	// all-hidden strip) draws nothing and reserves no space.
	Meta *CardMeta
}

// DefaultArticleBodyLines is the body clamp used when BodyLines is unset (zero
// or negative): a summary shows at most this many lines before it is cut.
const DefaultArticleBodyLines = 3

// NewArticleCard builds an ArticleCard with a title, a summary body and an
// optional meta strip (nil for none). The body uses DefaultArticleBodyLines;
// set the BodyLines field afterwards to change the cap.
func NewArticleCard(title, body string, meta *CardMeta) *ArticleCard {
	return &ArticleCard{Title: title, Body: body, Meta: meta}
}

// bodyCap is the effective body line clamp: BodyLines when positive, otherwise
// DefaultArticleBodyLines.
func (c *ArticleCard) bodyCap() int {
	if c.BodyLines > 0 {
		return c.BodyLines
	}
	return DefaultArticleBodyLines
}

// layout computes the card's blocks at content width cw: the wrapped title
// lines, the wrapped-and-clamped body lines, the meta height, each block's top
// y-offset and the total content height (before the CardPadY inset).
func (c *ArticleCard) layout(cw int) (title, body []string, metaH int, ys []int, total int) {
	f := c.EffectiveFont()
	title = wrapText(f, c.Title, cw)
	body = clampLines(f, wrapText(f, c.Body, cw), c.bodyCap(), cw)
	titleH := textBlockHeight(f, len(title))
	bodyH := textBlockHeight(f, len(body))
	if c.Meta != nil {
		metaH = c.Meta.Measure(cw)
	}
	ys, total = stackLayout([]int{titleH, bodyH, metaH})
	return
}

// Measure reports the card's height at the given outer width — the wrapped
// title, the clamped body and the meta strip stacked with CardGapY between
// them, plus the CardPadY inset top and bottom.
func (c *ArticleCard) Measure(width int) int {
	_, _, _, _, total := c.layout(width - 2*CardPadX)
	return total + 2*CardPadY
}

// Children yields the meta strip when present so a generic walk (accessibility,
// text selection) reaches it. The title and body are drawn directly and are not
// sub-widgets.
func (c *ArticleCard) Children() []Widget {
	if c.Meta == nil {
		return nil
	}
	return []Widget{c.Meta}
}

// Draw paints the frame, the wrapped title, the clamped body and the meta
// strip. Content fills exactly Measure(Bounds().W): the same layout drives both.
func (c *ArticleCard) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	inner := cardFrame(p, theme, r)
	title, body, metaH, ys, _ := c.layout(inner.W)

	c.drawTextBlock(p, inner.X, inner.Y+ys[0], inner.W, title, theme.OnSurface)
	c.drawTextBlock(p, inner.X, inner.Y+ys[1], inner.W, body, dimInk(theme))

	if metaH > 0 {
		c.Meta.SetBounds(Rect{X: inner.X, Y: inner.Y + ys[2], W: inner.W, H: metaH})
		c.Meta.Draw(p, theme)
	}
}
