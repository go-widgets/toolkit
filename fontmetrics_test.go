// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func TestDefaultFontMetrics(t *testing.T) {
	// Out of the box the active font is the unscaled 5x7 bitmap.
	if GlyphAdvance() != baseGlyphAdvance || GlyphHeight() != baseGlyphHeight {
		t.Errorf("default metrics = (%d,%d), want (%d,%d)",
			GlyphAdvance(), GlyphHeight(), baseGlyphAdvance, baseGlyphHeight)
	}
	if CurrentFont() != defaultFont {
		t.Error("CurrentFont should start as the default font")
	}
}

func TestNewBitmapFontScaleAndClamp(t *testing.T) {
	two := NewBitmapFont(2)
	if two.Advance() != 2*baseGlyphAdvance || two.Height() != 2*baseGlyphHeight {
		t.Errorf("scale-2 metrics = (%d,%d), want doubled", two.Advance(), two.Height())
	}
	// A sub-1 scale clamps to 1 rather than producing a zero-size font.
	if got := NewBitmapFont(0).Advance(); got != baseGlyphAdvance {
		t.Errorf("scale-0 advance = %d, want clamped to %d", got, baseGlyphAdvance)
	}
}

func TestSetFontSwapsAndRestores(t *testing.T) {
	defer SetFont(nil) // restore the default for other tests
	SetFont(NewBitmapFont(3))
	if GlyphHeight() != 3*baseGlyphHeight {
		t.Errorf("after SetFont(3x), GlyphHeight = %d, want %d", GlyphHeight(), 3*baseGlyphHeight)
	}
	// A nil font restores the built-in default.
	SetFont(nil)
	if CurrentFont() != defaultFont || GlyphHeight() != baseGlyphHeight {
		t.Error("SetFont(nil) should restore the default font")
	}
}

func TestScaledFontRendersBiggerGlyph(t *testing.T) {
	defer SetFont(nil)
	// The letter 'I' at scale 1 vs scale 2: doubling the font should light up
	// noticeably more pixels (each lit bit becomes a 2x2 block).
	const w, h = 40, 40
	count := func() int {
		buf := makeSurface(w, h)
		DrawText(newP(buf, w), 1, 1, "I", RGB(0x11, 0x22, 0x33))
		return countInk(buf, w, h, RGB(0x11, 0x22, 0x33))
	}
	SetFont(nil)
	one := count()
	SetFont(NewBitmapFont(2))
	two := count()
	if two <= one {
		t.Errorf("scaled glyph lit %d px, want more than unscaled %d", two, one)
	}
	// Scale-2 blocks are 2x2, so the count should be ~4x the unscaled one.
	if two != 4*one {
		t.Errorf("scale-2 lit %d px, want 4×%d = %d", two, one, 4*one)
	}
}

func TestScaledFontCellPainterDelegates(t *testing.T) {
	defer SetFont(nil)
	// On a non-pixel painter the bitmap font delegates to Text regardless of
	// scale (the pixel-block path is skipped).
	SetFont(NewBitmapFont(2))
	cp := painter.NewCellPainter(10, 1)
	DrawText(cp, 0, 0, "Hi", RGB(0, 0, 0))
	if cp.Cells[0].Rune != 'H' {
		t.Errorf("cell painter glyph = %q, want H", cp.Cells[0].Rune)
	}
}
