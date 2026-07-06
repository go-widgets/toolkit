// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// FormFieldLabelH is the height in pixels of the label row drawn at
// the top of a FormField. One glyph row plus 2px of breathing space
// keeps the label snug against the input beneath it without touching
// the glyph's descender pixels.
const FormFieldLabelH = GlyphHeight + 2

// FormFieldChildGap is the vertical gap in pixels between the bottom
// of the label row and the top of the composed Child widget.
const FormFieldChildGap = 4

// FormFieldHelpGap is the vertical gap in pixels between the bottom
// of the Child widget and the top of the help / error caption row.
const FormFieldHelpGap = 2

// FormFieldPadX is the horizontal padding applied on both sides of
// the FormField body. Kept at 0 by default: a form is expected to
// live inside a container (VBox, Card, ...) that supplies its own
// margin. Callers that need extra breathing room can wrap the field
// in a Card.
const FormFieldPadX = 0

// FormFieldPadY is the vertical padding applied at the top + bottom
// of the FormField body. Small: keeps a stack of fields compact
// without having every caller compute inter-field spacing.
const FormFieldPadY = 4

// formFieldErrorInk is the fixed red used for the Error caption. Not
// pulled from the Theme because "error" carries semantic meaning that
// must survive theme changes (a red field on a red-tinted theme still
// needs to read as "problem"). Same shade as the toolkit's other
// destructive-state widgets.
var formFieldErrorInk = RGBA{R: 190, G: 60, B: 60, A: 255}

// FormField is a labelled input row: a Label above (in theme.OnBack-
// ground), an optional Child input widget below, and an optional
// caption row underneath the Child that shows either an Error (in
// fixed red) or Help text (in theme.Border for a muted look). Error
// takes precedence over Help when both are set.
//
// FormField sits directly on theme.Background (it is a form
// container, not a card) and does not fill its own body — the label
// glyphs and the composed Child provide their own inks. Callers
// wanting a filled body can wrap the FormField in a Card.
//
// Child composition: SetBounds on the Child is called during Draw so
// callers only have to position the FormField itself. OnEvent
// forwards clicks (and other event kinds' point events) to the Child
// when (X, Y) falls inside the Child rect, translating coordinates
// into Child-local space. Non-point events (keyboard) are forwarded
// unconditionally so the Child can react to focus-driven input.
type FormField struct {
	Base
	Label string
	Help  string // optional dim caption below the child
	Error string // optional error caption below the child (takes precedence)
	Child Widget // the actual input; may be nil
}

// NewFormField constructs a FormField wrapping child with a label
// above. Help + Error remain empty; the caller assigns them as the
// field's state changes.
func NewFormField(label string, child Widget) *FormField {
	return &FormField{Label: label, Child: child}
}

// childRect returns the rect assigned to the Child widget: the strip
// between the label row and the caption row, honouring the pad + gap
// constants. Extracted so Draw and OnEvent agree on the same layout.
func (f *FormField) childRect() Rect {
	r := f.Bounds()
	captionH := 0
	if f.Error != "" || f.Help != "" {
		captionH = GlyphHeight + FormFieldHelpGap
	}
	top := r.Y + FormFieldPadY + FormFieldLabelH + FormFieldChildGap
	bottom := r.Y + r.H - FormFieldPadY - captionH
	h := bottom - top
	if h < 0 {
		h = 0
	}
	return Rect{
		X: r.X + FormFieldPadX,
		Y: top,
		W: r.W - 2*FormFieldPadX,
		H: h,
	}
}

// Draw paints the label row, positions + draws the Child (when non-
// nil), and paints the caption row (Error > Help > nothing).
func (f *FormField) Draw(p painter.Painter, theme *Theme) {
	r := f.Bounds()
	// Label at the top in OnBackground (form sits on Background).
	DrawText(p, r.X+FormFieldPadX, r.Y+FormFieldPadY, f.Label, theme.OnBackground)
	// Child in the middle strip.
	cr := f.childRect()
	if f.Child != nil {
		f.Child.SetBounds(cr)
		f.Child.Draw(p, theme)
	}
	// Caption row: Error > Help > nothing.
	captionY := cr.Y + cr.H + FormFieldHelpGap
	if f.Error != "" {
		DrawText(p, r.X+FormFieldPadX, captionY, f.Error, formFieldErrorInk)
		return
	}
	if f.Help != "" {
		DrawText(p, r.X+FormFieldPadX, captionY, f.Help, theme.Border)
	}
}

// OnEvent forwards the event to Child when Child is non-nil. Point
// events (EventClick) are gated on the Child rect so a click outside
// the input body is dropped; non-point events (keyboard/composition)
// are forwarded unconditionally so a focused Child sees them. Nil
// Child is a no-op.
func (f *FormField) OnEvent(ev Event) {
	if f.Child == nil {
		return
	}
	if ev.Kind == EventClick {
		cr := f.childRect()
		if ev.X < cr.X || ev.X >= cr.X+cr.W || ev.Y < cr.Y || ev.Y >= cr.Y+cr.H {
			return
		}
		ev.X -= cr.X
		ev.Y -= cr.Y
	}
	f.Child.OnEvent(ev)
}
