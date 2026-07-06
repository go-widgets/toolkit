// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// Card is a bordered container laid out as three optional zones: a
// header strip at the top (title text on a SurfaceAlt background), a
// body area with multi-line text (each '\n'-separated line rendered on
// its own row) and a footer strip at the bottom (SurfaceAlt like the
// header). The whole card sits on a Theme.Surface fill and is framed
// by a 1-px Theme.Border stroke — the same visual grammar Button and
// Menu use so a Card composes cleanly next to them.
//
// Any zone may be empty:
//   - Title == ""  -> the header strip is skipped, the body starts at r.Y.
//   - Body  == ""  -> no text lines are drawn (the surface fill still shows).
//   - Footer== ""  -> the footer strip is skipped, the body flows to r.Y+r.H.
//
// Card is a passive display container — it does not intercept input
// (HitTest / OnEvent stay as Base defaults) so a caller that needs an
// interactive Card wraps it with an outer container or overlays a
// Button on top.
type Card struct {
	Base
	Title  string
	Body   string
	Footer string
}

// Card sizing constants. Header + Footer strips are the same size so
// the card reads as a symmetric frame; the body gets a matching inner
// pad on the left and top.
const (
	// CardPadX is the horizontal inset for header / body / footer text.
	CardPadX = 8
	// CardPadY is the vertical inset for the body text above the first
	// line + between the footer text and its strip border.
	CardPadY = 6
	// CardHeaderH is the height of the header strip when Title != "".
	CardHeaderH = GlyphHeight + 2*CardPadY
	// CardFooterH is the height of the footer strip when Footer != "".
	CardFooterH = GlyphHeight + 2*CardPadY
	// CardLineSpacing is the extra vertical gap inserted between two
	// body lines so successive glyph rows don't touch.
	CardLineSpacing = 2
)

// NewCard constructs a Card with the given title, body + footer.
// Any of the three may be "" to skip that zone.
func NewCard(title, body, footer string) *Card {
	return &Card{Title: title, Body: body, Footer: footer}
}

// Draw paints the surface fill, the optional header and footer strips,
// each body line and finally the outer border stroke. Draw order is
// bottom-to-top (fill, then decorations, then border) so the 1-px
// border always sits on top and clips overlapping strips.
func (c *Card) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	bodyTop := r.Y

	if c.Title != "" {
		fillRect(p, r.X, r.Y, r.W, CardHeaderH, theme.SurfaceAlt)
		ty := r.Y + (CardHeaderH-GlyphHeight)/2
		DrawText(p, r.X+CardPadX, ty, c.Title, theme.OnSurface)
		// Divider between header and body.
		fillRect(p, r.X, r.Y+CardHeaderH, r.W, 1, theme.Border)
		bodyTop = r.Y + CardHeaderH + 1
	}

	if c.Footer != "" {
		footerY := r.Y + r.H - CardFooterH
		fillRect(p, r.X, footerY, r.W, CardFooterH, theme.SurfaceAlt)
		ty := footerY + (CardFooterH-GlyphHeight)/2
		DrawText(p, r.X+CardPadX, ty, c.Footer, theme.OnSurface)
		// Divider between body and footer.
		fillRect(p, r.X, footerY-1, r.W, 1, theme.Border)
	}

	if c.Body != "" {
		lineH := GlyphHeight + CardLineSpacing
		y := bodyTop + CardPadY
		for _, ln := range strings.Split(c.Body, "\n") {
			DrawText(p, r.X+CardPadX, y, ln, theme.OnSurface)
			y += lineH
		}
	}

	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}
