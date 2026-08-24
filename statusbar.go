// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Statusbar is a thin horizontal strip at the bottom of a window that
// shows N text segments (e.g. "Line 12, Col 4" + "UTF-8" + "Plain
// text" in an editor). Segments paint left-to-right with a 1-pixel
// divider between them; the LAST segment expands to fill any remaining
// width so an empty Statusbar still looks deliberate.
//
// Statusbar is the natural pairing for MenuBar + Toolbar above and a
// document area in the middle — together they assemble the "stock
// GTK" window frame.
//
// # Interactive segments
//
// Beyond the plain [Statusbar.Segments] text strip, a Statusbar can host
// INTERACTIVE segments grouped into three clusters — [Statusbar.Left],
// [Statusbar.Center] and [Statusbar.Right] — packed against the left edge,
// centred in the bar, and packed against the right edge respectively. Each
// [StatusSegment] either carries an OnClick (a plain text cell that fires a
// callback when clicked) or hosts a real interactive Widget (a Button, a
// Switch, ...) laid out into the segment's box. [Statusbar.OnEvent] hit-tests a
// pointer event against the segment boxes and routes it — invoking the OnClick
// or forwarding the (widget-local–translated) event to the hosted widget — so an
// app never hand-rolls `x >= W-liveServerWidth` math to find which region a
// click landed on. [Statusbar.SegmentAt] answers the same "which segment is
// under this point" query for cursor/tooltip use.
//
// The two APIs are independent and back-compatible: with all three groups empty
// a Statusbar draws and behaves EXACTLY as the text-only strip it always was
// (its OnEvent is then a no-op). Populate any group and the interactive layout
// takes over; a host that wants both static text and a clickable region models
// the static text as an OnClick-less (non-interactive) StatusSegment.
type Statusbar struct {
	Base
	Segments []string

	// Left, Center and Right are the interactive segment groups. Left packs
	// from the bar's left edge, Right packs against its right edge (keeping each
	// group's own order left-to-right), and Center is centred in the bar. All
	// three default empty, in which case the plain Segments strip is drawn.
	Left   []StatusSegment
	Center []StatusSegment
	Right  []StatusSegment

	// SegmentMinW is the minimum width any non-last segment takes. The
	// last segment ALWAYS fills the rest of the bar.
	SegmentMinW int // default StatusbarSegmentMinW
}

// StatusSegment is one interactive cell in a Statusbar's Left/Center/Right
// groups. It is either a text cell — Text painted centred-vertically, with
// OnClick fired on a click inside its box — or a widget cell, when Widget is
// non-nil: the hosted Widget is laid out into the segment's box (its bounds set
// each Draw) and every event over the box is forwarded to it in its own local
// coordinates, so a Button / Switch / IconButton behaves exactly as it would
// anywhere else. When Widget is set, Text and OnClick are ignored.
//
// MinW floors the segment's width; a zero MinW fits the text (or the hosted
// widget's own width). It lets a group of cells align to a fixed grid without
// jittering as their text changes.
type StatusSegment struct {
	Text    string
	OnClick func()
	Widget  Widget
	MinW    int
}

// Sizing constants.
const (
	StatusbarH           = 18
	StatusbarSegmentMinW = 80
	StatusbarPadX        = 6
)

// NewStatusbar builds a Statusbar with the given segments. SegmentMinW is left
// at zero — "use the default" — so the default minimum resolves through scaled at
// draw time and grows with HiDPI and touch density; a caller that wants a fixed
// minimum sets SegmentMinW explicitly (honoured verbatim). At compact/1x the
// resolved default is StatusbarSegmentMinW, byte-identical to before.
func NewStatusbar(segs []string) *Statusbar {
	return &Statusbar{Segments: segs}
}

// SetSegment replaces the i-th segment in place. Indexes out of range
// are appended (filling intermediate slots with "") so callers can
// grow the bar lazily.
func (s *Statusbar) SetSegment(i int, text string) {
	if i < 0 {
		return
	}
	for len(s.Segments) <= i {
		s.Segments = append(s.Segments, "")
	}
	s.Segments[i] = text
}

// hasGroups reports whether any interactive segment group is populated — the
// switch between the interactive layout and the legacy text strip.
func (s *Statusbar) hasGroups() bool {
	return len(s.Left)+len(s.Center)+len(s.Right) > 0
}

// segWidth is the box width of one interactive segment: the hosted widget's own
// width when it has one, else its text plus horizontal padding, floored at MinW.
func (s *Statusbar) segWidth(seg StatusSegment, padX int) int {
	w := s.textWidth(seg.Text) + 2*padX
	if seg.Widget != nil && seg.Widget.Bounds().W > 0 {
		w = seg.Widget.Bounds().W
	}
	if w < seg.MinW {
		w = seg.MinW
	}
	return w
}

// statusBox pairs an interactive segment with its widget-local box (the same
// coordinate space OnEvent's X/Y and SegmentAt's arguments live in).
type statusBox struct {
	seg  *StatusSegment
	rect Rect
}

