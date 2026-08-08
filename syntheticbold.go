// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"

	"github.com/go-widgets/painter"
)

// syntheticBoldOffset is how far right the over-strike lands, in pixels. One
// pixel thickens every stem by exactly one column: enough to read as bold at UI
// sizes, small enough not to smear counters shut.
const syntheticBoldOffset = 1

// syntheticBoldFont fakes a bold weight by over-striking the run one pixel to
// the right.
//
// It exists for faces that ship no bold instance the rasteriser can reach. The
// motivating case is a host UI typeface: macOS's SFNS is a variable font, and
// the pure-Go engine renders its default (Regular) master only, so an
// application matching the native look has a Regular face and no bold at all.
// Over-striking is the same trick X11 and early PostScript printers used, and it
// is markedly better than silently drawing headings at body weight.
//
// Prefer a real bold face whenever the family ships one — this thickens stems
// uniformly and cannot reproduce a designed bold's altered proportions.
type syntheticBoldFont struct {
	base Font
}

// NewSyntheticBoldFont returns a Font that draws f at a faux-bold weight. Use it
// only when the family has no true bold instance to load; a designed bold is
// always better. It errors on a nil font.
//
// The wrapper composes over any Font, including a fallback chain, so the whole
// chain is emboldened rather than only its primary face.
func NewSyntheticBoldFont(f Font) (Font, error) {
	if f == nil {
		return nil, fmt.Errorf("toolkit: synthetic bold needs a font")
	}
	return &syntheticBoldFont{base: f}, nil
}

// Advance is the base face's step: over-striking thickens glyphs, it does not
// re-space them.
func (f *syntheticBoldFont) Advance() int { return f.base.Advance() }

// Height is the base face's line height, unchanged by over-striking.
func (f *syntheticBoldFont) Height() int { return f.base.Height() }

// Ascent reports the base face's baseline offset when it exposes one, so a
// caller positioning by the text top is unaffected by the wrapper.
func (f *syntheticBoldFont) Ascent() int {
	if a, ok := f.base.(interface{ Ascent() int }); ok {
		return a.Ascent()
	}
	return 0
}

// Measure includes the over-strike column, so the reported width covers every
// pixel Draw paints. Without it two adjacent runs would overlap by one column,
// and a right-aligned or clipped string would lose its final stem.
func (f *syntheticBoldFont) Measure(text string) int {
	if text == "" {
		return 0
	}
	return f.base.Measure(text) + syntheticBoldOffset
}

// Draw paints the run twice, the second pass one pixel right. Both passes use
// the same ink, so overlapping coverage stays the same colour rather than
// compounding into a darker fringe.
func (f *syntheticBoldFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	f.base.Draw(p, x, y, text, ink)
	f.base.Draw(p, x+syntheticBoldOffset, y, text, ink)
}

// FontData forwards the base face's sfnt bytes when it has them, so a vector
// back-end can still embed the real font. The emboldening is a raster effect and
// does not survive into the embedded font.
func (f *syntheticBoldFont) FontData() []byte {
	if d, ok := f.base.(interface{ FontData() []byte }); ok {
		return d.FontData()
	}
	return nil
}

// SizePx forwards the base face's em size when it has one.
func (f *syntheticBoldFont) SizePx() int {
	if s, ok := f.base.(interface{ SizePx() int }); ok {
		return s.SizePx()
	}
	return 0
}
