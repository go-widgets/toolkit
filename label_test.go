// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

// TestLabelInkOverride checks that a non-zero Ink paints the text in that
// colour, while the zero value inherits the theme's OnSurface (backward compat).
func TestLabelInkOverride(t *testing.T) {
	theme := DefaultLight()
	const w, h = 120, 24
	const text = "Ink"

	// (a) Default: no Ink → OnSurface. This is the reference render.
	def := NewLabel(text)
	def.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	defBuf := makeSurface(w, h)
	def.Draw(newP(defBuf, w), theme)

	// (b) Ink explicitly set to OnSurface must be byte-identical to the default
	// (the A!=0 branch resolves to the same colour).
	same := NewLabel(text)
	same.Ink = theme.OnSurface
	same.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	sameBuf := makeSurface(w, h)
	same.Draw(newP(sameBuf, w), theme)
	if !bytes.Equal(defBuf, sameBuf) {
		t.Fatal("Ink==OnSurface must render byte-identically to the default")
	}

	// (c) A distinct Ink must change the rendered pixels.
	red := NewLabel(text)
	red.Ink = RGBA{R: 220, G: 20, B: 20, A: 255}
	red.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	redBuf := makeSurface(w, h)
	red.Draw(newP(redBuf, w), theme)
	if bytes.Equal(defBuf, redBuf) {
		t.Fatal("a distinct Ink colour must change the rendered text")
	}
	// The red channel must appear somewhere in the glyph pixels.
	found := false
	for i := 0; i+3 < len(redBuf); i += 4 {
		if redBuf[i] > 150 && redBuf[i+1] < 100 && redBuf[i+2] < 100 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Ink red not present in the rendered label")
	}
}
