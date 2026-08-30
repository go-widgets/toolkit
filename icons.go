// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Icons draws a small library of pixel-art stock icons into an RGBA
// buffer, so a Toolbar can render New / Open / Save / Cut / Copy /
// Paste / Undo / Redo without every caller shipping their own bitmap
// artwork. Each Draw function takes a 24x24 target rect (matches
// ToolbarButtonW/H) — smaller/larger rects still work (the shapes
// scale-fit) but 24x24 is the ergonomic default.
//
// The set is intentionally small (10 icons); it covers the "File +
// Edit toolbar" case that every text-app needs. Adding an icon =
// one new DrawIcon*** function + one entry in a host's ToolbarItem
// slice.
//
// All icons are painted with 1-pixel strokes in the given ink; no
// filled shapes, no anti-aliasing. Reads cleanly at 24-32 px.

// DrawIconNew paints a document-outline icon (rectangle with a
// folded top-right corner).
func DrawIconNew(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Outer rectangle sans top-right corner.
	strokeRect(p, x, y, w, h, ink)
	// Fold: a small triangle in the top-right corner.
	fold := w / 3
	for i := 0; i < fold; i++ {
		putPixel(p, x+w-fold+i, y, ink)
		putPixel(p, x+w, y+i, ink)
		putPixel(p, x+w-i, y+i, ink)
	}
}

// DrawIconOpen paints a folder-outline icon (rectangle with a small
// tab on the top-left).
func DrawIconOpen(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset+2
	w, h := r.W-2*inset, r.H-2*inset-2
	// Tab on top.
	tabW := w / 3
	for i := 0; i < tabW; i++ {
		putPixel(p, x+i, y-1, ink)
		putPixel(p, x+i, y-2, ink)
	}
	putPixel(p, x+tabW, y-1, ink)
	putPixel(p, x+tabW, y-2, ink)
	// Folder body.
	strokeRect(p, x, y, w, h, ink)
}

// DrawIconSave paints a floppy-disk-outline icon (outer square with
// a small label rectangle on top).
func DrawIconSave(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	strokeRect(p, x, y, w, h, ink)
	// Label rect in the upper half.
	labelW := w * 2 / 3
	labelH := h / 3
	strokeRect(p, x+(w-labelW)/2, y+2, labelW, labelH, ink)
}

// DrawIconCut paints a pair-of-scissors icon (two open-circle
// handles + crossed blades).
func DrawIconCut(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Two small circles as handles.
	strokeRect(p, x, y+h*2/3, w/4, w/4, ink)
	strokeRect(p, x+w-w/4, y+h*2/3, w/4, w/4, ink)
	// Crossed blades: two diagonal lines from the handles to the top-centre.
	cx := x + w/2
	cy := y + 2
	for t := 0; t < h*2/3-2; t++ {
		// left blade rising to the centre
		putPixel(p, x+w/8+t*(cx-x-w/8)/(h*2/3-2), y+h*2/3-t, ink)
		// right blade rising to the centre
		putPixel(p, x+w-w/8-t*(x+w-w/8-cx)/(h*2/3-2), y+h*2/3-t, ink)
	}
	_ = cy
}

// DrawIconCopy paints two overlapping document outlines.
func DrawIconCopy(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Back page (offset up-left).
	strokeRect(p, x, y, w-w/4, h-h/4, ink)
	// Front page (offset down-right).
	strokeRect(p, x+w/4, y+h/4, w-w/4, h-h/4, ink)
}

// DrawIconPaste paints a clipboard outline with a clip on top.
func DrawIconPaste(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset+2
	w, h := r.W-2*inset, r.H-2*inset-2
	// Clip: a small rectangle centred at the top.
	clipW := w / 3
	clipH := 3
	clipX := x + (w-clipW)/2
	strokeRect(p, clipX, y-2, clipW, clipH, ink)
	// Board.
	strokeRect(p, x, y, w, h, ink)
}

