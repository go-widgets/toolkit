// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// --- Base font accessors -------------------------------------------------

// EffectiveFont inherits the global active font when no per-widget Font is set,
// and returns the widget's own Font once one is assigned.
func TestBaseEffectiveFontInheritsAndOverrides(t *testing.T) {
	var b Base
	// nil override -> inherit whatever the package-level active font is.
	if b.EffectiveFont() != CurrentFont() {
		t.Fatalf("EffectiveFont with nil Font = %v, want CurrentFont %v", b.EffectiveFont(), CurrentFont())
	}
	// Assigning Font makes EffectiveFont return exactly that font.
	own := newTTF(t, 24)
	b.Font = own
	if b.EffectiveFont() != own {
		t.Fatalf("EffectiveFont with Font set = %v, want the widget's own font", b.EffectiveFont())
	}
	// The override is scoped: a swap of the GLOBAL font does not disturb it.
	SetFont(NewBitmapFont(3))
	defer SetFont(nil)
	if b.EffectiveFont() != own {
		t.Fatal("per-widget Font must survive a global SetFont")
	}
}

// SetFont sets the override, returns the Base for chaining, and clears back to
// inheritance when passed nil.
func TestBaseSetFont(t *testing.T) {
	var b Base
	own := newTTF(t, 18)
	if got := b.SetFont(own); got != &b {
		t.Fatalf("SetFont should return the Base for chaining, got %v", got)
	}
	if b.Font != own || b.EffectiveFont() != own {
		t.Fatal("SetFont(own) did not install the override")
	}
	// nil clears the override -> back to inheriting the global font.
	b.SetFont(nil)
	if b.Font != nil || b.EffectiveFont() != CurrentFont() {
		t.Fatal("SetFont(nil) did not restore inheritance")
	}
}

// The measurement + metric helpers read through the effective font: a
// proportional TrueType override reports a different (proportional) width and a
// taller line box than the monospace bitmap default, while glyphAdvance tracks
// the effective font's advance.
func TestBaseFontHelpersFollowEffectiveFont(t *testing.T) {
	var b Base

	// Inheriting the (bitmap) default: monospace, so iii == MMM in width.
	if b.textWidth("iii") != b.textWidth("MMM") {
		t.Fatal("bitmap default should measure iii and MMM equal")
	}
	if b.glyphHeight() != baseGlyphHeight || b.glyphAdvance() != baseGlyphAdvance {
		t.Fatalf("inherited metrics = (%d,%d), want (%d,%d)",
			b.glyphHeight(), b.glyphAdvance(), baseGlyphHeight, baseGlyphAdvance)
	}

	// Overriding with a proportional TrueType face.
	b.Font = newTTF(t, 24)
	if !(b.textWidth("iii") < b.textWidth("MMM")) {
		t.Fatalf("proportional override: textWidth(iii)=%d should be < textWidth(MMM)=%d",
			b.textWidth("iii"), b.textWidth("MMM"))
	}
	if b.glyphHeight() <= baseGlyphHeight {
		t.Fatalf("24px override glyphHeight=%d should exceed bitmap %d", b.glyphHeight(), baseGlyphHeight)
	}
	if b.glyphAdvance() != b.Font.Advance() {
		t.Fatalf("glyphAdvance=%d should equal effective font Advance=%d", b.glyphAdvance(), b.Font.Advance())
	}
}

// drawText paints through the effective font: an override rasterises anti-
// aliased (partial-coverage) pixels a hard bitmap can never produce.
func TestBaseDrawTextUsesEffectiveFont(t *testing.T) {
	const w, h = 32, 32
	ink := RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}

	var b Base
	b.Font = newTTF(t, 24)
	buf := makeSurface(w, h)
	b.drawText(painter.NewPixelPainter(buf, w, h), 2, 2, "M", ink)

	var partial int
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] > 0x10 && buf[i] < 0xC8 {
			partial++
		}
	}
	if partial == 0 {
		t.Fatal("Base.drawText via a TrueType override produced no anti-aliased pixels")
	}
}

