// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func op(r, g, b uint8) RGBA { return RGBA{R: r, G: g, B: b, A: 0xFF} }

// assertPalette checks every canonical field of a Theme against expected values
// (exact, byte-for-byte) plus the Extra["OnAccent"] tag.
func assertPalette(t *testing.T, name string, th *Theme, bg, surf, surfAlt, onBg, onSurf, accent, border, onAccent RGBA) {
	t.Helper()
	if th.Background != bg {
		t.Errorf("%s Background = %+v, want %+v", name, th.Background, bg)
	}
	if th.Surface != surf {
		t.Errorf("%s Surface = %+v, want %+v", name, th.Surface, surf)
	}
	if th.SurfaceAlt != surfAlt {
		t.Errorf("%s SurfaceAlt = %+v, want %+v", name, th.SurfaceAlt, surfAlt)
	}
	if th.OnBackground != onBg {
		t.Errorf("%s OnBackground = %+v, want %+v", name, th.OnBackground, onBg)
	}
	if th.OnSurface != onSurf {
		t.Errorf("%s OnSurface = %+v, want %+v", name, th.OnSurface, onSurf)
	}
	if th.Accent != accent {
		t.Errorf("%s Accent = %+v, want %+v", name, th.Accent, accent)
	}
	if th.Border != border {
		t.Errorf("%s Border = %+v, want %+v", name, th.Border, border)
	}
	got, ok := th.Extra["OnAccent"]
	if !ok {
		t.Fatalf("%s missing Extra[\"OnAccent\"]", name)
	}
	if got != onAccent {
		t.Errorf("%s Extra[\"OnAccent\"] = %+v, want %+v", name, got, onAccent)
	}
}

func TestAdwaitaLight(t *testing.T) {
	assertPalette(t, "AdwaitaLight", AdwaitaLight(),
		op(0xFA, 0xFA, 0xFA), // Background
		op(0xFF, 0xFF, 0xFF), // Surface
		op(0xF0, 0xF0, 0xF0), // SurfaceAlt
		op(0x2E, 0x34, 0x36), // OnBackground
		op(0x2E, 0x34, 0x36), // OnSurface
		op(0x35, 0x84, 0xE4), // Accent
		op(0xD4, 0xD4, 0xD4), // Border
		op(0xFF, 0xFF, 0xFF), // OnAccent
	)
}

func TestAdwaitaDark(t *testing.T) {
	assertPalette(t, "AdwaitaDark", AdwaitaDark(),
		op(0x24, 0x24, 0x24), // Background
		op(0x30, 0x30, 0x30), // Surface
		op(0x1E, 0x1E, 0x1E), // SurfaceAlt
		op(0xFF, 0xFF, 0xFF), // OnBackground
		op(0xEE, 0xEE, 0xEE), // OnSurface
		op(0x35, 0x84, 0xE4), // Accent
		op(0x1B, 0x1B, 0x1B), // Border
		op(0xFF, 0xFF, 0xFF), // OnAccent
	)
}

// TestAdwaitaLightDarkDiffer guards against a copy/paste palette regression: the
// two variants must differ on the grounds and inks (accent stays shared).
func TestAdwaitaLightDarkDiffer(t *testing.T) {
	l, d := AdwaitaLight(), AdwaitaDark()
	if l.Background == d.Background {
		t.Error("Adwaita light/dark Background must differ")
	}
	if l.OnBackground == d.OnBackground {
		t.Error("Adwaita light/dark OnBackground must differ")
	}
	if l.Accent != d.Accent {
		t.Errorf("Adwaita accent should be shared across variants: %+v vs %+v", l.Accent, d.Accent)
	}
}

// TestThemeWithOnAccentPreservesExtra covers the branch where Extra is already
// populated: the helper must add OnAccent without dropping existing keys.
func TestThemeWithOnAccentPreservesExtra(t *testing.T) {
	th := &Theme{Extra: map[string]RGBA{"success_color": op(0x2F, 0xA8, 0x4F)}}
	got := themeWithOnAccent(th, op(0x11, 0x22, 0x33))
	if got != th {
		t.Fatal("themeWithOnAccent should return the same pointer")
	}
	if _, ok := th.Extra["success_color"]; !ok {
		t.Error("existing Extra key was dropped")
	}
	if th.Extra["OnAccent"] != op(0x11, 0x22, 0x33) {
		t.Errorf("OnAccent = %+v, want %+v", th.Extra["OnAccent"], op(0x11, 0x22, 0x33))
	}
}
