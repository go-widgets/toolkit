// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestIconButtonNewDefaults(t *testing.T) {
	ib := NewIconButton("+", nil)
	if ib.Icon != "+" {
		t.Fatalf("Icon = %q, want %q", ib.Icon, "+")
	}
	if ib.OnClick != nil {
		t.Fatal("OnClick should default to the caller-supplied value (nil here)")
	}
}

func TestIconButtonNewEmptyIcon(t *testing.T) {
	ib := NewIconButton("", nil)
	if ib.Icon != "" {
		t.Fatalf("empty-icon constructor bad: %+v", *ib)
	}
}

// --- Draw: auto-size branches --------------------------------------------

func TestIconButtonAutoSizeZeroBounds(t *testing.T) {
	ib := NewIconButton("+", nil)
	theme := DefaultLight()
	surf := makeSurface(IconButtonSize, IconButtonSize)
	ib.Draw(newP(surf, IconButtonSize), theme)
	if ib.Bounds().W != IconButtonSize || ib.Bounds().H != IconButtonSize {
		t.Fatalf("auto-size bounds = %+v, want %dx%d", ib.Bounds(), IconButtonSize, IconButtonSize)
	}
}

func TestIconButtonAutoSizeZeroWidthPreservesHeight(t *testing.T) {
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 40})
	theme := DefaultLight()
	surf := makeSurface(IconButtonSize, 40)
	ib.Draw(newP(surf, IconButtonSize), theme)
	if ib.Bounds().W != IconButtonSize || ib.Bounds().H != 40 {
		t.Fatalf("auto-size with non-zero H = %+v, want %dx40", ib.Bounds(), IconButtonSize)
	}
}

func TestIconButtonPreSizedBoundsHonoured(t *testing.T) {
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	theme := DefaultLight()
	surf := makeSurface(40, 30)
	ib.Draw(newP(surf, 40), theme)
	if ib.Bounds().W != 40 || ib.Bounds().H != 30 {
		t.Fatalf("pre-sized bounds mutated to %+v", ib.Bounds())
	}
}

// --- Draw: face + border in light and dark themes ------------------------

func TestIconButtonDrawFaceLight(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	buf := makeSurface(w, h)
	ib.Draw(newP(buf, w), theme)
	// Top-left border corner.
	if got := pixelAt(buf, w, 0, 0); got != theme.Border {
		t.Fatalf("border corner = %+v, want Border %+v", got, theme.Border)
	}
	// Interior face pixel, off-centre to avoid the centred glyph.
	if got := pixelAt(buf, w, 2, 2); got != theme.Surface {
		t.Fatalf("face pixel = %+v, want Surface %+v", got, theme.Surface)
	}
}

func TestIconButtonDrawFaceDark(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultDark()
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	buf := makeSurface(w, h)
	ib.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 2, 2); got != theme.Surface {
		t.Fatalf("dark face = %+v, want Surface %+v", got, theme.Surface)
	}
	if got := pixelAt(buf, w, 0, 0); got != theme.Border {
		t.Fatalf("dark border corner = %+v, want Border %+v", got, theme.Border)
	}
}

// --- Draw: empty-icon branch ---------------------------------------------

func TestIconButtonDrawEmptyIcon(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	ib := NewIconButton("", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	buf := makeSurface(w, h)
	ib.Draw(newP(buf, w), theme) // no panic; the label branch is skipped
	// Centre pixel should be Surface (no glyph drawn).
	if got := pixelAt(buf, w, 15, 15); got != theme.Surface {
		t.Fatalf("centre pixel with empty icon = %+v, want Surface", got)
	}
}

// --- Draw: zero-width bounds triggers auto-size + no panic ---------------

func TestIconButtonDrawZeroBoundsAutoSizes(t *testing.T) {
	ib := NewIconButton("v", nil)
	theme := DefaultLight()
	buf := makeSurface(IconButtonSize, IconButtonSize)
	ib.Draw(newP(buf, IconButtonSize), theme)
	// After the first Draw the widget has been auto-sized, so the
	// border corner is now painted.
	if got := pixelAt(buf, IconButtonSize, 0, 0); got != theme.Border {
		t.Fatalf("auto-size did not paint border: got %+v", got)
	}
}

// --- OnEvent -------------------------------------------------------------

func TestIconButtonClickFires(t *testing.T) {
	clicked := 0
	ib := NewIconButton("+", func() { clicked++ })
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	ib.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if clicked != 1 {
		t.Fatalf("click fired %d times, want 1", clicked)
	}
}

func TestIconButtonNilCallbackNoPanic(t *testing.T) {
	ib := NewIconButton("+", nil)
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	ib.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
}

func TestIconButtonIgnoresNonClickEvents(t *testing.T) {
	fired := 0
	ib := NewIconButton("+", func() { fired++ })
	ib.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	ib.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	ib.OnEvent(Event{Kind: EventChar, Code: "a"})
	ib.OnEvent(Event{Kind: EventCompositionStart})
	if fired != 0 {
		t.Fatalf("non-click fired %d times", fired)
	}
}
