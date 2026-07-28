// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"

	"github.com/go-opentype/opentype"
	"github.com/go-opentype/shape"
	"github.com/go-widgets/painter"
)

// truetypeFont renders anti-aliased, proportional text from a parsed
// TrueType/OpenType face. It is an opt-in replacement for the built-in 5x7
// bitmap font: SetFont(NewTrueTypeFont(ttf, px)) rescales the whole UI's
// typography to crisp vector glyphs without touching any widget, because
// widgets measure text through Measure/TextWidth and paint through Draw.
//
// Parsing + rasterisation are provided by github.com/go-opentype/opentype — a
// pure-Go, zero-dependency (stdlib-only) TrueType engine. The toolkit therefore
// carries NO third-party font dependency: glyph outlines are decoded and
// scan-converted to anti-aliased coverage masks entirely within the go-widgets
// ecosystem.
//
// The face and its metrics are parsed once in NewTrueTypeFont and cached; the
// face is not safe for concurrent Draw calls, matching the toolkit's
// single-threaded render model.
type truetypeFont struct {
	face    *opentype.Face
	parsed  *opentype.Font // the parsed font, for per-rune glyph-coverage queries
	advance int            // width of a space — the fallback monospace-ish step
	height  int            // line height (ascent + descent + line gap)
	ascent  int            // baseline offset from the text top
}

// covers reports whether this font has a glyph for r (used by fallbackFont to
// route each rune to a font that can render it).
func (f *truetypeFont) covers(r rune) bool {
	_, ok := f.parsed.GlyphIndex(r)
	return ok
}

// NewTrueTypeFont parses ttf (a TrueType byte blob) and returns a Font that
// renders it anti-aliased at sizePx pixels. Parse failures are wrapped and
// returned; on success the face and its metrics are cached for the font's life.
//
// Typical use pairs it with an embedded face, e.g.:
//
//	f, err := NewTrueTypeFont(myFontTTF, 16)
//	if err != nil { /* handle */ }
//	SetFont(f)
func NewTrueTypeFont(ttf []byte, sizePx int) (Font, error) {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("toolkit: parse TrueType font: %w", err)
	}
	face := parsed.NewFace(sizePx)
	m := face.Metrics()
	return &truetypeFont{
		face:    face,
		parsed:  parsed,
		advance: face.Measure(" "),
		height:  m.Height,
		ascent:  m.Ascent,
	}, nil
}

// Advance is the horizontal step of a space — a coarse monospace-ish fallback
// for callers that still assume a fixed cell (the bitmap-font contract). Real
// text layout should use Measure, which honours per-glyph widths.
func (f *truetypeFont) Advance() int { return f.advance }

// Height is the face's line height (ascent + descent + line gap), in pixels.
func (f *truetypeFont) Height() int { return f.height }

// Measure is the total rendered width of text in pixels: it shapes the run
// through the complex-text shaper (see Draw) and sums the positioned glyph
// advances, so the reported width matches what Draw paints — cursive joining,
// ligatures and kerning that change the advances are reflected. For plain
// Latin with no active GSUB/GPOS this equals the sum of per-glyph advances, so
// "iii" is still narrower than "MMM"; an empty string measures 0.
func (f *truetypeFont) Measure(text string) int {
	total := 0
	for _, g := range shape.Shape(f.face, text, shape.Options{Direction: textDirection.base()}) {
		total += g.XAdvance
	}
	return total
}

// Draw paints text anti-aliased with (x, y) as the text top-left corner (the
// toolkit convention), computing the baseline from the face ascent.
//
// On a *painter.PixelPainter it runs the complex-text shaper
// (github.com/go-opentype/shape) to turn the logical run into positioned glyphs
// in visual order — resolving bidi embedding levels, Arabic cursive joining
// (init/medi/fina/isol GSUB), ligatures, mark attachment and kerning — then
// blits each glyph by its index at (pen+XOffset, baseline-YOffset), advancing
// the pen by the shaped XAdvance. YOffset is font-space y-up (a positive value
// lifts a diacritic above the baseline), so it is subtracted in the screen's
// y-down space. This renders real Arabic joined-and-marked, unlike the previous
// per-rune presentation-form fallback. Each covered pixel is written as the ink
// colour with its alpha scaled by the mask coverage, so the painter's src-over
// PutPixel blends glyph edges into partial-coverage pixels.
//
// On any other painter (a CellPainter for a TUI, an SvgPainter for vector
// output) pixel coverage — and glyph indices — are meaningless, so it reorders
// the text into visual order (bidi only, no GID shaping) and delegates to that
// painter's own rune-based Text primitive, mirroring the bitmap font.
//
// Glyphs the font cannot map (control characters, unassigned code points shape
// to .notdef, index 0) are not painted, matching the bitmap font's
// blank-for-unknown behaviour.
func (f *truetypeFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	pix, isPixel := p.(*painter.PixelPainter)
	if !isPixel {
		// Non-pixel painter: no glyph-index blitting is possible, so fall back
		// to the bidi visual-order reorder and hand runes to the painter.
		p.Text(x, y, visualText(text), ink)
		return
	}
	f.drawShaped(pix, x, y+f.ascent, text, ink)
}

// drawShaped shapes + blits text with an explicit baseline (so a fallback chain
// can align runs from different faces to one common baseline). Returns the pen
// advance.
func (f *truetypeFont) drawShaped(pix *painter.PixelPainter, x, baseline int, text string, ink RGBA) int {
	pen := x
	for _, g := range shape.Shape(f.face, text, shape.Options{Direction: textDirection.base()}) {
		// .notdef (index 0) is the shaper's blank-for-unknown; every other
		// index the shaper emits is a valid, renderable glyph of this font.
		if g.GID != 0 {
			dr, mask, maskp, _, _ := f.face.GlyphMaskIndex(g.GID, pen+g.XOffset, baseline-g.YOffset)
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
		}
		pen += g.XAdvance
	}
	return pen
}
