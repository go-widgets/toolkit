// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image/color"
	"strconv"
	"strings"
	"sync"

	"github.com/go-gfx/gfx/svg"
	"github.com/go-widgets/painter"
)

// iconRasterPx is the pixel size an icon SVG is rasterised to before the painter
// scales it into the (usually smaller) target box. It bounds the work and memory
// regardless of the SVG's own viewBox — an icon authored on a 960-unit grid and
// one on a 24-unit grid both rasterise to roughly this many pixels, not 960×2 vs
// 24×2. Chosen a touch above a comfortable HiDPI row height, so the downscale to
// a ~14–28 px tree row stays crisp.
const iconRasterPx = 64.0

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
	res, err := svg.Rasterize(doc, svg.Options{
		Scale: iconScale(doc),
		Ink:   color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A},
	})
	if err == nil && res != nil && res.Image != nil && res.Image.W > 0 && res.Image.H > 0 {
		ras = iconRaster{pix: res.Image.Pix, w: res.Image.W, h: res.Image.H}
	}
	iconCache.Store(key, ras)
	return ras
}

// iconScale returns the svg.Rasterize scale that renders doc's viewBox to about
// iconRasterPx on its larger side, so the raster size is bounded no matter what
// coordinate grid the icon was authored on. A doc with no readable viewBox falls
// back to the rasteriser's own default (a small icon rendered at 1:1-ish).
func iconScale(doc string) float64 {
	if longest := svgViewBoxLongestSide(doc); longest > 0 {
		return iconRasterPx / longest
	}
	return 0 // svg.Rasterize treats <=0 as its default scale
}

// svgViewBoxLongestSide reads the width/height of the first viewBox attribute in
// doc and returns the larger, or 0 when there is no readable "viewBox=\"minX minY
// w h\"". It is a light string scan, not an XML parse — enough to size the raster.
func svgViewBoxLongestSide(doc string) float64 {
	i := strings.Index(doc, "viewBox")
	if i < 0 {
		return 0
	}
	rest := doc[i+len("viewBox"):]
	q := strings.IndexAny(rest, `"'`)
	if q < 0 {
		return 0
	}
	quote := rest[q]
	rest = rest[q+1:]
	closeAt := strings.IndexByte(rest, quote)
	if closeAt < 0 {
		return 0
	}
	fields := strings.Fields(strings.ReplaceAll(rest[:closeAt], ",", " "))
	if len(fields) != 4 {
		return 0
	}
	w, err1 := strconv.ParseFloat(fields[2], 64)
	h, err2 := strconv.ParseFloat(fields[3], 64)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0
	}
	if w > h {
		return w
	}
	return h
}
