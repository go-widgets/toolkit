// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// RadioButton is a circular toggle paired with a label. RadioButtons
// are typically grouped via RadioGroup so exactly one in the group is
// Checked at any time. A standalone RadioButton (not added to a
// group) behaves like a CheckButton (toggleable on click).
type RadioButton struct {
	Base
	focusState
	Label string

	// checked is the reactive checked state, MVVM-only: it lives in an
	// unexported Observable exposed via [RadioButton.Checked]. A host binds it
	// (Set / Subscribe / two-way); there is no settable Checked field.
	checked *mvvm.Observable[bool]

	group *RadioGroup
	index int
}

// Checked is the current checked state as a shared [mvvm.Observable]: a host
// binds it (Set / Subscribe / two-way) — there is no settable Checked field. A
// click (standalone) or a group selection Sets it, notifying subscribers.
func (r *RadioButton) Checked() *mvvm.Observable[bool] {
	if r.checked == nil {
		r.checked = mvvm.NewObservable(false)
	}
	return r.checked
}

// radioBoxSize is the pixel diameter of the round mark.
const radioBoxSize = 12

// radioDotInset is the inset (logical pixels) from the mark's edge to the filled
// dot on each side; radioLabelGap is the logical gap between the mark and the
// label. Both route through scaled so the mark's interior and the label spacing
// grow with HiDPI and touch density; at compact/1x they equal their constants,
// keeping the drawn radio byte-identical.
const (
	radioDotInset = 3
	radioLabelGap = 4
)

// NewRadioButton constructs a standalone RadioButton with the given
// label. Add it to a RadioGroup with group.Add(r) for mutual-exclusion
// behaviour.
func NewRadioButton(label string) *RadioButton {
	r := &RadioButton{Label: label}
	r.checked = mvvm.NewObservable(false)
	return r
}

// Draw paints the circular mark + label. The "circle" is a 12 x 12
// box with a 1-pixel inset on every side, painted as a stroked
// rectangle (approximate to avoid bringing in trig). When Checked,
// a smaller Accent-filled rect sits inside as the radio dot.
func (r *RadioButton) Draw(p painter.Painter, theme *Theme) {
	b := r.Bounds()
	boxY := b.Y + (b.H-scaled(radioBoxSize))/2
	// A disabled radio mutes its mark, dot and label; the enabled draw is
	// unchanged (the branch is only taken when Disabled).
	face, border, dot, labelInk := theme.Surface, theme.Border, theme.Accent, theme.OnBackground
	if r.Disabled().Get() {
		face, border, dot, labelInk = mutedFace(theme), mutedInk(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, b.X, boxY, scaled(radioBoxSize), scaled(radioBoxSize), face)
	strokeRect(p, b.X, boxY, scaled(radioBoxSize), scaled(radioBoxSize), border)
	if r.Checked().Get() {
		inset := scaled(radioDotInset)
		fillRect(p, b.X+inset, boxY+inset, scaled(radioBoxSize)-2*inset, scaled(radioBoxSize)-2*inset, dot)
	}
	textY := b.Y + (b.H-r.glyphHeight())/2
	r.drawText(p, b.X+scaled(radioBoxSize)+scaled(radioLabelGap), textY, r.Label, labelInk)
	r.drawFocusRing(p, theme, b)
}

// OnEvent: on click, route through the group (if any) so siblings
// clear; otherwise toggle Checked locally.
func (r *RadioButton) OnEvent(ev Event) {
	if r.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		if r.group != nil {
			r.group.activate(r.index)
			return
		}
		r.toggleStandalone()
	case EventKeyDown:
		if r.group != nil {
			// Arrow keys move the checked member through the group, wrapping,
			// setting the newly-checked member's Checked Observable (reusing
			// group.activate) and following focus to it -- the ARIA radio-group
			// convention.
			switch ev.Code {
			case "ArrowDown", "ArrowRight":
				r.group.moveChecked(r, +1)
			case "ArrowUp", "ArrowLeft":
				r.group.moveChecked(r, -1)
			}
			return
		}
		// A standalone radio (no group) behaves like a CheckButton: Space/Enter
		// toggles it, reusing the click path.
		switch ev.Code {
		case " ", "Space", "Enter":
			r.toggleStandalone()
		}
	}
}

// toggleStandalone flips a group-less radio's Checked Observable -- the shared
// mutate path for a click and a Space/Enter key press. Subscribers are notified
// on change.
func (r *RadioButton) toggleStandalone() {
	r.Checked().Set(!r.Checked().Get())
}

// HitRect is the radio button's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]). Like
// the checkbox, its short row grows to the >=44px finger floor under
// DensityTouch while the drawn 12px mark is untouched; byte-identical to Bounds
// under DensityCompact.
func (r *RadioButton) HitRect() Rect { return touchHitRect(r.Bounds()) }

// HitTest reports whether a surface point falls on the radio button's
// (touch-clamped) hit rect.
func (r *RadioButton) HitTest(px, py int) bool { return r.HitRect().Contains(px, py) }

// RadioGroup makes a set of RadioButtons mutually exclusive. Active
// is the index of the currently-checked member, or -1 when none has
// been clicked yet.
type RadioGroup struct {
	Members []*RadioButton

	// active is the index of the currently-checked member, MVVM-only: it lives in
	// an unexported Observable exposed via [RadioGroup.Active]. A host binds it
	// (Set / Subscribe / two-way); there is no settable Active field.
	active *mvvm.Observable[int]
}

// Active is the index of the currently-checked member as a shared
// [mvvm.Observable], or -1 when none has been clicked yet: a host binds it (Set
// / Subscribe / two-way) — there is no settable Active field. A click on a
// member, or an arrow key moving the checked member, Sets it and notifies
// subscribers. A bare &RadioGroup{} lazy-inits Active to 0; NewRadioGroup starts
// it at -1.
func (g *RadioGroup) Active() *mvvm.Observable[int] {
	if g.active == nil {
		g.active = mvvm.NewObservable(0)
	}
	return g.active
}

// NewRadioGroup builds an empty group with Active = -1.
func NewRadioGroup() *RadioGroup {
	g := &RadioGroup{}
	g.active = mvvm.NewObservable(-1)
	return g
}

// Add appends r to the group + remembers its membership so a click
// on any member can clear the others.
func (g *RadioGroup) Add(r *RadioButton) {
	r.group = g
	r.index = len(g.Members)
	g.Members = append(g.Members, r)
}

// activate Sets Active = idx and clears every other member's Checked, Setting
// the newly-checked one's Checked to true. A host observes the selection through
// the members' Checked() Observables and/or the group's Active() Observable.
func (g *RadioGroup) activate(idx int) {
	g.Active().Set(idx)
	for i, m := range g.Members {
		m.Checked().Set(i == idx)
	}
}

// moveChecked steps the checked member delta places (±1) from the currently
// focused member from, wrapping at both ends, activates it (reusing activate so
// the member Checked Observable Sets exactly as a click would), and follows
// keyboard focus to the newly-checked member -- the ARIA radio-group arrow
// convention.
func (g *RadioGroup) moveChecked(from *RadioButton, delta int) {
	n := len(g.Members)
	if n == 0 {
		return
	}
	next := ((from.index+delta)%n + n) % n
	g.activate(next)
	from.SetFocused(false)
	g.Members[next].SetFocused(true)
}