// DrawIconUndo paints a curved arrow pointing left (approximated as
// a horizontal stroke + a triangular head).
func DrawIconUndo(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+r.H/2
	w := r.W - 2*inset
	// Horizontal shaft.
	fillRect(p, x, y, w, 1, ink)
	// Arrowhead: left-pointing triangle.
	for t := 0; t <= 3; t++ {
		putPixel(p, x+t, y-t, ink)
		putPixel(p, x+t, y+t, ink)
	}
}

// DrawIconRedo paints a curved arrow pointing right (mirror of Undo).
func DrawIconRedo(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+r.H/2
	w := r.W - 2*inset
	fillRect(p, x, y, w, 1, ink)
	for t := 0; t <= 3; t++ {
		putPixel(p, x+w-t, y-t, ink)
		putPixel(p, x+w-t, y+t, ink)
	}
}

// DrawIconSearch paints a magnifying-glass icon (a circle + a
// diagonal handle).
func DrawIconSearch(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	d := r.W - 2*inset
	if r.H-2*inset < d {
		d = r.H - 2*inset
	}
	// Circle approximated as a rounded square: 3-px inset from each corner.
	// Draw the lens outline as a hollow rectangle so we don't need a
	// midpoint-circle routine.
	lensW := d * 2 / 3
	strokeRect(p, x, y, lensW, lensW, ink)
	// Handle: a diagonal from the bottom-right corner of the lens to
	// the icon's bottom-right corner.
	for t := 0; t <= d/3; t++ {
		putPixel(p, x+lensW+t, y+lensW+t, ink)
	}
}

// DrawIconSettings paints a gear-outline icon (approximated as a
// square with corner "teeth").
func DrawIconSettings(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Inner square (the gear body).
	inner := 2
	strokeRect(p, x+inner, y+inner, w-2*inner, h-2*inner, ink)
	// Four teeth: one on each edge (top/bottom/left/right, centred).
	fillRect(p, x+w/2-1, y, 2, inner, ink)
	fillRect(p, x+w/2-1, y+h-inner, 2, inner, ink)
	fillRect(p, x, y+h/2-1, inner, 2, ink)
	fillRect(p, x+w-inner, y+h/2-1, inner, 2, ink)
}

// iconInset returns the pixel inset from a rect's edges to the icon
// content — 3 px for a standard 24x24 button, scaling proportionally
// for other sizes. Prevents the icon from hugging the button border.
func iconInset(r Rect) int {
	// Roughly 12 % of the smaller dimension, min 2.
	d := r.W
	if r.H < d {
		d = r.H
	}
	inset := d / 8
	if inset < 2 {
		inset = 2
	}
	return inset
}

// DrawIconGlasses paints XR glasses seen from the front: two lens rectangles
// joined by a bridge, with a temple angling back from each outer edge.
//
// It is in the stock set because a headset is a DEVICE CLASS, like a printer or
// a camera, and an application that lets a person choose between two pairs of
// glasses should not have to ship artwork to do it. Drawn rather than
// photographed for the same reason every icon here is: a photograph of somebody
// else's product is their picture, and an offline application has nowhere to
// fetch one from anyway.
func DrawIconGlasses(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset

	// Two lenses either side of a bridge a sixth of the width, centred
	// vertically on the upper half where a pair of glasses actually sits.
	bridge := w / 6
	if bridge < 1 {
		bridge = 1
	}
	lensW := (w - bridge) / 2
	lensH := h / 2
	if lensH < 1 {
		lensH = 1
	}
	top := y + (h-lensH)/2
	strokeRect(p, x, top, lensW, lensH, ink)
	strokeRect(p, x+lensW+bridge, top, lensW, lensH, ink)
	// The bridge itself, along the lenses' top edge.
	fillRect(p, x+lensW, top, bridge, 1, ink)

	// A temple from each outer edge, angling back and down: two pixels out for
	// every one down, which reads as a hinge at this size.
	arm := h / 3
	for t := 0; t < arm; t++ {
		putPixel(p, x-t/2, top+t, ink)
		putPixel(p, x+w+t/2, top+t, ink)
	}
}

