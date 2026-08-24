// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DockPanel composes an [AppDock] launcher bar with optional accessory widgets
// pinned at its leading (left) and trailing (right) ends, plus an optional
// right-click context menu — the shell chrome a desktop dock needs around the
// bare item run.
//
// The launcher bar hosts only its own Items; a shell that also wants a workspace
// Pager, a Clock, an attention Badge or a plain Label on the dock used to
// hand-compose a scene around the AppDock (measuring the ends, laying the
// accessories out, forwarding events, popping a menu on right-click). DockPanel
// is that composition made shared and data-driven: drop the accessories into
// Leading / Trailing, hand it a Menu, and the panel lays everything out, routes
// events to the right widget and confines the AppDock's hover magnification to
// its own run so a swelling item never paints over — or steals a click from — an
// accessory.
//
// Layout mirrors [HeaderBar]: Leading widgets run left-to-right from the left
// edge, Trailing widgets right-to-left from the right edge (Trailing[0] is the
// rightmost), each keeping its own width and fitted to the bar's height; the
// AppDock fills whatever horizontal span is left between the two groups. With no
// accessories the AppDock is given the panel's exact bounds, so a DockPanel then
// renders byte-for-byte identical to a standalone AppDock.
//
// The AppDock is painted CLIPPED to its own run, so a magnified item that
// overflows its bounds is cut at the run's edge instead of spilling onto an
// accessory; and events are offered to the accessories BEFORE the dock, so an
// overflowing item can never shadow an accessory's click either.
type DockPanel struct {
	Base
	// Dock is the launcher bar this panel wraps. Its bounds are managed by the
	// panel (set to the span between the accessory groups); a nil Dock lays out
	// the accessories alone.
	Dock *AppDock
	// Leading are accessory widgets pinned at the panel's leading (left) end,
	// laid out left-to-right. Set-once composition config; each is a real toolkit
	// widget that receives events and appears in the accessibility tree.
	Leading []Widget
	// Trailing are accessory widgets pinned at the panel's trailing (right) end,
	// laid out right-to-left (Trailing[0] is the rightmost). Set-once config, same
	// as Leading.
	Trailing []Widget
	// Menu, when set, is the context menu opened at the pointer on a secondary
	// (right / two-finger) click anywhere over the bar. It is a [ContextMenu] —
	// the toolkit's right-click overlay — so it owns its own open state, clamps
	// itself inside its surface and dismisses on an outside click; the host sizes
	// its Bounds to the surface it may cover, exactly as with a standalone
	// ContextMenu. Nil disables the menu.
	Menu *ContextMenu
}

// NewDockPanel wraps dock as a DockPanel with no accessories and no menu; the
// caller fills Leading / Trailing / Menu before the first Draw.
func NewDockPanel(dock *AppDock) *DockPanel { return &DockPanel{Dock: dock} }

// A11y reports the panel as presentational: it is pure layout chrome, so a
// screen reader looks THROUGH it to the AppDock (a toolbar) and the accessory
// widgets it hosts, exactly as it looks through an HBox or a Border.
func (d *DockPanel) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// SetBounds positions the panel and lays out its dock + accessories so their
// bounds are correct before the first paint (and before any OnEvent dispatch).
func (d *DockPanel) SetBounds(r Rect) {
	d.Base.SetBounds(r)
	d.layout()
}

// dockGap is the padding at the panel's ends and the gap between adjacent
// accessories (and between an accessory group and the dock). It reuses the
// dock's own item gap so the accessories sit on the same rhythm as the items.
func (d *DockPanel) dockGap() int { return scaled(AppDockGap) }

