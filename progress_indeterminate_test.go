// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestIndetSpan covers the sliding-chunk geometry: mid-track visible, the
// off-near-edge (length 0) case, the far-edge clamp, and the chunk-width floor.
func TestIndetSpan(t *testing.T) {
	// Mid phase: chunk sits inside the track, no clamps.
	if off, ln := indetSpan(100, 0.5); off <= 0 || ln <= 0 || off+ln > 100 {
		t.Fatalf("mid: off=%d ln=%d", off, ln)
	}
	// Phase 0: chunk is entirely off the near edge → nothing visible.
	if off, ln := indetSpan(100, 0.0); off != 0 || ln != 0 {
		t.Fatalf("near-edge: off=%d ln=%d, want 0,0", off, ln)
	}
	// Late phase: chunk runs past the far edge → clamped to the track end.
	if off, ln := indetSpan(100, 0.9); off+ln != 100 || ln <= 0 {
		t.Fatalf("far clamp: off=%d ln=%d, want end at 100", off, ln)
	}
	// Tiny track: the 30% chunk rounds to 0 and is floored to 1.
	if _, ln := indetSpan(2, 0.5); ln < 1 {
		t.Fatalf("tiny track ln=%d, want >=1", ln)
	}
	// Phase wraps modulo 1 (>=1 behaves like its fractional part).
	a1, b1 := indetSpan(100, 1.5)
	a2, b2 := indetSpan(100, 0.5)
	if a1 != a2 || b1 != b2 {
		t.Fatalf("phase wrap: (%d,%d) != (%d,%d)", a1, b1, a2, b2)
	}
}

func TestProgressBarIndeterminateHorizontal(t *testing.T) {
	const w, h = 80, 20
	theme := DefaultLight()
	pb := &ProgressBar{Indeterminate: true, Phase: 0.5, Label: "Loading…"}
	pb.SetBounds(Rect{X: 0, Y: 0, W: 76, H: 16})
	buf := makeSurface(w, h)
	pb.Draw(newP(buf, w), theme)
	// The moving chunk (~mid-track at phase 0.5) paints Accent somewhere.
	accent := false
	for x := 0; x < 76; x++ {
		if pixelAt(buf, w, x, 8) == theme.Accent {
			accent = true
			break
		}
	}
	if !accent {
		t.Fatal("indeterminate bar painted no Accent chunk at phase 0.5")
	}
	// Phase 0 → chunk off the near edge → no Accent (covers the ln==0 skip).
	pb.Phase = 0
	buf0 := makeSurface(w, h)
	pb.Draw(newP(buf0, w), theme)
	for x := 0; x < 76; x++ {
		if pixelAt(buf0, w, x, 8) == theme.Accent {
			t.Fatal("phase 0 should paint no chunk")
		}
	}
}

func TestProgressBarIndeterminateVertical(t *testing.T) {
	const w, h = 20, 80
	theme := DefaultLight()
	pb := &ProgressBar{Indeterminate: true, Phase: 0.5, Orientation: Vertical}
	pb.SetBounds(Rect{X: 0, Y: 0, W: 16, H: 76})
	buf := makeSurface(w, h)
	pb.Draw(newP(buf, w), theme)
	accent := false
	for y := 0; y < 76; y++ {
		if pixelAt(buf, w, 8, y) == theme.Accent {
			accent = true
			break
		}
	}
	if !accent {
		t.Fatal("vertical indeterminate bar painted no Accent chunk")
	}
}
