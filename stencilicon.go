// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sync"

	"github.com/go-widgets/painter"
)

// StencilIcon turns a picture whose ALPHA is the shape into an icon drawer, for
// a [IconCell.Icon], a [TreeTableNode.Icon], or anywhere else the toolkit takes
// a func(painter.Painter, Rect, RGBA).
//
// A STENCIL, not a photograph: the pixels say where the ink goes and the caller
// says what colour it is, which is what a platform means by a "template" image.
// The colours in the source are ignored -- only the alpha channel is read -- so
// the same picture is dark on a light theme, light on a dark one, and the
// accent colour when whatever holds it is selected.
//
// It exists because a system's own symbols come as exactly that. An application
// that has one -- the menu-bar glyph the platform draws for its own bar -- had
// no way to put it in a toolkit widget except as an [Image], which paints the
// pixels as they are: a black glyph that vanishes on a dark card and never
// highlights. Rasterising it a second time as artwork, or hand-blitting it
// beside the widget, are the two things this exists to make unnecessary.
//
// The tinted copy is kept and reused until the ink changes, so a grid redrawing
// every frame tints once rather than once a frame. The source is not modified
// and is not retained beyond this drawer.
//
// The picture keeps its aspect ratio inside the box, centred: a system symbol
// is rarely square, and stretching one to a square box is what makes it look
// like somebody else's icon. Pixels shorter than w*h*4 and an empty box draw
// nothing; a back end without the image primitive still draws, one pixel at a
// time, because that is what blitting here already falls back to.
func StencilIcon(pix []byte, w, h int) func(p painter.Painter, r Rect, ink RGBA) {
	var (
		mu      sync.Mutex
		tinted  []byte
		lastInk RGBA
	)
	return func(p painter.Painter, r Rect, ink RGBA) {
		if r.W <= 0 || r.H <= 0 || w <= 0 || h <= 0 || len(pix) < w*h*4 {
			return
		}
		mu.Lock()
		if tinted == nil || lastInk != ink {
			tinted, lastInk = tintStencil(pix, w, h, ink), ink
		}
		buf := tinted
		mu.Unlock()
		dst := FitBounds(w, h, r)
		blitImage(p, dst, dst, buf, w, h)
	}
}

// tintStencil copies pix with every pixel set to ink, keeping the source's
// alpha as the coverage.
//
// The ink's OWN alpha multiplies it rather than replacing it: an ink that is
// half transparent gives a half-transparent icon, and an opaque one -- which is
// every theme colour here -- leaves the stencil exactly as it was drawn.
func tintStencil(pix []byte, w, h int, ink RGBA) []byte {
	out := make([]byte, w*h*4)
	for i := 0; i+3 < w*h*4; i += 4 {
		out[i], out[i+1], out[i+2] = ink.R, ink.G, ink.B
		out[i+3] = byte(int(pix[i+3]) * int(ink.A) / 255)
	}
	return out
}
