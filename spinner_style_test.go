// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestSpinnerActiveObservable covers the zero-value lazy-init of the Active
// accessor and the host binding path: a Spinner built as a bare struct (no
// NewSpinner) still yields a usable Observable defaulting to false, and Setting
// it from outside toggles what the widget reports and draws (there is no
// imperative Active field).
func TestSpinnerActiveObservable(t *testing.T) {
	s := &Spinner{} // no NewSpinner → active Observable is nil until accessed
	if s.Active().Get() {
		t.Fatalf("bare Spinner Active = true, want false")
	}
	if s.Animating() {
		t.Fatalf("bare Spinner Animating = true, want false")
	}
	seen := false
	s.Active().Subscribe(func(v bool) { seen = v })
	s.Active().Set(true) // a host drives the spinner through the Observable
	if !s.Active().Get() || !seen {
		t.Fatalf("host Set: active=%v subscriber=%v, want true/true", s.Active().Get(), seen)
	}
	if !s.Animating() {
		t.Fatalf("Animating after Set(true) = false, want true")
	}
}

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
	s.Active().Set(true)
	s.Style, s.Phase = style, phase
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
		s.Active().Set(true)
		s.Style = style
		s.SetBounds(Rect{X: 0, Y: 0, W: 6, H: 6})
		s.Draw(newP(makeSurface(8, 8), 8), DefaultLight())
	}
}
