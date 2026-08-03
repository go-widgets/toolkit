// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Thumbnail renders a source RGBA buffer scaled down (aspect-preserved,
// centred) into its bounds, with an optional caption strip and a
// selected/hover border. It is the window-preview tile an Exposé grid, an
// Alt-Tab switcher, or a dock-hover peek is built from: give it the client's
// framebuffer + a title, size it into a grid cell, and it paints a shrunk
// snapshot with a label under it.
//
// Downscale quality: Nearest (the zero value) is one sample per destination
// pixel — fast, fine for a live-updating peek. Area averages the whole source
// box that maps to each destination pixel, so a large snapshot shrunk to a
// small tile stays legible instead of shimmering; use it for static previews.
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
	// Selected / Hover drive the border (see the type doc). Area selects the
	// box-averaging downscale over the default nearest-neighbour.
	Selected bool
	Hover    bool
	Area     bool
	// OnClick fires on EventClick (nil-safe) so a container can select the tile.
	OnClick func()
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
	for dy := 0; dy < dst.H; dy++ {
		sy := dy * t.IH / dst.H
		for dx := 0; dx < dst.W; dx++ {
			sx := dx * t.IW / dst.W
			p.PutPixel(dst.X+dx, dst.Y+dy, samplePixel(t.Pixels, t.IW, sx, sy))
		}
	}
}

// drawArea averages the source box that maps to each destination pixel — a box
// (area) downscale that keeps a large snapshot legible when shrunk hard.
func (t *Thumbnail) drawArea(p painter.Painter, dst Rect) {
	for dy := 0; dy < dst.H; dy++ {
		sy0 := dy * t.IH / dst.H
		sy1 := (dy + 1) * t.IH / dst.H
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dst.W; dx++ {
			sx0 := dx * t.IW / dst.W
			sx1 := (dx + 1) * t.IW / dst.W
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			p.PutPixel(dst.X+dx, dst.Y+dy, t.boxAverage(sx0, sy0, sx1, sy1))
		}
	}
}

// boxAverage is the mean RGBA over the source box [sx0,sx1) x [sy0,sy1).
func (t *Thumbnail) boxAverage(sx0, sy0, sx1, sy1 int) RGBA {
	var sr, sg, sb, sa, n int
	for sy := sy0; sy < sy1; sy++ {
		for sx := sx0; sx < sx1; sx++ {
			c := samplePixel(t.Pixels, t.IW, sx, sy)
			sr += int(c.R)
			sg += int(c.G)
			sb += int(c.B)
			sa += int(c.A)
			n++
		}
	}
	return RGBA{R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: uint8(sa / n)}
}

// OnEvent fires OnClick on EventClick; other event kinds are ignored. OnClick
// is nil-safe.
func (t *Thumbnail) OnEvent(ev Event) {
	if ev.Kind == EventClick && t.OnClick != nil {
		t.OnClick()
	}
}
