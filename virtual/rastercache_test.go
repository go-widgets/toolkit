// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"strconv"
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// cacheBG is a distinctive non-black backdrop, so a row's gap pixels (its border,
// a rounded corner) are visibly the background and a wrong composite shows up.
var cacheBG = toolkit.RGBA{R: 40, G: 50, B: 60, A: 255}

// gapRender draws a row that both leaves gaps (a 2px border of untouched
// background) and composites a translucent overlay — the two things a naive cache
// gets wrong.
func gapRender(p painter.Painter, _ *toolkit.Theme, r toolkit.Rect, _ int, item int) {
	p.FillRect(toolkit.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}, toolkit.RGBA{R: uint8(10 + item*30), G: 20, B: 20, A: 255})
	p.FillRect(toolkit.Rect{X: r.X, Y: r.Y, W: r.W / 2, H: r.H}, toolkit.RGBA{R: 0, G: 0, B: 200, A: 120})
}

func filledSurface(w, h int, c toolkit.RGBA) *painter.PixelPainter {
	p := painter.NewPixelPainter(make([]byte, w*h*4), w, h)
	p.FillRect(toolkit.Rect{W: w, H: h}, c)
	return p
}

func newCacheList(render func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int), n int) *VirtualList[int] {
	m := mvvm.NewObservableList[int](intItems(n)...)
	v := NewVirtualList[int](m, func(int) int { return 20 }, render)
	v.SetBounds(toolkit.Rect{W: 40, H: 100}) // 5 rows of 20px
	return v
}

// A cached draw is byte-for-byte identical to an uncached one, over the same
// backdrop — on both the first (miss) and second (hit) frame.
func TestRasterCachePixelEquivalent(t *testing.T) {
	const w, h = 40, 100

	uncached := newCacheList(gapRender, 5)
	pa := filledSurface(w, h, cacheBG)
	uncached.Draw(pa, toolkit.DefaultDark())

	cached := newCacheList(gapRender, 5)
	cached.CacheKey = func(i int, item int) string { return strconv.Itoa(item) }
	cached.CacheBackground = cacheBG

	pb := filledSurface(w, h, cacheBG)
	cached.Draw(pb, toolkit.DefaultDark()) // miss: renders + caches + blits
	for i := range pa.Buf {
		if pa.Buf[i] != pb.Buf[i] {
			t.Fatalf("miss frame differs at byte %d: uncached=%d cached=%d", i, pa.Buf[i], pb.Buf[i])
		}
	}

	pc := filledSurface(w, h, cacheBG)
	cached.Draw(pc, toolkit.DefaultDark()) // hit: blits the stored tiles
	for i := range pa.Buf {
		if pa.Buf[i] != pc.Buf[i] {
			t.Fatalf("hit frame differs at byte %d: uncached=%d cached=%d", i, pa.Buf[i], pc.Buf[i])
		}
	}
}

// A second frame with an unchanged key does NOT call Render (the tile is reused);
// a key change re-renders that row.
func TestRasterCacheHitsAndReRendersOnKeyChange(t *testing.T) {
	const w, h = 40, 100
	version := 0
	calls := 0
	render := func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i, item int) {
		calls++
		gapRender(p, th, r, i, item)
	}
	v := newCacheList(render, 5)
	v.CacheBackground = cacheBG
	v.CacheKey = func(i, item int) string { return strconv.Itoa(item) + "v" + strconv.Itoa(version) }

	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	after1 := calls
	if after1 != 5 {
		t.Fatalf("first frame rendered %d rows, want 5", after1)
	}
	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	if calls != after1 {
		t.Fatalf("second frame re-rendered %d rows, want 0 (all cache hits)", calls-after1)
	}
	version++ // every key changes -> every row re-renders
	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	if calls != after1+5 {
		t.Fatalf("after a key change rendered %d rows, want 5", calls-after1)
	}
}

// The cache is bounded to the working set: tiles for rows no longer visible are
// swept, so scrolling a long list does not grow it without bound.
func TestRasterCacheSweepsToWorkingSet(t *testing.T) {
	const w, h = 40, 100
	v := newCacheList(gapRender, 1000)
	v.CacheBackground = cacheBG
	v.CacheKey = func(i, item int) string { return strconv.Itoa(item) }

	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	if n := len(v.rowCache); n == 0 || n > 8 {
		t.Fatalf("cache holds %d tiles after one frame, want ~the visible few", n)
	}
	first := len(v.rowCache)
	v.ScrollByRows(500)
	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	if n := len(v.rowCache); n > first+1 {
		t.Fatalf("cache grew to %d tiles after scrolling; sweep did not bound it", n)
	}
}

// Without CacheKey the list renders directly every frame (no cache allocated) —
// the unchanged path for an existing consumer.
func TestRasterCacheOffByDefault(t *testing.T) {
	const w, h = 40, 100
	v := newCacheList(gapRender, 5)
	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark())
	if v.rowCache != nil {
		t.Fatalf("a list without CacheKey allocated a cache: %v", v.rowCache)
	}
}

// cardRender stands in for a real PostCard: a rounded frame, whose anti-aliased
// corners are the exact primitive the reader profile showed dominating a frame
// (FillRoundRect / cornerFillCoverage). This is where caching pays — a blit is
// far cheaper than re-rasterising the AA corners every frame.
func cardRender(p painter.Painter, _ *toolkit.Theme, r toolkit.Rect, _ int, _ int) {
	pp := p.(*painter.PixelPainter)
	pp.FillRoundRect(toolkit.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}, 12, toolkit.RGBA{R: 30, G: 30, B: 34, A: 255})
}

// benchCardList is a realistic feed: wide rows with a rounded card each.
func benchCardList() *VirtualList[int] {
	m := mvvm.NewObservableList[int](intItems(200)...)
	v := NewVirtualList[int](m, func(int) int { return 150 }, cardRender)
	v.SetBounds(toolkit.Rect{W: 1000, H: 700}) // ~5 cards visible
	return v
}

func BenchmarkVirtualListDrawUncached(b *testing.B) {
	v := benchCardList()
	p := filledSurface(1000, 700, cacheBG)
	th := toolkit.DefaultDark()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Draw(p, th)
	}
}

func BenchmarkVirtualListDrawCached(b *testing.B) {
	v := benchCardList()
	v.CacheBackground = cacheBG
	v.CacheKey = func(i, item int) string { return strconv.Itoa(item) }
	p := filledSurface(1000, 700, cacheBG)
	th := toolkit.DefaultDark()
	v.Draw(p, th) // warm the cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Draw(p, th)
	}
}
