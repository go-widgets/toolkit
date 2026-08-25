// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Foreign is a rectangle of the interface the toolkit LAYS OUT but does not
// paint: the host renders what goes there.
//
// A toolkit application in a browser is one <canvas> with every pixel blitted
// from wasm. That is the right answer for a diagram, a game or a document
// preview, and the wrong one for anything the browser already does better than
// a pixel buffer: text it can select and search, a video it decodes on the
// compositor, an <input> that raises the right on-screen keyboard and speaks to
// the platform's own accessibility tree. Rendering those into the canvas does
// not merely cost bytes — it deletes capabilities the page had for free.
//
// Foreign is how a widget tree asks for them back without a second layout
// system. It measures and positions like any other widget, so it can sit in a
// VBox, be scrolled, be clipped; it simply declares that its CONTENT belongs to
// the host. [WalkForeign] hands the host every such rectangle after a frame,
// and the host places its own object there.
//
// A host that does not answer for a Kind leaves the region unclaimed, and the
// Fallback widget renders in place — so the same tree still works on a native
// window, in a terminal or in a snapshot test. Declaring a Fallback is
// therefore not optional politeness: it is what keeps the app portable.
//
// Claimed is cross-boundary state (the host tells the widget it took over), so
// it is an [mvvm.Observable] rather than a field the host writes behind the
// widget's back.
type Foreign struct {
	Base

	// Key is the caller's stable identity for this region. The host keys its
	// own object on it, so the same Key across frames means "the same thing
	// moved or resized", not "a new one appeared". Regions without a Key are
	// reported, but a host cannot track them across a relayout.
	Key string

	// Kind names WHAT the host should place here — "html", "svg", "video",
	// "input". It is the host's vocabulary, not the toolkit's: the toolkit
	// never interprets it, it only reports it, so a host may define kinds this
	// package has never heard of.
	Kind string

	// Content is the payload for that Kind, opaque here.
	Content string

	// Fallback renders in place while no host has claimed the region. Its
	// bounds follow this widget's.
	Fallback Widget

	claimed *mvvm.Observable[bool]
}

// Claimed reports whether a host has taken the region over, as a shared
// [mvvm.Observable]: the host sets it after placing its object, the widget
// stops drawing its Fallback, and any view model bound to it sees the change.
// Lazily created.
func (f *Foreign) Claimed() *mvvm.Observable[bool] {
	if f.claimed == nil {
		f.claimed = mvvm.NewObservable(false)
	}
	return f.claimed
}

// Draw paints the Fallback while the region is unclaimed, and nothing at all
// once a host owns it — painting under a host object would waste a blit on
// pixels no one sees, and would show through anything the host draws with
// transparency.
func (f *Foreign) Draw(p painter.Painter, theme *Theme) {
	if f.Claimed().Get() || f.Fallback == nil {
		return
	}
	f.Fallback.SetBounds(f.Bounds())
	f.Fallback.Draw(p, theme)
}

// OnEvent forwards input to the Fallback while it is the thing on screen. Once
// the host has claimed the region its own object is above the canvas and
// receives the events directly, so nothing is forwarded.
func (f *Foreign) OnEvent(ev Event) {
	if f.Claimed().Get() || f.Fallback == nil {
		return
	}
	f.Fallback.OnEvent(ev)
}

// Children exposes the Fallback so the ordinary tree walks (accessibility,
// text runs, layout debugging) see what is actually on screen.
func (f *Foreign) Children() []Widget { return nonNil(f.Fallback) }

// A11y describes the region as presentational: while it is unclaimed the
// Fallback below speaks for itself, and once the host has claimed it the host's
// own object is in the platform's accessibility tree already — which is a large
// part of why the region was handed over.
func (f *Foreign) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// ForeignPlacement is one region and where it ended up, in surface
// coordinates: what a host needs to position its object over the canvas.
type ForeignPlacement struct {
	Key     string
	Kind    string
	Content string

	// Rect is where the region wants to be.
	Rect Rect

	// Clip is the part of Rect an enclosing viewport still shows. A region
	// scrolled halfway out of a ScrollView reports the visible half here; one
	// scrolled entirely out reports an empty Clip. A host that ignores Clip
	// paints its object over the widgets around the viewport, which is the
	// single most visible way an overlay betrays that it is not really in the
	// tree.
	Clip Rect

	// Visible is false when Clip is empty — the region is laid out but nothing
	// of it is on screen. A host should hide its object rather than destroy
	// it: the region usually comes back on the next scroll.
	Visible bool
}

// WalkForeign returns every [Foreign] region in the tree rooted at w, with its
// placement and the clip an enclosing viewport imposes, in visual order.
//
// It descends through the same childContainer / childOffsetter convention
// [WalkA11y] uses, so a host composes its tree normally and does not keep a
// flat list of its regions. The clip is the intersection of the bounds of every
// enclosing viewport — a widget that offsets its children is one that scrolls
// them, and a scroller shows only what its own rectangle covers.
func WalkForeign(w Widget) []ForeignPlacement {
	var out []ForeignPlacement
	var walk func(x Widget, dx, dy int, clip Rect, clipped bool)
	walk = func(x Widget, dx, dy int, clip Rect, clipped bool) {
		if x == nil {
			return
		}
		if fw, ok := x.(*Foreign); ok {
			r := fw.Bounds()
			r.X, r.Y = r.X+dx, r.Y+dy
			c := r
			if clipped {
				c = intersectRect(r, clip)
			}
			out = append(out, ForeignPlacement{
				Key: fw.Key, Kind: fw.Kind, Content: fw.Content,
				Rect: r, Clip: c, Visible: c.W > 0 && c.H > 0,
			})
		}
		if o, ok := x.(childOffsetter); ok {
			// A widget that offsets its children scrolls them, and shows only
			// what its own rectangle covers.
			vp := x.Bounds()
			if cc, ok := x.(childClipper); ok {
				// A viewport that reserves room for scrollbars shows its child in
				// less than its own bounds, and says so.
				vp = cc.ChildClip()
			}
			vp.X, vp.Y = vp.X+dx, vp.Y+dy
			if clipped {
				clip = intersectRect(clip, vp)
			} else {
				clip, clipped = vp, true
			}
			ox, oy := o.ChildOffset()
			dx, dy = dx+ox, dy+oy
		}
		if c, ok := x.(childContainer); ok {
			for _, child := range c.Children() {
				walk(child, dx, dy, clip, clipped)
			}
		}
	}
	walk(w, 0, 0, Rect{}, false)
	return out
}

// childClipper is implemented by a widget that shows its children in a
// rectangle SMALLER than its own bounds — a viewport reserving room for
// scrollbars. A widget that offsets its children without implementing this is
// taken to show them across its whole rectangle.
type childClipper interface{ ChildClip() Rect }
