// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

const redSquareSVG = `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10" fill="#ff0000"/></svg>`

// TestSVGIconRenders rasterises a solid-red SVG and blits it into a box; the box
// must carry red. A second draw exercises the cache path.
func TestSVGIconRenders(t *testing.T) {
	const w, h = 24, 24
	draw := SVGIcon(redSquareSVG)
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	box := Rect{X: 4, Y: 4, W: 16, H: 16}
	draw(p, box, RGBA{R: 0, G: 0, B: 0, A: 255})

	// Centre of the box should be predominantly red.
	i := ((box.Y+box.H/2)*w + (box.X + box.W/2)) * 4
	if buf[i] < 180 || buf[i+1] > 80 || buf[i+2] > 80 {
		t.Fatalf("box centre = (%d,%d,%d,%d), want red", buf[i], buf[i+1], buf[i+2], buf[i+3])
	}

	// Second draw hits the cache (same result, no re-parse).
	buf2 := make([]byte, w*h*4)
	draw(painter.NewPixelPainter(buf2, w, h), box, RGBA{A: 255})
	j := ((box.Y+box.H/2)*w + (box.X + box.W/2)) * 4
	if buf2[j] < 180 {
		t.Fatalf("cached draw did not paint red: %d", buf2[j])
	}
}

// TestSVGIconBadDocDrawsNothing: an unparseable document caches empty and paints
// nothing, rather than panicking.
func TestSVGIconBadDocDrawsNothing(t *testing.T) {
	const w, h = 16, 16
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	SVGIcon("this is not an svg")(p, Rect{X: 0, Y: 0, W: w, H: h}, RGBA{A: 255})
	for _, b := range buf {
		if b != 0 {
			t.Fatal("a bad SVG should paint nothing")
		}
	}
}

// TestSVGIconGuards covers the early-out branches: a zero-area box and a painter
// that cannot blit an image both draw nothing.
func TestSVGIconGuards(t *testing.T) {
	draw := SVGIcon(redSquareSVG)
	// Zero-area box: returns before rasterising.
	draw(painter.NewPixelPainter(make([]byte, 4), 1, 1), Rect{X: 0, Y: 0, W: 0, H: 10}, RGBA{A: 255})

	// A painter that is not an ImagePainter (nonClipPainter has no DrawImage).
	const w, h = 16, 16
	buf := make([]byte, w*h*4)
	np := &nonClipPainter{inner: painter.NewPixelPainter(buf, w, h)}
	draw(np, Rect{X: 0, Y: 0, W: w, H: h}, RGBA{A: 255})
	for _, b := range buf {
		if b != 0 {
			t.Fatal("a non-ImagePainter should paint nothing")
		}
	}
}

// TestSVGViewBoxLongestSide covers the viewBox scan: a valid box returns its
// larger side; the many malformed/absent forms return 0.
func TestSVGViewBoxLongestSide(t *testing.T) {
	for _, tc := range []struct {
		doc  string
		want float64
	}{
		{`<svg viewBox="0 0 24 32">`, 32},
		{`<svg viewBox='0 -960 960 940'>`, 960},
		{`<svg viewBox="0,0,10,50">`, 50}, // comma-separated
		{`<svg width="10">`, 0},           // no viewBox
		{`<svg viewBox=`, 0},              // no quote
		{`<svg viewBox="0 0 24 32`, 0},    // unclosed quote
		{`<svg viewBox="0 0 24">`, 0},     // wrong field count
		{`<svg viewBox="0 0 x 32">`, 0},   // non-numeric
		{`<svg viewBox="0 0 0 32">`, 0},   // non-positive width
	} {
		if got := svgViewBoxLongestSide(tc.doc); got != tc.want {
			t.Errorf("svgViewBoxLongestSide(%q) = %g, want %g", tc.doc, got, tc.want)
		}
	}
}

// TestIconScaleBoundsLargeViewBox: an icon on a large grid rasterises to about
// iconRasterPx, not viewBox-sized — iconScale shrinks it, and a small grid falls
// back to the rasteriser default.
func TestIconScaleBoundsLargeViewBox(t *testing.T) {
	if s := iconScale(`<svg viewBox="0 0 960 960"><rect/></svg>`); s <= 0 || s > 0.1 {
		t.Errorf("large viewBox scale = %g, want a small positive (~%g/960)", s, iconRasterPx)
	}
	if s := iconScale(`<svg><rect/></svg>`); s != 0 {
		t.Errorf("no-viewBox scale = %g, want 0 (rasteriser default)", s)
	}
	// End to end: a 960-grid SVG rasterises to a bounded ~iconRasterPx square.
	res := rasterizeIcon(`<svg viewBox="0 0 960 960"><rect width="960" height="960" fill="#000"/></svg>`, RGBA{A: 255})
	if res.w <= 0 || res.w > int(iconRasterPx)+2 {
		t.Errorf("rastered width = %d, want ~%g (bounded)", res.w, iconRasterPx)
	}
}
