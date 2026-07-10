// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// FontChooser is a vertical picker of fonts: each option's name is drawn in
// that very font, so the list doubles as a live size/style preview. Clicking a
// row selects it, applies it as the active font via SetFont, and fires
// OnChoose. It is the picker the Font interface (v0.20) unblocked — the
// long-deferred sibling of ColorChooser / FileChooser.
//
// With no options supplied it defaults to three scales of the built-in bitmap
// font (Regular / Large / Extra Large), so an app gets a working font size
// picker for free.
type FontChooser struct {
	Base
	Options  []FontOption
	Selected int
	OnChoose func(idx int, f Font)
}

// FontOption is one named font in a FontChooser.
type FontOption struct {
	Name string
	Font Font
}

// FontChooser sizing.
const (
	// FontChooserPad is the panel's inner inset.
	FontChooserPad = 4
	// FontChooserRowPad is the vertical padding above+below each row's glyphs.
	FontChooserRowPad = 3
)

// defaultFontOptions is the built-in scale ladder used when Options is empty.
func defaultFontOptions() []FontOption {
	return []FontOption{
		{Name: "Regular", Font: NewBitmapFont(1)},
		{Name: "Large", Font: NewBitmapFont(2)},
		{Name: "Extra Large", Font: NewBitmapFont(3)},
	}
}

// NewFontChooser builds a FontChooser over the given options (defaulting to the
// built-in scale ladder when none are supplied).
func NewFontChooser(options []FontOption) *FontChooser {
	if len(options) == 0 {
		options = defaultFontOptions()
	}
	return &FontChooser{Options: options}
}

// rowHeight is the pixel height of option i's row: its font's glyph height plus
// padding above and below.
func (fc *FontChooser) rowHeight(i int) int {
	return fc.Options[i].Font.Height() + 2*FontChooserRowPad
}

// Draw paints the panel and each option's name rendered in its own font, the
// Selected row on an Accent band.
func (fc *FontChooser) Draw(p painter.Painter, theme *Theme) {
	r := fc.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	y := r.Y + FontChooserPad
	for i, opt := range fc.Options {
		rh := fc.rowHeight(i)
		ink := theme.OnSurface
		if i == fc.Selected {
			fillRect(p, r.X+1, y, r.W-2, rh, theme.Accent)
			ink = theme.Background
		}
		// Render the name in the option's OWN font so the row previews it.
		opt.Font.Draw(p, r.X+FontChooserPad, y+FontChooserRowPad, opt.Name, ink)
		y += rh
	}
}

// rowAt maps a panel-local y to an option index, or -1 if it lands outside any
// row (in the top/bottom padding).
func (fc *FontChooser) rowAt(y int) int {
	cy := FontChooserPad
	for i := range fc.Options {
		rh := fc.rowHeight(i)
		if y >= cy && y < cy+rh {
			return i
		}
		cy += rh
	}
	return -1
}

// OnEvent: a click on a row selects it, applies it as the active font (SetFont),
// and fires OnChoose.
func (fc *FontChooser) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	idx := fc.rowAt(ev.Y)
	if idx < 0 {
		return
	}
	fc.Selected = idx
	SetFont(fc.Options[idx].Font)
	if fc.OnChoose != nil {
		fc.OnChoose(idx, fc.Options[idx].Font)
	}
}
