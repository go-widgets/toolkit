// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import _ "embed"

// WhiteSur is Vince Liu's macOS Big Sur-styled GTK theme
// (https://www.gnome-look.org/p/1403328,
// https://github.com/vinceliuice/WhiteSur-gtk-theme, GPL-3.0), one of the
// most-downloaded GTK themes on gnome-look.org. These two palettes are the
// hand-resolved LIGHT and DARK variants (DEFAULT accent #0860F2), embedded so
// WhiteSurLight/WhiteSurDark are first-class built-ins alongside
// DefaultLight/DefaultDark -- an app pairs them with wasmbox's --frame=aqua for
// a near-pixel-accurate Big Sur look (Aqua window chrome around the WhiteSur
// widget palette). The palettes carry every named @define-color (headerbar,
// card, view, success/warning/error, ...) in the returned Theme's Extra map, so
// a header-bar or sidebar-drawing app looks up those roles by name.
//
// The CSS is the single source of truth: WhiteSurLight/WhiteSurDark parse it
// through LoadGTKTheme, so a built-in theme is byte-identical to loading the
// same file at runtime.

//go:embed themes/whitesur-light.css
var whiteSurLightCSS string

//go:embed themes/whitesur-dark.css
var whiteSurDarkCSS string

// WhiteSurLight returns the WhiteSur light palette as a Theme. It never fails
// (the embedded CSS is non-empty and well-formed), so unlike LoadGTKTheme it
// has no error return -- callers use it as a drop-in for DefaultLight.
func WhiteSurLight() *Theme { return mustGTKTheme(whiteSurLightCSS) }

// WhiteSurDark returns the WhiteSur dark palette as a Theme, the drop-in dark
// sibling of WhiteSurLight.
func WhiteSurDark() *Theme { return mustGTKTheme(whiteSurDarkCSS) }

// mustGTKTheme parses an embedded, known-good palette. The error branch is
// unreachable for the embedded CSS (non-empty by construction); it panics
// rather than returning an error so the exported constructors stay ergonomic.
func mustGTKTheme(css string) *Theme {
	t, err := LoadGTKTheme(css)
	if err != nil {
		panic("toolkit: embedded WhiteSur theme failed to parse: " + err.Error())
	}
	return t
}
