// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-iconoir/iconoir"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Dialog is a modal overlay: a centred Surface card with an optional
// Title bar, a Content widget filling the body, and an action-button
// strip at the bottom. The compositor draws a semi-darkened backdrop
// over the rest of the surface so the user's attention focuses on
// the dialog.
//
// v0.3 ships the structure; the host app is responsible for routing
// input events only to the dialog while it's open (existing wasmbox
// modal-grab behaviour). A self-contained modal that owns its own scrim,
// centring, click-outside + Escape dismissal is [ModalWindow], which
// composes a Dialog as its centred panel.
//
// Two features are optional and off by default, so an existing Dialog is
// byte-identical to before:
//
//   - Closable adds a close (×) button at the trailing edge of the title
//     bar, wired to OnClose.
//   - Input adds a single-line input bar in a strip directly below the
//     title bar (a search / filter field); the Content body starts below
//     it. Any [DialogInput] fits — a SearchEntry (the search-modal
//     default) or an Entry (for a Placeholder hint).
type Dialog struct {
	Base
	Title   string
	Content Widget
	Buttons []*Button
	OnClose func()

	// Closable, when true, shows a close (×) button at the trailing edge of the
	// title bar wired to OnClose. The zero value (false) keeps the title bar
	// exactly as before — no close control.
	Closable bool

	// Input, when non-nil, is a single-line input bar drawn in a strip directly
	// below the title bar; the Content body starts below it, and keyboard input
	// plus a click on the strip route to the field. Nil (the default) draws no
	// input bar and the body fills the whole area between the title bar and the
	// button strip, byte-identical to before. Its text is a shared observable —
	// read/bind it via Input.Text().
	Input DialogInput

	// DragBounds, when its W and H are both positive, is the area the panel is
	// kept inside while the user drags it by the title bar: the panel origin is
	// clamped so the whole card (and thus the title bar) stays reachable within
	// it. The zero value (an empty rect) leaves a title-bar drag unclamped — the
	// host is then responsible for any bounds. [ModalWindow] sets this to its own
	// bounds (the whole surface) so a modal's panel cannot be dragged off-screen.
	DragBounds Rect

	// closeBtn is the lazily-built close (×) control; see closeButton.
	closeBtn *IconButton

	// Title-bar drag state, following the toolkit's grab-on-EventClick /
	// track-on-EventMouseDrag / release-on-EventMouseUp convention (as Paned's
	// handle and Kanban's cards do). dragging is true between a press on the
	// title bar and the release; dragLastX/Y is the last pointer position in
	// absolute (surface) coordinates, so the per-tick delta is immune to the
	// panel itself moving under the pointer mid-drag.
	dragging             bool
	dragLastX, dragLastY int

	// baseRect is the layout-requested rect from the most recent SetBounds,
	// before the drag offset is applied. dragOffX/Y is the accumulated title-bar
	// drag displacement, re-applied over baseRect on every layout. Keeping the
	// requested origin and the user displacement apart is what makes a drag
	// authoritative: a container that re-centres or a host that re-anchors the
	// panel each frame (calling SetBounds with the same origin) sets baseRect,
	// and the offset is laid back over it, so the panel holds where it was
	// dragged instead of snapping back.
	baseRect           Rect
	dragOffX, dragOffY int
}

// DialogInput is the contract a Dialog's optional top input bar satisfies: a
// single-line widget exposing its text as a shared [mvvm.Observable]. Both
// [Entry] (which also carries a Placeholder) and [SearchEntry] (a search-prefix
// glyph + clear affordance) implement it, so either drops in as the input bar.
type DialogInput interface {
	Widget
	// Text is the field's current value as a shared observable a host binds or
	// subscribes to.
	Text() *mvvm.Observable[string]
}

// DialogTitleH is the pixel height of the title bar.
const DialogTitleH = 28

// DialogButtonStripH is the pixel height of the bottom action strip.
const DialogButtonStripH = 32

// DialogButtonW is the width allocated per action button.
const DialogButtonW = 90

