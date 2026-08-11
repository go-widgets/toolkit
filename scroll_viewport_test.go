// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// plainPainter implements the base Painter and nothing else — no Clipper, no
// Translator. It stands for a back-end that cannot translate, and exists to
// prove the fallback still scrolls rather than showing the content unmoved.
type plainPainter struct {
	w, h  int
	drawn []painter.Rect
}

func (p *plainPainter) FillRect(r painter.Rect, _ painter.RGBA) {
	p.drawn = append(p.drawn, r)
}
func (p *plainPainter) StrokeRect(painter.Rect, painter.RGBA, int)           {}
func (p *plainPainter) FillRoundRect(painter.Rect, int, painter.RGBA)        {}
func (p *plainPainter) StrokeRoundRect(painter.Rect, int, painter.RGBA, int) {}
func (p *plainPainter) PutPixel(int, int, painter.RGBA)                      {}
func (p *plainPainter) Text(int, int, string, painter.RGBA)                  {}
func (p *plainPainter) Size() (int, int)                                     { return p.w, p.h }

// markerWidget paints one rectangle at its own bounds, so a test can see
// exactly where a parent decided to put it.
type markerWidget struct{ Base }

func (m *markerWidget) Draw(p painter.Painter, _ *Theme) {
	p.FillRect(m.Bounds(), painter.RGBA{R: 255, G: 255, B: 255, A: 255})
}

func newScrolled(child Widget) *ScrollView {
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})
	sv.SetContentSize(100, 400)
	sv.Scroll(0, 120)
	return sv
}

// The scrolled child is PAINTED at the offset position while its bounds stay
// at the viewport origin. That separation is the whole point: bounds are
// content space, the paint is viewport space, and anything reading geometry
// between frames — the accessibility bridges — now sees a stable answer
// instead of whatever the last Draw happened to leave behind.
func TestScrollViewPaintsTranslatedWithoutMovingTheChild(t *testing.T) {
	child := &markerWidget{}
	child.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	sv := newScrolled(child)

	buf := make([]byte, 100*50*4)
	p := &painter.PixelPainter{Buf: buf, Width: 100, Height: 50}
	sv.Draw(p, DefaultDark())

	if got := child.Bounds(); got.X != 0 || got.Y != 0 {
		t.Errorf("child bounds after Draw = %+v, want the viewport origin 0,0", got)
	}
	// The marker covers the whole content, so with 120 scrolled away the top
	// row of the viewport must still be painted.
	if buf[3] == 0 {
		t.Error("nothing painted at the top of the viewport")
	}
}

// A back-end that cannot translate must still scroll. It falls back to moving
// the child's bounds, which is what this widget did everywhere before.
func TestScrollViewFallsBackWhenThePainterCannotTranslate(t *testing.T) {
	child := &markerWidget{}
	child.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	sv := newScrolled(child)

	p := &plainPainter{w: 100, h: 50}
	sv.Draw(p, DefaultDark())

	var found bool
	for _, r := range p.drawn {
		if r.Y == -120 {
			found = true
		}
	}
	if !found {
		t.Fatalf("the child was not drawn at the scrolled position; got %+v", p.drawn)
	}
	// The fallback restores the bounds it borrowed, so nothing leaks out of Draw.
	if got := child.Bounds(); got.Y != 0 {
		t.Errorf("child bounds after the fallback Draw = %+v, want them restored", got)
	}
}
