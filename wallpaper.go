// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// WallpaperMode selects how a [Wallpaper]'s source image maps onto its bounds.
type WallpaperMode int

const (
	// WallpaperFill ("cover") scales the image — preserving aspect — to the
	// smallest size that covers the whole bounds, centring it and cropping the
	// overflow. It is the zero value: the desktop-wallpaper default where the
	// picture fills the screen edge-to-edge.
	WallpaperFill WallpaperMode = iota
	// WallpaperFit ("contain") scales the image — preserving aspect — to the
	// largest size that fits entirely within the bounds and centres it; the
	// margin around it shows the fallback fill.
	WallpaperFit
	// WallpaperCenter paints the image 1:1 (no scaling) centred in the bounds;
	// an image smaller than the bounds shows the fallback around it, a larger
	// one is cropped to the bounds.
	WallpaperCenter
	// WallpaperTile repeats the image 1:1 from the top-left to cover the whole
	// bounds — the classic pattern backdrop.
	WallpaperTile
)

// Wallpaper is a full-bounds desktop backdrop: it paints an optional RGBA image
// scaled by Mode (fill / fit / center / tile) over a solid or vertical-gradient
// fallback. It complements [Backdrop] (a flat fill + optional grid) for the case
// a compositor actually wants a picture behind the scene.
//
// Fallback: the ground under (and around) the image is a vertical gradient from
// Top to Bottom. A zero (A==0) Top falls back to Theme.Background; a zero Bottom
// makes the fill a solid Top (no gradient). So a bare Wallpaper with no colours
// set reads as the theme background, a single opaque Top is a solid colour, and
// Top+Bottom is a gradient — all without an image.
//
// Like the corrected [Backdrop], a Wallpaper is event-transparent by default:
// its HitTest returns false so clicks pass THROUGH to the widgets composited
// over it (a full-cover backdrop that reported hits would swallow every click).
// Set Interactive to opt back in (a picker preview that should catch clicks).
type Wallpaper struct {
	Base
	// Pixels is the RGBA image (IW*IH*4 bytes); nil/empty paints only the
	// fallback.
	Pixels []byte
	IW, IH int
	// Mode selects the image placement (default WallpaperFill / cover).
	Mode WallpaperMode
	// Top / Bottom are the fallback gradient stops (top → bottom). Zero Top
	// (A==0) => Theme.Background; zero Bottom (A==0) => solid Top (no gradient).
	Top, Bottom RGBA
	// Interactive makes the Wallpaper catch pointer events; the zero value is
	// event-transparent (clicks pass through to the content above it).
	Interactive bool
}

// NewWallpaper builds an image Wallpaper (length must equal w*h*4) in the given
// mode. The fallback colours are left zero so uncovered margins read as the
// theme background.
func NewWallpaper(pixels []byte, w, h int, mode WallpaperMode) *Wallpaper {
	return &Wallpaper{Pixels: pixels, IW: w, IH: h, Mode: mode}
}

// NewWallpaperGradient builds an image-less Wallpaper that paints a vertical
// gradient from top to bottom (pass an equal pair for a solid colour).
func NewWallpaperGradient(top, bottom RGBA) *Wallpaper {
	return &Wallpaper{Top: top, Bottom: bottom}
}

// hasImage reports whether Pixels is a usable RGBA buffer for the source dims.
func (w *Wallpaper) hasImage() bool {
	return w.IW > 0 && w.IH > 0 && len(w.Pixels) >= w.IW*w.IH*4
}

// HitTest returns false unless Interactive is set, so by default a full-cover
// wallpaper lets clicks pass through to the widgets composited over it (the
// Backdrop / Label pass-through idiom). When Interactive is set it hit-tests
// against its Bounds like any other widget.
func (w *Wallpaper) HitTest(px, py int) bool {
	if !w.Interactive {
		return false
	}
	return w.Bounds().Contains(px, py)
}

// Draw paints the fallback ground then, if a valid image is present, the image
// placed per Mode. An empty rectangle paints nothing.
func (w *Wallpaper) Draw(p painter.Painter, theme *Theme) {
	r := w.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	w.paintFallback(p, r, theme)
	if !w.hasImage() {
		return
	}
	switch w.Mode {
	case WallpaperFit:
		img := Image{Pixels: w.Pixels, W: w.IW, H: w.IH, Scale: ScaleFit}
		img.SetBounds(r)
		img.Draw(p, theme)
	case WallpaperCenter:
		w.paintCenter(p, r)
	case WallpaperTile:
		w.paintTile(p, r)
	default: // WallpaperFill (cover)
		w.paintCover(p, r)
	}
}

// paintFallback fills r with the vertical gradient (or a solid colour).
func (w *Wallpaper) paintFallback(p painter.Painter, r Rect, theme *Theme) {
	top := w.Top
	if top.A == 0 {
		top = theme.Background
	}
	if w.Bottom.A == 0 { // solid: no gradient stop
		fillRect(p, r.X, r.Y, r.W, r.H, top)
		return
	}
	for row := 0; row < r.H; row++ {
		fillRect(p, r.X, r.Y+row, r.W, 1, lerpRGBA(top, w.Bottom, row, r.H))
	}
}

// paintCover scales the image to cover the whole rect (aspect-preserved),
// centred, cropping the overflow — nearest-neighbour.
func (w *Wallpaper) paintCover(p painter.Painter, r Rect) {
	scaledW, scaledH := r.W, w.IH*r.W/w.IW
	if scaledH < r.H {
		scaledH, scaledW = r.H, w.IW*r.H/w.IH
	}
	ox := r.X + (r.W-scaledW)/2
	oy := r.Y + (r.H-scaledH)/2
	// The scaled image covers r by construction, so painting it whole and
	// letting the clip crop the overflow samples exactly what the per-pixel
	// version sampled -- its clamp never bit inside r.
	blitImage(p, Rect{X: ox, Y: oy, W: scaledW, H: scaledH}, r, w.Pixels, w.IW, w.IH)
}

// paintCenter blits the image 1:1 centred in r, clipping to r.
func (w *Wallpaper) paintCenter(p painter.Painter, r Rect) {
	ox := r.X + (r.W-w.IW)/2
	oy := r.Y + (r.H-w.IH)/2
	blitImage(p, Rect{X: ox, Y: oy, W: w.IW, H: w.IH}, r, w.Pixels, w.IW, w.IH)
}

// paintTile repeats the image 1:1 from the top-left to cover r.
func (w *Wallpaper) paintTile(p painter.Painter, r Rect) {
	// One 1:1 blit per tile: each is a whole image at its own origin, and the
	// clip trims the tiles that run off the right and bottom edges.
	for ty := r.Y; ty < r.Y+r.H; ty += w.IH {
		for tx := r.X; tx < r.X+r.W; tx += w.IW {
			blitImage(p, Rect{X: tx, Y: ty, W: w.IW, H: w.IH}, r, w.Pixels, w.IW, w.IH)
		}
	}
}

// lerpRGBA interpolates from a (num==0) to b (num==den-1) linearly. den<=1
// returns a. The result is opaque.
func lerpRGBA(a, b RGBA, num, den int) RGBA {
	if den <= 1 {
		return a
	}
	d := den - 1
	return RGBA{
		R: uint8(int(a.R) + (int(b.R)-int(a.R))*num/d),
		G: uint8(int(a.G) + (int(b.G)-int(a.G))*num/d),
		B: uint8(int(a.B) + (int(b.B)-int(a.B))*num/d),
		A: 255,
	}
}
