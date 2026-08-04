// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

// --- NewCarousel -----------------------------------------------------------

func TestNewCarouselDefaults(t *testing.T) {
	s1, s2 := NewLabel("a"), NewLabel("b")
	c := NewCarousel([]Widget{s1, s2})
	if len(c.Slides) != 2 || c.Slides[0] != s1 || c.Slides[1] != s2 {
		t.Fatalf("Slides not stored: %+v", c.Slides)
	}
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0", c.Current)
	}
	if c.Wrap {
		t.Fatal("Wrap should default false")
	}
}

// --- Next / Prev -------------------------------------------------------

func TestCarouselNextAdvancesInMiddle(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Next()
	if c.Current != 1 {
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

func TestCarouselPrevRetreatsInMiddle(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = 2
	c.Prev()
	if c.Current != 1 {
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

func TestCarouselNextWrapsAtEnd(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Wrap = true
	c.Current = 2
	c.Next()
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0 (wrapped)", c.Current)
	}
}

func TestCarouselPrevWrapsAtStart(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Wrap = true
	c.Current = 0
	c.Prev()
	if c.Current != 2 {
		t.Fatalf("Current = %d, want 2 (wrapped)", c.Current)
	}
}

func TestCarouselNextClampsAtEndWithoutWrap(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = 2
	c.Next()
	if c.Current != 2 {
		t.Fatalf("Current = %d, want 2 (clamped)", c.Current)
	}
}

func TestCarouselPrevClampsAtStartWithoutWrap(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Prev()
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0 (clamped)", c.Current)
	}
}

func TestCarouselNextEmptySlidesNoOp(t *testing.T) {
	c := NewCarousel(nil)
	c.Next()
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0", c.Current)
	}
}

func TestCarouselPrevEmptySlidesNoOp(t *testing.T) {
	c := NewCarousel(nil)
	c.Prev()
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0", c.Current)
	}
}

// A Current left stale below the valid range (e.g. after external mutation)
// is clamped to 0 before Next steps from there.
func TestCarouselNextClampsStaleNegativeCurrent(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = -5
	c.Next()
	if c.Current != 1 { // clamp to 0, then advance to 1
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

// A Current left stale above the valid range is clamped to the last index
// before Prev steps from there.
func TestCarouselPrevClampsStaleOverflowCurrent(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = 99
	c.Prev()
	if c.Current != 1 { // clamp to 2 (last index), then retreat to 1
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

// --- Draw --------------------------------------------------------------

func TestCarouselDrawEmptySlidesPaintsNothing(t *testing.T) {
	const w, h = 64, 64
	c := NewCarousel(nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 64, H: 64})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), DefaultLight())
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) != sentinel {
				t.Fatalf("empty carousel painted at (%d,%d): %+v", x, y, pixelAt(buf, w, x, y))
			}
		}
	}
}

func TestCarouselDrawClipsSlideToContent(t *testing.T) {
	// A slide bigger than the content rect must not overdraw the gutters or
	// the dots strip -- same regression class ScrollView guards against.
	const w, h = 220, 130
	red := RGBA{R: 255, A: 255}
	slide := &fillWidget{color: red}
	c := NewCarousel([]Widget{slide})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), DefaultLight())

	content := c.contentRect()
	// Inside content: painted red.
	if got := pixelAt(buf, w, content.X+10, content.Y+10); got != red {
		t.Fatalf("inside content not painted: %+v", got)
	}
	// Left gutter: not painted red.
	lg := c.leftGutter()
	if got := pixelAt(buf, w, lg.X+2, lg.Y+2); got == red {
		t.Fatalf("left gutter overdrawn by slide: %+v", got)
	}
	// Right gutter: not painted red.
	rg := c.rightGutter()
	if got := pixelAt(buf, w, rg.X+2, rg.Y+2); got == red {
		t.Fatalf("right gutter overdrawn by slide: %+v", got)
	}
	// Dots strip: not painted red.
	dr := c.dotsRect()
	if got := pixelAt(buf, w, dr.X+2, dr.Y+2); got == red {
		t.Fatalf("dots strip overdrawn by slide: %+v", got)
	}
}

func TestCarouselDrawWithoutClipperFallsBackNoPanic(t *testing.T) {
	const w, h = 220, 130
	slide := &fillWidget{color: RGBA{R: 1, A: 255}}
	c := NewCarousel([]Widget{slide})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(noClipPainter{newP(buf, w)}, DefaultLight())
}

func TestCarouselDrawOutOfRangeCurrentSkipsSlideNoPanic(t *testing.T) {
	const w, h = 220, 130
	theme := DefaultLight()
	slide := &fillWidget{color: RGBA{R: 1, A: 255}}
	c := NewCarousel([]Widget{slide})
	c.Current = 7 // out of range for a single slide
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// The slide must not have painted (nil currentSlide), but arrows/dots
	// still render -- e.g. the dot strip has a dot rect painted.
	dr := c.dotRect(0)
	if pixelAt(buf, w, dr.X+2, dr.Y+2) == (RGBA{R: 1, A: 255}) {
		t.Fatal("out-of-range Current still drew the slide")
	}
}

func TestCarouselDrawDotsAccentOnCurrent(t *testing.T) {
	const w, h = 220, 130
	theme := DefaultLight()
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = 1
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	d0, d1 := c.dotRect(0), c.dotRect(1)
	if got := pixelAt(buf, w, d1.X+2, d1.Y+2); got != theme.Accent {
		t.Fatalf("current dot fill = %+v, want Accent", got)
	}
	if got := pixelAt(buf, w, d0.X+2, d0.Y+2); got != theme.SurfaceAlt {
		t.Fatalf("non-current dot fill = %+v, want SurfaceAlt", got)
	}
}

