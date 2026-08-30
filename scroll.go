// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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
	offsetX, offsetY *mvvm.Observable[int] // scroll offsets, reactive via OffsetX()/OffsetY()
	contentW         int
	contentH         int

	// sbV and sbH track an in-progress drag of the vertical / horizontal
	// scrollbar thumb respectively.
	sbV, sbH scrollDrag

	// pan tracks an in-progress drag of the CONTENT itself.
	pan contentPan

	// momX and momY are the per-axis touch-scroll engines, created on first
	// use. They hold the authoritative position, which OffsetX/OffsetY mirror
	// clamped and overscrollX/Y complete.
	momX, momY *Momentum
	// trackX and trackY smooth the drag samples into a release velocity.
	trackX, trackY VelocityTracker
	// overscrollX and overscrollY are the rubber-band displacement beyond the
	// clamped offsets; only Draw consults them.
	overscrollX, overscrollY int
	// driver, when set, is whatever owns this view's touch scrolling; the
	// view then does not pan or fling on its own. See SetScrollDriver.
	driver any
}

// OffsetX/OffsetY are the scroll offsets as shared [mvvm.Observable]s; the wheel
// and clamping Set them. Lazily created.
func (s *ScrollView) OffsetX() *mvvm.Observable[int] {
	if s.offsetX == nil {
		s.offsetX = mvvm.NewObservable(0)
	}
	return s.offsetX
}
func (s *ScrollView) OffsetY() *mvvm.Observable[int] {
	if s.offsetY == nil {
		s.offsetY = mvvm.NewObservable(0)
	}
	return s.offsetY
}

// contentPan is a drag of the scrolled content — the gesture that scrolls a
// view by pulling what is in it, rather than by moving a scrollbar thumb.
//
// It is what makes a scrollable widget usable at all without a mouse or a
// keyboard. A touch screen has no wheel to turn and no arrow keys to press, and
// a scrollbar thumb is a target a few pixels wide: on a phone, dragging the
// content IS the way to scroll, and before this a ScrollView on a touch device
// could not be scrolled by any means.
//
// The content follows the finger: dragging down moves the content down, which
// is a DECREASING scroll offset. Reversing that is the "natural scrolling"
// question, and the answer for a direct-touch surface is not a preference —
// the content is under the finger and has to go where the finger goes.
type contentPan struct {
	active bool
	x, y   int // the last pointer position, in surface coordinates
	// accumX and accumY are the pixels moved since the last tick, which is
	// how a velocity is measured at all: a toolkit.Event carries no
	// timestamp, so the only clock available is the tick.
	accumX, accumY int
}

// start begins a pan at the pointer's position.
func (p *contentPan) start(ev Event) {
	p.active, p.x, p.y = true, ev.X, ev.Y
	p.accumX, p.accumY = 0, 0
}

// delta reports how far to scroll for this pointer sample and remembers the
// position for the next one. The result is the movement of the CONTENT
// inverted, i.e. what to add to the scroll offset.
func (p *contentPan) delta(ev Event) (dx, dy int) {
	dx, dy = p.x-ev.X, p.y-ev.Y
	p.x, p.y = ev.X, ev.Y
	p.accumX += dx
	p.accumY += dy
	return dx, dy
}

// release ends the pan.
func (p *contentPan) release() { p.active = false }

// scrollbarWidth is the LOGICAL pixel thickness of a scrollbar track — the ONE
// canonical width every scrollbar in the toolkit uses: slim and modern while
// still grabbable. Routed through [scrollbarTrack] so a HiDPI host gets a track
// scaled to its device pixels. A host that hides a widget's own bar and paints
// its own (the HideScrollbar pattern) reads it through [ScrollbarWidth] so its
// bar aligns with every embedded one by construction, not by picking a number.
const scrollbarWidth = 6

// scrollbarGap is the LOGICAL pixel gap the scrolled content is inset by ON TOP
// of the scrollbar track, so the content never sits flush against the thumb.
// Every scrollable reader widget (ScrollView, ListBox, TreeView) insets its
// content by the same [scrollGutter], so the gap reads identically across them.
const scrollbarGap = 4

