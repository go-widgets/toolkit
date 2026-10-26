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
	"github.com/go-icons/iconoir"
	"github.com/go-widgets/painter"
)

// DrawIconoir paints the named Iconoir glyph into r, recoloured to ink: the SVG
// comes from go-icons/iconoir (data) and [SVGIcon] renders it over go-gfx (code) —
// icons in one place, rasterising in another. An unknown name draws nothing.
//
// It is the toolkit's single seam for an Iconoir glyph, exported so a consumer
// draws one THROUGH the toolkit (its own controls do the same) rather than
// depending on an icon-pack renderer directly. Pair it with [IconoirNames] to
// enumerate the available stems.
func DrawIconoir(p painter.Painter, r Rect, name string, ink RGBA) {
	SVGIcon(iconoir.Icon(name))(p, r, ink)
}

// IconoirNames is the sorted list of available Iconoir glyph stems (the names
// [DrawIconoir] accepts), from go-icons/iconoir.
func IconoirNames() []string { return iconoir.Names() }

// SVGIcon returns an icon drawer that renders the SVG document doc into the box
// it is handed — for a [TreeTableNode.Icon], or anywhere a
// func(painter.Painter, Rect, RGBA) drawer is wanted. The SVG is rasterised
// ONCE per (document, ink, box size) at the box's own size — anti-aliased by
// the rasteriser at the pixels it will be shown at, so a HiDPI host (a larger
// box under a larger MetricScale) gets a sharper glyph, not an enlarged one —
// and cached; each draw blits the cached bitmap 1:1, so a file tree can show a
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
		ras := rasterizeIcon(doc, ink, iconTargetPx(r))
		if ras.pix == nil {
			return
		}
		// The raster is the box's own size (when the SVG has a viewBox), so the
		// blit is 1:1: DrawImage samples nearest-neighbour, and squeezing a
		// 64-px raster into a 14-px box dropped most of each stroke -- the
		// jagged icons a HiDPI-aware host still showed.
		dst := Rect{X: r.X + (r.W-ras.w)/2, Y: r.Y + (r.H-ras.h)/2, W: ras.w, H: ras.h}
		if ras.w > r.W || ras.h > r.H {
			dst = r // no viewBox: the rasteriser's default size, scaled to fit
		}
		ip.DrawImage(dst, ras.pix, ras.w, ras.h)
	}
}

// maxIconPx bounds the raster an icon box may ask for (a 512-px icon is
// already a page-sized glyph).
const maxIconPx = 512

// iconTargetPx is the raster size for box r: its longer side, bounded.
func iconTargetPx(r Rect) int { return min(max(r.W, r.H), maxIconPx) }

// iconRaster is a cached rasterised icon: straight-alpha RGBA, w*h pixels.
type iconRaster struct {
	pix  []byte
	w, h int
}

// iconKey identifies a cached raster by its source document, recolour ink and
// pixel size.
type iconKey struct {
	doc  string
	ink  RGBA
	size int
}

// iconCache memoises rasterised icons so the SVG is parsed once per (doc, ink,
// size), not once per frame. It is safe for concurrent use (a native host may
// draw off the main goroutine); the wasm host is single-threaded and pays
// nothing.
var iconCache sync.Map // iconKey -> iconRaster

// rasterizeIcon renders doc so its viewBox's longer side is size pixels,
// recoloured to ink, with a transparent background, and caches the result
// (empty on any parse error).
func rasterizeIcon(doc string, ink RGBA, size int) iconRaster {
	key := iconKey{doc: doc, ink: ink, size: size}
	if v, ok := iconCache.Load(key); ok {
		return v.(iconRaster)
	}
	var ras iconRaster
	res, err := svg.Rasterize(doc, svg.Options{
		Scale: iconScale(doc, float64(size)),
		Ink:   color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A},
	})
	if err == nil && res != nil && res.Image != nil && res.Image.W > 0 && res.Image.H > 0 {
		ras = iconRaster{pix: res.Image.Pix, w: res.Image.W, h: res.Image.H}
	}
	iconCache.Store(key, ras)
	return ras
}

// iconScale returns the svg.Rasterize scale that renders doc's viewBox to px
// pixels on its larger side, whatever coordinate grid the icon was authored
// on. A doc with no readable viewBox falls back to the rasteriser's own
// default (a small icon rendered at 1:1-ish).
func iconScale(doc string, px float64) float64 {
	if longest := svgViewBoxLongestSide(doc); longest > 0 {
		return px / longest
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
