// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// mockHBWidget is a minimal Widget for HeaderBar tests. It embeds
// Base (so Bounds/SetBounds/HitTest/OnEvent are inherited) and
// counts Draw invocations so tests can assert both positioning +
// dispatch.
type mockHBWidget struct {
	Base
	draws int
}

func (m *mockHBWidget) Draw(p painter.Painter, theme *Theme) {
	_, _ = p, theme
	m.draws++
}

// topmostInkRow returns the smallest y such that some pixel of that
// row equals target; -1 when no such pixel is found.
func topmostInkRow(buf []byte, w, h int, target RGBA) int {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == target {
				return y
			}
		}
	}
	return -1
}

// --- API surface ---------------------------------------------------------

func TestHeaderBarHeightConstant(t *testing.T) {
	if HeaderBarHeight != 40 {
		t.Fatalf("HeaderBarHeight = %d, want 40", HeaderBarHeight)
	}
}

func TestNewHeaderBarCarriesTitle(t *testing.T) {
	h := NewHeaderBar("Files")
	if h.Title != "Files" {
		t.Fatalf("Title = %q, want %q", h.Title, "Files")
	}
	if h.Subtitle != "" {
		t.Fatalf("Subtitle = %q, want empty", h.Subtitle)
	}
	if len(h.Start) != 0 || len(h.End) != 0 {
		t.Fatalf("NewHeaderBar left Start/End non-empty: %d/%d",
			len(h.Start), len(h.End))
	}
}

// --- Body + border -------------------------------------------------------

// An empty HeaderBar (no title, no subtitle, no children) still
// paints its body (SurfaceAlt) + border (Border stroke). Covers the
// early return inside the single-line branch when Title == "".
func TestHeaderBarEmptyPaintsBodyAndBorder(t *testing.T) {
	const w = 200
	theme := DefaultLight()
	h := NewHeaderBar("")
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	// Interior pixel = SurfaceAlt (body fill).
	if got := pixelAt(buf, w, 50, 20); got != theme.SurfaceAlt {
		t.Fatalf("body pixel = %+v, want SurfaceAlt", got)
	}
	// Top-left corner = Border (stroke).
	if got := pixelAt(buf, w, 0, 0); got != theme.Border {
		t.Fatalf("top-left border = %+v, want Border", got)
	}
	// No OnSurface ink anywhere (no title drawn).
	for y := 0; y < HeaderBarHeight; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				t.Fatalf("unexpected OnSurface ink at (%d,%d)", x, y)
			}
		}
	}
}

// --- Title (single-line) -------------------------------------------------

// Title (no subtitle) is centred horizontally + vertically.
func TestHeaderBarTitleOnlyIsCentred(t *testing.T) {
	const w = 200
	theme := DefaultLight()
	h := NewHeaderBar("HI")
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	// TextWidth("HI") = 12; centred x ≈ (200-12)/2 = 94.
	// Vertically: ty = (40-7)/2 = 16, so ink appears at rows 16..22.
	found := false
	for y := 14; y < 24 && !found; y++ {
		for x := 80; x < 120 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no OnSurface ink found in the expected title centre band")
	}
}

// --- Subtitle (two-line) -------------------------------------------------

// Subtitle non-empty → block layout shifts the title UP. Verify by
// comparing the topmost OnSurface row of a single-line render vs a
// two-line render of the same title.
func TestHeaderBarSubtitleShiftsTitleUp(t *testing.T) {
	const w = 200
	theme := DefaultLight()

	// Single-line render.
	h1 := NewHeaderBar("HI")
	h1.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf1 := makeSurface(w, HeaderBarHeight)
	h1.Draw(newP(buf1, w), theme)
	top1 := topmostInkRow(buf1, w, HeaderBarHeight, theme.OnSurface)

	// Two-line render (same title + a subtitle).
	h2 := NewHeaderBar("HI")
	h2.Subtitle = "sub"
	h2.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf2 := makeSurface(w, HeaderBarHeight)
	h2.Draw(newP(buf2, w), theme)
	top2 := topmostInkRow(buf2, w, HeaderBarHeight, theme.OnSurface)

	if top1 < 0 || top2 < 0 {
		t.Fatalf("no title ink found: top1=%d top2=%d", top1, top2)
	}
	if top2 >= top1 {
		t.Fatalf("subtitle-mode title row (%d) should be above single-line row (%d)",
			top2, top1)
	}

	// Subtitle painted in Border ink INSIDE the bar (not the outer
	// stroke). Look for Border ink in the interior rows/cols.
	sub := false
	for y := 20; y < HeaderBarHeight-2 && !sub; y++ {
		for x := 4; x < w-4 && !sub; x++ {
			if pixelAt(buf2, w, x, y) == theme.Border {
				sub = true
			}
		}
	}
	if !sub {
		t.Fatal("no Border-tone subtitle ink found in the bar's interior")
	}
}

// Subtitle without a Title paints subtitle only; the title branch
// inside the two-line layout is skipped.
func TestHeaderBarSubtitleWithoutTitle(t *testing.T) {
	const w = 200
	theme := DefaultLight()
	h := NewHeaderBar("")
	h.Subtitle = "sub"
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	// Subtitle ink appears inside the bar (Border tone, off the
	// outer stroke).
	sub := false
	for y := 20; y < HeaderBarHeight-2 && !sub; y++ {
		for x := 4; x < w-4 && !sub; x++ {
			if pixelAt(buf, w, x, y) == theme.Border {
				sub = true
			}
		}
	}
	if !sub {
		t.Fatal("subtitle-only header should paint subtitle ink somewhere inside the bar")
	}

	// No OnSurface ink anywhere: Title is empty so its DrawText call
	// must be skipped inside the two-line branch.
	for y := 0; y < HeaderBarHeight; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				t.Fatalf("empty Title should not paint OnSurface ink (found at %d,%d)",
					x, y)
			}
		}
	}
}