// scrollbarTrack is the device-pixel thickness of a scrollbar track at the
// current metric scale. The track column is painted this wide, and the drag
// hit-band matches it via sbGeom.crossW.
func scrollbarTrack() int { return scaled(scrollbarWidth) }

// scrollGutter is the device-pixel width the scrolled content is inset by to
// clear the scrollbar: the scaled track thickness plus a scaled normalized gap.
// Because ScrollView, ListBox and TreeView all inset by this one value, content
// never touches the thumb and the gap is the same everywhere.
func scrollGutter() int { return scaled(scrollbarWidth) + scaled(scrollbarGap) }

// ScrollbarWidth is the device-pixel thickness of the toolkit's scrollbar track
// — the single source of truth. A host that hides a widget's own bar and paints
// its own (see the HideScrollbar pattern) sizes it through this, so every bar in
// the app matches the embedded ones without hardcoding a width of its own.
func ScrollbarWidth() int { return scrollbarTrack() }

// ScrollGutter is the device-pixel width scrolled content is inset by to clear
// the scrollbar (track plus gap) — the same reservation every scrollable widget
// makes, exported so a host overlaying its own bar reserves the identical gutter.
func ScrollGutter() int { return scrollGutter() }

// localViewport is [ScrollView.viewport] in WIDGET-LOCAL coordinates, the space
// an event arrives in.
//
// A parent hands a child an event measured from the CHILD's top-left (see
// translateEvent), while viewport() is derived from Bounds and is therefore
// surface-absolute. Comparing one against the other silently works for a
// ScrollView that happens to sit at the surface origin and silently fails for
// every other one — which is what happened: a ScrollView nested in a box never
// armed its content pan, so it could not be scrolled by dragging at all.
func (s *ScrollView) localViewport() Rect {
	r, vp := s.Bounds(), s.viewport()
	return Rect{X: vp.X - r.X, Y: vp.Y - r.Y, W: vp.W, H: vp.H}
}

// viewport is the visible content rect: the bounds minus the always-reserved
// right scrollbar gutter and — when the content overflows horizontally — the
// bottom scrollbar gutter. The gutter is the track plus a normalized gap, so the
// content stops short of the track rather than butting against it.
func (s *ScrollView) viewport() Rect {
	r := s.Bounds()
	vw := r.W - scrollGutter()
	vh := r.H
	if s.contentW > vw {
		vh -= scrollGutter()
	}
	return Rect{X: r.X, Y: r.Y, W: vw, H: vh}
}

// focusableChildren yields the scrolled content, so the focus walker can reach
// the controls inside it. Without this a ScrollView is a wall: a panel of ten
// inputs in a scrolling column had none of them reachable by Tab.
func (s *ScrollView) focusableChildren() []Widget { return nonNil(s.Child) }

// revealFocused scrolls so that the focused descendant, if there is one, is
// inside the viewport. Reaching a control by Tab and leaving it out of sight
// would trade one defect for a worse one — a cursor blinking where nobody can
// see it — and a scrolling panel is exactly where that happens: ten controls in
// a column three deep leaves eight of them below the fold.
//
// A child's Bounds do not move when the view scrolls; SetBounds anchors the
// child at the viewport's origin with the whole content's height, and the
// offset is applied when it is drawn. So a child's position within the content
// is its Bounds minus that origin, and what has to change is the offset.
func (s *ScrollView) revealFocused() {
	w := focusedInList(focusListOf(s))
	if w == nil {
		return
	}
	vp, b := s.viewport(), w.Bounds()
	s.Scroll(revealDelta(b.X-vp.X, b.W, s.OffsetX().Get(), vp.W),
		revealDelta(b.Y-vp.Y, b.H, s.OffsetY().Get(), vp.H))
}

// revealDelta is how far to scroll along one axis so that the span [at, at+size)
// of the content lies inside the window [offset, offset+extent). It brings the
// near edge in when the span is before the window and the far edge in when it is
// after, and asks for nothing when the span is already inside or is larger than
// the window can hold — in which case the near edge is the one worth showing.
func revealDelta(at, size, offset, extent int) int {
	if at < offset {
		return at - offset
	}
	if at+size > offset+extent && size <= extent {
		return at + size - offset - extent
	}
	if at >= offset+extent {
		return at - offset
	}
	return 0
}

