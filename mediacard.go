// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"

	"github.com/go-widgets/painter"
)

// MediaCard is a content card led by a prominent thumbnail: the image spans the
// full content width at the top, the wrapped title sits below it, and an
// optional CardMeta strip closes the card. It is the tile a media / video /
// photo feed is built from.
//
// Layout (top to bottom, inside the CardPadX/Y inset):
//
//	┌──────────────────────────┐
//	│   full-width thumbnail    │  ← Thumbnail, at the image's own aspect
//	│                          │
//	├──────────────────────────┤
//	│ Wrapped title over as     │  ← Title, wrapped to the content width
//	│ many lines as it needs    │
//	│ author · 3h · ▲12 · 💬4   │  ← Meta (optional)
//	└──────────────────────────┘
//
// The thumbnail is scaled to the full content width at its own aspect ratio, so
// it fills the width with no letterbox; a nil Thumbnail drops the image band
// entirely. MediaCard is passive content — no hover, no selection.
type MediaCard struct {
	Base
	// Title is the headline, wrapped to the content width over as many lines as
	// it needs. Empty draws no title.
	Title string
	// Thumbnail is the lead image; nil drops the image band. It is scaled to the
	// full content width at its own aspect ratio.
	Thumbnail *image.RGBA
	// Meta is the optional byline strip drawn under the title; nil (or an
	// all-hidden strip) draws nothing and reserves no space.
	Meta *CardMeta
}

// NewMediaCard builds a MediaCard with a title, an optional thumbnail (nil for
// none) and an optional meta strip (nil for none).
func NewMediaCard(title string, thumb *image.RGBA, meta *CardMeta) *MediaCard {
	return &MediaCard{Title: title, Thumbnail: thumb, Meta: meta}
}

// thumbHeight is the image band's height at content width cw: cw scaled to the
// thumbnail's own aspect ratio. Zero when there is no usable image.
func (c *MediaCard) thumbHeight(cw int) int {
	if c.Thumbnail == nil {
		return 0
	}
	b := c.Thumbnail.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return 0
	}
	return cw * b.Dy() / b.Dx()
}

// layout computes the card's blocks at content width cw: the thumbnail band
// height, the wrapped title lines, the meta height, each block's top y-offset
// and the total content height (before the CardPadY inset is added back).
func (c *MediaCard) layout(cw int) (thumbH int, title []string, metaH int, ys []int, total int) {
	f := c.EffectiveFont()
	thumbH = c.thumbHeight(cw)
	title = wrapText(f, c.Title, cw)
	titleH := textBlockHeight(f, len(title))
	if c.Meta != nil {
		metaH = c.Meta.Measure(cw)
	}
	ys, total = stackLayout([]int{thumbH, titleH, metaH})
	return
}

// Measure reports the card's height at the given outer width — the thumbnail
// band, the wrapped title and the meta strip stacked with CardGapY between
// them, plus the CardPadY inset top and bottom.
func (c *MediaCard) Measure(width int) int {
	_, _, _, _, total := c.layout(width - 2*CardPadX)
	return total + 2*CardPadY
}

// Children yields the meta strip when present, so a generic walk (accessibility,
// text selection) reaches it. The title and thumbnail are drawn directly and
// are not sub-widgets.
func (c *MediaCard) Children() []Widget {
	if c.Meta == nil {
		return nil
	}
	return []Widget{c.Meta}
}

// Draw paints the frame, the full-width thumbnail, the wrapped title and the
// meta strip. Content fills exactly Measure(Bounds().W): the same layout drives
// both.
func (c *MediaCard) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	inner := cardFrame(p, theme, r)
	thumbH, title, metaH, ys, _ := c.layout(inner.W)

	if thumbH > 0 {
		pix, iw, ih := rgbaPixels(c.Thumbnail)
		band := Rect{X: inner.X, Y: inner.Y + ys[0], W: inner.W, H: thumbH}
		blitImage(p, band, band, pix, iw, ih)
	}

	c.drawTextBlock(p, inner.X, inner.Y+ys[1], inner.W, title, theme.OnSurface)

	if metaH > 0 {
		c.Meta.SetBounds(Rect{X: inner.X, Y: inner.Y + ys[2], W: inner.W, H: metaH})
		c.Meta.Draw(p, theme)
	}
}
