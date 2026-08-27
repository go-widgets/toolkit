// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// formFieldLabelPad is the LOGICAL breathing space under the label glyph row,
// scaled at use; identity at compact/1x.
const formFieldLabelPad = 2

// FormFieldLabelH is the height in pixels of the label row drawn at
// the top of a FormField. One glyph row plus a scaled 2px of breathing space
// keeps the label snug against the input beneath it without touching
// the glyph's descender pixels, and grows with HiDPI / touch density.
func FormFieldLabelH() int { return GlyphHeight() + scaled(formFieldLabelPad) }

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
	Help  string                   // optional dim caption below the child
	error *mvvm.Observable[string] // reactive via Error()
	Child Widget                   // the actual input; may be nil
	Rules []Rule                   // optional validation rules run by Validate
}

// Error is reactive state as a shared [mvvm.Observable]; edits Set it. Lazily created.
func (f *FormField) Error() *mvvm.Observable[string] {
	if f.error == nil {
		f.error = mvvm.NewObservable[string]("")
	}
	return f.error
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
	padY := scaled(FormFieldPadY)
	padX := scaled(FormFieldPadX)
	captionH := 0
	if f.Error().Get() != "" || f.Help != "" {
		captionH = f.glyphHeight() + scaled(FormFieldHelpGap)
	}
	top := r.Y + padY + FormFieldLabelH() + scaled(FormFieldChildGap)
	bottom := r.Y + r.H - padY - captionH
	h := bottom - top
	if h < 0 {
		h = 0
	}
	return Rect{
		X: r.X + padX,
		Y: top,
		W: r.W - 2*padX,
		H: h,
	}
}

// SetBounds positions the field and seats its Child in the strip between the
// label row and the caption row.
//
// Draw does this too, and did it alone until now, which meant a Child had NO
// bounds until the first frame was painted. Everything that asks a widget where
// it is before then got Rect{}: a click routed to a list inside a form field
// picked its row from a zero-height rectangle, an accessibility walk reported
// every control at the origin, and a layout assertion measured a tree that
// looked empty. Positioning is layout, so it belongs here; Draw keeps its own
// call, which costs nothing and re-seats the child if Help or Error changed the
// strip since.
func (f *FormField) SetBounds(r Rect) {
	f.Base.SetBounds(r)
	if f.Child != nil {
		f.Child.SetBounds(f.childRect())
	}
}

// Draw paints the label row, positions + draws the Child (when non-
// nil), and paints the caption row (Error > Help > nothing).
func (f *FormField) Draw(p painter.Painter, theme *Theme) {
	r := f.Bounds()
	padX := scaled(FormFieldPadX)
	// Label at the top in OnBackground (form sits on Background).
	f.drawText(p, r.X+padX, r.Y+scaled(FormFieldPadY), f.Label, theme.OnBackground)
	// Child in the middle strip.
	cr := f.childRect()
	if f.Child != nil {
		f.Child.SetBounds(cr)
		f.Child.Draw(p, theme)
	}
	// Caption row: Error > Help > nothing.
	captionY := cr.Y + cr.H + scaled(FormFieldHelpGap)
	if f.Error().Get() != "" {
		f.drawText(p, r.X+padX, captionY, f.Error().Get(), formFieldErrorInk)
		return
	}
	if f.Help != "" {
		f.drawText(p, r.X+padX, captionY, f.Help, theme.Border)
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
		// ev is widget-local; childRect() is in surface (absolute) coordinates.
		// Reconstruct the absolute click to hit-test the child rect, then hand
		// the child its own local frame via translateEvent — instead of the old
		// code, which compared local ev against absolute cr and so dropped every
		// click whenever the FormField was not at the origin.
		r := f.Bounds()
		cr := f.childRect()
		if !cr.Contains(ev.X+r.X, ev.Y+r.Y) {
			return
		}
		f.Child.OnEvent(translateEvent(ev, r, cr))
		return
	}
	f.Child.OnEvent(ev)
}

// valueGetter is implemented by input widgets that expose their
// current text (Entry, ...). FormField.Value type-asserts Child
// against it instead of depending on any concrete input type, so
// FormField stays usable with future input widgets that adopt the
// same accessor.
type valueGetter interface {
	Value() string
}

// Value returns the current text of the wrapped Child, or "" when
// Child is nil or does not implement valueGetter.
func (f *FormField) Value() string {
	if vg, ok := f.Child.(valueGetter); ok {
		return vg.Value()
	}
	return ""
}

// Validate runs Rules against the field's current Value, in order,
// stopping at the first failure -- the same short-circuit semantics
// as the package-level Validate. On failure, Error is set to the
// failing rule's message and Validate returns false. On success (or
// when Rules is empty), Error is cleared and Validate returns true.
//
// Validate only ever touches Error; it does not repaint -- callers
// invoke it (typically from a submit handler or an OnChange
// callback on Child) and then trigger their own redraw so the
// caption row picks up the new Error.
func (f *FormField) Validate() bool {
	if err := Validate(f.Value(), f.Rules...); err != nil {
		f.Error().Set(err.Error())
		return false
	}
	f.Error().Set("")
	return true
}
