// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/painter"
)

// ColorPicker is a rich HSV colour picker: a saturation/value square for the
// current hue, a vertical hue strip, a horizontal alpha slider (checkerboard
// under a live transparent-to-opaque gradient of the current colour), a
// solid preview swatch, and an eyedropper affordance.
//
// Unlike ColorChooser (3 independent R/G/B sliders), ColorPicker keeps its
// state in HSV -- the natural coordinate system for a 2D saturation/value
// surface -- and derives the RGBA on demand via Color().
//
// The eyedropper button only *signals intent*: OnEyedrop fires on click, but
// sampling an actual screen pixel is inherently host-specific (it needs a
// screenshot/compositor hook the toolkit doesn't have), so that part is the
// host's job. A typical host response is to enter a "pick" mode, read the
// pixel under the next click anywhere on screen, and feed it back in via
// SetColor.
type ColorPicker struct {
	Base

	// H is the hue in [0, 360). S and V (saturation, value) are both in
	// [0, 1].
	H, S, V float64

	// Alpha is the opacity channel, independent of the HSV triple.
	Alpha uint8

	// OnChange fires with the new RGBA whenever the SV square, hue strip,
	// or alpha slider changes the colour.
	OnChange func(c RGBA)

	// OnEyedrop fires when the eyedropper affordance is clicked. Actual
	// pixel sampling is the host's responsibility -- see the type doc.
	OnEyedrop func()

	// active tracks which region (if any) is being dragged: set on
	// EventClick, consulted on EventMouseDrag, cleared on EventMouseUp.
	active int
}

// active region identifiers for ColorPicker.active.
const (
	colorPickerActiveNone = iota
	colorPickerActiveSV
	colorPickerActiveHue
	colorPickerActiveAlpha
)

// Sizing. The SV square and hue strip share a height; the alpha slider spans
// their combined width beneath them; the swatch + eyedropper button sit in a
// final row.
const (
	ColorPickerSquareSize  = 120
	ColorPickerHueStripW   = 18
	ColorPickerGap         = 8
	ColorPickerAlphaH      = 16
	ColorPickerSwatchSize  = 28
	ColorPickerEyedropSize = 20

	// ColorPickerWidth and ColorPickerHeight are the picker's natural
	// (unclamped) LOGICAL footprint. Use [ColorPickerNaturalSize] for the
	// footprint to lay out with: at a metric scale above 1 it is larger, like
	// every other metric.
	ColorPickerWidth  = ColorPickerSquareSize + ColorPickerGap + ColorPickerHueStripW
	ColorPickerHeight = ColorPickerSquareSize + ColorPickerGap + ColorPickerAlphaH + ColorPickerGap + ColorPickerSwatchSize
)

// ColorPickerNaturalSize is the picker's footprint in device pixels at the
// current [MetricScale].
func ColorPickerNaturalSize() (w, h int) {
	return scaled(ColorPickerWidth), scaled(ColorPickerHeight)
}

// NewColorPicker builds a picker seeded from initial, converting its RGB to
// HSV and carrying its alpha through unchanged.
func NewColorPicker(initial RGBA) *ColorPicker {
	h, s, v := rgbToHSV(initial.R, initial.G, initial.B)
	return &ColorPicker{H: h, S: s, V: v, Alpha: initial.A}
}

// Color returns the current HSV + Alpha converted back to RGBA.
func (c *ColorPicker) Color() RGBA {
	r, g, b := hsvToRGB(c.H, c.S, c.V)
	return RGBA{R: r, G: g, B: b, A: c.Alpha}
}

// SetColor reseeds H/S/V/Alpha from an RGBA -- e.g. the host feeding back an
// eyedropper sample or a sibling hex Entry's parsed value.
func (c *ColorPicker) SetColor(rgba RGBA) {
	c.H, c.S, c.V = rgbToHSV(rgba.R, rgba.G, rgba.B)
	c.Alpha = rgba.A
}

// --- HSV <-> RGB -----------------------------------------------------------

