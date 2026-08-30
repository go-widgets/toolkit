// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
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

	// PlainTitle draws the title bar in the panel's Surface (with an OnSurface
	// title and a bottom hairline) instead of the accent fill, for a calmer
	// card-style modal that matches a hand-built Surface panel. The zero value
	// (false) keeps the accent bar, so existing dialogs are unchanged.
	PlainTitle bool

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

	// Minimisable and Maximisable add the two title-bar controls beside the
	// close one. Both default false: a Dialog is a sheet, and a sheet has one
	// thing to do with it — dismiss it. An application that wants a window says
	// so.
	Minimisable bool
	Maximisable bool

	// minimised / maximised are the two states, reactive via the accessors.
	// restore is the rect a maximised panel returns to.
	minimised, maximised *mvvm.Observable[bool]
	restoreOffX          int
	restoreOffY          int
	minBtn, maxBtn       *IconButton
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

// stripH is the height reserved for (and drawn as) the bottom action strip: zero
// when the dialog has no action Buttons, so a modal that keeps its controls inside
// Content is a clean card with no empty strip; otherwise DialogButtonStripH.
func (d *Dialog) stripH() int {
	if len(d.Buttons) == 0 {
		return 0
	}
	return scaled(DialogButtonStripH)
}

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
		d.closeBtn = d.titleControl(func() {
			if d.OnClose != nil {
				d.OnClose()
			}
		})
		// A real ✕ from the icon set the org already owns, not the letter "x"
		// that stood in for one because the old 5x7 bitmap font had no glyph for
		// it. And flat: a framed square in the corner of a title bar reads as a
		// control belonging to the content rather than to the window.
		d.closeBtn.Glyph = func(p painter.Painter, r Rect, ink RGBA) {
			DrawIconoir(p, r, "xmark", ink)
		}
	}
	return d.closeBtn
}

// Minimised is the rolled-up state as a shared [mvvm.Observable]: a minimised
// panel shows its title bar and nothing else, so the window is out of the way
// without being dismissed. Lazily created.
func (d *Dialog) Minimised() *mvvm.Observable[bool] {
	if d.minimised == nil {
		d.minimised = mvvm.NewObservable(false)
	}
	return d.minimised
}

// Maximised is the filled state as a shared [mvvm.Observable]: a maximised panel
// takes its whole DragBounds and returns to where it was when un-maximised.
// Lazily created.
func (d *Dialog) Maximised() *mvvm.Observable[bool] {
	if d.maximised == nil {
		d.maximised = mvvm.NewObservable(false)
	}
	return d.maximised
}

// titleButtons are the title-bar controls in TRAILING-EDGE order — the one
// nearest the corner first — so layout and hit-testing walk the same list and
// cannot disagree about which square belongs to which control.
func (d *Dialog) titleButtons() []*IconButton {
	var out []*IconButton
	if d.Closable {
		out = append(out, d.closeButton())
	}
	if d.Maximisable {
		out = append(out, d.maxButton())
	}
	if d.Minimisable {
		out = append(out, d.minButton())
	}
	return out
}

// maxButton toggles Maximised. The glyph follows the state: a panel that fills
// its bounds offers to shrink back.
func (d *Dialog) maxButton() *IconButton {
	if d.maxBtn == nil {
		d.maxBtn = d.titleControl(func() { d.Maximised().Set(!d.Maximised().Get()); d.applyBounds() })
	}
	name := "expand"
	if d.Maximised().Get() {
		name = "collapse"
	}
	d.maxBtn.Glyph = func(p painter.Painter, r Rect, ink RGBA) { DrawIconoir(p, r, name, ink) }
	return d.maxBtn
}

// minButton toggles Minimised.
func (d *Dialog) minButton() *IconButton {
	if d.minBtn == nil {
		d.minBtn = d.titleControl(func() { d.Minimised().Set(!d.Minimised().Get()); d.applyBounds() })
	}
	name := "minus"
	if d.Minimised().Get() {
		name = "plus"
	}
	d.minBtn.Glyph = func(p painter.Painter, r Rect, ink RGBA) { DrawIconoir(p, r, name, ink) }
	return d.minBtn
}

