// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Surface shows a framebuffer the application renders itself.
//
// Most applications describe what they want and let the toolkit paint it. Some
// cannot: a game, a video player, a browser engine, a news reader with its own
// scene and hit-testing. They produce finished pixels, and what they need from
// a widget set is somewhere to put them, input in the coordinates those pixels
// use, and a way to still be readable by a screen reader.
//
// Historically such an application had to reach past the painter for the raw
// buffer, which is exactly what stopped it being hosted by a back-end that
// hands out a Painter and nothing else — a recording painter, a damage-tracked
// one, a remote one. Surface is the seam that removes the excuse: it blits
// through the painter's image primitive and degrades to a per-pixel loop on a
// back-end without one.
//
// The buffer is drawn 1:1 at the widget's bounds, and no scaling is invented:
// the application is told the size it has (through Resize on its own side) and
// renders at it. Anything else would resample pixels that were composed for a
// specific size.
//
// # Accessibility
//
// A surface is otherwise opaque — one rectangle of pixels, which is what a
// screen reader would be told, and useless. An application that can say what it
// is showing sets Elements, and each entry becomes a child the accessibility
// walk reads in order. That is what keeps [WalkA11y] and the platform bridges
// working for an application whose widgets the toolkit never sees.
type Surface struct {
	Base

	// Frame is asked for the buffer to show, once per Draw. It returns the
	// RGBA pixels and their dimensions; a nil Frame, or one returning a buffer
	// too short for w*h*4, paints nothing rather than guessing.
	//
	// It is a function rather than a field so the application can hand over
	// whatever it has this frame without copying it anywhere first.
	Frame func() (pix []byte, w, h int)

	// Elements, when set, is asked what the surface is currently showing, in
	// reading order. Rects are in the BUFFER's own pixel coordinates — the same
	// space Frame's pixels and OnInput's events use — and Surface offsets them
	// onto the surface, because that is the one space the application and this
	// widget already agree on.
	Elements func() []SurfaceElement

	// OnInput receives events with coordinates translated into the buffer's
	// space. A nil OnInput drops them.
	OnInput func(Event)

	// Damage, when set, reports which rectangles of the buffer changed since the
	// frame the host last presented, in the buffer's OWN pixel coordinates (the
	// space Frame's pixels, Elements and OnInput use). It is what turns a Surface
	// into a damage-aware root (the host's DamageRenderer capability): a host that
	// presents incrementally then repaints and blits ONLY those rectangles
	// instead of the whole buffer. That is the difference between re-presenting a
	// small animation — a loading spinner, a blinking caret, a progress bar — and
	// re-blitting the whole window for it, which for a full-window application
	// buffer is its single biggest per-frame cost.
	//
	// The contract is exact: return every rectangle whose pixels differ from the
	// frame the host last presented, and no others — a change omitted here is a
	// change the host never shows, because the framebuffer persists between
	// frames. Return nil (or an empty slice) to mean "assume the whole buffer
	// changed", which is always safe and is exactly what a Surface that leaves
	// Damage unset does. An application that cannot cheaply track its own damage
	// simply leaves this nil and keeps the whole-surface path.
	Damage func() []Rect
}

// SurfaceElement is one thing a [Surface] is showing: what it is, what it says,
// and where it sits in the buffer.
type SurfaceElement struct {
	Role  Role
	Name  string
	Value string
	// X, Y, W and H are in the buffer's pixel coordinates.
	X, Y, W, H int
}

// NewSurface returns a Surface fed by frame.
func NewSurface(frame func() (pix []byte, w, h int)) *Surface {
	return &Surface{Frame: frame}
}

// Draw blits the application's current frame at the widget's bounds, clipped to
// them so a buffer larger than the space it was given cannot paint over its
// neighbours.
func (s *Surface) Draw(p painter.Painter, theme *Theme) {
	_ = theme // the application chose every pixel; the theme is not ours to apply
	r := s.Bounds()
	if s.Frame == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	pix, w, h := s.Frame()
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return
	}
	blitImage(p, Rect{X: r.X, Y: r.Y, W: w, H: h}, r, pix, w, h)
}

