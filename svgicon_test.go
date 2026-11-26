// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/svg"
	"github.com/go-widgets/painter"
)

const redSquareSVG = `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10" fill="#ff0000"/></svg>`

// TestDrawBuiltinIcon: a glyph of the built-in set paints ink; a name outside
// it — a real Iconoir icon the toolkit never draws, and a non-icon — draws
// nothing rather than panicking.
func TestDrawBuiltinIcon(t *testing.T) {
	const sz = 24
	ink := func(name string) bool {
		buf := make([]byte, sz*sz*4)
		drawBuiltinIcon(painter.NewPixelPainter(buf, sz, sz), Rect{W: sz, H: sz}, name, RGB(0x20, 0x20, 0x20))
		for _, b := range buf {
			if b != 0 {
				return true
			}
		}
		return false
	}
	if !ink("xmark") {
		t.Error(`drawBuiltinIcon("xmark") painted no ink`)
	}
	for _, name := range []string{"airplane", "not-a-real-iconoir-stem", ""} {
		if ink(name) {
			t.Errorf("drawBuiltinIcon(%q) painted something; it is not in the built-in set", name)
		}
	}
}

// TestSVGIconRenders rasterises a solid-red SVG and blits it into a box; the box
// must carry red. A second draw exercises the cache path.
func TestSVGIconRenders(t *testing.T) {
	const w, h = 24, 24
	draw := SVGIcon(redSquareSVG)
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	box := Rect{X: 4, Y: 4, W: 16, H: 16}
	draw(p, box, RGBA{R: 0, G: 0, B: 0, A: 255})

	// Centre of the box should be predominantly red.
	i := ((box.Y+box.H/2)*w + (box.X + box.W/2)) * 4
	if buf[i] < 180 || buf[i+1] > 80 || buf[i+2] > 80 {
		t.Fatalf("box centre = (%d,%d,%d,%d), want red", buf[i], buf[i+1], buf[i+2], buf[i+3])
	}

	// Second draw hits the cache (same result, no re-parse).
	buf2 := make([]byte, w*h*4)
	draw(painter.NewPixelPainter(buf2, w, h), box, RGBA{A: 255})
	j := ((box.Y+box.H/2)*w + (box.X + box.W/2)) * 4
	if buf2[j] < 180 {
		t.Fatalf("cached draw did not paint red: %d", buf2[j])
	}
}

// TestSVGIconBadDocDrawsNothing: an unparseable document caches empty and paints
// nothing, rather than panicking.
func TestSVGIconBadDocDrawsNothing(t *testing.T) {
	const w, h = 16, 16
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	SVGIcon("this is not an svg")(p, Rect{X: 0, Y: 0, W: w, H: h}, RGBA{A: 255})
	for _, b := range buf {
		if b != 0 {
			t.Fatal("a bad SVG should paint nothing")
		}
	}
}

// TestSVGIconGuards covers the early-out branches: a zero-area box and a painter
// that cannot blit an image both draw nothing.
func TestSVGIconGuards(t *testing.T) {
	draw := SVGIcon(redSquareSVG)
	// Zero-area box: returns before rasterising.
	draw(painter.NewPixelPainter(make([]byte, 4), 1, 1), Rect{X: 0, Y: 0, W: 0, H: 10}, RGBA{A: 255})

	// A painter that is not an ImagePainter (nonClipPainter has no DrawImage).
	const w, h = 16, 16
	buf := make([]byte, w*h*4)
	np := &nonClipPainter{inner: painter.NewPixelPainter(buf, w, h)}
	draw(np, Rect{X: 0, Y: 0, W: w, H: h}, RGBA{A: 255})
	for _, b := range buf {
		if b != 0 {
			t.Fatal("a non-ImagePainter should paint nothing")
		}
	}
}

