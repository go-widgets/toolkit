// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- NewCard round-trip --------------------------------------------------

func TestNewCardStoresFields(t *testing.T) {
	c := NewCard("Title", "line1\nline2", "footer")
	if c.Title != "Title" || c.Body != "line1\nline2" || c.Footer != "footer" {
		t.Fatalf("NewCard round-trip broken: %+v", c)
	}
}

// --- Draw branches -------------------------------------------------------

// All zones populated: the header + footer strips paint SurfaceAlt, the
// body area shows the Surface fill, the border corner pixel is Border.
func TestCardDrawAllZonesPopulated(t *testing.T) {
	const w, h = 120, 80
	theme := DefaultLight()
	c := NewCard("Head", "one\ntwo", "Foot")
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	// Border corner pixel.
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("top-left corner = %+v, want Border", pixelAt(buf, w, 0, 0))
	}
	// Header strip pixel — inside the header strip, away from text col.
	if pixelAt(buf, w, w-4, 2) != theme.SurfaceAlt {
		t.Fatalf("header strip pixel = %+v, want SurfaceAlt", pixelAt(buf, w, w-4, 2))
	}
	// Footer strip pixel — inside the footer strip.
	if pixelAt(buf, w, w-4, h-2) != theme.SurfaceAlt {
		t.Fatalf("footer strip pixel = %+v, want SurfaceAlt", pixelAt(buf, w, w-4, h-2))
	}
	// Body area (between header and footer) should show the Surface fill.
	bodyY := CardHeaderH + 4
	if pixelAt(buf, w, w-4, bodyY) != theme.Surface {
		t.Fatalf("body area pixel = %+v, want Surface", pixelAt(buf, w, w-4, bodyY))
	}
	// The body ink lands somewhere inside the body area — at least one
	// pixel painted in OnSurface confirms the split-on-\n loop ran.
	painted := 0
	for y := CardHeaderH + 1; y < h-CardFooterH-1; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("body lines should paint OnSurface ink")
	}
}

// Title == "" branch: no header strip painted. The row that would have
// been the header body should still show the Surface fill.
func TestCardDrawEmptyTitleSkipsHeader(t *testing.T) {
	const w, h = 80, 60
	theme := DefaultLight()
	c := NewCard("", "body", "")
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 60})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// A pixel where the header strip *would* have lived: away from border.
	if pixelAt(buf, w, w/2, 2) != theme.Surface {
		t.Fatalf("no-header pixel = %+v, want Surface fill", pixelAt(buf, w, w/2, 2))
	}
}

// Footer == "" branch: no footer strip painted, bottom row shows Surface.
func TestCardDrawEmptyFooterSkipsFooter(t *testing.T) {
	const w, h = 80, 60
	theme := DefaultLight()
	c := NewCard("Hi", "body", "")
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 60})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// One row above the border: should be Surface, not SurfaceAlt.
	if pixelAt(buf, w, w/2, h-2) != theme.Surface {
		t.Fatalf("no-footer pixel = %+v, want Surface fill", pixelAt(buf, w, w/2, h-2))
	}
}

// Body == "" branch: the split-on-\n loop is skipped.
func TestCardDrawEmptyBodyPaintsNoLines(t *testing.T) {
	const w, h = 80, 60
	theme := DefaultLight()
	c := NewCard("T", "", "F")
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 60})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// Body area — between header divider and footer divider — no ink.
	inked := 0
	for y := CardHeaderH + 2; y < h-CardFooterH-2; y++ {
		for x := 4; x < w-4; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				inked++
			}
		}
	}
	if inked != 0 {
		t.Fatalf("empty Body should paint 0 ink pixels; got %d", inked)
	}
}

// All zones empty: a minimal frame (Surface fill + Border stroke) is
// painted; no header, no footer, no body ink.
func TestCardDrawAllEmptyRendersFrameOnly(t *testing.T) {
	const w, h = 40, 30
	theme := DefaultLight()
	c := NewCard("", "", "")
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// Corner is border.
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("corner = %+v, want Border", pixelAt(buf, w, 0, 0))
	}
	// Interior is Surface.
	if pixelAt(buf, w, w/2, h/2) != theme.Surface {
		t.Fatalf("centre = %+v, want Surface", pixelAt(buf, w, w/2, h/2))
	}
}

// Body with a single line (no '\n') still enters the loop once — this
// covers the "one element" branch of strings.Split.
func TestCardDrawSingleLineBody(t *testing.T) {
	const w, h = 80, 60
	theme := DefaultLight()
	c := NewCard("", "single", "")
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 60})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	painted := 0
	for y := CardPadY; y < CardPadY+GlyphHeight; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("single-line body should paint ink pixels")
	}
}