// RenderDamaged paints this frame into p and returns the rectangles it painted,
// in SURFACE coordinates, so a damage-aware host presents exactly their union.
// It is the incremental-present counterpart of [Surface.Draw] and makes Surface
// satisfy a host's DamageRenderer capability (see [Surface.Damage]).
//
// With Damage unset (or reporting no rectangles) it blits the whole buffer and
// returns its footprint — pixel-identical to Draw followed by a full present.
// With Damage set it blits ONLY the reported rectangles: each is read from the
// buffer's coordinates, offset onto the surface, intersected with the drawn
// buffer footprint, and blitted through the painter's clip so nothing outside it
// is touched; the surviving rectangles are returned. A reported rectangle that
// falls wholly off the buffer contributes nothing and is dropped.
//
// It shares Draw's guards: a nil Frame, a non-positive bounds, or a buffer too
// short for w*h*4 paints nothing and reports no damage.
func (s *Surface) RenderDamaged(p painter.Painter, theme *Theme) []Rect {
	_ = theme // the application chose every pixel; the theme is not ours to apply
	r := s.Bounds()
	if s.Frame == nil || r.W <= 0 || r.H <= 0 {
		return nil
	}
	pix, w, h := s.Frame()
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return nil
	}
	dst := Rect{X: r.X, Y: r.Y, W: w, H: h}
	// The buffer's on-surface footprint: where Draw's whole-buffer blit lands,
	// clipped to the bounds, and the coordinate space the returned damage is in.
	foot := intersectRect(dst, r)
	var dmg []Rect
	if s.Damage != nil {
		dmg = s.Damage()
	}
	if len(dmg) == 0 {
		blitImage(p, dst, r, pix, w, h)
		return []Rect{foot}
	}
	out := make([]Rect, 0, len(dmg))
	for _, d := range dmg {
		// Buffer coordinates -> surface coordinates, then confined to the buffer's
		// on-surface footprint so a rectangle reaching past the buffer (or past the
		// widget's bounds) cannot present or paint stale pixels.
		sc := intersectRect(Rect{X: r.X + d.X, Y: r.Y + d.Y, W: d.W, H: d.H}, foot)
		if sc.W <= 0 || sc.H <= 0 {
			continue
		}
		blitImage(p, dst, sc, pix, w, h)
		out = append(out, sc)
	}
	return out
}

// intersectRect returns the overlap of a and b, or a zero-size rectangle when
// they do not meet.
func intersectRect(a, b Rect) Rect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// OnEvent hands the event to the application with its coordinates moved into
// the buffer's space.
func (s *Surface) OnEvent(ev Event) {
	if s.OnInput == nil {
		return
	}
	r := s.Bounds()
	ev.X -= r.X
	ev.Y -= r.Y
	s.OnInput(ev)
}

// A11y reports the surface itself as presentation: it is a container of what
// the application describes, not a thing in its own right, and announcing it
// would put an unnamed group between the reader and the content.
func (s *Surface) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// Children returns one proxy widget per element the application reports, with
// the element's rectangle moved onto the surface.
//
// The proxies are built fresh on every call and never drawn. That is deliberate:
// what the application is showing changes as it renders, and a cached child
// would describe the frame before last. The accessibility walk is not a
// per-frame path, so building a handful of small structs when a screen reader
// asks costs nothing worth keeping stale answers for.
func (s *Surface) Children() []Widget {
	if s.Elements == nil {
		return nil
	}
	els := s.Elements()
	if len(els) == 0 {
		return nil
	}
	r := s.Bounds()
	out := make([]Widget, 0, len(els))
	for _, e := range els {
		p := &surfaceProxy{info: A11yInfo{Role: e.Role, Name: e.Name, Value: e.Value}}
		p.SetBounds(Rect{X: r.X + e.X, Y: r.Y + e.Y, W: e.W, H: e.H})
		out = append(out, p)
	}
	return out
}

// surfaceProxy stands for one thing the application drew, so the accessibility
// walk finds a tree where there is only a rectangle of pixels.
//
// It deliberately does NOT override Draw: Base's no-op default is already
// exactly right -- the application painted this long before anyone asked what
// it was -- and an empty override of my own would be a function with no
// statements, which reports as 0.0% and fails the per-function coverage gate
// for having nothing to cover.
type surfaceProxy struct {
	Base
	info A11yInfo
}

func (p *surfaceProxy) A11y() A11yInfo { return p.info }
