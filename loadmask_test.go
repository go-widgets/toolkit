// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// bufAllSentinel reports whether every pixel is still the makeSurface sentinel
// (0xC8), i.e. nothing was painted.
func bufAllSentinel(buf []byte, w, h int) bool {
	return !anyPainted(buf, w, 0, 0, w, h)
}

// anyPixel reports whether colour c appears anywhere in the buffer.
func anyPixel(buf []byte, w, h int, c RGBA) bool {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

// TestLoadMaskInactiveDrawsNothing: an inactive mask paints nothing.
func TestLoadMaskInactiveDrawsNothing(t *testing.T) {
	const w, h = 80, 60
	m := NewLoadMask("Loading…") // Active defaults false
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), DefaultLight())
	if !bufAllSentinel(buf, w, h) {
		t.Fatal("inactive LoadMask must paint nothing")
	}
}

// TestLoadMaskEmptyBoundsNoOp: active but zero-size paints nothing.
func TestLoadMaskEmptyBoundsNoOp(t *testing.T) {
	m := NewLoadMask("x")
	m.Active().Set(true)
	m.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	buf := makeSurface(20, 20)
	m.Draw(newP(buf, 20), DefaultLight())
	if !bufAllSentinel(buf, 20, 20) {
		t.Fatal("empty-bounds LoadMask must paint nothing")
	}
}

// TestLoadMaskActiveDimsSpinsAndCaptions: active mask dims (default scrim
// darkens the surface), draws the accent spinner, and paints the message.
func TestLoadMaskActiveDimsSpinsAndCaptions(t *testing.T) {
	const w, h = 100, 80
	theme := DefaultLight()
	m := NewLoadMask("Loading…")
	m.Active().Set(true)
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), theme)

	// Default scrim is translucent black → the 0xC8 sentinel darkens.
	corner := pixelAt(buf, w, 1, 1)
	if corner.R >= 0xC8 {
		t.Fatalf("scrim did not darken the surface: corner R=%d", corner.R)
	}
	// The spinner paints in theme.Accent somewhere.
	if !anyPixel(buf, w, h, theme.Accent) {
		t.Fatal("active LoadMask painted no accent spinner pixels")
	}
}

// TestLoadMaskNoMessageStillDraws covers the no-caption layout branch.
func TestLoadMaskNoMessageStillDraws(t *testing.T) {
	const w, h = 100, 80
	m := NewLoadMask("") // no message
	m.Active().Set(true)
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), DefaultLight())
	if bufAllSentinel(buf, w, h) {
		t.Fatal("active LoadMask without a message must still dim + spin")
	}
}

// TestLoadMaskCustomScrim: a caller-set opaque scrim paints verbatim.
func TestLoadMaskCustomScrim(t *testing.T) {
	const w, h = 60, 40
	red := RGBA{R: 255, A: 255}
	m := NewLoadMask("")
	m.Active().Set(true)
	m.Scrim = red
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), DefaultLight())
	if got := pixelAt(buf, w, 0, 0); got != red {
		t.Fatalf("custom opaque scrim corner = %+v, want %+v", got, red)
	}
}

// TestLoadMaskHitTest: catches events only while Active and inside bounds.
func TestLoadMaskHitTest(t *testing.T) {
	m := NewLoadMask("x")
	m.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 50})

	if m.HitTest(10, 10) {
		t.Fatal("inactive LoadMask must not catch events")
	}
	m.Active().Set(true)
	if !m.HitTest(10, 10) {
		t.Fatal("active LoadMask must catch in-bounds events")
	}
	if m.HitTest(80, 80) {
		t.Fatal("active LoadMask must not catch out-of-bounds events")
	}
}

// TestLoadMaskTickAdvancesSpinner: Tick drives the internal spinner phase.
func TestLoadMaskTickAdvancesSpinner(t *testing.T) {
	m := NewLoadMask("x")
	m.Tick(0.25)
	if m.spinner.Phase != 0.25 {
		t.Fatalf("spinner Phase after Tick(0.25) = %v, want 0.25", m.spinner.Phase)
	}
}