// NewScrollView builds a ScrollView around child. Call SetContentSize
// after construction to declare the child's logical extent so the
// thumb is sized correctly + scrolling is clamped.
func NewScrollView(child Widget) *ScrollView {
	return &ScrollView{Child: child}
}

// SetBounds positions the view and, when the child can say how big it wants to
// be, sizes the child and declares the content extent from that.
//
// This is the other half of a scroll view, and it was the caller's until now:
// nothing here gave the child any bounds, and Draw kept only the width and
// height the child already carried. So a scroll view around a freshly built
// column showed nothing at all -- the child had no size -- and one whose content
// changed kept scrolling over the extent it was told about the last time
// somebody remembered to call SetContentSize.
//
// Only a MEASURABLE child is sized here. A child that cannot answer is left
// exactly as it was, so a caller who lays out and declares the extent by hand
// keeps doing so, and one who calls SetContentSize after SetBounds still wins:
// their call is simply later.
//
// The content is never shorter than the viewport, so a short child fills the
// view instead of leaving a strip of bare surface under it.
func (s *ScrollView) SetBounds(r Rect) {
	s.Base.SetBounds(r)
	if s.Child == nil {
		return
	}
	h, ok := scrollNatural(s.Child, s.viewport().W)
	if !ok {
		return
	}
	vp := s.viewport()
	if h < vp.H {
		h = vp.H
	}
	s.Child.SetBounds(Rect{X: vp.X, Y: vp.Y, W: vp.W, H: h})
	s.SetContentSize(vp.W, h)
}

// scrollNatural is the height a child asks for at width w, and whether it could
// say. It is the same question a column asks of a card -- see WidthMeasurer --
// and deliberately does NOT fall back to the child's own bounds: a child that
// cannot measure must be left alone, not re-declared at whatever size it happens
// to be carrying.
func scrollNatural(child Widget, w int) (int, bool) {
	if m, ok := child.(WidthMeasurer); ok {
		if h := m.Measure(w); h > 0 {
			return h, true
		}
	}
	if m, ok := child.(Measurer); ok {
		if _, h := m.Measure(w, measureUnbounded); h > 0 {
			return h, true
		}
	}
	return 0, false
}

// SetContentSize tells the ScrollView how big the child's logical
// drawing area is. Used by Scroll() to clamp + by Draw() to size
// the thumb. A measurable child has this done for it by SetBounds.
func (s *ScrollView) SetContentSize(w, h int) {
	s.contentW = w
	s.contentH = h
}

