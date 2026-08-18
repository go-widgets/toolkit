// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// RadioButton is a circular toggle paired with a label. RadioButtons
// are typically grouped via RadioGroup so exactly one in the group is
// Checked at any time. A standalone RadioButton (not added to a
// group) behaves like a CheckButton (toggleable on click).
type RadioButton struct {
	Base
	focusState
	Label    string
	Checked  bool
	OnToggle func(checked bool)

	group *RadioGroup
	index int
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
	return &RadioButton{Label: label}
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
	if r.Disabled {
		face, border, dot, labelInk = mutedFace(theme), mutedInk(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, b.X, boxY, scaled(radioBoxSize), scaled(radioBoxSize), face)
	strokeRect(p, b.X, boxY, scaled(radioBoxSize), scaled(radioBoxSize), border)
	if r.Checked {
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
	if r.Disabled {
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
			// firing the newly-checked member's callback (reusing group.activate)
			// and following focus to it -- the ARIA radio-group convention.
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

// toggleStandalone flips a group-less radio's Checked and fires OnToggle
// (nil-safe) -- the shared mutate path for a click and a Space/Enter key press.
func (r *RadioButton) toggleStandalone() {
	r.Checked = !r.Checked
	if r.OnToggle != nil {
		r.OnToggle(r.Checked)
	}
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
	Active  int

	// OnChange fires whenever the checked member changes through a user
	// interaction: a click on a member, or an arrow key moving the checked
	// member through the group. active is the new Active index. It fires
	// alongside the newly-checked member's own OnToggle, giving the group a
	// single-argument slot the whole selection can be observed through (e.g.
	// via mvvmtk.BindRadioGroup). Nil is safe.
	OnChange func(active int)
}

// NewRadioGroup builds an empty group with Active = -1.
func NewRadioGroup() *RadioGroup { return &RadioGroup{Active: -1} }

// Add appends r to the group + remembers its membership so a click
// on any member can clear the others.
func (g *RadioGroup) Add(r *RadioButton) {
	r.group = g
	r.index = len(g.Members)
	g.Members = append(g.Members, r)
}

// activate sets Active = idx, clears every other member's Checked,
// + fires OnToggle on the newly-checked one.
func (g *RadioGroup) activate(idx int) {
	g.Active = idx
	for i, m := range g.Members {
		m.Checked = i == idx
	}
	if cb := g.Members[idx].OnToggle; cb != nil {
		cb(true)
	}
	if g.OnChange != nil {
		g.OnChange(idx)
	}
}

// moveChecked steps the checked member delta places (±1) from the currently
// focused member from, wrapping at both ends, activates it (reusing activate so
// the member callback fires exactly as a click would), and follows keyboard
// focus to the newly-checked member -- the ARIA radio-group arrow convention.
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