// rgbToHSV converts 8-bit RGB into hue in [0, 360), saturation/value in
// [0, 1]. Grey (R == G == B, including black and white) is achromatic and
// reports H == 0, S == 0 by convention.
func rgbToHSV(r, g, b uint8) (h, s, v float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min

	v = max
	if max > 0 {
		s = delta / max
	}
	if delta == 0 {
		return 0, s, v
	}

	switch max {
	case rf:
		h = 60 * math.Mod((gf-bf)/delta, 6)
	case gf:
		h = 60 * ((bf-rf)/delta + 2)
	default: // bf
		h = 60 * ((rf-gf)/delta + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// hsvToRGB converts hue (any real value, wrapped mod 360) + saturation/value
// in [0, 1] (clamped) into 8-bit RGB.
func hsvToRGB(h, s, v float64) (r, g, b uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = clamp01(s)
	v = clamp01(v)

	ch := v * s
	x := ch * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - ch

	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = ch, x, 0
	case h < 120:
		rf, gf, bf = x, ch, 0
	case h < 180:
		rf, gf, bf = 0, ch, x
	case h < 240:
		rf, gf, bf = 0, x, ch
	case h < 300:
		rf, gf, bf = x, 0, ch
	default:
		rf, gf, bf = ch, 0, x
	}
	return to8(rf + m), to8(gf + m), to8(bf + m)
}

// clamp01 confines v to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// to8 converts a [0, 1] float channel into a rounded, clamped 8-bit value.
func to8(v float64) uint8 {
	v = clamp01(v)
	return uint8(math.Round(v * 255))
}

// --- layout (widget-local, 0-based -- see OnEvent's coordinate contract) --

// svRectLocal is the saturation/value square, top-left of the picker.
func (c *ColorPicker) svRectLocal() Rect {
	return Rect{X: 0, Y: 0, W: scaled(ColorPickerSquareSize), H: scaled(ColorPickerSquareSize)}
}

// hueRectLocal is the vertical hue strip, to the right of the SV square.
func (c *ColorPicker) hueRectLocal() Rect {
	return Rect{X: scaled(ColorPickerSquareSize) + scaled(ColorPickerGap), Y: 0, W: scaled(ColorPickerHueStripW), H: scaled(ColorPickerSquareSize)}
}

// alphaRectLocal is the horizontal alpha slider beneath the square + strip. Its
// width is the combined SCALED span of the square, gap and hue strip so its right
// edge lines up with the hue strip at every scale — the raw ColorPickerWidth
// constant used here before stayed 146 device pixels while everything above it
// doubled, leaving the slider short on HiDPI / touch. At compact/1x the scaled
// sum is exactly ColorPickerWidth, so the layout is byte-identical.
func (c *ColorPicker) alphaRectLocal() Rect {
	return Rect{
		X: 0,
		Y: scaled(ColorPickerSquareSize) + scaled(ColorPickerGap),
		W: scaled(ColorPickerSquareSize) + scaled(ColorPickerGap) + scaled(ColorPickerHueStripW),
		H: scaled(ColorPickerAlphaH),
	}
}

// EyedropHitRect is the finger target for the eyedropper button, in the same
// widget-local frame OnEvent hit-tests: the drawn button clamped up to the touch
// minimum on each axis and centred over it, so the 20-logical-pixel chip reaches
// the 44px floor under [DensityTouch]. At [DensityCompact] it equals the drawn
// button byte-for-byte.
func (c *ColorPicker) EyedropHitRect() Rect { return hitRectFor(c.eyedropRectLocal()) }

// swatchRectLocal is the solid preview swatch, bottom-left.
func (c *ColorPicker) swatchRectLocal() Rect {
	y := scaled(ColorPickerSquareSize) + scaled(ColorPickerGap) + scaled(ColorPickerAlphaH) + scaled(ColorPickerGap)
	return Rect{X: 0, Y: y, W: scaled(ColorPickerSwatchSize), H: scaled(ColorPickerSwatchSize)}
}

// eyedropRectLocal is the eyedropper button, beside the swatch.
func (c *ColorPicker) eyedropRectLocal() Rect {
	sw := c.swatchRectLocal()
	return Rect{
		X: sw.X + sw.W + scaled(ColorPickerGap),
		Y: sw.Y + (scaled(ColorPickerSwatchSize)-scaled(ColorPickerEyedropSize))/2,
		W: scaled(ColorPickerEyedropSize),
		H: scaled(ColorPickerEyedropSize),
	}
}

// --- Draw --------------------------------------------------------------

// Draw paints the SV square, hue strip, alpha slider, swatch + eyedropper
// button onto the widget's Bounds.
func (c *ColorPicker) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)

	c.drawSVSquare(p, r)
	c.drawHueStrip(p, r)
	c.drawAlphaSlider(p, r, theme)
	c.drawSwatch(p, r, theme)
	c.drawEyedrop(p, r, theme)
}

