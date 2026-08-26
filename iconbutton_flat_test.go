// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// ibRender draws b onto a fresh w×h buffer and returns it.
func ibRender(b *IconButton, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	b.Draw(painter.NewPixelPainter(buf, w, h), theme)
	return buf
}

// ibPainted reports whether any pixel was touched.
func ibPainted(buf []byte) bool {
	for _, v := range buf {
		if v != 0 {
			return true
		}
	}
	return false
}

// A Flat button is invisible at rest — no face, no border — and paints a wash
// only under the pointer. A framed square in the corner of a title bar reads as
// a control belonging to the content rather than to the window.
func TestIconButtonFlatIsInvisibleUntilHovered(t *testing.T) {
	th := DefaultLight()
	b := NewIconButton("", nil)
	b.Flat = true
	b.SetBounds(Rect{W: 28, H: 28})

	if ibPainted(ibRender(b, 28, 28, th)) {
		t.Error("a flat button at rest must paint nothing")
	}

	b.OnEvent(Event{Kind: EventMouseMove, X: 14, Y: 14})
	if !ibPainted(ibRender(b, 28, 28, th)) {
		t.Error("a hovered flat button must paint its wash")
	}

	// The wash is ROUNDED: the very corner pixel stays untouched while the
	// middle of the same edge is painted.
	buf := ibRender(b, 28, 28, th)
	corner := buf[0:4]
	edge := buf[(0*28+14)*4 : (0*28+14)*4+4]
	if corner[3] != 0 {
		t.Errorf("the wash must be rounded: corner pixel painted %v", corner)
	}
	if edge[3] == 0 {
		t.Error("the wash must reach the middle of the top edge")
	}
}

// A framed button is unchanged: face and border at rest, exactly as every
// existing caller draws it.
func TestIconButtonFramedIsUnchanged(t *testing.T) {
	b := NewIconButton("x", nil)
	b.SetBounds(Rect{W: 28, H: 28})
	buf := ibRender(b, 28, 28, DefaultLight())
	if !ibPainted(buf) {
		t.Fatal("a framed button paints its face at rest")
	}
	if buf[3] == 0 {
		t.Error("a framed button fills its corner: the face is square")
	}
}

// Glyph replaces the text path, so a real icon can be drawn instead of a letter
// standing in for one.
func TestIconButtonGlyphReplacesTheText(t *testing.T) {
	var got Rect
	b := NewIconButton("x", nil)
	b.Glyph = func(_ painter.Painter, r Rect, _ RGBA) { got = r }
	b.SetBounds(Rect{X: 5, Y: 7, W: 28, H: 28})
	ibRender(b, 40, 40, DefaultLight())

	if got.W <= 0 {
		t.Fatal("Glyph was never called")
	}
	if got.W != got.H {
		t.Errorf("the glyph square is not square: %+v", got)
	}
	if got.X <= 5 || got.Y <= 7 || got.X+got.W >= 33 || got.Y+got.H >= 35 {
		t.Errorf("the glyph must be inset inside the button: %+v", got)
	}
}

// The glyph square fits the SHORTER side and never collapses below a pixel.
func TestIconButtonGlyphRect(t *testing.T) {
	wide := iconButtonGlyphRect(Rect{W: 60, H: 28})
	if wide.W != wide.H || wide.H >= 28 {
		t.Errorf("a wide button's glyph must fit its height: %+v", wide)
	}
	tiny := iconButtonGlyphRect(Rect{W: 2, H: 2})
	if tiny.W < 1 || tiny.H < 1 {
		t.Errorf("a tiny button still gets a 1px glyph: %+v", tiny)
	}
}

// The hover veil must read on ANY ground the host paints behind it. It was a
// theme FACE first, and a flat button in a SurfaceAlt title bar then hovered
// invisibly — the veil and the ground were the same colour. A translucent veil
// of the ink darkens a light ground and lightens a dark one.
func TestIconButtonFlatHoverShowsOnEveryGround(t *testing.T) {
	th := DefaultDark()
	for _, ground := range []struct {
		name string
		c    RGBA
	}{
		{"Surface", th.Surface},
		{"SurfaceAlt", th.SurfaceAlt},
		{"Background", th.Background},
	} {
		const W, H = 28, 28
		buf := make([]byte, 4*W*H)
		p := painter.NewPixelPainter(buf, W, H)
		fillRect(p, 0, 0, W, H, ground.c)
		before := make([]byte, len(buf))
		copy(before, buf)

		b := NewIconButton("", nil)
		b.Flat = true
		b.SetBounds(Rect{W: W, H: H})
		b.OnEvent(Event{Kind: EventMouseMove, X: W / 2, Y: H / 2})
		b.Draw(p, th)

		i := (H/2*W + W/2) * 4
		if buf[i] == before[i] && buf[i+1] == before[i+1] && buf[i+2] == before[i+2] {
			t.Errorf("on %s the hover veil is invisible: pixel unchanged %v", ground.name, buf[i:i+4])
		}
	}
}

// Pressed paints a heavier veil than hovered, so the button answers a press
// visibly rather than only on release.
func TestIconButtonFlatPressIsHeavierThanHover(t *testing.T) {
	th := DefaultDark()
	veil := func(press bool) RGBA {
		const W, H = 28, 28
		buf := make([]byte, 4*W*H)
		p := painter.NewPixelPainter(buf, W, H)
		b := NewIconButton("", nil)
		b.Flat = true
		b.SetBounds(Rect{W: W, H: H})
		b.OnEvent(Event{Kind: EventMouseMove, X: W / 2, Y: H / 2})
		if press {
			b.OnEvent(Event{Kind: EventClick, X: W / 2, Y: H / 2})
		}
		b.Draw(p, th)
		i := (H/2*W + W/2) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	if h, pr := veil(false), veil(true); h == pr {
		t.Errorf("press and hover paint the same veil %v; a press must read heavier", h)
	}
}