// boxes lays every interactive segment out into a widget-local box: the Left
// group packed from x=0, the Right group packed against the right edge, and the
// Center group centred in the bar. The returned rects share the coordinate space
// OnEvent receives, so hit-testing and drawing (offset by Bounds) agree exactly.
func (s *Statusbar) boxes() []statusBox {
	r := s.Bounds()
	padX := scaled(StatusbarPadX)
	out := make([]statusBox, 0, len(s.Left)+len(s.Center)+len(s.Right))

	x := 0
	for i := range s.Left {
		w := s.segWidth(s.Left[i], padX)
		out = append(out, statusBox{&s.Left[i], Rect{X: x, Y: 0, W: w, H: r.H}})
		x += w
	}

	rws := make([]int, len(s.Right))
	rsum := 0
	for i := range s.Right {
		rws[i] = s.segWidth(s.Right[i], padX)
		rsum += rws[i]
	}
	x = r.W - rsum
	for i := range s.Right {
		out = append(out, statusBox{&s.Right[i], Rect{X: x, Y: 0, W: rws[i], H: r.H}})
		x += rws[i]
	}

	cws := make([]int, len(s.Center))
	csum := 0
	for i := range s.Center {
		cws[i] = s.segWidth(s.Center[i], padX)
		csum += cws[i]
	}
	x = (r.W - csum) / 2
	for i := range s.Center {
		out = append(out, statusBox{&s.Center[i], Rect{X: x, Y: 0, W: cws[i], H: r.H}})
		x += cws[i]
	}
	return out
}

// SegmentAt returns the interactive segment whose box contains the widget-local
// point (x, y) — the same coordinate space [Statusbar.OnEvent] receives — or nil
// when the point is over no segment (or the bar has no interactive segments). A
// host uses it to set a pointer cursor or a tooltip for the region under the
// mouse without re-deriving the group layout.
func (s *Statusbar) SegmentAt(x, y int) *StatusSegment {
	for _, b := range s.boxes() {
		if b.rect.Contains(x, y) {
			return b.seg
		}
	}
	return nil
}

// OnEvent routes a pointer event to the interactive segment under it: an event
// over a widget-hosting segment is forwarded to that widget in its own local
// coordinates; an EventClick over a text segment fires the segment's OnClick.
// The event's X/Y are widget-local (the container already offset them into the
// bar's frame). A Disabled bar, an event over no segment, or a non-click over a
// plain text segment is a no-op — and a bar with no interactive segments ignores
// every event, exactly as the legacy text strip did.
func (s *Statusbar) OnEvent(ev Event) {
	if s.Disabled().Get() {
		return
	}
	for _, b := range s.boxes() {
		if !b.rect.Contains(ev.X, ev.Y) {
			continue
		}
		if b.seg.Widget != nil {
			local := ev
			local.X = ev.X - b.rect.X
			local.Y = ev.Y - b.rect.Y
			b.seg.Widget.OnEvent(local)
			return
		}
		if ev.Kind == EventClick && b.seg.OnClick != nil {
			b.seg.OnClick()
		}
		return
	}
}

// Draw paints the strip + every segment.
func (s *Statusbar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if s.hasGroups() {
		s.drawGroups(p, theme, r)
		return
	}
	s.drawTextStrip(p, theme, r)
}

// drawGroups paints the interactive Left/Center/Right segments: a hosted widget
// is placed into its box (surface coordinates) and drawn; a text segment paints
// its label centred vertically at the box's left pad.
func (s *Statusbar) drawGroups(p painter.Painter, theme *Theme, r Rect) {
	padX := scaled(StatusbarPadX)
	for _, b := range s.boxes() {
		bx, by := r.X+b.rect.X, r.Y+b.rect.Y
		if b.seg.Widget != nil {
			b.seg.Widget.SetBounds(Rect{X: bx, Y: by, W: b.rect.W, H: b.rect.H})
			b.seg.Widget.Draw(p, theme)
			continue
		}
		ty := by + (b.rect.H-s.glyphHeight())/2
		s.drawText(p, bx+padX, ty, b.seg.Text, theme.OnSurface)
	}
}

// drawTextStrip paints the legacy Segments strip: each non-last segment sized to
// its text (floored at the resolved minimum), the last expanding to fill, with a
// 1-pixel divider between them. Byte-identical to the pre-groups Draw.
func (s *Statusbar) drawTextStrip(p painter.Painter, theme *Theme, r Rect) {
	// The default min-width resolves through scaled (an explicit SegmentMinW is
	// honoured verbatim); the horizontal pad routes through scaled too. Both equal
	// their logical constants at compact/1x, keeping the strip byte-identical.
	min := s.SegmentMinW
	if min <= 0 {
		min = scaled(StatusbarSegmentMinW)
	}
	padX := scaled(StatusbarPadX)
	x := r.X
	n := len(s.Segments)
	for i, seg := range s.Segments {
		var w int
		if i == n-1 {
			w = r.X + r.W - x
		} else {
			w = s.textWidth(seg) + 2*padX
			if w < min {
				w = min
			}
		}
		ty := r.Y + (r.H-s.glyphHeight())/2
		s.drawText(p, x+padX, ty, seg, theme.OnSurface)
		if i < n-1 {
			fillRect(p, x+w-1, r.Y+2, 1, r.H-4, theme.Border)
		}
		x += w
	}
}
