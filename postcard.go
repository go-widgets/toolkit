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
	// ThumbPlaceholder is the muted label drawn in the thumbnail box when the post
	// declares media but no decoded Thumbnail has landed yet (e.g. "image", "video").
	// A non-empty value reserves the thumbnail column even while Thumbnail is nil, so
	// the card does not reflow (grow a column) when the image finally arrives; the
	// box shows the label centred on the SurfaceAlt ground until then, reproducing
	// the historical "loading" card. Empty draws no column unless Thumbnail is set.
	ThumbPlaceholder string
	// MaxTitleLines caps the wrapped title; a non-positive value selects the
	// default (DefaultPostCardTitleLines).
	MaxTitleLines int

	// Per-element fonts give the card its type hierarchy — a larger bold title
	// over a smaller muted subtitle / meta, and a small pill — instead of one
	// uniform size. Each is optional: a nil font falls back to the card's
	// EffectiveFont (Base.Font, else the package font), so the zero value keeps
	// the previous single-font behaviour. TitleFont sizes the headline lines,
	// SubtitleFont the channel text, MetaFont the footer, PillFont the badge label.
	TitleFont, SubtitleFont, MetaFont, PillFont Font

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

// hasThumb reports whether the card carries a decoded thumbnail image.
func (c *PostCard) hasThumb() bool { return c.Thumbnail != nil }

// reservesThumb reports whether the card reserves a thumbnail column at all: it
// does so both when a decoded image is present and when only a placeholder label
// is set (media declared, image not yet landed). Column geometry keys off this so
// a card measured before its image arrives already holds the column, and mounting
// the image later does not reflow the layout.
func (c *PostCard) reservesThumb() bool { return c.Thumbnail != nil || c.ThumbPlaceholder != "" }

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
	// The caller's ThumbW/H (and the defaults) are logical pixels; scale them to
	// device pixels so the thumbnail column tracks the rest of the card's metrics.
	return scaled(w), scaled(h)
}

// maxLines is the effective title line cap: MaxTitleLines when positive, else
// DefaultPostCardTitleLines.
func (c *PostCard) maxLines() int {
	if c.MaxTitleLines > 0 {
		return c.MaxTitleLines
	}
	return DefaultPostCardTitleLines
}

// titleFont / subtitleFont / metaFont / pillFont resolve each element's font,
// falling back to the card's EffectiveFont when the caller left it nil.
func (c *PostCard) titleFont() Font    { return orFont(c.TitleFont, c.EffectiveFont()) }
func (c *PostCard) subtitleFont() Font { return orFont(c.SubtitleFont, c.EffectiveFont()) }
func (c *PostCard) metaFont() Font     { return orFont(c.MetaFont, c.EffectiveFont()) }
func (c *PostCard) pillFont() Font     { return orFont(c.PillFont, c.EffectiveFont()) }

// orFont returns f when non-nil, else the fallback.
func orFont(f, fallback Font) Font {
	if f != nil {
		return f
	}
	return fallback
}

// badgeRowH is the badge row's height, or 0 when neither a pill nor a subtitle is
// shown (the row then collapses and reserves no space). It is the taller of the
// pill (its font plus the badge's vertical padding) and the subtitle font.
func (c *PostCard) badgeRowH() int {
	h := 0
	if c.Pill != "" {
		h = c.pillFont().Height() + 2*scaled(BadgePadY)
	}
	if c.Subtitle != "" {
		if sh := c.subtitleFont().Height(); sh > h {
			h = sh
		}
	}
	return h
}

// titleSlot is the vertical slot one wrapped title line occupies.
func (c *PostCard) titleSlot() int { return c.titleFont().Height() + scaled(CardLineSpacing) }

// metaH is the meta line's height, or 0 when no meta is shown.
func (c *PostCard) metaH() int {
	if c.Meta == "" {
		return 0
	}
	return c.metaFont().Height()
}

// contentWidth is the text width the title wraps into at outer width outerW: the
// card inset by CardPadX on each side, less the thumbnail column and its gap when
// present. Clamped to at least 1 so a pathologically narrow card still wraps.
func (c *PostCard) contentWidth(outerW int) int {
	cw := outerW - 2*scaled(CardPadX)
	if c.reservesThumb() {
		tw, _ := c.thumbSize()
		cw -= tw + scaled(CardGapX)
	}
	if cw < 1 {
		cw = 1
	}
	return cw
}

// titleLines word-wraps the title to content width cw and clamps it to maxLines,
// ellipsising the last kept line on overflow. An empty title yields no lines.
func (c *PostCard) titleLines(cw int) []string {
	f := c.titleFont()
	return clampLines(f, wrapText(f, c.Title, cw), c.maxLines(), cw)
}

