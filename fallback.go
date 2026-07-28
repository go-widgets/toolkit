// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"

	"github.com/go-widgets/painter"
)

// fallbackFont renders text across a chain of TrueType faces: each rune is drawn
// by the first face in the chain that has a glyph for it, so a Latin primary can
// fall back to a CJK (or Arabic, Devanagari, …) face for scripts it does not
// cover. Runs of consecutive same-face runes are shaped together and aligned to
// the PRIMARY face's baseline, so mixed text (e.g. "Show HN: 中文标题") lays out on
// one line. Metrics (Advance/Height) come from the primary face.
type fallbackFont struct {
	fonts []*truetypeFont // primary first, then fallbacks in priority order
}

// NewFallbackFont chains fonts so glyphs missing from earlier fonts are rendered
// by later ones. The first font is the primary (its metrics and baseline drive
// layout). Every argument must be a font from NewTrueTypeFont; at least one is
// required. This is how the app renders scripts outside the primary UI face —
// pass a CJK face after the Latin one to display Chinese/Japanese text.
func NewFallbackFont(fonts ...Font) (Font, error) {
	if len(fonts) == 0 {
		return nil, fmt.Errorf("toolkit: fallback font needs at least one font")
	}
	tts := make([]*truetypeFont, 0, len(fonts))
	for _, f := range fonts {
		tt, ok := f.(*truetypeFont)
		if !ok {
			return nil, fmt.Errorf("toolkit: fallback font requires TrueType fonts (from NewTrueTypeFont)")
		}
		tts = append(tts, tt)
	}
	return &fallbackFont{fonts: tts}, nil
}

// Advance is the primary face's space step.
func (f *fallbackFont) Advance() int { return f.fonts[0].advance }

// Height is the primary face's line height.
func (f *fallbackFont) Height() int { return f.fonts[0].height }

// pick returns the first face covering r, or the primary when none does (so an
// unknown rune still lands on the primary's .notdef, matching a single face).
func (f *fallbackFont) pick(r rune) *truetypeFont {
	for _, tt := range f.fonts {
		if tt.covers(r) {
			return tt
		}
	}
	return f.fonts[0]
}

// fbRun is a maximal slice of text rendered by a single face.
type fbRun struct {
	text string
	font *truetypeFont
}

// runs splits text into maximal runs, each routed to the face that covers it.
func (f *fallbackFont) runs(text string) []fbRun {
	var out []fbRun
	var cur *truetypeFont
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, fbRun{text: b.String(), font: cur})
			b.Reset()
		}
	}
	for _, r := range text {
		ft := f.pick(r)
		if ft != cur {
			flush()
			cur = ft
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

// Measure sums each run's width in its own face, so the total matches Draw.
func (f *fallbackFont) Measure(text string) int {
	total := 0
	for _, run := range f.runs(text) {
		total += run.font.Measure(run.text)
	}
	return total
}

// Draw paints each run with its covering face, all aligned to the primary
// baseline. On a non-pixel painter it reorders bidi and hands runes to the
// painter, mirroring truetypeFont.
func (f *fallbackFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	pix, isPixel := p.(*painter.PixelPainter)
	if !isPixel {
		p.Text(x, y, visualText(text), ink)
		return
	}
	baseline := y + f.fonts[0].ascent
	pen := x
	for _, run := range f.runs(text) {
		pen = run.font.drawShaped(pix, pen, baseline, run.text, ink)
	}
}
