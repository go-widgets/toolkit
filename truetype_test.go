// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/go-opentype/bidi"
	"github.com/go-opentype/fonts/inter"
	"github.com/go-opentype/fonts/notosansarabic"
	"github.com/go-opentype/shape"
	"github.com/go-widgets/painter"
)

// testFontTTF is a real TrueType face (Go Regular, Bigelow & Holmes, under a
// BSD-3-style license — see testdata/goregular.ttf) embedded so the TrueType
// tests exercise go-opentype's parser + rasteriser on a genuine multi-table
// font (712 glyphs, composite glyphs, real cmap) without any third-party
// module dependency.
//
//go:embed testdata/goregular.ttf
var testFontTTF []byte

// newTTF builds a TrueType font from the embedded Go Regular face at the given
// pixel size, failing the test on any parse error.
func newTTF(t *testing.T, px int) Font {
	t.Helper()
	f, err := NewTrueTypeFont(testFontTTF, px)
	if err != nil {
		t.Fatalf("NewTrueTypeFont(goregular, %d) error: %v", px, err)
	}
	return f
}

// --- constructor ---------------------------------------------------------

// A valid TTF parses and yields sane, strictly-positive metrics.
func TestNewTrueTypeFontMetrics(t *testing.T) {
	f := newTTF(t, 16)
	if f.Advance() <= 0 {
		t.Fatalf("Advance() = %d, want > 0", f.Advance())
	}
	if f.Height() <= 0 {
		t.Fatalf("Height() = %d, want > 0", f.Height())
	}
	// A 16px face should stand clearly taller than the 7px bitmap font.
	if f.Height() < 8 {
		t.Fatalf("Height() = %d, want a real vector line height", f.Height())
	}
}

// A malformed blob is rejected with a wrapped error, not a panic.
func TestNewTrueTypeFontParseError(t *testing.T) {
	f, err := NewTrueTypeFont([]byte("this is not a font"), 16)
	if err == nil {
		t.Fatal("NewTrueTypeFont on junk bytes: want error, got nil")
	}
	if f != nil {
		t.Fatalf("NewTrueTypeFont on junk bytes: want nil Font, got %v", f)
	}
}

// --- proportional measurement -------------------------------------------

// The defining property of a proportional font: narrow glyphs measure less
// than wide ones. The bitmap font, being monospace, measures them equal.
func TestTrueTypeMeasureIsProportional(t *testing.T) {
	tt := newTTF(t, 16)
	narrow := tt.Measure("iii")
	wide := tt.Measure("MMM")
	if !(narrow < wide) {
		t.Fatalf("proportional: Measure(iii)=%d should be < Measure(MMM)=%d", narrow, wide)
	}
	if tt.Measure("") != 0 {
		t.Fatalf("Measure(\"\") = %d, want 0", tt.Measure(""))
	}

	// The bitmap font is monospace: the same two strings measure equal.
	bm := NewBitmapFont(1)
	if bm.Measure("iii") != bm.Measure("MMM") {
		t.Fatalf("bitmap monospace: Measure(iii)=%d != Measure(MMM)=%d",
			bm.Measure("iii"), bm.Measure("MMM"))
	}
	if bm.Measure("abcd") != 4*bm.Advance() {
		t.Fatalf("bitmap Measure(abcd)=%d, want %d", bm.Measure("abcd"), 4*bm.Advance())
	}
}

// TextWidth follows the active font through SetFont: proportional under
// TrueType, monospace-equal under the bitmap default.
func TestTextWidthFollowsActiveFont(t *testing.T) {
	// Bitmap default: "iii" and "MMM" are the same width.
	if TextWidth("iii") != TextWidth("MMM") {
		t.Fatalf("bitmap TextWidth(iii)=%d != TextWidth(MMM)=%d", TextWidth("iii"), TextWidth("MMM"))
	}
	SetFont(newTTF(t, 16))
	defer SetFont(nil)
	if !(TextWidth("iii") < TextWidth("MMM")) {
		t.Fatalf("truetype TextWidth(iii)=%d should be < TextWidth(MMM)=%d",
			TextWidth("iii"), TextWidth("MMM"))
	}
}

// --- anti-aliased rasterisation -----------------------------------------

// The core AA proof: rendering an opaque glyph over a flat background must
// produce partial-coverage pixels — values strictly between ink and background
// — which a hard on/off bitmap can never do.
func TestTrueTypeDrawIsAntiAliased(t *testing.T) {
	const w, h = 24, 24
	buf := makeSurface(w, h) // 0xC8 grey background
	ink := RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}
	p := painter.NewPixelPainter(buf, w, h)

	tt := newTTF(t, 16)
	tt.Draw(p, 2, 2, "M", ink)

	var inked, partial int
	for i := 0; i+3 < len(buf); i += 4 {
		r := buf[i]
		switch {
		case r == 0x10: // fully inked (coverage == 255)
			inked++
		case r > 0x10 && r < 0xC8: // partial coverage: blended ink<->bg
			partial++
		}
	}
	if inked == 0 {
		t.Fatal("no fully-inked pixels: glyph did not render")
	}
	if partial == 0 {
		t.Fatal("no partial-coverage pixels: rendering is not anti-aliased")
	}
	t.Logf("AA proof: fully-inked=%d partial-coverage=%d", inked, partial)
}