// Scroll mutates the offsets by (dx, dy) and clamps to
// [0, contentSize - viewportSize] so the thumb never falls off the
// track. Negative offsets are clamped to 0.
func (s *ScrollView) Scroll(dx, dy int) {
	s.OffsetX().Set(s.OffsetX().Get() + dx)
	s.OffsetY().Set(s.OffsetY().Get() + dy)
	vp := s.viewport()
	maxX := s.contentW - vp.W
	if maxX < 0 {
		maxX = 0
	}
	maxY := s.contentH - vp.H
	if maxY < 0 {
		maxY = 0
	}
	if s.OffsetX().Get() < 0 {
		s.OffsetX().Set(0)
	}
	if s.OffsetX().Get() > maxX {
		s.OffsetX().Set(maxX)
	}
	if s.OffsetY().Get() < 0 {
		s.OffsetY().Set(0)
	}
	if s.OffsetY().Get() > maxY {
		s.OffsetY().Set(maxY)
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
	if thumbH < scaled(8) {
		thumbH = scaled(8)
	}
	maxOff := s.contentH - vp.H // > 0 here
	return sbGeom{
		cross0:     r.W - scrollbarTrack(),
		crossW:     scrollbarTrack(),
		trackStart: 0,
		trackLen:   vp.H,
		thumbStart: s.OffsetY().Get() * (vp.H - thumbH) / maxOff,
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
	if thumbW < scaled(8) {
		thumbW = scaled(8)
	}
	maxOff := s.contentW - vp.W // > 0 here
	return sbGeom{
		horizontal: true,
		cross0:     r.H - scrollbarTrack(),
		crossW:     scrollbarTrack(),
		trackStart: 0,
		trackLen:   vp.W,
		thumbStart: s.OffsetX().Get() * (vp.W - thumbW) / maxOff,
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
		// Both axes: a vertical notch moves OffsetY, a horizontal swipe
		// (ev.DeltaX, from the browser wheel's deltaX) moves OffsetX, so a
		// left/right two-finger swipe drives the horizontal scrollbar instead
		// of being dropped. Scroll clamps each axis independently.
		s.Scroll(ev.DeltaX*line, ev.Delta*line)
	case EventClick:
		// A press on either scrollbar grabs its thumb, or pages the track
		// toward the click; the content area stays passive for clicks.
		if g, ok := s.vscrollGeom(); s.sbV.press(g, ok, ev, s.viewport().H, func(d int) { s.Scroll(0, d) }) {
			return
		}
		if g, ok := s.hscrollGeom(); s.sbH.press(g, ok, ev, s.viewport().W, func(d int) { s.Scroll(d, 0) }) {
			return
		}
		// Neither scrollbar wanted the press, so it landed on the content --
		// which means it was meant for whatever is drawn there.
		//
		// The content used to be PASSIVE for clicks: the press began a pan and
		// nothing was forwarded, so every control inside a scroll view was dead
		// to the mouse. A long form is exactly what a scroll view is for, and
		// none of its drop-downs, switches or buttons could be operated.
		//
		// Translated by the scroll offset, because that is how the content is
		// PAINTED: the child sits at the viewport origin and Draw shifts the
		// paint by the offset, so the thing under the pointer is the content
		// point offset further in. Without this, clicking a control after
		// scrolling hits whatever was in its place before.
		if s.Child != nil && s.localViewport().Contains(ev.X, ev.Y) {
			inner := translateEvent(ev, s.Bounds(), s.Child.Bounds())
			inner.X += s.OffsetX().Get()
			inner.Y += s.OffsetY().Get()
			s.Child.OnEvent(inner)
		}
		// And a pan as well, on the same press: a click acts on what it hits,
		// and a DRAG that follows scrolls the view. Both are true of every
		// scroll view a person has used.
		if !s.ScrollDriven() && s.localViewport().Contains(ev.X, ev.Y) {
			// Catching a coasting view stops it dead, the way putting a
			// finger on a spinning record does.
			s.stopTouchScroll()
			s.pan.start(ev)
			s.beginTouchScroll()
		}
	case EventMouseDrag:
		gv, okv := s.vscrollGeom()
		s.sbV.drag(gv, okv, ev, func(target int) { s.Scroll(0, target-s.OffsetY().Get()) })
		gh, okh := s.hscrollGeom()
		s.sbH.drag(gh, okh, ev, func(target int) { s.Scroll(target-s.OffsetX().Get(), 0) })
		// A grabbed thumb owns the drag; the content only pans when neither
		// scrollbar is being dragged, so one gesture never moves the view
		// twice.
		if s.pan.active && !s.sbV.active && !s.sbH.active {
			s.dragTouchScroll(s.pan.delta(ev))
		}
	case EventMouseUp:
		s.sbV.release()
		s.sbH.release()
		if s.pan.active {
			// Let go while still moving and the view carries on.
			s.pan.release()
			s.endTouchScroll()
		}
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
		// The rubber band rides ON TOP of the offset, and only here: it moves
		// the painted content without moving the scroll position anything else
		// reads. At rest it is zero and this is the offset alone.
		px, py := s.OffsetX().Get()+s.overscrollX, s.OffsetY().Get()+s.overscrollY
		tr, canTranslate := p.(painter.Translator)
		if canTranslate {
			s.Child.SetBounds(Rect{X: r.X, Y: r.Y, W: cb.W, H: cb.H})
			tr.PushTranslate(-px, -py)
		} else {
			// A back-end with no translation still has to scroll, so fall back
			// to the old shape rather than showing the content unscrolled.
			s.Child.SetBounds(Rect{X: r.X - px, Y: r.Y - py, W: cb.W, H: cb.H})
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
	track := scrollbarTrack()
	trackX := r.X + r.W - track
	paintScrollTrack(p, theme, trackX, r.Y, track, vp.H)
	if g, ok := s.vscrollGeom(); ok {
		paintScrollThumb(p, theme, r.X+g.cross0, r.Y+g.thumbStart, track, g.thumbLen)
	}
	// Horizontal scrollbar (bottom edge), only when the content overflows
	// horizontally (which is exactly when the viewport reserved the bottom row).
	if s.contentW > vp.W {
		trackY := r.Y + r.H - track
		paintScrollTrack(p, theme, r.X, trackY, vp.W, track)
		if g, ok := s.hscrollGeom(); ok {
			paintScrollThumb(p, theme, r.X+g.thumbStart, trackY, g.thumbLen, track)
		}
	}
}

// HitTest covers the full bounds (the scrollbar is interactive too).
func (s *ScrollView) HitTest(px, py int) bool { return s.Bounds().Contains(px, py) }

// The touch-scroll engine.
//
// ScrollView drives a [Momentum] per axis — the same engine [MomentumScroller]
// exposes for a host that routes touch itself — so a drag rubber-bands past the
// ends, a release coasts with the engine's physics, and an overscrolled view
// springs home. It needs no host wiring at all: ScrollView is an [Animator], so
// a host already calling [TickTree] drives it, and [TreeAnimating] lets that
// host stop scheduling frames the moment the view comes to rest.
//
// OffsetX and OffsetY REMAIN STRICTLY CLAMPED to [0, max]. That is deliberate,
// and it is the difference between this and writing the engine's offset
// straight into them: they are exported fields that callers read to convert a
// screen point into a content point, and the scrollbar geometry computes the
// thumb from them — an offset past a bound puts the thumb outside its own
// track. So the engine's excursion is split in two. The part inside the bounds
// is the offset; the part beyond is [ScrollView.Overscroll], which only Draw
// consults, shifting the painted content without ever moving the scroll
// position a caller can see.

// axis returns the engine for one axis, creating it on first use so a
// zero-value ScrollView needs no constructor change.
func (s *ScrollView) axis(a **Momentum) *Momentum {
	if *a == nil {
		*a = NewMomentum()
	}
	return *a
}

// syncEngines refreshes each engine's clamp window from the current geometry
// and seeds it with the offset the view actually has, so a drag starts from
// where the content sits however it got there (a wheel notch, a key, a thumb).
func (s *ScrollView) syncEngines() {
	x, y := s.axis(&s.momX), s.axis(&s.momY)
	x.SetBounds(0, float64(s.maxOffsetX()))
	y.SetBounds(0, float64(s.maxOffsetY()))
	x.SetOffset(float64(s.OffsetX().Get()))
	y.SetOffset(float64(s.OffsetY().Get()))
}

// beginTouchScroll arms both engines for a content drag.
func (s *ScrollView) beginTouchScroll() {
	s.syncEngines()
	s.momX.BeginDrag()
	s.momY.BeginDrag()
	s.trackX.Reset()
	s.trackY.Reset()
}

// dragTouchScroll feeds one drag sample: dx, dy is the movement to add to the
// scroll offset, already inverted so the content follows the finger.
func (s *ScrollView) dragTouchScroll(dx, dy int) {
	s.axis(&s.momX).DragBy(float64(dx))
	s.axis(&s.momY).DragBy(float64(dy))
	s.pan.accumX += dx
	s.pan.accumY += dy
	s.pushEngines()
}

// endTouchScroll releases the drag into a coast at the velocity the trackers
// smoothed from the recent samples. A release while overscrolled springs home
// whatever the velocity was.
func (s *ScrollView) endTouchScroll() {
	s.axis(&s.momX).EndDrag(s.trackX.Velocity())
	s.axis(&s.momY).EndDrag(s.trackY.Velocity())
}

// pushEngines writes the engines' positions back to the view, splitting each
// into the clamped offset and the overscroll beyond it.
func (s *ScrollView) pushEngines() {
	ox, overX := splitOverscroll(s.axis(&s.momX).OffsetInt(), s.maxOffsetX())
	s.OffsetX().Set(ox)
	s.overscrollX = overX
	oy, overY := splitOverscroll(s.axis(&s.momY).OffsetInt(), s.maxOffsetY())
	s.OffsetY().Set(oy)
	s.overscrollY = overY
}

// splitOverscroll divides an engine position into the part a scroll offset may
// legally hold and the rubber-band excess beyond it.
func splitOverscroll(pos, max int) (offset, over int) {
	switch {
	case pos < 0:
		return 0, pos
	case pos > max:
		return max, pos - max
	default:
		return pos, 0
	}
}

// Overscroll reports the rubber-band displacement, in pixels: negative past the
// start of the content, positive past its end, zero at rest. It is what Draw
// shifts the content by ON TOP of the scroll offset, so a caller wanting to know
// where the content actually sits adds it to OffsetX/OffsetY; a caller wanting
// the scroll position ignores it, which is why the offsets stay clamped.
func (s *ScrollView) Overscroll() (x, y int) { return s.overscrollX, s.overscrollY }

// Tick advances the touch-scroll engines by dt seconds, and turns the movement
// the last frame's drag accumulated into a velocity.
//
// It makes ScrollView an [Animator], so a host already calling [TickTree]
// drives the coast with no extra wiring. Velocity is measured HERE rather than
// in the drag handler because a toolkit.Event carries no timestamp: the only
// clock in the toolkit is the dt a host hands to a tick.
func (s *ScrollView) Tick(dt float64) {
	if dt <= 0 || s.ScrollDriven() {
		return
	}
	if s.pan.active {
		// Under the finger: measure, do not advance. The drag itself is what
		// moves the content while the contact lasts.
		s.trackX.Sample(float64(s.pan.accumX), dt)
		s.trackY.Sample(float64(s.pan.accumY), dt)
		s.pan.accumX, s.pan.accumY = 0, 0
		return
	}
	if s.momX == nil && s.momY == nil {
		return
	}
	s.axis(&s.momX).Tick(dt)
	s.axis(&s.momY).Tick(dt)
	s.pushEngines()
}

// Animating reports whether the view still needs frames: while an engine is
// settling, or while a finger is down and its speed is being measured.
func (s *ScrollView) Animating() bool {
	if s.pan.active {
		return true
	}
	return (s.momX != nil && s.momX.Settling()) || (s.momY != nil && s.momY.Settling())
}

// stopTouchScroll brings the view to rest wherever it is, discarding any coast.
func (s *ScrollView) stopTouchScroll() {
	if s.momX != nil {
		s.momX.Stop()
	}
	if s.momY != nil {
		s.momY.Stop()
	}
	s.overscrollX, s.overscrollY = 0, 0
}

// maxOffsetX and maxOffsetY are the largest legal scroll offsets: how far the
// content can move before its far edge meets the viewport's.
func (s *ScrollView) maxOffsetX() int { return maxInt(0, s.contentW-s.viewport().W) }
func (s *ScrollView) maxOffsetY() int { return maxInt(0, s.contentH-s.viewport().H) }

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ScrollDriven reports whether something else owns this view's scrolling.
//
// A [MomentumScroller] wrapping a ScrollView consumes the same drag the view's
// own content pan does, and a host has to deliver those events to the widget
// tree anyway — buttons and lists need them. So without this the gesture lands
// twice and the view scrolls at double speed: one 30-pixel drag sample moved a
// wrapped view 60 pixels.
//
// A driven view therefore stands down: it neither pans nor flings on its own,
// and leaves OffsetX/OffsetY entirely to its driver. Everything else — wheel,
// arrow and page keys, scrollbar thumbs — keeps working, because those are not
// what the driver consumes.
func (s *ScrollView) ScrollDriven() bool { return s.driver != nil }

// SetScrollDriver hands this view's touch scrolling to driver, or takes it back
// when driver is nil. [NewMomentumScroller] calls it, so wrapping a view is all
// a caller has to do.
func (s *ScrollView) SetScrollDriver(driver any) {
	s.driver = driver
	if driver != nil {
		s.pan.release()
		s.stopTouchScroll()
	}
}
