// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/go-widgets/painter"
)

// RenderImage is the toolkit's headless "screenshot" path: it renders any
// Widget into an in-memory *image.RGBA instead of a live pixel surface. The
// same Draw call a window or wasmbox surface would trigger runs here against
// a throwaway PixelPainter, so a widget doesn't need to know it's being
// captured rather than displayed. Callers use this to save a widget's
// appearance to disk, embed it in a generated report, or assert on pixels
// in a test.
//
// width and height must both be positive; a widget with zero (or negative)
// extent has no pixels to capture. On success the returned image is exactly
// width x height, with Background painted first so any pixel the widget
// itself doesn't touch (e.g. outside its own drawn shape) still comes out as
// the theme's canvas colour rather than transparent black.
func RenderImage(w Widget, width, height int, theme *Theme) (*image.RGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("toolkit: RenderImage: invalid dimensions %dx%d", width, height)
	}

	buf := make([]byte, 4*width*height)
	p := painter.NewPixelPainter(buf, width, height)
	p.FillRect(Rect{X: 0, Y: 0, W: width, H: height}, theme.Background)

	w.SetBounds(Rect{X: 0, Y: 0, W: width, H: height})
	w.Draw(p, theme)

	// PixelPainter's buffer is already laid out exactly like image.RGBA's
	// Pix: 4 bytes per pixel (R, G, B, A), row-major, stride = 4*Width. A
	// freshly allocated image.RGBA has Stride == 4*Rect.Dx(), so the two
	// buffers agree byte-for-byte and a straight copy suffices.
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, p.Buf)
	return img, nil
}

// RenderPNG renders w exactly as RenderImage does, then encodes the result
// as a PNG. It is the one-call path from a widget to bytes suitable for
// os.WriteFile, an HTTP response body, or embedding in a document.
//
// png.Encode into an in-memory bytes.Buffer does not fail in practice (it
// has no I/O to fail on), so its error is simply propagated rather than
// wrapped — the only error this function can realistically surface is
// RenderImage's dimension check.
//
// Encode runs as its own statement (not inlined into the return) so buf is
// fully populated before buf.Bytes() is evaluated: Go evaluates a return
// statement's operands left to right, and inlining would capture buf.Bytes()
// while the buffer was still empty.
func RenderPNG(w Widget, width, height int, theme *Theme) ([]byte, error) {
	img, err := RenderImage(w, width, height, theme)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	return buf.Bytes(), err
}
