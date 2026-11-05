// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// The Diff's line gap, vertical padding and leading-glyph padding are logical
// pixels: at 2x each is exactly twice its 1x value (the glyph part of the line
// stride is the font's, and already follows the text scale).
func TestDiffSpacingFollowsMetricScale(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(1)
	gap1 := DiffLineH() - GlyphHeight()
	if gap1 != 2 {
		t.Fatalf("1x line gap %d, want 2", gap1)
	}
	SetMetricScale(2)
	if gap2 := DiffLineH() - GlyphHeight(); gap2 != 2*gap1 {
		t.Errorf("2x line gap %d, want %d", gap2, 2*gap1)
	}

	// Row 0 starts scaled(DiffPadY) below the top and its tint spans the
	// scaled stride.
	const w, h = 80, 60
	d := NewDiff([]DiffLine{{Kind: DiffAdded, Text: "a"}})
	d.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), DefaultLight())
	if pixelAt(buf, w, w-3, scaled(DiffPadY)-1) == diffAddedFill {
		t.Errorf("tint above the scaled top padding")
	}
	if pixelAt(buf, w, w-3, scaled(DiffPadY)) != diffAddedFill {
		t.Errorf("no tint at the first row (y=%d)", scaled(DiffPadY))
	}
	if pixelAt(buf, w, w-3, scaled(DiffPadY)+DiffLineH()) == diffAddedFill {
		t.Errorf("tint below the scaled row stride")
	}
}
