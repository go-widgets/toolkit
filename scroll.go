// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ScrollView is a viewport over a child widget whose content may be
// larger than the visible area. The child's own Bounds is logical
// (= content size); ScrollView paints the child clipped to its own
// Bounds, with origin shifted by -OffsetX/-OffsetY.
//
// A thin scrollbar track (8 px wide) is painted on the right edge in
// Theme.SurfaceAlt; a Theme.Accent thumb sized proportionally to the
// viewport/content ratio shows the current scroll position.
//
// Wheel events scroll vertically; horizontal scrolling is supported
// via direct Scroll(dx, dy) calls (no horizontal scrollbar drawn in
// v0.2).
type ScrollView struct {
	Base
	Child            Widget
	OffsetX, OffsetY int
	contentW         int
	contentH         int
}

// scrollbarWidth is the pixel width of the right-edge scrollbar track.
const scrollbarWidth = 8

// NewScrollView builds a ScrollView around child. Call SetContentSize
// after construction to declare the child's logical extent so the
// thumb is sized correctly + scrolling is clamped.
func NewScrollView(child Widget) *ScrollView {
	return &ScrollView{Child: child}
}

// SetContentSize tells the ScrollView how big the child's logical
// drawing area is. Used by Scroll() to clamp + by Draw() to size
// the thumb. Caller is responsible for invoking this when the child
// grows / shrinks.
func (s *ScrollView) SetContentSize(w, h int) {
	s.contentW = w
	s.contentH = h
}

// Scroll mutates the offsets by (dx, dy) and clamps to
// [0, contentSize - viewportSize] so the thumb never falls off the
// track. Negative offsets are clamped to 0.
func (s *ScrollView) Scroll(dx, dy int) {
	s.OffsetX += dx
	s.OffsetY += dy
	r := s.Bounds()
	maxX := s.contentW - (r.W - scrollbarWidth)
	if maxX < 0 {
		maxX = 0
	}
	maxY := s.contentH - r.H
	if maxY < 0 {
		maxY = 0
	}
	if s.OffsetX < 0 {
		s.OffsetX = 0
	}
	if s.OffsetX > maxX {
		s.OffsetX = maxX
	}
	if s.OffsetY < 0 {
		s.OffsetY = 0
	}
	if s.OffsetY > maxY {
		s.OffsetY = maxY
	}
}

// Draw paints the child clipped to the viewport, then the scrollbar
// track + thumb on the right edge.
func (s *ScrollView) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	// Child viewport excludes the scrollbar column on the right.
	if s.Child != nil {
		cb := s.Child.Bounds()
		s.Child.SetBounds(Rect{
			X: r.X - s.OffsetX,
			Y: r.Y - s.OffsetY,
			W: cb.W,
			H: cb.H,
		})
		s.Child.Draw(p, theme)
		s.Child.SetBounds(cb)
	}
	// Scrollbar track.
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, r.H, theme.SurfaceAlt)
	// Thumb sized to viewport/content ratio, positioned by OffsetY.
	if s.contentH > r.H && r.H > 0 {
		thumbH := r.H * r.H / s.contentH
		if thumbH < 8 {
			thumbH = 8
		}
		thumbY := r.Y
		if s.contentH-r.H > 0 {
			thumbY += s.OffsetY * (r.H - thumbH) / (s.contentH - r.H)
		}
		fillRect(p, trackX, thumbY, scrollbarWidth, thumbH, theme.Accent)
	}
}

// HitTest covers the full bounds (the scrollbar is interactive too).
func (s *ScrollView) HitTest(px, py int) bool { return s.Bounds().Contains(px, py) }
