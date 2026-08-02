// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// wdPx reads the RGBA pixel at (x, y) from a width-strided RGBA buffer.
func wdPx(buf []byte, width, x, y int) painter.RGBA {
	i := (y*width + x) * 4
	return painter.RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

// wdRender draws d onto a freshly zeroed w×h pixel buffer and returns it. An
// untouched pixel keeps the zero RGBA (A=0), so tests can tell the painted
// chrome from the transparent body hole apart.
func wdRender(d *WindowDecoration, w, h int) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	d.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	d.Draw(p, DefaultDark())
	return buf
}

// wdHas reports whether colour c appears anywhere in the (x,y,w,h) region.
func wdHas(buf []byte, width int, x, y, w, h int, c painter.RGBA) bool {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if wdPx(buf, width, xx, yy) == c {
				return true
			}
		}
	}
	return false
}

func TestNewWindowDecoration(t *testing.T) {
	d := NewWindowDecoration()
	if d == nil {
		t.Fatal("NewWindowDecoration returned nil")
	}
	if len(d.Buttons) != 0 {
		t.Errorf("fresh decoration has %d buttons, want 0", len(d.Buttons))
	}
	b := DecoButton{Rect: Rect{X: 1, Y: 2, W: 8, H: 8}, Face: painter.RGB(1, 2, 3)}
	if got := d.AddButton(b); got != d {
		t.Error("AddButton did not return the receiver for chaining")
	}
	if len(d.Buttons) != 1 || d.Buttons[0] != b {
		t.Errorf("AddButton did not append the button: %+v", d.Buttons)
	}
}

// TestWindowDecorationDrawFull renders a fully-populated Openbox-like frame with
// one rectangular (close, ×) button and one circular (traffic-light) button, a
// grip, a shadow and a border, and asserts each element painted its colour where
// expected — and left the body hole transparent.
func TestWindowDecorationDrawFull(t *testing.T) {
	red := painter.RGB(0x9b, 0x1c, 0x2e)  // titlebar band
	ink := painter.RGB(0xf5, 0xf6, 0xfa)  // title text
	hair := painter.RGB(0xbf, 0xbf, 0xbf) // bottom hairline
	border := painter.RGB(0xef, 0x44, 0x44)
	shadow := painter.RGB(0x99, 0x99, 0x99)
	grip := painter.RGB(0x5b, 0x60, 0x72)
	face := painter.RGB(0xe6, 0xe7, 0xee)  // close-box face
	glyph := painter.RGB(0x1a, 0x1a, 0x2e) // × ink
	dot := painter.RGB(0xff, 0x5f, 0x57)   // traffic-light fill
	outl := painter.RGB(0xe0, 0x44, 0x3e)  // traffic-light outline

	d := &WindowDecoration{
		Title:       "Hi",
		TitleInk:    ink,
		TitleColor:  red,
		Titlebar:    Rect{X: 0, Y: 0, W: 40, H: 10},
		Hairline:    hair,
		Border:      Rect{X: 0, Y: 0, W: 40, H: 30},
		BorderColor: border,
		Shadow:      shadow,
		Grip:        Rect{X: 30, Y: 20, W: 8, H: 8},
		ShowGrip:    true,
		GripColor:   grip,
	}
	d.AddButton(DecoButton{
		Rect: Rect{X: 24, Y: 1, W: 8, H: 8}, Shape: DecoButtonRect,
		Face: face, Glyph: DecoGlyphClose, GlyphInk: glyph,
	})
	d.AddButton(DecoButton{
		Rect: Rect{X: 2, Y: 1, W: 8, H: 8}, Shape: DecoButtonCircle,
		Face: dot, Outline: outl,
	})

	const w, h = 41, 31
	buf := wdRender(d, w, h)

	// Title-bar band interior (clear of caption + buttons + hairline).
	if got := wdPx(buf, w, 18, 3); got != red {
		t.Errorf("band interior (18,3) = %v, want band %v", got, red)
	}
	// Caption ink appears somewhere in the left of the band.
	if !wdHas(buf, w, 6, 1, 12, 8, ink) {
		t.Error("title caption ink not found in the band")
	}
	// Bottom hairline of the band.
	if got := wdPx(buf, w, 18, 9); got != hair {
		t.Errorf("hairline (18,9) = %v, want %v", got, hair)
	}
	// Close-box face (clear of the × diagonals).
	if got := wdPx(buf, w, 30, 2); got != face {
		t.Errorf("close-box face (30,2) = %v, want %v", got, face)
	}
	// The × glyph painted its ink inside the close box.
	if !wdHas(buf, w, 24, 1, 8, 8, glyph) {
		t.Error("close × glyph ink not found in the close box")
	}
	// Traffic-light fill present in the circle button (its round outline is
	// fully anti-aliased on a perfect circle; TestWindowDecorationCircleOutline
	// asserts the outline colour on a shape with straight edges).
	_ = outl
	if !wdHas(buf, w, 2, 1, 8, 8, dot) {
		t.Error("traffic-light fill not found")
	}
	// Border on the right edge, shadow one unit past it.
	if got := wdPx(buf, w, 39, 15); got != border {
		t.Errorf("border right edge (39,15) = %v, want %v", got, border)
	}
	if got := wdPx(buf, w, 40, 15); got != shadow {
		t.Errorf("shadow right band (40,15) = %v, want %v", got, shadow)
	}
	if got := wdPx(buf, w, 18, 30); got != shadow {
		t.Errorf("shadow bottom band (18,30) = %v, want %v", got, shadow)
	}
	// Grip diagonals painted their colour in the corner.
	if !wdHas(buf, w, 30, 20, 9, 9, grip) {
		t.Error("resize grip colour not found in the grip corner")
	}
	// The body hole (below the band, inside the border) stays transparent.
	if got := wdPx(buf, w, 18, 18); got.A != 0 {
		t.Errorf("body hole (18,18) = %v, want transparent (A=0)", got)
	}
}

