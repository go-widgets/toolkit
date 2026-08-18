// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-typeset/bidi"
	"github.com/go-widgets/painter"
)

// Hebrew logical string (Alef, Bet, Gimel) and its reversed visual form.
const (
	hebAlef = 'א'
	hebBet  = 'ב'
	hebGim  = 'ג'
)

// cellRunes reads the runes written to the first len cells of row 0 of a
// CellPainter, so a text primitive's visual output can be asserted at
// cell precision (the CellPainter lays one rune per cell left-to-right).
func cellRunes(cp *painter.CellPainter, x, n int) []rune {
	out := make([]rune, n)
	for i := 0; i < n; i++ {
		out[i] = cp.Cells[0*cp.W+x+i].Rune
	}
	return out
}

func eqRunes(a []rune, b ...rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- direction accessors -------------------------------------------------

// SetTextDirection round-trips through CurrentTextDirection; the default is
// DirLTR.
func TestTextDirectionAccessors(t *testing.T) {
	if CurrentTextDirection() != DirLTR {
		t.Fatalf("default direction = %v, want DirLTR", CurrentTextDirection())
	}
	SetTextDirection(DirRTL)
	defer SetTextDirection(DirLTR)
	if CurrentTextDirection() != DirRTL {
		t.Fatalf("after SetTextDirection(DirRTL) = %v, want DirRTL", CurrentTextDirection())
	}
	SetTextDirection(DirAuto)
	if CurrentTextDirection() != DirAuto {
		t.Fatalf("after SetTextDirection(DirAuto) = %v, want DirAuto", CurrentTextDirection())
	}
}

// base() maps each toolkit direction onto the right bidi.Direction, including
// the default (DirLTR) branch.
func TestTextDirectionBase(t *testing.T) {
	if DirLTR.base() != bidi.LeftToRight {
		t.Fatalf("DirLTR.base() = %v, want LeftToRight", DirLTR.base())
	}
	if DirRTL.base() != bidi.RightToLeft {
		t.Fatalf("DirRTL.base() = %v, want RightToLeft", DirRTL.base())
	}
	if DirAuto.base() != bidi.Auto {
		t.Fatalf("DirAuto.base() = %v, want Auto", DirAuto.base())
	}
}

// hasRTL flags the strong-RTL classes (R, AL) and Arabic numbers (AN), and
// clears for pure LTR.
func TestHasRTL(t *testing.T) {
	if !hasRTL(string(hebAlef)) { // Hebrew -> class R
		t.Fatal("hasRTL(Hebrew) = false, want true")
	}
	if !hasRTL("ب") { // Arabic Beh -> class AL
		t.Fatal("hasRTL(Arabic letter) = false, want true")
	}
	if !hasRTL("٠") { // Arabic-Indic digit zero -> class AN
		t.Fatal("hasRTL(Arabic number) = false, want true")
	}
	if hasRTL("Hello, World!") {
		t.Fatal("hasRTL(Latin) = true, want false")
	}
}

// --- visual reordering ---------------------------------------------------

// A Hebrew string is laid out in reversed (right-to-left) visual order: the
// first logical letter lands rightmost. Asserted at cell precision through the
// active-font DrawText façade.
func TestDrawTextHebrewReversedVisualOrder(t *testing.T) {
	SetTextDirection(DirRTL)
	defer SetTextDirection(DirLTR)

	cp := painter.NewCellPainter(8, 1)
	DrawText(cp, 0, 0, string([]rune{hebAlef, hebBet, hebGim}), RGB(0xFF, 0xFF, 0xFF))

	got := cellRunes(cp, 0, 3)
	if !eqRunes(got, hebGim, hebBet, hebAlef) {
		t.Fatalf("Hebrew visual order = %U, want reversed [Gimel Bet Alef]", got)
	}
}

// Mixed "abc <hebrew> def": the Latin runs stay left-to-right while the Hebrew
// run is reversed in place, under a left-to-right base direction.
func TestDrawTextMixedBidiKeepsLatinLTR(t *testing.T) {
	// Default DirLTR: base is left-to-right, first strong char is Latin.
	cp := painter.NewCellPainter(16, 1)
	logical := "abc " + string([]rune{hebAlef, hebBet, hebGim}) + " def"
	DrawText(cp, 0, 0, logical, RGB(0xFF, 0xFF, 0xFF))

	got := cellRunes(cp, 0, 11)
	want := []rune{'a', 'b', 'c', ' ', hebGim, hebBet, hebAlef, ' ', 'd', 'e', 'f'}
	if !eqRunes(got, want...) {
		t.Fatalf("mixed visual order = %U\n want                = %U", got, want)
	}
}

// DirAuto resolves the base direction from the first strong character: a
// Hebrew-led string is arranged right-to-left even without an explicit DirRTL.
func TestDrawTextAutoResolvesRTLBase(t *testing.T) {
	SetTextDirection(DirAuto)
	defer SetTextDirection(DirLTR)

	cp := painter.NewCellPainter(8, 1)
	DrawText(cp, 0, 0, string([]rune{hebAlef, hebBet, hebGim}), RGB(0xFF, 0xFF, 0xFF))
	if got := cellRunes(cp, 0, 3); !eqRunes(got, hebGim, hebBet, hebAlef) {
		t.Fatalf("auto RTL visual order = %U, want reversed", got)
	}
}

// An Arabic word picks contextual presentation forms: the middle letter of a
// three-letter run resolves to its MEDIAL form (Beh medial = U+FE92), proving
// the JoinForms + PresentationForm fallback runs.
func TestDrawTextArabicMedialForm(t *testing.T) {
	SetTextDirection(DirRTL)
	defer SetTextDirection(DirLTR)

	const beh = 'ب'
	cp := painter.NewCellPainter(8, 1)
	DrawText(cp, 0, 0, string([]rune{beh, beh, beh}), RGB(0xFF, 0xFF, 0xFF))

	// Visual order (RTL) is [final, medial, initial]; the middle cell is medial.
	got := cellRunes(cp, 0, 3)
	if !eqRunes(got, 'ﺐ', 'ﺒ', 'ﺑ') {
		t.Fatalf("Arabic forms = %U, want [final FE90, medial FE92, initial FE91]", got)
	}
}

// Rule L4: paired punctuation inside a right-to-left run is mirrored — an
// opening "(" becomes a closing ")".
func TestDrawTextRTLMirrorsBrackets(t *testing.T) {
	SetTextDirection(DirRTL)
	defer SetTextDirection(DirLTR)

	cp := painter.NewCellPainter(8, 1)
	DrawText(cp, 0, 0, string([]rune{hebAlef, '('}), RGB(0xFF, 0xFF, 0xFF))
	if got := cellRunes(cp, 0, 2); !eqRunes(got, ')', hebAlef) {
		t.Fatalf("RTL mirror = %U, want [')' Alef]", got)
	}
}

// --- LTR regression (byte-identity) --------------------------------------

// Under the default DirLTR, visualText returns an all-LTR string byte-for-byte
// unchanged, so Latin/CJK rendering is identical to the pre-bidi toolkit.
func TestVisualTextLTRByteIdentical(t *testing.T) {
	for _, s := range []string{"", "Hello, World!", "MMM iii 123", "日本語のテキスト"} {
		if got := visualText(s); got != s {
			t.Fatalf("visualText(%q) = %q, want byte-identical", s, got)
		}
	}
}

// End-to-end pixel regression: an all-Latin string rendered through a TrueType
// active font produces the exact same pixel buffer as it did before bidi
// (the fast path leaves the text untouched, so the glyph loop is unchanged).
// The buffer is compared against a direct, bidi-free render of the same text.
func TestDrawTextLTRPixelUnchanged(t *testing.T) {
	tt := newTTF(t, 16)
	SetFont(tt)
	defer SetFont(nil)

	const w, h = 64, 24
	const s = "Ag1!"

	// Reference: render the (LTR) text directly, bypassing visualText — this is
	// what the pre-bidi Draw loop produced.
	ref := makeSurface(w, h)
	rp := painter.NewPixelPainter(ref, w, h)
	drawGlyphsRaw(tt.(*truetypeFont), rp, 2, 2, s, RGB(0x10, 0x20, 0x30))

	// Through the full DrawText + bidi seam under the default DirLTR.
	got := makeSurface(w, h)
	DrawText(painter.NewPixelPainter(got, w, h), 2, 2, s, RGB(0x10, 0x20, 0x30))

	for i := range got {
		if got[i] != ref[i] {
			t.Fatalf("LTR pixel mismatch at byte %d: got %#x want %#x", i, got[i], ref[i])
		}
	}
}

// drawGlyphsRaw replays the truetypeFont pixel loop on already-visual text,
// i.e. exactly the pre-bidi Draw body. It is the bidi-free oracle for the LTR
// pixel-identity regression above.
func drawGlyphsRaw(f *truetypeFont, p painter.Painter, x, y int, text string, ink RGBA) {
	pix := p.(*painter.PixelPainter)
	baseline := y + f.ascent
	pen := x
	for _, r := range text {
		dr, mask, maskp, advance, ok := f.face.GlyphMask(r, pen, baseline)
		if !ok {
			continue
		}
		for j := 0; j < dr.Dy(); j++ {
			for i := 0; i < dr.Dx(); i++ {
				a := mask.AlphaAt(maskp.X+i, maskp.Y+j).A
				if a == 0 {
					continue
				}
				alpha := uint8(uint32(a) * uint32(ink.A) / 255)
				pix.PutPixel(dr.Min.X+i, dr.Min.Y+j, RGBA{R: ink.R, G: ink.G, B: ink.B, A: alpha})
			}
		}
		pen += advance
	}
}
