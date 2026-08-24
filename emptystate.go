// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// emptyStateGap is the default vertical gap (logical px) between an EmptyState's
// icon, message and caption. emptyStateIcon is the default logical-px square the
// optional icon is laid out at.
const (
	emptyStateGap  = 8
	emptyStateIcon = 48
)

// EmptyState is the centred "there is nothing here yet" placeholder a view shows
// in place of its content — a "Folder is empty" line, an optional glyph above it,
// an optional secondary caption below. Every app that needed one re-derived the
// centring by hand (measure the text, halve the leftover, DrawText), which is the
// boilerplate this widget removes: it composes a centred [Label] for the message
// (RoleText, so a screen reader announces it) with an optional icon widget and an
// optional muted caption, stacked and vertically centred as a group within its
// bounds.
//
// The message is MVVM-only, exposed as an [mvvm.Observable] via [EmptyState.Message];
// a host binds it (Set / Subscribe / two-way) rather than mutating a field. The
// widget is decorative (it paints text, takes no input), and presentational to
// the accessibility tree — its child Label carries the announced text.
type EmptyState struct {
	Base
	// Icon, when non-nil, is laid out as a square of IconSize logical pixels,
	// centred above the message. The zero value omits the glyph entirely.
	Icon Widget
	// IconSize is the icon's logical-pixel side length; a non-positive value
	// uses emptyStateIcon. Ignored when Icon is nil.
	IconSize int
	// Gap is the vertical gap (logical px) between the icon, message and
	// caption; a non-positive value uses emptyStateGap.
	Gap int

	msg     *Label
	caption *Label
}

// NewEmptyState builds an EmptyState whose primary message is message, centred
// horizontally. Add an icon by setting Icon, and a secondary line with
// [EmptyState.SetCaption].
func NewEmptyState(message string) *EmptyState {
	e := &EmptyState{}
	e.msg = NewLabel(message)
	e.msg.Align = AlignCenter
	return e
}

// Message is the primary text as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable message field.
func (e *EmptyState) Message() *mvvm.Observable[string] { return e.msg.Text() }

// SetCaption sets (creating on first use) the muted secondary line drawn under
// the message and returns the EmptyState for fluent chaining. The caption text is
// itself reactive via [EmptyState.Caption].
func (e *EmptyState) SetCaption(text string) *EmptyState {
	if e.caption == nil {
		e.caption = NewLabel(text)
		e.caption.Align = AlignCenter
	} else {
		e.caption.Text().Set(text)
	}
	return e
}

// Caption is the secondary line's text as a shared [mvvm.Observable], created on
// first access so a host can bind it even before [EmptyState.SetCaption]. Once
// present the caption is drawn (in a muted ink) under the message.
func (e *EmptyState) Caption() *mvvm.Observable[string] {
	if e.caption == nil {
		e.caption = NewLabel("")
		e.caption.Align = AlignCenter
	}
	return e.caption.Text()
}

// gap is the vertical spacing in device pixels.
func (e *EmptyState) gap() int {
	if e.Gap > 0 {
		return scaled(e.Gap)
	}
	return scaled(emptyStateGap)
}

// iconSquare is the icon's device-pixel side length, or 0 when there is no icon.
func (e *EmptyState) iconSquare() int {
	if e.Icon == nil {
		return 0
	}
	if e.IconSize > 0 {
		return scaled(e.IconSize)
	}
	return scaled(emptyStateIcon)
}

// SetBounds stacks the optional icon, the message and the optional caption and
// centres the whole group vertically within the bounds; each text line spans the
// full width so its own centre-alignment centres it horizontally.
func (e *EmptyState) SetBounds(r Rect) {
	e.Base.SetBounds(r)
	is := e.iconSquare()
	g := e.gap()
	msgH := e.msg.faceFor().Height()
	capH := 0
	if e.caption != nil {
		capH = e.caption.faceFor().Height()
	}

	total := msgH
	if e.Icon != nil {
		total += is + g
	}
	if e.caption != nil {
		total += g + capH
	}

	top := r.Y + (r.H-total)/2
	if e.Icon != nil {
		e.Icon.SetBounds(Rect{X: r.X + (r.W-is)/2, Y: top, W: is, H: is})
		top += is + g
	}
	e.msg.SetBounds(Rect{X: r.X, Y: top, W: r.W, H: msgH})
	top += msgH
	if e.caption != nil {
		top += g
		e.caption.SetBounds(Rect{X: r.X, Y: top, W: r.W, H: capH})
	}
}

// Draw paints the icon (if any), the message and the caption; the caption is
// tinted with the theme's muted ink so it reads as secondary.
func (e *EmptyState) Draw(p painter.Painter, theme *Theme) {
	if e.Icon != nil {
		e.Icon.Draw(p, theme)
	}
	e.msg.Draw(p, theme)
	if e.caption != nil {
		e.caption.Ink = mutedInk(theme)
		e.caption.Draw(p, theme)
	}
}

// Children yields the icon (if any), the message and the caption (if any) in
// visual order, so the accessibility and text-selection walks reach them.
func (e *EmptyState) Children() []Widget {
	out := make([]Widget, 0, 3)
	if e.Icon != nil {
		out = append(out, e.Icon)
	}
	out = append(out, e.msg)
	if e.caption != nil {
		out = append(out, e.caption)
	}
	return out
}

// A11y reports the EmptyState as presentational: its message Label (RoleText)
// carries the announced text, so the wrapper itself adds no semantics.
func (e *EmptyState) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }
