// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"strings"

	"github.com/go-widgets/painter"
)

// Shared layout + drawing helpers for the content-card family (CardMeta,
// MediaCard, ArticleCard, LinkCard). These cards are PASSIVE content widgets:
// they lay out and paint their own content (text wrapping, a thumbnail blit,
// a meta strip) and expose a Measure(width) that reports the exact height that
// content needs at that width. A feed list (CardList / VirtualList) puts the
// selection, hover and disabled affordances on TOP; the cards themselves never
// read input, so nothing here touches HitTest / OnEvent.
//
// Every card shares one visual grammar — a Theme.Surface fill with rounded
// corners and a 1-px Theme.Border stroke, its content inset by CardPadX/Y —
// so a mixed feed of the four types reads as one system.

const (
	// CardCornerRadius is the corner rounding of a content card's frame, in
	// pixels. Matched to Badge / GalleryView so a card sits next to them cleanly;
	// a cell back-end that cannot round degrades to square corners.
	CardCornerRadius = 6
	// CardGapX is the horizontal gap between a leading glyph column (a favicon,
	// say) and the text beside it.
	CardGapX = 6
	// CardGapY is the vertical gap inserted between two stacked content blocks
	// (thumbnail / title / body / meta). Distinct from CardLineSpacing, which is
	// the tighter gap between successive lines WITHIN one wrapped text block.
	CardGapY = 4
)

// cardFrame paints the shared card ground — a Theme.Surface fill with rounded
// corners under a 1-px Theme.Border stroke — into r and returns the inner
// content rectangle (r inset by CardPadX horizontally and CardPadY vertically
// on every side) that the card lays its content into. A card's Draw calls this
// first, then positions its blocks inside the returned rect.
func cardFrame(p painter.Painter, theme *Theme, r Rect) Rect {
	fillRoundRect(p, r.X, r.Y, r.W, r.H, CardCornerRadius, theme.Surface)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, CardCornerRadius, theme.Border)
	return Rect{
		X: r.X + CardPadX,
		Y: r.Y + CardPadY,
		W: r.W - 2*CardPadX,
		H: r.H - 2*CardPadY,
	}
}

// wrapText greedily breaks text into lines that each fit within width pixels
// when measured in font f, breaking only at spaces. A single word wider than
// width is placed on its own line rather than split mid-word (the drawing side
// ellipsises such an over-long line to the width). Empty / all-whitespace text
// yields no lines.
func wrapText(f Font, text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, len(words))
	cur := words[0]
	for _, w := range words[1:] {
		try := cur + " " + w
		if f.Measure(try) <= width {
			cur = try
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	return append(lines, cur)
}

// clampLines truncates lines to at most max entries, marking the truncation by
// ellipsising the new last line to width (so it ends with "…"). A non-positive
// max, or a slice already within max, is returned unchanged.
func clampLines(f Font, lines []string, max, width int) []string {
	if max <= 0 || len(lines) <= max {
		return lines
	}
	lines = lines[:max]
	last := max - 1
	lines[last] = ellipsize(f, lines[last], width)
	return lines
}

// textBlockHeight is the pixel height a block of n lines occupies when drawn in
// font f: n glyph rows with CardLineSpacing between successive rows. Zero (or
// fewer) lines occupy no height.
func textBlockHeight(f Font, n int) int {
	if n <= 0 {
		return 0
	}
	return n*f.Height() + (n-1)*CardLineSpacing
}

// stackLayout lays block heights out top-to-bottom, inserting CardGapY between
// each pair of consecutive NON-ZERO blocks (a zero-height block — an absent
// thumbnail, an empty meta strip — takes no space and adds no gap). It returns
// the top y-offset of each block (relative to the content origin) and the total
// stacked height. Both Measure and Draw drive off this one function so the
// height a card reports and the height it paints can never diverge.
func stackLayout(blocks []int) (ys []int, total int) {
	ys = make([]int, len(blocks))
	y := 0
	prev := false
	for i, b := range blocks {
		ys[i] = y
		if b <= 0 {
			continue
		}
		if prev {
			y += CardGapY
			ys[i] = y
		}
		y += b
		prev = true
	}
	return ys, y
}

// drawTextBlock paints each line of a wrapped text block left-aligned at x,
// starting at y, in ink, ellipsising any line still wider than width (an
// over-long unbreakable word). It returns the y just past the block.
func (b *Base) drawTextBlock(p painter.Painter, x, y, width int, lines []string, ink RGBA) int {
	f := b.EffectiveFont()
	lineH := f.Height() + CardLineSpacing
	for _, ln := range lines {
		if f.Measure(ln) > width {
			ln = ellipsize(f, ln, width)
		}
		b.drawText(p, x, y, ln, ink)
		y += lineH
	}
	return y
}

// rgbaPixels exposes an *image.RGBA as the tightly-packed RGBA byte buffer
// blitImage expects, with its width and height. When the image is already
// contiguous with a zero origin (the shape image.NewRGBA produces) its own Pix
// slice is returned with no copy; a sub-image or strided buffer is repacked row
// by row.
func rgbaPixels(img *image.RGBA) (pix []byte, w, h int) {
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	if img.Rect.Min.X == 0 && img.Rect.Min.Y == 0 && img.Stride == w*4 {
		return img.Pix, w, h
	}
	pix = make([]byte, w*h*4)
	for row := 0; row < h; row++ {
		src := img.PixOffset(b.Min.X, b.Min.Y+row)
		copy(pix[row*w*4:(row+1)*w*4], img.Pix[src:src+w*4])
	}
	return pix, w, h
}