// TestWindowDecorationCenteredTitle covers the macOS-style centred caption.
func TestWindowDecorationCenteredTitle(t *testing.T) {
	ink := painter.RGB(0x3c, 0x3c, 0x43)
	d := &WindowDecoration{
		Title: "AB", TitleInk: ink, TitleColor: painter.RGB(0xec, 0xec, 0xec),
		Titlebar: Rect{X: 0, Y: 0, W: 60, H: 12}, TitleCenter: true,
	}
	const w, h = 60, 14
	buf := wdRender(d, w, h)
	// Centred: ink must appear near the horizontal middle, not at the left pad.
	if wdHas(buf, w, 0, 0, 20, 12, ink) {
		t.Error("centred caption ink found at the left edge (should be centred)")
	}
	if !wdHas(buf, w, 20, 0, 20, 12, ink) {
		t.Error("centred caption ink not found near the middle")
	}
}

// TestWindowDecorationGlyphs covers the minimize + maximize glyph strokes.
func TestWindowDecorationGlyphs(t *testing.T) {
	ink := painter.RGB(0x10, 0x20, 0x30)
	face := painter.RGB(0xd0, 0xd0, 0xd0)
	for _, tc := range []struct {
		name  string
		glyph DecoGlyph
	}{
		{"minimize", DecoGlyphMinimize},
		{"maximize", DecoGlyphMaximize},
		{"none", DecoGlyphNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &WindowDecoration{}
			d.AddButton(DecoButton{
				Rect: Rect{X: 0, Y: 0, W: 14, H: 14}, Shape: DecoButtonRect,
				Face: face, Glyph: tc.glyph, GlyphInk: ink,
			})
			buf := wdRender(d, 14, 14)
			has := wdHas(buf, 14, 0, 0, 14, 14, ink)
			if tc.glyph == DecoGlyphNone && has {
				t.Error("DecoGlyphNone painted glyph ink")
			}
			if tc.glyph != DecoGlyphNone && !has {
				t.Errorf("%s glyph ink not found", tc.name)
			}
		})
	}
}

