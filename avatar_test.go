// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestAvatarAutoSizesWhenWZero covers the auto-size branch: Bounds()
// starts with W = 0 (and H = 0) and after Draw both must be AvatarSize.
func TestAvatarAutoSizesWhenWZero(t *testing.T) {
	a := NewAvatar("AB")
	a.SetBounds(Rect{X: 4, Y: 4, W: 0, H: 0})
	a.Draw(newP(makeSurface(80, 80), 80), DefaultLight())
	got := a.Bounds()
	if got.W != AvatarSize || got.H != AvatarSize {
		t.Fatalf("auto-sized Bounds = %+v, want %d x %d", got, AvatarSize, AvatarSize)
	}
}

// TestAvatarAutoSizeWZeroPreservesH covers the sub-branch where W is
// zero but H is already set: only W should be filled in, H must be
// left as the caller-supplied value.
func TestAvatarAutoSizeWZeroPreservesH(t *testing.T) {
	a := NewAvatar("A")
	a.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 48})
	a.Draw(newP(makeSurface(80, 80), 80), DefaultLight())
	got := a.Bounds()
	if got.W != AvatarSize {
		t.Fatalf("auto-sized W = %d, want %d", got.W, AvatarSize)
	}
	if got.H != 48 {
		t.Fatalf("caller-supplied H clobbered: got H = %d, want 48", got.H)
	}
}

// TestAvatarPreSizedNoOverride covers the non-auto-size branch: when W
// is already set, Draw must respect the caller's Bounds verbatim.
func TestAvatarPreSizedNoOverride(t *testing.T) {
	a := NewAvatar("XY")
	a.SetBounds(Rect{X: 2, Y: 2, W: 40, H: 40})
	a.Draw(newP(makeSurface(60, 60), 60), DefaultLight())
	got := a.Bounds()
	if got.W != 40 || got.H != 40 {
		t.Fatalf("pre-sized Bounds changed: %+v", got)
	}
}

// TestAvatarBodyFillsAccentWhenColorZero samples an interior pixel and
// checks it lands in Theme.Accent when Color is the zero RGBA.
func TestAvatarBodyFillsAccentWhenColorZero(t *testing.T) {
	theme := DefaultLight()
	a := NewAvatar("A")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize+4, AvatarSize+4)
	a.Draw(newP(surf, AvatarSize+4), theme)
	// (2, 0) is inside the pill body's top row (the top row spans
	// x = 1 .. W-2 in the three-band pill).
	if got := pixelAt(surf, AvatarSize+4, 2, 0); got != theme.Accent {
		t.Fatalf("avatar top-row body = %+v, want Accent", got)
	}
}

// TestAvatarBodyFillsCustomColor covers the non-zero Color branch: a
// caller-supplied face colour must be honoured verbatim.
func TestAvatarBodyFillsCustomColor(t *testing.T) {
	theme := DefaultLight()
	custom := RGB(0x11, 0x22, 0x33)
	a := &Avatar{Initials: "X", Color: custom}
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize+4, AvatarSize+4)
	a.Draw(newP(surf, AvatarSize+4), theme)
	if got := pixelAt(surf, AvatarSize+4, 2, 0); got != custom {
		t.Fatalf("avatar body with Color set = %+v, want %+v", got, custom)
	}
}

// TestAvatarCornerClipped covers the three-band pill's rounded-corner
// approximation: (0, 0) is outside the three fills and must remain the
// sentinel colour.
func TestAvatarCornerClipped(t *testing.T) {
	theme := DefaultLight()
	a := NewAvatar("X")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	a.Draw(newP(surf, AvatarSize), theme)
	if pixelAt(surf, AvatarSize, 0, 0) == theme.Accent {
		t.Fatal("top-left corner should be clipped, not Accent-filled")
	}
}

// TestAvatarInkUsesOnAccentFromExtra covers the accentInk path when a
// theme carries an OnAccent override in Extra: a glyph pixel inside the
// avatar body must land in that custom colour.
func TestAvatarInkUsesOnAccentFromExtra(t *testing.T) {
	theme := DefaultLight()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	a := NewAvatar("A")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	a.Draw(newP(surf, AvatarSize), theme)
	found := false
	for y := 0; y < AvatarSize && !found; y++ {
		for x := 0; x < AvatarSize; x++ {
			if pixelAt(surf, AvatarSize, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in avatar")
	}
}

// TestAvatarInkFallbackWithNilExtra exercises the accentInk fall-through
// when Extra is nil — the glyph must land in Theme.Background.
func TestAvatarInkFallbackWithNilExtra(t *testing.T) {
	theme := DefaultLight()
	theme.Extra = nil
	a := NewAvatar("A")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	a.Draw(newP(surf, AvatarSize), theme)
	found := false
	for y := 0; y < AvatarSize && !found; y++ {
		for x := 0; x < AvatarSize; x++ {
			if pixelAt(surf, AvatarSize, x, y) == theme.Background {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no Background-coloured glyph pixel found in avatar")
	}
}

// TestAvatarInkFallbackWithExtraNoKey exercises the accentInk
// ok==false branch: Extra is non-nil but has no OnAccent entry.
func TestAvatarInkFallbackWithExtraNoKey(t *testing.T) {
	theme := DefaultDark()
	theme.Extra = map[string]RGBA{"headerbar_bg_color": RGB(1, 2, 3)}
	a := NewAvatar("A")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	a.Draw(newP(surf, AvatarSize), theme)
	if got := pixelAt(surf, AvatarSize, 2, 0); got != theme.Accent {
		t.Fatalf("body pixel = %+v, want Accent %+v", got, theme.Accent)
	}
}

// TestAvatarEmptyInitials covers the empty-Initials branch: no glyphs
// are drawn but the body still paints without panicking.
func TestAvatarEmptyInitials(t *testing.T) {
	a := NewAvatar("")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	a.Draw(newP(makeSurface(AvatarSize, AvatarSize), AvatarSize), DefaultLight())
}

// TestAvatarDarkTheme sanity-covers a second theme so the theme wiring
// isn't accidentally coupled to DefaultLight.
func TestAvatarDarkTheme(t *testing.T) {
	theme := DefaultDark()
	a := NewAvatar("Z")
	a.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	a.Draw(newP(surf, AvatarSize), theme)
	if got := pixelAt(surf, AvatarSize, 2, 0); got != theme.Accent {
		t.Fatalf("dark-theme body = %+v, want Accent %+v", got, theme.Accent)
	}
}
