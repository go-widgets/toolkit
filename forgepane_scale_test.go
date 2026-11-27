// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A pane built at one scale and laid out at another has the row heights and the
// note wrap of the second, not the first.
func TestForgePaneRebuildsWhenTheScaleMoves(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(1)
	f := NewForgePane(newFakeForge())
	f.SetBounds(Rect{W: 300, H: 400})
	one := f.Parts()["repo"].H
	if one == 0 {
		t.Fatal("no repo row at 1x")
	}
	SetMetricScale(2)
	f.SetBounds(Rect{W: 600, H: 800})
	if two := f.Parts()["repo"].H; two != 2*one {
		t.Fatalf("repo row is %d at 2x, want %d: the layout kept the 1x heights", two, 2*one)
	}
}

func TestForgePaneRewrapsWhenOnlyTheScaleMoves(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(1)
	m := newFakeForge()
	f := NewForgePane(m)
	var cols []int
	f.NoteWrap = func(text string, c int) string { cols = append(cols, c); return text }
	f.SetBounds(Rect{W: 600, H: 400})
	m.Note().Set("a note")
	f.Draw(newP(makeSurface(600, 400), 600), DefaultDark())
	before := len(cols)
	f.Draw(newP(makeSurface(600, 400), 600), DefaultDark())
	if len(cols) != before {
		t.Fatal("an unchanged pane at an unchanged scale was wrapped again")
	}
	page := f.client
	SetMetricScale(2) // same width, bigger face: fewer columns fit
	f.Draw(newP(makeSurface(600, 400), 600), DefaultDark())
	if len(cols) == before {
		t.Fatal("the note was not re-wrapped after the scale changed")
	}
	_ = page
}

// A tiny scale must never ask the rasteriser for a zero-pixel face.
func TestLabelFontSizeNeverRoundsToZeroPixels(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(0.1)
	l := NewLabel("x")
	l.SetFont(newTTF(t, 14))
	l.SetFontSize(1)
	if l.faceFor().Height() < 1 {
		t.Fatal("the sized face collapsed to nothing")
	}
	if l.sizedFor != 1 {
		t.Fatalf("device size = %d, want the 1 px floor", l.sizedFor)
	}
}

// Drawing at an unchanged scale must not rebuild the layouts: that would throw
// away the controls' state on every frame.
func TestForgePaneKeepsItsLayoutsAtAnUnchangedScale(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(1)
	f := NewForgePane(newFakeForge())
	f.SetBounds(Rect{W: 300, H: 400})
	page := f.client
	f.Draw(newP(makeSurface(300, 400), 300), DefaultDark())
	f.SetBounds(Rect{W: 300, H: 400})
	if f.client != page {
		t.Fatal("the layouts were rebuilt although the scale did not move")
	}
	SetMetricScale(2)
	f.SetBounds(Rect{W: 600, H: 800})
	if f.client == page {
		t.Fatal("the layouts were not rebuilt after the scale moved")
	}
}
