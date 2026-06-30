// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// RadioButton is a circular toggle paired with a label. RadioButtons
// are typically grouped via RadioGroup so exactly one in the group is
// Checked at any time. A standalone RadioButton (not added to a
// group) behaves like a CheckButton (toggleable on click).
type RadioButton struct {
	Base
	Label    string
	Checked  bool
	OnToggle func(checked bool)

	group *RadioGroup
	index int
}

// radioBoxSize is the pixel diameter of the round mark.
const radioBoxSize = 12

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
func (r *RadioButton) Draw(surface []byte, surfaceW int, theme *Theme) {
	b := r.Bounds()
	boxY := b.Y + (b.H-radioBoxSize)/2
	fillRect(surface, surfaceW, b.X, boxY, radioBoxSize, radioBoxSize, theme.Surface)
	strokeRect(surface, surfaceW, b.X, boxY, radioBoxSize, radioBoxSize, theme.Border)
	if r.Checked {
		fillRect(surface, surfaceW, b.X+3, boxY+3, radioBoxSize-6, radioBoxSize-6, theme.Accent)
	}
	textY := b.Y + (b.H-GlyphHeight)/2
	DrawText(surface, surfaceW, b.X+radioBoxSize+4, textY, r.Label, theme.OnBackground)
}

// OnEvent: on click, route through the group (if any) so siblings
// clear; otherwise toggle Checked locally.
func (r *RadioButton) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if r.group != nil {
		r.group.activate(r.index)
		return
	}
	r.Checked = !r.Checked
	if r.OnToggle != nil {
		r.OnToggle(r.Checked)
	}
}

// RadioGroup makes a set of RadioButtons mutually exclusive. Active
// is the index of the currently-checked member, or -1 when none has
// been clicked yet.
type RadioGroup struct {
	Members []*RadioButton
	Active  int
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
}
