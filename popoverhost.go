// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// PopoverOwner is the capability of a widget part of which is drawn OUTSIDE its own
// bounds and on top of everything else: [DropDown]'s option list is the one in
// the package today.
//
// Such a widget cannot draw that part itself. Every widget draws inside its
// bounds and in tree order, so a list painted from the control's own Draw would
// be overpainted by whatever comes after it in the tree, and its bounds would
// not cover it in the first place. The drawing and the click routing therefore
// belong to whoever owns the surface — and [PopoverHost] is that owner, so no
// application has to be.
type PopoverOwner interface {
	Widget
	// PopoverOpen reports whether the floating part is showing.
	PopoverOpen() bool
	// PopoverBounds is where it goes, in surface coordinates.
	PopoverBounds() Rect
	// DrawPopover paints it. It is called after the whole tree, so it lands on
	// top of everything.
	DrawPopover(p painter.Painter, theme *Theme)
	// PopoverClick routes a click while it is open, in the same coordinates as
	// PopoverBounds. It reports whether it consumed the click.
	PopoverClick(x, y int) bool
}

// PopoverHost is the piece of a host every application was expected to write.
//
// A [DropDown] is a complete widget: it opens on a click, moves with the arrow
// keys, commits on Enter, scrolls a long list, and knows exactly where its
// option list goes and how to paint it. What it cannot do is get that list onto
// the surface, because a widget draws inside its own bounds and in tree order.
// The four methods above are the hand-over, and until now every host had to
// write the other half itself: walk the tree, find the open ones, draw them
// after everything else, and offer them the click before hit-testing.
//
// Nobody did. The measured result was a control that opened onto nothing: the
// chevron worked, the list never appeared, and no option could be chosen — the
// widget behaving exactly as documented while being useless in every
// application that placed one. A capability that needs a page of host code to
// work is a capability nobody has.
//
// So the other half lives here, once. Wrap the root:
//
//	win.Run(toolkit.NewPopoverHost(root))
//
// and every DropDown anywhere in the tree works, now and after the tree changes
// shape. The host must be the ROOT: it draws over the whole surface and routes
// in surface coordinates, which is the frame [PopoverBounds] is expressed in.
type PopoverHost struct {
	Base
	// Content is the real tree. It fills the host's bounds.
	Content Widget
}

// NewPopoverHost wraps content, which may be nil.
func NewPopoverHost(content Widget) *PopoverHost { return &PopoverHost{Content: content} }

// SetBounds gives the whole surface to Content: the host is a wrapper and takes
// no room of its own.
func (h *PopoverHost) SetBounds(r Rect) {
	h.Base.SetBounds(r)
	if h.Content != nil {
		h.Content.SetBounds(r)
	}
}

// Children yields Content, so accessibility, text selection, animation and every
// other generic walk descends through the host as if it were not there.
func (h *PopoverHost) Children() []Widget { return nonNil(h.Content) }

// focusableChildren yields Content so the focus walker reaches through the host.
func (h *PopoverHost) focusableChildren() []Widget { return nonNil(h.Content) }

// A11y reports the host as presentational: it is plumbing, not content.
func (h *PopoverHost) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// Draw paints Content, then every open popover in the tree, in tree order, on
// top of it.
//
// After the whole tree and not merely after its own subtree: a popover opening
// downwards out of a control near the top of a form covers widgets that come
// later in the tree, and a list drawn before them would be painted over by the
// very widgets it is meant to cover.
func (h *PopoverHost) Draw(p painter.Painter, theme *Theme) {
	if h.Content == nil {
		return
	}
	h.Content.Draw(p, theme)
	for _, pop := range PopoverOwners(h.Content) {
		pop.DrawPopover(p, theme)
	}
}

// HitTest covers the host's whole rectangle, including wherever a popover is
// showing: the floating part is outside its control's bounds, so a host that
// hit-tested only its children would report the option list as empty surface.
func (h *PopoverHost) HitTest(px, py int) bool { return h.Bounds().Contains(px, py) }

// OnEvent offers a click to any open popover BEFORE the tree sees it, and
// forwards everything else to Content.
//
// The order is the whole point. An open option list sits over other widgets, and
// those widgets are still where they were: routed normally, a click meant for
// the third option would land on whatever the list happens to cover. So an open
// popover is asked first, and the click stops there when it is consumed —
// selecting an option, or closing the list because the click was elsewhere.
//
// A wheel notch over an open list scrolls the list rather than the form behind
// it, for the same reason.
func (h *PopoverHost) OnEvent(ev Event) {
	if h.Content == nil {
		return
	}
	switch ev.Kind {
	case EventClick:
		// Topmost first: later in the tree is drawn later, so it is on top.
		open := PopoverOwners(h.Content)
		for i := len(open) - 1; i >= 0; i-- {
			if open[i].PopoverClick(ev.X+h.Bounds().X, ev.Y+h.Bounds().Y) {
				return
			}
		}
	case EventScroll:
		for _, pop := range PopoverOwners(h.Content) {
			if pop.PopoverBounds().Contains(ev.X+h.Bounds().X, ev.Y+h.Bounds().Y) {
				pop.OnEvent(ev)
				return
			}
		}
	}
	h.Content.OnEvent(translateEvent(ev, h.Bounds(), h.Content.Bounds()))
}

// Popovers gathers every [PopoverOwner] in the tree rooted at w whose floating part
// is currently showing, in tree order — so the last one returned is the one
// drawn on top.
//
// It descends through [childContainer] exactly like [TickTree] and [CollectRuns].
// A host that composes its own surface (a compositor drawing popovers onto a
// separate layer, say) uses this directly instead of [PopoverHost]; everything
// else wraps its root and forgets about it.
func PopoverOwners(w Widget) []PopoverOwner {
	var out []PopoverOwner
	collectPopoverOwners(w, &out)
	return out
}

func collectPopoverOwners(w Widget, out *[]PopoverOwner) {
	if w == nil {
		return
	}
	if p, ok := w.(PopoverOwner); ok && p.PopoverOpen() {
		*out = append(*out, p)
	}
	if c, ok := w.(childContainer); ok {
		for _, kid := range c.Children() {
			collectPopoverOwners(kid, out)
		}
	}
}
