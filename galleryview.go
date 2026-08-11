// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// GalleryView is a preview-plus-filmstrip browser: a large preview of the
// current item filling the top region, and a horizontally-scrolling row of
// small thumbnails along the bottom. It generalizes a file-manager "gallery
// view" (macOS Finder's Gallery) — where an icon grid shows every item at the
// same small size, a GalleryView commits most of its area to ONE big preview
// and relegates the rest to a filmstrip, so a caller browsing photos or
// documents reads the current item large while still seeing its neighbours.
//
// Layout: the body is filled with Theme.Surface; the top region (70% of the
// height) is the preview — the current item's raster thumbnail fit and centred
// inside a subtle rounded frame (a dark raster sits on a light backing chip so
// it stays visible on a dark theme), or a document glyph for a non-image item,
// with the item's label in a caption band beneath it. The bottom region (30%)
// is the filmstrip: a Theme.SurfaceAlt band under a hairline Theme.Border, laid
// out left to right at a uniform thumbnail size, scrolled horizontally and
// clipped to the widget bounds. The selected thumbnail is centred in the band
// and drawn with a soft accent field and an accent ring.
//
// Selection + navigation: a click selects the thumbnail under the pointer
// (firing OnSelect); a second click on the already-selected thumbnail activates
// it (firing OnActivate). Left/Right (or ArrowLeft/ArrowRight) move the
// selection and auto-scroll the strip to keep it centred; Home/End jump to the
// ends; Enter/Return/Space activate the current item. Selected / SetSelected
// read and drive the selection programmatically. Because a gallery always shows
// a current item, a fresh GalleryView with at least one item selects index 0.
type GalleryView struct {
	Base

	// Items is the ordered gallery content. Mutate it directly and call
	// SetItems (or SetBounds) to re-normalize the selection and strip scroll.
	Items []GalleryItem

	// Empty is the message centred when there are no items; a blank Empty falls
	// back to a generic default.
	Empty string

	// OnSelect fires when a click or key moves the selection to a NEW item, with
	// its index. Nil-guarded.
	OnSelect func(index int)

	// OnActivate fires when the already-selected item is clicked again, or
	// Enter/Return/Space is pressed, with its index. Nil-guarded.
	OnActivate func(index int)

	sel         int // selected index, or -1 when nothing is selected
	stripScroll int // filmstrip horizontal scroll offset in pixels
}

// GalleryItem is one gallery entry: a thumbnail/icon and a label. Raster marks
// Image as a real raster thumbnail (a photo, a rendered preview) so a light chip
// is painted behind it; leave it false for a flat vector/symbol icon that needs
// no backing. A nil Image with Raster false draws a document glyph. Key is an
// opaque caller identity the widget never interprets.
type GalleryItem struct {
	Image  *Image
	Label  string
	Key    string
	Raster bool
}

// GalleryView metrics (pixels).
const (
	gvStripPct   = 30 // filmstrip height as a percent of the widget height
	gvCaptionH   = 28 // caption band height under the preview image
	gvPreviewPad = 20 // padding around the preview image inside the preview region
	gvStripPad   = 10 // padding around thumbnails inside the filmstrip band
	gvThumbGap   = 12 // horizontal gap between filmstrip thumbnails
	gvChip       = 3  // inset of the light chip around a raster preview/thumbnail
	gvRing       = 3  // how far the selection ring sits outside a thumbnail
	gvRingW      = 2  // selection ring stroke width
	gvFrameRad   = 8  // corner radius of the preview frame / chips
	gvThumbRad   = 6  // corner radius of a filmstrip thumbnail
)

// NewGalleryView builds a GalleryView over items. With at least one item the
// first is selected (a gallery always shows a current item); with none nothing
// is selected (Selected returns -1). Call SetBounds to lay it out before
// drawing.
func NewGalleryView(items ...GalleryItem) *GalleryView {
	g := &GalleryView{Items: items, sel: -1}
	g.normalize()
	return g
}

// GalleryView describes itself for accessibility.
var _ Accessible = (*GalleryView)(nil)

// A11y reports the GalleryView as a grid. Value is the current item's label, or
// empty when nothing is selected.
func (g *GalleryView) A11y() A11yInfo {
	label := ""
	if g.sel >= 0 && g.sel < len(g.Items) {
		label = g.Items[g.sel].Label
	}
	return A11yInfo{Role: RoleGrid, Value: label}
}

