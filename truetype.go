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
	data    []byte         // the original sfnt bytes, retained for the painter.Face seam
	sizePx  int            // the em size the face was built at, in pixels
	advance int            // width of a space — the fallback monospace-ish step
	height  int            // line height (ascent + descent + line gap)
	ascent  int            // baseline offset from the text top

	// shapeCache holds shaped runs; see shaped.
	shapeCache map[shapedKey][]shape.Glyph
}

// shapedKey identifies a shaped run. The shaper's output depends on the face,
// the string and the paragraph direction -- and on nothing else, in particular
// not on where the run is painted -- so those are the whole key. The face is
// implicit: the cache hangs off the font.
type shapedKey struct {
	text string
	dir  TextDirection
}

// maxShapedRuns bounds the cache. A UI redraws a small, stable set of labels,
// but a text editor or a browser can feed it unbounded distinct strings, so it
// is emptied wholesale when it fills rather than being allowed to grow. Dropping
// everything costs one re-shape of the runs still on screen and needs no
// bookkeeping per entry, which an LRU would.
const maxShapedRuns = 512

// shaped returns the positioned glyphs for text, shaping it only the first time
// it is seen. Shaping a 43-character label costs about 6 microseconds and 25
// allocations totalling 7.8 KB, and a repaint re-shapes every label on screen:
// the allocation, not the arithmetic, is what makes text expensive, because it
// is the garbage collector that ends up paying.
//
// The returned slice is the cache's own. Callers read it and must not write to
// it.
func (f *truetypeFont) shaped(text string) []shape.Glyph {
	k := shapedKey{text: text, dir: textDirection}
	if g, ok := f.shapeCache[k]; ok {
		return g
	}
	g := shape.Shape(f.face, text, shape.Options{Direction: textDirection.base()})
	if f.shapeCache == nil {
		f.shapeCache = make(map[shapedKey][]shape.Glyph)
	}
	if len(f.shapeCache) >= maxShapedRuns {
		clear(f.shapeCache)
	}
	f.shapeCache[k] = g
	return g
}

// FontData returns the original TrueType/OpenType sfnt bytes, implementing
// painter.Face so a vector back-end can embed the true font for selectable text.
// The slice is the caller's original blob; it must not be mutated.
func (f *truetypeFont) FontData() []byte { return f.data }

// SizePx returns the em size the face was built at, in pixels — the size widgets
// laid their text out against. Implements painter.Face.
func (f *truetypeFont) SizePx() int { return f.sizePx }

// Ascent returns the baseline offset from the text top, in pixels, so a
// baseline-origin back-end (PDF) can place the run from the toolkit's top-left
// convention. Implements painter.Face.
func (f *truetypeFont) Ascent() int { return f.ascent }

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
		data:    ttf,
		sizePx:  sizePx,
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
	for _, g := range f.shaped(text) {
		total += g.XAdvance
	}
	return total
}

// invisible reports whether the shaper marked g as not to be painted. Kept as a
// named helper so Draw and any future back-end apply the same rule.
func invisible(g shape.Glyph) bool { return g.GID == 0 || g.Invisible }

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
// On any other painter (a CellPainter for a TUI, an SvgPainter or a PDF vector
// painter) pixel coverage — and glyph masks — are meaningless, so the text is
// reordered into visual order (bidi only, no GID blitting) and handed off:
//
//   - A painter.FacePainter (a vector/recording back-end that can embed a font)
//     receives the run PLUS this face, so it emits real, selectable text in the
//     true font at the true size — go-pdfkit turns a TrueType-font widget label
//     into selectable PDF text this way, instead of a rasterised image.
//   - Any other painter falls back to the plain rune-based Text primitive
//     (one rune per cell for a TUI, the painter's own font for a plain vector
//     back-end), mirroring the bitmap font.
//
// Glyphs the font cannot map (control characters, unassigned code points shape
// to .notdef, index 0) are not painted, matching the bitmap font's
// blank-for-unknown behaviour.
func (f *truetypeFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	// A face-aware back-end embeds this face and renders real, selectable text,
	// which beats any rasterisation, so it is asked first even if it can also
	// take a mask.
	if fp, ok := p.(painter.FacePainter); ok {
		fp.TextFace(x, y, visualText(text), f, ink)
		return
	}
	// A cell grid is asked BEFORE the mask capability, not after: a CellPainter
	// does implement painter.MaskPainter, but a 16-pixel glyph spread over 16
	// rows of cells is not text. A terminal's own Text primitive is its real
	// renderer, one rune per cell.
	if _, isCell := p.(*painter.CellPainter); !isCell {
		if mp, ok := p.(painter.MaskPainter); ok {
			f.drawShaped(mp, x, y+f.ascent, text, ink)
			return
		}
	}
	p.Text(x, y, visualText(text), ink)
}

// drawShaped shapes + blits text with an explicit baseline (so a fallback chain
// can align runs from different faces to one common baseline). Returns the pen
// advance.
func (f *truetypeFont) drawShaped(mp painter.MaskPainter, x, baseline int, text string, ink RGBA) int {
	pen := x
	for _, g := range f.shaped(text) {
		// .notdef (index 0) is the shaper's blank-for-unknown; every other
		// index the shaper emits is a valid, renderable glyph of this font.
		// Invisible marks a default-ignorable the shaper did not consume — a
		// leftover joiner, variation selector or soft hyphen. It already carries a
		// zero advance, but the font may well map it to a real glyph (Go Regular
		// draws a hyphen for U+00AD), so it has to be skipped explicitly or it
		// would stamp itself on top of the next letter.
		if !invisible(g) {
			dr, mask, maskp, _, ok := f.face.GlyphMaskIndex(g.GID, pen+g.XOffset, baseline-g.YOffset)
			// A glyph with an advance but no coverage -- a space -- comes back
			// with no mask at all. The loop this replaced never noticed, because
			// an empty rectangle simply never entered its body; handing the
			// rectangle to a primitive means saying so explicitly.
			if !ok || mask == nil || dr.Empty() {
				pen += g.XAdvance
				continue
			}
			// The glyph is a window into the face's mask, so the painter is
			// handed that window -- the sub-slice starting at the glyph's own
			// corner, plus the atlas stride -- rather than a copy of it.
			mp.DrawMask(
				Rect{X: dr.Min.X, Y: dr.Min.Y, W: dr.Dx(), H: dr.Dy()},
				mask.Pix[mask.PixOffset(maskp.X, maskp.Y):], mask.Stride, ink)
		}
		pen += g.XAdvance
	}
	return pen
}
