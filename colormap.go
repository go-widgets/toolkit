// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "math"

// viridisStops are nine evenly spaced samples of matplotlib's "viridis"
// colormap (Stéfan van der Walt and Nathaniel Smith, CC0). Viridis is
// perceptually uniform and stays readable to the common forms of colour
// blindness and in greyscale, which is why it is the default sequential map
// of matplotlib and of most plotting libraries since.
var viridisStops = [...]RGBA{
	RGB(0x44, 0x01, 0x54),
	RGB(0x47, 0x2c, 0x7a),
	RGB(0x3b, 0x51, 0x8b),
	RGB(0x2c, 0x71, 0x8e),
	RGB(0x21, 0x90, 0x8d),
	RGB(0x27, 0xad, 0x81),
	RGB(0x5c, 0xc8, 0x63),
	RGB(0xaa, 0xdc, 0x32),
	RGB(0xfd, 0xe7, 0x25),
}

// Viridis maps t in [0, 1] onto the viridis sequential colormap, from dark
// purple (0) through teal to yellow (1), by linear interpolation between
// nine samples of the reference map. t outside [0, 1] is clamped and NaN maps
// to 0. It is the colour scale for a heatmap -- a [Table] with CellFill, a
// spectrogram, any matrix shown as colour -- so every app scales values the
// same way instead of inventing its own ramp.
func Viridis(t float64) RGBA {
	if !(t > 0) { // also catches NaN
		return viridisStops[0]
	}
	if t >= 1 {
		return viridisStops[len(viridisStops)-1]
	}
	pos := t * float64(len(viridisStops)-1)
	i := int(pos)
	f := pos - float64(i)
	a, b := viridisStops[i], viridisStops[i+1]
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*f)) }
	return RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xFF}
}

// ContrastInk returns black or white, whichever has the higher WCAG contrast
// ratio against bg -- the ink for text laid over an arbitrary fill, such as a
// heatmap cell.
func ContrastInk(bg RGBA) RGBA {
	l := relativeLuminance(bg)
	// Contrast with black is (l+0.05)/0.05, with white 1.05/(l+0.05); black
	// wins when the first is larger, i.e. when (l+0.05)^2 > 0.0525.
	if (l+0.05)*(l+0.05) > 0.0525 {
		return RGB(0, 0, 0)
	}
	return RGB(0xFF, 0xFF, 0xFF)
}

// relativeLuminance is the WCAG 2.x relative luminance of c, ignoring alpha.
func relativeLuminance(c RGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}