// SetItems replaces the gallery content and re-normalizes the selection (a
// now-out-of-range or unset selection snaps to the first item, or clears when
// there are no items) and the strip scroll.
func (g *GalleryView) SetItems(items []GalleryItem) {
	g.Items = items
	g.normalize()
}

// Selected returns the current item index, or -1 when nothing is selected.
func (g *GalleryView) Selected() int { return g.sel }

// SetSelected selects item index and auto-scrolls the strip to keep it centred;
// an out-of-range index clears the selection to -1 (the preview goes blank).
// Unlike a key/click move it does not fire OnSelect.
func (g *GalleryView) SetSelected(index int) {
	if index >= 0 && index < len(g.Items) {
		g.sel = index
	} else {
		g.sel = -1
	}
	g.scrollToSelection()
}

// SetBounds records the widget bounds and re-anchors the strip scroll so the
// current selection stays centred at the new size.
func (g *GalleryView) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	g.scrollToSelection()
}

// normalize clamps the selection into range (an unset selection snaps to the
// first item when there is one, an empty gallery clears it) and re-anchors the
// strip scroll.
func (g *GalleryView) normalize() {
	n := len(g.Items)
	switch {
	case n == 0:
		g.sel = -1
	case g.sel < 0:
		g.sel = 0
	case g.sel >= n:
		g.sel = n - 1
	}
	g.scrollToSelection()
}

// --- geometry ------------------------------------------------------------

// stripH is the filmstrip band height in pixels.
func (g *GalleryView) stripH() int { return g.Bounds().H * gvStripPct / 100 }

// PreviewRect is the top region that shows the large preview (the body minus
// the filmstrip band).
func (g *GalleryView) PreviewRect() Rect {
	b := g.Bounds()
	return Rect{X: b.X, Y: b.Y, W: b.W, H: b.H - g.stripH()}
}

// StripRect is the bottom filmstrip band.
func (g *GalleryView) StripRect() Rect {
	b := g.Bounds()
	sh := g.stripH()
	return Rect{X: b.X, Y: b.Y + b.H - sh, W: b.W, H: sh}
}

// captionRect is the caption band at the bottom of the preview region.
func (g *GalleryView) captionRect() Rect {
	pr := g.PreviewRect()
	return Rect{X: pr.X, Y: pr.Y + pr.H - gvCaptionH, W: pr.W, H: gvCaptionH}
}

// previewImageRect is the padded area above the caption the preview image fits
// into.
func (g *GalleryView) previewImageRect() Rect {
	pr := g.PreviewRect()
	return Rect{
		X: pr.X + gvPreviewPad,
		Y: pr.Y + gvPreviewPad,
		W: pr.W - 2*gvPreviewPad,
		H: pr.H - gvCaptionH - 2*gvPreviewPad,
	}
}

// thumbSize is the side of a filmstrip thumbnail square.
func (g *GalleryView) thumbSize() int {
	s := g.StripRect().H - 2*gvStripPad
	if s < 1 {
		s = 1
	}
	return s
}

// cellW is a thumbnail plus its trailing gap.
func (g *GalleryView) cellW() int { return g.thumbSize() + gvThumbGap }

// contentW is the total pixel width of the filmstrip content, including the
// leading and trailing pad and every thumbnail but not the final trailing gap.
func (g *GalleryView) contentW() int {
	n := len(g.Items)
	if n == 0 {
		return 0
	}
	return 2*gvStripPad + n*g.cellW() - gvThumbGap
}

// maxStripScroll is the largest valid strip scroll (0 when the content fits).
func (g *GalleryView) maxStripScroll() int {
	m := g.contentW() - g.StripRect().W
	if m < 0 {
		m = 0
	}
	return m
}

// clampStripScroll clamps s into [0, maxStripScroll].
func (g *GalleryView) clampStripScroll(s int) int {
	if s < 0 {
		return 0
	}
	if m := g.maxStripScroll(); s > m {
		return m
	}
	return s
}

// scrollToSelection re-anchors the strip so the selected thumbnail is centred
// (clamped at both ends); with nothing selected it just re-clamps the offset.
func (g *GalleryView) scrollToSelection() {
	if g.sel < 0 {
		g.stripScroll = g.clampStripScroll(g.stripScroll)
		return
	}
	sr := g.StripRect()
	want := gvStripPad + g.sel*g.cellW() + g.thumbSize()/2 - sr.W/2
	g.stripScroll = g.clampStripScroll(want)
}

