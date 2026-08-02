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

// UseOpenTypeTextSize is UseOpenTypeText at an explicit pixel size — for apps
// (or high-DPI surfaces) that want AA text larger or smaller than
// DefaultOpenTypeSizePx. The active font is only swapped on success; on a parse
// error it is left untouched and the error is returned.
func UseOpenTypeTextSize(sizePx int) error {
	f, err := DefaultOpenTypeFont(sizePx)
	if err != nil {
		return err
	}
	SetFont(f)
	return nil
}
