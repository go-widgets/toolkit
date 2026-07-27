// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"image"

	"github.com/go-widgets/painter"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// truetypeFont renders anti-aliased, proportional text from a parsed
// TrueType/OpenType face. It is an opt-in replacement for the built-in 5x7
// bitmap font: SetFont(NewTrueTypeFont(ttf, px)) rescales the whole UI's
// typography to crisp vector glyphs without touching any widget, because
// widgets measure text through Measure/TextWidth and paint through Draw.
//
// Rasterisation uses x/image/font/opentype's coverage masks: each glyph pixel
// carries an 8-bit alpha that is composited onto the destination via the
// painter's src-over PutPixel, so edges land as partial-coverage pixels (true
// anti-aliasing) rather than the bitmap font's hard on/off blocks.
//
// The face and its metrics are parsed once in NewTrueTypeFont and cached; the
// underlying x/image face is not safe for concurrent Draw calls, matching the
// toolkit's single-threaded render model.
type truetypeFont struct {
	face    font.Face
	advance int // width of a space — the fallback monospace-ish step
	height  int // line height (ascent + descent + line gap)
	ascent  int // baseline offset from the text top
}

// NewTrueTypeFont parses ttf (a TrueType/OpenType byte blob) and returns a Font
// that renders it anti-aliased at sizePx pixels. Parse failures are wrapped and
// returned; on success the face and its metrics are cached for the font's life.
//
// Typical use pairs it with an embedded face, e.g.:
//
//	f, err := NewTrueTypeFont(goregular.TTF, 16)
//	if err != nil { /* handle */ }
//	SetFont(f)
func NewTrueTypeFont(ttf []byte, sizePx int) (Font, error) {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("toolkit: parse TrueType font: %w", err)
	}
	// opentype.NewFace cannot fail for a successfully parsed font with these
	// options (it only allocates + scales), so its error is discarded to keep
	// the constructor branch-free; the signature is honoured for forward
	// compatibility with x/image.
	face, _ := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    float64(sizePx),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	m := face.Metrics()
	return &truetypeFont{
		face:    face,
		advance: font.MeasureString(face, " ").Ceil(),
		height:  m.Height.Ceil(),
		ascent:  m.Ascent.Ceil(),
	}, nil
}

// Advance is the horizontal step of a space — a coarse monospace-ish fallback
// for callers that still assume a fixed cell (the bitmap-font contract). Real
// text layout should use Measure, which honours per-glyph widths.
func (f *truetypeFont) Advance() int { return f.advance }

// Height is the face's line height (ascent + descent + line gap), in pixels.
func (f *truetypeFont) Height() int { return f.height }

// Measure is the total rendered width of text in pixels, summing each glyph's
// proportional advance (so "iii" is narrower than "MMM").
func (f *truetypeFont) Measure(text string) int {
	return font.MeasureString(f.face, text).Ceil()
}

// Draw paints text anti-aliased with (x, y) as the text top-left corner (the
// toolkit convention), computing the baseline from the face ascent.
//
// On a *painter.PixelPainter it rasterises each rune to an alpha coverage mask
// and composites it: every covered pixel is written as the ink colour with its
// alpha scaled by the mask coverage, so the painter's src-over PutPixel blends
// glyph edges into partial-coverage pixels. On any other painter (a CellPainter
// for a TUI, an SvgPainter for vector output) pixel coverage is meaningless, so
// it delegates to that painter's own Text primitive, mirroring the bitmap font.
//
// Runes the face cannot map (control characters, unassigned code points) are
// skipped, matching the bitmap font's blank-for-unknown behaviour.
func (f *truetypeFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	pix, isPixel := p.(*painter.PixelPainter)
	if !isPixel {
		p.Text(x, y, text, ink)
		return
	}
	baseline := fixed.I(y + f.ascent)
	pen := fixed.I(x)
	for _, r := range text {
		dot := fixed.Point26_6{X: pen, Y: baseline}
		dr, mask, maskp, advance, ok := f.face.Glyph(dot, r)
		if !ok {
			// Unmapped rune: nothing to paint and no advance to trust.
			continue
		}
		am := mask.(*image.Alpha)
		for j := 0; j < dr.Dy(); j++ {
			for i := 0; i < dr.Dx(); i++ {
				a := am.AlphaAt(maskp.X+i, maskp.Y+j).A
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
