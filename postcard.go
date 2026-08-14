// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"

	"github.com/go-widgets/painter"
)

// PostCard is a rich feed row: a coloured pill (a source / category tag) with a
// muted subtitle beside it, a wrapped multi-line title, a muted meta line pinned
// to the bottom, and an optional lead thumbnail pinned to the top of a right-hand
// column. It is the generic form of a social / news / discussion card — an app
// maps its own item onto the fields (Pill = source, Subtitle = channel, Title =
// headline, Meta = score · age, Thumbnail = cached image) rather than
// hand-drawing the row.
//
// Layout (inside the CardPadX/Y inset):
//
//	┌──────────────────────────────┬────────┐
//	│ [Pill] subtitle               │        │  ← badge row: pill + muted subtitle
//	│ A wrapped title over as many  │ thumb  │  ← Title, one Label per wrapped line
//	│ lines as it needs to fit      │        │
//	│                              │        │  ← flex spacer pushes meta down
//	│ meta · line · here            │        │  ← Meta (muted), bottom-pinned
//	└──────────────────────────────┴────────┘
//
// The content column is an HBox flex child; the thumbnail (when present) is a
// fixed column whose image is pinned to the top, so a tall (multi-line) card
// leaves blank space below the thumbnail rather than stretching it. The subtitle,
// each wrapped title line and the meta line are real Labels exposed through
// Children, so CollectRuns lifts them out as selectable text runs.
//
// PostCard is passive content: it lays out and paints itself and reports its exact
// height through Measure(width); a feed list (CardList / VirtualList) puts
// selection / hover / disabled affordances on top.
type PostCard struct {
	Base
	// Pill is the coloured tag text (e.g. the source). Empty draws no pill.
	Pill string
	// PillColor is the pill body colour; the zero value (A==0) lets the pill fall
	// back to Theme.Accent.
	PillColor RGBA
	// PillInk is the pill text colour. The zero value (A==0) derives a readable
	// ink from PillColor (near-black on a light pill, near-white on a dark one),
	// or falls back to Theme.Background when PillColor is also unset.
	PillInk RGBA
	// Subtitle is the muted text beside the pill (e.g. the channel). Empty hides
	// it; when both Pill and Subtitle are empty the whole badge row collapses.
	Subtitle string
	// Title is the headline, wrapped to the content width over as many lines as it
	// needs, capped at MaxTitleLines with the last shown line ellipsised. Empty
	// draws no title.
	Title string
	// Meta is the muted footer line (e.g. "▲128 · 3h"). Empty hides it.
	Meta string
	// Thumbnail is the optional lead image; nil drops the whole thumbnail column.
	// It is scaled to fit its box preserving aspect ratio.
	Thumbnail *image.RGBA
	// ThumbW / ThumbH size the thumbnail column; a non-positive value selects the
	// default (DefaultPostCardThumbW / DefaultPostCardThumbH).
	ThumbW, ThumbH int
	// MaxTitleLines caps the wrapped title; a non-positive value selects the
	// default (DefaultPostCardTitleLines).
	MaxTitleLines int

	// Built leaves, rebuilt from the fields on every assemble so Children reflects
	// the current content. They are the SelectableText runs CollectRuns lifts out.
	subtitleLbl *Label
	titleLbls   []*Label
	metaLbl     *Label
}

const (
	// DefaultPostCardThumbW / DefaultPostCardThumbH are the thumbnail column's
	// pixel size when ThumbW / ThumbH are left unset.
	DefaultPostCardThumbW = 72
	DefaultPostCardThumbH = 72
	// DefaultPostCardTitleLines caps the wrapped title when MaxTitleLines is
	// unset: a long headline shows at most this many lines before it is cut.
	DefaultPostCardTitleLines = 3
)

// NewPostCard builds a PostCard from its four text fields. Set PillColor,
// Thumbnail and the sizing fields afterwards as needed.
func NewPostCard(pill, subtitle, title, meta string) *PostCard {
	return &PostCard{Pill: pill, Subtitle: subtitle, Title: title, Meta: meta}
}

// hasThumb reports whether the card carries a thumbnail column (a non-nil image).
func (c *PostCard) hasThumb() bool { return c.Thumbnail != nil }

// thumbSize is the thumbnail column's pixel size: the caller's ThumbW / ThumbH
// when positive, else the defaults.
func (c *PostCard) thumbSize() (w, h int) {
	w, h = c.ThumbW, c.ThumbH
	if w <= 0 {
		w = DefaultPostCardThumbW
	}
	if h <= 0 {
		h = DefaultPostCardThumbH
	}
	return w, h
}

