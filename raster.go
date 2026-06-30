// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// fillRect paints (x, y, w, h) with c into an RGBA surface of stride
// surfaceW * 4 bytes. Coordinates may extend past the surface -- per-
// pixel clipping prevents OOB writes so callers can pass arbitrary
// rects (e.g. a partially-occluded button) without pre-clipping.
//
// Used by every widget's Draw; centralised here so a future SIMD
// fast-path lands once + benefits every widget.
func fillRect(surface []byte, surfaceW int, x, y, w, h int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	rowBytes := surfaceW * 4
	for j := 0; j < h; j++ {
		py := y + j
		if py < 0 {
			continue
		}
		rowOff := py * rowBytes
		if rowOff >= len(surface) {
			return
		}
		for i := 0; i < w; i++ {
			px := x + i
			if px < 0 || px >= surfaceW {
				continue
			}
			off := rowOff + px*4
			if off+3 >= len(surface) {
				continue
			}
			surface[off+0] = c.R
			surface[off+1] = c.G
			surface[off+2] = c.B
			surface[off+3] = c.A
		}
	}
}

// strokeRect paints a 1-pixel border on the outline of (x, y, w, h)
// with c. Used by widgets that draw a frame around their body --
// Button, GroupBox, focus indicator, etc.
func strokeRect(surface []byte, surfaceW int, x, y, w, h int, c RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	fillRect(surface, surfaceW, x, y, w, 1, c)
	fillRect(surface, surfaceW, x, y+h-1, w, 1, c)
	fillRect(surface, surfaceW, x, y, 1, h, c)
	fillRect(surface, surfaceW, x+w-1, y, 1, h, c)
}
