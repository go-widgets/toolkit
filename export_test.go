// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/go-widgets/painter"
)

// insetFillWidget (distinct from the package's own fillWidget test helper,
// which fills its whole bounds) paints a solid, easy-to-assert-on colour
// into a margin-inset
// rectangle within its bounds, leaving a border strip untouched so tests can
// distinguish "background the widget never touched" from "pixel the widget
// drew".
type insetFillWidget struct {
	Base
	color RGBA
}

func (f *insetFillWidget) Draw(p painter.Painter, _ *Theme) {
	r := f.Bounds()
	p.FillRect(Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}, f.color)
}

var fillColor = RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}

func TestRenderImage(t *testing.T) {
	theme := DefaultLight()
	w := &insetFillWidget{color: fillColor}

	img, err := RenderImage(w, 20, 10, theme)
	if err != nil {
		t.Fatalf("RenderImage: unexpected error: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 20, 10) {
		t.Fatalf("bounds = %v, want (0,0)-(20,10)", got)
	}

	// Corner pixel: inside the canvas but outside the widget's inset fill —
	// must be the theme's background, proving RenderImage painted the
	// background before drawing.
	if got := img.RGBAAt(0, 0); got != color2rgba(theme.Background) {
		t.Fatalf("corner pixel = %+v, want background %+v", got, theme.Background)
	}

	// Centre pixel: inside the widget's fill — must be the widget-drawn
	// colour, proving Draw actually ran against the returned buffer.
	if got := img.RGBAAt(10, 5); got != color2rgba(fillColor) {
		t.Fatalf("centre pixel = %+v, want widget colour %+v", got, fillColor)
	}
}

func TestRenderImage_InvalidDimensions(t *testing.T) {
	theme := DefaultLight()
	w := &insetFillWidget{color: fillColor}

	cases := []struct {
		name          string
		width, height int
	}{
		{"zero width", 0, 10},
		{"negative width", -5, 10},
		{"zero height", 10, 0},
		{"negative height", 10, -5},
		{"both invalid", -1, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img, err := RenderImage(w, c.width, c.height, theme)
			if err == nil {
				t.Fatalf("RenderImage(%d,%d) = %v, nil, want an error", c.width, c.height, img)
			}
			if img != nil {
				t.Fatalf("RenderImage(%d,%d) returned non-nil image on error", c.width, c.height)
			}
		})
	}
}

func TestRenderPNG(t *testing.T) {
	theme := DefaultLight()
	w := &insetFillWidget{color: fillColor}

	data, err := RenderPNG(w, 20, 10, theme)
	if err != nil {
		t.Fatalf("RenderPNG: unexpected error: %v", err)
	}

	magic := []byte{0x89, 'P', 'N', 'G'}
	if len(data) < len(magic) || !bytes.Equal(data[:len(magic)], magic) {
		t.Fatalf("RenderPNG output missing PNG magic header, got % x", data[:min(8, len(data))])
	}

	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode roundtrip failed: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 20, 10) {
		t.Fatalf("decoded bounds = %v, want (0,0)-(20,10)", got)
	}
	r, g, b, a := decoded.At(10, 5).RGBA()
	want := fillColor
	if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || uint8(a>>8) != want.A {
		t.Fatalf("decoded centre pixel = (%d,%d,%d,%d), want %+v", r>>8, g>>8, b>>8, a>>8, want)
	}
}

func TestRenderPNG_InvalidDimensions(t *testing.T) {
	theme := DefaultLight()
	w := &insetFillWidget{color: fillColor}

	if data, err := RenderPNG(w, 0, 10, theme); err == nil {
		t.Fatalf("RenderPNG(0,10) = %v, nil, want an error", data)
	}
	if data, err := RenderPNG(w, 10, 0, theme); err == nil {
		t.Fatalf("RenderPNG(10,0) = %v, nil, want an error", data)
	}
}

// TestRenderPNG_RealWidget exercises the export path against a real,
// production widget (Button) rather than the synthetic insetFillWidget above, to
// prove RenderImage/RenderPNG work against the toolkit's normal Draw
// implementations (border, fill, centred text) and not just a trivial one.
func TestRenderPNG_RealWidget(t *testing.T) {
	theme := DefaultLight()
	btn := NewButton("OK", nil)

	data, err := RenderPNG(btn, 60, 24, theme)
	if err != nil {
		t.Fatalf("RenderPNG(Button): unexpected error: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode roundtrip failed: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 60, 24) {
		t.Fatalf("decoded bounds = %v, want (0,0)-(60,24)", got)
	}

	// The button's face (theme.Surface) should have been painted somewhere
	// inside its body — pick a point away from the rounded corners and the
	// centred label.
	r, g, b, a := decoded.At(3, 12).RGBA()
	want := theme.Surface
	if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || uint8(a>>8) != want.A {
		t.Fatalf("button face pixel = (%d,%d,%d,%d), want Surface %+v", r>>8, g>>8, b>>8, a>>8, want)
	}
}

// color2rgba converts the toolkit's RGBA (an alias of painter.RGBA) into a
// standard library color.RGBA for comparison against image.RGBAAt results.
func color2rgba(c RGBA) color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}