// TestSVGViewBoxLongestSide covers the viewBox scan: a valid box returns its
// larger side; the many malformed/absent forms return 0.
func TestSVGViewBoxLongestSide(t *testing.T) {
	for _, tc := range []struct {
		doc  string
		want float64
	}{
		{`<svg viewBox="0 0 24 32">`, 32},
		{`<svg viewBox='0 -960 960 940'>`, 960},
		{`<svg viewBox="0,0,10,50">`, 50}, // comma-separated
		{`<svg width="10">`, 0},           // no viewBox
		{`<svg viewBox=`, 0},              // no quote
		{`<svg viewBox="0 0 24 32`, 0},    // unclosed quote
		{`<svg viewBox="0 0 24">`, 0},     // wrong field count
		{`<svg viewBox="0 0 x 32">`, 0},   // non-numeric
		{`<svg viewBox="0 0 0 32">`, 0},   // non-positive width
	} {
		if got := svgViewBoxLongestSide(tc.doc); got != tc.want {
			t.Errorf("svgViewBoxLongestSide(%q) = %g, want %g", tc.doc, got, tc.want)
		}
	}
}

// TestIconRastersAtTheBoxSize: an icon rasterises at the size of the box it is
// drawn in (whatever its viewBox grid), bounded, and is blitted 1:1 — the
// nearest-neighbour squeeze of a 64-px raster into a small box is what made
// small icons jagged.
func TestIconRastersAtTheBoxSize(t *testing.T) {
	if s := iconScale(`<svg viewBox="0 0 960 960"><rect/></svg>`, 16); s != 16.0/960 {
		t.Errorf("scale = %g, want 16/960", s)
	}
	if s := iconScale(`<svg><rect/></svg>`, 16); s != 0 {
		t.Errorf("no-viewBox scale = %g, want 0 (rasteriser default)", s)
	}
	res := rasterizeIcon(`<svg viewBox="0 0 960 960"><rect width="960" height="960" fill="#000"/></svg>`, RGBA{A: 255}, 16)
	if res.w != 16 || res.h != 16 {
		t.Errorf("raster = %dx%d, want 16x16", res.w, res.h)
	}
	if iconTargetPx(Rect{W: 14, H: 20}) != 20 || iconTargetPx(Rect{W: 5000, H: 9}) != maxIconPx {
		t.Error("iconTargetPx")
	}
	// An Iconoir stroke glyph in a 14-px box keeps grey (anti-aliased) edge
	// pixels: more than two distinct alpha values.
	const sz = 14
	buf := make([]byte, sz*sz*4)
	drawBuiltinIcon(painter.NewPixelPainter(buf, sz, sz), Rect{W: sz, H: sz}, "folder", RGB(0, 0, 0))
	alphas := map[byte]bool{}
	for i := 3; i < len(buf); i += 4 {
		alphas[buf[i]] = true
	}
	if len(alphas) < 4 {
		t.Errorf("a 14-px icon has %d alpha levels, want anti-aliased edges", len(alphas))
	}
	// No viewBox: the rasteriser's default size, scaled into the box.
	big := make([]byte, 8*8*4)
	SVGIcon(`<svg width="40" height="40"><rect width="40" height="40" fill="#ff0000"/></svg>`)(painter.NewPixelPainter(big, 8, 8), Rect{W: 8, H: 8}, RGBA{A: 255})
	if big[(4*8+4)*4] < 180 {
		t.Error("a viewBox-less icon was not scaled into its box")
	}
}

// blank reports whether the 24x24 box drawn by draw stayed fully transparent.
func blank(draw func(painter.Painter, Rect, RGBA)) bool {
	buf := make([]byte, 24*24*4)
	draw(painter.NewPixelPainter(buf, 24, 24), Rect{W: 24, H: 24}, RGBA{A: 255})
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}

