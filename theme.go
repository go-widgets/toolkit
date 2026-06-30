// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// RGBA is a 32-bit colour value packed as bytes (Red, Green, Blue,
// Alpha). Alpha is honoured by the rasterizer when supplied, but the
// stock widgets all paint opaque pixels (A=0xFF) for simplicity.
type RGBA struct{ R, G, B, A uint8 }

// RGB constructs an opaque colour with A=0xFF. Used in theme literals
// so the alpha plumbing isn't visible at the call site.
func RGB(r, g, b uint8) RGBA { return RGBA{r, g, b, 0xFF} }

// Theme bundles every visual constant a widget needs to render itself.
// One Theme value cascades through every widget in an app, so swapping
// to a dark / Aqua / Fluxbox theme is a single assignment.
//
// Field naming follows Material/Fluxbox conventions:
//   - Background = the surface a widget sits on (panel/window body)
//   - Surface    = the widget's own filled body (button face, ...)
//   - SurfaceAlt = a contrasting tone (hovered button, alternating row)
//   - OnBackground / OnSurface = ink/text on those grounds
//   - Accent     = focus rings, the active-tab underline, the link colour
//   - Border     = a thin separator line drawn around or between
//                  surface regions
type Theme struct {
	Background   RGBA
	Surface      RGBA
	SurfaceAlt   RGBA
	OnBackground RGBA
	OnSurface    RGBA
	Accent       RGBA
	Border       RGBA

	// Extra holds @define-color entries from GTK-source themes that don't
	// map to one of the canonical fields above (headerbar_bg_color,
	// success_color, ...). Populated by LoadGTKTheme; nil for code-built
	// themes. The compositor / a host app can look up custom colors here
	// when wiring window-chrome theming (Niveau B subsumption of
	// wasmaqua) without growing this struct for every GTK color name in
	// the wild.
	Extra map[string]RGBA
}

// DefaultLight is a low-stakes light theme used by tests + as the
// fall-through when an app doesn't supply its own. Numbers are the
// Fluxbox Light palette wasmbox's dock already uses, so a widget
// dropped into the dock without an explicit theme renders cleanly.
func DefaultLight() *Theme {
	return &Theme{
		Background:   RGB(0xFA, 0xFA, 0xFA),
		Surface:      RGB(0xE8, 0xEA, 0xED),
		SurfaceAlt:   RGB(0xD0, 0xD4, 0xD8),
		OnBackground: RGB(0x1A, 0x1A, 0x1A),
		OnSurface:    RGB(0x1A, 0x1A, 0x1A),
		Accent:       RGB(0x35, 0x84, 0xE4),
		Border:       RGB(0xB0, 0xB4, 0xB8),
	}
}

// DefaultDark is a low-contrast dark theme. Same shape as
// DefaultLight; used by themed wasmaqua apps + test coverage.
func DefaultDark() *Theme {
	return &Theme{
		Background:   RGB(0x14, 0x16, 0x1A),
		Surface:      RGB(0x1F, 0x22, 0x28),
		SurfaceAlt:   RGB(0x2A, 0x2E, 0x36),
		OnBackground: RGB(0xE6, 0xE7, 0xEE),
		OnSurface:    RGB(0xE6, 0xE7, 0xEE),
		Accent:       RGB(0x4F, 0x9D, 0xF2),
		Border:       RGB(0x3A, 0x3E, 0x46),
	}
}