// --- End-to-end proof on a real widget -----------------------------------

// A widget with a per-widget TrueType Font lays out + paints with THAT font
// (proportional, larger, anti-aliased) while a sibling with no override renders
// exactly as before — byte-for-byte identical to the global-font path.
func TestPerWidgetFontOnLabel(t *testing.T) {
	theme := DefaultLight()
	const w, h = 200, 40
	const text = "Mailbox"

	// (a) Default label (no per-widget font) — the reference render.
	def := NewLabel(text)
	def.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	defBuf := makeSurface(w, h)
	def.Draw(newP(defBuf, w), theme)

	// (b) The SAME label but with the override explicitly set to the current
	// global font must produce a byte-identical buffer — proving EffectiveFont's
	// inherit path and its explicit path are the same code path.
	explicit := NewLabel(text)
	explicit.Font = CurrentFont()
	explicit.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	expBuf := makeSurface(w, h)
	explicit.Draw(newP(expBuf, w), theme)
	if !bytes.Equal(defBuf, expBuf) {
		t.Fatal("Font==CurrentFont() must render byte-identically to Font==nil (backward compat)")
	}

	// (c) A label with a big proportional TrueType override.
	big := NewLabel(text)
	big.Font = newTTF(t, 24)
	big.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	bigBuf := makeSurface(w, h)
	big.Draw(newP(bigBuf, w), theme)

	// The override measures a DIFFERENT (proportional, larger) width...
	if big.textWidth(text) == def.textWidth(text) {
		t.Fatalf("override width %d must differ from default width %d",
			big.textWidth(text), def.textWidth(text))
	}
	if !(big.textWidth(text) > def.textWidth(text)) {
		t.Fatalf("24px override width %d should exceed 7px bitmap width %d",
			big.textWidth(text), def.textWidth(text))
	}

	// ...produces anti-aliased partial-coverage pixels (the bitmap default cannot).
	partial := func(buf []byte, on RGBA) int {
		n := 0
		for i := 0; i+3 < len(buf); i += 4 {
			// A partially-covered text pixel is neither the background sentinel
			// (0xC8) nor an exact glyph-ink match.
			px := RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
			if px != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) && px != on {
				n++
			}
		}
		return n
	}
	if got := partial(bigBuf, theme.OnSurface); got == 0 {
		t.Fatal("TrueType per-widget label produced no anti-aliased pixels")
	}
	// The default bitmap label has ZERO partial-coverage pixels: every painted
	// pixel is either background or exactly the ink (hard on/off).
	if got := partial(defBuf, theme.OnSurface); got != 0 {
		t.Fatalf("default bitmap label produced %d non-{bg,ink} pixels, want 0 (no AA)", got)
	}

	// And the two renders differ overall — the override visibly changed output.
	if bytes.Equal(defBuf, bigBuf) {
		t.Fatal("per-widget TrueType label rendered identically to the bitmap default")
	}
}

// Two widgets in the same scene can carry DIFFERENT fonts simultaneously — the
// go-news-reader use case (small badge + large title) — with neither disturbing
// the other nor the global font.
func TestTwoWidgetsDistinctFonts(t *testing.T) {
	small := &Badge{Text: "12"}
	small.Font = newTTF(t, 10)
	large := NewLabel("Headline")
	large.Font = newTTF(t, 22)

	if small.EffectiveFont() == large.EffectiveFont() {
		t.Fatal("the two widgets should hold distinct font instances")
	}
	if small.glyphHeight() >= large.glyphHeight() {
		t.Fatalf("10px badge height %d should be < 22px title height %d",
			small.glyphHeight(), large.glyphHeight())
	}
	// The global font is untouched by either override.
	if CurrentFont() != defaultFont {
		t.Fatal("per-widget fonts must not mutate the global active font")
	}
}