// DialogInputH is the pixel height of the optional input-bar strip below the
// title bar (present only when Input is set).
const DialogInputH = 32

// dialogInputPad is the inset in logical pixels between the input strip's edges
// and the input field drawn inside it.
const dialogInputPad = 4

// DialogRadius is the panel's corner radius in pixels before scaling, and
// DialogShadow the offset of its drop shadow.
//
// A dialog is a sheet floating over the application, and a square-cornered sheet
// reads as a region of it. But rounding ALONE is not enough to say so: measured
// on the live playground, an 8px radius against a dark scrim showed the scrim
// through the corner at [16,18,21] where the edge read [58,62,70] — present, and
// invisible. A sheet reads as floating when it is rounded AND it casts a shadow,
// which is what the page cards in PagedView already do.
const (
	DialogRadius = 12
	DialogShadow = 4
)

// dialogShadowColor is the translucent black a panel casts onto what is under
// it — the same value a page card uses.
var dialogShadowColor = RGBA{R: 0, G: 0, B: 0, A: 0x30}

// closeButton lazily builds (and caches) the title-bar close control, wired to
// fire OnClose live at click time (nil-safe). Only ever consulted when Closable.
func (d *Dialog) closeButton() *IconButton {
	if d.closeBtn == nil {
		d.closeBtn = NewIconButton("", func() {
			if d.OnClose != nil {
				d.OnClose()
			}
		})
		// A real ✕ from the icon set the org already owns, not the letter "x"
		// that stood in for one because the old 5x7 bitmap font had no glyph for
		// it. And flat: a framed square in the corner of a title bar reads as a
		// control belonging to the content rather than to the window.
		d.closeBtn.Glyph = func(p painter.Painter, r Rect, ink RGBA) {
			iconoir.Draw(p, r, "xmark", ink)
		}
		d.closeBtn.Flat = true
	}
	return d.closeBtn
}

// NewDialog builds a Dialog with the given title, content + action
// buttons. Buttons are laid out right-aligned in the bottom strip.
func NewDialog(title string, content Widget, buttons ...*Button) *Dialog {
	return &Dialog{Title: title, Content: content, Buttons: buttons}
}

// SetBounds records the layout-requested rect, then lays the panel and its
// children out at that rect with any accumulated title-bar drag offset applied
// on top (see applyBounds). Storing the requested origin apart from the drawn
// origin is what lets a drag survive a re-centring / re-anchoring relayout: a
// container or host that re-calls SetBounds with the same origin every frame
// only sets the base, and the persistent offset is re-applied over it.
func (d *Dialog) SetBounds(r Rect) {
	d.baseRect = r
	d.applyBounds()
}

// applyBounds lays the panel out at baseRect shifted by the drag offset and
// clamped within DragBounds, then positions the input bar, close button,
// content and action buttons relative to that final rect. With a zero drag
// offset — and a DragBounds the panel already fits inside, which [ModalWindow]
// guarantees by clamping the panel size to its bounds — the final rect equals
// the requested rect, so an undragged Dialog lays out byte-identically to
// before this method existed.
func (d *Dialog) applyBounds() {
	r := d.baseRect
	r.X += d.dragOffX
	r.Y += d.dragOffY
	if d.DragBounds.W > 0 && d.DragBounds.H > 0 {
		r.X = clampInt(r.X, d.DragBounds.X, maxInt(d.DragBounds.X, d.DragBounds.X+d.DragBounds.W-r.W))
		r.Y = clampInt(r.Y, d.DragBounds.Y, maxInt(d.DragBounds.Y, d.DragBounds.Y+d.DragBounds.H-r.H))
	}
	d.Base.SetBounds(r)
	// The body starts below the title bar, and below the input strip when one is
	// present. With no input bar `top` is r.Y+titleH, so the content rect below is
	// byte-identical to before this field existed.
	top := r.Y + scaled(DialogTitleH)
	if d.Input != nil {
		pad := scaled(dialogInputPad)
		d.Input.SetBounds(Rect{X: r.X + pad, Y: top + pad, W: r.W - 2*pad, H: scaled(DialogInputH) - 2*pad})
		top += scaled(DialogInputH)
	}
	if d.Content != nil {
		body := Rect{
			X: r.X,
			Y: top,
			W: r.W,
			H: r.Y + r.H - scaled(DialogButtonStripH) - top,
		}
		d.Content.SetBounds(body)
	}
	// Close (×) button: a square filling the title bar's trailing edge.
	if d.Closable {
		s := scaled(DialogTitleH)
		d.closeButton().SetBounds(Rect{X: r.X + r.W - s, Y: r.Y, W: s, H: s})
	}
	// Right-align the action buttons in the bottom strip.
	stripY := r.Y + r.H - scaled(DialogButtonStripH)
	bx := r.X + r.W - 8
	for i := len(d.Buttons) - 1; i >= 0; i-- {
		bx -= scaled(DialogButtonW)
		d.Buttons[i].SetBounds(Rect{X: bx, Y: stripY + 4, W: scaled(DialogButtonW) - 8, H: scaled(DialogButtonStripH) - 8})
		bx -= 8 // gap
	}
}

