// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestWhiteSurLight(t *testing.T) {
	th := WhiteSurLight()
	// Canonical fields, mapped from whitesur-light.css by LoadGTKTheme.
	cases := map[string]struct {
		got, want RGBA
	}{
		"Background":   {th.Background, RGB(0xf5, 0xf5, 0xf5)},   // window_bg_color
		"OnBackground": {th.OnBackground, RGB(0x24, 0x24, 0x24)}, // window_fg_color
		"Surface":      {th.Surface, RGB(0xff, 0xff, 0xff)},      // view_bg_color
		"OnSurface":    {th.OnSurface, RGB(0x36, 0x36, 0x36)},    // view_fg_color
		"SurfaceAlt":   {th.SurfaceAlt, RGB(0xfb, 0xfb, 0xfb)},   // card_bg_color
		"Accent":       {th.Accent, RGB(0x08, 0x60, 0xf2)},       // accent_bg_color
		"Border":       {th.Border, RGBA{R: 0, G: 0, B: 0, A: 0x1E}}, // borders rgba(0,0,0,0.12)
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("WhiteSurLight().%s = %v, want %v", name, c.got, c.want)
		}
	}
	// Extra carries the named roles a header-bar / sidebar app looks up.
	extra := map[string]RGBA{
		"headerbar_bg_color": RGB(0xeb, 0xeb, 0xeb),
		"card_bg_color":      RGB(0xfb, 0xfb, 0xfb),
		"accent_fg_color":    RGB(0xff, 0xff, 0xff),
		"success_color":      RGB(0x79, 0xB7, 0x57),
		"warning_color":      RGB(0xF3, 0xBA, 0x4B),
		"error_color":        RGB(0xED, 0x5F, 0x5D),
	}
	for name, want := range extra {
		if got, ok := th.Extra[name]; !ok || got != want {
			t.Errorf("WhiteSurLight().Extra[%q] = (%v, %v), want (%v, true)", name, got, ok, want)
		}
	}
}

func TestWhiteSurDark(t *testing.T) {
	th := WhiteSurDark()
	cases := map[string]struct {
		got, want RGBA
	}{
		"Background":   {th.Background, RGB(0x33, 0x33, 0x33)},   // window_bg_color
		"OnBackground": {th.OnBackground, RGB(0xde, 0xde, 0xde)}, // window_fg_color
		"Surface":      {th.Surface, RGB(0x24, 0x24, 0x24)},      // view_bg_color
		"SurfaceAlt":   {th.SurfaceAlt, RGB(0x2c, 0x2c, 0x2c)},   // card_bg_color
		"Accent":       {th.Accent, RGB(0x08, 0x60, 0xf2)},       // accent_bg_color
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("WhiteSurDark().%s = %v, want %v", name, c.got, c.want)
		}
	}
	if got, want := th.Extra["headerbar_bg_color"], RGB(0x1f, 0x1f, 0x1f); got != want {
		t.Errorf("WhiteSurDark().Extra[headerbar_bg_color] = %v, want %v", got, want)
	}
}

// mustGTKTheme panics on an empty/malformed palette; the embedded WhiteSur CSS
// never hits it, so exercise the branch directly to keep coverage complete.
func TestMustGTKThemePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("mustGTKTheme(\"\") did not panic")
		}
	}()
	_ = mustGTKTheme("")
}
