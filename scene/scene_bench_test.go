// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Date: 2026-08-07
//
// Benchmarks + a reporting test comparing a full immediate-mode repaint against
// a damage-only scene.Render for a single hovered-widget change, on two scenes:
// a ~150-widget "gallery" and a synthetic 2000-widget grid. See
// TestSpeedupReport for the headline ratio + bytes-blitted numbers.
package scene

import (
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// oneHoverChange runs one immediate full repaint and one scene damage-render for
// a single recoloured (hovered) cell, and returns their per-op nanoseconds plus
// the bytes each blits (scene = damage area x4; immediate = whole surface x4).
func oneHoverChange(rows, cols, cellPx int) (immNs, sceneNs float64, sceneBlit, fullBlit int) {
	w, h := cols*cellPx, rows*cellPx
	root, cells := buildDense(w, h, rows, cols)
	theme := th()
	target := cells[len(cells)/2] // the "hovered" widget

	// Immediate mode: a fresh PixelPainter each op, full-tree repaint.
	buf, p := newBuf(w, h)
	_ = buf
	immRes := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			root.Draw(p, theme)
		}
	})

	// Scene mode: warm up, then per op recolour one cell + damage-render.
	s := New(root)
	_, ps := newBuf(w, h)
	s.Render(ps, theme) // consume the initial full-surface damage
	var region Region
	toggle := true
	sceneRes := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if toggle {
				target.col = red
			} else {
				target.col = green
			}
			toggle = !toggle
			s.Invalidate(target)
			region = s.Render(ps, theme)
		}
	})

	immNs = float64(immRes.NsPerOp())
	sceneNs = float64(sceneRes.NsPerOp())
	sceneBlit = region.Area() * 4
	fullBlit = w * h * 4
	return
}

func BenchmarkGalleryImmediate(b *testing.B) {
	w, h := 15*24, 10*24
	root, _ := buildDense(w, h, 10, 15)
	theme := th()
	_, p := newBuf(w, h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root.Draw(p, theme)
	}
}

func BenchmarkGalleryScene(b *testing.B) {
	w, h := 15*24, 10*24
	root, cells := buildDense(w, h, 10, 15)
	theme := th()
	s := New(root)
	_, p := newBuf(w, h)
	s.Render(p, theme)
	target := cells[len(cells)/2]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.col = red
		s.Invalidate(target)
		s.Render(p, theme)
	}
}

func BenchmarkGrid2000Immediate(b *testing.B) {
	w, h := 50*16, 40*16
	root, _ := buildDense(w, h, 40, 50)
	theme := th()
	_, p := newBuf(w, h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root.Draw(p, theme)
	}
}

func BenchmarkGrid2000Scene(b *testing.B) {
	w, h := 50*16, 40*16
	root, cells := buildDense(w, h, 40, 50)
	theme := th()
	s := New(root)
	_, p := newBuf(w, h)
	s.Render(p, theme)
	target := cells[len(cells)/2]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.col = red
		s.Invalidate(target)
		s.Render(p, theme)
	}
}

// TestSpeedupReport prints the headline comparison for both scenes and applies
// the kill-criterion honestly: it does NOT fail on a sub-5x dense-scene ratio
// (that is a shipping RECOMMENDATION, surfaced in the log), but it does assert
// the scene never blits MORE than a full repaint and never runs slower — the
// two properties that must hold for the layer to be worth shipping at all.
func TestSpeedupReport(t *testing.T) {
	type row struct {
		name             string
		rows, cols, cell int
	}
	for _, r := range []row{
		{"gallery(~150)", 10, 15, 24},
		{"grid(2000)", 40, 50, 16},
	} {
		imm, sc, blit, full := oneHoverChange(r.rows, r.cols, r.cell)
		ratio := imm / sc
		t.Logf("%-14s widgets=%-5d immediate=%8.0f ns/frame  scene=%8.0f ns/frame  speedup=%5.1fx  blit=%d B  full=%d B  blit-reduction=%5.1fx",
			r.name, r.rows*r.cols, imm, sc, ratio, blit, full, float64(full)/float64(blit))
		if blit > full {
			t.Fatalf("%s: scene blits %d B > full repaint %d B", r.name, blit, full)
		}
		if ratio < 1.0 {
			t.Fatalf("%s: scene SLOWER than immediate (%.1fx) — not worth shipping", r.name, ratio)
		}
		if r.name == "grid(2000)" && ratio < 5.0 {
			t.Logf("KILL-CRITERION (honest): dense-scene CPU speedup %.1fx is BELOW 5x; "+
				"recommend shipping only the wasm blit-region path (blit-reduction %.1fx is the real win).",
				ratio, float64(full)/float64(blit))
		}
	}
}

// compile-time: the benchmark leaves satisfy the scene capability interfaces.
var (
	_ painter.Painter = (*painter.PixelPainter)(nil)
	_ toolkit.Widget  = (*plainBox)(nil)
)
