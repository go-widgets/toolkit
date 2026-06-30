// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// makeSurface allocates a w*h RGBA byte slice and pre-fills it with
// a sentinel non-theme colour so tests can detect "this pixel was
// painted by the widget" vs "this pixel is the original sentinel".
func makeSurface(w, h int) []byte {
	buf := make([]byte, w*h*4)
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i+0], buf[i+1], buf[i+2], buf[i+3] = 0xC8, 0xC8, 0xC8, 0xFF
	}
	return buf
}

func pixelAt(buf []byte, w, x, y int) RGBA {
	o := (y*w + x) * 4
	return RGBA{buf[o], buf[o+1], buf[o+2], buf[o+3]}
}

// --- Rect.Contains -------------------------------------------------------

func TestRectContains(t *testing.T) {
	r := Rect{X: 5, Y: 10, W: 20, H: 8}
	cases := []struct {
		px, py int
		want   bool
	}{
		{5, 10, true},   // top-left corner included
		{24, 17, true},  // bottom-right interior
		{25, 10, false}, // right edge EXCLUSIVE
		{5, 18, false},  // bottom edge EXCLUSIVE
		{4, 10, false},  // before left
		{5, 9, false},   // before top
		{100, 100, false},
	}
	for _, c := range cases {
		if got := r.Contains(c.px, c.py); got != c.want {
			t.Errorf("Contains(%d,%d) = %v, want %v", c.px, c.py, got, c.want)
		}
	}
}

// --- Base default behaviour ----------------------------------------------

func TestBaseDefaults(t *testing.T) {
	var b Base
	b.SetBounds(Rect{X: 3, Y: 4, W: 5, H: 6})
	if got := b.Bounds(); got.X != 3 || got.W != 5 {
		t.Fatalf("SetBounds/Bounds round-trip broken: %+v", got)
	}
	if !b.HitTest(4, 5) {
		t.Fatal("HitTest should report hit inside Bounds")
	}
	if b.HitTest(0, 0) {
		t.Fatal("HitTest should miss outside Bounds")
	}
	// Default OnEvent + Draw are no-ops; just verify they don't panic.
	(&b).OnEvent(Event{Kind: EventClick})
	(&b).Draw(nil, 0, nil)
}

// --- Themes --------------------------------------------------------------

func TestDefaultLightDarkAreDistinct(t *testing.T) {
	l := DefaultLight()
	d := DefaultDark()
	if l.Background == d.Background {
		t.Fatal("DefaultLight + DefaultDark must not share Background")
	}
	if l.OnSurface == d.OnSurface {
		t.Fatal("text colour must differ between light + dark")
	}
}

func TestRGBHasOpaqueAlpha(t *testing.T) {
	c := RGB(0x10, 0x20, 0x30)
	if c.A != 0xFF {
		t.Fatalf("RGB built %d alpha, want 0xFF", c.A)
	}
}

// --- raster --------------------------------------------------------------

func TestFillRectPaintsInBoundsOnly(t *testing.T) {
	const w, h = 16, 16
	buf := makeSurface(w, h)
	fillRect(buf, w, 4, 4, 6, 6, RGB(0x10, 0x20, 0x30))
	if got := pixelAt(buf, w, 5, 5); got.R != 0x10 || got.G != 0x20 {
		t.Fatalf("interior pixel = %+v, want filled", got)
	}
	if got := pixelAt(buf, w, 0, 0); got.R != 0xC8 {
		t.Fatalf("out-of-bounds pixel was painted: %+v", got)
	}
}

func TestFillRectClipsOOB(t *testing.T) {
	const w, h = 8, 8
	buf := makeSurface(w, h)
	// Rectangle that overflows on every side; must not panic + must
	// only paint the in-bounds slice.
	fillRect(buf, w, -3, -3, 20, 20, RGB(0xAA, 0xBB, 0xCC))
	if got := pixelAt(buf, w, 0, 0); got.R != 0xAA {
		t.Fatalf("(0,0) should be painted, got %+v", got)
	}
}

// Surface truncated mid-row: the per-pixel `off+3 >= len(surface)`
// guard must skip the OOB write instead of panicking. Real callers
// always pass a correctly-sized buffer, but a defensive guard
// shouldn't be dead code.
func TestFillRectTruncatedSurface(t *testing.T) {
	const w = 16
	// Allocate room for only 1 row + 2 extra pixels; the fill targets
	// rows 0 + 1, so the row-1 pixels past offset len will trip the
	// per-pixel guard rather than panicking.
	buf := make([]byte, w*4+8)
	fillRect(buf, w, 0, 0, w, 2, RGB(1, 2, 3))
}

