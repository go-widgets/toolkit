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
// A thin scrollbar track (8 px) is painted on the right edge, and — when the
// content is wider than the viewport — along the bottom edge too, each in
// Theme.SurfaceAlt with a Theme.Accent thumb sized proportionally to the
// viewport/content ratio. Scroll(dx, dy) moves on both axes.
type ScrollView struct {
	Base
	Child            Widget
	OffsetX, OffsetY int
	contentW         int
	contentH         int
}

// scrollbarWidth is the pixel thickness of a scrollbar track.
const scrollbarWidth = 8

// viewport is the visible content rect: the bounds minus the always-reserved
// right scrollbar column and — when the content overflows horizontally — the
// bottom scrollbar row.
func (s *ScrollView) viewport() Rect {
	r := s.Bounds()
	vw := r.W - scrollbarWidth
	vh := r.H
	if s.contentW > vw {
		vh -= scrollbarWidth
	}
	return Rect{X: r.X, Y: r.Y, W: vw, H: vh}
}

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
	vp := s.viewport()
	maxX := s.contentW - vp.W
	if maxX < 0 {
		maxX = 0
	}
	maxY := s.contentH - vp.H
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
	vp := s.viewport()
	if s.Child != nil {
		// Confine the child to the viewport so content scrolled out of view (or
		// wider than it) can't overdraw neighbours or the scrollbars. Requires a
		// Painter that supports clipping; back-ends that don't fall back to the
		// previous surface-edge-only behaviour. Popped before the scrollbars.
		clr, canClip := p.(painter.Clipper)
		if canClip {
			clr.PushClip(vp)
		}
		cb := s.Child.Bounds()
		s.Child.SetBounds(Rect{X: r.X - s.OffsetX, Y: r.Y - s.OffsetY, W: cb.W, H: cb.H})
		s.Child.Draw(p, theme)
		s.Child.SetBounds(cb)
		if canClip {
			clr.PopClip()
		}
	}
	// Vertical scrollbar (right edge), sized to the viewport height so it leaves
	// the corner for a horizontal bar.
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, vp.H, theme.SurfaceAlt)
	if s.contentH > vp.H && vp.H > 0 {
		thumbH := vp.H * vp.H / s.contentH
		if thumbH < 8 {
			thumbH = 8
		}
		thumbY := r.Y
		if s.contentH-vp.H > 0 {
			thumbY += s.OffsetY * (vp.H - thumbH) / (s.contentH - vp.H)
		}
		fillRect(p, trackX, thumbY, scrollbarWidth, thumbH, theme.Accent)
	}
	// Horizontal scrollbar (bottom edge), only when the content overflows
	// horizontally (which is exactly when the viewport reserved the bottom row).
	if s.contentW > vp.W {
		trackY := r.Y + r.H - scrollbarWidth
		fillRect(p, r.X, trackY, vp.W, scrollbarWidth, theme.SurfaceAlt)
		if vp.W > 0 {
			thumbW := vp.W * vp.W / s.contentW
			if thumbW < 8 {
				thumbW = 8
			}
			thumbX := r.X
			if s.contentW-vp.W > 0 {
				thumbX += s.OffsetX * (vp.W - thumbW) / (s.contentW - vp.W)
			}
			fillRect(p, thumbX, trackY, thumbW, scrollbarWidth, theme.Accent)
		}
	}
}

// HitTest covers the full bounds (the scrollbar is interactive too).
func (s *ScrollView) HitTest(px, py int) bool { return s.Bounds().Contains(px, py) }
