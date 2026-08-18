// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ColorChooser is a 3-channel R/G/B picker with a live preview. Each
// channel is rendered as a horizontal track with a 1-pixel knob the
// user drags to change the value. The OnChange callback fires with
// the new RGBA whenever any channel moves.
//
// The widget owns the RGBA value; the host reads .Color() to get the
// current pick + may also stash a hex string via SetHex if there is
// a sibling Entry the user can type into.
type ColorChooser struct {
	Base
	Color    RGBA
	OnChange func(c RGBA)

	// active is the channel grabbed by the current press/drag as a 1-based
	// index (0 = none, 1 = R, 2 = G, 3 = B). Set on the EventClick that lands
	// on a track, consulted on each EventMouseDrag so the knob keeps scrubbing
	// its channel even after the pointer leaves the row, and cleared on
	// EventMouseUp -- mirroring ColorPicker.active.
	active int
}

// Sizing.
const (
	ColorChooserChannelH    = 22
	ColorChooserPreviewH    = 36
	ColorChooserPadX        = 8
	ColorChooserChannelPadY = 4
)

// Interior sizing bases in LOGICAL pixels, routed through scaled at use so the
// track, knob, swatch and captions grow with HiDPI and touch Density; identity
// at compact/1x. colorChooserLabelGutter is consumed by BOTH Draw and the click
// hit-test (setChannelFromX) so a scrub lands on the same value it paints.
const (
	colorChooserLabelGutter = 12 // gap between the R/G/B label and its track
	colorChooserTrackThick  = 4  // slider groove thickness
	colorChooserKnobW       = 3  // channel knob width
	colorChooserKnobH       = 10 // channel knob height
	colorChooserKnobRise    = 3  // knob overhang above the groove
	colorChooserSwatchW     = 40 // preview swatch width
	colorChooserSwatchInset = 48 // swatch distance from the right edge
	colorChooserSwatchPadY  = 8  // swatch top inset
	colorChooserLabelPadX   = 2  // R/G/B label left inset
	colorChooserHexPad      = 4  // hex caption right inset
	colorChooserHexGap      = 2  // gap under the swatch before the hex caption
)

// HitRect is the ColorChooser field's tap target: Bounds clamped up to the touch
// minimum on each axis and centred. Byte-identical to Bounds at
// [DensityCompact].
func (c *ColorChooser) HitRect() Rect { return hitRectFor(c.Bounds()) }

// NewColorChooser builds a chooser starting at initial. Alpha is
// forced to 0xFF so a freshly-constructed chooser always reads as
// fully-opaque.
func NewColorChooser(initial RGBA) *ColorChooser {
	if initial.A == 0 {
		initial.A = 0xFF
	}
	return &ColorChooser{Color: initial}
}

// Draw paints the 3 sliders + preview swatch + hex label.
func (c *ColorChooser) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)

	// 3 channel tracks.
	channelW := r.W - 2*scaled(ColorChooserPadX)
	gutter := scaled(colorChooserLabelGutter)
	trackThick := scaled(colorChooserTrackThick)
	knobW, knobH, knobRise := scaled(colorChooserKnobW), scaled(colorChooserKnobH), scaled(colorChooserKnobRise)
	for i, ch := range [3]string{"R", "G", "B"} {
		y := r.Y + scaled(ColorChooserChannelPadY) + i*scaled(ColorChooserChannelH)
		labelX := r.X + scaled(colorChooserLabelPadX)
		c.drawText(p, labelX, y+(scaled(ColorChooserChannelH)-c.glyphHeight())/2, ch, theme.OnSurface)
		trackX := r.X + scaled(ColorChooserPadX) + gutter
		trackY := y + scaled(ColorChooserChannelH)/2 - trackThick/2
		trackW := channelW - gutter
		fillRect(p, trackX, trackY, trackW, trackThick, theme.SurfaceAlt)
		strokeRect(p, trackX, trackY, trackW, trackThick, theme.Border)
		v := int(c.channel(i))
		knobX := trackX + v*trackW/255
		fillRect(p, knobX-knobW/2, trackY-knobRise, knobW, knobH, theme.Accent)
	}
	// Preview swatch in the right margin (centred on the chooser body).
	swatchW := scaled(colorChooserSwatchW)
	previewX := r.X + r.W - scaled(colorChooserSwatchInset)
	previewY := r.Y + scaled(colorChooserSwatchPadY)
	fillRect(p, previewX, previewY, swatchW, scaled(ColorChooserPreviewH), c.Color)
	strokeRect(p, previewX, previewY, swatchW, scaled(ColorChooserPreviewH), theme.Border)
	// Hex string under the swatch, right-aligned to the widget's edge so a
	// 7-char "#RRGGBB" — wider than the 40px swatch — never spills past the
	// right border (clamped so it also never runs off the left).
	hex := c.Hex()
	hexX := r.X + r.W - c.textWidth(hex) - scaled(colorChooserHexPad)
	if hexX < r.X+scaled(colorChooserLabelPadX) {
		hexX = r.X + scaled(colorChooserLabelPadX)
	}
	c.drawText(p, hexX, previewY+scaled(ColorChooserPreviewH)+scaled(colorChooserHexGap), hex, theme.OnSurface)
}

