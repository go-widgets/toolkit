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
//
// Three optional enrichments layer on top without disturbing the plain
// path (Icon nil, Lines empty, Actions empty renders byte-identically to
// the original single-line / single-action Toast):
//
//   - Icon: an [IconFunc] vector glyph or an RGBA image ([Pixels]/[IW]/[IH])
//     painted, vertically centred, to the LEFT of the text.
//   - Lines: distinct message rows (e.g. a bold-reading title line plus a
//     body line) stacked instead of a single joined Text.
//   - Actions: a slice of ([ToastAction]) buttons (each a label + callback)
//     laid out right-to-left along the pill's right edge, superseding the
//     single ActionLabel/Action pair.
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
	// pre-action Toast. Superseded by Actions when that slice is non-empty.
	ActionLabel string
	// Action is invoked when the action button is clicked. Nil-safe:
	// clicking the button still dismisses the toast when Action is nil.
	Action func()

	// Lines, when non-empty, supplies the message as distinct rows (a title
	// line plus one or more body lines) stacked top-to-bottom, instead of the
	// single joined Text. The zero value (nil/empty) falls back to Text, so a
	// one-line toast is unchanged.
	Lines []string

	// Actions, when non-empty, supplies several action buttons (superseding
	// the single ActionLabel/Action pair). Buttons are laid out along the
	// right edge in slice order, each with its own divider + label. The zero
	// value (nil/empty) falls back to the ActionLabel/Action pair.
	Actions []ToastAction

	// Icon paints a vector glyph to the left of the text when Pixels is not a
	// valid image. May be nil (no icon).
	Icon IconFunc
	// Pixels is an optional RGBA image (IW*IH*4 bytes) drawn to the left of the
	// text instead of Icon, aspect-preserved + centred. IW/IH are its source
	// dimensions.
	Pixels []byte
	IW, IH int
}

