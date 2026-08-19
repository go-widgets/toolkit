// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"

	"github.com/go-widgets/painter"
)

// syntheticItalicNum / syntheticItalicDen give the faux-italic shear as a
// rational slope: a raster row that sits d pixels above the baseline is pushed
// right by d*num/den, so the top of a glyph leans forward over its base. A
// quarter-pixel-per-row slope reads as italic at UI sizes without tipping the
// stems into illegibility.
const (
	syntheticItalicNum = 1
	syntheticItalicDen = 4
)

// syntheticItalicFont fakes an oblique (italic) weight by rendering the base run
// to an offscreen buffer and blitting it back one row at a time, each row shifted
// right in proportion to its height above the baseline — a true per-row shear.
//
// It exists for the same reason as [syntheticBoldFont]: the toolkit's built-in
// 5x7 bitmap face ships a single upright instance and no italic the rasteriser
// can reach, so an editor that wants to distinguish *emphasis* has to synthesise
// the slant. Rendering through the base face means it slants whatever the face
// rasterises (the bitmap font or a TrueType face alike); only a non-pixel
// back-end (a terminal cell grid, a vector snapshot) falls back to upright text,
// where a slant is not meaningful — a documented limitation, exactly like
// synthetic bold cannot reproduce a designed bold's proportions.
//
// Prefer a real italic instance whenever the family ships one.
type syntheticItalicFont struct {
	base Font
}

// NewSyntheticItalicFont returns a Font that draws f at a faux-italic slant. It
// errors on a nil font. Use it only when the family has no true italic instance
// to load; a designed italic is always better.
func NewSyntheticItalicFont(f Font) (Font, error) {
	if f == nil {
		return nil, fmt.Errorf("toolkit: synthetic italic needs a font")
	}
	return &syntheticItalicFont{base: f}, nil
}

// Advance is the base face's step: shearing leans the glyphs, it does not
// re-space them.
func (f *syntheticItalicFont) Advance() int { return f.base.Advance() }

// Height is the base face's line height, unchanged by the shear.
func (f *syntheticItalicFont) Height() int { return f.base.Height() }

// Measure is the base face's advance width. The shear leans pixels beyond the
// nominal box by up to Height*num/den on the top row, but keeping Measure equal
// to the base means adjacent runs stay on their grid; the small lean bleed is
// covered by the widget's own clip.
func (f *syntheticItalicFont) Measure(text string) int { return f.base.Measure(text) }

// Ascent forwards the base face's baseline offset when it exposes one.
func (f *syntheticItalicFont) Ascent() int {
	if a, ok := f.base.(interface{ Ascent() int }); ok {
		return a.Ascent()
	}
	return 0
}

// Draw renders the base run to a scratch buffer and blits it back row by row,
// each row shifted right by (height-1-row)*num/den so the top leans forward. A
// non-pixel back-end can't be sheared this way, so it gets the upright run.
func (f *syntheticItalicFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	pp, ok := p.(*painter.PixelPainter)
	if !ok {
		f.base.Draw(p, x, y, text, ink)
		return
	}
	w := f.base.Measure(text)
	h := f.base.Height()
	if w <= 0 || h <= 0 {
		return
	}
	tmp := make([]byte, 4*w*h)
	f.base.Draw(painter.NewPixelPainter(tmp, w, h), 0, 0, text, ink)
	for row := 0; row < h; row++ {
		shift := (h - 1 - row) * syntheticItalicNum / syntheticItalicDen
		for sx := 0; sx < w; sx++ {
			i := (row*w + sx) * 4
			if tmp[i+3] == 0 {
				continue
			}
			pp.PutPixel(x+sx+shift, y+row, RGBA{R: tmp[i], G: tmp[i+1], B: tmp[i+2], A: tmp[i+3]})
		}
	}
}

// FontData / SizePx forward the base face's sfnt bytes / em size so a vector
// back-end can still embed the real font; the shear is a raster effect that does
// not survive into the embedded font.
func (f *syntheticItalicFont) FontData() []byte {
	if d, ok := f.base.(interface{ FontData() []byte }); ok {
		return d.FontData()
	}
	return nil
}

func (f *syntheticItalicFont) SizePx() int {
	if s, ok := f.base.(interface{ SizePx() int }); ok {
		return s.SizePx()
	}
	return 0
}
