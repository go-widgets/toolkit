// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

var (
	lvlRed   = RGB(0xC0, 0x30, 0x30)
	lvlAmber = RGB(0xE0, 0xA0, 0x30)
	lvlGreen = RGB(0x2E, 0x8B, 0x57)
)

// firstCellColor samples the centre of the first (leftmost) cell of a
// horizontal LevelBar drawn at (0,0,w,h).
func firstCellColor(t *testing.T, l *LevelBar, w, h int) RGBA {
	t.Helper()
	l.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	l.Draw(newP(buf, w), DefaultLight())
	return pixelAt(buf, w, 1, h/2)
}

// TestLevelBarThresholdBandSelected checks the filled cells take the colour of
// the band whose Min is the greatest not exceeding Value.
func TestLevelBarThresholdBandSelected(t *testing.T) {
	mk := func() *LevelBar {
		l := NewLevelBar(10)
		l.Thresholds = []LevelThreshold{{0, lvlRed}, {4, lvlAmber}, {8, lvlGreen}}
		return l
	}
	// Value 5 -> amber band (4 <= 5 < 8).
	l := mk()
	l.Value = 5
	if got := firstCellColor(t, l, 120, 12); got != lvlAmber {
		t.Fatalf("Value=5 fill = %+v, want amber", got)
	}
	// Value 9 -> green band (highest Min 8 <= 9).
	l = mk()
	l.Value = 9
	if got := firstCellColor(t, l, 120, 12); got != lvlGreen {
		t.Fatalf("Value=9 fill = %+v, want green", got)
	}
	// Value 2 -> red band (Min 0).
	l = mk()
	l.Value = 2
	if got := firstCellColor(t, l, 120, 12); got != lvlRed {
		t.Fatalf("Value=2 fill = %+v, want red", got)
	}
}

// TestLevelBarThresholdUnorderedAndUnmatched covers fillColor's ordering
// (thresholds not sorted) and the no-match fallback to Accent.
func TestLevelBarThresholdUnorderedAndUnmatched(t *testing.T) {
	// Unordered thresholds: {8,green} first still wins for Value=9.
	l := NewLevelBar(10)
	l.Thresholds = []LevelThreshold{{8, lvlGreen}, {0, lvlRed}, {4, lvlAmber}}
	l.Value = 9
	if got := firstCellColor(t, l, 120, 12); got != lvlGreen {
		t.Fatalf("unordered thresholds fill = %+v, want green", got)
	}
	// No band matches (all Min > Value) -> Accent fallback.
	theme := DefaultLight()
	l2 := NewLevelBar(10)
	l2.Thresholds = []LevelThreshold{{5, lvlGreen}}
	l2.Value = 3
	if got := firstCellColor(t, l2, 120, 12); got != theme.Accent {
		t.Fatalf("no-match fill = %+v, want Accent %+v", got, theme.Accent)
	}
	// No thresholds at all -> Accent (backward compat).
	l3 := NewLevelBar(10)
	l3.Value = 3
	if got := firstCellColor(t, l3, 120, 12); got != theme.Accent {
		t.Fatalf("no-threshold fill = %+v, want Accent", got)
	}
}

// TestLevelBarLabelDrawn checks a horizontal Label paints in OnSurface ink and
// that an empty Label leaves the render byte-identical.
func TestLevelBarLabelDrawn(t *testing.T) {
	const w, h = 120, 16
	theme := DefaultLight()

	plain := NewLevelBar(10)
	plain.Value = 7
	plain.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	plainBuf := makeSurface(w, h)
	plain.Draw(newP(plainBuf, w), theme)

	labelled := NewLevelBar(10)
	labelled.Value = 7
	labelled.Label = "70%"
	labelled.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	labBuf := makeSurface(w, h)
	labelled.Draw(newP(labBuf, w), theme)

	if bytes.Equal(plainBuf, labBuf) {
		t.Fatal("a Label must change the rendered pixels")
	}
	found := false
	for i := 0; i+3 < len(labBuf); i += 4 {
		if (RGBA{R: labBuf[i], G: labBuf[i+1], B: labBuf[i+2], A: labBuf[i+3]}) == theme.OnSurface {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no OnSurface label pixel found")
	}
}