// maxLines is the effective title line cap: MaxTitleLines when positive, else
// DefaultPostCardTitleLines.
func (c *PostCard) maxLines() int {
	if c.MaxTitleLines > 0 {
		return c.MaxTitleLines
	}
	return DefaultPostCardTitleLines
}

// badgeRowH is the badge row's height, or 0 when neither a pill nor a subtitle is
// shown (the row then collapses and reserves no space).
func (c *PostCard) badgeRowH() int {
	if c.Pill == "" && c.Subtitle == "" {
		return 0
	}
	return c.glyphHeight() + 2*BadgePadY
}

// titleSlot is the vertical slot one wrapped title line occupies.
func (c *PostCard) titleSlot() int { return c.glyphHeight() + CardLineSpacing }

// metaH is the meta line's height, or 0 when no meta is shown.
func (c *PostCard) metaH() int {
	if c.Meta == "" {
		return 0
	}
	return c.glyphHeight()
}

// contentWidth is the text width the title wraps into at outer width outerW: the
// card inset by CardPadX on each side, less the thumbnail column and its gap when
// present. Clamped to at least 1 so a pathologically narrow card still wraps.
func (c *PostCard) contentWidth(outerW int) int {
	cw := outerW - 2*CardPadX
	if c.hasThumb() {
		tw, _ := c.thumbSize()
		cw -= tw + CardGapX
	}
	if cw < 1 {
		cw = 1
	}
	return cw
}

// titleLines word-wraps the title to content width cw and clamps it to maxLines,
// ellipsising the last kept line on overflow. An empty title yields no lines.
func (c *PostCard) titleLines(cw int) []string {
	f := c.EffectiveFont()
	return clampLines(f, wrapText(f, c.Title, cw), c.maxLines(), cw)
}

// contentHeight is the content column's height at content width cw: the badge
// row, the wrapped title, a CardGapY spacer and the meta line stacked. When a
// thumbnail is present and taller than that stack, the thumbnail dictates the
// height instead so the image is never clipped by a short card.
func (c *PostCard) contentHeight(cw int) int {
	n := len(c.titleLines(cw))
	h := c.badgeRowH() + n*c.titleSlot() + CardGapY + c.metaH()
	if c.hasThumb() {
		if _, th := c.thumbSize(); th > h {
			h = th
		}
	}
	return h
}

// Measure reports the card's exact height at outer width width: the content
// column height plus the CardPadY inset top and bottom. A feed list allocates
// exactly this, and Draw fills exactly this, because both drive off contentHeight.
func (c *PostCard) Measure(width int) int {
	return 2*CardPadY + c.contentHeight(c.contentWidth(width))
}

// pillInk is the pill's text colour: PillInk when set, else a readable ink
// derived from PillColor, else the zero RGBA (letting Badge fall back to the
// theme background).
func (c *PostCard) pillInk() RGBA {
	if c.PillInk.A != 0 {
		return c.PillInk
	}
	if c.PillColor.A != 0 {
		return readableInk(c.PillColor)
	}
	return RGBA{}
}

// readableInk returns a near-black or near-white ink, whichever reads better on
// bg, using the Rec. 601 luma of the background.
func readableInk(bg RGBA) RGBA {
	luma := (299*int(bg.R) + 587*int(bg.G) + 114*int(bg.B)) / 1000
	if luma >= 140 {
		return RGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xFF}
	}
	return RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
}

// innerRect is the content rectangle: the card's Bounds inset by CardPadX / Y.
func (c *PostCard) innerRect() Rect {
	r := c.Bounds()
	return Rect{X: r.X + CardPadX, Y: r.Y + CardPadY, W: r.W - 2*CardPadX, H: r.H - 2*CardPadY}
}