// OnEvent moves a channel knob by press + drag. An EventClick on a track grabs
// that channel (remembered in active) and sets it from the pointer X; each
// following EventMouseDrag re-runs the set for the grabbed channel from the new
// X -- so a drag scrubs the value continuously, even once the pointer strays out
// of the row -- and EventMouseUp releases the grab. A click that misses every
// track (e.g. on the preview/hex area) grabs nothing. Coordinates are
// widget-local.
func (c *ColorChooser) OnEvent(ev Event) {
	r := c.Bounds()
	switch ev.Kind {
	case EventClick:
		ch := c.channelAt(ev.Y, r)
		if ch < 0 {
			c.active = 0
			return
		}
		c.active = ch + 1
		c.setChannelFromX(ch, ev.X, r)
	case EventMouseDrag:
		if c.active == 0 {
			return
		}
		c.setChannelFromX(c.active-1, ev.X, r)
	case EventMouseUp:
		c.active = 0
	}
}

// channelAt returns the channel index (0=R, 1=G, 2=B) whose row contains the
// widget-local y, or -1 when y falls outside every channel row.
func (c *ColorChooser) channelAt(y int, r Rect) int {
	if y < 0 || y >= r.H {
		return -1
	}
	for i := 0; i < 3; i++ {
		yMin := scaled(ColorChooserChannelPadY) + i*scaled(ColorChooserChannelH)
		yMax := yMin + scaled(ColorChooserChannelH)
		if y >= yMin && y < yMax {
			return i
		}
	}
	return -1
}

// setChannelFromX maps a widget-local x to channel i's value (clamped to the
// track's ends, mirroring Draw's knobX = trackX + v*trackW/255 placement) and
// fires OnChange. Shared by the click and drag arms so a press and a scrub land
// on identical values for the same x.
func (c *ColorChooser) setChannelFromX(i, x int, r Rect) {
	gutter := scaled(colorChooserLabelGutter)
	trackX := scaled(ColorChooserPadX) + gutter
	channelW := r.W - 2*scaled(ColorChooserPadX)
	trackW := channelW - gutter
	switch {
	case x < trackX:
		c.setChannel(i, 0)
	case x >= trackX+trackW:
		c.setChannel(i, 255)
	default:
		c.setChannel(i, uint8((x-trackX)*255/trackW))
	}
	if c.OnChange != nil {
		c.OnChange(c.Color)
	}
}

// channel returns channel i (0=R, 1=G, 2=B).
func (c *ColorChooser) channel(i int) uint8 {
	switch i {
	case 0:
		return c.Color.R
	case 1:
		return c.Color.G
	case 2:
		return c.Color.B
	}
	return 0
}

func (c *ColorChooser) setChannel(i int, v uint8) {
	switch i {
	case 0:
		c.Color.R = v
	case 1:
		c.Color.G = v
	case 2:
		c.Color.B = v
	}
}

// Hex returns the color as "#RRGGBB".
func (c *ColorChooser) Hex() string {
	digits := "0123456789ABCDEF"
	b := []byte{'#', 0, 0, 0, 0, 0, 0}
	b[1] = digits[c.Color.R>>4]
	b[2] = digits[c.Color.R&0x0F]
	b[3] = digits[c.Color.G>>4]
	b[4] = digits[c.Color.G&0x0F]
	b[5] = digits[c.Color.B>>4]
	b[6] = digits[c.Color.B&0x0F]
	return string(b)
}

// SetHex parses "#RRGGBB" or "RRGGBB" into the chooser's color. Bad
// input is silently ignored so a malformed Entry payload can't break
// the picker state.
func (c *ColorChooser) SetHex(s string) {
	if len(s) == 7 && s[0] == '#' {
		s = s[1:]
	}
	if len(s) != 6 {
		return
	}
	r, ok1 := hex2(s[0], s[1])
	g, ok2 := hex2(s[2], s[3])
	b, ok3 := hex2(s[4], s[5])
	if !ok1 || !ok2 || !ok3 {
		return
	}
	c.Color = RGBA{R: r, G: g, B: b, A: 0xFF}
	if c.OnChange != nil {
		c.OnChange(c.Color)
	}
}

func hex2(hi, lo byte) (uint8, bool) {
	h, ok1 := hexNib(hi)
	l, ok2 := hexNib(lo)
	if !ok1 || !ok2 {
		return 0, false
	}
	return h<<4 | l, true
}

func hexNib(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}
