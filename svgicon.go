// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"container/list"
	"crypto/sha256"
	"image/color"
	"strconv"
	"strings"
	"sync"

	"github.com/go-gfx/gfx/svg"
	"github.com/go-widgets/painter"
)

//go:generate go test -run ^TestBuiltinIconsetIsCurrent$ -update-iconset .

// drawBuiltinIcon paints one of the toolkit's built-in Iconoir glyphs into r,
// recoloured to ink; a name outside the built-in set draws nothing.
//
// The toolkit does not import go-icons/iconoir: that package embeds the whole
// set, which the linker cannot trim, so every toolkit program carried ~1,500
// SVGs (#588). The glyphs the widgets draw are copied into iconset_gen.go by
// internal/iconsetgen, and TestBuiltinIconsetIsCurrent fails when that copy is
// stale. A program that wants any Iconoir glyph imports
// github.com/go-widgets/toolkit/iconoir, which pays for the whole set on
// purpose, or hands its own SVG to [SVGIcon].
func drawBuiltinIcon(p painter.Painter, r Rect, name string, ink RGBA) {
	SVGIcon(builtinIcon(name))(p, r, ink)
}

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
// their own colours keeps them. A document that fails to parse, is refused by
// the rasteriser's resource limits (1 MiB, 20,000 elements, 200,000 path
// segments -- safe for server-supplied logos), or makes it panic, caches empty
// and draws nothing, so one bad icon never panics a tree. The cache of
// rasterised icons is bounded (512 icons, 32 MiB of pixels, least recently drawn
// evicted), so a program may hand SVGIcon any number of distinct documents. A painter that cannot blit
// an image draws nothing (only PixelPainter and CellPainter implement
// painter.ImagePainter — both do).
func SVGIcon(doc string) func(p painter.Painter, r Rect, ink RGBA) {
	sum := sha256.Sum256([]byte(doc))
	return func(p painter.Painter, r Rect, ink RGBA) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		ip, ok := p.(painter.ImagePainter)
		if !ok {
			return
		}
		ras := rasterizeIconSum(doc, sum, ink, iconTargetPx(r))
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

// iconKey identifies a cached raster by the SHA-256 of its source document
// (never the document itself: a key holding a server-supplied string would keep
// up to 1 MiB alive per entry), the recolour ink and the pixel size.
type iconKey struct {
	doc  [sha256.Size]byte
	ink  RGBA
	size int
}

// The icon cache is bounded, because the documents may come from a server and
// a stream of distinct ones must not grow it without limit: at most
// maxIconCacheEntries rasters and maxIconCacheBytes of pixels, the least
// recently drawn evicted first. An evicted icon is simply rasterised again.
const (
	maxIconCacheEntries = 512
	maxIconCacheBytes   = 32 << 20
)

// iconLRU is a mutex-guarded LRU of rasterised icons, bounded in entries and in
// pixel bytes. It is safe for concurrent use (a native host may draw off the
// main goroutine); the wasm host is single-threaded and pays nothing.
type iconLRU struct {
	mu         sync.Mutex
	maxEntries int
	maxBytes   int
	bytes      int
	order      *list.List // front = most recently used; values are *iconEntry
	byKey      map[iconKey]*list.Element
}

type iconEntry struct {
	key iconKey
	ras iconRaster
}

func newIconLRU(maxEntries, maxBytes int) *iconLRU {
	return &iconLRU{maxEntries: maxEntries, maxBytes: maxBytes, order: list.New(), byKey: map[iconKey]*list.Element{}}
}

func (c *iconLRU) get(k iconKey) (iconRaster, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byKey[k]
	if !ok {
		return iconRaster{}, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*iconEntry).ras, true
}

func (c *iconLRU) put(k iconKey, ras iconRaster) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.byKey[k]; ok { // two goroutines rasterised the same icon
		c.order.MoveToFront(e)
		return
	}
	c.byKey[k] = c.order.PushFront(&iconEntry{key: k, ras: ras})
	c.bytes += len(ras.pix)
	for c.order.Len() > c.maxEntries || c.bytes > c.maxBytes {
		last := c.order.Back() // never nil: the loop runs only while over a bound
		ev := c.order.Remove(last).(*iconEntry)
		delete(c.byKey, ev.key)
		c.bytes -= len(ev.ras.pix)
	}
}

// iconCache memoises rasterised icons so the SVG is parsed once per (doc, ink,
// size), not once per frame.
var iconCache = newIconLRU(maxIconCacheEntries, maxIconCacheBytes)

// rasterizeIcon renders doc so its viewBox's longer side is size pixels,
// recoloured to ink, with a transparent background, and caches the result
// (empty on any parse error).
func rasterizeIcon(doc string, ink RGBA, size int) iconRaster {
	return rasterizeIconSum(doc, sha256.Sum256([]byte(doc)), ink, size)
}

// rasterizeIconSum is rasterizeIcon with the document's digest already taken,
// so a drawer hashes its document once rather than once per frame.
func rasterizeIconSum(doc string, sum [sha256.Size]byte, ink RGBA, size int) iconRaster {
	key := iconKey{doc: sum, ink: ink, size: size}
	if ras, ok := iconCache.get(key); ok {
		return ras
	}
	ras := safeRasterize(doc, ink, size)
	iconCache.put(key, ras)
	return ras
}

// Icon documents may come from a server (a logo), so the rasteriser is held to
// icon-sized ceilings: a document past one is refused with a *svg.LimitError
// and the icon draws nothing. These are far below the svg package's defaults.
const (
	iconMaxBytes    = 1 << 20
	iconMaxElements = 20_000
	iconMaxSegments = 200_000
	iconMaxPixels   = 2 * maxIconPx * maxIconPx
)

// rasterizeSVG is svg.Rasterize; a variable so a test can make it panic.
var rasterizeSVG = svg.Rasterize

// safeRasterize renders one icon. Any failure -- a refused document, a parse
// error or a panic inside the rasteriser -- yields the empty raster, so an
// icon that fails draws nothing and never takes the program down.
func safeRasterize(doc string, ink RGBA, size int) (ras iconRaster) {
	defer func() {
		if recover() != nil {
			ras = iconRaster{}
		}
	}()
	res, err := rasterizeSVG(doc, svg.Options{
		Scale:       iconScale(doc, float64(size)),
		Ink:         color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: ink.A},
		MaxBytes:    iconMaxBytes,
		MaxElements: iconMaxElements,
		MaxSegments: iconMaxSegments,
		MaxPixels:   iconMaxPixels,
	})
	if err == nil && res != nil && res.Image != nil && res.Image.W > 0 && res.Image.H > 0 {
		ras = iconRaster{pix: res.Image.Pix, w: res.Image.W, h: res.Image.H}
	}
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
