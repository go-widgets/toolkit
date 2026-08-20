// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Carousel shows one child Widget (a "slide") at a time from Slides,
// picked by Current. A gutter on each side hosts a ◂ / ▸ arrow
// affordance for stepping to the previous / next slide, and a row of
// dot indicators below the content marks the total slide count + the
// active one. Navigation wraps around the ends when Wrap is set;
// otherwise it clamps.
//
// Suitable for image galleries, onboarding panels, or featured-content
// rotators — anywhere a single "card" from a set is shown with an
// obvious way to step through the rest.
type Carousel struct {
	Base
	focusState
	// Slides + Wrap are config. The reactive slide index is MVVM-only: it lives
	// in an unexported Observable exposed via [Carousel.Current].
	Slides []Widget
	Wrap   bool

	current *mvvm.Observable[int]
}

// Current is the shown slide index as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Current field. A
// gutter-arrow step (Prev/Next), a dot-indicator click or an arrow key Sets it;
// subscribers are notified on change (an unchanged value is a no-op, so
// re-selecting the shown slide is silent).
func (c *Carousel) Current() *mvvm.Observable[int] {
	if c.current == nil {
		c.current = mvvm.NewObservable(0)
	}
	return c.current
}

// Carousel layout constants. carouselGutterW is the pixel width of each
// side arrow gutter; carouselDotsH is the height of the bottom
// dot-indicator strip; carouselDotSize / carouselDotGap size + space the
// dots within that strip.
const (
	carouselGutterW = 24
	carouselDotsH   = 16
	carouselDotSize = 8
	carouselDotGap  = 6
	// carouselDotStride is the horizontal step between successive dot
	// origins (the dot itself plus the gap to the next one).
	carouselDotStride = carouselDotSize + carouselDotGap
)

// NewCarousel builds a Carousel over slides, starting at Current = 0
// with Wrap = false (clamp at the ends).
func NewCarousel(slides []Widget) *Carousel {
	c := &Carousel{Slides: slides}
	c.current = mvvm.NewObservable(0)
	return c
}

// clampCurrent brings Current into [0, len(Slides)-1]. Called before Next /
// Prev step relative to it, so a Current left stale by an external mutation
// of Slides (e.g. the slice shrank) doesn't index out of range.
func (c *Carousel) clampCurrent() {
	n := len(c.Slides)
	cur := c.Current().Get()
	if cur < 0 {
		c.Current().Set(0)
	} else if cur >= n {
		c.Current().Set(n - 1)
	}
}

// currentSlide returns the widget at Current, or nil when Slides is empty
// or Current (defensively re-checked here too) falls outside it.
func (c *Carousel) currentSlide() Widget {
	cur := c.Current().Get()
	if cur < 0 || cur >= len(c.Slides) {
		return nil
	}
	return c.Slides[cur]
}

// Next advances Current by one slide. At the last slide it wraps to the
// first when Wrap is set, otherwise it stays put (clamped). A no-op when
// Slides is empty.
func (c *Carousel) Next() {
	n := len(c.Slides)
	if n == 0 {
		return
	}
	c.clampCurrent()
	cur := c.Current().Get()
	if cur == n-1 {
		if c.Wrap {
			c.setCurrent(0)
		}
		return
	}
	c.setCurrent(cur + 1)
}

// setCurrent moves the shown slide to i by Setting the Current Observable -- the
// shared mutate path Prev/Next, a dot click and the arrow keys all funnel
// through. Subscribers are notified on change; re-selecting the shown slide is a
// no-op (per mvvm.Observable), keeping a two-way binding loop-free.
func (c *Carousel) setCurrent(i int) {
	c.Current().Set(i)
}

// Prev retreats Current by one slide. At the first slide it wraps to the
// last when Wrap is set, otherwise it stays put (clamped). A no-op when
// Slides is empty.
func (c *Carousel) Prev() {
	n := len(c.Slides)
	if n == 0 {
		return
	}
	c.clampCurrent()
	cur := c.Current().Get()
	if cur == 0 {
		if c.Wrap {
			c.setCurrent(n - 1)
		}
		return
	}
	c.setCurrent(cur - 1)
}

// contentRect is the slide viewport: the bounds minus the two side gutters
// and the bottom dots strip.
func (c *Carousel) contentRect() Rect {
	r := c.Bounds()
	return Rect{X: r.X + carouselGutterW, Y: r.Y, W: r.W - 2*carouselGutterW, H: r.H - carouselDotsH}
}

// leftGutter / rightGutter are the arrow-affordance strips flanking the
// content, each spanning the height above the dots strip.
func (c *Carousel) leftGutter() Rect {
	r := c.Bounds()
	return Rect{X: r.X, Y: r.Y, W: carouselGutterW, H: r.H - carouselDotsH}
}

func (c *Carousel) rightGutter() Rect {
	r := c.Bounds()
	return Rect{X: r.X + r.W - carouselGutterW, Y: r.Y, W: carouselGutterW, H: r.H - carouselDotsH}
}

// dotsRect is the bottom strip housing the page indicators.
func (c *Carousel) dotsRect() Rect {
	r := c.Bounds()
	return Rect{X: r.X, Y: r.Y + r.H - carouselDotsH, W: r.W, H: carouselDotsH}
}

