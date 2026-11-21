// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A card Frame is a rounded surface: its corner shows the page behind it, its
// middle the card colour, and a light theme's shadow darkens the page below it.
func TestFrameCardIsARoundedSurfaceWithAShadow(t *testing.T) {
	const w, h = 120, 90
	card := func(theme *Theme, title string) []byte {
		f := NewFrame(NewLabel("body"))
		f.Card, f.Title = true, title
		f.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 60})
		surf := make([]byte, w*h*4)
		fillRect(newP(surf, w), 0, 0, w, h, theme.Background)
		f.Draw(newP(surf, w), theme)
		return surf
	}
	light := SlateLight(RGB(0, 0x79, 0xA8))
	s := card(light, "")
	if got := pixelAt(s, w, 10, 10); got != light.Background {
		t.Errorf("the corner pixel %+v must still be the page %+v (rounded)", got, light.Background)
	}
	if got := pixelAt(s, w, 60, 40); got != light.Surface {
		t.Errorf("the middle %+v must be the card %+v", got, light.Surface)
	}
	if got := pixelAt(s, w, 60, 73); got == light.Background {
		t.Error("the shadow below a light card left the page untouched")
	}
	d := SlateDark(RGB(0, 0x79, 0xA8))
	if got := pixelAt(card(d, ""), w, 60, 73); got != d.Background {
		t.Errorf("a dark card has no shadow: %+v", got)
	}
	// A titled card sets its heading in the ink over a hairline, not a band.
	titled := card(light, "Detail")
	band := pixelAt(titled, w, 60, 18)
	if band == light.SurfaceAlt {
		t.Error("a card heading must not be a filled band")
	}
	hairline := false
	for y := 20; y < 50; y++ {
		if pixelAt(titled, w, 60, y) == light.Border {
			hairline = true
		}
	}
	if !hairline {
		t.Error("no hairline under the heading")
	}
	// The plain frame is unchanged: square, border only.
	plain := NewFrame(NewLabel("body"))
	plain.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 60})
	ps := make([]byte, w*h*4)
	plain.Draw(newP(ps, w), light)
	if pixelAt(ps, w, 10, 10) != light.Border {
		t.Error("a plain frame's corner is its square border")
	}
}

func TestFrameCardPaddingAndRadius(t *testing.T) {
	f := NewFrame(NewLabel("x"))
	f.Card = true
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	if b := f.child.Bounds(); b.X != 1+frameCardPadding || b.Y != 1+frameCardPadding {
		t.Errorf("card padding: child at %v", b)
	}
	if f.cardRadius() != frameCardRadius {
		t.Errorf("default radius %d", f.cardRadius())
	}
	f.CornerRadius = 5
	if f.cardRadius() != 5 {
		t.Errorf("radius %d", f.cardRadius())
	}
	f.Padding = 3 // an explicit padding wins over the card default
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	if f.child.Bounds().X != 1+3 {
		t.Errorf("explicit padding: child at %v", f.child.Bounds())
	}
}

func TestChipStyles(t *testing.T) {
	const w, h = 90, 20
	draw := func(theme *Theme, st ChipStyle) []byte {
		c := NewChip("ok")
		c.Style = st
		c.SetBounds(Rect{X: 2, Y: 2, W: 60, H: 16})
		surf := make([]byte, w*h*4)
		fillRect(newP(surf, w), 0, 0, w, h, theme.Surface)
		c.Draw(newP(surf, w), theme)
		return surf
	}
	light := SlateLight(RGB(0, 0x79, 0xA8))
	// A pill: the corner is the ground, the middle the tint.
	for _, tc := range []struct {
		st   ChipStyle
		want RGBA
	}{{ChipPlain, light.Extra["chip_plain_bg"]}, {ChipBrand, light.Extra["chip_brand_bg"]}} {
		s := draw(light, tc.st)
		if pixelAt(s, w, 2, 2) != light.Surface {
			t.Errorf("style %d: the corner is not rounded", tc.st)
		}
		if got := pixelAt(s, w, 30, 4); got != tc.want {
			t.Errorf("style %d: fill %+v, want %+v", tc.st, got, tc.want)
		}
	}
	// The default chip is the original rectangle: its corner is its border.
	if pixelAt(draw(light, ChipDefault), w, 2, 2) != light.Border {
		t.Error("the default chip changed")
	}
	// A theme with no chip colours falls back to SurfaceAlt / OnSurface / Accent.
	bare := DefaultLight()
	if themeChipBg(bare, ChipPlain) != bare.SurfaceAlt || themeChipInk(bare, ChipPlain) != bare.OnSurface {
		t.Error("plain chip fallback")
	}
	if themeChipBg(bare, ChipBrand) != blendRGBA(bare.Accent, bare.Surface, 0.10) || themeChipInk(bare, ChipBrand) != bare.Accent {
		t.Error("brand chip fallback")
	}
	// An empty pill is not drawn at all; an empty default chip still is.
	for _, st := range []ChipStyle{ChipPlain, ChipBrand} {
		e := NewChip("")
		e.Style = st
		e.SetBounds(Rect{X: 2, Y: 2, W: 60, H: 16})
		surf := make([]byte, w*h*4)
		fillRect(newP(surf, w), 0, 0, w, h, light.Surface)
		e.Draw(newP(surf, w), light)
		if pixelAt(surf, w, 30, 4) != light.Surface {
			t.Errorf("style %d: an empty pill was drawn", st)
		}
	}
	// A closable pill draws its x in the chip's ink.
	cc := NewClosableChip("ok", nil)
	cc.Style = ChipBrand
	cc.SetBounds(Rect{X: 2, Y: 2, W: 70, H: 16})
	surf := make([]byte, w*h*4)
	cc.Draw(newP(surf, w), light)
}

func TestGraphViewCornerRadiusRoundsNodesAndClusters(t *testing.T) {
	const w, h = 400, 120
	draw := func(radius int) ([]byte, *GraphView) {
		g := NewGraphView(manyClusters(1, 2))
		g.CornerRadius = radius
		g.SetBounds(Rect{W: w, H: h})
		surf := makeSurface(w, h)
		g.Draw(newP(surf, w), DefaultLight())
		g.Model.Selected.Set("node:n0_0")
		g.Draw(newP(surf, w), DefaultLight())
		return surf, g
	}
	sq, g := draw(0)
	rd, _ := draw(6)
	n, _ := g.NodeBox("n0_0")
	c := g.clusterBox["c0"]
	if pixelAt(sq, w, n.X, n.Y) == pixelAt(rd, w, n.X, n.Y) {
		t.Error("a rounded node's corner pixel equals the square one's")
	}
	if pixelAt(sq, w, c.X, c.Y) == pixelAt(rd, w, c.X, c.Y) {
		t.Error("a rounded cluster's corner pixel equals the square one's")
	}
	if g.clusterRadius() != 0 {
		t.Error("clusterRadius of a square graph")
	}
	g.CornerRadius = 8
	if g.clusterRadius() != 12 {
		t.Errorf("clusterRadius %d", g.clusterRadius())
	}
}
