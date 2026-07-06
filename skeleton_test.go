// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestSkeletonNewTextDefaultLines covers the default-lines branch of
// NewSkeleton: SkeletonText + non-positive lines -> 3.
func TestSkeletonNewTextDefaultLines(t *testing.T) {
	s := NewSkeleton(SkeletonText, 0)
	if s.Lines != 3 {
		t.Fatalf("default Lines = %d, want 3", s.Lines)
	}
	s = NewSkeleton(SkeletonText, -5)
	if s.Lines != 3 {
		t.Fatalf("negative Lines = %d, want 3", s.Lines)
	}
}

// TestSkeletonNewTextKeepsPositiveLines covers the "already-positive"
// half of the branch.
func TestSkeletonNewTextKeepsPositiveLines(t *testing.T) {
	s := NewSkeleton(SkeletonText, 5)
	if s.Lines != 5 {
		t.Fatalf("Lines = %d, want 5", s.Lines)
	}
}

// TestSkeletonNewNonTextIgnoresLines covers the "kind != SkeletonText"
// branch: the lines value is stored verbatim even when non-positive.
func TestSkeletonNewNonTextIgnoresLines(t *testing.T) {
	s := NewSkeleton(SkeletonAvatar, -1)
	if s.Lines != -1 {
		t.Fatalf("Avatar Lines = %d, want -1 (stored verbatim)", s.Lines)
	}
	s = NewSkeleton(SkeletonBlock, 0)
	if s.Lines != 0 {
		t.Fatalf("Block Lines = %d, want 0", s.Lines)
	}
}

// TestSkeletonDrawText verifies SkeletonText draws N bars in SurfaceAlt
// with the last one narrower.
func TestSkeletonDrawText(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonText, 3)
	const w = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: 80})
	surf := makeSurface(w, 80)
	s.Draw(newP(surf, w), theme)

	// First bar covers full width. Sample at x = w-1, y in bar 0.
	if got := pixelAt(surf, w, w-1, 3); got != theme.SurfaceAlt {
		t.Fatalf("bar 0 right edge = %+v, want SurfaceAlt", got)
	}
	// Last bar is 60% width -> a pixel at x = w-1 in the last bar's
	// row must NOT be SurfaceAlt (it's outside the shortened bar).
	lastY := 2*(SkeletonLineH+SkeletonLineGap) + SkeletonLineH/2
	if got := pixelAt(surf, w, w-1, lastY); got == theme.SurfaceAlt {
		t.Fatal("last bar should be 60% width -- right edge must be untouched")
	}
	// But its left side should still be filled.
	if got := pixelAt(surf, w, 2, lastY); got != theme.SurfaceAlt {
		t.Fatalf("last bar left edge = %+v, want SurfaceAlt", got)
	}
}

// TestSkeletonDrawAvatar verifies SkeletonAvatar draws the three-band
// pill in SurfaceAlt.
func TestSkeletonDrawAvatar(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonAvatar, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	s.Draw(newP(surf, AvatarSize), theme)
	if got := pixelAt(surf, AvatarSize, 2, 0); got != theme.SurfaceAlt {
		t.Fatalf("avatar top row = %+v, want SurfaceAlt", got)
	}
	// Corner clipped.
	if pixelAt(surf, AvatarSize, 0, 0) == theme.SurfaceAlt {
		t.Fatal("top-left corner should be clipped")
	}
}

// TestSkeletonDrawBlock verifies SkeletonBlock fills the inset rect in
// SurfaceAlt.
func TestSkeletonDrawBlock(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonBlock, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	// Centre pixel filled.
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("block centre = %+v, want SurfaceAlt", got)
	}
	// Padded corner NOT filled (SkeletonLinePad = 4 -> (0,0) is outside).
	if got := pixelAt(surf, 40, 0, 0); got == theme.SurfaceAlt {
		t.Fatal("block corner should stay in sentinel colour (inset by SkeletonLinePad)")
	}
}

// TestSkeletonDrawTextZeroLines covers Lines <= 0 on a text skeleton
// created via the struct literal (bypassing NewSkeleton's default).
// The Draw loop must simply skip.
func TestSkeletonDrawTextZeroLines(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonText, Lines: 0}
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	// No fill happened -- every pixel is still the sentinel.
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	if got := pixelAt(surf, 40, 5, 5); got != sentinel {
		t.Fatalf("expected sentinel at (5,5), got %+v", got)
	}
}

// TestSkeletonDrawTextSingleLine covers Lines == 1: the sole bar IS the
// last bar, so it renders at 60% width.
func TestSkeletonDrawTextSingleLine(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonText, 1)
	const w = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: 20})
	surf := makeSurface(w, 20)
	s.Draw(newP(surf, w), theme)
	// The single bar is 60% wide -- right edge must be untouched.
	if got := pixelAt(surf, w, w-1, SkeletonLineH/2); got == theme.SurfaceAlt {
		t.Fatal("single-bar skeleton should render at 60% width")
	}
	// Left side is filled.
	if got := pixelAt(surf, w, 2, SkeletonLineH/2); got != theme.SurfaceAlt {
		t.Fatalf("single-bar left = %+v, want SurfaceAlt", got)
	}
}

// TestSkeletonDrawDarkTheme sanity-covers a second theme so the theme
// wiring isn't accidentally coupled to DefaultLight.
func TestSkeletonDrawDarkTheme(t *testing.T) {
	theme := DefaultDark()
	s := NewSkeleton(SkeletonBlock, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("dark block = %+v, want SurfaceAlt", got)
	}
}