// Draw paints card + title + content + buttons.
func (d *Dialog) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	rad := scaled(DialogRadius)
	// The shadow first, offset down and right, so the sheet reads as lifted off
	// what is behind it.
	drop := scaled(DialogShadow)
	fillRoundRect(p, r.X+drop, r.Y+drop, r.W, r.H, rad, dialogShadowColor)
	fillRoundRect(p, r.X, r.Y, r.W, r.H, rad, theme.Background)
	// Title bar. It is filled as a round rect so it follows the panel's top
	// corners, then squared off along its lower half — filling it as a plain
	// rect would poke out past the rounding at both top corners.
	th := scaled(DialogTitleH)
	fillRoundRect(p, r.X, r.Y, r.W, th, rad, theme.SurfaceAlt)
	if th > rad {
		fillRect(p, r.X, r.Y+rad, r.W, th-rad, theme.SurfaceAlt)
	}
	// A hairline between the title bar and what is under it, so the two read as
	// separate bands rather than one field of colour.
	fillRect(p, r.X, r.Y+th-strokeWidth(), r.W, strokeWidth(), theme.Border)
	titleY := r.Y + (th-d.glyphHeight())/2
	d.drawText(p, r.X+8, titleY, d.Title, theme.OnSurface)
	// Close (×) button.
	if d.Closable {
		d.closeButton().Draw(p, theme)
	}
	// Optional input-bar strip below the title bar.
	if d.Input != nil {
		fillRect(p, r.X, r.Y+scaled(DialogTitleH), r.W, scaled(DialogInputH), theme.Background)
		d.Input.Draw(p, theme)
	}
	// Content.
	if d.Content != nil {
		d.Content.Draw(p, theme)
	}
	// Action strip, rounded along the panel's BOTTOM corners the same way the
	// title bar is rounded along the top.
	sh := scaled(DialogButtonStripH)
	stripY := r.Y + r.H - sh
	fillRoundRect(p, r.X, stripY, r.W, sh, rad, theme.SurfaceAlt)
	if sh > rad {
		fillRect(p, r.X, stripY, r.W, sh-rad, theme.SurfaceAlt)
	}
	fillRect(p, r.X, stripY, r.W, strokeWidth(), theme.Border)
	for _, b := range d.Buttons {
		b.Draw(p, theme)
	}
	// The panel outline last, so neither band paints over it.
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, rad, theme.Border)
}

