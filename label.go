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

// Label is a passive widget that displays Text in the theme's OnSurface colour,
// drawn with the toolkit's 5x7 bitmap font. Horizontally aligned per Align
// (left by default), vertically centred if Bounds.H exceeds GlyphHeight().
//
// Label is non-interactive: HitTest returns false so clicks pass
// through to the widget beneath. Apps that want a clickable label
// should compose a Button with the text instead.
type Label struct {
	Base
	Text  string
	Align Align
}

// NewLabel constructs a Label carrying text.
func NewLabel(text string) *Label { return &Label{Text: text} }

// HitTest returns false unconditionally: a Label is decorative, not
// interactive. Override (or compose with a Button) to make a label
// receive events.
func (l *Label) HitTest(_, _ int) bool { return false }

// Draw paints the Label's text with the toolkit's bitmap font. If
// Bounds.H > GlyphHeight() the text is vertically centred; otherwise
// it lands at Bounds.Y.
func (l *Label) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	ty := r.Y
	if r.H > GlyphHeight() {
		ty += (r.H - GlyphHeight()) / 2
	}
	x := r.X
	switch l.Align {
	case AlignCenter:
		x = r.X + (r.W-TextWidth(l.Text))/2
	case AlignRight:
		x = r.X + r.W - TextWidth(l.Text)
	}
	if x < r.X {
		x = r.X // never start left of the widget
	}
	DrawText(p, x, ty, l.Text, theme.OnSurface)
}
