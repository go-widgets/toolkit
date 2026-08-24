// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"strconv"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// cachedRow is one row's rasterised tile: the RGBA pixels and their size.
type cachedRow struct {
	buf  []byte
	w, h int
}

// drawRow paints row i at rect r. With CacheKey set and a painter that can blit
// an image, it draws through the raster cache: a miss renders the row once into
// an offscreen tile (filled with CacheBackground first, so gaps and translucency
// composite over the on-screen backdrop) and stores it; a hit blits the stored
// tile, skipping Render entirely. Without caching — CacheKey nil, a non-image
// painter, or an empty rect — it renders directly, unchanged. sweepCache evicts
// tiles not touched in the current frame.
func (v *VirtualList[T]) drawRow(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T) {
	ip, canBlit := p.(painter.ImagePainter)
	if v.CacheKey == nil || !canBlit || r.W <= 0 || r.H <= 0 {
		v.Render(p, th, r, i, item)
		return
	}
	key := v.CacheKey(i, item) + "|" + strconv.Itoa(r.W) + "x" + strconv.Itoa(r.H)
	cr, ok := v.rowCache[key]
	if !ok {
		buf := make([]byte, r.W*r.H*4)
		op := painter.NewPixelPainter(buf, r.W, r.H)
		tile := toolkit.Rect{W: r.W, H: r.H}
		op.FillRect(tile, v.CacheBackground)
		v.Render(op, th, tile, i, item)
		cr = cachedRow{buf: buf, w: r.W, h: r.H}
		if v.rowCache == nil {
			v.rowCache = make(map[string]cachedRow)
		}
		v.rowCache[key] = cr
	}
	ip.DrawImage(r, cr.buf, cr.w, cr.h)
	if v.cacheHit == nil {
		v.cacheHit = make(map[string]bool)
	}
	v.cacheHit[key] = true
}

// sweepCache drops tiles not drawn this frame, bounding the cache to the working
// set (the viewport, plus whatever briefly scrolled through it since the last
// sweep). It is a no-op when caching is off.
func (v *VirtualList[T]) sweepCache() {
	if v.CacheKey == nil || len(v.rowCache) == 0 {
		return
	}
	for k := range v.rowCache {
		if !v.cacheHit[k] {
			delete(v.rowCache, k)
		}
	}
	for k := range v.cacheHit {
		delete(v.cacheHit, k)
	}
}