// ToastAction is one button in a multi-action Toast: a Label the user clicks
// and a Callback run on click. Callback is nil-safe (the toast still dismisses
// when it is nil), matching the legacy single-Action contract.
type ToastAction struct {
	Label    string
	Callback func()
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
	// ToastLineGap is the vertical space between stacked message lines in a
	// multi-line (Lines) toast. Irrelevant to a single-line toast.
	ToastLineGap = 2
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

// lines returns the message rows: Lines when non-empty, else a single-element
// slice holding Text (the backward-compatible default).
func (t *Toast) lines() []string {
	if len(t.Lines) > 0 {
		return t.Lines
	}
	return []string{t.Text}
}

// acts returns the action buttons: Actions when non-empty, else a single-element
// slice from the legacy ActionLabel/Action pair when ActionLabel is set, else
// nil (a plain, action-less toast).
func (t *Toast) acts() []ToastAction {
	if len(t.Actions) > 0 {
		return t.Actions
	}
	if t.ActionLabel != "" {
		return []ToastAction{{Label: t.ActionLabel, Callback: t.Action}}
	}
	return nil
}

// hasImage reports whether Pixels is a usable RGBA buffer for the source dims.
func (t *Toast) hasImage() bool {
	return t.IW > 0 && t.IH > 0 && len(t.Pixels) >= t.IW*t.IH*4
}

// hasIcon reports whether the toast paints a leading icon (image or vector).
func (t *Toast) hasIcon() bool { return t.hasImage() || t.Icon != nil }

// iconSlotW is the pixel width reserved left of the text for the icon (a square
// the glyph-line tall plus a ToastPadX gap), or 0 when no icon is set.
func (t *Toast) iconSlotW() int {
	if !t.hasIcon() {
		return 0
	}
	return t.glyphHeight() + ToastPadX
}

// actionsW is the total pixel width of the action-button zone: a leading
// ToastPadX gap from the message text, then per button a 1-px divider plus the
// button's own ToastPadX padding on both sides of its label. Returns 0 when
// there are no actions. For a single button this equals the pre-multi-action
// slot width (3*ToastPadX + 1 + label), keeping the legacy layout byte-exact.
func (t *Toast) actionsW() int {
	a := t.acts()
	if len(a) == 0 {
		return 0
	}
	w := ToastPadX // leading gap from the message text
	for _, act := range a {
		w += 1 + 2*ToastPadX + t.textWidth(act.Label)
	}
	return w
}

// linesW is the widest message row, in pixels.
func (t *Toast) linesW() int {
	w := 0
	for _, ln := range t.lines() {
		if lw := t.textWidth(ln); lw > w {
			w = lw
		}
	}
	return w
}

// contentH is the pill's inner text-block height: N line boxes plus the gaps
// between them. For a single line this is exactly glyphHeight (no gap), so the
// pill height reduces to the pre-multi-line value.
func (t *Toast) contentH() int {
	n := len(t.lines())
	return n*t.glyphHeight() + (n-1)*ToastLineGap
}

// AnchorIn sizes the toast to its content (icon + text lines + action buttons,
// each present) and positions it at corner of host, stacked at row index (0 =
// the row nearest the docked edge). Top corners stack downward, bottom corners
// upward, so a host can lay out a column of toasts by calling AnchorIn once per
// visible toast with an increasing index.
func (t *Toast) AnchorIn(host Rect, corner Corner, index int) {
	w := t.iconSlotW() + t.linesW() + 2*ToastPadX
	if t.actionsW() > 0 {
		// The action zone's own trailing ToastPadX already plays the
		// role of the pill's plain right-edge padding, so only the
		// zone's extra width (gap + dividers + button padding + labels)
		// is added on top of the base two-sided text padding.
		w += t.actionsW() - ToastPadX
	}
	h := t.contentH() + 2*ToastPadY
	offset := index * (h + ToastGap)
	t.SetBounds(anchorCorner(host, w, h, corner, ToastMargin, offset))
}

// Draw paints the pill when Visible. Filled Kind-coloured panel with a 1-px
// Border stroke; the icon (when set) at the left, then the message line(s) in
// the accent-inverted ink so they stay legible against every Kind's face. Each
// action button is a 1-px Border divider plus its label, laid out along the
// right edge in Actions order. Nothing drawn when hidden.
func (t *Toast) Draw(p painter.Painter, theme *Theme) {
	if !t.Visible {
		return
	}
	r := t.Bounds()
	face := toastFace(t.Kind, theme)
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	ink := accentInk(theme)

	textX := r.X + ToastPadX
	if t.hasIcon() {
		sz := t.glyphHeight()
		iconR := Rect{X: r.X + ToastPadX, Y: r.Y + (r.H-sz)/2, W: sz, H: sz}
		if t.hasImage() {
			img := Image{Pixels: t.Pixels, W: t.IW, H: t.IH, Scale: ScaleFit}
			img.SetBounds(iconR)
			img.Draw(p, theme)
		} else {
			t.Icon(p, iconR, ink)
		}
		textX += t.iconSlotW()
	}

	gh := t.glyphHeight()
	for i, ln := range t.lines() {
		ly := r.Y + ToastPadY + i*(gh+ToastLineGap)
		t.drawText(p, textX, ly, ln, ink)
	}

	if aw := t.actionsW(); aw > 0 {
		bx := r.X + r.W - aw + ToastPadX // skip the leading text-gap
		by := r.Y + (r.H-gh)/2
		for _, act := range t.acts() {
			fillRect(p, bx, r.Y, 1, r.H, theme.Border)
			t.drawText(p, bx+1+ToastPadX, by, act.Label, ink)
			bx += 1 + 2*ToastPadX + t.textWidth(act.Label)
		}
	}
}

// OnEvent runs the clicked button's Callback + hides the toast when a click
// lands inside an action button; a click anywhere else in the pill (or when
// there are no actions) is a no-op. ev.X is widget-local. The Callback is
// nil-checked, so an action-less button still dismisses the toast on click.
func (t *Toast) OnEvent(ev Event) {
	a := t.acts()
	if ev.Kind != EventClick || len(a) == 0 {
		return
	}
	r := t.Bounds()
	bx := r.W - t.actionsW() + ToastPadX // local x of the first button
	for _, act := range a {
		seg := 1 + 2*ToastPadX + t.textWidth(act.Label)
		if ev.X >= bx && ev.X < bx+seg {
			if act.Callback != nil {
				act.Callback()
			}
			t.Visible = false
			return
		}
		bx += seg
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
