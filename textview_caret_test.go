// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import "testing"

// A focused TextView draws its insertion caret by default; SetCaretVisible(false)
// suppresses it (the seam a host uses to blink the caret) without disturbing focus,
// and SetCaretVisible(true) restores it. The caret is asserted from the painted
// pixels: hiding it must remove OnSurface ink (the caret bar) from the view.
func TestTextViewCaretVisibility(t *testing.T) {
	const w, h = 120, 40
	theme := DefaultLight()
	v := NewTextView("")
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v.Focused().Set(true)

	if !v.CaretVisible() {
		t.Fatal("caret should be visible by default")
	}

	onSurfacePixels := func() int {
		buf := makeSurface(w, h)
		v.Draw(newP(buf, w), theme)
		n := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pixelAt(buf, w, x, y) == theme.OnSurface {
					n++
				}
			}
		}
		return n
	}

	visible := onSurfacePixels()

	v.SetCaretVisible(false)
	if v.CaretVisible() {
		t.Fatal("CaretVisible should be false after SetCaretVisible(false)")
	}
	hidden := onSurfacePixels()
	if hidden >= visible {
		t.Fatalf("hiding the caret should remove OnSurface ink: visible=%d hidden=%d", visible, hidden)
	}

	v.SetCaretVisible(true)
	if !v.CaretVisible() {
		t.Fatal("CaretVisible should be true again after SetCaretVisible(true)")
	}
	if restored := onSurfacePixels(); restored != visible {
		t.Fatalf("restoring the caret should paint it again: got %d, want %d", restored, visible)
	}
}
