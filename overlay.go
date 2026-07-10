// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Overlay is a z-ordered stacking container: a primary Content child that fills
// the bounds, plus a stack of Layers painted on top of it in order (the last
// Layer is topmost). It is the piece the widget model was missing so transient
// widgets -- Popover, Toast, Notification, Tooltip, ContextMenu -- can float
// above the main UI without the host arranging screen positions or z-order.
//
// Unlike Stack (which shows exactly one page at a time), an Overlay draws every
// layer, and events route top-down: the topmost Layer whose HitTest covers the
// point handles the event; if none do, the event falls through to Content --
// unless Modal is set, in which case a miss while any Layer is up is swallowed
// (a modal backdrop). Layers self-position via their own Bounds; only Content
// is resized to fill the Overlay.
type Overlay struct {
	Base
	Content Widget
	Layers  []Widget
	Modal   bool
}

// NewOverlay builds an Overlay around the given primary child (which may be nil
// and set later).
func NewOverlay(content Widget) *Overlay { return &Overlay{Content: content} }

// Push adds w as the new topmost layer.
func (o *Overlay) Push(w Widget) { o.Layers = append(o.Layers, w) }

// Pop removes and returns the topmost layer, or nil when there are none.
func (o *Overlay) Pop() Widget {
	if len(o.Layers) == 0 {
		return nil
	}
	top := o.Layers[len(o.Layers)-1]
	o.Layers = o.Layers[:len(o.Layers)-1]
	return top
}

// Top returns the topmost layer without removing it, or nil when there are none.
func (o *Overlay) Top() Widget {
	if len(o.Layers) == 0 {
		return nil
	}
	return o.Layers[len(o.Layers)-1]
}

// Clear removes every layer, leaving just the Content.
func (o *Overlay) Clear() { o.Layers = nil }

// SetBounds resizes Content to fill the Overlay; layers keep their own bounds
// (they self-position at a point).
func (o *Overlay) SetBounds(r Rect) {
	o.Base.SetBounds(r)
	if o.Content != nil {
		o.Content.SetBounds(r)
	}
}

// Draw paints Content first, then each layer bottom-to-top.
func (o *Overlay) Draw(p painter.Painter, theme *Theme) {
	if o.Content != nil {
		o.Content.Draw(p, theme)
	}
	for _, l := range o.Layers {
		l.Draw(p, theme)
	}
}

// OnEvent routes to the topmost layer whose HitTest covers the point; failing
// that, to Content -- or, when Modal and a layer is present, nowhere (the
// backdrop swallows the click). Coordinates are passed through unchanged (an
// Overlay is a surface-frame container, like Paned).
func (o *Overlay) OnEvent(ev Event) {
	for i := len(o.Layers) - 1; i >= 0; i-- {
		if o.Layers[i].HitTest(ev.X, ev.Y) {
			o.Layers[i].OnEvent(ev)
			return
		}
	}
	if o.Modal && len(o.Layers) > 0 {
		return
	}
	if o.Content != nil {
		o.Content.OnEvent(ev)
	}
}