// TestSVGIconRefusedDocumentDrawsNothing: a document past the icon ceilings is
// refused by the rasteriser and draws nothing; the same shape within them draws.
func TestSVGIconRefusedDocumentDrawsNothing(t *testing.T) {
	var many strings.Builder
	many.WriteString(`<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg">`)
	for range iconMaxElements + 1 {
		many.WriteString(`<g/>`)
	}
	many.WriteString(`</svg>`)
	if !blank(SVGIcon(many.String())) {
		t.Error("a document past the element ceiling drew something")
	}
	deep := strings.Repeat("<g>", 100_000) + strings.Repeat("</g>", 100_000)
	if !blank(SVGIcon(`<svg viewBox="0 0 10 10">` + deep + `</svg>`)) {
		t.Error("a 100000-deep document drew something")
	}
	if blank(SVGIcon(redSquareSVG)) {
		t.Error("a small document drew nothing")
	}
}

// TestSVGIconPanicDrawsNothing: a panic inside the rasteriser is contained.
func TestSVGIconPanicDrawsNothing(t *testing.T) {
	old := rasterizeSVG
	defer func() { rasterizeSVG = old }()
	rasterizeSVG = func(string, svg.Options) (*svg.Result, error) { panic("boom") }
	if !blank(SVGIcon(`<svg viewBox="0 0 10 10"><rect width="9" height="9" fill="#123456"/></svg>`)) {
		t.Error("an icon whose rasteriser panicked drew something")
	}
}

// TestIconCacheStaysBoundedUnderADistinctStream: a stream of distinct documents
// (a server handing out logos) cannot grow the cache past its two ceilings.
func TestIconCacheStaysBoundedUnderADistinctStream(t *testing.T) {
	for i := range 3 * maxIconCacheEntries {
		doc := fmt.Sprintf(`<svg viewBox="0 0 10 10"><rect width="%d" height="10" fill="#123456"/><!-- %d --></svg>`, 1+i%10, i)
		if rasterizeIcon(doc, RGBA{A: 255}, 64).pix == nil {
			t.Fatalf("doc %d did not rasterise", i)
		}
	}
	iconCache.mu.Lock()
	n, b := iconCache.order.Len(), iconCache.bytes
	iconCache.mu.Unlock()
	if n > maxIconCacheEntries || b > maxIconCacheBytes {
		t.Errorf("cache holds %d entries / %d bytes, ceilings %d / %d", n, b, maxIconCacheEntries, maxIconCacheBytes)
	}
	if n != maxIconCacheEntries {
		t.Errorf("cache holds %d entries after 3x the cap, want exactly the cap", n)
	}
}

// TestIconLRUEvictsLeastRecentlyUsedAndByBytes covers both ceilings and the
// recency order.
func TestIconLRUEvictsLeastRecentlyUsedAndByBytes(t *testing.T) {
	key := func(n int) iconKey { return iconKey{size: n} }
	ras := func(n int) iconRaster { return iconRaster{pix: make([]byte, n), w: 1, h: 1} }

	c := newIconLRU(2, 1<<20)
	c.put(key(1), ras(1))
	c.put(key(2), ras(1))
	if _, ok := c.get(key(1)); !ok { // 1 becomes the most recent
		t.Fatal("entry 1 missing")
	}
	c.put(key(3), ras(1)) // evicts 2, the least recent
	if _, ok := c.get(key(2)); ok {
		t.Error("the least recently used entry survived")
	}
	for _, k := range []int{1, 3} {
		if _, ok := c.get(key(k)); !ok {
			t.Errorf("entry %d was evicted", k)
		}
	}
	c.put(key(3), ras(1)) // a duplicate put changes nothing
	if c.order.Len() != 2 || c.bytes != 2 {
		t.Errorf("duplicate put: %d entries, %d bytes", c.order.Len(), c.bytes)
	}

	b := newIconLRU(100, 10)
	b.put(key(1), ras(6))
	b.put(key(2), ras(6)) // 12 > 10: the older goes
	if _, ok := b.get(key(1)); ok || b.bytes != 6 {
		t.Errorf("byte ceiling: entry 1 kept=%v, bytes=%d", ok, b.bytes)
	}
}
