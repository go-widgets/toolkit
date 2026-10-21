// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// popupThemes are the palettes the outline is held to: the toolkit's two
// built-ins plus the go-slides playground's light and dark tokens, the palettes
// the bug was reported on ("the outline of an open combobox is not visible", in
// the dark and then in the light theme).
func popupThemes() map[string]*Theme {
	pg := func(bg, surf, ink, line, border, accent RGBA) *Theme {
		return &Theme{Background: bg, Surface: surf, SurfaceAlt: line, OnBackground: ink,
			OnSurface: ink, Accent: accent, Border: border}
	}
	return map[string]*Theme{
		"default-light": DefaultLight(),
		"default-dark":  DefaultDark(),
		"slides-light": pg(RGB(0xff, 0xff, 0xff), RGB(0xf7, 0xf8, 0xfb), RGB(0x0f, 0x11, 0x15),
			RGB(0xe7, 0xe9, 0xee), RGB(0xcf, 0xd4, 0xdd), RGB(0x4f, 0x46, 0xe5)),
		"slides-dark": pg(RGB(0x0b, 0x0e, 0x14), RGB(0x14, 0x1a, 0x24), RGB(0xe6, 0xed, 0xf3),
			RGB(0x23, 0x2a, 0x36), RGB(0x3a, 0x45, 0x56), RGB(0x9e, 0x99, 0xf1)),
	}
}

// TestPopupBorderContrastTokens: computed from the tokens alone, the popup
// outline reaches WCAG 2.2 1.4.11's 3:1 against the page and the popup body in
// every theme, and the raw Theme.Border does NOT (so the test fails on the old
// tokens in both light and dark).
func TestPopupBorderContrastTokens(t *testing.T) {
	for name, th := range popupThemes() {
		b := PopupBorder(th)
		for what, ground := range map[string]RGBA{"background": th.Background, "surface": th.Surface} {
			if r := contrastRatio(b, ground); r < PopupBorderMinContrast {
				t.Errorf("%s: popup border vs %s = %.2f:1, want >= 3:1", name, what, r)
			}
		}
		raw := contrastRatio(th.Border, th.Background)
		if raw >= PopupBorderMinContrast {
			t.Errorf("%s: raw Border already %.2f:1; the fixture no longer exercises the defect", name, raw)
		}
		t.Logf("%s: Border %.2f:1 -> PopupBorder %.2f:1 vs background", name, raw, contrastRatio(b, th.Background))
	}
	// A theme whose Border is already strong is returned unchanged.
	strong := &Theme{Background: RGB(255, 255, 255), Surface: RGB(255, 255, 255),
		OnSurface: RGB(0, 0, 0), Border: RGB(0x40, 0x40, 0x40)}
	if PopupBorder(strong) != strong.Border {
		t.Error("a strong Border must be kept as is")
	}
	// Nothing reaches 3:1 (everything mid grey): fall back to OnSurface.
	grey := &Theme{Background: RGB(0x80, 0x80, 0x80), Surface: RGB(0x80, 0x80, 0x80),
		OnSurface: RGB(0x70, 0x70, 0x70), Border: RGB(0x7c, 0x7c, 0x7c)}
	if PopupBorder(grey) != grey.OnSurface {
		t.Error("unreachable contrast must fall back to OnSurface")
	}
}

// TestOpenDropDownFieldTakesAccentEdge: an open DropDown's field is stroked in
// the accent, like an open ComboBox, so field and list read as one unit.
func TestOpenDropDownFieldTakesAccentEdge(t *testing.T) {
	th := DefaultDark()
	d := NewDropDown([]string{"a", "b"}, 0)
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 22})
	buf := makeSurface(100, 40)
	d.Draw(newP(buf, 100), th)
	if got := pixelAt(buf, 100, 40, 0); got != th.Border {
		t.Fatalf("closed field edge = %+v, want Border", got)
	}
	d.Open().Set(true)
	d.Draw(newP(buf, 100), th)
	if got := pixelAt(buf, 100, 40, 0); got != th.Accent {
		t.Fatalf("open field edge = %+v, want Accent", got)
	}
}

// TestOpenPopupOutlineContrastPixels renders each popup-like widget open, on its
// theme's page colour, and measures the outline pixel actually painted.
func TestOpenPopupOutlineContrastPixels(t *testing.T) {
	const w, h = 220, 200
	for name, th := range popupThemes() {
		type drawn struct {
			pb   Rect
			draw func(buf []byte)
		}
		dd := NewDropDown([]string{"Alpha", "Beta", "Gamma"}, 1)
		dd.SetBounds(Rect{X: 10, Y: 10, W: 120, H: 22})
		dd.Open().Set(true)
		cb := NewComboBox([]string{"Alpha", "Beta", "Gamma"})
		cb.SetBounds(Rect{X: 10, Y: 10, W: 120, H: 22})
		cb.Open().Set(true)
		mn := NewMenu([]MenuItem{{Label: "Open"}, {Label: "Save"}})
		mn.SetBounds(Rect{X: 10, Y: 40, W: 120, H: 50})
		cases := map[string]drawn{
			"dropdown": {dd.PopoverBounds(), func(b []byte) { dd.DrawPopover(newP(b, w), th) }},
			"combobox": {cb.PopoverBounds(), func(b []byte) { cb.Draw(newP(b, w), th) }},
			"menu":     {mn.Bounds(), func(b []byte) { mn.Draw(newP(b, w), th) }},
		}
		for wn, c := range cases {
			buf := makeSurface(w, h)
			for i := 0; i+3 < len(buf); i += 4 {
				buf[i], buf[i+1], buf[i+2], buf[i+3] = th.Background.R, th.Background.G, th.Background.B, 0xFF
			}
			c.draw(buf)
			// Top edge, mid width: the outline row.
			got := pixelAt(buf, w, c.pb.X+c.pb.W/2, c.pb.Y)
			for what, ground := range map[string]RGBA{"background": th.Background, "surface": th.Surface} {
				if r := contrastRatio(got, ground); r < PopupBorderMinContrast {
					t.Errorf("%s/%s: painted outline %v vs %s = %.2f:1, want >= 3:1", name, wn, got, what, r)
				}
			}
		}
	}
}
