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
//
// A Toast may also carry a single action ("Copied — Undo"): set
// ActionLabel + Action to render a small button inside the pill's
// right edge. Leaving ActionLabel empty (the zero value) opts out --
// the pill renders + sizes exactly as a plain message toast.
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

	// ActionLabel, when non-empty, arms a small action button rendered
	// right-aligned inside the pill (e.g. "Undo") and makes OnEvent
	// route clicks landing in that button to Action. Empty (the zero
	// value) means "no action" -- Draw + AnchorIn behave exactly as a
	// pre-action Toast.
	ActionLabel string
	// Action is invoked when the action button is clicked. Nil-safe:
	// clicking the button still dismisses the toast when Action is nil.
	Action func()
}

// ToastPadX / ToastPadY are the internal margin between the pill
// edges and the text. Slightly tighter than Notification's 12/8 so
// several stacked pills read as a compact column.
const (
	ToastPadX = 10
	ToastPadY = 6
	// ToastMargin is the gap between a corner-anchored toast and the host
	// edges; ToastGap is the vertical space between stacked toasts.
	ToastMargin = 12
	ToastGap    = 6
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

// actionSlotW returns the pixel width of the action-button zone -- a
// ToastPadX gap from the message text, a 1-px divider, then the
// button's own ToastPadX padding on both sides of ActionLabel -- or 0
// when ActionLabel is empty. AnchorIn folds it into the toast's total
// width; Draw + OnEvent both derive the button's on-pill position from
// it, so sizing, painting + hit-testing always agree on the same box.
func (t *Toast) actionSlotW() int {
	if t.ActionLabel == "" {
		return 0
	}
	return 3*ToastPadX + 1 + t.textWidth(t.ActionLabel)
}

// AnchorIn sizes the toast to its Text (+ action button, when present) and
// positions it at corner of host, stacked at row index (0 = the row nearest
// the docked edge). Top corners stack downward, bottom corners upward, so a
// host can lay out a column of toasts by calling AnchorIn once per visible
// toast with an increasing index.
func (t *Toast) AnchorIn(host Rect, corner Corner, index int) {
	w := t.textWidth(t.Text) + 2*ToastPadX
	if t.ActionLabel != "" {
		// The action slot's own trailing ToastPadX already plays the
		// role of the pill's plain right-edge padding, so only the
		// slot's extra width (gap + divider + button padding + label)
		// is added on top of the base two-sided text padding.
		w += t.actionSlotW() - ToastPadX
	}
	h := t.glyphHeight() + 2*ToastPadY
	offset := index * (h + ToastGap)
	t.SetBounds(anchorCorner(host, w, h, corner, ToastMargin, offset))
}

// Draw paints the pill when Visible. Filled Kind-coloured panel with a
// 1-px Border stroke; Text in the accent-inverted ink so it stays
// legible against every Kind's face. When ActionLabel is set, a 1-px
// Border divider + the action label (same accent-inverted ink) are
// painted right-aligned inside the pill. Nothing drawn when hidden.
func (t *Toast) Draw(p painter.Painter, theme *Theme) {
	if !t.Visible {
		return
	}
	r := t.Bounds()
	face := toastFace(t.Kind, theme)
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	ink := accentInk(theme)
	t.drawText(p, r.X+ToastPadX, r.Y+ToastPadY, t.Text, ink)

	if t.ActionLabel != "" {
		aw := t.actionSlotW()
		ax := r.X + r.W - aw
		fillRect(p, ax+ToastPadX, r.Y, 1, r.H, theme.Border)
		t.drawText(p, ax+2*ToastPadX+1, r.Y+ToastPadY, t.ActionLabel, ink)
	}
}

// OnEvent runs Action + hides the toast when a click lands inside the
// action button; a click anywhere else in the pill (or when
// ActionLabel is empty) is a no-op. ev.X is widget-local, matching
// SplitButton's arrow-slot convention. Action is nil-checked, so an
// action-less callback still dismisses the toast on click.
func (t *Toast) OnEvent(ev Event) {
	if ev.Kind != EventClick || t.ActionLabel == "" {
		return
	}
	r := t.Bounds()
	btnW := t.actionSlotW() - ToastPadX // slot minus the leading text-gap
	if ev.X >= r.W-btnW {
		if t.Action != nil {
			t.Action()
		}
		t.Visible = false
	}
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
