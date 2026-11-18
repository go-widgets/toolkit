// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Slate is the look of a Tailwind "slate" web app with one brand accent: a
// slate-50 page, white cards with a soft border and shadow, slate-900 ink, and
// the brand colour for the accent, the primary button and the brand chip. It
// is modelled on the go-authn account page (login.mesocentre.plateau-de-saclay.net):
// `--brand:#0079a8; --brand-on:#fff; --brand-hover:#006890; --brand-ink:#0079a8`
// in light, `--brand-hover:#248cb4; --brand-ink:#4ea2c3` in dark.
//
// SlateLight and SlateDark take the brand colour as a parameter and DERIVE the
// two variants from it the way that page's values relate: hover is the brand
// darkened 14% in light and lightened 14% in dark; ink (the brand as text on a
// dark ground) is the brand lightened 31%. For #0079a8 that reproduces the page's
// #006890, #248cb4 and #4ea2c3 exactly (see the test).
//
// What a palette cannot say a Theme says in Extra, which every widget that wants
// it reads and every widget that does not ignores:
//
//	OnAccent        ink on an accent fill (brand-on)
//	brand_hover     the accent when pointed at
//	brand_ink       the brand as TEXT or an icon on this theme's ground
//	chip_brand_bg   a brand chip's fill (brand-ink at 10% over the surface)
//	chip_plain_bg   a plain chip's fill (slate-100 / white at 10%)
//	chip_plain_ink  a plain chip's text (slate-700 / slate-200)
//	shadow          the card shadow's colour with its alpha; absent in dark,
//	                where the page has none
//
// The system's light/dark preference is the HOST's to read (a browser's
// prefers-color-scheme, a desktop's setting): Slate(brand, dark) returns the
// variant to use, and the host calls it again when the preference changes.

// The Tailwind v4 slate scale (the ramp the reference page is built from).
var (
	slate50  = RGB(0xF8, 0xFA, 0xFC)
	slate100 = RGB(0xF1, 0xF5, 0xF9)
	slate200 = RGB(0xE2, 0xE8, 0xF0)
	slate300 = RGB(0xCB, 0xD5, 0xE1)
	slate400 = RGB(0x94, 0xA3, 0xB8)
	slate700 = RGB(0x33, 0x41, 0x55)
	slate900 = RGB(0x0F, 0x17, 0x2A)

	// The reference page's dark ground, card and raised card.
	slateDarkBg      = RGB(0x12, 0x15, 0x1A)
	slateDarkSurface = RGB(0x1B, 0x20, 0x27)
	slateDarkRaised  = RGB(0x24, 0x2B, 0x34)

	white = RGB(0xFF, 0xFF, 0xFF)
)

// shade moves c toward black (t < 0) or white (t > 0) by |t|.
func shade(c RGBA, t float64) RGBA {
	if t < 0 {
		return blendRGBA(RGB(0, 0, 0), c, -t)
	}
	return blendRGBA(white, c, t)
}

// SlateLight is the light Slate theme with the given brand accent.
func SlateLight(brand RGBA) *Theme {
	return &Theme{
		Background:   slate50,
		Surface:      white,
		SurfaceAlt:   slate100,
		OnBackground: slate900,
		OnSurface:    slate900,
		Accent:       brand,
		// slate-200 at 80% over white: the card border.
		Border:     blendRGBA(slate200, white, 0.8),
		StarFilled: RGB(0xF5, 0xB8, 0x00),
		StarEmpty:  slate300,
		Extra: map[string]RGBA{
			"OnAccent":       white,
			"brand_hover":    shade(brand, -0.14),
			"brand_ink":      brand,
			"chip_brand_bg":  blendRGBA(brand, white, 0.10),
			"chip_plain_bg":  slate100,
			"chip_plain_ink": slate700,
			"muted":          slate400,
			// #101828 at 4% and 8%: the .surface shadow's two layers.
			"shadow": RGBA{R: 0x10, G: 0x18, B: 0x28, A: 0x14},
		},
	}
}

// SlateDark is the dark Slate theme with the given brand accent. The accent
// itself stays the brand (a fill with white ink); text and icons in the brand
// use brand_ink, which is lightened to read on the dark ground.
func SlateDark(brand RGBA) *Theme {
	return &Theme{
		Background:   slateDarkBg,
		Surface:      slateDarkSurface,
		SurfaceAlt:   slateDarkRaised,
		OnBackground: slate100,
		OnSurface:    slate100,
		Accent:       brand,
		// white at 7% over the card: the card border.
		Border:     blendRGBA(white, slateDarkSurface, 0.07),
		StarFilled: RGB(0xFF, 0xC9, 0x33),
		StarEmpty:  slate700,
		Extra: map[string]RGBA{
			"OnAccent":       white,
			"brand_hover":    shade(brand, 0.14),
			"brand_ink":      shade(brand, 0.306),
			"chip_brand_bg":  blendRGBA(shade(brand, 0.306), slateDarkSurface, 0.10),
			"chip_plain_bg":  blendRGBA(white, slateDarkSurface, 0.10),
			"chip_plain_ink": slate200,
			"muted":          slate400,
		},
	}
}

// Slate returns SlateDark(brand) when dark, else SlateLight(brand): the theme a
// host picks from the system's colour-scheme preference.
func Slate(brand RGBA, dark bool) *Theme {
	if dark {
		return SlateDark(brand)
	}
	return SlateLight(brand)
}

// themeInk is the brand-as-text colour of a theme: Extra["brand_ink"] when the
// theme carries one, else its Accent.
func themeInk(theme *Theme) RGBA {
	if c, ok := theme.Extra["brand_ink"]; ok {
		return c
	}
	return theme.Accent
}
