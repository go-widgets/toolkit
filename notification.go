// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Notification is a transient toast — an auto-dismissing banner that
// slides in over the app's normal frame, holds for a few ticks, then
// hides itself. Cousin of Tooltip (both are informational overlays)
// but with three key differences:
//   1. Notification is time-bounded (Tick decrements Life; hides at 0).
//   2. Notification is positioned by the host (typically top-right or
//      bottom-centre), NOT anchored to a source widget.
//   3. Notification stays up while the user is doing something else —
//      Tooltip requires the mouse to hover over its anchor.
//
// The host drives Life via Tick() from its own animation loop
// (typically a rAF tick). One Notification instance can be reused —
// call Show(text) to re-arm it with a fresh Life budget.
type Notification struct {
	Base
	Text    string
	Visible bool

	// Life is the number of Tick() calls remaining before the
	// notification auto-hides. NotificationLife (~180 ≈ 3 s at 60 Hz)
	// is a reasonable default; Show() re-arms it. Set directly for
	// long-lived notifications (e.g. Life = 3600 for a persistent
	// "network offline" banner the host manually Hide()s later).
	Life int
}

// NotificationPadX / NotificationPadY / NotificationLife are the
// visual + timing defaults; a caller wanting shorter or louder
// toasts overrides them per-instance (via SetBounds + direct Life
// assignment).
const (
	NotificationPadX = 12
	NotificationPadY = 8
	NotificationLife = 180
)

// NewNotification builds a hidden notification with the given text +
// the default Life budget pre-armed (so a caller who forgets to call
// Show still gets a sensible time-out on the first Tick loop).
func NewNotification(text string) *Notification {
	return &Notification{Text: text, Life: NotificationLife}
}

// Show makes the notification visible + resets Life to
// NotificationLife. Bounds are auto-sized to the text width + the
// standard padding; the host is responsible for positioning
// (SetBounds) BEFORE calling Show — Show only refreshes the width to
// match the current Text.
func (n *Notification) Show(text string) {
	n.Text = text
	n.Visible = true
	n.Life = NotificationLife
	r := n.Bounds()
	r.W = TextWidth(text) + 2*NotificationPadX
	r.H = GlyphHeight + 2*NotificationPadY
	n.SetBounds(r)
}

// Hide dismisses the notification immediately (independent of Life).
func (n *Notification) Hide() {
	n.Visible = false
	n.Life = 0
}

// Tick decrements Life by 1. When Life reaches 0, the notification
// auto-hides. The host calls this from its animation loop; a
// rAF-driven caller ticks 60 Hz so NotificationLife = 180 ≈ 3 s.
// Callers wanting a paused notification (freeze on user hover) just
// skip the Tick during the pause.
func (n *Notification) Tick() {
	if !n.Visible {
		return
	}
	n.Life--
	if n.Life <= 0 {
		n.Hide()
	}
}

// Draw paints the toast when Visible. Filled Accent panel with a
// 1-px Border stroke, Text in the Background ink (inverted for
// contrast). Nothing drawn when hidden.
func (n *Notification) Draw(p painter.Painter, theme *Theme) {
	if !n.Visible {
		return
	}
	r := n.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Accent)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	DrawText(p, r.X+NotificationPadX, r.Y+NotificationPadY, n.Text, theme.Background)
}
