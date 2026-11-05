// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"sort"
	"testing"

	"github.com/go-widgets/painter"
)

// fillPolygonRef is fillPolygon as it was before the scanline slice was
// reused: a fresh crossing list per row. It is the equality oracle.
func fillPolygonRef(p painter.Painter, pts [][2]int, col RGBA) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0][1], pts[0][1]
	for _, pt := range pts {
		minY, maxY = min(minY, pt[1]), max(maxY, pt[1])
	}
	for y := minY; y <= maxY; y++ {
		var xs []int
		for i := range pts {
			j := (i + 1) % len(pts)
			y0, y1 := pts[i][1], pts[j][1]
			if (y0 <= y) != (y1 <= y) {
				x0, x1 := pts[i][0], pts[j][0]
				xs = append(xs, x0+(x1-x0)*(y-y0)/(y1-y0))
			}
		}
		sort.Ints(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			fillRect(p, xs[k], y, xs[k+1]-xs[k]+1, 1, col)
		}
	}
}

// star is a concave 10-vertex polygon, 2x-sized like a Retina radar series.
var star = [][2]int{
	{200, 20}, {240, 150}, {380, 150}, {265, 230}, {310, 370},
	{200, 285}, {90, 370}, {135, 230}, {20, 150}, {160, 150},
}

func TestFillPolygonMatchesPerRowSliceVersion(t *testing.T) {
	const w, h = 400, 400
	want, got := make([]byte, 4*w*h), make([]byte, 4*w*h)
	fillPolygonRef(painter.NewPixelPainter(want, w, h), star, RGB(10, 200, 90))
	fillPolygon(painter.NewPixelPainter(got, w, h), star, RGB(10, 200, 90))
	if !bytes.Equal(want, got) {
		t.Fatal("fillPolygon differs from the per-row-slice version")
	}
	if bytes.Equal(got, make([]byte, len(got))) {
		t.Fatal("nothing was painted: the comparison proves nothing")
	}
}

// fillPolygon allocates its crossing list once per polygon, not once per
// scanline: the star spans 200+ rows, the count is a small constant.
func TestFillPolygonAllocatesOncePerPolygon(t *testing.T) {
	const w, h = 400, 400
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	got := testing.AllocsPerRun(20, func() { fillPolygon(p, star, RGB(10, 200, 90)) })
	ref := testing.AllocsPerRun(20, func() { fillPolygonRef(p, star, RGB(10, 200, 90)) })
	t.Logf("allocs per polygon: reused %v, per-row %v", got, ref)
	if got > 1 {
		t.Fatalf("fillPolygon allocates %v times per polygon, want at most 1", got)
	}
	if ref <= got {
		t.Fatalf("the per-row reference allocates %v, not more than %v: the test measures nothing", ref, got)
	}
}

func BenchmarkFillPolygon(b *testing.B) {
	const w, h = 400, 400
	p := painter.NewPixelPainter(make([]byte, 4*w*h), w, h)
	b.ReportAllocs()
	for b.Loop() {
		fillPolygon(p, star, RGB(10, 200, 90))
	}
}