// --- Start row -----------------------------------------------------------

// Single Start widget is positioned at bar.X + pad, with its
// original W preserved + H fitted to the bar's inner height. Draw
// is invoked exactly once per child.
func TestHeaderBarStartSingleWidgetPositioning(t *testing.T) {
	const w = 200
	theme := DefaultLight()
	h := NewHeaderBar("T")
	m := &mockHBWidget{}
	m.SetBounds(Rect{W: 30, H: 0}) // original W is preserved
	h.Start = []Widget{m}
	h.SetBounds(Rect{X: 10, Y: 5, W: w - 10, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight+10)
	h.Draw(newP(buf, w), theme)

	if m.draws != 1 {
		t.Fatalf("Start widget Draw count = %d, want 1", m.draws)
	}
	got := m.Bounds()
	wantX := 10 + HeaderBarPad
	wantY := 5 + HeaderBarPad/2
	wantH := HeaderBarHeight - HeaderBarPad
	if got.X != wantX || got.Y != wantY || got.W != 30 || got.H != wantH {
		t.Fatalf("Start bounds = %+v, want (X=%d Y=%d W=30 H=%d)",
			got, wantX, wantY, wantH)
	}
}

// Multiple Start widgets are laid out left-to-right; each child's X
// sits at the previous child's right edge.
func TestHeaderBarStartMultipleLTR(t *testing.T) {
	const w = 300
	theme := DefaultLight()
	h := NewHeaderBar("")
	a := &mockHBWidget{}
	a.SetBounds(Rect{W: 24})
	b := &mockHBWidget{}
	b.SetBounds(Rect{W: 40})
	c := &mockHBWidget{}
	c.SetBounds(Rect{W: 16})
	h.Start = []Widget{a, b, c}
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	if a.Bounds().X != HeaderBarPad {
		t.Fatalf("a.X = %d, want %d", a.Bounds().X, HeaderBarPad)
	}
	if b.Bounds().X != HeaderBarPad+24 {
		t.Fatalf("b.X = %d, want %d", b.Bounds().X, HeaderBarPad+24)
	}
	if c.Bounds().X != HeaderBarPad+24+40 {
		t.Fatalf("c.X = %d, want %d", c.Bounds().X, HeaderBarPad+24+40)
	}
	if a.draws != 1 || b.draws != 1 || c.draws != 1 {
		t.Fatalf("Start children Draw counts = %d/%d/%d, want 1/1/1",
			a.draws, b.draws, c.draws)
	}
}

// --- End row -------------------------------------------------------------

// Multiple End widgets are laid out RIGHT-TO-LEFT: End[0] sits
// against the bar's right edge, End[1] to its left, etc.
func TestHeaderBarEndMultipleRTL(t *testing.T) {
	const w = 300
	theme := DefaultLight()
	h := NewHeaderBar("")
	a := &mockHBWidget{}
	a.SetBounds(Rect{W: 20})
	b := &mockHBWidget{}
	b.SetBounds(Rect{W: 30})
	h.End = []Widget{a, b}
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	// End[0] sits against the right edge.
	if a.Bounds().X != w-HeaderBarPad-20 {
		t.Fatalf("End[0].X = %d, want %d", a.Bounds().X, w-HeaderBarPad-20)
	}
	// End[1] sits to End[0]'s left.
	if b.Bounds().X != w-HeaderBarPad-20-30 {
		t.Fatalf("End[1].X = %d, want %d", b.Bounds().X, w-HeaderBarPad-20-30)
	}
	if a.draws != 1 || b.draws != 1 {
		t.Fatalf("End children Draw counts = %d/%d, want 1/1", a.draws, b.draws)
	}
}

// --- Combined Start + End + Title ----------------------------------------

// With Start + End both populated the title still lands in the
// remaining central strip.
func TestHeaderBarStartAndEndComposition(t *testing.T) {
	const w = 300
	theme := DefaultLight()
	h := NewHeaderBar("HI")
	s := &mockHBWidget{}
	s.SetBounds(Rect{W: 24})
	e := &mockHBWidget{}
	e.SetBounds(Rect{W: 24})
	h.Start = []Widget{s}
	h.End = []Widget{e}
	h.SetBounds(Rect{X: 0, Y: 0, W: w, H: HeaderBarHeight})
	buf := makeSurface(w, HeaderBarHeight)
	h.Draw(newP(buf, w), theme)

	if s.Bounds().X != HeaderBarPad {
		t.Fatalf("Start.X = %d, want %d", s.Bounds().X, HeaderBarPad)
	}
	if e.Bounds().X != w-HeaderBarPad-24 {
		t.Fatalf("End.X = %d, want %d", e.Bounds().X, w-HeaderBarPad-24)
	}
	// Title ink lands somewhere between Start's right edge + End's
	// left edge.
	seen := false
	for y := 0; y < HeaderBarHeight && !seen; y++ {
		for x := HeaderBarPad + 24; x < w-HeaderBarPad-24 && !seen; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				seen = true
			}
		}
	}
	if !seen {
		t.Fatal("title ink missing from the region between Start and End widgets")
	}
}