// layout places every Leading / Trailing accessory and sizes the Dock to the
// span left between the two groups. Each accessory keeps its own Bounds.W and is
// fitted to the panel's full height; the AppDock is given the middle span
// (clamped to a non-negative width). With no accessories the dock's span is the
// panel's exact bounds — the byte-identical back-compat case.
func (d *DockPanel) layout() {
	r := d.Bounds()
	gap := d.dockGap()

	// Leading group: left-to-right from the left edge, a gap.
	leadEnd := r.X
	first := true
	for _, w := range d.Leading {
		if w == nil {
			continue
		}
		if first {
			leadEnd += gap
			first = false
		}
		wb := w.Bounds()
		w.SetBounds(Rect{X: leadEnd, Y: r.Y, W: wb.W, H: r.H})
		leadEnd += wb.W + gap
	}

	// Trailing group: right-to-left from the right edge (Trailing[0] rightmost).
	trailStart := r.X + r.W
	first = true
	for _, w := range d.Trailing {
		if w == nil {
			continue
		}
		if first {
			trailStart -= gap
			first = false
		}
		wb := w.Bounds()
		trailStart -= wb.W
		w.SetBounds(Rect{X: trailStart, Y: r.Y, W: wb.W, H: r.H})
		trailStart -= gap
	}

	if d.Dock != nil {
		w := trailStart - leadEnd
		if w < 0 {
			w = 0
		}
		d.Dock.SetBounds(Rect{X: leadEnd, Y: r.Y, W: w, H: r.H})
	}
}

// Draw paints the dock (clipped to its run so an overflowing magnified item is
// cut at the run's edge rather than spilling onto an accessory), then every
// accessory on top, then the context menu (which paints only while open).
func (d *DockPanel) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	d.layout()
	if d.Dock != nil {
		withClip(p, d.Dock.Bounds(), func() { d.Dock.Draw(p, theme) })
	}
	for _, w := range d.Leading {
		if w != nil {
			w.Draw(p, theme)
		}
	}
	for _, w := range d.Trailing {
		if w != nil {
			w.Draw(p, theme)
		}
	}
	if d.Menu != nil {
		d.Menu.Draw(p, theme)
	}
}

// OnEvent routes a widget-local event. While the context menu is open every
// event goes to it (so a row-click fires and an outside-click dismisses). A
// secondary click over the bar opens the menu at the pointer. Otherwise a move
// is broadcast to the accessories (hover) and the dock (magnify), and any other
// event is offered to the accessories FIRST — so an overflowing magnified item
// can never steal an accessory's click — then to the dock.
func (d *DockPanel) OnEvent(ev Event) {
	d.layout()
	pr := d.Bounds()

	if d.Menu != nil && d.Menu.Open().Get() {
		d.Menu.OnEvent(translateEvent(ev, pr, d.Menu.Bounds()))
		return
	}
	if ev.Kind == EventSecondaryClick {
		if d.Menu != nil && d.localInBounds(ev.X, ev.Y) {
			me := translateEvent(ev, pr, d.Menu.Bounds())
			d.Menu.Popup(me.X, me.Y)
		}
		return
	}
	if ev.Kind == EventMouseMove {
		for _, w := range d.Leading {
			if w != nil {
				w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			}
		}
		for _, w := range d.Trailing {
			if w != nil {
				w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			}
		}
		if d.Dock != nil {
			d.Dock.OnEvent(translateEvent(ev, pr, d.Dock.Bounds()))
		}
		return
	}

	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, w := range d.Leading {
		if w != nil && w.HitTest(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
	for _, w := range d.Trailing {
		if w != nil && w.HitTest(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
	if d.Dock != nil && d.Dock.HitTest(sx, sy) >= 0 {
		d.Dock.OnEvent(translateEvent(ev, pr, d.Dock.Bounds()))
	}
}

// appDockNode adapts an [AppDock] — whose HitTest returns an item INDEX, not a
// bool, so the type does not itself satisfy [Widget] — into a Widget, purely so
// the accessibility walk ([WalkA11y] / [Children]) reaches the dock's toolbar
// node and reports it at the dock's bounds. Bounds/A11y mirror the dock; Draw
// and OnEvent forward to it; HitTest collapses the item index to "over an item".
// It carries no state of its own, so a fresh value per Children call is free.
type appDockNode struct{ d *AppDock }

func (n appDockNode) Bounds() Rect                     { return n.d.Bounds() }
func (n appDockNode) SetBounds(r Rect)                 { n.d.SetBounds(r) }
func (n appDockNode) Draw(p painter.Painter, t *Theme) { n.d.Draw(p, t) }
func (n appDockNode) HitTest(px, py int) bool          { return n.d.HitTest(px, py) >= 0 }
func (n appDockNode) OnEvent(ev Event)                 { n.d.OnEvent(ev) }
func (n appDockNode) A11y() A11yInfo                   { return n.d.A11y() }
