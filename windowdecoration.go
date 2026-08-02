// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// WindowDecoration paints a window's frame chrome — a title-bar band (fill,
// title text, optional bottom hairline), a cluster of title-bar buttons
// (rectangular close/minimize/maximize boxes OR round "traffic-light" dots), a
// frame border, an optional faux drop shadow and an optional bottom-right resize
// grip — with EXPLICIT colours and EXPLICIT frame-local geometry.
//
// Unlike Window (a self-contained draggable panel that reads the theme and owns
// its own hit-testing + body), WindowDecoration is a pure painter for a host
// compositor that already owns the window model: the host passes the exact rects
// it hit-tests against (title-bar, each button, border, grip — all in the
// decoration's own coordinate space) plus the exact palette its style demands,
// and the widget just paints them. This keeps geometry a SINGLE source of truth
// on the host side (the same rects drive both hit-testing and paint) while the
// pixels are produced by the toolkit painter — so a compositor can drop its
// hand-rolled canvas draws and blit a rendered buffer instead, and every style
// (a red Openbox bar, a macOS traffic-light bar, any themed palette) keeps its
// own colours rather than collapsing to a shared theme.
//
// The BODY region between the title bar and the bottom border is never touched:
// a decoration rendered into a zeroed RGBA buffer leaves the body fully
// transparent (A=0), so a host composites the decoration OVER the live window
// body (source-over) and the body shows through the hole.
//
// Every colour follows the toolkit convention that a zero-value RGBA (A=0) means
// "absent": a zero Hairline / Border / Shadow paints nothing, and a zero button
// Outline draws no outline. TitleColor / TitleInk / GripColor are painted as
// given (a host that wants them absent gives the enclosing rect a zero size).
type WindowDecoration struct {
	Base

	// Title is the title-bar caption; TitleInk is its colour. Titlebar is the
	// band rect (frame-local); TitleColor fills it. TitleCenter centres the
	// caption horizontally (macOS style) instead of left-aligning it (the
	// default). Hairline, when non-zero, paints a 1-unit line along the band's
	// bottom edge (the macOS titlebar separator).
	Title       string
	TitleInk    RGBA
	TitleColor  RGBA
	Titlebar    Rect
	TitleCenter bool
	Hairline    RGBA

	// Border is the full frame extent (frame-local); BorderColor strokes its
	// 1-unit outline (zero = no border). Shadow, when non-zero, paints a 1-unit
	// faux drop shadow one unit past the border's right and bottom edges.
	Border      Rect
	BorderColor RGBA
	Shadow      RGBA

	// Grip is the bottom-right resize handle rect; GripColor draws its two
	// diagonal rules. ShowGrip gates the whole grip (a shaded/undecorated window
	// has none).
	Grip      Rect
	ShowGrip  bool
	GripColor RGBA

	// Buttons is the ordered title-bar button cluster (close/minimize/maximize).
	// Each carries its own rect + colours, so the host mixes box buttons and
	// traffic-lights freely.
	Buttons []DecoButton
}

// DecoButtonShape selects how a title-bar button's face is drawn.
type DecoButtonShape int

const (
	// DecoButtonRect draws a filled rectangular face (the Openbox close/minimize
	// box), with any Glyph stroked on top in GlyphInk.
	DecoButtonRect DecoButtonShape = iota
	// DecoButtonCircle draws a filled circle with an optional 1-unit outline (the
	// macOS traffic-light dot); a circle button usually carries no glyph.
	DecoButtonCircle
)

// DecoGlyph selects the symbol stroked inside a button face.
type DecoGlyph int

const (
	// DecoGlyphNone draws no symbol (a bare face / traffic-light dot).
	DecoGlyphNone DecoGlyph = iota
	// DecoGlyphClose draws an "×" (two diagonals).
	DecoGlyphClose
	// DecoGlyphMinimize draws a low horizontal bar.
	DecoGlyphMinimize
	// DecoGlyphMaximize draws a square outline.
	DecoGlyphMaximize
)

// DecoButton is one title-bar button: a face (rectangle or circle) filled with
// Face, an optional Outline (circle only; A=0 = none) and an optional Glyph
// stroked in GlyphInk. Rect is frame-local.
type DecoButton struct {
	Rect     Rect
	Shape    DecoButtonShape
	Face     RGBA
	Outline  RGBA
	Glyph    DecoGlyph
	GlyphInk RGBA
}

// decoGlyphInset is the padding (in units) between a button's edge and its
// stroked glyph, matching the compositor's hand-drawn close/minimize glyphs.
const decoGlyphInset = 3

