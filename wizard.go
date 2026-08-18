// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// WizardStep is one page of a Wizard: a Title shown in the top Steps
// strip, a Body widget shown in the content area while the step is
// active, and an optional CanAdvance gate. CanAdvance is consulted
// before the Wizard lets the user move past this step; a nil
// CanAdvance means "always allowed" (the common case — a step with
// no validation).
type WizardStep struct {
	Title      string
	Body       Widget
	CanAdvance func() bool // nil = always allowed
}

// Wizard geometry constants. The strip + button row are fixed-height
// bands pinned to the top/bottom edges; the Body of the active step
// fills whatever is left between them (mirroring Notebook's
// strip-plus-body split in notebook.go).
const (
	// WizardStripH is the pixel height of the top Steps strip.
	WizardStripH = 40
	// WizardButtonRowH is the pixel height of the bottom Back/Next/
	// Finish button row.
	WizardButtonRowH = 32
	// WizardButtonW is the pixel width of each Back/Next/Finish button.
	WizardButtonW = 90
	// WizardButtonGap is the horizontal gap between a button and the
	// Wizard's edge (Back hugs the left edge, Next/Finish the right).
	WizardButtonGap = 8
)

// Wizard is a multi-step "Assistant" flow: a Steps strip across the
// top tracks progress through Steps, the current step's Body fills
// the middle, and a Back / Next-or-Finish button row sits at the
// bottom. Next is disabled (a no-op) whenever the current step's
// CanAdvance reports false; Back is disabled on the first step.
// Advancing past the last step swaps the Next label to "Finish" and
// invokes OnFinish instead of moving further.
type Wizard struct {
	Base
	// Steps is the ordered step list (config). The reactive step index is
	// MVVM-only: the current step lives in an unexported Observable exposed via
	// [Wizard.Current].
	Steps    []WizardStep
	OnFinish func()

	// PressFeedback shows the pressed face on the Back / Next button while it is
	// held (EventClick → EventMouseUp). NewWizard enables it; set false to opt
	// out.
	PressFeedback bool

	current    *mvvm.Observable[int]
	pressedBtn int // wizBtnNone / wizBtnBack / wizBtnNext
}

// Current is the active step index as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Current field. Back /
// Next Set it (clamped to [0, len(Steps)-1]); subscribers are notified.
func (w *Wizard) Current() *mvvm.Observable[int] {
	if w.current == nil {
		w.current = mvvm.NewObservable(0)
	}
	return w.current
}

const (
	wizBtnNone = iota
	wizBtnBack
	wizBtnNext
)

// NewWizard constructs a Wizard over the given steps, starting on the
// first one (Current == 0).
func NewWizard(steps []WizardStep) *Wizard {
	w := &Wizard{Steps: steps, PressFeedback: true}
	w.current = mvvm.NewObservable(0)
	return w
}

// onLastStep reports whether Current is at (or past) the final step
// index — the point at which Next() means "finish" rather than
// "advance".
func (w *Wizard) onLastStep() bool {
	return w.Current().Get() >= len(w.Steps)-1
}

// currentCanAdvance reports whether the active step permits moving
// past it. A Current outside the valid step range (nothing active)
// is treated as unrestricted; a nil CanAdvance on the active step
// means "always allowed" per WizardStep's doc.
func (w *Wizard) currentCanAdvance() bool {
	cur := w.Current().Get()
	if cur < 0 || cur >= len(w.Steps) {
		return true
	}
	gate := w.Steps[cur].CanAdvance
	if gate == nil {
		return true
	}
	return gate()
}

// nextButtonLabel is "Finish" on the last step, "Next" otherwise —
// the label Draw paints on the right-hand button.
func (w *Wizard) nextButtonLabel() string {
	if w.onLastStep() {
		return "Finish"
	}
	return "Next"
}

// Next advances to the following step when the current one's
// CanAdvance allows it. On the last step it instead invokes OnFinish
// (if set) and leaves Current unchanged — Next() is the "Finish"
// action once there is nowhere further to advance to.
func (w *Wizard) Next() {
	if w.onLastStep() {
		if w.OnFinish != nil {
			w.OnFinish()
		}
		return
	}
	if !w.currentCanAdvance() {
		return
	}
	w.Current().Set(w.Current().Get() + 1)
}

// Back moves to the previous step, clamped at 0 (a no-op on the
// first step).
func (w *Wizard) Back() {
	if cur := w.Current().Get(); cur > 0 {
		w.Current().Set(cur - 1)
	}
}

// stepTitles collects each step's Title for handing to the Steps
// strip widget.
func (w *Wizard) stepTitles() []string {
	titles := make([]string, len(w.Steps))
	for i, st := range w.Steps {
		titles[i] = st.Title
	}
	return titles
}

