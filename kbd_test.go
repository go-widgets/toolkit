// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestKbdConstructorStoresKeys checks that NewKbd carries the caller's
// key text through — future refactors that change the field name break
// this immediately.
func TestKbdConstructorStoresKeys(t *testing.T) {
	k := NewKbd("Ctrl-A")
	if k.Keys != "Ctrl-A" {
		t.Fatalf("Keys = %q, want %q", k.Keys, "Ctrl-A")
	}
}

// TestKbdDrawSurfaceAndBorder verifies Draw paints a Surface body with
// a Border stroke — the two visual elements that make Kbd read as a
// bordered chip rather than a bare label.
func TestKbdDrawSurfaceAndBorder(t *testing.T) {
	theme := DefaultLight()
	k := NewKbd("K")
	k.SetBounds(Rect{X: 2, Y: 2, W: 20, H: 12})
	surf := makeSurface(40, 20)
	k.Draw(newP(surf, 40), theme)
	// Corner pixel = Border stroke.
	if got := pixelAt(surf, 40, 2, 2); got != theme.Border {
		t.Fatalf("kbd corner = %+v, want Border", got)
	}
	// Interior body pixel above the text row = Surface.
	// Text centred at ty = 2 + (12-7)/2 = 4 → row 3 is above the glyph.
	if got := pixelAt(surf, 40, 3, 3); got != theme.Surface {
		t.Fatalf("kbd interior = %+v, want Surface", got)
	}
}

// TestKbdDrawTextInOnSurface samples a lit pixel from the "T" glyph to
// prove the text is drawn in Theme.OnSurface. Column 0 of 'T' has bit
// pattern 0x01 → only row 0 lit; with Bounds {W:20,H:12}, "T" as single
// char gives tx = 2 + (20-6)/2 = 9 and ty = 2 + (12-7)/2 = 4, so
// the lit pixel lands at (9, 4).
func TestKbdDrawTextInOnSurface(t *testing.T) {
	theme := DefaultLight()
	k := NewKbd("T")
	k.SetBounds(Rect{X: 2, Y: 2, W: 20, H: 12})
	surf := makeSurface(40, 20)
	k.Draw(newP(surf, 40), theme)
	if got := pixelAt(surf, 40, 9, 4); got != theme.OnSurface {
		t.Fatalf("kbd 'T' pixel = %+v, want OnSurface", got)
	}
}

// TestKbdEmptyKeys covers the empty-Keys code path: TextWidth("") is 0
// so the centre calculation still runs and Draw must not panic on an
// empty glyph loop.
func TestKbdEmptyKeys(t *testing.T) {
	theme := DefaultLight()
	k := NewKbd("")
	k.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 12})
	k.Draw(newP(makeSurface(40, 20), 40), theme)
}

// TestKbdZeroBounds covers the fillRect/strokeRect guards: with W=0
// they early-return and Draw is effectively a no-op.
func TestKbdZeroBounds(t *testing.T) {
	theme := DefaultLight()
	k := NewKbd("A")
	k.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	surf := makeSurface(20, 12)
	k.Draw(newP(surf, 20), theme)
	// Nothing painted → all pixels remain sentinel.
	if pixelAt(surf, 20, 0, 0) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("zero-Bounds Draw should not paint anything")
	}
}
