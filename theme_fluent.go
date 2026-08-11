// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Fluent approximates the Windows 11 (Fluent) look & feel as a pair of
// code-built palettes, the LIGHT and DARK variants. They are first-class
// built-ins alongside DefaultLight/DefaultDark, WhiteSurLight/WhiteSurDark and
// AdwaitaLight/AdwaitaDark, so an OS-adaptive app can pick a Windows-flavoured
// palette without shipping a theme file. Each palette tags Extra["OnAccent"]
// with the label colour to draw on accent fills; the dark variant's cyan accent
// is light enough to want black ink, unlike every other built-in.
//
// themeWithOnAccent is defined in theme_adwaita.go (same package).

// FluentLight returns the Fluent light palette as a Theme. Like DefaultLight it
// never fails, so callers use it as a drop-in.
func FluentLight() *Theme {
	return themeWithOnAccent(&Theme{
		Background:   RGB(0xF3, 0xF3, 0xF3),
		Surface:      RGB(0xFF, 0xFF, 0xFF),
		SurfaceAlt:   RGB(0xEB, 0xEB, 0xEB),
		OnBackground: RGB(0x20, 0x20, 0x20),
		OnSurface:    RGB(0x20, 0x20, 0x20),
		Accent:       RGB(0x00, 0x67, 0xC0),
		Border:       RGB(0xDF, 0xDF, 0xDF),
	}, RGB(0xFF, 0xFF, 0xFF))
}

// FluentDark returns the Fluent dark palette as a Theme, the drop-in dark
// sibling of FluentLight. Its accent (#4CC2FF) is a bright cyan, so on-accent
// ink is BLACK for contrast.
func FluentDark() *Theme {
	return themeWithOnAccent(&Theme{
		Background:   RGB(0x20, 0x20, 0x20),
		Surface:      RGB(0x2B, 0x2B, 0x2B),
		SurfaceAlt:   RGB(0x27, 0x27, 0x27),
		OnBackground: RGB(0xFF, 0xFF, 0xFF),
		OnSurface:    RGB(0xF0, 0xF0, 0xF0),
		Accent:       RGB(0x4C, 0xC2, 0xFF),
		Border:       RGB(0x1D, 0x1D, 0x1D),
	}, RGB(0x00, 0x00, 0x00))
}
