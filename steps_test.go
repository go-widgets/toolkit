// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewStepsStoresFields(t *testing.T) {
	s := NewSteps([]string{"a", "b"}, 1)
	if len(s.Labels) != 2 || s.Current != 1 {
		t.Fatalf("NewSteps round-trip broken: %+v", s)
	}
}

// --- Draw branches -------------------------------------------------------

// Empty labels: the early return fires; buffer stays untouched.
func TestStepsDrawEmptyLabelsEarlyReturn(t *testing.T) {
	const w, h = 40, 20
	theme := DefaultLight()
	s := NewSteps(nil, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y).R != 0xC8 {
				t.Fatalf("empty Steps painted (%d,%d) = %+v",
					x, y, pixelAt(buf, w, x, y))
			}
		}
	}
}

// Single label: no connector drawn (the `i > 0` branch stays false).
func TestStepsDrawSingleLabelNoConnector(t *testing.T) {
	const w, h = 80, 40
	theme := DefaultLight()
	s := NewSteps([]string{"only"}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// A badge should be painted at (or near) x = 0. The current-step
	// branch fires (Current == 0) so the badge fill is Accent.
	if pixelAt(buf, w, StepBoxW/2, 12+StepBoxH/2) != theme.Accent &&
		pixelAt(buf, w, StepBoxW/2, StepBoxH/2) != theme.Accent {
		// One of the two centred / uncentred layouts must show Accent.
		t.Fatalf("single-badge fill NOT Accent anywhere; got %+v / %+v",
			pixelAt(buf, w, StepBoxW/2, 12+StepBoxH/2),
			pixelAt(buf, w, StepBoxW/2, StepBoxH/2))
	}
}

// Current == 0 with multiple labels: badge 0 is Accent, badges 1..n-1
// are SurfaceAlt. Also exercises the connector-drawing branch.
func TestStepsDrawCurrentZeroWithMultipleLabels(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	s := NewSteps([]string{"", "", ""}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	yMid := 12 + StepBoxH/2 // vertical centre of badge row
	// Badge 0 centre pixel is Accent.
	if pixelAt(buf, w, StepBoxW/2, yMid) != theme.Accent {
		t.Fatalf("badge0 fill = %+v, want Accent", pixelAt(buf, w, StepBoxW/2, yMid))
	}
	// Badge 1 centre pixel is SurfaceAlt (pending).
	badge1X := StepBoxW + StepConnectorW + StepBoxW/2
	if pixelAt(buf, w, badge1X, yMid) != theme.SurfaceAlt {
		t.Fatalf("badge1 fill = %+v, want SurfaceAlt", pixelAt(buf, w, badge1X, yMid))
	}
	// Connector row at yMid between badge0 and badge1 is Border.
	connX := StepBoxW + StepConnectorW/2
	if pixelAt(buf, w, connX, yMid) != theme.Border {
		t.Fatalf("connector pixel = %+v, want Border", pixelAt(buf, w, connX, yMid))
	}
}

// Current == len-1: every badge is Accent (all done).
func TestStepsDrawCurrentLast(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	s := NewSteps([]string{"", "", ""}, 2)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	yMid := 12 + StepBoxH/2
	badge2X := 2*(StepBoxW+StepConnectorW) + StepBoxW/2
	if pixelAt(buf, w, badge2X, yMid) != theme.Accent {
		t.Fatalf("last-badge fill = %+v, want Accent", pixelAt(buf, w, badge2X, yMid))
	}
}

// Current < 0 (out of range low): every badge is SurfaceAlt (pending).
func TestStepsDrawCurrentBelowRange(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	s := NewSteps([]string{"", "", ""}, -1)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	yMid := 12 + StepBoxH/2
	// Badge 0 pending -> SurfaceAlt.
	if pixelAt(buf, w, StepBoxW/2, yMid) != theme.SurfaceAlt {
		t.Fatalf("Current<0 badge0 fill = %+v, want SurfaceAlt",
			pixelAt(buf, w, StepBoxW/2, yMid))
	}
}

// Current >= len (out of range high): every badge is Accent (all done).
func TestStepsDrawCurrentAboveRange(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	s := NewSteps([]string{"", "", ""}, 99)
	s.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	yMid := 12 + StepBoxH/2
	badge2X := 2*(StepBoxW+StepConnectorW) + StepBoxW/2
	if pixelAt(buf, w, badge2X, yMid) != theme.Accent {
		t.Fatalf("Current>len last-badge fill = %+v, want Accent",
			pixelAt(buf, w, badge2X, yMid))
	}
}

// Labels with non-empty caption: the caption branch fires and paints
// OnBackground ink below the badge.
func TestStepsDrawWithCaption(t *testing.T) {
	const w, h = 80, 40
	theme := DefaultLight()
	s := NewSteps([]string{"Go"}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Caption sits at y = badgeY + StepBoxH + StepLabelGap for one row.
	// badgeY = (H - StepBoxH)/2 = (40-16)/2 = 12.
	captionY := 12 + StepBoxH + StepLabelGap
	painted := 0
	for y := captionY; y < captionY+GlyphHeight(); y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("captioned Steps painted 0 OnBackground pixels below the badge")
	}
}

// Labels with empty caption: the caption branch is skipped — no ink
// lands below the badge. Combined with the previous test this covers
// both sides of the `if lab != ""` guard.
func TestStepsDrawEmptyCaptionSkipsBelow(t *testing.T) {
	const w, h = 80, 40
	theme := DefaultLight()
	s := NewSteps([]string{""}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	captionY := 12 + StepBoxH + StepLabelGap
	for y := captionY; y < captionY+GlyphHeight(); y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				t.Fatalf("empty-caption Steps painted ink at (%d,%d)", x, y)
			}
		}
	}
}

// Tight bounds (r.H <= StepBoxH): the centring branch is skipped so
// the badge anchors at r.Y.
func TestStepsDrawTightBoundsSkipsCentring(t *testing.T) {
	const w, h = 40, StepBoxH + 2
	theme := DefaultLight()
	s := NewSteps([]string{""}, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: StepBoxH})
	buf := makeSurface(w, h)
	s.Draw(newP(buf, w), theme)
	// Top-left pixel of the badge = the border corner at (0,0).
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("tight-bounds badge corner = %+v, want Border",
			pixelAt(buf, w, 0, 0))
	}
}
