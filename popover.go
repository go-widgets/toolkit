// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Popover is a Visible floating container for a single child widget,
// modelled on GTK 4's Popover -- a rectangular panel with a border
// stroke and an optional Title header. Popover is the natural home for
// dropdown menus, ephemeral pickers and detail overlays that the host
// wants to show and hide without tearing down and rebuilding the
// underlying child.
//
// Distinct from Card (a passive display container) in two ways:
//
//  1. Popover has a Visible toggle: the whole widget short-circuits
//     Draw + OnEvent when hidden so the host does not have to unlink
//     the child from the tree between showings.
//  2. Popover forwards input events to its child with coordinates
//     translated for the pad + optional title header, so the child
//     sees widget-local coords in its own frame.
type Popover struct {
	Base
	Visible bool
	Child   Widget
	Title   string
}

// Popover sizing constants. PopoverPadX / PopoverPadY are the inner
// margin between the Popover's outer edge and the child's frame;
// PopoverBorderR is the border stroke width, matching what strokeRect
// paints so a host that lays out around the popover can budget the
// right number of pixels for the frame.
const (
	PopoverPadX    = 8
	PopoverPadY    = 6
	PopoverBorderR = 1
)

// NewPopover constructs a hidden Popover wrapping child. child may be
// nil, in which case the Popover renders as an empty framed panel.
func NewPopover(child Widget) *Popover { return &Popover{Child: child} }

// headerH returns the vertical space consumed by the Title strip.
// Zero when Title is empty so the child sits flush against the top pad.
func (p *Popover) headerH() int {
	if p.Title == "" {
		return 0
	}
	return GlyphHeight() + PopoverPadY
}

// childRect returns the surface-coordinate rect the child widget
// occupies. Insets both axes by PopoverPad + drops the top by the
// header strip height when Title is present.
func (p *Popover) childRect() Rect {
	r := p.Bounds()
	h := p.headerH()
	return Rect{
		X: r.X + PopoverPadX,
		Y: r.Y + PopoverPadY + h,
		W: r.W - 2*PopoverPadX,
		H: r.H - 2*PopoverPadY - h,
	}
}

// Draw paints the surface fill + border, optionally draws the Title
// at the top-left inside PopoverPad, then draws Child (if non-nil)
// into the inset child rect. Nothing drawn when !Visible.
func (p *Popover) Draw(pnt painter.Painter, theme *Theme) {
	if !p.Visible {
		return
	}
	r := p.Bounds()
	fillRect(pnt, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(pnt, r.X, r.Y, r.W, r.H, theme.Border)
	if p.Title != "" {
		DrawText(pnt, r.X+PopoverPadX, r.Y+PopoverPadY, p.Title, theme.OnSurface)
	}
	if p.Child != nil {
		p.Child.SetBounds(p.childRect())
		p.Child.Draw(pnt, theme)
	}
}

// OnEvent forwards the event to Child with coordinates translated
// into the child's local frame. No-op when !Visible or Child is nil.
// Mirrors the translateEvent pattern used by HBox / VBox / Grid.
func (p *Popover) OnEvent(ev Event) {
	if !p.Visible || p.Child == nil {
		return
	}
	p.Child.OnEvent(translateEvent(ev, p.Bounds(), p.Child.Bounds()))
}
