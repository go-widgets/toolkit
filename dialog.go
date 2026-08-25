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

	// Input, when non-nil, is a single-line input bar drawn in a strip directly
	// below the title bar; the Content body starts below it, and keyboard input
	// plus a click on the strip route to the field. Nil (the default) draws no
	// input bar and the body fills the whole area between the title bar and the
	// button strip, byte-identical to before. Its text is a shared observable —
	// read/bind it via Input.Text().
	Input DialogInput

	// closeBtn is the lazily-built close (×) control; see closeButton.
	closeBtn *IconButton
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

// dialogCloseGlyph is the glyph rendered on the close button. The toolkit's 5x7
// bitmap font carries no "×", so "x" stands in — the same close/reset affordance
// SearchEntry already uses for its clear slot.
const dialogCloseGlyph = "x"

// closeButton lazily builds (and caches) the title-bar close control, wired to
// fire OnClose live at click time (nil-safe). Only ever consulted when Closable.
func (d *Dialog) closeButton() *IconButton {
	if d.closeBtn == nil {
		d.closeBtn = NewIconButton(dialogCloseGlyph, func() {
			if d.OnClose != nil {
				d.OnClose()
			}
		})
	}
	return d.closeBtn
}

// NewDialog builds a Dialog with the given title, content + action
// buttons. Buttons are laid out right-aligned in the bottom strip.
func NewDialog(title string, content Widget, buttons ...*Button) *Dialog {
	return &Dialog{Title: title, Content: content, Buttons: buttons}
}

// SetBounds also lays out the input bar, close button, content + button
// positions.
func (d *Dialog) SetBounds(r Rect) {
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
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Background)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// Title bar.
	fillRect(p, r.X, r.Y, r.W, scaled(DialogTitleH), theme.SurfaceAlt)
	titleY := r.Y + (scaled(DialogTitleH)-d.glyphHeight())/2
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
	// Action strip.
	stripY := r.Y + r.H - scaled(DialogButtonStripH)
	fillRect(p, r.X, stripY, r.W, scaled(DialogButtonStripH), theme.SurfaceAlt)
	for _, b := range d.Buttons {
		b.Draw(p, theme)
	}
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
	if d.Content != nil {
		// Content occupies the body between the title bar and the button strip
		// (bounded in SetBounds). Translate the event into its local frame — the
		// buttons above already reconstruct absolute coords, but the content was
		// forwarded raw, so a click on interactive content landed scaled(DialogTitleH)
		// too low (plus the Dialog's own origin).
		d.Content.OnEvent(translateEvent(ev, d.Bounds(), d.Content.Bounds()))
	}
}

// NewMessageDialog is a convenience constructor for the most common
// dialog: a title, a Label as content, and an OK button that calls
// onOK + closes the dialog via the caller's OnClose hook.
func NewMessageDialog(title, message string, onOK func()) *Dialog {
	ok := NewButton("OK", onOK)
	return NewDialog(title, NewLabel(message), ok)
}
