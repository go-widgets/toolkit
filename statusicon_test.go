// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// solidBuf builds a w*h opaque RGBA source of one colour so an image draw can
// be detected by any painted pixel.
func solidBuf(w, h int, r, g, b byte) []byte {
	buf := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		buf[i*4], buf[i*4+1], buf[i*4+2], buf[i*4+3] = r, g, b, 0xFF
	}
	return buf
}

// anyPainted reports whether any pixel in a w-wide RGBA buffer has alpha != 0.
func anyAlpha(buf []byte) bool {
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0 {
			return true
		}
	}
	return false
}

func TestStatusIconAutoSizeAndVectorDraw(t *testing.T) {
	const w, h = 30, 30
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	var gotInk RGBA
	var gotRect Rect
	icon := func(_ painter.Painter, r Rect, ink RGBA) { gotInk, gotRect = ink, r }
	s := NewStatusIcon(icon)
	s.Draw(p, th)
	// Auto-sized to the default square.
	if b := s.Bounds(); b.W != StatusIconSize || b.H != StatusIconSize {
		t.Fatalf("auto-size = %+v, want %d square", b, StatusIconSize)
	}
	// Icon received the whole cell + the theme ink (Ink unset).
	if gotRect != s.Bounds() {
		t.Fatalf("icon rect = %+v, want %+v", gotRect, s.Bounds())
	}
	if gotInk != th.OnSurface {
		t.Fatalf("icon ink = %+v, want theme OnSurface", gotInk)
	}
}

func TestStatusIconInkOverrideAndPresetHeight(t *testing.T) {
	const w, h = 30, 30
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	var gotInk RGBA
	s := NewStatusIcon(func(_ painter.Painter, _ Rect, ink RGBA) { gotInk = ink })
	s.Ink = RGB(0x11, 0x22, 0x33)
	// W==0 but H preset: auto-size must keep H.
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 20})
	s.Draw(p, th)
	if b := s.Bounds(); b.W != StatusIconSize || b.H != 20 {
		t.Fatalf("bounds = %+v, want W=%d H=20", b, StatusIconSize)
	}
	if gotInk != (RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}) {
		t.Fatalf("ink override not honoured: %+v", gotInk)
	}
}

func TestStatusIconImageBeatsIconAndBadge(t *testing.T) {
	const w, h = 24, 24
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	iconCalled := false
	s := NewStatusIconImage(solidBuf(2, 2, 0x40, 0x80, 0xC0), 2, 2)
	s.Icon = func(_ painter.Painter, _ Rect, _ RGBA) { iconCalled = true }
	s.Badge = NewBadge("3")
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	s.Draw(p, th)
	if iconCalled {
		t.Fatal("Icon called even though a valid image is present")
	}
	if !anyAlpha(buf) {
		t.Fatal("image drew nothing")
	}
	// Badge auto-sized + anchored to the top-right of the cell.
	b := s.Badge.Bounds()
	if b.W == 0 || b.H == 0 {
		t.Fatalf("badge not sized: %+v", b)
	}
	if b.X != 20-b.W || b.Y != 0 {
		t.Fatalf("badge not top-right: %+v (cell right=20)", b)
	}
}

func TestStatusIconNoIconNoImageJustPaintsBadge(t *testing.T) {
	const w, h = 24, 24
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	s := &StatusIcon{Badge: NewBadge("9")} // no Icon, no Pixels
	s.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	s.Draw(p, th) // exercises the neither-image-nor-icon path
	if !anyAlpha(buf) {
		t.Fatal("badge should still paint")
	}
}

func TestStatusIconInvalidImageFallsToIcon(t *testing.T) {
	const w, h = 24, 24
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	iconCalled := false
	// Pixels too short for IW*IH*4 => hasImage false => Icon path.
	s := &StatusIcon{Pixels: make([]byte, 3), IW: 2, IH: 2,
		Icon: func(_ painter.Painter, _ Rect, _ RGBA) { iconCalled = true }}
	s.SetBounds(Rect{X: 0, Y: 0, W: 18, H: 18})
	s.Draw(p, th)
	if !iconCalled {
		t.Fatal("invalid image should fall through to Icon")
	}
}

func TestStatusIconClicks(t *testing.T) {
	left, right := 0, 0
	s := NewStatusIcon(nil)
	s.OnClick = func() { left++ }
	s.OnRightClick = func() { right++ }
	s.OnEvent(Event{Kind: EventClick})                            // primary
	s.OnEvent(Event{Kind: EventClick, Code: StatusIconSecondary}) // secondary
	s.OnEvent(Event{Kind: EventKeyDown})                          // ignored
	if left != 1 || right != 1 {
		t.Fatalf("clicks = left %d right %d, want 1/1", left, right)
	}
}

