// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestScrollbarThumbGeometry(t *testing.T) {
	// Vertical: viewport is half the content → thumb ~half the track; offset at
	// the bottom pins the thumb to the end.
	sb := NewScrollbar()
	sb.Total, sb.Viewport = 200, 100
	sb.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})

	sb.Offset = 0
	top := sb.ThumbRect()
	if top.H != 50 || top.Y != 0 {
		t.Fatalf("top thumb = %+v, want H=50 Y=0", top)
	}
	sb.Offset = 100 // max (Total-Viewport)
	bot := sb.ThumbRect()
	if bot.Y+bot.H != 100 {
		t.Fatalf("bottom thumb should reach the track end: %+v", bot)
	}
	sb.Offset = 50 // middle
	mid := sb.ThumbRect()
	if !(mid.Y > top.Y && mid.Y < bot.Y) {
		t.Fatalf("mid thumb %d not between %d and %d", mid.Y, top.Y, bot.Y)
	}

	// Over/under-shoot offsets clamp.
	sb.Offset = -20
	if sb.ThumbRect().Y != 0 {
		t.Fatal("negative offset must clamp to the top")
	}
	sb.Offset = 9999
	if r := sb.ThumbRect(); r.Y+r.H != 100 {
		t.Fatalf("huge offset must clamp to the bottom: %+v", r)
	}
}

func TestScrollbarEverythingFitsAndMinThumb(t *testing.T) {
	// Everything visible → thumb fills the track.
	sb := NewScrollbar()
	sb.Total, sb.Viewport = 80, 100 // viewport >= total
	sb.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	if r := sb.ThumbRect(); r.H != 100 {
		t.Fatalf("thumb should fill the track when all fits: %+v", r)
	}

	// A tiny viewport in huge content still yields a grabbable minimum thumb.
	sb.Total, sb.Viewport = 100000, 100
	if r := sb.ThumbRect(); r.H != scrollbarMinThumb {
		t.Fatalf("thumb = %d, want min %d", r.H, scrollbarMinThumb)
	}

	// Degenerate totals default to 1 (no divide-by-zero).
	sb.Total, sb.Viewport = 0, 0
	if r := sb.ThumbRect(); r.H <= 0 {
		t.Fatalf("degenerate totals should still yield a thumb: %+v", r)
	}
}

func TestScrollbarHorizontal(t *testing.T) {
	sb := NewScrollbar()
	sb.Horizontal = true
	sb.Total, sb.Viewport, sb.Offset = 300, 100, 200
	sb.SetBounds(Rect{X: 10, Y: 0, W: 120, H: 8})
	r := sb.ThumbRect()
	if r.W >= 120 || r.X+r.W != 130 {
		t.Fatalf("horizontal thumb at max offset should reach the right edge: %+v", r)
	}
}

func TestScrollbarTinyTrack(t *testing.T) {
	// A track shorter than the minimum thumb: the min-thumb bump would exceed the
	// track, so the thumb clamps back down to the track length.
	sb := NewScrollbar()
	sb.Total, sb.Viewport = 1000, 100
	sb.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 12}) // 12 < scrollbarMinThumb (24)
	if r := sb.ThumbRect(); r.H != 12 {
		t.Fatalf("thumb should clamp to the tiny track height 12, got %d", r.H)
	}
}

func TestBlendRGBA(t *testing.T) {
	a := RGBA{R: 200, G: 200, B: 200, A: 255}
	b := RGBA{R: 0, G: 0, B: 0, A: 255}
	if got := blendRGBA(a, b, 1.5); got != a {
		t.Fatalf("t>1 should clamp to a, got %+v", got)
	}
	if got := blendRGBA(a, b, -1); got != b {
		t.Fatalf("t<0 should clamp to b, got %+v", got)
	}
	if mid := blendRGBA(a, b, 0.5); mid.R != 100 || mid.A != 255 {
		t.Fatalf("mid blend = %+v, want R≈100 opaque", mid)
	}
}

func TestScrollbarHorizontalDraw(t *testing.T) {
	// A horizontal bar exercises the r.H < r.W radius branch in Draw.
	sb := NewScrollbar()
	sb.Horizontal = true
	sb.Total, sb.Viewport, sb.Offset = 300, 100, 50
	sb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 8})
	buf := makeSurface(120, 8)
	sb.Draw(newP(buf, 120), DefaultLight())
	painted := false
	for _, v := range buf {
		if v != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("horizontal scrollbar drew nothing")
	}
}

func TestScrollbarDrawAndInert(t *testing.T) {
	sb := NewScrollbar()
	sb.Total, sb.Viewport, sb.Offset = 200, 100, 40
	sb.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 100})
	buf := makeSurface(8, 100)
	sb.Draw(newP(buf, 8), DefaultLight())
	// The thumb (Border ink) must have painted some non-track pixels.
	painted := false
	for _, b := range buf {
		if b != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("scrollbar drew nothing")
	}
	// Grabbable: a point inside its bounds hit-tests true (so a container routes
	// drags to it), while a zero-size widget draws nothing and hit-tests false.
	if !sb.HitTest(4, 40) {
		t.Fatal("scrollbar inside its bounds should hit-test true")
	}
	empty := NewScrollbar()
	if empty.HitTest(0, 0) {
		t.Fatal("zero-bounds scrollbar must not hit-test")
	}
	empty.Draw(newP(makeSurface(1, 1), 1), DefaultLight()) // no panic on zero bounds
	if r := empty.ThumbRect(); r != (Rect{}) {
		t.Fatalf("zero-bounds thumb should be empty: %+v", r)
	}
}