// OnEvent forwards to the close button, action buttons, optional input bar and
// content, in that order. A click on the close (×) button fires OnClose; a click
// on the input strip focuses the field and forwards to it, and all keyboard
// input routes to the field when one is present (so a search/filter box can be
// typed into). A click that doesn't land on any of them falls through silently
// (the app keeps the dialog open).
func (d *Dialog) OnEvent(ev Event) {
	// Close (×) button.
	if d.Closable && ev.Kind == EventClick {
		cb := d.closeButton()
		if cb.Bounds().Contains(ev.X+d.Bounds().X, ev.Y+d.Bounds().Y) {
			cb.OnEvent(Event{Kind: EventClick})
			return
		}
	}
	for _, b := range d.Buttons {
		if b.Bounds().Contains(ev.X+d.Bounds().X, ev.Y+d.Bounds().Y) && ev.Kind == EventClick {
			b.OnEvent(Event{Kind: EventClick})
			return
		}
	}
	// Optional top input bar: a click on the strip focuses + forwards to it, and
	// all keyboard input routes to it so the field can be typed into.
	if d.Input != nil {
		switch ev.Kind {
		case EventClick:
			if d.Input.Bounds().Contains(ev.X+d.Bounds().X, ev.Y+d.Bounds().Y) {
				if f, ok := d.Input.(Focusable); ok {
					f.SetFocused(true)
				}
				d.Input.OnEvent(translateEvent(ev, d.Bounds(), d.Input.Bounds()))
				return
			}
		case EventChar, EventKeyDown:
			d.Input.OnEvent(translateEvent(ev, d.Bounds(), d.Input.Bounds()))
			return
		}
	}
	// Title-bar drag: a press on the title strip (never the × or the input bar,
	// both handled above) arms a drag; each following EventMouseDrag moves the
	// panel by the pointer delta, and EventMouseUp releases. None of this fires
	// unless a drag was armed on the title bar, so every other press-drag-release
	// — on content, buttons or the input bar — still forwards exactly as before.
	switch ev.Kind {
	case EventClick:
		if d.onTitleBar(ev) {
			d.dragging = true
			d.dragLastX = ev.X + d.Bounds().X
			d.dragLastY = ev.Y + d.Bounds().Y
			return
		}
	case EventMouseDrag:
		if d.dragging {
			ax, ay := ev.X+d.Bounds().X, ev.Y+d.Bounds().Y
			d.moveBy(ax-d.dragLastX, ay-d.dragLastY)
			d.dragLastX, d.dragLastY = ax, ay
			return
		}
	case EventMouseUp:
		if d.dragging {
			d.dragging = false
			return
		}
	}
	if d.Content != nil {
		// Content occupies the body between the title bar and the button strip
		// (bounded in SetBounds). Translate the event into its local frame — the
		// buttons above already reconstruct absolute coords, but the content was
		// forwarded raw, so a click on interactive content landed scaled(DialogTitleH)
		// too low (plus the Dialog's own origin).
		d.Content.OnEvent(translateEvent(ev, d.Bounds(), d.Content.Bounds()))
	}
}

// onTitleBar reports whether a widget-local event point falls on the draggable
// part of the title bar: the title strip, minus the trailing close (×) button
// when Closable. The optional input bar sits in its own strip below the title
// bar, so it is never in this zone.
func (d *Dialog) onTitleBar(ev Event) bool {
	if ev.Y < 0 || ev.Y >= scaled(DialogTitleH) {
		return false
	}
	rightLimit := d.Bounds().W
	if d.Closable {
		rightLimit -= scaled(DialogTitleH) // the square × button at the trailing edge
	}
	return ev.X >= 0 && ev.X < rightLimit
}

// moveBy accumulates a title-bar drag displacement into the persistent offset
// and re-lays the panel out, so the move is clamped within DragBounds and the
// new position survives the next relayout.
func (d *Dialog) moveBy(dx, dy int) {
	d.dragOffX += dx
	d.dragOffY += dy
	d.applyBounds()
}

// NewMessageDialog is a convenience constructor for the most common
// dialog: a title, a Label as content, and an OK button that calls
// onOK + closes the dialog via the caller's OnClose hook.
func NewMessageDialog(title, message string, onOK func()) *Dialog {
	ok := NewButton("OK", onOK)
	return NewDialog(title, NewLabel(message), ok)
}
