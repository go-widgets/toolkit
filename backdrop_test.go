// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// bdPx reads the RGBA pixel at (x, y) from a width-strided RGBA buffer.
func bdPx(buf []byte, width, x, y int) painter.RGBA {
	i := (y*width + x) * 4
	return painter.RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

// bdRender draws b onto a freshly zeroed w×h pixel buffer and returns it. An
// untouched pixel stays the zero RGBA (A=0), so tests can tell painted from
// unpainted ground apart.
func bdRender(b *Backdrop, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	b.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	b.Draw(p, theme)
	return buf
}

func TestNewBackdrop(t *testing.T) {
	fill := painter.RGB(0x11, 0x13, 0x1a)
	grid := painter.RGB(0x17, 0x1a, 0x24)
	b := NewBackdrop(fill, grid, 40)
	if b.Fill != fill {
		t.Errorf("Fill = %v, want %v", b.Fill, fill)
	}
	if b.Grid != grid {
		t.Errorf("Grid = %v, want %v", b.Grid, grid)
	}
	if b.Step != 40 {
		t.Errorf("Step = %d, want 40", b.Step)
	}
}

func TestBackdropDrawFillAndGrid(t *testing.T) {
	fill := painter.RGB(0x11, 0x13, 0x1a)
	grid := painter.RGB(0x17, 0x1a, 0x24)
	const w, h, step = 20, 20, 10
	buf := bdRender(NewBackdrop(fill, grid, step), w, h, DefaultDark())

	// A vertical grid line sits at x=0 and x=step; a horizontal one at y=0 and
	// y=step. A cell interior pixel (5,5) is neither, so it shows the fill.
	if got := bdPx(buf, w, 5, 5); got != fill {
		t.Errorf("interior (5,5) = %v, want fill %v", got, fill)
	}
	if got := bdPx(buf, w, step, 5); got != grid {
		t.Errorf("vertical line (%d,5) = %v, want grid %v", step, got, grid)
	}
	if got := bdPx(buf, w, 5, step); got != grid {
		t.Errorf("horizontal line (5,%d) = %v, want grid %v", step, got, grid)
	}
	if got := bdPx(buf, w, 0, 0); got != grid {
		t.Errorf("origin (0,0) = %v, want grid %v", got, grid)
	}
}

func TestBackdropDrawNoGrid(t *testing.T) {
	fill := painter.RGB(0x22, 0x22, 0x22)
	const w, h = 12, 12
	// Step <= 0 paints the fill only — no pixel carries the grid colour.
	buf := bdRender(NewBackdrop(fill, painter.RGB(0x99, 0x99, 0x99), 0), w, h, DefaultDark())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if got := bdPx(buf, w, x, y); got != fill {
				t.Fatalf("(%d,%d) = %v, want fill %v (grid must not paint)", x, y, got, fill)
			}
		}
	}
}

func TestBackdropDrawThemeDefaults(t *testing.T) {
	// Zero-value Fill/Grid fall back to theme.Background / theme.Border.
	theme := DefaultDark()
	const w, h, step = 16, 16, 8
	buf := bdRender(NewBackdrop(painter.RGBA{}, painter.RGBA{}, step), w, h, theme)

	if got := bdPx(buf, w, 3, 3); got != theme.Background {
		t.Errorf("interior (3,3) = %v, want theme.Background %v", got, theme.Background)
	}
	if got := bdPx(buf, w, step, 3); got != theme.Border {
		t.Errorf("grid line (%d,3) = %v, want theme.Border %v", step, got, theme.Border)
	}
}

func TestBackdropDrawEmptyBounds(t *testing.T) {
	theme := DefaultDark()
	// A zero-width backdrop paints nothing (W<=0 short-circuits)...
	zeroW := &Backdrop{Fill: painter.RGB(1, 2, 3), Step: 4}
	zeroW.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	p1 := painter.NewPixelPainter(make([]byte, 0), 0, 10)
	zeroW.Draw(p1, theme) // must not panic; nothing to assert on an empty buffer

	// ...and a zero-height backdrop (W>0, H<=0) exercises the second predicate.
	buf := make([]byte, 4*10*1)
	zeroH := &Backdrop{Fill: painter.RGB(1, 2, 3), Step: 4}
	zeroH.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 0})
	p2 := painter.NewPixelPainter(buf, 10, 1)
	zeroH.Draw(p2, theme)
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("empty-height backdrop painted byte %d = %d, want 0", i, v)
		}
	}
}