// Ink alpha scales the coverage: a half-transparent ink lands lighter than the
// opaque ink over the same background, exercising the alpha-scaling path.
func TestTrueTypeDrawHonoursInkAlpha(t *testing.T) {
	const w, h = 24, 24
	tt := newTTF(t, 16)

	maxInk := func(buf []byte) int {
		best := 0xC8
		for i := 0; i+3 < len(buf); i += 4 {
			if int(buf[i]) < best {
				best = int(buf[i])
			}
		}
		return best // darkest (most-inked) red channel reached
	}

	opaque := makeSurface(w, h)
	tt.Draw(painter.NewPixelPainter(opaque, w, h), 2, 2, "M", RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})

	half := makeSurface(w, h)
	tt.Draw(painter.NewPixelPainter(half, w, h), 2, 2, "M", RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0x80})

	if !(maxInk(half) > maxInk(opaque)) {
		t.Fatalf("half-alpha darkest=%d should be lighter (greater) than opaque darkest=%d",
			maxInk(half), maxInk(opaque))
	}
}

// A rune the face cannot map (a control character) is skipped without panicking
// and without inking any pixels, mirroring the bitmap font's blank-for-unknown.
func TestTrueTypeDrawSkipsUnmappedRune(t *testing.T) {
	const w, h = 24, 24
	buf := makeSurface(w, h)
	ink := RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}
	tt := newTTF(t, 16)
	tt.Draw(painter.NewPixelPainter(buf, w, h), 2, 2, "\n", ink) // '\n' -> ok=false
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 0xC8 {
			t.Fatalf("unmapped rune inked pixel: R=%#x", buf[i])
		}
	}
}

// A blank glyph with an empty bounding box (the space) draws nothing and does
// not panic — the mask-iteration loops simply run zero times.
func TestTrueTypeDrawBlankGlyph(t *testing.T) {
	const w, h = 24, 24
	buf := makeSurface(w, h)
	tt := newTTF(t, 16)
	tt.Draw(painter.NewPixelPainter(buf, w, h), 2, 2, " ", RGBA{R: 1, G: 2, B: 3, A: 0xFF})
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 0xC8 {
			t.Fatalf("space inked a pixel: R=%#x", buf[i])
		}
	}
}

// On a non-Pixel painter, Draw delegates to the painter's own Text primitive
// (pixel coverage being meaningless there), just like the bitmap font.
func TestTrueTypeDrawDelegatesOnNonPixelPainter(t *testing.T) {
	cp := painter.NewCellPainter(10, 2)
	tt := newTTF(t, 16)
	tt.Draw(cp, 2, 0, "OK", RGB(0xFF, 0xFF, 0xFF))
	if cp.Cells[0*10+2].Rune != 'O' {
		t.Fatalf("first cell rune = %q, want 'O'", cp.Cells[0*10+2].Rune)
	}
	if cp.Cells[0*10+3].Rune != 'K' {
		t.Fatalf("second cell rune = %q, want 'K'", cp.Cells[0*10+3].Rune)
	}
}

// End-to-end through the toolkit's SetFont / DrawText façade: swapping in a
// TrueType font routes DrawText through it and produces AA output, proving the
// whole-UI font swap works without touching a widget.
func TestSetFontTrueTypeThenDrawText(t *testing.T) {
	SetFont(newTTF(t, 16))
	defer SetFont(nil)

	const w, h = 24, 24
	buf := makeSurface(w, h)
	DrawText(painter.NewPixelPainter(buf, w, h), 2, 2, "A", RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
	partial := 0
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] > 0x10 && buf[i] < 0xC8 {
			partial++
		}
	}
	if partial == 0 {
		t.Fatal("DrawText via TrueType active font produced no anti-aliased pixels")
	}
}

// --- complex-text shaping (github.com/go-opentype/shape) ------------------

// newTTFace builds the concrete *truetypeFont (not the Font interface) so a
// shaping test can reach the underlying *opentype.Face for a naive, unshaped
// reference render.
func newTTFace(t *testing.T, ttf []byte, px int) *truetypeFont {
	t.Helper()
	f, err := NewTrueTypeFont(ttf, px)
	if err != nil {
		t.Fatalf("NewTrueTypeFont: %v", err)
	}
	return f.(*truetypeFont)
}

// drawIsolated blits each rune's isolated (cmap) glyph strictly left-to-right
// with no shaping — no cursive joining, no reordering, no marks. It reproduces
// the pre-shaper best-effort layout and is the negative oracle: correct
// shaping of a cursive Arabic word must NOT match it.
func drawIsolated(f *truetypeFont, p *painter.PixelPainter, x, y int, text string, ink RGBA) {
	baseline := y + f.ascent
	pen := x
	for _, r := range text {
		dr, mask, maskp, advance, ok := f.face.GlyphMask(r, pen, baseline)
		if ok && mask != nil {
			for j := 0; j < dr.Dy(); j++ {
				for i := 0; i < dr.Dx(); i++ {
					a := mask.AlphaAt(maskp.X+i, maskp.Y+j).A
					if a == 0 {
						continue
					}
					alpha := uint8(uint32(a) * uint32(ink.A) / 255)
					p.PutPixel(dr.Min.X+i, dr.Min.Y+j, RGBA{R: ink.R, G: ink.G, B: ink.B, A: alpha})
				}
			}
		}
		pen += advance
	}
}

