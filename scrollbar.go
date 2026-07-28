// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// scrollbarMinThumb is the smallest thumb length (px) so it stays grabbable even
// when the content dwarfs the viewport.
const scrollbarMinThumb = 24

// Scrollbar is a slim position indicator for scrollable content: a rounded track
// with a thumb sized to Viewport/Total and positioned by Offset, showing where
// the view sits within the whole. Vertical by default; set Horizontal for a
// bottom scrollbar. When everything fits (Total <= Viewport) the thumb fills the
// track.
type Scrollbar struct {
	Base
	Total      int  // total content length along the scroll axis
	Viewport   int  // visible length
	Offset     int  // scroll offset; clamped to [0, Total-Viewport]
	Horizontal bool // false = vertical (the default for a scrollbar)
}

// NewScrollbar builds an empty vertical scrollbar.
func NewScrollbar() *Scrollbar { return &Scrollbar{} }

// HitTest returns false: the scrollbar is a passive indicator (the app already
// owns wheel/drag scrolling). Compose with a drag handler to make it grabbable.
func (s *Scrollbar) HitTest(_, _ int) bool { return false }

// ThumbRect returns the thumb's rectangle for the current Total/Viewport/Offset,
// so callers can hit-test or animate it. It is empty when the widget has no area.
func (s *Scrollbar) ThumbRect() Rect {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return Rect{}
	}
	total, view := s.Total, s.Viewport
	if total < 1 {
		total = 1
	}
	if view < 1 {
		view = 1
	}
	if view > total {
		view = total
	}
	maxOff := total - view
	off := s.Offset
	if off < 0 {
		off = 0
	}
	if off > maxOff {
		off = maxOff
	}
	length := r.H
	if s.Horizontal {
		length = r.W
	}
	thumb := length * view / total
	if thumb < scrollbarMinThumb {
		thumb = scrollbarMinThumb
	}
	if thumb > length {
		thumb = length
	}
	pos := 0
	if maxOff > 0 {
		pos = (length - thumb) * off / maxOff
	}
	if s.Horizontal {
		return Rect{X: r.X + pos, Y: r.Y, W: thumb, H: r.H}
	}
	return Rect{X: r.X, Y: r.Y + pos, W: r.W, H: thumb}
}

// Draw paints the track and the thumb. The thumb is drawn in Theme.Border so it
// reads against the SurfaceAlt track in both light and dark themes.
func (s *Scrollbar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	radius := r.W / 2
	if r.H < r.W {
		radius = r.H / 2
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, radius, theme.SurfaceAlt)
	t := s.ThumbRect()
	fillRoundRect(p, t.X, t.Y, t.W, t.H, radius, theme.Border)
}
