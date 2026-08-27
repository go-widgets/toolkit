// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// strokeSpy records the strokes a widget asks for, which is the whole of what a
// SelectionBox does.
type strokeSpy struct {
	strokes []struct {
		r painter.Rect
		c RGBA
		w int
	}
}

func (s *strokeSpy) FillRect(painter.Rect, RGBA) {}
func (s *strokeSpy) StrokeRect(r painter.Rect, c RGBA, w int) {
	s.strokes = append(s.strokes, struct {
		r painter.Rect
		c RGBA
		w int
	}{r, c, w})
}
func (s *strokeSpy) FillRoundRect(painter.Rect, int, RGBA)        {}
func (s *strokeSpy) StrokeRoundRect(painter.Rect, int, RGBA, int) {}
func (s *strokeSpy) PutPixel(int, int, RGBA)                      {}
func (s *strokeSpy) Text(int, int, string, RGBA)                  {}
func (s *strokeSpy) Size() (int, int)                             { return 4000, 4000 }

// TestASelectionBoxIsOneBorderInsideItsBounds.
//
// Inside, not around: a border drawn outside the rectangle it marks would sit in
// whatever gap the host left between tiles — over the neighbour, if there is
// none — and on an outermost tile it would fall off the surface altogether.
func TestASelectionBoxIsOneBorderInsideItsBounds(t *testing.T) {
	want := Rect{X: 40, Y: 60, W: 300, H: 200}
	b := NewSelectionBox(RGB(0xFF, 0x8C, 0x1A))
	b.SetBounds(want)

	spy := &strokeSpy{}
	b.Draw(spy, DefaultDark())
	if len(spy.strokes) != 1 {
		t.Fatalf("it drew %d strokes, want the one", len(spy.strokes))
	}
	got := spy.strokes[0]
	if got.r != (painter.Rect{X: want.X, Y: want.Y, W: want.W, H: want.H}) {
		t.Errorf("the border is at %+v, want the bounds %+v", got.r, want)
	}
	if got.c != RGB(0xFF, 0x8C, 0x1A) {
		t.Errorf("the border is %v, want the ink it was given", got.c)
	}
	if got.w != DefaultSelectionWeight {
		t.Errorf("the border is %d thick, want the default %d", got.w, DefaultSelectionWeight)
	}
}

// TestAnUnsetInkTakesTheThemesAccent, which is what a selection is in the
// theme's own terms.
func TestAnUnsetInkTakesTheThemesAccent(t *testing.T) {
	for name, theme := range map[string]*Theme{
		"dark":  DefaultDark(),
		"light": DefaultLight(),
	} {
		b := &SelectionBox{}
		b.SetBounds(Rect{W: 100, H: 100})
		spy := &strokeSpy{}
		b.Draw(spy, theme)
		if len(spy.strokes) != 1 || spy.strokes[0].c != theme.Accent {
			t.Errorf("%s: the border is %v, want the accent %v",
				name, spy.strokes[0].c, theme.Accent)
		}
	}
	// A transparent ink is "unset" and not "invisible": a see-through selection
	// is not a thing to want, and treating it as one would hide the border for
	// a caller who simply left the field alone.
	b := &SelectionBox{Ink: RGBA{R: 0xFF, A: 0}}
	b.SetBounds(Rect{W: 100, H: 100})
	spy := &strokeSpy{}
	b.Draw(spy, DefaultDark())
	if spy.strokes[0].c != DefaultDark().Accent {
		t.Errorf("a transparent ink gave %v", spy.strokes[0].c)
	}
}