// NewWindowDecoration builds an empty decoration; set the exported fields (or
// use the ruby-widgets Decoration binding) before rendering.
func NewWindowDecoration() *WindowDecoration { return &WindowDecoration{} }

// AddButton appends a button to the cluster and returns the decoration for
// fluent construction.
func (d *WindowDecoration) AddButton(b DecoButton) *WindowDecoration {
	d.Buttons = append(d.Buttons, b)
	return d
}

// Draw paints the decoration: title-bar band + hairline + caption, the button
// cluster, then (over the body area, which stays untouched) the resize grip, the
// faux shadow and the border last so it sits on top of the body edges.
func (d *WindowDecoration) Draw(p painter.Painter, theme *Theme) {
	_ = theme // WindowDecoration is fully self-coloured; the theme is unused.

	// Title-bar band + bottom hairline + caption.
	if d.Titlebar.W > 0 && d.Titlebar.H > 0 {
		p.FillRect(d.Titlebar, d.TitleColor)
		if d.Hairline.A != 0 {
			fillRect(p, d.Titlebar.X, d.Titlebar.Y+d.Titlebar.H-1, d.Titlebar.W, 1, d.Hairline)
		}
		d.drawTitle(p)
	}

	// Button cluster.
	for _, b := range d.Buttons {
		d.drawButton(p, b)
	}

	// Resize grip: two short diagonals in the bottom-right corner.
	if d.ShowGrip && d.Grip.W > 0 && d.Grip.H > 0 {
		g := d.Grip
		drawLine(p, g.X+g.W, g.Y+g.H*4/10, g.X+g.W*4/10, g.Y+g.H, d.GripColor)
		drawLine(p, g.X+g.W, g.Y+g.H*3/4, g.X+g.W*3/4, g.Y+g.H, d.GripColor)
	}

	// Faux drop shadow, one unit past the border's right + bottom edges.
	if d.Shadow.A != 0 && d.Border.W > 0 && d.Border.H > 0 {
		b := d.Border
		fillRect(p, b.X+b.W, b.Y+1, 1, b.H, d.Shadow)
		fillRect(p, b.X+1, b.Y+b.H, b.W, 1, d.Shadow)
	}

	// Border last so it lands on top of the body's outer edge.
	if d.BorderColor.A != 0 && d.Border.W > 0 && d.Border.H > 0 {
		p.StrokeRect(d.Border, d.BorderColor, 1)
	}
}

// drawTitle paints the caption left-aligned (default) or centred, vertically
// centred in the band.
func (d *WindowDecoration) drawTitle(p painter.Painter) {
	if d.Title == "" {
		return
	}
	ty := d.Titlebar.Y + (d.Titlebar.H-d.glyphHeight())/2
	var tx int
	if d.TitleCenter {
		tx = d.Titlebar.X + (d.Titlebar.W-d.textWidth(d.Title))/2
	} else {
		tx = d.Titlebar.X + 6
	}
	d.drawText(p, tx, ty, d.Title, d.TitleInk)
}

// drawButton paints one button's face + optional glyph.
func (d *WindowDecoration) drawButton(p painter.Painter, b DecoButton) {
	r := b.Rect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	switch b.Shape {
	case DecoButtonCircle:
		radius := r.W
		if r.H < radius {
			radius = r.H
		}
		radius /= 2
		fillRoundRect(p, r.X, r.Y, r.W, r.H, radius, b.Face)
		if b.Outline.A != 0 {
			strokeRoundRect(p, r.X, r.Y, r.W, r.H, radius, b.Outline)
		}
	default: // DecoButtonRect
		p.FillRect(r, b.Face)
	}
	d.drawGlyph(p, b)
}

// drawGlyph strokes a button's symbol inside its face.
func (d *WindowDecoration) drawGlyph(p painter.Painter, b DecoButton) {
	r := b.Rect
	switch b.Glyph {
	case DecoGlyphClose:
		x0, y0 := r.X+decoGlyphInset, r.Y+decoGlyphInset
		x1, y1 := r.X+r.W-decoGlyphInset-1, r.Y+r.H-decoGlyphInset-1
		drawLine(p, x0, y0, x1, y1, b.GlyphInk)
		drawLine(p, x1, y0, x0, y1, b.GlyphInk)
	case DecoGlyphMinimize:
		fillRect(p, r.X+decoGlyphInset, r.Y+r.H-4, r.W-2*decoGlyphInset, 1, b.GlyphInk)
	case DecoGlyphMaximize:
		strokeRect(p, r.X+decoGlyphInset, r.Y+decoGlyphInset,
			r.W-2*decoGlyphInset, r.H-2*decoGlyphInset, b.GlyphInk)
	}
}
