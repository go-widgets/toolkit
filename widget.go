// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package toolkit provides a pure-Go widget set for wasmdesk native
// apps. Widgets render per-pixel into an RGBA byte buffer (the SAB
// backed framebuffer wasmbox clients write to) and dispatch input
// events received from the wasmbox compositor.
//
// Design notes:
//
//   - Every widget exposes the same three-method interface, so a
//     container (HBox, VBox, ScrollView, ...) can hold any leaf.
//   - Drawing is allocation-free in the steady state: the widget
//     writes into a caller-owned RGBA slice + reads its theme by
//     reference. Per-frame work is bounded by the widget's bbox.
//   - Coordinates are integer pixels in the caller's surface space;
//     the widget's Rect is its placement within that surface.
//   - Events are pre-translated into widget-local (X, Y) before
//     dispatch by the parent container (HBox/VBox/ScrollView do the
//     hit-testing + offset adjustment).
package toolkit

import "github.com/go-widgets/painter"

// Rect is an axis-aligned rectangle in pixel coordinates. X/Y is the
// top-left corner; W/H are width/height. Aliased to painter.Rect so
// widgets can render on any painter.Painter (PixelPainter for a
// pixel buffer, CellPainter for a terminal grid) without a type
// conversion. Contains() is inherited from painter.Rect.
type Rect = painter.Rect

// EventKind enumerates the input event types a widget can receive.
// The wasmbox compositor routes DOM events through this enum so
// widgets don't depend on the browser's exact event names.
type EventKind int

const (
	// EventClick fires on a mousedown+mouseup pair inside the widget.
	// X/Y carry widget-local coordinates.
	EventClick EventKind = iota
	// EventKeyDown fires when a key is pressed while the widget has
	// focus. Code carries the key name (e.g. "Enter", "ArrowLeft").
	EventKeyDown
	// EventKeyUp is the symmetric release event.
	EventKeyUp
	// EventChar fires for printable character input (post-IME).
	// Code carries the character as a one-rune string.
	EventChar
	// EventCompositionStart fires when an IME composition begins
	// (typically a dead-key press or a CJK IME popup opening). Widgets
	// that echo text (Entry / TextView) should render Code as the
	// "in-progress" preview string, underlined or ghosted, WITHOUT
	// committing it to their buffer. The host is responsible for
	// resolving the composition via EventCompositionUpdate ticks and
	// finally an EventChar (post-commit).
	EventCompositionStart
	// EventCompositionUpdate refreshes the preview string mid-flow.
	// Code carries the current, un-committed composed text.
	EventCompositionUpdate
	// EventCompositionEnd fires when the composition is either
	// committed (host follows up with EventChar carrying the same
	// text) or cancelled (host does NOT send an EventChar and the
	// widget discards the preview).
	EventCompositionEnd
)

// Event is one input event delivered to a widget. The parent container
// translated mouse coordinates into widget-local pixels; Code is the
// key/char text for keyboard events.
type Event struct {
	Kind EventKind
	X, Y int
	Code string
}

// Widget is the toolkit's single core abstraction. Every widget --
// Button, Label, TextInput, HBox, ScrollView, ... -- implements it.
// Containers themselves are widgets too: a VBox passes Draw / OnEvent
// to its children after offsetting coordinates by the child's Rect.
type Widget interface {
	// Bounds returns the widget's placement within its parent surface.
	// Used by containers for hit-testing + relative-coordinate translation.
	Bounds() Rect

	// SetBounds updates the placement. Containers call this during
	// layout to position children.
	SetBounds(r Rect)

	// Draw paints the widget onto the Painter using the supplied
	// theme. The Painter's back-end decides whether the primitives
	// land as pixels (browser canvas, native window, image file) or
	// cells (terminal grid). Widgets MUST NOT draw outside their
	// Bounds() rectangle.
	Draw(p painter.Painter, theme *Theme)

	// HitTest reports whether (px, py) (in surface coordinates) falls
	// on a sensitive part of the widget. Most widgets just return
	// Bounds().Contains(px, py); transparent or overlapping widgets
	// may return false even within their bounds.
	HitTest(px, py int) bool

	// OnEvent delivers an input event whose X/Y are WIDGET-LOCAL.
	// The widget mutates its internal state + may schedule a redraw
	// (the caller is responsible for invoking Draw again).
	OnEvent(ev Event)
}

// Base provides default Bounds/SetBounds/HitTest impls so a widget
// embedding it only has to implement Draw + OnEvent. Embedding is
// optional but convenient.
type Base struct {
	rect Rect
}

func (b *Base) Bounds() Rect              { return b.rect }
func (b *Base) SetBounds(r Rect)          { b.rect = r }
func (b *Base) HitTest(px, py int) bool   { return b.rect.Contains(px, py) }
func (b *Base) OnEvent(ev Event) { _ = ev /* no-op default; widgets override */ }
func (b *Base) Draw(p painter.Painter, theme *Theme) {
	// no-op default; concrete widgets override Draw.
	_, _ = p, theme
}