// drawSVSquare fills the square per-pixel: x-axis = saturation, y-axis =
// value (top = full value), for the current hue, then paints a ring marker
// at the current (S, V).
func (c *ColorPicker) drawSVSquare(p painter.Painter, r Rect) {
	sv := c.svRectLocal()
	wSpan := max(sv.W-1, 1)
	hSpan := max(sv.H-1, 1)
	for y := 0; y < sv.H; y++ {
		vv := 1 - float64(y)/float64(hSpan)
		for x := 0; x < sv.W; x++ {
			ss := float64(x) / float64(wSpan)
			rr, gg, bb := hsvToRGB(c.H, ss, vv)
			putPixel(p, r.X+sv.X+x, r.Y+sv.Y+y, RGBA{R: rr, G: gg, B: bb, A: 0xFF})
		}
	}
	mx := r.X + sv.X + int(clamp01(c.S)*float64(wSpan))
	my := r.Y + sv.Y + int((1-clamp01(c.V))*float64(hSpan))
	drawPickerMarker(p, mx, my)
}

// drawHueStrip fills the strip per-pixel with a full-height rainbow (top =
// hue 0, bottom = hue 360) and paints a bar marker at the current hue.
func (c *ColorPicker) drawHueStrip(p painter.Painter, r Rect) {
	hr := c.hueRectLocal()
	hSpan := max(hr.H-1, 1)
	for y := 0; y < hr.H; y++ {
		h := 360 * float64(y) / float64(hSpan)
		rr, gg, bb := hsvToRGB(h, 1, 1)
		col := RGBA{R: rr, G: gg, B: bb, A: 0xFF}
		for x := 0; x < hr.W; x++ {
			putPixel(p, r.X+hr.X+x, r.Y+hr.Y+y, col)
		}
	}
	my := r.Y + hr.Y + int(c.H/360*float64(hSpan))
	strokeRect(p, r.X+hr.X-1, my-1, hr.W+2, 3, RGB(0, 0, 0))
	strokeRect(p, r.X+hr.X, my, hr.W, 1, RGB(255, 255, 255))
}

// drawAlphaSlider paints a checkerboard, then overlays a left(transparent)-
// to-right(opaque) gradient of the current RGB via the painter's src-over
// PutPixel compositing, so the checkerboard shows through low-alpha pixels
// exactly as it would over any other content.
func (c *ColorPicker) drawAlphaSlider(p painter.Painter, r Rect, theme *Theme) {
	ar := c.alphaRectLocal()
	cell := scaled(6)
	light := RGB(0xFF, 0xFF, 0xFF)
	dark := RGB(0xC0, 0xC0, 0xC0)
	for y := 0; y < ar.H; y++ {
		for x := 0; x < ar.W; x++ {
			bg := dark
			if ((x/cell)+(y/cell))%2 == 0 {
				bg = light
			}
			putPixel(p, r.X+ar.X+x, r.Y+ar.Y+y, bg)
		}
	}
	rr, gg, bb := hsvToRGB(c.H, c.S, c.V)
	wSpan := max(ar.W-1, 1)
	for x := 0; x < ar.W; x++ {
		a := uint8(255 * x / wSpan)
		col := RGBA{R: rr, G: gg, B: bb, A: a}
		for y := 0; y < ar.H; y++ {
			putPixel(p, r.X+ar.X+x, r.Y+ar.Y+y, col)
		}
	}
	strokeRect(p, r.X+ar.X, r.Y+ar.Y, ar.W, ar.H, theme.Border)
	mx := r.X + ar.X + int(float64(c.Alpha)/255*float64(wSpan))
	strokeRect(p, mx-1, r.Y+ar.Y-1, 3, ar.H+2, theme.OnSurface)
}

// drawSwatch paints a solid preview of Color() with a border.
func (c *ColorPicker) drawSwatch(p painter.Painter, r Rect, theme *Theme) {
	sw := c.swatchRectLocal()
	fillRect(p, r.X+sw.X, r.Y+sw.Y, sw.W, sw.H, c.Color())
	strokeRect(p, r.X+sw.X, r.Y+sw.Y, sw.W, sw.H, theme.Border)
}