func TestStatusIconNilCallbacksSafe(t *testing.T) {
	s := NewStatusIcon(nil) // OnClick + OnRightClick nil
	s.OnEvent(Event{Kind: EventClick})
	s.OnEvent(Event{Kind: EventClick, Code: StatusIconSecondary})
	// no panic == pass
}

func TestStatusAreaLayoutDefaultsAndRouting(t *testing.T) {
	const w, h = 100, 30
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	th := DefaultLight()
	c0, c1 := 0, 0
	i0 := NewStatusIcon(DrawIconSettings)
	i0.OnClick = func() { c0++ }
	i1 := NewStatusIcon(DrawIconSearch)
	i1.OnClick = func() { c1++ }
	a := NewStatusArea(i0)
	a.Add(i1) // Add re-flows
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	// Defaults: StatusIconSize cells, StatusAreaGap between, vertically centred.
	wantY := (h - StatusIconSize) / 2
	if b := i0.Bounds(); b != (Rect{X: 0, Y: wantY, W: StatusIconSize, H: StatusIconSize}) {
		t.Fatalf("icon0 = %+v", b)
	}
	if b := i1.Bounds(); b.X != StatusIconSize+StatusAreaGap {
		t.Fatalf("icon1 X = %d, want %d", b.X, StatusIconSize+StatusAreaGap)
	}
	a.Draw(p, th)
	if !anyAlpha(buf) {
		t.Fatal("status area drew nothing")
	}
	// Route a click onto icon1's cell (surface coords translated by OnEvent).
	a.OnEvent(Event{Kind: EventClick, X: i1.Bounds().X + 2, Y: wantY + 2})
	if c0 != 0 || c1 != 1 {
		t.Fatalf("routing: c0=%d c1=%d, want 0/1", c0, c1)
	}
	// A click in the gap (no cell) is dropped.
	a.OnEvent(Event{Kind: EventClick, X: StatusIconSize + 1, Y: 0})
	if c0 != 0 || c1 != 1 {
		t.Fatalf("gap click should be dropped: c0=%d c1=%d", c0, c1)
	}
}

func TestStatusAreaGapAndSizeOverrides(t *testing.T) {
	i0 := NewStatusIcon(nil)
	i1 := NewStatusIcon(nil)
	// Negative gap clamps to 0 (flush); IconSize override.
	a := &StatusArea{Icons: []*StatusIcon{i0, i1}, Gap: -5, IconSize: 22}
	a.SetBounds(Rect{X: 3, Y: 0, W: 100, H: 22})
	if b := i0.Bounds(); b != (Rect{X: 3, Y: 0, W: 22, H: 22}) {
		t.Fatalf("icon0 = %+v", b)
	}
	if b := i1.Bounds(); b.X != 3+22 { // gap clamped to 0
		t.Fatalf("icon1 X = %d, want %d (gap 0)", b.X, 3+22)
	}
	// Positive explicit gap.
	a2 := &StatusArea{Icons: []*StatusIcon{i0, i1}, Gap: 7}
	a2.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	if b := i1.Bounds(); b.X != StatusIconSize+7 {
		t.Fatalf("icon1 X = %d, want %d", b.X, StatusIconSize+7)
	}
}

func TestStatusAreaEmptyRoutingNoop(t *testing.T) {
	a := NewStatusArea() // no icons
	a.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 20})
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // must not panic
}

// TestStatusAreaBackgroundPlate covers the optional bar plate: a zero-alpha
// Background leaves a corner pixel at the sentinel (transparent, icons only),
// while a non-zero Background fills the whole area behind the icons.
func TestStatusAreaBackgroundPlate(t *testing.T) {
	const w, h = 60, 20
	plate := RGB(0x30, 0x30, 0x40)

	// (a) Default (zero Background): the top-left corner — clear of any icon
	// glyph — keeps the makeSurface sentinel, proving nothing painted there.
	off := NewStatusArea(NewStatusIcon(DrawIconSettings))
	off.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	offBuf := makeSurface(w, h)
	off.Draw(newP(offBuf, w), DefaultLight())
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	if got := pixelAt(offBuf, w, w-1, h-1); got != sentinel {
		t.Fatalf("transparent StatusArea painted the far corner: %+v", got)
	}

	// (b) With a Background, that same corner is the plate colour.
	on := NewStatusArea(NewStatusIcon(DrawIconSettings))
	on.Background = plate
	on.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	onBuf := makeSurface(w, h)
	on.Draw(newP(onBuf, w), DefaultLight())
	if got := pixelAt(onBuf, w, w-1, h-1); got != plate {
		t.Fatalf("plate corner = %+v, want %+v", got, plate)
	}
}
