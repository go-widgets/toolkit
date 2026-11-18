// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

var slateBrand = RGB(0x00, 0x79, 0xA8) // the go-authn account page's brand

// near reports whether two colours agree within 1 per channel.
func near(a, b RGBA) bool {
	d := func(x, y uint8) bool {
		if x > y {
			return x-y <= 1
		}
		return y-x <= 1
	}
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

// The derived brand variants reproduce the reference page's own values.
func TestSlateDerivesTheReferencePagesBrandVariants(t *testing.T) {
	l, d := SlateLight(slateBrand), SlateDark(slateBrand)
	for name, c := range map[string][2]RGBA{
		"light hover (#006890)": {l.Extra["brand_hover"], RGB(0x00, 0x68, 0x90)},
		"light ink (#0079a8)":   {l.Extra["brand_ink"], RGB(0x00, 0x79, 0xA8)},
		"dark hover (#248cb4)":  {d.Extra["brand_hover"], RGB(0x24, 0x8C, 0xB4)},
		"dark ink (#4ea2c3)":    {d.Extra["brand_ink"], RGB(0x4E, 0xA2, 0xC3)},
	} {
		if !near(c[0], c[1]) {
			t.Errorf("%s: got %+v, want %+v", name, c[0], c[1])
		}
	}
}

func TestSlatePalettes(t *testing.T) {
	l, d := SlateLight(slateBrand), SlateDark(slateBrand)
	assertPalette(t, "SlateLight", l,
		op(0xF8, 0xFA, 0xFC), // Background: slate-50
		op(0xFF, 0xFF, 0xFF), // Surface: white card
		op(0xF1, 0xF5, 0xF9), // SurfaceAlt: slate-100
		op(0x0F, 0x17, 0x2A), // OnBackground: slate-900
		op(0x0F, 0x17, 0x2A), // OnSurface
		op(0x00, 0x79, 0xA8), // Accent: the brand
		l.Border,             // Border: derived, pinned below
		op(0xFF, 0xFF, 0xFF), // OnAccent
	)
	assertPalette(t, "SlateDark", d,
		op(0x12, 0x15, 0x1A), // dark-bg
		op(0x1B, 0x20, 0x27), // dark-surface
		op(0x24, 0x2B, 0x34), // dark-raised
		op(0xF1, 0xF5, 0xF9), // slate-100
		op(0xF1, 0xF5, 0xF9),
		op(0x00, 0x79, 0xA8),
		d.Border,
		op(0xFF, 0xFF, 0xFF),
	)
	// The card border is a soft tone of its ground, not an ink.
	if !near(l.Border, RGB(0xE8, 0xED, 0xF3)) {
		t.Errorf("light border %+v", l.Border)
	}
	if !near(d.Border, RGB(0x2A, 0x2F, 0x36)) {
		t.Errorf("dark border %+v", d.Border)
	}
	// Light has the card shadow, dark does not (the page has none).
	if l.Extra["shadow"].A == 0 {
		t.Error("light theme has no shadow")
	}
	if _, ok := d.Extra["shadow"]; ok {
		t.Error("dark theme has a shadow")
	}
	for _, th := range []*Theme{l, d} {
		for _, k := range []string{"OnAccent", "brand_hover", "brand_ink", "chip_brand_bg", "chip_plain_bg", "chip_plain_ink", "muted"} {
			if _, ok := th.Extra[k]; !ok {
				t.Errorf("Extra[%q] missing", k)
			}
		}
	}
}

func TestSlateFollowsThePreference(t *testing.T) {
	if Slate(slateBrand, false).Background != slate50 || Slate(slateBrand, true).Background != slateDarkBg {
		t.Error("Slate(brand, dark) must pick the variant")
	}
	// Any brand works: the accent is the parameter.
	other := RGB(0x7C, 0x3A, 0xED)
	if SlateLight(other).Accent != other || SlateDark(other).Accent != other {
		t.Error("the accent is not the brand parameter")
	}
}

func TestShadeAndThemeInk(t *testing.T) {
	if shade(RGB(100, 100, 100), -1) != RGB(0, 0, 0) || shade(RGB(100, 100, 100), 1) != RGB(255, 255, 255) {
		t.Error("shade extremes")
	}
	if themeInk(SlateDark(slateBrand)) != SlateDark(slateBrand).Extra["brand_ink"] {
		t.Error("themeInk must prefer brand_ink")
	}
	if themeInk(DefaultLight()) != DefaultLight().Accent {
		t.Error("themeInk falls back to the accent")
	}
}
