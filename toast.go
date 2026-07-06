// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ToastKind selects the semantic colour of a Toast pill. ToastInfo
// reuses the theme's Accent (the same tint used by focus rings + the
// Notification banner); the other three carry hard-coded shades tuned
// for meaning -- green for success, amber for warning, red for error --
// mirroring AlertKind so a Toast and an Alert with the same kind read
// as visual siblings.
type ToastKind int

const (
	// ToastInfo is a neutral heads-up ("Copied to clipboard"). Rendered
	// in Theme.Accent so it matches the app's own accent colour.
	ToastInfo ToastKind = iota
	// ToastSuccess signals a completed operation ("File uploaded"). Green.
	ToastSuccess
	// ToastWarning flags a non-fatal issue ("Battery low"). Amber.
	ToastWarning
	// ToastError signals a failure the user must address ("Network
	// unreachable"). Red.
	ToastError
)

// Toast is a short-lived, self-dismissing pill that slides in over the
// app's normal frame, holds for a few ticks, then hides itself.
// Distinct from Notification in three ways:
//
//  1. Toast carries a Kind (like Alert) so the pill's fill colour
//     conveys severity at a glance; Notification is always Accent.
//  2. Toast's Life = 0 sentinel means "sticky" (do not auto-hide),
//     letting a host post a persistent pill without a matching
//     Life-budget assignment.
//  3. Toast is designed to STACK: several Toast values can share the
//     same host, each Bounds()'d to its own row; the host mutates
//     Visible + Life directly and iterates Tick over the collection.
//
// The host drives Life via Tick() from its own animation loop
// (typically a rAF tick).
type Toast struct {
	Base
	Text    string
	Kind    ToastKind
	Visible bool

	// Life is the number of Tick() calls remaining before the toast
	// auto-hides. The zero value is a sentinel meaning "sticky": Tick
	// is a no-op until the host assigns a positive Life. When Life is
	// positive, each Tick decrements it; when the countdown reaches
	// zero Visible is cleared.
	Life int
}

// ToastPadX / ToastPadY are the internal margin between the pill
// edges and the text. Slightly tighter than Notification's 12/8 so
// several stacked pills read as a compact column.
const (
	ToastPadX = 10
	ToastPadY = 6
)

// NewToast builds a hidden Toast with the given text + kind. The host
// sets Visible=true (typically via a Show helper it wraps around the
// widget) + assigns Life to arm the auto-dismiss countdown.
func NewToast(text string, kind ToastKind) *Toast {
	return &Toast{Text: text, Kind: kind}
}

// toastFace maps a Kind to a background colour. ToastInfo defers to
// the theme so it blends with the app's accent choice; the other
// three carry fixed shades since the theme doesn't (and shouldn't)
// grow semantic-colour slots for every widget that wants one. Shades
// match Alert's Success/Warning/Error tuples so a Toast + Alert with
// the same Kind look like siblings on screen.
func toastFace(kind ToastKind, theme *Theme) RGBA {
	switch kind {
	case ToastSuccess:
		return RGB(0x2E, 0x8B, 0x57) // sea green
	case ToastWarning:
		return RGB(0xE0, 0xA0, 0x30) // amber
	case ToastError:
		return RGB(0xC0, 0x30, 0x30) // brick red
	default: // ToastInfo (also any out-of-range Kind values)
		return theme.Accent
	}
}

// Draw paints the pill when Visible. Filled Kind-coloured panel with a
// 1-px Border stroke; Text in the accent-inverted ink so it stays
// legible against every Kind's face. Nothing drawn when hidden.
func (t *Toast) Draw(p painter.Painter, theme *Theme) {
	if !t.Visible {
		return
	}
	r := t.Bounds()
	face := toastFace(t.Kind, theme)
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	DrawText(p, r.X+ToastPadX, r.Y+ToastPadY, t.Text, accentInk(theme))
}

// Tick decrements Life by 1 when Life is positive. When the countdown
// reaches 0 the toast auto-hides. Life == 0 is a sticky sentinel and
// leaves Visible untouched, so a host may post a persistent toast by
// leaving Life at its zero value.
func (t *Toast) Tick() {
	if t.Life <= 0 {
		return
	}
	t.Life--
	if t.Life == 0 {
		t.Visible = false
	}
}
