// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"

	"github.com/go-images/images"
	"github.com/go-widgets/painter"
)

// Thumbnail renders a source RGBA buffer scaled down (aspect-preserved,
// centred) into its bounds, with an optional caption strip and a
// selected/hover border. It is the window-preview tile an Exposé grid, an
// Alt-Tab switcher, or a dock-hover peek is built from: give it the client's
// framebuffer + a title, size it into a grid cell, and it paints a shrunk
// snapshot with a label under it.
//
// Downscale quality: Nearest (the zero value) is one sample per destination
// pixel — fast, fine for a live-updating peek. Area averages the source region
// each destination pixel covers, so a large snapshot shrunk to a small tile
// stays legible instead of shimmering; use it for static previews.
//
// The average comes from go-images (images.Area, the same box filter as PIL's
// Image.BOX and OpenCV's INTER_AREA) rather than from a loop written here: it
// is an image-processing operation and go-images is where those live. It is
// also weighted by fractional coverage where the toolkit's own loop truncated
// the box to whole source pixels, so an uneven ratio now averages what it
// actually covers.
//
// Because a scaled image depends only on the source and the target size, it is
// computed once and kept. A caller that overwrites the CONTENTS of Pixels in
// place must call Invalidate; assigning a new buffer through SetPixels does it
// for them. This is the static-preview case by construction — a live-updating
// peek wants Nearest, which keeps no cache and reads Pixels every frame.
//
// Selection: Selected draws a 2-px Accent border (the switcher's current
// choice); Hover draws a 1-px Accent border (the pointer is over the tile).
// Selected wins when both are set. With neither, a plain 1-px Border frames the
// image. OnClick fires on EventClick so a grid can select the tile.
type Thumbnail struct {
	Base
	// Pixels is the source RGBA image (IW*IH*4 bytes); an invalid buffer paints
	// just the frame + label.
	Pixels []byte
	IW, IH int
	// Label is an optional caption drawn in a strip along the bottom; empty
	// gives the whole cell to the image.
	Label string
	// Alt is the tile's accessible name when it should differ from the visible
	// Label — a filename shown as the caption, say, while the picture itself is
	// worth describing. Empty falls back to Label.
	Alt string
	// Selected / Hover drive the border (see the type doc). Area selects the
	// box-averaging downscale over the default nearest-neighbour.
	Selected bool
	Hover    bool
	Area     bool
	// OnClick fires on EventClick (nil-safe) so a container can select the tile.
	OnClick func()

	// areaCache holds the last Area downscale, valid for the source and the
	// destination size recorded beside it.
	areaCache          []byte
	areaW, areaH       int
	areaSrcW, areaSrcH int
	areaSrcLen         int
}

// SetPixels replaces the source image and drops any cached downscale.
func (t *Thumbnail) SetPixels(pixels []byte, w, h int) {
	t.Pixels, t.IW, t.IH = pixels, w, h
	t.Invalidate()
}

// Invalidate drops the cached Area downscale. Call it after overwriting the
// contents of Pixels in place; SetPixels already does.
func (t *Thumbnail) Invalidate() {
	t.areaCache, t.areaW, t.areaH = nil, 0, 0
}

// ThumbnailLabelPad is the vertical padding above + below the caption text in
// the label strip.
const ThumbnailLabelPad = 2

// NewThumbnail wraps a source image (length must equal w*h*4) in a nearest-
// neighbour thumbnail with no caption.
func NewThumbnail(pixels []byte, w, h int) *Thumbnail {
	return &Thumbnail{Pixels: pixels, IW: w, IH: h}
}

// hasImage reports whether Pixels is a usable RGBA buffer for the source dims.
func (t *Thumbnail) hasImage() bool {
	return t.IW > 0 && t.IH > 0 && len(t.Pixels) >= t.IW*t.IH*4
}

// labelStrip is the height reserved at the bottom for the caption (0 when
// Label is empty).
func (t *Thumbnail) labelStrip() int {
	if t.Label == "" {
		return 0
	}
	return t.glyphHeight() + 2*ThumbnailLabelPad
}

// Draw paints the (downscaled) image into the image area, the optional caption
// strip, then the selection/hover/plain border over the whole cell. An empty
// rectangle paints nothing.
func (t *Thumbnail) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	area := Rect{X: r.X, Y: r.Y, W: r.W, H: r.H - t.labelStrip()}
	if t.hasImage() && area.W > 0 && area.H > 0 {
		dst := FitBounds(t.IW, t.IH, area)
		if t.Area {
			t.drawArea(p, dst)
		} else {
			t.drawNearest(p, dst)
		}
	}
	if t.Label != "" {
		strip := Rect{X: r.X, Y: r.Y + r.H - t.labelStrip(), W: r.W, H: t.labelStrip()}
		fillRect(p, strip.X, strip.Y, strip.W, strip.H, theme.Surface)
		tw := t.textWidth(t.Label)
		tx := strip.X + (strip.W-tw)/2
		ty := strip.Y + ThumbnailLabelPad
		t.drawText(p, tx, ty, t.Label, theme.OnSurface)
	}
	switch {
	case t.Selected:
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Accent)
		strokeRect(p, r.X+1, r.Y+1, r.W-2, r.H-2, theme.Accent)
	case t.Hover:
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Accent)
	default:
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	}
}

// drawNearest samples one source pixel per destination pixel.
func (t *Thumbnail) drawNearest(p painter.Painter, dst Rect) {
	blitImage(p, dst, dst, t.Pixels, t.IW, t.IH)
}

// drawArea blits the cached area downscale, computing it first if the source or
// the destination size has changed since it was made.
func (t *Thumbnail) drawArea(p painter.Painter, dst Rect) {
	if !t.areaValid(dst) {
		t.buildArea(dst)
	}
	// The cache is already the destination size, so this is a 1:1 blit and the
	// painter copies it a row at a time.
	blitImage(p, dst, dst, t.areaCache, dst.W, dst.H)
}

// areaValid reports whether the cache was made from this source at this size.
func (t *Thumbnail) areaValid(dst Rect) bool {
	return t.areaCache != nil &&
		t.areaW == dst.W && t.areaH == dst.H &&
		t.areaSrcW == t.IW && t.areaSrcH == t.IH && t.areaSrcLen == len(t.Pixels)
}

// buildArea scales the source down once, through go-images rather than a loop
// of our own. The source is wrapped, not copied: image.RGBA is a header over
// exactly the bytes Pixels already holds.
func (t *Thumbnail) buildArea(dst Rect) {
	src := &image.RGBA{
		Pix:    t.Pixels,
		Stride: t.IW * 4,
		Rect:   image.Rect(0, 0, t.IW, t.IH),
	}
	// The error case is dst.W or dst.H being non-positive, which Draw has
	// already excluded; falling back to nearest rather than panicking keeps a
	// future caller's mistake to a worse picture instead of a crash.
	out, err := images.Resize(src, dst.W, dst.H, images.Area)
	if err != nil {
		t.areaCache, t.areaW, t.areaH = nil, 0, 0
		return
	}
	t.areaCache = out.Pix
	t.areaW, t.areaH = dst.W, dst.H
	t.areaSrcW, t.areaSrcH, t.areaSrcLen = t.IW, t.IH, len(t.Pixels)
}

// OnEvent fires OnClick on EventClick; other event kinds are ignored. OnClick
// is nil-safe.
func (t *Thumbnail) OnEvent(ev Event) {
	if ev.Kind == EventClick && t.OnClick != nil {
		t.OnClick()
	}
}
