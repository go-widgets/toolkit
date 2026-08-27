// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SettingsGroup is a titled card that stacks SettingRows, drawing the
// inter-row dividers for you: it is the container half of the preferences
// pattern, the way a run of settings actually ships (a captioned card with
// a divided list of rows inside). It composes the shared card frame
// (cardFrame: a rounded Theme.Surface fill under a 1-px Theme.Border) so it
// sits in a settings screen next to the other cards as one system, then
// lays each row full-width inside the frame and turns the divider on for
// every row but the last (the card border closes the group, so a trailing
// divider would double it).
//
// Layout (inside the CardPadX/Y inset):
//
//	┌──────────────────────────────────────────────┐
//	│ Title                                          │  ← optional group caption
//	│ Row 0 …………………………………… [Control] │
//	│ ─────────────────────────────────────────────│  ← divider between rows
//	│ Row 1 …………………………………… [Control] │
//	│ ─────────────────────────────────────────────│
//	│ Row 2 …………………………………… [Control] │  ← no trailing divider
//	└──────────────────────────────────────────────┘
type SettingsGroup struct {
	Base
	// Title is the optional group caption drawn dim at the top of the card.
	// Empty draws no caption and the rows start at the top inset.
	Title string
	// Rows are the settings rows stacked top-to-bottom.
	Rows []*SettingRow
}

// NewSettingsGroup builds a SettingsGroup with the given caption (may be
// "") and rows.
func NewSettingsGroup(title string, rows ...*SettingRow) *SettingsGroup {
	return &SettingsGroup{Title: title, Rows: rows}
}

// headerH is the caption strip's height: one glyph row plus a CardGapY of
// breathing space, or zero when Title is empty.
func (g *SettingsGroup) headerH() int {
	if g.Title == "" {
		return 0
	}
	return g.glyphHeight() + scaled(CardGapY)
}

// Measure reports the card's exact height at outer width width: the CardPadY
// inset top and bottom, the optional caption, and every row's own measured
// height at the inner (inset) width.
func (g *SettingsGroup) Measure(width int) int {
	h := 2*scaled(CardPadY) + g.headerH()
	inner := width - 2*scaled(CardPadX)
	for _, row := range g.Rows {
		h += row.Measure(inner)
	}
	return h
}

// SetBounds positions the group and lays out its rows, each at its own measured
// height inside the card inset -- the same arrangement Draw makes, made before
// the first frame rather than during it, so a row and its Control have bounds as
// soon as the group does.
func (g *SettingsGroup) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	inner := cardContent(r)
	y := inner.Y + g.headerH()
	last := len(g.Rows) - 1
	for i, row := range g.Rows {
		row.Divider = i < last
		rh := row.Measure(inner.W)
		row.SetBounds(Rect{X: inner.X, Y: y, W: inner.W, H: rh})
		y += rh
	}
}

// Draw paints the card frame + optional caption, then lays each row full
// inner width and draws it, setting Divider on every row but the last.
// Row positioning is a side effect (SetBounds), so a later OnEvent can
// hit-test the rows the group just placed.
func (g *SettingsGroup) Draw(p painter.Painter, theme *Theme) {
	inner := cardFrame(p, theme, g.Bounds())
	y := inner.Y
	if g.Title != "" {
		g.drawText(p, inner.X, y, g.Title, dimInk(theme))
		y += g.glyphHeight() + scaled(CardGapY)
	}
	last := len(g.Rows) - 1
	for i, row := range g.Rows {
		row.Divider = i < last
		rh := row.Measure(inner.W)
		row.SetBounds(Rect{X: inner.X, Y: y, W: inner.W, H: rh})
		row.Draw(p, theme)
		y += rh
	}
}

// OnEvent forwards a click to the row whose (last-laid-out) bounds contain
// it, translating the coordinates into that row's local space. Non-click
// events and clicks that miss every row are dropped — a settings card is
// navigated through its rows' own controls.
func (g *SettingsGroup) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := g.Bounds()
	for _, row := range g.Rows {
		rb := row.Bounds()
		if rb.Contains(ev.X+r.X, ev.Y+r.Y) {
			row.OnEvent(translateEvent(ev, r, rb))
			return
		}
	}
}

// A11y reports the card as a group named by its Title. The rows are reached
// through Children, so WalkA11y descends group -> row -> control.
func (g *SettingsGroup) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: g.Title}
}
