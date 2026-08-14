// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"

	"github.com/go-widgets/painter"
)

// LinkCard is a content card for an external link: a small square favicon on
// the left, a wrapped title beside it and the source domain under the title in
// the dim tone, with an optional CardMeta strip closing the card. It is the
// unfurled-link tile a bookmarks list or a link-sharing feed is built from.
//
// Layout (inside the CardPadX/Y inset):
//
//	┌────────────────────────────┐
//	│ ▣  Wrapped link title over  │  ← Favicon (left), Title wrapped in the
//	│    as many lines as needed  │    column to its right
//	│    example.com              │  ← Domain, dim, under the title
//	│ author · 3h · ▲12 · 💬4     │  ← Meta (optional), full content width
//	└────────────────────────────┘
//
// The favicon is a one-line-tall square; a nil Favicon drops it and the title
// column spans the full content width. The domain is a single elided line.
// LinkCard is passive content — no hover, no selection.
type LinkCard struct {
	Base
	// Favicon is the site glyph shown at the left; nil drops it and the title
	// column takes the full width. It is scaled into a square the height of one
	// text line.
	Favicon *image.RGBA
	// Title is the link headline, wrapped to the title column over as many lines
	// as it needs. Empty draws no title.
	Title string
	// Domain is the source host ("example.com"), drawn dim under the title as a
	// single elided line. Empty draws no domain.
	Domain string
	// Meta is the optional byline strip drawn under the title column; nil (or an
	// all-hidden strip) draws nothing and reserves no space.
	Meta *CardMeta
}

// NewLinkCard builds a LinkCard with an optional favicon (nil for none), a
// title, a domain and an optional meta strip (nil for none).
func NewLinkCard(favicon *image.RGBA, title, domain string, meta *CardMeta) *LinkCard {
	return &LinkCard{Favicon: favicon, Title: title, Domain: domain, Meta: meta}
}

// hasFavicon reports whether the favicon band is drawn: a non-nil image with a
// positive extent.
func (c *LinkCard) hasFavicon() bool {
	if c.Favicon == nil {
		return false
	}
	b := c.Favicon.Bounds()
	return b.Dx() > 0 && b.Dy() > 0
}

// layout computes the card's geometry at content width cw: the favicon square
// side (0 when absent), the width of the title column beside it, the wrapped
// title lines, the domain height, the head-row height (favicon vs stacked
// title+domain, whichever is taller), the meta height, the block y-offsets
// (head row, then meta) and the total content height.
func (c *LinkCard) layout(cw int) (fav, colW int, title []string, domainH, headH, metaH int, ys []int, total int) {
	f := c.EffectiveFont()
	colW = cw
	if c.hasFavicon() {
		fav = f.Height()
		colW = cw - fav - CardGapX
	}
	if colW < 1 {
		colW = 1
	}
	title = wrapText(f, c.Title, colW)
	titleH := textBlockHeight(f, len(title))
	if c.Domain != "" {
		domainH = f.Height()
	}
	_, colH := stackLayout([]int{titleH, domainH})
	headH = colH
	if fav > headH {
		headH = fav
	}
	if c.Meta != nil {
		metaH = c.Meta.Measure(cw)
	}
	ys, total = stackLayout([]int{headH, metaH})
	return
}

// Measure reports the card's height at the given outer width — the head row
// (favicon beside the stacked title and domain) and the meta strip stacked with
// CardGapY between them, plus the CardPadY inset top and bottom.
func (c *LinkCard) Measure(width int) int {
	_, _, _, _, _, _, _, total := c.layout(width - 2*CardPadX)
	return total + 2*CardPadY
}

// Children yields the meta strip when present so a generic walk (accessibility,
// text selection) reaches it. The favicon, title and domain are drawn directly
// and are not sub-widgets.
func (c *LinkCard) Children() []Widget {
	if c.Meta == nil {
		return nil
	}
	return []Widget{c.Meta}
}

// Draw paints the frame, the favicon, the wrapped title, the dim domain line
// and the meta strip. Content fills exactly Measure(Bounds().W): the same
// layout drives both.
func (c *LinkCard) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	inner := cardFrame(p, theme, r)
	fav, colW, title, domainH, _, metaH, ys, _ := c.layout(inner.W)
	f := c.EffectiveFont()

	headY := inner.Y + ys[0]
	colX := inner.X
	if fav > 0 {
		pix, iw, ih := rgbaPixels(c.Favicon)
		band := Rect{X: inner.X, Y: headY, W: fav, H: fav}
		blitImage(p, band, band, pix, iw, ih)
		colX = inner.X + fav + CardGapX
	}

	tYs, _ := stackLayout([]int{textBlockHeight(f, len(title)), domainH})
	c.drawTextBlock(p, colX, headY+tYs[0], colW, title, theme.OnSurface)
	if domainH > 0 {
		c.drawTextBlock(p, colX, headY+tYs[1], colW, []string{c.Domain}, dimInk(theme))
	}

	if metaH > 0 {
		c.Meta.SetBounds(Rect{X: inner.X, Y: inner.Y + ys[1], W: inner.W, H: metaH})
		c.Meta.Draw(p, theme)
	}
}