// titleControl builds one flat icon button for the title bar.
func (d *Dialog) titleControl(onClick func()) *IconButton {
	b := NewIconButton("", onClick)
	b.Flat = true
	return b
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
		if d.Maximised().Get() {
			// Filled: the whole area the panel is allowed to occupy. Without
			// DragBounds there is nothing to fill, so maximising does nothing
			// rather than guessing at a size.
			r = d.DragBounds
		} else {
			r.X = clampInt(r.X, d.DragBounds.X, maxInt(d.DragBounds.X, d.DragBounds.X+d.DragBounds.W-r.W))
			r.Y = clampInt(r.Y, d.DragBounds.Y, maxInt(d.DragBounds.Y, d.DragBounds.Y+d.DragBounds.H-r.H))
		}
	}
	if d.Minimised().Get() {
		// Rolled up to its title bar: out of the way without being dismissed.
		r.H = scaled(DialogTitleH)
	}
	d.Base.SetBounds(r)
	d.layoutTitleButtons(r)
	if d.Minimised().Get() {
		return // nothing below the title bar to place
	}
	// The body starts below the title bar, and below the input strip when one is
	// present. With no input bar `top` is r.Y+titleH, so the content rect below is
	// byte-identical to before this field existed.
	top := r.Y + scaled(DialogTitleH)
	if d.Input != nil {
		pad := scaled(dialogInputPad)
		d.Input.SetBounds(Rect{X: r.X + pad, Y: top + pad, W: r.W - 2*pad, H: scaled(DialogInputH) - 2*pad})
		top += scaled(DialogInputH)
	}
	sh := d.stripH()
	if d.Content != nil {
		body := Rect{
			X: r.X,
			Y: top,
			W: r.W,
			H: r.Y + r.H - sh - top,
		}
		d.Content.SetBounds(body)
	}
	// Right-align the action buttons in the bottom strip (none when Buttons is
	// empty — the modal then holds its own controls inside Content).
	stripY := r.Y + r.H - sh
	bx := r.X + r.W - 8
	for i := len(d.Buttons) - 1; i >= 0; i-- {
		bx -= scaled(DialogButtonW)
		d.Buttons[i].SetBounds(Rect{X: bx, Y: stripY + 4, W: scaled(DialogButtonW) - 8, H: scaled(DialogButtonStripH) - 8})
		bx -= 8 // gap
	}
}

// layoutTitleButtons places the title-bar controls as squares along the trailing
// edge, the closest to the corner first.
func (d *Dialog) layoutTitleButtons(r Rect) {
	sq := scaled(DialogTitleH)
	x := r.X + r.W
	for _, b := range d.titleButtons() {
		x -= sq
		b.SetBounds(Rect{X: x, Y: r.Y, W: sq, H: sq})
	}
}

// titleBarInk is what a title bar's text and controls are drawn in. The bar is
// filled with the theme's accent, so the ink is the colour the toolkit already
// uses ON the accent — the face a pressed button shows its glyph against.
func titleBarInk(theme *Theme) RGBA { return theme.Background }

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
	// The bar carries the theme's ACCENT: it is what tells a window from the
	// content under it at a glance, and it is the surface a drag starts on, so
	// it should look like a thing to grab. PlainTitle instead keeps the bar in the
	// panel's Surface with a bottom hairline — a calmer card look.
	barFill, titleInk := theme.Accent, titleBarInk(theme)
	if d.PlainTitle {
		barFill, titleInk = theme.Surface, theme.OnSurface
	}
	fillRoundRect(p, r.X, r.Y, r.W, th, rad, barFill)
	if th > rad && !d.Minimised().Get() {
		fillRect(p, r.X, r.Y+rad, r.W, th-rad, barFill)
	}
	if d.PlainTitle && !d.Minimised().Get() {
		// The Surface bar does not separate itself from the body, so restore the
		// hairline the accent used to stand in for.
		fillRect(p, r.X, r.Y+th-scaled(1), r.W, scaled(1), theme.Border)
	}
	titleY := r.Y + (th-d.glyphHeight())/2
	d.drawText(p, r.X+8, titleY, d.Title, titleInk)
	// The title-bar controls take the same ink as the title text, so they read on
	// either the accent bar (Background) or the PlainTitle Surface bar (OnSurface).
	barTheme := *theme
	barTheme.OnSurface = titleInk
	for _, b := range d.titleButtons() {
		b.Draw(p, &barTheme)
	}
	if d.Minimised().Get() {
		// Rolled up: the bar, its controls, and its outline. Nothing else exists
		// to draw, and drawing the strip would paint below the panel.
		strokeRoundRect(p, r.X, r.Y, r.W, r.H, rad, theme.Border)
		return
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
	// title bar is rounded along the top. Absent when there are no action buttons —
	// a modal that carries its controls inside Content gets a clean card with no
	// empty strip.
	if sh := d.stripH(); sh > 0 {
		stripY := r.Y + r.H - sh
		fillRoundRect(p, r.X, stripY, r.W, sh, rad, theme.SurfaceAlt)
		if sh > rad {
			fillRect(p, r.X, stripY, r.W, sh-rad, theme.SurfaceAlt)
		}
		fillRect(p, r.X, stripY, r.W, strokeWidth(), theme.Border)
		for _, b := range d.Buttons {
			b.Draw(p, theme)
		}
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
	// The title-bar controls.
	if ev.Kind == EventClick {
		for _, b := range d.titleButtons() {
			if b.Bounds().Contains(ev.X+d.Bounds().X, ev.Y+d.Bounds().Y) {
				b.OnEvent(Event{Kind: EventClick})
				return
			}
		}
	}
	if d.Minimised().Get() {
		return // rolled up: only the bar exists, and it was handled above
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
	// Every control on the bar is a square at the trailing edge, and none of them
	// starts a drag. Counting only the close one — which is what this did before
	// the other two existed — would arm a drag on top of them.
	rightLimit := d.Bounds().W - len(d.titleButtons())*scaled(DialogTitleH)
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