// DrawIconApp paints an application window: a frame with a title bar across its
// top and a dot in it where a close button sits.
//
// It is in the stock set for the same reason as [DrawIconGlasses]: a running
// APPLICATION is a thing an interface has to offer a person a picture of — a
// gallery of what is open, a picker for which window goes where — and no
// application should have to hand-draw beside the widget to say "this is a
// program". What it deliberately is not is any particular program's icon: those
// belong to whoever wrote them, and a system that has one will hand it over as
// an [Image], which a cell prefers when both are set.
func DrawIconApp(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	if w < 1 || h < 1 {
		return
	}
	strokeRect(p, x, y, w, h, ink)

	// The title bar: a fifth of the height, at least one pixel, closed off by a
	// line so the frame reads as a window rather than as an empty box.
	bar := h / 5
	if bar < 1 {
		bar = 1
	}
	fillRect(p, x, y+bar, w, 1, ink)

	// One dot in the bar, a bar-height in from the left, which is where every
	// window on every system this runs on keeps its buttons.
	dot := bar - 1
	if dot < 1 {
		dot = 1
	}
	if bar+dot < h {
		fillRect(p, x+dot, y+(bar-dot)/2, dot, dot, ink)
	}
}

// DrawIconPlus paints a plus: two bars of equal thickness crossing at the
// centre, symmetric about both axes at every size.
//
// It is in the stock set because "add one more" is drawn everywhere and because
// the obvious alternative is not symmetric. A "+" TYPESET from a font is a
// glyph with side bearings and a baseline: it sits left of centre in its own
// box, its arms are unequal, and at the size a headset needs — a plus the width
// of a hand at arm's length — that is plainly visible. It was reported from a
// pair of glasses as "not properly symmetric", which it was not.
//
// The bar thickness is a fifth of the box, rounded to an ODD number of pixels
// whenever the box is odd and an even one when it is even, so the two arms
// either side of centre are the same length. That is the whole of why this is
// not three lines.
func DrawIconPlus(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	if w < 1 || h < 1 {
		return
	}
	// A square, centred: a plus in a rectangle is a cross, and nobody means a
	// cross.
	side := w
	if h < side {
		side = h
	}
	x += (w - side) / 2
	y += (h - side) / 2

	thick := side / 5
	if thick < 1 {
		thick = 1
	}
	// Same parity as the side, so (side-thick) is even and the two arms are
	// equal to the pixel.
	if (side-thick)%2 != 0 {
		thick++
	}
	off := (side - thick) / 2
	fillRect(p, x+off, y, thick, side, ink)
	fillRect(p, x, y+off, side, thick, ink)
}

// DrawIconDot paints a filled disc: the smallest thing a picture can say.
//
// It is what a status LIGHT is — "this is live", "this is recording", "this is
// connected" — put on top of another icon or beside a row of them. A dot rather
// than a shape change, because a colour is read at a glance where a different
// outline has to be looked at twice, and because the one place these appear
// most is a menu bar somebody is not looking at yet.
//
// It fills the rectangle it is given, less the inset every icon here leaves, so
// a caller sizes it by sizing the rectangle. Round in a square box and an
// ellipse in any other: a caller asking for a wide box means a wide dot.
func DrawIconDot(p painter.Painter, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	if w < 1 || h < 1 {
		return
	}
	// A round rectangle whose radius is half its shorter side IS a disc, and
	// the painter already draws those with the antialiasing the rest of this
	// package gets. Nothing here rasterises a circle by hand.
	radius := w
	if h < radius {
		radius = h
	}
	p.FillRoundRect(Rect{X: x, Y: y, W: w, H: h}, radius/2, ink)
}
