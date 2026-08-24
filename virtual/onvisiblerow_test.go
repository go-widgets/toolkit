// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"strconv"
	"testing"

	"github.com/go-widgets/toolkit"
)

// OnVisibleRow fires once per visible row every frame, with the row's on-screen
// rect — INCLUDING frames whose paint is served from the raster cache, which is
// the whole point: per-row side effects (a11y run collection, seen-reporting)
// must not be skipped just because the pixels were cached.
func TestOnVisibleRowFiresEveryFrameIncludingCacheHits(t *testing.T) {
	const w, h = 40, 100 // 5 rows of 20px
	v := newCacheList(gapRender, 1000)
	v.CacheBackground = cacheBG
	v.CacheKey = func(i, item int) string { return strconv.Itoa(item) }

	type visit struct {
		i int
		r toolkit.Rect
	}
	var visits []visit
	v.OnVisibleRow = func(i int, r toolkit.Rect, item int) {
		visits = append(visits, visit{i, r})
	}

	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark()) // miss frame
	first := len(visits)
	if first != 5 {
		t.Fatalf("first frame visited %d rows, want 5", first)
	}
	if visits[0].i != 0 || visits[0].r.Y != 0 || visits[0].r.H != 20 || visits[0].r.W != 40 {
		t.Fatalf("row 0 visit = %+v, want the top 40x20 rect", visits[0])
	}

	visits = visits[:0]
	v.Draw(filledSurface(w, h, cacheBG), toolkit.DefaultDark()) // cached (hit) frame
	if len(visits) != 5 {
		t.Fatalf("cache-hit frame visited %d rows, want 5 (side effects must not be skipped)", len(visits))
	}
}

// A nil OnVisibleRow is simply not called (the unchanged path).
func TestOnVisibleRowNilIsNoop(t *testing.T) {
	v := newCacheList(gapRender, 5)
	// No OnVisibleRow, no CacheKey: must draw without panicking.
	v.Draw(filledSurface(40, 100, cacheBG), toolkit.DefaultDark())
}