func TestFillRectZeroSizeNoOp(t *testing.T) {
	const w, h = 4, 4
	buf := makeSurface(w, h)
	fillRect(buf, w, 1, 1, 0, 5, RGB(1, 2, 3))
	fillRect(buf, w, 1, 1, 5, 0, RGB(1, 2, 3))
	if pixelAt(buf, w, 1, 1) != (RGBA{0xC8, 0xC8, 0xC8, 0xFF}) {
		t.Fatal("zero-width/height fill should not paint anything")
	}
}

func TestStrokeRectPaintsBorderOnly(t *testing.T) {
	const w, h = 12, 12
	buf := makeSurface(w, h)
	strokeRect(buf, w, 2, 2, 6, 6, RGB(0x11, 0x22, 0x33))
	// Border pixel painted.
	if got := pixelAt(buf, w, 2, 2); got.R != 0x11 {
		t.Fatalf("top-left border = %+v, want painted", got)
	}
	// Interior pixel NOT painted.
	if got := pixelAt(buf, w, 4, 4); got.R != 0xC8 {
		t.Fatalf("interior should still be sentinel: %+v", got)
	}
}

func TestStrokeRectZeroSizeNoOp(t *testing.T) {
	const w, h = 4, 4
	buf := makeSurface(w, h)
	strokeRect(buf, w, 1, 1, 0, 5, RGB(1, 2, 3))
	strokeRect(buf, w, 1, 1, 5, 0, RGB(1, 2, 3))
	if pixelAt(buf, w, 1, 1) != (RGBA{0xC8, 0xC8, 0xC8, 0xFF}) {
		t.Fatal("zero-dimension stroke should not paint anything")
	}
}

// --- Button --------------------------------------------------------------

func TestButtonClickFiresHandler(t *testing.T) {
	clicks := 0
	b := NewButton("OK", func() { clicks++ })
	b.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 24})
	b.OnEvent(Event{Kind: EventClick, X: 25, Y: 12})
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
}

func TestButtonClickWithNilHandlerNoPanic(t *testing.T) {
	b := NewButton("OK", nil)
	b.OnEvent(Event{Kind: EventClick}) // must not panic
}

func TestButtonIgnoresNonClick(t *testing.T) {
	clicks := 0
	b := NewButton("OK", func() { clicks++ })
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if clicks != 0 {
		t.Fatalf("KeyDown shouldn't fire OnClick (got clicks=%d)", clicks)
	}
}

func TestButtonDrawStates(t *testing.T) {
	const w, h = 32, 16
	theme := DefaultLight()
	b := NewButton("X", nil)
	b.SetBounds(Rect{X: 2, Y: 2, W: 20, H: 10})
	// Rest state: Surface fill.
	rest := makeSurface(w, h)
	b.Draw(rest, w, theme)
	if pixelAt(rest, w, 10, 6) != theme.Surface {
		t.Fatalf("rest face = %+v, want Surface", pixelAt(rest, w, 10, 6))
	}
	// Hover: SurfaceAlt fill.
	hov := makeSurface(w, h)
	b.SetHovered(true)
	b.Draw(hov, w, theme)
	if pixelAt(hov, w, 10, 6) != theme.SurfaceAlt {
		t.Fatalf("hover face = %+v, want SurfaceAlt", pixelAt(hov, w, 10, 6))
	}
	// Press: Accent fill.
	prs := makeSurface(w, h)
	b.SetPressed(true)
	b.Draw(prs, w, theme)
	if pixelAt(prs, w, 10, 6) != theme.Accent {
		t.Fatalf("press face = %+v, want Accent", pixelAt(prs, w, 10, 6))
	}
	// Border drawn at corner in every state.
	if pixelAt(prs, w, 2, 2) != theme.Border {
		t.Fatalf("top-left corner border = %+v, want Border", pixelAt(prs, w, 2, 2))
	}
}

// --- Label ---------------------------------------------------------------

func TestLabelHitTestIsNever(t *testing.T) {
	l := NewLabel("hi")
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	if l.HitTest(10, 10) {
		t.Fatal("Label.HitTest must always be false")
	}
}

func TestLabelDrawPaintsInkLine(t *testing.T) {
	const w, h = 32, 16
	theme := DefaultLight()
	l := NewLabel("hi")
	l.SetBounds(Rect{X: 2, Y: 4, W: 20, H: 8})
	buf := makeSurface(w, h)
	l.Draw(buf, w, theme)
	// Midpoint horizontal line painted in OnSurface.
	midY := 4 + 8/2
	if pixelAt(buf, w, 10, midY) != theme.OnSurface {
		t.Fatalf("ink line missing at midpoint: %+v", pixelAt(buf, w, 10, midY))
	}
	// Above/below the line still sentinel.
	if pixelAt(buf, w, 10, 4) != (RGBA{0xC8, 0xC8, 0xC8, 0xFF}) {
		t.Fatal("non-line row should not be inked")
	}
}
