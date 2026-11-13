// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// magentaBox is the bounding box of the exact-magenta pixels of buf.
func magentaBox(buf []byte, w, h int) (x0, y0, x1, y1 int, any bool) {
	x0, y0, x1, y1 = w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if buf[i] == 0xFF && buf[i+1] == 0 && buf[i+2] == 0xFF {
				x0, y0, x1, y1, any = min(x0, x), min(y0, y), max(x1, x), max(y1, y), true
			}
		}
	}
	return
}

// A co-editor's caret is as wide and as tall, in logical pixels, at MetricScale
// 2 as at 1: its overhang over the glyphs scales with them, as the local
// caret's does.
func TestRemoteCaretScalesWithTheMetricScale(t *testing.T) {
	defer SetMetricScale(MetricScale())
	magenta := RGBA{R: 0xFF, B: 0xFF, A: 0xFF}
	size := func(scale float64) (w, h int) {
		SetMetricScale(scale)
		const W, H = 200, 120
		v := NewTextView("hello world")
		v.SetBounds(Rect{W: W, H: H})
		v.Decorations = []Decoration{{Color: magenta, CursorLine: 0, CursorCol: 3}} // no tag: the caret alone
		buf := makeSurface(W, H)
		v.Draw(newP(buf, W), DefaultDark())
		x0, y0, x1, y1, ok := magentaBox(buf, W, H)
		if !ok {
			t.Fatalf("scale %v: no co-editor caret painted", scale)
		}
		return x1 - x0 + 1, y1 - y0 + 1
	}
	w1, h1 := size(1)
	w2, h2 := size(2)
	if w1 != 2 {
		t.Errorf("caret width %d at scale 1, want the historical 2", w1)
	}
	if w2 != 2*w1 || h2 != 2*h1 {
		t.Errorf("caret %dx%d at scale 2, %dx%d at scale 1: want double", w2, h2, w1, h1)
	}
}

// The tag over the caret scales too: its padding and its offset.
func TestRemoteCaretTagScalesWithTheMetricScale(t *testing.T) {
	defer SetMetricScale(MetricScale())
	magenta := RGBA{R: 0xFF, B: 0xFF, A: 0xFF}
	box := func(scale float64) (x0, y0, x1, y1 int) {
		SetMetricScale(scale)
		const W, H = 300, 160
		v := NewTextView("\n\nhello world")
		v.SetBounds(Rect{W: W, H: H})
		v.Decorations = []Decoration{{Label: "Bob", Color: magenta, CursorLine: 2, CursorCol: 3}}
		buf := makeSurface(W, H)
		v.Draw(newP(buf, W), DefaultDark())
		x0, y0, x1, y1, ok := magentaBox(buf, W, H)
		if !ok {
			t.Fatalf("scale %v: nothing painted", scale)
		}
		return
	}
	a0, b0, a1, b1 := box(1)
	c0, d0, c1, d1 := box(2)
	// the union of caret and tag is as tall, in logical pixels, at both scales
	if h1, h2 := b1-b0+1, d1-d0+1; h2 != 2*h1 {
		t.Errorf("caret plus tag is %d tall at scale 2, %d at scale 1: want double", h2, h1)
	}
	if w1, w2 := a1-a0+1, c1-c0+1; w2 < 2*w1-2 || w2 > 2*w1+2 {
		t.Errorf("caret plus tag is %d wide at scale 2, %d at scale 1: want about double", w2, w1)
	}
}
