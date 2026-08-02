// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Align is a widget's horizontal text alignment within its bounds. The zero
// value is AlignLeft, so an unset Align keeps the original left-aligned layout.
type Align int

const (
	// AlignLeft anchors text to the left edge (the default).
	AlignLeft Align = iota
	// AlignCenter centres text horizontally.
	AlignCenter
	// AlignRight anchors text to the right edge.
	AlignRight
)

// VAlign is a widget's vertical text alignment within its bounds height. The
// zero value VAuto preserves the label's original layout (centred when the bounds
// are taller than the text, else top-anchored), so existing labels are unchanged;
// VTop/VMiddle/VBottom force a specific edge.
type VAlign int

const (
	// VAuto keeps the original behaviour: vertically centred when Bounds.H
	// exceeds the glyph height, otherwise top-anchored. The default.
	VAuto VAlign = iota
	// VTop anchors text to the top edge.
	VTop
	// VMiddle centres text vertically within the bounds height.
	VMiddle
	// VBottom anchors text to the bottom edge.
	VBottom
)

// Label is a passive widget that displays Text in the theme's OnSurface colour,
// drawn with the toolkit's 5x7 bitmap font. It is horizontally aligned per
// Align (left by default) and vertically aligned per VAlign (VAuto keeps the
// original centre-when-taller layout by default). When Ellipsis is set,
// over-wide text is truncated with a trailing "…" to fit the bounds width.
//
// Label is non-interactive: HitTest returns false so clicks pass
// through to the widget beneath. Apps that want a clickable label
// should compose a Button with the text instead.
type Label struct {
	Base
	Text  string
	Align Align
	// VAlign is the vertical alignment of the text within the Label's bounds
	// height. The zero value VAuto keeps the original layout (centred when taller than the
	// text, else top); VTop/VMiddle/VBottom force a specific edge.
	VAlign VAlign
	// Ellipsis truncates the text with a trailing "…" when it is wider than the
	// Label's bounds width. The zero value false renders the full text, which
	// may overflow the bounds (the original behaviour).
	Ellipsis bool
	// Ink overrides the text colour. The zero value (A==0) means "inherit the
	// theme's OnSurface colour"; set a colour with a non-zero alpha to paint the
	// label in it (e.g. a muted or accent tone).
	Ink RGBA
}

// NewLabel constructs a Label carrying text.
func NewLabel(text string) *Label { return &Label{Text: text} }

// HitTest returns false unconditionally: a Label is decorative, not
// interactive. Override (or compose with a Button) to make a label
// receive events.
func (l *Label) HitTest(_, _ int) bool { return false }

// Draw paints the Label's text with the toolkit's bitmap font. The text is
// positioned vertically per VAlign (VAuto keeps the original centre-when-taller)
// and horizontally per Align. When Ellipsis is set and the text is wider than
// the bounds it is truncated with a trailing "…" so it fits the width.
func (l *Label) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	gh := l.glyphHeight()

	text := l.Text
	if l.Ellipsis && l.textWidth(text) > r.W {
		text = ellipsize(l.EffectiveFont(), text, r.W)
	}

	ty := r.Y // VTop: anchored to the top edge.
	switch l.VAlign {
	case VAuto:
		// Original behaviour: centre when the box is taller than the text.
		if r.H > gh {
			ty = r.Y + (r.H-gh)/2
		}
	case VMiddle:
		ty = r.Y + (r.H-gh)/2
	case VBottom:
		ty = r.Y + r.H - gh
	}

	x := r.X
	switch l.Align {
	case AlignCenter:
		x = r.X + (r.W-l.textWidth(text))/2
	case AlignRight:
		x = r.X + r.W - l.textWidth(text)
	}
	if x < r.X {
		x = r.X // never start left of the widget
	}
	ink := theme.OnSurface
	if l.Ink.A != 0 {
		ink = l.Ink
	}
	l.drawText(p, x, ty, text, ink)
}

// ellipsize returns s trimmed to fit width in font f, with a trailing
// horizontal ellipsis ("…"). It drops trailing runes until the prefix plus the
// ellipsis measures within width; if not even the bare ellipsis fits, the bare
// ellipsis is still returned.
func ellipsize(f Font, s string, width int) string {
	const ell = "…"
	runes := []rune(s)
	for len(runes) > 0 {
		if f.Measure(string(runes)+ell) <= width {
			return string(runes) + ell
		}
		runes = runes[:len(runes)-1]
	}
	return ell
}
