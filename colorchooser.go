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
}

// Sizing.
const (
	ColorChooserChannelH    = 22
	ColorChooserPreviewH    = 36
	ColorChooserPadX        = 8
	ColorChooserChannelPadY = 4
)

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
	channelW := r.W - 2*ColorChooserPadX
	for i, ch := range [3]string{"R", "G", "B"} {
		y := r.Y + ColorChooserChannelPadY + i*ColorChooserChannelH
		labelX := r.X + 2
		DrawText(p, labelX, y+(ColorChooserChannelH-GlyphHeight)/2, ch, theme.OnSurface)
		trackX := r.X + ColorChooserPadX + 12
		trackY := y + ColorChooserChannelH/2 - 2
		trackW := channelW - 12
		fillRect(p, trackX, trackY, trackW, 4, theme.SurfaceAlt)
		strokeRect(p, trackX, trackY, trackW, 4, theme.Border)
		v := int(c.channel(i))
		knobX := trackX + v*trackW/255
		fillRect(p, knobX-1, trackY-3, 3, 10, theme.Accent)
	}
	// Preview swatch in the right margin (centred on the chooser body).
	previewX := r.X + r.W - 48
	previewY := r.Y + 8
	fillRect(p, previewX, previewY, 40, ColorChooserPreviewH, c.Color)
	strokeRect(p, previewX, previewY, 40, ColorChooserPreviewH, theme.Border)
	// Hex string under the swatch.
	hex := c.Hex()
	DrawText(p, previewX, previewY+ColorChooserPreviewH+2, hex, theme.OnSurface)
}

// OnEvent handles clicks on the 3 tracks to move the channel knob.
func (c *ColorChooser) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := c.Bounds()
	// Translate ev to widget-local; callers may already have done this.
	x := ev.X
	y := ev.Y
	// Reject events outside the vertical band (the channel rows live
	// at predictable Y ranges); horizontal overshoot is clamped per
	// channel so a click "far right" snaps to the track's right edge.
	if y < 0 || y >= r.H {
		return
	}
	for i := 0; i < 3; i++ {
		yMin := ColorChooserChannelPadY + i*ColorChooserChannelH
		yMax := yMin + ColorChooserChannelH
		if y < yMin || y >= yMax {
			continue
		}
		trackX := ColorChooserPadX + 12
		channelW := r.W - 2*ColorChooserPadX
		trackW := channelW - 12
		if x < trackX {
			c.setChannel(i, 0)
		} else if x >= trackX+trackW {
			c.setChannel(i, 255)
		} else {
			v := (x - trackX) * 255 / trackW
			c.setChannel(i, uint8(v))
		}
		if c.OnChange != nil {
			c.OnChange(c.Color)
		}
		return
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
