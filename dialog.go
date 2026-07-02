// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Dialog is a modal overlay: a centred Surface card with an optional
// Title bar, a Content widget filling the body, and an action-button
// strip at the bottom. The compositor draws a semi-darkened backdrop
// over the rest of the surface so the user's attention focuses on
// the dialog.
//
// v0.3 ships the structure; the host app is responsible for routing
// input events only to the dialog while it's open (existing wasmbox
// modal-grab behaviour).
type Dialog struct {
	Base
	Title    string
	Content  Widget
	Buttons  []*Button
	OnClose  func()
}

// DialogTitleH is the pixel height of the title bar.
const DialogTitleH = 28

// DialogButtonStripH is the pixel height of the bottom action strip.
const DialogButtonStripH = 32

// DialogButtonW is the width allocated per action button.
const DialogButtonW = 90

// NewDialog builds a Dialog with the given title, content + action
// buttons. Buttons are laid out right-aligned in the bottom strip.
func NewDialog(title string, content Widget, buttons ...*Button) *Dialog {
	return &Dialog{Title: title, Content: content, Buttons: buttons}
}

// SetBounds also lays out the content + button positions.
func (d *Dialog) SetBounds(r Rect) {
	d.Base.SetBounds(r)
	if d.Content != nil {
		body := Rect{
			X: r.X,
			Y: r.Y + DialogTitleH,
			W: r.W,
			H: r.H - DialogTitleH - DialogButtonStripH,
		}
		d.Content.SetBounds(body)
	}
	// Right-align the action buttons in the bottom strip.
	stripY := r.Y + r.H - DialogButtonStripH
	bx := r.X + r.W - 8
	for i := len(d.Buttons) - 1; i >= 0; i-- {
		bx -= DialogButtonW
		d.Buttons[i].SetBounds(Rect{X: bx, Y: stripY + 4, W: DialogButtonW - 8, H: DialogButtonStripH - 8})
		bx -= 8 // gap
	}
}

// Draw paints card + title + content + buttons.
func (d *Dialog) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Background)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// Title bar.
	fillRect(p, r.X, r.Y, r.W, DialogTitleH, theme.SurfaceAlt)
	titleY := r.Y + (DialogTitleH-GlyphHeight)/2
	DrawText(p, r.X+8, titleY, d.Title, theme.OnSurface)
	// Content.
	if d.Content != nil {
		d.Content.Draw(p, theme)
	}
	// Action strip.
	stripY := r.Y + r.H - DialogButtonStripH
	fillRect(p, r.X, stripY, r.W, DialogButtonStripH, theme.SurfaceAlt)
	for _, b := range d.Buttons {
		b.Draw(p, theme)
	}
}

// OnEvent forwards to content + buttons. A click that doesn't land
// on any button or the content falls through silently (the app
// keeps the dialog open).
func (d *Dialog) OnEvent(ev Event) {
	for _, b := range d.Buttons {
		if b.Bounds().Contains(ev.X+d.Bounds().X, ev.Y+d.Bounds().Y) && ev.Kind == EventClick {
			b.OnEvent(Event{Kind: EventClick})
			return
		}
	}
	if d.Content != nil {
		d.Content.OnEvent(ev)
	}
}

// NewMessageDialog is a convenience constructor for the most common
// dialog: a title, a Label as content, and an OK button that calls
// onOK + closes the dialog via the caller's OnClose hook.
func NewMessageDialog(title, message string, onOK func()) *Dialog {
	ok := NewButton("OK", onOK)
	return NewDialog(title, NewLabel(message), ok)
}
