// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// PreferredWidth grows with the button's content — a leading icon, a longer
// caption (the Source⇄WYSIWYG toggle case), and a shortcut hint each add width —
// so a host can size a button to fit instead of hard-coding a width that clips.
func TestButtonPreferredWidth(t *testing.T) {
	b := NewButton("Find", nil)
	base := b.PreferredWidth()
	if base <= 0 {
		t.Fatalf("captioned button width = %d, want > 0", base)
	}
	b.LeadingIcon().Set(func(p painter.Painter, r Rect, ink RGBA) {})
	withIcon := b.PreferredWidth()
	if withIcon <= base {
		t.Errorf("a leading icon should widen the button: base=%d withIcon=%d", base, withIcon)
	}
	b.Shortcut().Set("Ctrl+F")
	withSc := b.PreferredWidth()
	if withSc <= withIcon {
		t.Errorf("a shortcut should widen the button: withIcon=%d withSc=%d", withIcon, withSc)
	}

	// The toggled-caption case: WYSIWYG must ask for more width than Source.
	if long, short := NewButton("WYSIWYG", nil).PreferredWidth(), NewButton("Source", nil).PreferredWidth(); long <= short {
		t.Errorf("WYSIWYG (%d) should be wider than Source (%d)", long, short)
	}
	// An empty caption with only an icon is still positive (covers the empty-label
	// path), and an Icon-that-replaces-the-caption is a square.
	empty := NewButton("", nil)
	empty.LeadingIcon().Set(func(p painter.Painter, r Rect, ink RGBA) {})
	if empty.PreferredWidth() <= 0 {
		t.Error("an icon-only (empty caption) button needs a positive width")
	}
	ib := NewButton("x", nil)
	ib.Icon = func(p painter.Painter, r Rect, ink RGBA) {}
	if ib.PreferredWidth() <= 0 {
		t.Error("an Icon-replaces-caption button needs a positive width")
	}
}

// PlainTitle paints the dialog's title bar in the panel Surface (a card look)
// instead of the accent fill; the default keeps the accent bar.
func TestDialogPlainTitle(t *testing.T) {
	const W, H = 220, 160
	theme := DefaultLight()

	accentInTitle := func(plain bool) bool {
		d := &Dialog{Title: "Find and replace", Closable: true, PlainTitle: plain}
		d.SetBounds(Rect{X: 10, Y: 10, W: W - 20, H: H - 20})
		buf := makeSurface(W, H)
		d.Draw(newP(buf, W), theme)
		for y := 11; y < 10+scaled(DialogTitleH); y++ {
			for x := 11; x < W-11; x++ {
				if pixelAt(buf, W, x, y) == theme.Accent {
					return true
				}
			}
		}
		return false
	}

	if !accentInTitle(false) {
		t.Error("a default dialog should paint an accent title bar")
	}
	if accentInTitle(true) {
		t.Error("a PlainTitle dialog must not paint the accent title bar")
	}
}
