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

	// sbV and sbH track an in-progress drag of the vertical / horizontal
	// scrollbar thumb respectively.
	sbV, sbH scrollDrag
}

// scrollbarWidth is the pixel thickness of a scrollbar track — wide enough that
// the thumb is comfortably grabbable with the mouse.
const scrollbarWidth = 12

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

// vscrollGeom returns the vertical scrollbar's widget-local geometry and whether
// it is live (the content is taller than the viewport). It is the single
// definition of the vertical thumb shared by Draw and OnEvent; the scroll value
// the thumb travel maps to is the pixel OffsetY, clamped to contentH-viewport.
func (s *ScrollView) vscrollGeom() (sbGeom, bool) {
	r := s.Bounds()
	vp := s.viewport()
	if !(s.contentH > vp.H && vp.H > 0) {
		return sbGeom{}, false
	}
	thumbH := vp.H * vp.H / s.contentH
	if thumbH < 8 {
		thumbH = 8
	}
	maxOff := s.contentH - vp.H // > 0 here
	return sbGeom{
		cross0:     r.W - scrollbarWidth,
		trackStart: 0,
		trackLen:   vp.H,
		thumbStart: s.OffsetY * (vp.H - thumbH) / maxOff,
		thumbLen:   thumbH,
		travelNum:  vp.H - thumbH,
		travelDen:  maxOff,
		maxScroll:  maxOff,
	}, true
}

// hscrollGeom returns the horizontal scrollbar's widget-local geometry and
// whether it is live (the content is wider than the viewport). The scroll value
// the thumb travel maps to is the pixel OffsetX, clamped to contentW-viewport.
func (s *ScrollView) hscrollGeom() (sbGeom, bool) {
	r := s.Bounds()
	vp := s.viewport()
	if !(s.contentW > vp.W && vp.W > 0) {
		return sbGeom{}, false
	}
	thumbW := vp.W * vp.W / s.contentW
	if thumbW < 8 {
		thumbW = 8
	}
	maxOff := s.contentW - vp.W // > 0 here
	return sbGeom{
		horizontal: true,
		cross0:     r.H - scrollbarWidth,
		trackStart: 0,
		trackLen:   vp.W,
		thumbStart: s.OffsetX * (vp.W - thumbW) / maxOff,
		thumbLen:   thumbW,
		travelNum:  vp.W - thumbW,
		travelDen:  maxOff,
		maxScroll:  maxOff,
	}, true
}

// OnEvent gives ScrollView native wheel + keyboard scrolling. A ScrollView
// measures its content in pixels rather than rows, so it converts the
// EventScroll Delta (expressed in ROWS) into a pixel offset using its
// effective font's line height — one wheel notch moves one text line. The
// arrow keys scroll a line, Page{Up,Down} a viewport height, and Home / End
// jump to the top / bottom; Scroll() clamps every result. All conversions go
// through Scroll(0, dy) (vertical only — horizontal scrolling stays under the
// host's control via Scroll directly). Any other event kind is ignored, so a
// ScrollView remains a passive viewport for clicks exactly as before.
func (s *ScrollView) OnEvent(ev Event) {
	line := s.glyphHeight()
	switch ev.Kind {
	case EventScroll:
		s.Scroll(0, ev.Delta*line)
	case EventClick:
		// A press on either scrollbar grabs its thumb, or pages the track
		// toward the click; the content area stays passive for clicks.
		if g, ok := s.vscrollGeom(); s.sbV.press(g, ok, ev, s.viewport().H, func(d int) { s.Scroll(0, d) }) {
			return
		}
		if g, ok := s.hscrollGeom(); s.sbH.press(g, ok, ev, s.viewport().W, func(d int) { s.Scroll(d, 0) }) {
			return
		}
	case EventMouseDrag:
		gv, okv := s.vscrollGeom()
		s.sbV.drag(gv, okv, ev, func(target int) { s.Scroll(0, target-s.OffsetY) })
		gh, okh := s.hscrollGeom()
		s.sbH.drag(gh, okh, ev, func(target int) { s.Scroll(target-s.OffsetX, 0) })
	case EventMouseUp:
		s.sbV.release()
		s.sbH.release()
	case EventKeyDown:
		switch ev.Code {
		case "ArrowUp":
			s.Scroll(0, -line)
		case "ArrowDown":
			s.Scroll(0, line)
		case "PageUp":
			s.Scroll(0, -s.viewport().H)
		case "PageDown":
			s.Scroll(0, s.viewport().H)
		case "Home":
			s.Scroll(0, -scrollExtreme)
		case "End":
			s.Scroll(0, scrollExtreme)
		}
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
		// Scroll by translating the PAINT, not by moving the child.
		//
		// This used to set the child's bounds to the viewport origin MINUS the
		// scroll offset, draw, and put them back. Geometry that changes for the
		// duration of a paint is invisible to anything reading it between
		// frames — the accessibility bridges do exactly that — so a control
		// could be announced a whole viewport away from where it was painted.
		//
		// The child's own X/Y were never used even then (only its width and
		// height were kept), so placing it at the viewport origin loses
		// nothing and is simply true: that is where the content starts, and
		// the offset says how far it has scrolled away.
		cb := s.Child.Bounds()
		tr, canTranslate := p.(painter.Translator)
		if canTranslate {
			s.Child.SetBounds(Rect{X: r.X, Y: r.Y, W: cb.W, H: cb.H})
			tr.PushTranslate(-s.OffsetX, -s.OffsetY)
		} else {
			// A back-end with no translation still has to scroll, so fall back
			// to the old shape rather than showing the content unscrolled.
			s.Child.SetBounds(Rect{X: r.X - s.OffsetX, Y: r.Y - s.OffsetY, W: cb.W, H: cb.H})
		}
		s.Child.Draw(p, theme)
		if canTranslate {
			tr.PopTranslate()
		} else {
			s.Child.SetBounds(cb)
		}
		if canClip {
			clr.PopClip()
		}
	}
	// Vertical scrollbar (right edge), sized to the viewport height so it leaves
	// the corner for a horizontal bar. The track is always painted; the thumb
	// comes from vscrollGeom so Draw + OnEvent share one definition of it.
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, vp.H, theme.SurfaceAlt)
	if g, ok := s.vscrollGeom(); ok {
		fillRect(p, r.X+g.cross0, r.Y+g.thumbStart, scrollbarWidth, g.thumbLen, theme.Accent)
	}
	// Horizontal scrollbar (bottom edge), only when the content overflows
	// horizontally (which is exactly when the viewport reserved the bottom row).
	if s.contentW > vp.W {
		trackY := r.Y + r.H - scrollbarWidth
		fillRect(p, r.X, trackY, vp.W, scrollbarWidth, theme.SurfaceAlt)
		if g, ok := s.hscrollGeom(); ok {
			fillRect(p, r.X+g.thumbStart, trackY, g.thumbLen, scrollbarWidth, theme.Accent)
		}
	}
}

// HitTest covers the full bounds (the scrollbar is interactive too).
func (s *ScrollView) HitTest(px, py int) bool { return s.Bounds().Contains(px, py) }
