// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewToastDefaults(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	if tt.Text != "hi" {
		t.Fatalf("Text = %q, want %q", tt.Text, "hi")
	}
	if tt.Kind != ToastInfo {
		t.Fatalf("Kind = %d, want ToastInfo", tt.Kind)
	}
	if tt.Visible {
		t.Fatal("fresh Toast must be hidden")
	}
	if tt.Life != 0 {
		t.Fatalf("Life = %d, want 0", tt.Life)
	}
}

// --- Draw: hidden --------------------------------------------------------

func TestToastDrawHiddenNoOp(t *testing.T) {
	tt := NewToast("x", ToastInfo)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	surf := makeSurface(60, 20)
	before := make([]byte, len(surf))
	copy(before, surf)
	tt.Draw(newP(surf, 60), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw on hidden Toast touched byte %d: %d -> %d", i, before[i], surf[i])
		}
	}
}

// --- Draw: zero-width bounds --------------------------------------------

func TestToastDrawZeroWidthBoundsSkipsFill(t *testing.T) {
	// Position the widget so its whole footprint sits above the
	// surface -- text glyphs then clip out per-pixel and the fillRect
	// guard is the only pixel-writing path exercised, which we assert
	// leaves the buffer untouched.
	tt := NewToast("x", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: -30, W: 0, H: 20}) // zero W -> fillRect guard
	surf := makeSurface(20, 20)
	before := make([]byte, len(surf))
	copy(before, surf)
	tt.Draw(newP(surf, 20), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw at zero W painted byte %d", i)
		}
	}
	// Zero H as well -- exercises the second guard branch.
	tt.SetBounds(Rect{X: 0, Y: -30, W: 20, H: 0})
	surf2 := makeSurface(20, 20)
	copy(before, surf2)
	tt.Draw(newP(surf2, 20), DefaultLight())
	for i := range surf2 {
		if surf2[i] != before[i] {
			t.Fatalf("Draw at zero H painted byte %d", i)
		}
	}
}

// --- Draw: each Kind paints its documented face ------------------------

func TestToastDrawKindColours(t *testing.T) {
	theme := DefaultLight()
	cases := []struct {
		kind ToastKind
		want RGBA
	}{
		{ToastInfo, theme.Accent},
		{ToastSuccess, RGB(0x2E, 0x8B, 0x57)},
		{ToastWarning, RGB(0xE0, 0xA0, 0x30)},
		{ToastError, RGB(0xC0, 0x30, 0x30)},
	}
	for _, c := range cases {
		tt := NewToast("!", c.kind)
		tt.Visible = true
		tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
		buf := makeSurface(60, 20)
		tt.Draw(newP(buf, 60), theme)
		// Sample a pixel well inside the pill face, away from stroke +
		// text glyphs.
		if got := pixelAt(buf, 60, 40, 15); got != c.want {
			t.Fatalf("kind %d fill = %+v, want %+v", c.kind, got, c.want)
		}
	}
}

// --- Draw: dark theme + Extra OnAccent override ------------------------

func TestToastDrawUsesOnAccentFromExtra(t *testing.T) {
	tt := NewToast("XYZ", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 20})
	theme := DefaultDark()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	buf := makeSurface(80, 20)
	tt.Draw(newP(buf, 80), theme)
	// Somewhere in the pill body must be at least one custom-coloured
	// text glyph pixel -- proves accentInk resolved to Extra["OnAccent"].
	found := false
	for y := 0; y < 20 && !found; y++ {
		for x := 0; x < 80; x++ {
			if pixelAt(buf, 80, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in Toast body")
	}
}

// --- Draw: fallback when Extra is nil ----------------------------------

func TestToastDrawAccentInkFallbackWithNilExtra(t *testing.T) {
	tt := NewToast("q", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	theme := DefaultLight()
	theme.Extra = nil
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	// Fill still painted in Accent regardless of ink resolution.
	if pixelAt(buf, 60, 40, 15) != theme.Accent {
		t.Fatal("nil-Extra Toast body fill != Accent")
	}
}

// --- Draw: fallback when Extra map has no OnAccent key ----------------

func TestToastDrawAccentInkFallbackWithExtraNoKey(t *testing.T) {
	tt := NewToast("q", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{} // present but empty -> ok=false path
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	if pixelAt(buf, 60, 40, 15) != theme.Accent {
		t.Fatal("empty-Extra Toast body fill != Accent")
	}
}

// --- Tick: Life > 0 decrements without hiding --------------------------

func TestToastTickWithLifeAboveZeroDecrements(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 5
	tt.Tick()
	if tt.Life != 4 {
		t.Fatalf("Life after Tick = %d, want 4", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("Toast should stay Visible while Life > 0")
	}
}

// --- Tick: Life == 0 is a no-op (sticky sentinel) ---------------------

func TestToastTickWithLifeZeroNoOp(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 0 // sticky
	tt.Tick()
	if tt.Life != 0 {
		t.Fatalf("Life on sticky Toast = %d, want 0", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("sticky Toast should stay Visible on Tick")
	}
}

// --- Tick: hitting zero flips Visible to false -----------------------

func TestToastTickReachingZeroHides(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 1
	tt.Tick()
	if tt.Life != 0 {
		t.Fatalf("Life = %d, want 0", tt.Life)
	}
	if tt.Visible {
		t.Fatal("Tick that reaches 0 must clear Visible")
	}
}

// --- Tick: negative Life is treated as sticky (no-op) -----------------

func TestToastTickWithNegativeLifeNoOp(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = -3 // pathological input; guard treats it as sticky
	tt.Tick()
	if tt.Life != -3 {
		t.Fatalf("Life = %d, want -3", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("negative-Life Toast should stay Visible on Tick")
	}
}
