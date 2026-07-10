// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestBadgeAutoSizesWhenWZero covers the auto-size branch: Bounds()
// starts with W=0 (and H=0) and after Draw both must be non-zero and
// sized to the text plus BadgePadX/PadY on each side.
func TestBadgeAutoSizesWhenWZero(t *testing.T) {
	b := NewBadge("42")
	b.SetBounds(Rect{X: 5, Y: 5, W: 0, H: 0})
	theme := DefaultLight()
	b.Draw(newP(makeSurface(100, 40), 100), theme)
	got := b.Bounds()
	wantW := TextWidth("42") + 2*BadgePadX
	wantH := GlyphHeight() + 2*BadgePadY
	if got.W != wantW {
		t.Fatalf("auto-sized W = %d, want %d", got.W, wantW)
	}
	if got.H != wantH {
		t.Fatalf("auto-sized H = %d, want %d", got.H, wantH)
	}
}

// TestBadgeAutoSizesWZeroPreservesH covers the sub-branch where W is
// zero but H is already set: only W should be filled in, H must be
// left as the caller-supplied value.
func TestBadgeAutoSizesWZeroPreservesH(t *testing.T) {
	b := NewBadge("A")
	b.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	theme := DefaultLight()
	b.Draw(newP(makeSurface(100, 40), 100), theme)
	if b.Bounds().H != 20 {
		t.Fatalf("caller-supplied H clobbered: got H=%d, want 20", b.Bounds().H)
	}
	if b.Bounds().W == 0 {
		t.Fatal("W should have been auto-sized")
	}
}

// TestBadgePreSizedNoOverride covers the non-auto-size branch: when W
// is already set, Draw must respect the caller's Bounds verbatim.
func TestBadgePreSizedNoOverride(t *testing.T) {
	b := NewBadge("42")
	b.SetBounds(Rect{X: 5, Y: 5, W: 50, H: 20})
	theme := DefaultLight()
	b.Draw(newP(makeSurface(100, 40), 100), theme)
	got := b.Bounds()
	if got.W != 50 || got.H != 20 {
		t.Fatalf("pre-sized Bounds changed: %+v", got)
	}
}

// TestBadgeDrawFillsAccent samples an interior body pixel above the
// text-glyph row to prove the pill body is painted in Theme.Accent.
func TestBadgeDrawFillsAccent(t *testing.T) {
	theme := DefaultLight()
	b := NewBadge("9")
	b.SetBounds(Rect{X: 2, Y: 2, W: 20, H: 10})
	surf := makeSurface(40, 20)
	b.Draw(newP(surf, 40), theme)
	// (5, 2) is inside the pill body, above the text row (which starts
	// at y = 2 + (10-7)/2 = 3). Must be Accent.
	if got := pixelAt(surf, 40, 5, 2); got != theme.Accent {
		t.Fatalf("badge body at (5,2) = %+v, want Accent", got)
	}
	// The centre column of the pill also samples the body colour.
	if got := pixelAt(surf, 40, 12, 2); got != theme.Accent {
		t.Fatalf("badge centre body = %+v, want Accent", got)
	}
}

// TestBadgeCornerClippedForPill covers the "rounded corner" fill: the
// four corner pixels of the Bounds are OUTSIDE the three-fill pill and
// must therefore remain the sentinel colour.
func TestBadgeCornerClippedForPill(t *testing.T) {
	theme := DefaultLight()
	b := NewBadge("X")
	b.SetBounds(Rect{X: 0, Y: 0, W: 12, H: 8})
	surf := makeSurface(20, 12)
	b.Draw(newP(surf, 20), theme)
	// Top-left corner should NOT be Accent (pill clipped).
	if pixelAt(surf, 20, 0, 0) == theme.Accent {
		t.Fatal("top-left corner should be clipped, not Accent-filled")
	}
	// Top-right corner clipped.
	if pixelAt(surf, 20, 11, 0) == theme.Accent {
		t.Fatal("top-right corner should be clipped, not Accent-filled")
	}
}

// TestBadgeEmptyText covers the empty-Text auto-size branch: width
// collapses to 2*BadgePadX, and Draw must not panic on an empty glyph
// loop.
func TestBadgeEmptyText(t *testing.T) {
	b := NewBadge("")
	b.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	theme := DefaultLight()
	b.Draw(newP(makeSurface(40, 20), 40), theme)
	if b.Bounds().W != 2*BadgePadX {
		t.Fatalf("empty badge W = %d, want %d", b.Bounds().W, 2*BadgePadX)
	}
}
