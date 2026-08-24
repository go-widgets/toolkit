// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SettingRow is a preferences/settings row: a leading Title (with an
// optional dim Subtitle description line) and a single trailing Control
// widget — a Switch, a Scale, a Button, a DropDown, whatever the setting
// needs — pinned to the row's right edge, over an optional bottom
// Divider. Stack several in a SettingsGroup and the group reads as one
// settings card.
//
// It exists because the toolkit's ActionRow, while superficially similar,
// reserves a FIXED ActionRowSlotW-wide (32px) Suffix strip meant for an
// icon or a chevron — far too narrow for a Scale or a DropDown, and it
// forwards clicks only inside that fixed strip. SettingRow instead sizes
// the trailing slot to the Control itself (its own Bounds, or a sensible
// default), right-aligns it, and forwards events into it wherever it
// lands — so the Switch/Scale actually toggles/scrubs. FormField is the
// vertical, label-ABOVE-input form counterpart; SettingRow is the
// horizontal, label-BESIDE-control settings counterpart.
//
// Layout (logical, metric-scaled at use):
//
//	┌───────────────────────────────────────────────┐
//	│ Title                             [ Control ]  │
//	│ Subtitle description               (centred)   │
//	│ ───────────────────────────────────────────── │  ← Divider
//	└───────────────────────────────────────────────┘
//
// The Control is positioned + drawn by SettingRow: the caller only sizes
// it (SetBounds W/H, or leave them zero for the default slot) and never
// has to compute where it goes. The text block is vertically centred
// against the row so a tall control and a one-line label still align.
type SettingRow struct {
	Base
	// Title is the setting's name, drawn at the leading edge in OnSurface.
	Title string
	// Subtitle is an optional dim description line under the Title. Empty
	// draws nothing and tightens the text block to a single line.
	Subtitle string
	// Control is the trailing control widget (Switch / Scale / Button /
	// DropDown / ...). nil draws no control and the row is label-only.
	Control Widget
	// Divider draws a 1-pixel Theme.Border separator along the bottom edge
	// (the classic settings-list separator). NewSettingRow defaults it on;
	// a SettingsGroup clears it on its last row.
	Divider bool
}

// SettingRow sizing constants, all LOGICAL pixels scaled at use so the row
// grows under HiDPI / touch density.
const (
	// SettingRowPadX is the horizontal inset of the Title from the left
	// edge and of the Control from the right edge.
	SettingRowPadX = 12
	// SettingRowPadY is the vertical inset above + below the row's content;
	// it sets the row's breathing space around the centred text block.
	SettingRowPadY = 8
	// SettingRowSubtitleGap is the vertical gap between the Title glyph row
	// and the Subtitle glyph row.
	SettingRowSubtitleGap = 2
	// SettingRowControlW is the trailing slot's width used when the Control
	// carries no width of its own (Bounds().W <= 0).
	SettingRowControlW = 48
	// SettingRowControlH is the trailing slot's height used when the Control
	// carries no height of its own (Bounds().H <= 0).
	SettingRowControlH = 24
)

// NewSettingRow builds a SettingRow with the given title and trailing
// control (which may be nil for a label-only row). Subtitle starts empty
// and Divider starts on — the common case in a stacked group; the caller
// clears either as needed.
func NewSettingRow(title string, control Widget) *SettingRow {
	return &SettingRow{Title: title, Control: control, Divider: true}
}

// controlSize reports the trailing slot's width + height: the Control's
// own Bounds when set, falling back to the SettingRowControlW/H defaults
// for any non-positive dimension. A nil Control yields the zero size, so
// Measure treats a label-only row as having no control column.
func (s *SettingRow) controlSize() (int, int) {
	if s.Control == nil {
		return 0, 0
	}
	b := s.Control.Bounds()
	cw, ch := b.W, b.H
	if cw <= 0 {
		cw = scaled(SettingRowControlW)
	}
	if ch <= 0 {
		ch = scaled(SettingRowControlH)
	}
	return cw, ch
}

// controlRect is the trailing Control's rectangle: right-aligned inside
// the SettingRowPadX inset and vertically centred on the row. Only
// meaningful when Control != nil (controlSize returns a real size).
func (s *SettingRow) controlRect() Rect {
	r := s.Bounds()
	cw, ch := s.controlSize()
	return Rect{
		X: r.X + r.W - scaled(SettingRowPadX) - cw,
		Y: r.Y + (r.H-ch)/2,
		W: cw,
		H: ch,
	}
}

// Measure reports the row's natural height at the given outer width
// (width is accepted for parity with the card family but does not change
// the height): the taller of the text block and the Control, plus the
// SettingRowPadY inset top and bottom.
func (s *SettingRow) Measure(width int) int {
	_ = width
	text := s.glyphHeight()
	if s.Subtitle != "" {
		text += scaled(SettingRowSubtitleGap) + s.glyphHeight()
	}
	_, ch := s.controlSize()
	h := text
	if ch > h {
		h = ch
	}
	return h + 2*scaled(SettingRowPadY)
}

// Draw paints the row body, positions + draws the trailing Control (when
// non-nil), paints the Title (and Subtitle, when non-empty) vertically
// centred at the leading edge, and finally the bottom Divider (when set).
// Positioning side effect: the Control's Bounds are updated to its slot.
func (s *SettingRow) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	if s.Control != nil {
		s.Control.SetBounds(s.controlRect())
		s.Control.Draw(p, theme)
	}

	textX := r.X + scaled(SettingRowPadX)
	blockH := s.glyphHeight()
	if s.Subtitle != "" {
		blockH += scaled(SettingRowSubtitleGap) + s.glyphHeight()
	}
	ty := r.Y + (r.H-blockH)/2
	s.drawText(p, textX, ty, s.Title, theme.OnSurface)
	if s.Subtitle != "" {
		sy := ty + s.glyphHeight() + scaled(SettingRowSubtitleGap)
		s.drawText(p, textX, sy, s.Subtitle, dimInk(theme))
	}

	if s.Divider {
		fillRect(p, r.X, r.Y+r.H-1, r.W, 1, theme.Border)
	}
}

// OnEvent routes input to the trailing Control. A click is gated on the
// Control's slot rectangle (a click on the label column is dropped);
// every other event kind — keyboard, in particular — is forwarded
// unconditionally so a focused Control still reacts, mirroring FormField.
// A nil Control is a no-op. Click coordinates are translated into the
// Control's own widget-local space so the Switch/Scale sees a correct hit.
func (s *SettingRow) OnEvent(ev Event) {
	if s.Control == nil {
		return
	}
	if ev.Kind != EventClick {
		s.Control.OnEvent(ev)
		return
	}
	r := s.Bounds()
	cr := s.controlRect()
	if !cr.Contains(ev.X+r.X, ev.Y+r.Y) {
		return
	}
	s.Control.OnEvent(translateEvent(ev, r, cr))
}

// A11y reports the row as a labelled group named by its Title. The trailing
// Control is reached through Children, so WalkA11y announces the row's
// label and then the control (the switch/slider) beneath it — the standard
// "group labels its control" pattern.
func (s *SettingRow) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: s.Title}
}
