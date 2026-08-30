// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// A button's optional leading icon and trailing shortcut are reactive MVVM
// properties drawn ALONGSIDE the caption (not replacing it). Setting the icon
// paints it before the label; setting the shortcut paints a hint at the right.
func TestButtonLeadingIconAndShortcut(t *testing.T) {
	const w, h = 160, 30
	theme := DefaultLight()
	iconMark := RGB(0xE0, 0x10, 0x20) // a distinctive colour the icon paints

	b := NewButton("Find", nil)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	// Unset by default: leadingIconFn/shortcutText read nil/"" without forcing the
	// Observables to exist, and the caption draws centred (legacy path).
	if b.leadingIconFn() != nil || b.shortcutText() != "" {
		t.Fatal("a fresh button must report no icon and no shortcut")
	}

	drawn := func() []byte {
		buf := makeSurface(w, h)
		b.Draw(newP(buf, w), theme)
		return buf
	}
	has := func(buf []byte, c RGBA) bool {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pixelAt(buf, w, x, y) == c {
					return true
				}
			}
		}
		return false
	}

	base := drawn()
	if has(base, iconMark) {
		t.Fatal("no icon set, yet the icon mark appeared")
	}

	// Set the leading icon (a reactive property) — it paints its mark.
	b.LeadingIcon().Set(func(p painter.Painter, r Rect, ink RGBA) {
		fillRect(p, r.X, r.Y, r.W, r.H, iconMark)
	})
	if b.leadingIconFn() == nil {
		t.Fatal("LeadingIcon().Set did not take")
	}
	if !has(drawn(), iconMark) {
		t.Fatal("the leading icon was set but did not paint")
	}

	// Set the shortcut hint — text ink appears in the right third of the button.
	b.Shortcut().Set("Ctrl+F")
	if b.shortcutText() != "Ctrl+F" {
		t.Fatalf("shortcut = %q, want Ctrl+F", b.shortcutText())
	}
	buf := drawn()
	inkRight := false
	for y := 0; y < h; y++ {
		for x := 2 * w / 3; x < w; x++ {
			if pixelAt(buf, w, x, y) == mutedInk(theme) {
				inkRight = true
			}
		}
	}
	if !inkRight {
		t.Fatal("the shortcut hint did not paint in the button's right third")
	}
}

// The adorned row also handles an empty caption (icon-and-shortcut only), and
// clearing both adornments returns to the centred-caption path.
func TestButtonAdornmentsEmptyLabelAndClear(t *testing.T) {
	const w, h = 120, 28
	theme := DefaultLight()
	b := NewButton("", nil)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b.LeadingIcon().Set(func(p painter.Painter, r Rect, ink RGBA) {})
	b.Shortcut().Set("F3")
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme) // empty label + icon + shortcut: must not panic

	// Clearing both returns to the legacy centred path.
	b.LeadingIcon().Set(nil)
	b.Shortcut().Set("")
	b.Label().Set("Go")
	buf2 := makeSurface(w, h)
	b.Draw(newP(buf2, w), theme)
	if !func() bool {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pixelAt(buf2, w, x, y) == theme.OnSurface {
					return true
				}
			}
		}
		return false
	}() {
		t.Fatal("cleared button did not draw its centred caption")
	}
}
