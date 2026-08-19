// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func TestNewSyntheticItalicFontNil(t *testing.T) {
	if _, err := NewSyntheticItalicFont(nil); err == nil {
		t.Fatal("NewSyntheticItalicFont(nil) = nil error, want error")
	}
}

func TestSyntheticItalicMetricsForwardBitmap(t *testing.T) {
	base := NewBitmapFont(2)
	f, err := NewSyntheticItalicFont(base)
	if err != nil {
		t.Fatal(err)
	}
	if f.Advance() != base.Advance() {
		t.Errorf("Advance = %d, want %d", f.Advance(), base.Advance())
	}
	if f.Height() != base.Height() {
		t.Errorf("Height = %d, want %d", f.Height(), base.Height())
	}
	if f.Measure("abc") != base.Measure("abc") {
		t.Errorf("Measure = %d, want %d", f.Measure("abc"), base.Measure("abc"))
	}
	// The bitmap font exposes no Ascent / FontData / SizePx, so the wrapper
	// reports the zero values.
	it := f.(*syntheticItalicFont)
	if it.Ascent() != 0 || it.FontData() != nil || it.SizePx() != 0 {
		t.Errorf("bitmap forwards = (%d,%v,%d), want (0,nil,0)", it.Ascent(), it.FontData(), it.SizePx())
	}
}

func TestSyntheticItalicMetricsForwardTTF(t *testing.T) {
	base, err := NewTrueTypeFont(testFontTTF, 20)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := NewSyntheticItalicFont(base)
	it := f.(*syntheticItalicFont)
	if it.Ascent() <= 0 {
		t.Errorf("Ascent = %d, want > 0", it.Ascent())
	}
	if len(it.FontData()) == 0 {
		t.Error("FontData empty, want the sfnt bytes")
	}
	if it.SizePx() != 20 {
		t.Errorf("SizePx = %d, want 20", it.SizePx())
	}
	// Draw must not panic on a TTF base (it falls back to the painter's own
	// Text primitive through the shear wrapper).
	buf := make([]byte, 4*40*40)
	f.Draw(painter.NewPixelPainter(buf, 40, 40), 2, 2, "Ay", RGBA{R: 0xFF, A: 0xFF})
}

func TestSyntheticItalicDrawSlantsBitmap(t *testing.T) {
	base := NewBitmapFont(4) // tall enough for the slant to be visible
	f, _ := NewSyntheticItalicFont(base)
	const w, h = 60, 40
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	f.Draw(p, 2, 2, "I", RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})

	minY, maxY := 1<<30, -1
	topMinX, botMinX := 1<<30, 1<<30
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if buf[(y*w+x)*4+3] == 0 {
				continue
			}
			if y < minY {
				minY, topMinX = y, x
			} else if y == minY && x < topMinX {
				topMinX = x
			}
			if y > maxY {
				maxY, botMinX = y, x
			} else if y == maxY && x < botMinX {
				botMinX = x
			}
		}
	}
	if maxY < 0 {
		t.Fatal("nothing painted")
	}
	if topMinX <= botMinX {
		t.Errorf("top row min x %d not right of bottom row min x %d — no slant", topMinX, botMinX)
	}
}
