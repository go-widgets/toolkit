// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-opentype/fonts"

// This file gives consumers a one-call upgrade from the built-in 5x7 bitmap
// font to anti-aliased, shaped OpenType text, WITHOUT each app having to embed
// and parse a face of its own.
//
// The toolkit already renders crisp, multi-script text whenever a TrueType face
// is installed (see truetype.go / fallback.go): glyph outlines are scan-
// converted to anti-aliased coverage masks and complex scripts are run through
// github.com/go-opentype/shape (bidi, Arabic joining, ligatures, marks,
// kerning). What was missing was a batteries-included DEFAULT face, so every
// consumer had to source, embed and wire up its own font before it saw AA text.
//
// The bitmap font stays the compiled-in default so the existing pixel- and
// metric-precise tests (and any downstream golden/pixel-parity tests) keep
// their current geometry. Anti-aliased text is therefore an explicit, single-
// call opt-in: an app flips it once at start-up and every widget re-lays-out
// and repaints against the new face, because widgets measure through
// Measure/TextWidth and paint through DrawText rather than assuming the bitmap.
//
// The bundled face is Atkinson Hyperlegible from github.com/go-opentype/fonts,
// which the fonts package embeds directly (via go:embed, so it builds and runs
// under GOOS=js/GOARCH=wasm) and exposes with zero extra imports through
// fonts.MostLegible. It covers Latin and extended Latin; apps needing CJK,
// Arabic, Devanagari, … compose a fallback chain themselves — see
// DefaultOpenTypeFont for the recommended pattern.

// DefaultOpenTypeSizePx is the pixel size UseOpenTypeText renders the bundled
// default face at. It is chosen for comfortable on-screen UI text — clearly
// larger and more legible than the 7px bitmap default — while staying compact
// enough for dense chrome (window titles, dock labels, menus). Use
// UseOpenTypeTextSize (or DefaultOpenTypeFont) for a different size.
const DefaultOpenTypeSizePx = 16

// defaultFaceTTF returns the bytes of the toolkit's bundled default face. It is
// a package variable rather than a direct call to fonts.MostLegible purely so
// tests can substitute a malformed blob and exercise DefaultOpenTypeFont's
// parse-error path; production code never reassigns it.
var defaultFaceTTF = fonts.MostLegible

// DefaultOpenTypeFont returns the toolkit's bundled default face — Atkinson
// Hyperlegible, designed by the Braille Institute for maximum character
// distinction — as an anti-aliased, shaped Font at sizePx pixels. A parse
// failure (which the bundled face never triggers) is returned wrapped.
//
// Use it to install AA text yourself, or to build a multi-script fallback chain
// before installing:
//
//	base, _ := toolkit.DefaultOpenTypeFont(16)
//	cjk, _ := toolkit.NewTrueTypeFont(notosanssc.TTF, 16)
//	f, _ := toolkit.NewFallbackFont(base, cjk)
//	toolkit.SetFont(f)
func DefaultOpenTypeFont(sizePx int) (Font, error) {
	return NewTrueTypeFont(defaultFaceTTF(), sizePx)
}

// UseOpenTypeText switches the toolkit's active font from the built-in 5x7
// bitmap to anti-aliased, shaped OpenType text — the bundled Atkinson
// Hyperlegible face at DefaultOpenTypeSizePx — in a single call. After it, every
// widget (window titles, dock, menus, HUD, …) re-lays-out and repaints against
// the vector face without any further per-widget wiring.
//
// Call it once at start-up. It returns any parse error (the bundled face never
// produces one) and leaves the active font unchanged in that case, so a failure
// degrades to the still-working bitmap default rather than to no text. Restore
// the bitmap default at any time with SetFont(nil).
func UseOpenTypeText() error { return UseOpenTypeTextSize(DefaultOpenTypeSizePx) }

// openTypeLogicalPx is the size UseOpenTypeTextSize was last asked for, in
// LOGICAL pixels, or 0 when the bitmap default is active.
//
// It is remembered so [SetMetricScale] can re-render the face: every other
// metric in this toolkit is multiplied by the scale, and the one thing a
// person actually reads was not. On a display with two device pixels to the
// point that made every label half the size the caller asked for -- reported
// as "small but readable", which is a polite way of saying wrong.
var openTypeLogicalPx int

// rescaleText re-renders the active face at the current [MetricScale], if the
// caller asked for an OpenType face at all.
//
// A failure leaves the face that is working in place: a toolkit that dropped
// its text because a resize could not re-render it would be worse than one
// whose text is briefly the wrong size.
func rescaleText() {
	if openTypeLogicalPx <= 0 {
		return
	}
	px := int(float64(openTypeLogicalPx)*metricScale + 0.5)
	if px < 1 {
		px = 1
	}
	if f, err := DefaultOpenTypeFont(px); err == nil {
		SetFont(f)
	}
}

// UseOpenTypeTextSize is UseOpenTypeText at an explicit pixel size — for apps
// (or high-DPI surfaces) that want AA text larger or smaller than
// DefaultOpenTypeSizePx. The active font is only swapped on success; on a parse
// error it is left untouched and the error is returned.
func UseOpenTypeTextSize(sizePx int) error {
	// LOGICAL pixels, like every other metric this toolkit takes: the face is
	// rendered at sizePx x MetricScale, and re-rendered whenever that scale
	// changes. A caller asking for 16 gets type that reads the same size on
	// every display, which is the whole point of a scale.
	px := int(float64(sizePx)*metricScale + 0.5)
	if px < 1 {
		px = 1
	}
	f, err := DefaultOpenTypeFont(px)
	if err != nil {
		return err
	}
	SetFont(f)
	openTypeLogicalPx = sizePx
	return nil
}
