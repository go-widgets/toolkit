// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ActionRow is a libadwaita-style structured list row: a large Title
// with an optional dim Subtitle, plus optional Prefix and Suffix
// widget slots on the left and right edges. Composes into a settings-
// style list: stack several ActionRows in a VBox and each row reads
// as one entry.
//
// The row paints a Theme.Surface body with a 1-pixel Theme.Border
// divider along its bottom edge (the classic GTK list-row separator).
// Prefix / Suffix widget slots are fixed-width strips at the left and
// right; the Title (and, when non-empty, the Subtitle) flows in the
// remaining central column.
//
// ActionRow forwards EventClick events to whichever child (Prefix or
// Suffix) the click's X coordinate lands on. Clicks in the central
// text region are ignored — a caller wanting an activatable row
// wraps the ActionRow's Bounds in a container that intercepts clicks
// or overlays a button in the Suffix slot.
type ActionRow struct {
	Base
	Title    string
	Subtitle string
	Prefix   Widget // optional left slot; nil = no prefix drawn
	Suffix   Widget // optional right slot; nil = no suffix drawn
}

// Sizing constants. PadX / PadY inset text from the row edges;
// SubtitleGap is the vertical gap between the title's baseline and
// the subtitle's first row; SlotW is the width reserved for the
// optional Prefix / Suffix child widget slots.
const (
	// ActionRowPadX is the horizontal inset for the title / subtitle
	// text from the row's left edge (or from the prefix slot when a
	// Prefix widget is present).
	ActionRowPadX = 12
	// ActionRowPadY is the vertical inset for the title above the
	// row's top edge; the subtitle flows below the title.
	ActionRowPadY = 8
	// ActionRowSubtitleGap is the extra vertical gap between the title
	// glyph row and the subtitle glyph row.
	ActionRowSubtitleGap = 2
	// ActionRowSlotW is the fixed width of the Prefix / Suffix slots.
	ActionRowSlotW = 32
)

// NewActionRow constructs an ActionRow with the given title. Subtitle
// starts empty; Prefix and Suffix start nil. The caller may assign
// them before the first Draw.
func NewActionRow(title string) *ActionRow {
	return &ActionRow{Title: title}
}

// Draw paints the row body + bottom divider, then positions + draws
// the optional Prefix / Suffix child widgets, then paints the title
// (and, when non-empty, the subtitle) in the remaining central
// column. Positioning side effect: Prefix / Suffix widgets have
// their Bounds updated to reflect their slot rectangle inside the
// row.
func (a *ActionRow) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	// Bottom divider (list-row separator).
	fillRect(p, r.X, r.Y+r.H-1, r.W, 1, theme.Border)

	textX := r.X + ActionRowPadX
	if a.Prefix != nil {
		a.Prefix.SetBounds(Rect{X: r.X, Y: r.Y, W: ActionRowSlotW, H: r.H})
		a.Prefix.Draw(p, theme)
		textX = r.X + ActionRowSlotW + ActionRowPadX
	}

	ty := r.Y + ActionRowPadY
	DrawText(p, textX, ty, a.Title, theme.OnSurface)
	if a.Subtitle != "" {
		sy := ty + GlyphHeight() + ActionRowSubtitleGap
		DrawText(p, textX, sy, a.Subtitle, dimInk(theme))
	}

	if a.Suffix != nil {
		a.Suffix.SetBounds(Rect{
			X: r.X + r.W - ActionRowSlotW,
			Y: r.Y,
			W: ActionRowSlotW,
			H: r.H,
		})
		a.Suffix.Draw(p, theme)
	}
}

// OnEvent forwards EventClick to whichever Prefix / Suffix slot the
// click's X coordinate lands on, translating X into the child's
// widget-local space. Clicks in the central text region — or clicks
// on a slot whose child is nil — are ignored. Non-click events are
// dropped so keyboard input intended for a focused inner widget is
// not misrouted; a caller that needs richer keyboard routing wraps
// the ActionRow in its own dispatcher.
func (a *ActionRow) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := a.Bounds()
	if ev.X < ActionRowSlotW {
		if a.Prefix != nil {
			a.Prefix.OnEvent(Event{Kind: ev.Kind, X: ev.X, Y: ev.Y, Code: ev.Code})
		}
		return
	}
	if ev.X >= r.W-ActionRowSlotW {
		if a.Suffix != nil {
			a.Suffix.OnEvent(Event{
				Kind: ev.Kind,
				X:    ev.X - (r.W - ActionRowSlotW),
				Y:    ev.Y,
				Code: ev.Code,
			})
		}
		return
	}
}
