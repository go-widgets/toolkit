// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image/color"
	"sync"

	"github.com/go-gfx/gfx/svg"
	"github.com/go-widgets/painter"
)

// SVGIcon returns an icon drawer that renders the SVG document doc into the box
// it is handed — for a [TreeTableNode.Icon], or anywhere a
// func(painter.Painter, Rect, RGBA) drawer is wanted. The SVG is rasterised
// ONCE per (document, ink) and cached; each draw just scales the cached bitmap
// into the target box through the painter's DrawImage, so a file tree can show a
// real icon-pack glyph without re-parsing the SVG every frame.
//
// ink recolours the SVG's default / currentColor fills; a pack whose icons carry
// their own colours keeps them. A document that fails to parse caches empty and
// draws nothing, so one bad icon never panics a tree. A painter that cannot blit
// an image draws nothing (only PixelPainter and CellPainter implement
// painter.ImagePainter — both do).
func SVGIcon(doc string) func(p painter.Painter, r Rect, ink RGBA) {
	return func(p painter.Painter, r Rect, ink RGBA) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		ip, ok := p.(painter.ImagePainter)
		if !ok {
			return
		}
		ras := rasterizeIcon(doc, ink)
		if ras.pix == nil {
			return
		}
		ip.DrawImage(Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, ras.pix, ras.w, ras.h)
	}
}

// iconRaster is a cached rasterised icon: straight-alpha RGBA, w*h pixels.
type iconRaster struct {
	pix  []byte
	w, h int
}

// iconKey identifies a cached raster by its source document and recolour ink.
type iconKey struct {
	doc string
	ink RGBA
}

// iconCache memoises rasterised icons so the SVG is parsed once per (doc, ink),
// not once per frame. It is safe for concurrent use (a native host may draw off
// the main goroutine); the wasm host is single-threaded and pays nothing.
var iconCache sync.Map // iconKey -> iconRaster

// rasterizeIcon renders doc at the default scale, recoloured to ink, with a
// transparent background, and caches the result (empty on any parse error).
func rasterizeIcon(doc string, ink RGBA) iconRaster {
	key := iconKey{doc: doc, ink: ink}
	if v, ok := iconCache.Load(key); ok {
		return v.(iconRaster)
	}
	var ras iconRaster
	res, err := svg.Rasterize(doc, svg.Options{Ink: color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A}})
	if err == nil && res != nil && res.Image != nil && res.Image.W > 0 && res.Image.H > 0 {
		ras = iconRaster{pix: res.Image.Pix, w: res.Image.W, h: res.Image.H}
	}
	iconCache.Store(key, ras)
	return ras
}
