// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-opentype/fonts/notosansthai"
	"github.com/go-widgets/painter"
)

// stubFont is a non-TrueType Font used to exercise NewFallbackFont's type check.
type stubFont struct{}

func (stubFont) Advance() int                                 { return 1 }
func (stubFont) Height() int                                  { return 1 }
func (stubFont) Measure(string) int                           { return 1 }
func (stubFont) Draw(painter.Painter, int, int, string, RGBA) {}

func TestNewFallbackFontErrors(t *testing.T) {
	if _, err := NewFallbackFont(); err == nil {
		t.Fatal("no fonts should error")
	}
	if _, err := NewFallbackFont(stubFont{}); err == nil {
		t.Fatal("a non-TrueType font should error")
	}
}

func TestFallbackRoutesByCoverage(t *testing.T) {
	latin := newTTFace(t, testFontTTF, 20) // Go Regular: Latin, no Thai
	thai := newTTFace(t, notosansthai.TTF, 20)

	f, err := NewFallbackFont(latin, thai)
	if err != nil {
		t.Fatal(err)
	}
	fb := f.(*fallbackFont)

	// A Latin rune stays on the primary; a Thai rune (absent from Go Regular)
	// routes to the Thai fallback.
	if fb.pick('A') != latin {
		t.Fatal("Latin 'A' should stay on the primary face")
	}
	if !thai.covers('ก') {
		t.Fatal("test precondition: Thai face must cover ก")
	}
	if latin.covers('ก') {
		t.Fatal("test precondition: Go Regular must NOT cover ก")
	}
	if fb.pick('ก') != thai {
		t.Fatal("Thai rune should route to the Thai fallback")
	}
	// A rune no face covers (an astral emoji) falls back to the primary.
	if fb.pick('🙂') != latin {
		t.Fatal("uncovered rune should fall back to the primary")
	}

	// Metrics come from the primary.
	if f.Advance() != latin.advance || f.Height() != latin.height {
		t.Fatal("fallback metrics must mirror the primary")
	}
}

func TestFallbackMixedRunsMeasureAndDraw(t *testing.T) {
	latin := newTTFace(t, testFontTTF, 20)
	thai := newTTFace(t, notosansthai.TTF, 20)
	f, _ := NewFallbackFont(latin, thai)
	fb := f.(*fallbackFont)

	const mixed = "Hi ไทย!" // Latin + Thai + Latin → three runs, two faces
	runs := fb.runs(mixed)
	if len(runs) != 3 || runs[0].font != latin || runs[1].font != thai || runs[2].font != latin {
		t.Fatalf("run split = %d runs, want Latin/Thai/Latin", len(runs))
	}
	// Measure sums the runs.
	if got, want := f.Measure(mixed), latin.Measure("Hi ")+thai.Measure("ไทย")+latin.Measure("!"); got != want {
		t.Fatalf("Measure = %d, want %d", got, want)
	}

	// Draw on a pixel painter paints something (the Thai glyphs the primary lacks).
	const w, h = 160, 30
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), 2, 4, mixed, RGBA{R: 0, G: 0, B: 0, A: 255})
	painted := false
	for _, b := range buf {
		if b != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("mixed Latin+Thai text drew nothing")
	}

	// Empty text measures 0 and draws nothing (no panic).
	if f.Measure("") != 0 {
		t.Fatal("empty measures 0")
	}
	f.Draw(newP(makeSurface(4, 4), 4), 0, 0, "", RGBA{A: 255})
}

func TestFallbackNonPixelPainter(t *testing.T) {
	latin := newTTFace(t, testFontTTF, 20)
	thai := newTTFace(t, notosansthai.TTF, 20)
	f, _ := NewFallbackFont(latin, thai)
	// A CellPainter is not a *PixelPainter → the bidi visual-order text path.
	cp := painter.NewCellPainter(20, 2)
	f.Draw(cp, 0, 0, "Hi ไทย", RGBA{A: 255}) // must not panic
}
