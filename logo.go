// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DrawLogo paints the logo SVG doc into r, in ink for a document that draws with
// currentColor, or the toolkit's neutral application mark when doc is empty. The
// document is rasterised once per size and cached (see [SVGIcon]), so calling it
// every frame is cheap. The caller has already vetted doc: this does not fetch
// or sanitise anything.
func DrawLogo(p painter.Painter, r Rect, doc string, ink RGBA) {
	if doc == "" {
		DrawIconApp(p, r, ink)
		return
	}
	SVGIcon(doc)(p, r, ink)
}

// Logo is a widget that draws a logo given as an SVG document, square, at the
// left of its bounds and centred vertically, with a neutral mark when there is no
// document. Doc is asked at every draw, so a host that swaps the logo (a server
// chosen in a list) repaints the widget and nothing else.
type Logo struct {
	Base
	// Name is what an assistive technology reads for the picture.
	Name string
	// Doc returns the SVG document to show now, or "" for the neutral mark.
	Doc func() string
}

// NewLogo builds a Logo named name showing what doc returns.
func NewLogo(name string, doc func() string) *Logo { return &Logo{Name: name, Doc: doc} }

// HitTest is false: a logo is a picture, clicks pass through it.
func (l *Logo) HitTest(_, _ int) bool { return false }

// Draw paints the logo.
func (l *Logo) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	side := min(r.W, r.H)
	doc := ""
	if l.Doc != nil {
		doc = l.Doc()
	}
	DrawLogo(p, Rect{X: r.X, Y: r.Y + (r.H-side)/2, W: side, H: side}, doc, theme.OnSurface)
}

// A11y names the picture for the accessibility layer.
func (l *Logo) A11y() A11yInfo { return A11yInfo{Role: RoleImg, Name: l.Name} }

// LogoRow says what one row of a logo list shows.
type LogoRow struct {
	// Doc is the logo's SVG document, "" for the neutral mark.
	Doc string
	// Title is the first line, Subtitle the second (a state, an address).
	Title, Subtitle string
}

// LogoRows is a [ListBox.ItemRenderer] drawing each row as a logo with two lines
// of text beside it: Title, then Subtitle. row says what row i shows. Text is
// wrapped to the room beside the logo and never runs past the row's edge; the row
// is fixed in height, so when even the wrapped text is taller than the row its
// last visible line ends with "…". Set ListBox.RowHeight to fit the lines you
// expect.
func LogoRows(row func(i int) LogoRow) func(p painter.Painter, theme *Theme, rc Rect, i int, item string, selected bool, ink RGBA) {
	return func(p painter.Painter, _ *Theme, rc Rect, i int, _ string, _ bool, ink RGBA) {
		info := row(i)
		f := CurrentFont()
		pad := scaled(4)
		side := rc.H - 2*pad
		DrawLogo(p, Rect{X: rc.X + pad, Y: rc.Y + pad, W: side, H: side}, info.Doc, ink)
		x := rc.X + 2*pad + side
		lines := WrapLines(f, info.Title, max(rc.X+rc.W-pad-x, 1))
		lines = append(lines, WrapLines(f, info.Subtitle, max(rc.X+rc.W-pad-x, 1))...)
		fit := max((rc.H-2*scaled(2)+scaled(CardLineSpacing))/wrapPitch(f), 1)
		lines = clampLines(f, lines, fit, max(rc.X+rc.W-pad-x, 1))
		y := rc.Y + (rc.H-wrapBlockHeight(f, len(lines)))/2
		for _, line := range lines {
			f.Draw(p, x, y, line, ink)
			y += wrapPitch(f)
		}
	}
}