// drawEyedrop paints the eyedropper button: a rounded chip with a diagonal
// "pipette" glyph.
func (c *ColorPicker) drawEyedrop(p painter.Painter, r Rect, theme *Theme) {
	ed := c.eyedropRectLocal()
	rad := scaled(4)
	fillRoundRect(p, r.X+ed.X, r.Y+ed.Y, ed.W, ed.H, rad, theme.SurfaceAlt)
	strokeRoundRect(p, r.X+ed.X, r.Y+ed.Y, ed.W, ed.H, rad, theme.Border)
	drawLine(p, r.X+ed.X+rad, r.Y+ed.Y+ed.H-rad, r.X+ed.X+ed.W-rad, r.Y+ed.Y+rad, theme.OnSurface)
}

// drawPickerMarker paints a small white ring with a black halo centred at
// (cx, cy) -- readable against any hue/value the SV square or hue strip can
// show.
func drawPickerMarker(p painter.Painter, cx, cy int) {
	rad := scaled(4)
	strokeRoundRect(p, cx-rad-1, cy-rad-1, rad*2+2, rad*2+2, rad+1, RGB(0, 0, 0))
	strokeRoundRect(p, cx-rad, cy-rad, rad*2, rad*2, rad, RGB(255, 255, 255))
}

// --- OnEvent -------------------------------------------------------------

// OnEvent handles clicks + drags across the SV square, hue strip, and alpha
// slider (each grabs "active" on EventClick so a subsequent EventMouseDrag
// keeps moving the same control even after the cursor leaves its rect), and
// a plain click on the eyedropper button.
func (c *ColorPicker) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		switch {
		case c.eyedropRectLocal().Contains(ev.X, ev.Y):
			c.active = colorPickerActiveNone
			if c.OnEyedrop != nil {
				c.OnEyedrop()
			}
		case c.svRectLocal().Contains(ev.X, ev.Y):
			c.active = colorPickerActiveSV
			c.setSV(ev.X, ev.Y)
		case c.hueRectLocal().Contains(ev.X, ev.Y):
			c.active = colorPickerActiveHue
			c.setHue(ev.Y)
		case c.alphaRectLocal().Contains(ev.X, ev.Y):
			c.active = colorPickerActiveAlpha
			c.setAlpha(ev.X)
		default:
			c.active = colorPickerActiveNone
		}
	case EventMouseDrag:
		switch c.active {
		case colorPickerActiveSV:
			c.setSV(ev.X, ev.Y)
		case colorPickerActiveHue:
			c.setHue(ev.Y)
		case colorPickerActiveAlpha:
			c.setAlpha(ev.X)
		}
	case EventMouseUp:
		c.active = colorPickerActiveNone
	}
}

// setSV maps a widget-local (x, y), clamped into the SV square, to S/V and
// fires OnChange.
func (c *ColorPicker) setSV(x, y int) {
	sv := c.svRectLocal()
	x = clampInt(x, sv.X, sv.X+sv.W-1)
	y = clampInt(y, sv.Y, sv.Y+sv.H-1)
	c.S = float64(x-sv.X) / float64(max(sv.W-1, 1))
	c.V = 1 - float64(y-sv.Y)/float64(max(sv.H-1, 1))
	c.fireChange()
}

// setHue maps a widget-local y, clamped into the hue strip, to H and fires
// OnChange.
func (c *ColorPicker) setHue(y int) {
	hr := c.hueRectLocal()
	y = clampInt(y, hr.Y, hr.Y+hr.H-1)
	h := 360 * float64(y-hr.Y) / float64(max(hr.H-1, 1))
	if h >= 360 {
		h = 0
	}
	c.H = h
	c.fireChange()
}

// setAlpha maps a widget-local x, clamped into the alpha slider, to Alpha
// and fires OnChange.
func (c *ColorPicker) setAlpha(x int) {
	ar := c.alphaRectLocal()
	x = clampInt(x, ar.X, ar.X+ar.W-1)
	frac := float64(x-ar.X) / float64(max(ar.W-1, 1))
	c.Alpha = to8(frac)
	c.fireChange()
}

// fireChange invokes OnChange with the current Color(), if set.
func (c *ColorPicker) fireChange() {
	if c.OnChange != nil {
		c.OnChange(c.Color())
	}
}

// clampInt confines v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