func TestCarouselDrawArrowsEnabledUseOnSurfaceInk(t *testing.T) {
	const w, h = 220, 130
	theme := DefaultLight()
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Wrap = true // both arrows always enabled regardless of Current
	c.Current = 0
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	lg, rg := c.leftGutter(), c.rightGutter()
	if !scanForColor(buf, w, lg, theme.OnSurface) {
		t.Fatal("enabled left arrow: no OnSurface ink found")
	}
	if !scanForColor(buf, w, rg, theme.OnSurface) {
		t.Fatal("enabled right arrow: no OnSurface ink found")
	}
}

func TestCarouselDrawArrowsDisabledUseBorderInk(t *testing.T) {
	const w, h = 220, 130
	theme := DefaultLight()
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Wrap = false
	c.Current = 0 // left arrow disabled (no wrap, at first slide)
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	lg := c.leftGutter()
	if !scanForColor(buf, w, lg, theme.Border) {
		t.Fatal("disabled left arrow: no Border ink found")
	}

	c.Current = 2 // right arrow disabled (no wrap, at last slide)
	buf2 := makeSurface(w, h)
	c.Draw(newP(buf2, w), theme)
	rg := c.rightGutter()
	if !scanForColor(buf2, w, rg, theme.Border) {
		t.Fatal("disabled right arrow: no Border ink found")
	}
}

// scanForColor reports whether any pixel within r (surface coords) equals c.
func scanForColor(buf []byte, w int, r Rect, c RGBA) bool {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

// --- OnEvent -------------------------------------------------------------

// surfacePointToLocal converts a surface-coordinate point into an event
// local to c (subtracting c.Bounds()' origin), matching the widget-local
// convention OnEvent expects.
func surfacePointToLocal(c *Carousel, x, y int) Event {
	r := c.Bounds()
	return Event{Kind: EventClick, X: x - r.X, Y: y - r.Y}
}

func TestCarouselClickLeftGutterCallsPrev(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.Current = 2
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	lg := c.leftGutter()
	ev := surfacePointToLocal(c, lg.X+2, lg.Y+2)
	c.OnEvent(ev)
	if c.Current != 1 {
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

func TestCarouselClickRightGutterCallsNext(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	rg := c.rightGutter()
	ev := surfacePointToLocal(c, rg.X+2, rg.Y+2)
	c.OnEvent(ev)
	if c.Current != 1 {
		t.Fatalf("Current = %d, want 1", c.Current)
	}
}

func TestCarouselClickDotJumps(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	d2 := c.dotRect(2)
	ev := surfacePointToLocal(c, d2.X+2, d2.Y+2)
	c.OnEvent(ev)
	if c.Current != 2 {
		t.Fatalf("Current = %d, want 2", c.Current)
	}
}

func TestCarouselClickContentForwardsTranslatedCoords(t *testing.T) {
	rw := &recordingWidget{}
	c := NewCarousel([]Widget{rw})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	content := c.contentRect()
	// Pick a surface point inside content, offset from its top-left so the
	// translated local coordinate is verifiable.
	sx, sy := content.X+7, content.Y+3
	ev := surfacePointToLocal(c, sx, sy)
	c.OnEvent(ev)
	if len(rw.events) != 1 {
		t.Fatalf("content click did not reach the slide: events=%v", rw.events)
	}
	if rw.events[0].X != 7 || rw.events[0].Y != 3 {
		t.Fatalf("forwarded local coords = (%d,%d), want (7,3)", rw.events[0].X, rw.events[0].Y)
	}
}

func TestCarouselClickContentOutOfRangeCurrentNoForwardNoPanic(t *testing.T) {
	rw := &recordingWidget{}
	c := NewCarousel([]Widget{rw})
	c.Current = 9 // out of range
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	content := c.contentRect()
	ev := surfacePointToLocal(c, content.X+5, content.Y+5)
	c.OnEvent(ev)
	if len(rw.events) != 0 {
		t.Fatal("out-of-range Current must not forward the click")
	}
}

func TestCarouselClickDeadZoneNoOp(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	// A point in the dots strip between two dots -- not a gutter, not a
	// dot, not the content rect (which sits above the dots strip).
	d0, d1 := c.dotRect(0), c.dotRect(1)
	gapX := d0.X + d0.W + (d1.X-(d0.X+d0.W))/2
	gapY := d0.Y + d0.H/2
	ev := surfacePointToLocal(c, gapX, gapY)
	before := c.Current
	c.OnEvent(ev)
	if c.Current != before {
		t.Fatalf("dead-zone click changed Current: %d -> %d", before, c.Current)
	}
}

func TestCarouselOnEventIgnoresNonClick(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("a"), NewLabel("b"), NewLabel("c")})
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	// Left/Right step slides as of Wave 3; an unrelated key (Tab) must not.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0 (non-click ignored)", c.Current)
	}
}

func TestCarouselOnEventEmptySlidesNoOp(t *testing.T) {
	c := NewCarousel(nil)
	c.SetBounds(Rect{X: 10, Y: 5, W: 200, H: 120})
	c.OnEvent(Event{Kind: EventClick, X: 20, Y: 20})
	if c.Current != 0 {
		t.Fatalf("Current = %d, want 0", c.Current)
	}
}
