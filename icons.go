// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

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
func DrawIconNew(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Outer rectangle sans top-right corner.
	strokeRect(surface, surfaceW, x, y, w, h, ink)
	// Fold: a small triangle in the top-right corner.
	fold := w / 3
	for i := 0; i < fold; i++ {
		putPixel(surface, surfaceW, x+w-fold+i, y, ink)
		putPixel(surface, surfaceW, x+w, y+i, ink)
		putPixel(surface, surfaceW, x+w-i, y+i, ink)
	}
}

// DrawIconOpen paints a folder-outline icon (rectangle with a small
// tab on the top-left).
func DrawIconOpen(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset+2
	w, h := r.W-2*inset, r.H-2*inset-2
	// Tab on top.
	tabW := w / 3
	for i := 0; i < tabW; i++ {
		putPixel(surface, surfaceW, x+i, y-1, ink)
		putPixel(surface, surfaceW, x+i, y-2, ink)
	}
	putPixel(surface, surfaceW, x+tabW, y-1, ink)
	putPixel(surface, surfaceW, x+tabW, y-2, ink)
	// Folder body.
	strokeRect(surface, surfaceW, x, y, w, h, ink)
}

// DrawIconSave paints a floppy-disk-outline icon (outer square with
// a small label rectangle on top).
func DrawIconSave(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	strokeRect(surface, surfaceW, x, y, w, h, ink)
	// Label rect in the upper half.
	labelW := w * 2 / 3
	labelH := h / 3
	strokeRect(surface, surfaceW, x+(w-labelW)/2, y+2, labelW, labelH, ink)
}

// DrawIconCut paints a pair-of-scissors icon (two open-circle
// handles + crossed blades).
func DrawIconCut(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Two small circles as handles.
	strokeRect(surface, surfaceW, x, y+h*2/3, w/4, w/4, ink)
	strokeRect(surface, surfaceW, x+w-w/4, y+h*2/3, w/4, w/4, ink)
	// Crossed blades: two diagonal lines from the handles to the top-centre.
	cx := x + w/2
	cy := y + 2
	for t := 0; t < h*2/3-2; t++ {
		// left blade rising to the centre
		putPixel(surface, surfaceW, x+w/8+t*(cx-x-w/8)/(h*2/3-2), y+h*2/3-t, ink)
		// right blade rising to the centre
		putPixel(surface, surfaceW, x+w-w/8-t*(x+w-w/8-cx)/(h*2/3-2), y+h*2/3-t, ink)
	}
	_ = cy
}

// DrawIconCopy paints two overlapping document outlines.
func DrawIconCopy(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Back page (offset up-left).
	strokeRect(surface, surfaceW, x, y, w-w/4, h-h/4, ink)
	// Front page (offset down-right).
	strokeRect(surface, surfaceW, x+w/4, y+h/4, w-w/4, h-h/4, ink)
}

// DrawIconPaste paints a clipboard outline with a clip on top.
func DrawIconPaste(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset+2
	w, h := r.W-2*inset, r.H-2*inset-2
	// Clip: a small rectangle centred at the top.
	clipW := w / 3
	clipH := 3
	clipX := x + (w-clipW)/2
	strokeRect(surface, surfaceW, clipX, y-2, clipW, clipH, ink)
	// Board.
	strokeRect(surface, surfaceW, x, y, w, h, ink)
}

// DrawIconUndo paints a curved arrow pointing left (approximated as
// a horizontal stroke + a triangular head).
func DrawIconUndo(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+r.H/2
	w := r.W - 2*inset
	// Horizontal shaft.
	fillRect(surface, surfaceW, x, y, w, 1, ink)
	// Arrowhead: left-pointing triangle.
	for t := 0; t <= 3; t++ {
		putPixel(surface, surfaceW, x+t, y-t, ink)
		putPixel(surface, surfaceW, x+t, y+t, ink)
	}
}

// DrawIconRedo paints a curved arrow pointing right (mirror of Undo).
func DrawIconRedo(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+r.H/2
	w := r.W - 2*inset
	fillRect(surface, surfaceW, x, y, w, 1, ink)
	for t := 0; t <= 3; t++ {
		putPixel(surface, surfaceW, x+w-t, y-t, ink)
		putPixel(surface, surfaceW, x+w-t, y+t, ink)
	}
}

// DrawIconSearch paints a magnifying-glass icon (a circle + a
// diagonal handle).
func DrawIconSearch(surface []byte, surfaceW int, r Rect, ink RGBA) {
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
	strokeRect(surface, surfaceW, x, y, lensW, lensW, ink)
	// Handle: a diagonal from the bottom-right corner of the lens to
	// the icon's bottom-right corner.
	for t := 0; t <= d/3; t++ {
		putPixel(surface, surfaceW, x+lensW+t, y+lensW+t, ink)
	}
}

// DrawIconSettings paints a gear-outline icon (approximated as a
// square with corner "teeth").
func DrawIconSettings(surface []byte, surfaceW int, r Rect, ink RGBA) {
	inset := iconInset(r)
	x, y := r.X+inset, r.Y+inset
	w, h := r.W-2*inset, r.H-2*inset
	// Inner square (the gear body).
	inner := 2
	strokeRect(surface, surfaceW, x+inner, y+inner, w-2*inner, h-2*inner, ink)
	// Four teeth: one on each edge (top/bottom/left/right, centred).
	fillRect(surface, surfaceW, x+w/2-1, y, 2, inner, ink)
	fillRect(surface, surfaceW, x+w/2-1, y+h-inner, 2, inner, ink)
	fillRect(surface, surfaceW, x, y+h/2-1, inner, 2, ink)
	fillRect(surface, surfaceW, x+w-inner, y+h/2-1, inner, 2, ink)
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