// TestWindowDecorationCircleWideRect covers the radius-clamp branch where the
// button rect is wider than it is tall (radius clamps to the shorter side).
func TestWindowDecorationCircleWideRect(t *testing.T) {
	dot := painter.RGB(0x28, 0xc8, 0x40)
	d := &WindowDecoration{}
	d.AddButton(DecoButton{
		Rect: Rect{X: 0, Y: 0, W: 12, H: 8}, Shape: DecoButtonCircle, Face: dot,
	})
	buf := wdRender(d, 12, 8)
	if got := wdPx(buf, 12, 6, 4); got != dot {
		t.Errorf("wide-circle centre (6,4) = %v, want fill %v", got, dot)
	}
}

// TestWindowDecorationCircleOutline covers the circle Outline branch on a
// stadium (H > W) whose straight vertical edges stroke opaquely.
func TestWindowDecorationCircleOutline(t *testing.T) {
	fill := painter.RGB(0xfe, 0xbc, 0x2e)
	outl := painter.RGB(0xe0, 0xa1, 0x2b)
	d := &WindowDecoration{}
	d.AddButton(DecoButton{
		Rect: Rect{X: 0, Y: 0, W: 8, H: 14}, Shape: DecoButtonCircle,
		Face: fill, Outline: outl,
	})
	buf := wdRender(d, 8, 14)
	// The left straight edge (mid-height) is an axis-aligned opaque stroke.
	if got := wdPx(buf, 8, 0, 7); got != outl {
		t.Errorf("stadium left edge (0,7) = %v, want outline %v", got, outl)
	}
	// Interior stays the fill.
	if got := wdPx(buf, 8, 4, 7); got != fill {
		t.Errorf("stadium interior (4,7) = %v, want fill %v", got, fill)
	}
}

// TestWindowDecorationOmissions covers every "absent" branch: no title-bar, no
// hairline, no border, no shadow, no grip, an empty caption and a zero-sized
// button — none of which may paint.
func TestWindowDecorationOmissions(t *testing.T) {
	const w, h = 20, 20

	// Zero-size title-bar + zero-size border + grip off + zero-size button: a
	// completely blank buffer.
	blank := &WindowDecoration{
		Title: "x", TitleInk: painter.RGB(1, 1, 1), TitleColor: painter.RGB(2, 2, 2),
		Titlebar:    Rect{X: 0, Y: 0, W: 0, H: 10}, // W<=0 short-circuits the band
		Hairline:    painter.RGB(3, 3, 3),
		Border:      Rect{X: 0, Y: 0, W: 0, H: 0}, // no border / no shadow
		BorderColor: painter.RGB(4, 4, 4),
		Shadow:      painter.RGB(5, 5, 5),
		ShowGrip:    false,
		GripColor:   painter.RGB(6, 6, 6),
	}
	blank.AddButton(DecoButton{Rect: Rect{X: 0, Y: 0, W: 0, H: 8}, Face: painter.RGB(7, 7, 7)})
	buf := wdRender(blank, w, h)
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("fully-omitted decoration painted byte %d = %d, want 0", i, v)
		}
	}

	// A band with an empty caption + zero-A hairline: band fills, but no caption
	// ink and no hairline line.
	band := painter.RGB(0x30, 0x30, 0x30)
	d2 := &WindowDecoration{
		Title: "", TitleColor: band, Titlebar: Rect{X: 0, Y: 0, W: 20, H: 8},
		Hairline: painter.RGBA{}, // A==0 → no hairline
	}
	buf2 := wdRender(d2, w, h)
	if got := wdPx(buf2, w, 10, 4); got != band {
		t.Errorf("band (10,4) = %v, want %v", got, band)
	}
	// Bottom row of the band must still be the band fill (no hairline drawn).
	if got := wdPx(buf2, w, 10, 7); got != band {
		t.Errorf("band bottom row (10,7) = %v, want band fill %v (no hairline)", got, band)
	}

	// Border colour with A==0 must not stroke even with a sized Border rect.
	d3 := &WindowDecoration{
		Border: Rect{X: 0, Y: 0, W: 20, H: 20}, BorderColor: painter.RGBA{},
		Shadow: painter.RGBA{}, // A==0 → no shadow either
	}
	buf3 := wdRender(d3, w, h)
	for i, v := range buf3 {
		if v != 0 {
			t.Fatalf("A=0 border/shadow painted byte %d = %d, want 0", i, v)
		}
	}
}
