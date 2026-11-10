// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// cssNamedColors are the CSS named colours a theme sheet commonly uses; any
// other name is reported as not understood, never guessed.
var cssNamedColors = map[string]RGBA{
	"black": {R: 0, G: 0, B: 0, A: 255}, "white": {R: 255, G: 255, B: 255, A: 255},
	"red": {R: 255, G: 0, B: 0, A: 255}, "lime": {R: 0, G: 255, B: 0, A: 255},
	"blue": {R: 0, G: 0, B: 255, A: 255}, "yellow": {R: 255, G: 255, B: 0, A: 255},
	"cyan": {R: 0, G: 255, B: 255, A: 255}, "aqua": {R: 0, G: 255, B: 255, A: 255},
	"magenta": {R: 255, G: 0, B: 255, A: 255}, "fuchsia": {R: 255, G: 0, B: 255, A: 255},
	"gray": {R: 128, G: 128, B: 128, A: 255}, "grey": {R: 128, G: 128, B: 128, A: 255},
	"silver": {R: 192, G: 192, B: 192, A: 255}, "maroon": {R: 128, G: 0, B: 0, A: 255},
	"olive": {R: 128, G: 128, B: 0, A: 255}, "green": {R: 0, G: 128, B: 0, A: 255},
	"purple": {R: 128, G: 0, B: 128, A: 255}, "teal": {R: 0, G: 128, B: 128, A: 255},
	"navy": {R: 0, G: 0, B: 128, A: 255}, "orange": {R: 255, G: 165, B: 0, A: 255},
	"transparent": {},
}

// ParseCSSColor reads a CSS colour: #rgb, #rgba, #rrggbb, #rrggbbaa, rgb(),
// rgba(), hsl(), hsla() (comma or space separated, percentages or numbers) or
// one of the common names. ok is false for anything else (var(), calc(), an
// unlisted name): the caller then keeps the text as it is.
func ParseCSSColor(s string) (RGBA, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if c, ok := cssNamedColors[s]; ok {
		return c, true
	}
	if rest, ok := strings.CutPrefix(s, "#"); ok {
		return parseHexDigits(rest)
	}
	for _, fn := range []string{"rgba", "rgb", "hsla", "hsl"} {
		if body, ok := strings.CutPrefix(s, fn+"("); ok && strings.HasSuffix(body, ")") {
			return parseColorFunc(fn, strings.TrimSuffix(body, ")"))
		}
	}
	return RGBA{}, false
}

func parseHexDigits(h string) (RGBA, bool) {
	for _, r := range h {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return RGBA{}, false
		}
	}
	if len(h) == 3 || len(h) == 4 {
		var d []byte
		for i := 0; i < len(h); i++ {
			d = append(d, h[i], h[i])
		}
		h = string(d)
	}
	if len(h) != 6 && len(h) != 8 {
		return RGBA{}, false
	}
	v, _ := strconv.ParseUint(h, 16, 32) // h is 6 or 8 hex digits: it fits
	if len(h) == 6 {
		return RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, true
	}
	return RGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, true
}

func parseColorFunc(fn, body string) (RGBA, bool) {
	body = strings.NewReplacer(",", " ", "/", " ").Replace(body)
	f := strings.Fields(body)
	if len(f) != 3 && len(f) != 4 {
		return RGBA{}, false
	}
	// num reads a number, or a percentage scaled to full.
	num := func(t string, full float64) (float64, bool) {
		pct := strings.HasSuffix(t, "%")
		v, err := strconv.ParseFloat(strings.TrimSuffix(t, "%"), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		if pct {
			v = v / 100 * full
		}
		return v, true
	}
	alpha := 1.0
	if len(f) == 4 {
		a, ok := num(f[3], 1)
		if !ok {
			return RGBA{}, false
		}
		alpha = math.Min(math.Max(a, 0), 1)
	}
	clamp8 := func(v float64) uint8 { return uint8(math.Round(math.Min(math.Max(v, 0), 255))) }
	if strings.HasPrefix(fn, "rgb") {
		var ch [3]float64
		for i := range ch {
			v, ok := num(f[i], 255)
			if !ok {
				return RGBA{}, false
			}
			ch[i] = v
		}
		return RGBA{R: clamp8(ch[0]), G: clamp8(ch[1]), B: clamp8(ch[2]), A: clamp8(alpha * 255)}, true
	}
	h, ok := num(strings.TrimSuffix(f[0], "deg"), 360)
	if !ok {
		return RGBA{}, false
	}
	sat, ok1 := num(f[1], 1)
	lig, ok2 := num(f[2], 1)
	if !ok1 || !ok2 || !strings.HasSuffix(f[1], "%") || !strings.HasSuffix(f[2], "%") {
		return RGBA{}, false
	}
	r, g, b := hslToRGB(h, math.Min(math.Max(sat, 0), 1), math.Min(math.Max(lig, 0), 1))
	return RGBA{R: r, G: g, B: b, A: clamp8(alpha * 255)}, true
}

func hslToRGB(h, s, l float64) (r, g, b uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}
	return to8(rf + m), to8(gf + m), to8(bf + m)
}

// FormatCSSColor writes c as CSS hex: #rrggbb when opaque, #rrggbbaa otherwise.
func FormatCSSColor(c RGBA) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}
