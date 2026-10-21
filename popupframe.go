// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// PopupBorderMinContrast is the least contrast ratio the outline of a popup
// (drop-down list, combobox list, menu, command palette, completion list) must
// reach against both the page it floats over (Theme.Background) and its own
// body (Theme.Surface). It is the 3:1 that WCAG 2.2 success criterion 1.4.11
// (Non-text Contrast) asks of "visual information required to identify user
// interface components": a popup has no other cue that says where it stops.
const PopupBorderMinContrast = 3.0

// contrastRatio is the WCAG 2.x contrast ratio (L1+0.05)/(L2+0.05) of two
// colours, the lighter luminance on top, alpha ignored.
func contrastRatio(a, b RGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// PopupBorder is the colour a popup outline is stroked with under theme: the
// theme's own Border when that already reaches [PopupBorderMinContrast] against
// both Background and Surface, otherwise Border pulled toward OnSurface in 4%
// steps until it does (OnSurface itself when no mix gets there). The hue stays
// the theme's, and a theme whose Border is already strong is returned
// unchanged, so only the palettes that drew a faint hairline (the usual dark
// "#3a4556 on #141a24") change.
//
// Theme.Border is meant as a quiet separator between regions that already
// differ in tone; a popup's edge is the one place a separator is the only
// thing telling the popup from what is behind it, which is why it has its own
// resolution instead of lowering the bar for every separator.
func PopupBorder(theme *Theme) RGBA {
	ok := func(c RGBA) bool {
		return contrastRatio(c, theme.Background) >= PopupBorderMinContrast &&
			contrastRatio(c, theme.Surface) >= PopupBorderMinContrast
	}
	c := theme.Border
	if ok(c) {
		return c
	}
	for i := 1; i <= 25; i++ {
		c = blendRGBA(theme.OnSurface, theme.Border, float64(i)*0.04)
		if ok(c) {
			return c
		}
	}
	return theme.OnSurface
}

// strokePopupBorder outlines a popup's rectangle with [PopupBorder].
func strokePopupBorder(p painter.Painter, r Rect, theme *Theme) {
	strokeRect(p, r.X, r.Y, r.W, r.H, PopupBorder(theme))
}