// ThumbRect returns the on-screen rectangle of thumbnail i (accounting for the
// strip scroll) and whether it is at least partly visible in the strip band.
// ok is false for an out-of-range index.
func (g *GalleryView) ThumbRect(i int) (Rect, bool) {
	if i < 0 || i >= len(g.Items) {
		return Rect{}, false
	}
	sr := g.StripRect()
	ts := g.thumbSize()
	r := Rect{
		X: sr.X + gvStripPad - g.stripScroll + i*g.cellW(),
		Y: sr.Y + (sr.H-ts)/2,
		W: ts,
		H: ts,
	}
	visible := r.X+r.W > sr.X && r.X < sr.X+sr.W
	return r, visible
}

// ThumbAt maps a widget-local point to a thumbnail index, or -1 for the gap
// between thumbnails, past the last thumbnail, or outside the strip band.
func (g *GalleryView) ThumbAt(x, y int) int {
	sr := g.StripRect()
	if !sr.Contains(x, y) {
		return -1
	}
	local := x - sr.X - gvStripPad + g.stripScroll
	if local < 0 {
		return -1
	}
	cw := g.cellW()
	i := local / cw
	if local-i*cw >= g.thumbSize() {
		return -1 // in the inter-thumbnail gap
	}
	if i >= len(g.Items) {
		return -1
	}
	return i
}

// --- rendering -----------------------------------------------------------

// Draw paints the preview and filmstrip (or the empty-state message), clipped
// to the widget bounds.
func (g *GalleryView) Draw(p painter.Painter, theme *Theme) {
	b := g.Bounds()
	fillRect(p, b.X, b.Y, b.W, b.H, theme.Surface)
	if len(g.Items) == 0 {
		g.drawEmpty(p, theme)
		return
	}
	withClip(p, b, func() {
		g.drawPreview(p, theme)
		g.drawStrip(p, theme)
	})
}

// drawEmpty centres the empty-state message.
func (g *GalleryView) drawEmpty(p painter.Painter, theme *Theme) {
	msg := g.Empty
	if msg == "" {
		msg = "No items"
	}
	b := g.Bounds()
	tw := g.textWidth(msg)
	g.drawText(p, b.X+(b.W-tw)/2, b.Y+b.H/2-g.glyphHeight()/2, msg, mutedInk(theme))
}

// drawPreview paints the large centred preview of the current item — its raster
// thumbnail on a light chip, or a document glyph for a non-image item — inside a
// subtle rounded frame, then the caption. With nothing selected the preview is
// blank.
func (g *GalleryView) drawPreview(p painter.Painter, theme *Theme) {
	if g.sel < 0 {
		return
	}
	item := g.Items[g.sel]
	area := g.previewImageRect()

	var fit Rect
	if item.Image != nil {
		fit = FitBounds(item.Image.W, item.Image.H, area)
	} else {
		fit = squareIn(area)
	}

	if item.Raster {
		chip := expandRect(fit, gvChip)
		p.FillRoundRect(chip, gvFrameRad, chipColor(theme))
		p.StrokeRoundRect(chip, gvFrameRad, theme.Border, 1)
	}
	switch {
	case item.Image != nil:
		item.Image.SetBounds(fit)
		item.Image.Draw(p, theme)
	case !item.Raster:
		g.drawDocGlyph(p, theme, fit)
	}
	p.StrokeRoundRect(fit, gvFrameRad, theme.Border, 1)

	g.drawCaption(p, theme, item.Label)
}

// drawDocGlyph paints a document-page glyph filling r: a rounded page with a few
// muted text lines, standing in for a non-image item's large preview.
func (g *GalleryView) drawDocGlyph(p painter.Painter, theme *Theme, r Rect) {
	p.FillRoundRect(r, gvFrameRad, theme.SurfaceAlt)
	lineX := r.X + r.W/5
	lineW := r.W - 2*r.W/5
	lineH := r.H / 12
	if lineH < 1 {
		lineH = 1
	}
	for i := 1; i <= 3; i++ {
		y := r.Y + r.H*i/4
		fillRect(p, lineX, y, lineW, lineH, mutedInk(theme))
	}
}