// assemble (re)builds the card's widget tree from the current fields and lays it
// out inside inner, storing the leaf Labels for Children / CollectRuns. It
// returns the root HBox for Draw to paint. The badge row (pill + subtitle) is a
// fixed row; the wrapped title is one Label per line; a flex spacer pushes the
// meta line to the bottom; the thumbnail, when present, is pinned to the top of a
// fixed right-hand column.
func (c *PostCard) assemble(inner Rect) *HBox {
	f := c.EffectiveFont()
	contentW := inner.W
	if c.hasThumb() {
		tw, _ := c.thumbSize()
		contentW -= tw + CardGapX
	}
	if contentW < 1 {
		contentW = 1
	}

	col := NewVBox()
	col.Spacing = 0

	c.subtitleLbl = nil
	if c.badgeRowH() > 0 {
		row := NewHBox()
		row.Spacing = CardGapX
		if c.Pill != "" {
			badge := &Badge{Text: c.Pill, Fill: c.PillColor, Ink: c.pillInk()}
			row.AddFixed(badge, f.Measure(c.Pill)+2*BadgePadX)
		}
		if c.Subtitle != "" {
			c.subtitleLbl = NewLabel(c.Subtitle)
			c.subtitleLbl.Ellipsis = true
			c.subtitleLbl.VAlign = VMiddle
			row.AddFlex(c.subtitleLbl, 1)
		} else {
			row.AddFlex(NewLabel(""), 1) // filler keeps a lone pill from stretching
		}
		col.AddFixed(row, c.badgeRowH())
	}

	c.titleLbls = nil
	for _, ln := range c.titleLines(contentW) {
		lbl := NewLabel(ln)
		lbl.Ellipsis = true
		c.titleLbls = append(c.titleLbls, lbl)
		col.AddFixed(lbl, c.titleSlot())
	}

	col.AddFlex(NewLabel(""), 1) // spacer: pushes the meta line to the bottom

	c.metaLbl = nil
	if c.Meta != "" {
		c.metaLbl = NewLabel(c.Meta)
		c.metaLbl.Ellipsis = true
		c.metaLbl.VAlign = VBottom
		col.AddFixed(c.metaLbl, c.metaH())
	}

	root := NewHBox()
	root.Spacing = CardGapX
	root.AddFlex(col, 1)
	if c.hasThumb() {
		tw, th := c.thumbSize()
		tcol := NewVBox()
		tcol.Spacing = 0
		tcol.AddFixed(&postThumb{img: c.Thumbnail}, th)
		tcol.AddFlex(NewLabel(""), 1) // pin the image to the top of the column
		root.AddFixed(tcol, tw)
	}
	root.SetBounds(inner)
	return root
}

// Children yields the card's selectable Labels in visual order — the subtitle,
// each wrapped title line, then the meta line — so CollectRuns lifts them out as
// text runs and a11y / selection walks reach them. The pill and thumbnail are
// decoration and are not returned. Calling Children re-assembles the tree at the
// card's current bounds, so the returned Labels carry laid-out positions.
func (c *PostCard) Children() []Widget {
	c.assemble(c.innerRect())
	out := make([]Widget, 0, len(c.titleLbls)+2)
	if c.subtitleLbl != nil {
		out = append(out, c.subtitleLbl)
	}
	for _, l := range c.titleLbls {
		out = append(out, l)
	}
	if c.metaLbl != nil {
		out = append(out, c.metaLbl)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Draw paints the shared card frame, then the assembled content and thumbnail.
// The muted inks (subtitle, meta) and the title ink are theme-derived here, at
// paint time. Content fills exactly Measure(Bounds().W): the same layout drives
// both.
func (c *PostCard) Draw(p painter.Painter, theme *Theme) {
	inner := cardFrame(p, theme, c.Bounds())
	root := c.assemble(inner)
	if c.subtitleLbl != nil {
		c.subtitleLbl.Ink = dimInk(theme)
	}
	for _, l := range c.titleLbls {
		l.Ink = theme.OnSurface
	}
	if c.metaLbl != nil {
		c.metaLbl.Ink = dimInk(theme)
	}
	root.Draw(p, theme)
}

// A11y reports the card as a group named by its title.
func (c *PostCard) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: c.Title}
}

// postThumb is the leaf widget that paints a PostCard's thumbnail box: a muted
// SurfaceAlt ground with the image (when present) scaled to fit inside it,
// preserving aspect ratio and centred. It is laid out like any other child by the
// thumbnail column's box.
type postThumb struct {
	Base
	img *image.RGBA
}

// Draw fills the box with the theme's SurfaceAlt then blits the image scaled to
// fit (aspect-preserving, centred). A nil or zero-extent image leaves just the
// muted ground; an empty box paints nothing.
func (t *postThumb) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	if t.img == nil {
		return
	}
	b := t.img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw <= 0 || ih <= 0 {
		return
	}
	dw, dh := r.W, r.H
	if r.W*ih <= r.H*iw {
		dh = r.W * ih / iw // width-bound: fit to the box width
	} else {
		dw = r.H * iw / ih // height-bound: fit to the box height
	}
	if dw <= 0 {
		dw = 1
	}
	if dh <= 0 {
		dh = 1
	}
	dx := r.X + (r.W-dw)/2
	dy := r.Y + (r.H-dh)/2
	pix, pw, ph := rgbaPixels(t.img)
	blitImage(p, Rect{X: dx, Y: dy, W: dw, H: dh}, r, pix, pw, ph)
}
