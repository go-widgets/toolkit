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
// walk finds a tree where there is only a rectangle of pixels. It paints
// nothing: the application already painted it.
type surfaceProxy struct {
	Base
	info A11yInfo
}

func (p *surfaceProxy) Draw(painter.Painter, *Theme) {}

func (p *surfaceProxy) A11y() A11yInfo { return p.info }