// drawCaption paints the current item's label centred in the caption band,
// elided to the band width.
func (g *GalleryView) drawCaption(p painter.Painter, theme *Theme, label string) {
	cr := g.captionRect()
	if g.textWidth(label) > cr.W-2*gvPreviewPad {
		label = ellipsize(g.EffectiveFont(), label, cr.W-2*gvPreviewPad)
	}
	tw := g.textWidth(label)
	g.drawText(p, cr.X+(cr.W-tw)/2, cr.Y+(cr.H-g.glyphHeight())/2, label, theme.OnSurface)
}

// drawStrip paints the filmstrip band, its top hairline, and every visible
// thumbnail, clipped to the band.
func (g *GalleryView) drawStrip(p painter.Painter, theme *Theme) {
	sr := g.StripRect()
	fillRect(p, sr.X, sr.Y, sr.W, sr.H, theme.SurfaceAlt)
	fillRect(p, sr.X, sr.Y, sr.W, 1, theme.Border)
	withClip(p, sr, func() {
		for i := range g.Items {
			r, ok := g.ThumbRect(i)
			if !ok {
				continue
			}
			g.drawThumb(p, theme, r, i)
		}
	})
}

// drawThumb paints one filmstrip thumbnail: the selection field + accent ring
// when selected, the light chip behind a raster image, then the image (or a
// small document glyph for a non-image item).
func (g *GalleryView) drawThumb(p painter.Painter, theme *Theme, r Rect, i int) {
	item := g.Items[i]
	if i == g.sel {
		field := expandRect(r, gvRing)
		p.FillRoundRect(field, gvThumbRad, withAlpha(theme.Accent, 0x33))
		p.StrokeRoundRect(field, gvThumbRad, theme.Accent, gvRingW)
	}
	if item.Raster {
		p.FillRoundRect(r, gvThumbRad, chipColor(theme))
		p.StrokeRoundRect(r, gvThumbRad, theme.Border, 1)
	}
	switch {
	case item.Image != nil:
		item.Image.SetBounds(r)
		item.Image.Draw(p, theme)
	case !item.Raster:
		g.drawDocGlyph(p, theme, r)
	}
}

// OnEvent moves the selection on Left/Right/Home/End, activates on
// Enter/Return/Space, selects on a click, activates on a second click of the
// selected thumbnail, and is inert while Disabled or empty.
func (g *GalleryView) OnEvent(ev Event) {
	if g.Disabled {
		return
	}
	n := len(g.Items)
	if n == 0 {
		return
	}
	switch ev.Kind {
	case EventKeyDown:
		g.handleKey(ev, n)
	case EventClick:
		idx := g.ThumbAt(ev.X, ev.Y)
		if idx < 0 {
			return
		}
		if idx == g.sel {
			if g.OnActivate != nil {
				g.OnActivate(idx)
			}
			return
		}
		g.moveSelection(idx)
	}
}

// handleKey drives keyboard navigation over a non-empty gallery.
func (g *GalleryView) handleKey(ev Event, n int) {
	switch ev.Code {
	case "ArrowRight", "Right":
		if g.sel < 0 {
			g.moveSelection(0)
		} else {
			g.moveSelection(g.sel + 1)
		}
	case "ArrowLeft", "Left":
		if g.sel < 0 {
			g.moveSelection(0)
		} else {
			g.moveSelection(g.sel - 1)
		}
	case "Home":
		g.moveSelection(0)
	case "End":
		g.moveSelection(n - 1)
	case "Enter", "Return", " ", "Space":
		if g.sel >= 0 && g.OnActivate != nil {
			g.OnActivate(g.sel)
		}
	}
}

// moveSelection selects the clamped index to, auto-scrolls the strip to keep it
// centred, and fires OnSelect only when the selection actually changed.
func (g *GalleryView) moveSelection(to int) {
	if to < 0 {
		to = 0
	}
	if to >= len(g.Items) {
		to = len(g.Items) - 1
	}
	if to == g.sel {
		g.scrollToSelection()
		return
	}
	g.sel = to
	g.scrollToSelection()
	if g.OnSelect != nil {
		g.OnSelect(to)
	}
}

// expandRect returns r grown by d pixels on every side.
func expandRect(r Rect, d int) Rect {
	return Rect{X: r.X - d, Y: r.Y - d, W: r.W + 2*d, H: r.H + 2*d}
}

// squareIn returns the largest centred square that fits inside r — the preview
// footprint for a non-image (glyph) item, so its frame reads as a square page.
func squareIn(r Rect) Rect {
	s := r.W
	if r.H < s {
		s = r.H
	}
	return Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}
}