// stripRect is the top Steps band, in surface coordinates.
func (w *Wizard) stripRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X, Y: r.Y, W: r.W, H: WizardStripH}
}

// buttonRowRect is the bottom Back/Next band, in surface coordinates.
func (w *Wizard) buttonRowRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X, Y: r.Y + r.H - WizardButtonRowH, W: r.W, H: WizardButtonRowH}
}

// bodyRect is the content area between the strip and the button row,
// in surface coordinates — the space the active step's Body is laid
// out into (mirrors Notebook.bodyRect in notebook.go).
func (w *Wizard) bodyRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X, Y: r.Y + WizardStripH, W: r.W, H: r.H - WizardStripH - WizardButtonRowH}
}

// backRect is the Back button's rect, hugging the left edge of the
// button row.
func (w *Wizard) backRect() Rect {
	br := w.buttonRowRect()
	return Rect{X: br.X + WizardButtonGap, Y: br.Y, W: WizardButtonW, H: br.H}
}

// nextRect is the Next/Finish button's rect, hugging the right edge
// of the button row.
func (w *Wizard) nextRect() Rect {
	br := w.buttonRowRect()
	return Rect{X: br.X + br.W - WizardButtonGap - WizardButtonW, Y: br.Y, W: WizardButtonW, H: br.H}
}

// Draw paints the Steps strip, the active step's Body, and the
// Back/Next-or-Finish button row. A Wizard with no Steps paints
// nothing (there is nothing to show progress through). Back renders
// in ButtonSecondary tone (dimmed) on the first step; Next/Finish
// renders dimmed whenever the active step's CanAdvance forbids
// moving on.
func (w *Wizard) Draw(p painter.Painter, theme *Theme) {
	if len(w.Steps) == 0 {
		return
	}
	cur := w.Current().Get()
	strip := NewSteps(w.stepTitles(), cur)
	strip.SetBounds(w.stripRect())
	strip.Draw(p, theme)

	if cur >= 0 && cur < len(w.Steps) {
		body := w.Steps[cur].Body
		if body != nil {
			bodyR := w.bodyRect()
			body.SetBounds(bodyR)
			body.Draw(p, theme)
		}
	}

	back := NewButton("Back", nil)
	back.SetBounds(w.backRect())
	if cur == 0 {
		back.Style = ButtonSecondary
	}
	if w.pressedBtn == wizBtnBack {
		back.SetPressed(true)
	}
	back.Draw(p, theme)

	next := NewButton(w.nextButtonLabel(), nil)
	next.SetBounds(w.nextRect())
	next.Style = ButtonProminent
	if !w.currentCanAdvance() {
		next.Style = ButtonSecondary
	}
	if w.pressedBtn == wizBtnNext {
		next.SetPressed(true)
	}
	next.Draw(p, theme)
}

// OnEvent routes a click on the Back button to Back(), a click on the
// Next/Finish button to Next() (both no-ops when disabled — Current
// == 0 for Back, a failing CanAdvance for Next/Finish), and any other
// click that lands in the body area — or any non-click event — to the
// active step's Body, translated into its local coordinate space
// (the same pattern Notebook.OnEvent uses in notebook.go). A click
// that lands in neither the buttons nor the body (e.g. the empty
// strip band) is ignored.
func (w *Wizard) OnEvent(ev Event) {
	if len(w.Steps) == 0 {
		return
	}
	r := w.Bounds()
	if ev.Kind == EventMouseUp {
		w.pressedBtn = wizBtnNone // release the pressed Back/Next feedback
	}
	if ev.Kind == EventClick {
		// ev is widget-local; hit-test the buttons in surface coordinates.
		ax, ay := ev.X+r.X, ev.Y+r.Y
		if w.backRect().Contains(ax, ay) {
			if w.Current().Get() > 0 {
				if w.PressFeedback {
					w.pressedBtn = wizBtnBack
				}
				w.Back()
			}
			return
		}
		if w.nextRect().Contains(ax, ay) {
			if w.currentCanAdvance() {
				if w.PressFeedback {
					w.pressedBtn = wizBtnNext
				}
				w.Next()
			}
			return
		}
		if !w.bodyRect().Contains(ax, ay) {
			return
		}
	}
	cur := w.Current().Get()
	if cur < 0 || cur >= len(w.Steps) {
		return
	}
	body := w.Steps[cur].Body
	if body == nil {
		return
	}
	bodyR := w.bodyRect()
	body.SetBounds(bodyR)
	body.OnEvent(translateEvent(ev, r, bodyR))
}
