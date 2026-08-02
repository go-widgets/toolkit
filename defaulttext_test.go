// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// DefaultOpenTypeFont returns a working, anti-aliased, proportional face from
// the bundled default font: sane strictly-positive metrics, and a proportional
// (non-monospace) Measure that beats the 7px bitmap on line height.
func TestDefaultOpenTypeFontMetrics(t *testing.T) {
	f, err := DefaultOpenTypeFont(DefaultOpenTypeSizePx)
	if err != nil {
		t.Fatalf("DefaultOpenTypeFont(%d) error: %v", DefaultOpenTypeSizePx, err)
	}
	if f.Height() <= baseGlyphHeight {
		t.Fatalf("Height() = %d, want a real vector line height > bitmap %d", f.Height(), baseGlyphHeight)
	}
	if f.Advance() <= 0 {
		t.Fatalf("Advance() = %d, want > 0", f.Advance())
	}
	// Proportional: a narrow run is narrower than a wide run of equal length,
	// which a monospace font could never satisfy.
	if f.Measure("iiii") >= f.Measure("MMMM") {
		t.Fatalf("Measure not proportional: iiii=%d MMMM=%d", f.Measure("iiii"), f.Measure("MMMM"))
	}
}

// DefaultOpenTypeFont wraps and returns a parse error when the bundled bytes
// are unusable (exercised via the defaultFaceTTF seam).
func TestDefaultOpenTypeFontParseError(t *testing.T) {
	orig := defaultFaceTTF
	defaultFaceTTF = func() []byte { return []byte("not a font at all") }
	defer func() { defaultFaceTTF = orig }()

	if _, err := DefaultOpenTypeFont(16); err == nil {
		t.Fatal("DefaultOpenTypeFont on junk bytes: want error, got nil")
	}
}

// UseOpenTypeText flips the active font in one call: metrics change from the
// bitmap default to the AA face, and DrawText then paints partial-coverage
// (anti-aliased) edge pixels — impossible with the on/off bitmap font.
func TestUseOpenTypeTextInstallsAAFace(t *testing.T) {
	defer SetFont(nil) // restore the bitmap default for other tests

	// Sanity: the compiled-in default is still the bitmap font.
	if GlyphHeight() != baseGlyphHeight {
		t.Fatalf("pre-opt-in GlyphHeight() = %d, want bitmap default %d", GlyphHeight(), baseGlyphHeight)
	}

	if err := UseOpenTypeText(); err != nil {
		t.Fatalf("UseOpenTypeText() error: %v", err)
	}
	if GlyphHeight() != CurrentFont().Height() || GlyphHeight() <= baseGlyphHeight {
		t.Fatalf("after opt-in GlyphHeight() = %d, want the AA face height > %d", GlyphHeight(), baseGlyphHeight)
	}

	// Render a glyph and prove at least one partial-coverage (anti-aliased)
	// pixel was produced — a strictly-between-ink-and-background value that the
	// binary bitmap font can never emit over the sentinel background.
	const w, h = 24, 24
	buf := makeSurface(w, h)
	ink := RGB(0x10, 0x20, 0x30)
	DrawText(newP(buf, w), 2, 2, "e", ink)
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	partial := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px != sentinel && px != (RGBA{R: ink.R, G: ink.G, B: ink.B, A: 0xFF}) {
				partial = true
			}
		}
	}
	if !partial {
		t.Fatal("UseOpenTypeText did not produce any anti-aliased (partial-coverage) pixel")
	}
}

// UseOpenTypeTextSize honours an explicit size: a larger size yields a taller
// line than a smaller one, and both install onto the active font.
func TestUseOpenTypeTextSizeScales(t *testing.T) {
	defer SetFont(nil)

	if err := UseOpenTypeTextSize(12); err != nil {
		t.Fatalf("UseOpenTypeTextSize(12) error: %v", err)
	}
	small := GlyphHeight()

	if err := UseOpenTypeTextSize(28); err != nil {
		t.Fatalf("UseOpenTypeTextSize(28) error: %v", err)
	}
	large := GlyphHeight()

	if !(large > small) {
		t.Fatalf("line height did not scale with size: 12px=%d 28px=%d", small, large)
	}
}

// UseOpenTypeTextSize returns the parse error and leaves the active font
// untouched when the bundled bytes are unusable (seam-injected).
func TestUseOpenTypeTextParseErrorLeavesFontUntouched(t *testing.T) {
	defer SetFont(nil)

	orig := defaultFaceTTF
	defaultFaceTTF = func() []byte { return []byte("junk") }
	defer func() { defaultFaceTTF = orig }()

	before := CurrentFont()
	if err := UseOpenTypeText(); err == nil {
		t.Fatal("UseOpenTypeText with junk default face: want error, got nil")
	}
	if CurrentFont() != before {
		t.Fatal("active font must be left untouched when the face fails to parse")
	}
}

// The opt-in and the bitmap default agree on the non-pixel painter path: both
// hand runes to a CellPainter, so a TUI keeps working after the flip.
func TestUseOpenTypeTextNonPixelPainter(t *testing.T) {
	defer SetFont(nil)
	if err := UseOpenTypeText(); err != nil {
		t.Fatalf("UseOpenTypeText() error: %v", err)
	}
	cp := painter.NewCellPainter(10, 2)
	DrawText(cp, 1, 0, "Hi", RGB(0xFF, 0xFF, 0xFF))
	if cp.Cells[0*10+1].Rune != 'H' || cp.Cells[0*10+2].Rune != 'i' {
		t.Fatalf("cells = %q%q, want \"Hi\"", cp.Cells[0*10+1].Rune, cp.Cells[0*10+2].Rune)
	}
}
