// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Label is a passive widget that displays a string in the theme's
// OnSurface colour. v0 paints a solid coloured rectangle as a glyph
// placeholder; once the font package lands, the rectangle will be
// replaced with actual character bitmaps.
//
// Label is non-interactive: HitTest returns false so clicks pass
// through to the widget beneath. Apps that want a clickable label
// should compose a Button with the text instead.
type Label struct {
	Base
	Text string
}

// NewLabel constructs a Label carrying text.
func NewLabel(text string) *Label { return &Label{Text: text} }

// HitTest returns false unconditionally: a Label is decorative, not
// interactive. Override (or compose with a Button) to make a label
// receive events.
func (l *Label) HitTest(_, _ int) bool { return false }

// Draw paints the label into surface. v0 = a thin underline stub in
// the OnSurface colour so the widget is at least visible; v1 will
// replace this with bitmap-font rendering once the font package
// stabilises.
func (l *Label) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := l.Bounds()
	// 1-pixel-tall ink line at the vertical midpoint -- enough to
	// confirm the widget rendered + leaves room for font glyphs on
	// either side later.
	fillRect(surface, surfaceW, r.X, r.Y+r.H/2, r.W, 1, theme.OnSurface)
}
