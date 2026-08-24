// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "bytes"

// DiffRects returns the rectangles covering every pixel that differs between cur
// and prev — two w×h RGBA buffers (4 bytes per pixel, row-major) — coalesced into
// one rectangle per contiguous band of changed rows, each rectangle's x-span the
// union of the changed columns in its band.
//
// It is the companion of [Surface.Damage]: an application that keeps its previous
// frame reports incremental damage with
//
//	surf.Damage = func() []Rect { return DiffRects(cur, prev, w, h) }
//
// instead of enumerating which of its widgets animate. The diff is exact by
// construction — every changed pixel is inside a returned rectangle — so a host
// can never miss an update, and it self-scales: a small in-place animation (a
// spinner, a caret, a progress bar) yields a tight box, a large change yields the
// change. The cost is one scan of the buffers, so a caller that already knows a
// frame changed little is where it pays off; a caller expecting a near-full change
// is better served by whole-surface present (return nil).
//
// It returns nil when a buffer is unusable — a non-positive size, or one shorter
// than w*h*4 — and when the two buffers are identical; a host reads a nil (or
// empty) result as whole-surface damage. Coordinates are in the buffers' own
// pixel space.
func DiffRects(cur, prev []byte, w, h int) []Rect {
	stride := w * 4
	if w <= 0 || h <= 0 || len(cur) < stride*h || len(prev) < stride*h {
		return nil
	}
	var out []Rect
	bandOpen := false
	var bandY0, bandMinX, bandMaxX int
	flush := func(yEnd int) {
		if bandOpen {
			out = append(out, Rect{X: bandMinX, Y: bandY0, W: bandMaxX - bandMinX, H: yEnd - bandY0})
			bandOpen = false
		}
	}
	for y := 0; y < h; y++ {
		rc := cur[y*stride : y*stride+stride]
		rp := prev[y*stride : y*stride+stride]
		if bytes.Equal(rc, rp) {
			flush(y)
			continue
		}
		minX, maxX := 0, 0
		found := false
		for x := 0; x < w; x++ {
			o := x * 4
			if rc[o] != rp[o] || rc[o+1] != rp[o+1] || rc[o+2] != rp[o+2] || rc[o+3] != rp[o+3] {
				if !found {
					minX = x
					found = true
				}
				maxX = x
			}
		}
		if !bandOpen {
			bandOpen, bandY0, bandMinX, bandMaxX = true, y, minX, maxX+1
			continue
		}
		if minX < bandMinX {
			bandMinX = minX
		}
		if maxX+1 > bandMaxX {
			bandMaxX = maxX + 1
		}
	}
	flush(h)
	return out
}
