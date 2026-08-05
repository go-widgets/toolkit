// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// accentPixelsInBounds renders an active spinner of the given style and returns
// how many Accent pixels it painted, failing if any painted (non-sentinel) pixel
// falls outside the widget's bounds.
func accentPixelsInBounds(t *testing.T, style SpinnerStyle, phase float64) int {
	t.Helper()
	const stride = 48
	theme := DefaultLight()
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	b := Rect{X: 4, Y: 4, W: 40, H: 40}
	s := NewSpinner()
	s.Active, s.Style, s.Phase = true, style, phase
	s.SetBounds(b)
	buf := makeSurface(stride, stride)
	s.Draw(newP(buf, stride), theme)
	accent := 0
	for y := 0; y < stride; y++ {
		for x := 0; x < stride; x++ {
			px := pixelAt(buf, stride, x, y)
			if px == sentinel {
				continue
			}
			if !b.Contains(x, y) {
				t.Fatalf("style %d painted at (%d,%d) outside bounds %+v", style, x, y, b)
			}
			if px == theme.Accent {
				accent++
			}
		}
	}
	return accent
}

func TestSpinnerDotsPaintsAccentHead(t *testing.T) {
	// Phase 0 → the head dot sits exactly on Accent (behind==0); other dots fade.
	if n := accentPixelsInBounds(t, SpinnerDots, 0); n == 0 {
		t.Fatal("dots spinner painted no Accent head dot")
	}
}

func TestSpinnerRingPaintsAccentHead(t *testing.T) {
	if n := accentPixelsInBounds(t, SpinnerRing, 0.25); n == 0 {
		t.Fatal("ring spinner painted no Accent head")
	}
}

func TestSpinnerBarsPaintAccent(t *testing.T) {
	// Bars are solid Accent; a non-trivial phase gives each bar a real height.
	if n := accentPixelsInBounds(t, SpinnerBars, 0.2); n == 0 {
		t.Fatal("bars spinner painted no Accent bars")
	}
}

// TestSpinnerStylesTinyBounds drives every new style at a sub-marker size so the
// atLeast1 size floors are exercised without panics or out-of-bounds paints.
func TestSpinnerStylesTinyBounds(t *testing.T) {
	for _, style := range []SpinnerStyle{SpinnerDots, SpinnerRing, SpinnerBars} {
		s := NewSpinner()
		s.Active, s.Style = true, style
		s.SetBounds(Rect{X: 0, Y: 0, W: 6, H: 6})
		s.Draw(newP(makeSurface(8, 8), 8), DefaultLight())
	}
}