// dotsStartX is the left edge of the first dot such that the full row of
// len(Slides) dots is horizontally centred within the dots strip. Only
// called (via dotRect) with a non-empty Slides -- both call sites (drawDots,
// dotAt) are themselves gated on len(Slides) > 0 by their callers.
func (c *Carousel) dotsStartX() int {
	r := c.dotsRect()
	n := len(c.Slides)
	rowW := n*carouselDotSize + (n-1)*carouselDotGap
	return r.X + (r.W-rowW)/2
}

// dotRect is the i-th dot's paint/hit rect, in surface coordinates.
func (c *Carousel) dotRect(i int) Rect {
	r := c.dotsRect()
	x := c.dotsStartX() + i*carouselDotStride
	y := r.Y + (r.H-carouselDotSize)/2
	return Rect{X: x, Y: y, W: carouselDotSize, H: carouselDotSize}
}

// dotAt returns the dot index at a surface point, or -1 when none matches.
func (c *Carousel) dotAt(px, py int) int {
	for i := range c.Slides {
		if c.dotRect(i).Contains(px, py) {
			return i
		}
	}
	return -1
}

// drawArrow paints a solid ◂ (left) / ▸ (right) triangle centred in gutter.
// Built from stacked 1-px-tall fillRect rows growing from a 1-px tip to a
// 9-px base -- the same technique Expander uses for its chevron -- so it
// needs no trig / anti-aliasing. ink is dimmed to theme.Border when enabled
// is false (mirrors Pagination's disabled step-button tone).
func (c *Carousel) drawArrow(p painter.Painter, theme *Theme, gutter Rect, left, enabled bool) {
	ink := theme.OnSurface
	if !enabled {
		ink = theme.Border
	}
	cx := gutter.X + gutter.W/2
	cy := gutter.Y + gutter.H/2
	if left {
		// ◂ : tip at cx-2 (outer edge), flat base column at cx+2 (inner edge).
		for t := 0; t < 5; t++ {
			fillRect(p, cx-2+t, cy-t, 1, 1+2*t, ink)
		}
	} else {
		// ▸ : tip at cx+2 (outer edge), flat base column at cx-2 (inner edge).
		for t := 0; t < 5; t++ {
			fillRect(p, cx+2-t, cy-t, 1, 1+2*t, ink)
		}
	}
}

// drawDots paints the page-indicator row: the Current dot filled Accent,
// every other dot filled SurfaceAlt, each with a thin Border outline.
func (c *Carousel) drawDots(p painter.Painter, theme *Theme) {
	cur := c.Current().Get()
	for i := range c.Slides {
		dr := c.dotRect(i)
		fill := theme.SurfaceAlt
		if i == cur {
			fill = theme.Accent
		}
		fillRect(p, dr.X, dr.Y, dr.W, dr.H, fill)
		strokeRect(p, dr.X, dr.Y, dr.W, dr.H, theme.Border)
	}
}

// Draw paints the Current slide clipped to the content rect, the left/right
// arrow affordances, and the dot indicators. A Carousel with no Slides
// paints nothing.
func (c *Carousel) Draw(p painter.Painter, theme *Theme) {
	if len(c.Slides) == 0 {
		return
	}
	content := c.contentRect()
	if slide := c.currentSlide(); slide != nil {
		// Confine the slide to the content rect so a slide bigger than it (or
		// simply mid-transition content) can't overdraw the gutters or dots strip.
		// Requires a Painter that supports clipping; back-ends that don't fall
		// back to the previous surface-edge-only behaviour (see ScrollView).
		clr, canClip := p.(painter.Clipper)
		if canClip {
			clr.PushClip(content)
		}
		slide.SetBounds(content)
		slide.Draw(p, theme)
		if canClip {
			clr.PopClip()
		}
	}
	n := len(c.Slides)
	cur := c.Current().Get()
	c.drawArrow(p, theme, c.leftGutter(), true, c.Wrap || cur > 0)
	c.drawArrow(p, theme, c.rightGutter(), false, c.Wrap || cur < n-1)
	c.drawDots(p, theme)
	// Focus ring around the whole carousel (paints nothing when unfocused, so an
	// unfocused render is byte-identical).
	c.drawFocusRing(p, theme, c.Bounds())
}

// OnEvent: a click in the left/right gutter steps Prev/Next; a click on dot i
// jumps Current to i; a click inside the content rect forwards to the
// Current slide, translated into its local frame. Non-click events + a
// Carousel with no Slides are no-ops.
func (c *Carousel) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown {
		if c.Disabled().Get() || len(c.Slides) == 0 {
			return
		}
		// Left/Right step to the previous/next slide, reusing Prev/Next (which
		// wrap or clamp per Wrap).
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			c.Prev()
		case "ArrowRight", "ArrowDown":
			c.Next()
		}
		return
	}
	if ev.Kind != EventClick || len(c.Slides) == 0 {
		return
	}
	// ev is Carousel-local; the gutter/dot/content rects above are computed in
	// surface coordinates (same convention as Notebook/Pagination), so hit-test
	// against the surface position of the click.
	r := c.Bounds()
	sx, sy := ev.X+r.X, ev.Y+r.Y
	switch {
	case c.leftGutter().Contains(sx, sy):
		c.Prev()
	case c.rightGutter().Contains(sx, sy):
		c.Next()
	default:
		if idx := c.dotAt(sx, sy); idx >= 0 {
			c.setCurrent(idx)
			return
		}
		content := c.contentRect()
		if !content.Contains(sx, sy) {
			return
		}
		if slide := c.currentSlide(); slide != nil {
			slide.SetBounds(content)
			slide.OnEvent(translateEvent(ev, r, content))
		}
	}
}