// A real Arabic word renders through the complex shaper: its cursive joining
// forms, ccmp dot decomposition and GPOS mark placement make the picture differ
// from the naive isolated-glyph layout, and at least one glyph is lifted off
// the baseline (an attached dot), proving mark positioning ran.
func TestTrueTypeArabicIsShapedNotIsolated(t *testing.T) {
	SetTextDirection(DirRTL)
	defer SetTextDirection(DirLTR)

	f := newTTFace(t, notosansarabic.TTF, 40)
	const word = "بيت" // bayt: three cursive-joining letters, each dotted
	const w, h = 240, 80

	shaped := makeSurface(w, h)
	f.Draw(painter.NewPixelPainter(shaped, w, h), 6, 6, word, RGB(0x10, 0x20, 0x30))

	isolated := makeSurface(w, h)
	drawIsolated(f, painter.NewPixelPainter(isolated, w, h), 6, 6, word, RGB(0x10, 0x20, 0x30))

	if bytes.Equal(shaped, isolated) {
		t.Fatal("shaped Arabic is byte-identical to the isolated-glyph layout: shaping did not run")
	}
	if bytes.Equal(shaped, makeSurface(w, h)) {
		t.Fatal("shaped Arabic produced an empty buffer")
	}

	marked := false
	for _, g := range shape.Shape(f.face, word, shape.Options{Direction: bidi.RightToLeft}) {
		if g.YOffset != 0 {
			marked = true
		}
	}
	if !marked {
		t.Fatal("no glyph carries a non-zero YOffset: Arabic marks were not positioned")
	}
}

// Mixed Latin+Arabic under a left-to-right base: the Latin letter stays
// leftmost (cluster 0 first), while the trailing Arabic run is laid out
// right-to-left — its source clusters read in non-increasing order — which a
// naive left-to-right rune loop can never produce.
func TestTrueTypeMixedBidiShaping(t *testing.T) {
	f := newTTFace(t, notosansarabic.TTF, 32)
	glyphs := shape.Shape(f.face, "aبت", shape.Options{Direction: bidi.LeftToRight})
	if len(glyphs) < 3 {
		t.Fatalf("expected several glyphs, got %d", len(glyphs))
	}
	if glyphs[0].Cluster != 0 {
		t.Fatalf("first (leftmost) glyph cluster = %d, want 0 (Latin stays LTR-left)", glyphs[0].Cluster)
	}
	sawReversal := false
	prev := glyphs[1].Cluster
	for _, g := range glyphs[2:] {
		if g.Cluster > prev {
			t.Fatalf("Arabic run not right-to-left: cluster %d follows %d", g.Cluster, prev)
		}
		if g.Cluster < prev {
			sawReversal = true
		}
		prev = g.Cluster
	}
	if !sawReversal {
		t.Fatal("no right-to-left cluster reversal in the Arabic run")
	}
}

// Kerning changes the width: the shaped Measure of a negative-kern Latin pair
// ("AV") is strictly narrower than the sum of the two isolated advances, and
// TextWidth reports the same shaped width through the active font.
func TestTrueTypeMeasureAppliesKerning(t *testing.T) {
	f := newTTFace(t, inter.TTF, 64)
	shaped := f.Measure("AV")
	naive := f.face.Advance('A') + f.face.Advance('V')
	if !(shaped < naive) {
		t.Fatalf("kerned Measure(\"AV\")=%d should be < naive advance sum %d", shaped, naive)
	}

	SetFont(f)
	defer SetFont(nil)
	if TextWidth("AV") != shaped {
		t.Fatalf("TextWidth(\"AV\")=%d != Measure(\"AV\")=%d", TextWidth("AV"), shaped)
	}
}

// Pure-LTR Latin stays byte-for-byte identical through the shaper: an all-LTR
// run with no active GSUB/GPOS reproduces the plain advance-width layout, so
// the shaped Draw matches the naive per-rune isolated render exactly. (This
// complements the goregular regression in bidi_test.go on a second face.)
func TestTrueTypeLTRShapedByteIdentical(t *testing.T) {
	f := newTTFace(t, testFontTTF, 16)
	const w, h = 96, 24
	const s = "Widget 42!"

	shaped := makeSurface(w, h)
	f.Draw(painter.NewPixelPainter(shaped, w, h), 2, 2, s, RGB(0x10, 0x20, 0x30))

	isolated := makeSurface(w, h)
	drawIsolated(f, painter.NewPixelPainter(isolated, w, h), 2, 2, s, RGB(0x10, 0x20, 0x30))

	if !bytes.Equal(shaped, isolated) {
		t.Fatal("LTR Latin shaped render differs from the naive isolated layout: byte-identity broken")
	}
}
