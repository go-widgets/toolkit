// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestFontChooserDefaultsToScaleLadder(t *testing.T) {
	defer SetFont(nil)
	fc := NewFontChooser(nil)
	if len(fc.Options) != 3 {
		t.Fatalf("default options = %d, want 3", len(fc.Options))
	}
	// The ladder grows in height: Regular < Large < Extra Large.
	if !(fc.rowHeight(0) < fc.rowHeight(1) && fc.rowHeight(1) < fc.rowHeight(2)) {
		t.Errorf("rows not increasing: %d %d %d", fc.rowHeight(0), fc.rowHeight(1), fc.rowHeight(2))
	}
}

func TestFontChooserCustomOptions(t *testing.T) {
	defer SetFont(nil)
	fc := NewFontChooser([]FontOption{{Name: "Only", Font: NewBitmapFont(1)}})
	if len(fc.Options) != 1 || fc.Options[0].Name != "Only" {
		t.Fatalf("custom options not preserved: %+v", fc.Options)
	}
}

func TestFontChooserDrawSelectedBand(t *testing.T) {
	defer SetFont(nil)
	fc := NewFontChooser(nil)
	fc.Selected = 1
	fc.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100})
	surf := makeSurface(120, 100)
	fc.Draw(newP(surf, 120), DefaultLight())
	// The selected row draws an Accent band somewhere.
	if got := countInk(surf, 120, 100, DefaultLight().Accent); got == 0 {
		t.Error("selected row drew no accent band")
	}
}

func TestFontChooserClickSelectsAndAppliesFont(t *testing.T) {
	defer SetFont(nil) // restore default (this test mutates global font state)
	var chosenIdx int
	var chosenFont Font
	fc := NewFontChooser(nil)
	fc.OnChoose = func(idx int, f Font) { chosenIdx, chosenFont = idx, f }
	fc.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})

	// Click into the SECOND row (Large, 2×). Its y-range starts after row 0.
	yRow1 := FontChooserPad + fc.rowHeight(0) + FontChooserRowPad
	fc.OnEvent(Event{Kind: EventClick, X: 10, Y: yRow1})

	if fc.Selected != 1 || chosenIdx != 1 {
		t.Fatalf("Selected=%d chosenIdx=%d, want 1", fc.Selected, chosenIdx)
	}
	// The active font is now the chosen one → GlyphHeight doubled.
	if GlyphHeight() != 2*baseGlyphHeight {
		t.Errorf("SetFont not applied: GlyphHeight=%d, want %d", GlyphHeight(), 2*baseGlyphHeight)
	}
	if chosenFont != fc.Options[1].Font {
		t.Error("OnChoose received the wrong font")
	}
}

func TestFontChooserClickInPaddingIgnored(t *testing.T) {
	defer SetFont(nil)
	fc := NewFontChooser(nil)
	fc.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	// y=0 is inside the top padding (< FontChooserPad+row0), and far below every
	// row is past the last → both rowAt=-1, a no-op.
	fc.OnEvent(Event{Kind: EventClick, X: 10, Y: 0})
	fc.OnEvent(Event{Kind: EventClick, X: 10, Y: 5000})
	if fc.Selected != 0 {
		t.Errorf("padding click changed Selected to %d", fc.Selected)
	}
}

func TestFontChooserNonClickIgnored(t *testing.T) {
	defer SetFont(nil)
	fc := NewFontChooser(nil)
	fc.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	fc.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fc.Selected != 0 {
		t.Error("non-click event changed selection")
	}
}
