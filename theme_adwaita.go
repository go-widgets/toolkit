// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Adwaita approximates GNOME's default (libadwaita) look & feel as a pair of
// code-built palettes, the LIGHT and DARK variants. They are first-class
// built-ins alongside DefaultLight/DefaultDark and WhiteSurLight/WhiteSurDark,
// so an OS-adaptive app can pick a GNOME-flavoured palette without shipping a
// GTK CSS file. Each palette tags Extra["OnAccent"] with the label colour to
// draw on accent fills (the convention accent-filling widgets -- Button, Table,
// SplitButton, ViewSwitcher, ... -- look up first, falling back to Background),
// so header-bar / topbar text stays legible on the blue accent.

// themeWithOnAccent tags t with the ink colour used on accent fills, allocating
// the Extra map on demand, and returns t for fluent construction. Shared by the
// code-built accent palettes in this file and theme_fluent.go.
func themeWithOnAccent(t *Theme, onAccent RGBA) *Theme {
	if t.Extra == nil {
		t.Extra = map[string]RGBA{}
	}
	t.Extra["OnAccent"] = onAccent
	return t
}

// AdwaitaLight returns the Adwaita light palette as a Theme. Like DefaultLight
// it never fails, so callers use it as a drop-in.
func AdwaitaLight() *Theme {
	return themeWithOnAccent(&Theme{
		Background:   RGB(0xFA, 0xFA, 0xFA),
		Surface:      RGB(0xFF, 0xFF, 0xFF),
		SurfaceAlt:   RGB(0xF0, 0xF0, 0xF0),
		OnBackground: RGB(0x2E, 0x34, 0x36),
		OnSurface:    RGB(0x2E, 0x34, 0x36),
		Accent:       RGB(0x35, 0x84, 0xE4),
		Border:       RGB(0xD4, 0xD4, 0xD4),
	}, RGB(0xFF, 0xFF, 0xFF))
}

// AdwaitaDark returns the Adwaita dark palette as a Theme, the drop-in dark
// sibling of AdwaitaLight.
func AdwaitaDark() *Theme {
	return themeWithOnAccent(&Theme{
		Background:   RGB(0x24, 0x24, 0x24),
		Surface:      RGB(0x30, 0x30, 0x30),
		SurfaceAlt:   RGB(0x1E, 0x1E, 0x1E),
		OnBackground: RGB(0xFF, 0xFF, 0xFF),
		OnSurface:    RGB(0xEE, 0xEE, 0xEE),
		Accent:       RGB(0x35, 0x84, 0xE4),
		Border:       RGB(0x1B, 0x1B, 0x1B),
	}, RGB(0xFF, 0xFF, 0xFF))
}