// contentHeight is the content column's height at content width cw: the badge
// row, the wrapped title, a CardGapY spacer and the meta line stacked. When a
// thumbnail is present and taller than that stack, the thumbnail dictates the
// height instead so the image is never clipped by a short card.
func (c *PostCard) contentHeight(cw int) int {
	n := len(c.titleLines(cw))
	h := c.badgeRowH() + n*c.titleSlot() + scaled(CardGapY) + c.metaH()
	if c.reservesThumb() {
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
	return 2*scaled(CardPadY) + c.contentHeight(c.contentWidth(width))
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
	return Rect{X: r.X + scaled(CardPadX), Y: r.Y + scaled(CardPadY), W: r.W - 2*scaled(CardPadX), H: r.H - 2*scaled(CardPadY)}
}

// assemble (re)builds the card's widget tree from the current fields and lays it
// out inside inner, storing the leaf Labels for Children / CollectRuns. It
// returns the root HBox for Draw to paint. The badge row (pill + subtitle) is a
// fixed row; the wrapped title is one Label per line; a flex spacer pushes the
// meta line to the bottom; the thumbnail, when present, is pinned to the top of a
// fixed right-hand column.
func (c *PostCard) assemble(inner Rect) *HBox {
	contentW := inner.W
	if c.reservesThumb() {
		tw, _ := c.thumbSize()
		contentW -= tw + scaled(CardGapX)
	}
	if contentW < 1 {
		contentW = 1
	}

	col := NewVBox()
	col.Spacing = 0

	c.subtitleLbl = nil
	if c.badgeRowH() > 0 {
		row := NewHBox()
		row.Spacing = scaled(CardGapX)
		if c.Pill != "" {
			badge := &Badge{Text: c.Pill, Fill: c.PillColor, Ink: c.pillInk()}
			badge.Font = c.pillFont()
			row.AddFixed(badge, c.pillFont().Measure(c.Pill)+2*scaled(BadgePadX))
		}
		if c.Subtitle != "" {
			c.subtitleLbl = NewLabel(c.Subtitle)
			c.subtitleLbl.Font = c.subtitleFont()
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
		lbl.Font = c.titleFont()
		lbl.Ellipsis = true
		c.titleLbls = append(c.titleLbls, lbl)
		col.AddFixed(lbl, c.titleSlot())
	}

	col.AddFlex(NewLabel(""), 1) // spacer: pushes the meta line to the bottom

	c.metaLbl = nil
	if c.Meta != "" {
		c.metaLbl = NewLabel(c.Meta)
		c.metaLbl.Font = c.metaFont()
		c.metaLbl.Ellipsis = true
		c.metaLbl.VAlign = VBottom
		col.AddFixed(c.metaLbl, c.metaH())
	}

	root := NewHBox()
	root.Spacing = scaled(CardGapX)
	root.AddFlex(col, 1)
	if c.reservesThumb() {
		tw, th := c.thumbSize()
		tcol := NewVBox()
		tcol.Spacing = 0
		tcol.AddFixed(&postThumb{img: c.Thumbnail, placeholder: c.ThumbPlaceholder, phFont: c.metaFont()}, th)
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
// preserving aspect ratio and centred. When no image has landed but a placeholder
// label is set, it draws that label centred in muted ink instead, so a card whose
// media has not decoded yet reads as a labelled "loading" box rather than a blank
// one. It is laid out like any other child by the thumbnail column's box.
type postThumb struct {
	Base
	img         *image.RGBA
	placeholder string
	phFont      Font
}

// Draw fills the box with the theme's SurfaceAlt then blits the image scaled to
// fit (aspect-preserving, centred). With no image it leaves the muted ground, and
// draws the placeholder label centred in dim ink when one is set; an empty box
// paints nothing.
func (t *postThumb) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	if t.img == nil {
		t.drawPlaceholder(p, theme, r)
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

// drawPlaceholder centres the placeholder label in the box in dim ink, matching
// the historical "loading" thumbnail. It is a no-op when no label or font is set,
// or when the label would not fit the box.
func (t *postThumb) drawPlaceholder(p painter.Painter, theme *Theme, r Rect) {
	if t.placeholder == "" || t.phFont == nil {
		return
	}
	tw := t.phFont.Measure(t.placeholder)
	th := t.phFont.Height()
	if tw > r.W || th > r.H {
		return
	}
	x := r.X + (r.W-tw)/2
	y := r.Y + (r.H-th)/2
	t.phFont.Draw(p, x, y, t.placeholder, dimInk(theme))
}