// TestTheWeightScalesWithTheMetricScale.
//
// Logical pixels, like every other border here. A thickness in device pixels is
// a border that reads as a hairline on the next display.
func TestTheWeightScalesWithTheMetricScale(t *testing.T) {
	was := MetricScale()
	defer SetMetricScale(was)

	b := &SelectionBox{Weight: 3}
	b.SetBounds(Rect{W: 400, H: 400})

	SetMetricScale(1)
	spy := &strokeSpy{}
	b.Draw(spy, DefaultDark())
	one := spy.strokes[0].w

	SetMetricScale(2)
	spy = &strokeSpy{}
	b.Draw(spy, DefaultDark())
	two := spy.strokes[0].w

	if one != 3 {
		t.Errorf("at scale 1 the border is %d, want 3", one)
	}
	if two <= one {
		t.Errorf("at scale 2 the border is %d, no thicker than the %d at scale 1", two, one)
	}

	// And a scale small enough to round a border away leaves one pixel. A
	// border of nothing is a border that disappeared, which on a selection is
	// the difference between knowing what Enter does and guessing.
	SetMetricScale(0.05)
	spy = &strokeSpy{}
	b.Draw(spy, DefaultDark())
	if got := spy.strokes[0].w; got != 1 {
		t.Errorf("at a scale that rounds it away the border is %d, want 1", got)
	}
}

// TestABorderNeverFillsWhatItMarks.
//
// Past half the shorter side the two opposite borders meet and the outline
// becomes a solid block, hiding the very thing it was pointing at.
func TestABorderNeverFillsWhatItMarks(t *testing.T) {
	for _, tc := range []struct{ w, h, weight, want int }{
		{100, 100, 4, 4},
		{20, 20, 40, 10}, // clamped to half
		{20, 4, 40, 2},   // the SHORTER side decides
		{3, 3, 100, 1},   // and never below one
		{1, 1, 100, 1},
	} {
		b := &SelectionBox{Weight: tc.weight}
		b.SetBounds(Rect{W: tc.w, H: tc.h})
		spy := &strokeSpy{}
		b.Draw(spy, DefaultDark())
		if len(spy.strokes) != 1 {
			t.Fatalf("%dx%d weight %d drew %d strokes", tc.w, tc.h, tc.weight, len(spy.strokes))
		}
		if got := spy.strokes[0].w; got != tc.want {
			t.Errorf("%dx%d weight %d gave %d, want %d", tc.w, tc.h, tc.weight, got, tc.want)
		}
	}
}

// TestABoxWithNoRoomDrawsNothing: a widget given an empty rectangle must not
// paint, or a hidden one leaves a border where it used to be.
func TestABoxWithNoRoomDrawsNothing(t *testing.T) {
	for _, r := range []Rect{{}, {W: 0, H: 50}, {W: 50, H: 0}, {W: -5, H: 5}} {
		b := NewSelectionBox(RGB(1, 2, 3))
		b.SetBounds(r)
		spy := &strokeSpy{}
		b.Draw(spy, DefaultDark())
		if len(spy.strokes) != 0 {
			t.Errorf("bounds %+v drew %d strokes", r, len(spy.strokes))
		}
	}
}

// TestWhatAScreenReaderIsToldAboutASelection.
//
// A border has no name of its own: what is selected is whatever the host drew
// underneath, which may not be a widget at all. So one with nothing to say is
// presentation and is skipped, rather than announced as an anonymous rectangle.
func TestWhatAScreenReaderIsToldAboutASelection(t *testing.T) {
	quiet := NewSelectionBox(RGB(1, 2, 3))
	if got := quiet.A11y().Role; got != RolePresentation {
		t.Errorf("an unlabelled selection reports %q, want %q", got, RolePresentation)
	}
	if got := CollectA11y([]Widget{quiet}); len(got) != 0 {
		t.Errorf("it was collected anyway: %v", got)
	}

	named := NewSelectionBox(RGB(1, 2, 3))
	named.Label = "screen 3 of 6"
	info := named.A11y()
	if info.Role != RoleText || info.Name != "screen 3 of 6" {
		t.Errorf("a labelled selection reports %+v", info)
	}
	if got := CollectA11y([]Widget{named}); len(got) != 1 || got[0].Name != "screen 3 of 6" {
		t.Errorf("CollectA11y gave %v", got)
	}
}
