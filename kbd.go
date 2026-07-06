// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Kbd renders a keyboard-shortcut hint like the "⌘K" chip beside a
// menu item: a small bordered box with the key text centred inside.
// Uses Theme.Surface for the face + Theme.Border for the stroke so the
// chip reads as a raised inlay against the parent panel.
//
// Kbd is passive (no OnEvent handling); the caller sets Bounds to
// position it — a Kbd typically lives to the right of a menu label,
// vertically centred with the menu row. Nothing about the widget
// depends on the input actually being pressed; it's a purely visual
// mnemonic.
type Kbd struct {
	Base
	Keys string
}

// KbdPadX / KbdPadY are the internal margin between the border box and
// the key text glyphs. Small values keep the chip compact enough to
// nest inside a menu row.
const (
	KbdPadX = 4
	KbdPadY = 2
)

// NewKbd constructs a Kbd carrying the given key text. Callers set
// Bounds via SetBounds before Draw; a natural fit is
// {W: TextWidth(keys) + 2*KbdPadX, H: GlyphHeight + 2*KbdPadY}.
func NewKbd(keys string) *Kbd { return &Kbd{Keys: keys} }

// Draw paints the chip: filled Surface body, 1-px Border stroke,
// Keys text centred in OnSurface. Zero-size Bounds degrade to a no-op
// via fillRect/strokeRect's own dimension guards.
func (k *Kbd) Draw(p painter.Painter, theme *Theme) {
	r := k.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	tw := TextWidth(k.Keys)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-GlyphHeight)/2
	DrawText(p, tx, ty, k.Keys, theme.OnSurface)
}
