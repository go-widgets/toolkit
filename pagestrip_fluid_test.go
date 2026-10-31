// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func fluidStrip(n, w int) *PageStrip {
	s := NewPageStrip(n)
	s.Fluid = true
	s.SetBounds(Rect{X: 3, Y: 4, W: w, H: 400})
	return s
}

// A Fluid card is the strip's width less the padding, the number and the bar,
// within [MinThumbSize, MaxThumbSize], and keeps each page's aspect ratio.
func TestFluidCardsFollowTheStripWidth(t *testing.T) {
	s := fluidStrip(50, 160)
	s.SetAspects(func() [][2]int {
		a := make([][2]int, 50)
		for i := range a {
			a[i] = [2]int{16, 9}
		}
		a[1] = [2]int{4, 3}
		a[2] = [2]int{1, 2}
		return a
	}())
	slack := 2*scaled(stripPad) + scrollbarTrack() + s.numW()
	prev := 0
	for _, w := range []int{120, 160, 260, 400} {
		s.SetBounds(Rect{X: 3, Y: 4, W: w, H: 400})
		c0 := s.CardRect(0)
		want := w - slack
		if lo := scaled(48); want < lo {
			want = lo
		}
		if c0.W != want {
			t.Fatalf("strip %d: card %d wide, want %d", w, c0.W, want)
		}
		if c0.W <= prev {
			t.Fatalf("strip %d: card %d did not grow from %d", w, c0.W, prev)
		}
		prev = c0.W
		if h := s.CardRect(0).H; h != c0.W*9/16 {
			t.Fatalf("16:9 card %d x %d", c0.W, h)
		}
		if c1 := s.CardRect(1); c1.H != c1.W*3/4 {
			t.Fatalf("4:3 card %v", c1)
		}
		if c2 := s.CardRect(2); c2.H != c2.W*2 {
			t.Fatalf("1:2 card %v", c2)
		}
	}
	// Max: a very wide strip stops at MaxThumbSize.
	s.MaxThumbSize = 200
	s.SetBounds(Rect{X: 3, Y: 4, W: 900, H: 400})
	if got := s.CardRect(0).W; got != scaled(200) {
		t.Fatalf("max: card %d, want %d", got, scaled(200))
	}
	// Min: a narrow strip keeps MinThumbSize but never wider than the strip lets it show.
	s.MinThumbSize = 90
	s.SetBounds(Rect{X: 3, Y: 4, W: slack + 20, H: 400})
	if got := s.CardRect(0).W; got != 20 {
		t.Fatalf("narrower than the minimum: card %d, want the 20 available", got)
	}
	// Horizontal strips follow the height.
	h := NewPageStrip(30)
	h.Fluid, h.Horizontal = true, true
	h.SetBounds(Rect{W: 400, H: 100})
	a := h.CardRect(0).H
	h.SetBounds(Rect{W: 400, H: 160})
	if b := h.CardRect(0).H; b <= a {
		t.Fatalf("horizontal card height %d did not follow the strip (was %d)", b, a)
	}
	// Not Fluid: the card stays ThumbSize in a wider strip (unchanged behaviour).
	f := NewPageStrip(5)
	f.SetBounds(Rect{W: 600, H: 400})
	if f.CardRect(0).W != f.thumb() {
		t.Fatal("a non-Fluid card must stay ThumbSize")
	}
}

// Resizing a scrolled strip keeps the first visible page in view.
func TestFluidResizeKeepsTheFirstVisiblePage(t *testing.T) {
	s := fluidStrip(200, 160)
	s.Scroll().Set(1 << 20)
	s.Current().Set(-1)
	s.ScrollTo(100)
	buf := makeSurface(460, 420)
	s.Draw(newP(buf, 460), DefaultLight())
	f0, l0 := s.Visible()
	if f0 > 100 || l0 < 100 {
		t.Fatalf("page 100 not in view: [%d,%d]", f0, l0)
	}
	for _, w := range []int{300, 120, 440} {
		s.SetBounds(Rect{X: 3, Y: 4, W: w, H: 400})
		s.Draw(newP(buf, 460), DefaultLight())
		f, l := s.Visible()
		if d := f - f0; d < -2 || d > 2 {
			t.Fatalf("strip %d: first visible page went from %d to %d", w, f0, f)
		}
		if l < f {
			t.Fatalf("strip %d: empty window", w)
		}
	}
}

// A drag through many widths asks the host again only when the card changed by a
// quarter, and only for the pages in (or next to) the window.
func TestFluidResizeAsksSparinglyAndOnlyForTheWindow(t *testing.T) {
	s := fluidStrip(500, 160)
	asked := map[int]int{}
	s.NeedImage = func(i int) { asked[i]++ }
	buf := makeSurface(520, 420)
	p := newP(buf, 520)
	th := DefaultDark()
	s.Draw(p, th)
	first := len(asked)
	if first == 0 || first > 14 {
		t.Fatalf("first draw asked %d pages", first)
	}
	total := func() int {
		n := 0
		for _, v := range asked {
			n += v
		}
		return n
	}
	t0 := total()
	// small steps: 160 -> 175, well under a quarter of the card
	for w := 160; w <= 175; w++ {
		s.SetBounds(Rect{X: 3, Y: 4, W: w, H: 400})
		s.Draw(p, th)
	}
	if total() != t0 {
		t.Fatalf("a %d px change re-asked %d pages", 15, total()-t0)
	}
	// a drag from 175 to 400 in single pixels: each ask follows a quarter, never each pixel
	for w := 175; w <= 400; w++ {
		s.SetBounds(Rect{X: 3, Y: 4, W: w, H: 400})
		s.Draw(p, th)
	}
	f, l := s.Visible()
	win := l - f + 1 + 2*stripPrefetch
	// one round of asks per quarter step (about log1.25(400/160) = 4, plus the
	// pages entering the window), against 225 widths x the window if every pixel asked
	if asks := total() - t0; asks == 0 || asks > 60 {
		t.Fatalf("a drag over 225 widths asked %d times for a window of %d pages", asks, win)
	}
	for i := range asked {
		if i > l+stripPrefetch+20 { // pages far below the window are never asked
			t.Fatalf("page %d (window ends at %d) was asked for", i, l)
		}
	}
	// a bitmap in the page cache stays shown until its replacement arrives
	s.SetImage(0, solid(16, 9, RGB(9, 9, 9)), 16, 9)
	s.SetBounds(Rect{X: 3, Y: 4, W: 160, H: 400})
	s.Draw(p, th)
	if s.imgs[0] == nil {
		t.Fatal("a resize dropped a bitmap")
	}
}
