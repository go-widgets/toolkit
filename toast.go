// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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
//     same host, each Bounds()'d to its own row; the host drives each
//     toast's Visible + Life observables and iterates Tick over the
//     collection.
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
	Text string
	Kind ToastKind

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

	// MaxW, when positive, is the widest the pill may be drawn. Text is wrapped
	// across as many rows as it needs so the pill fits, instead of growing past
	// its host.
	//
	// ⛔ WITHOUT IT A LONG SENTENCE IS UNREADABLE, not merely untidy. A toast
	// sizes itself to its widest line with no upper bound, and a host that
	// docks it to a centre anchor then paints a pill wider than the view -- so
	// BOTH ENDS are cut and the reader gets the middle of a sentence. Measured
	// on a 1920-wide view at the size xrdesk draws its notices: "the camera was
	// refused; it is turned on again in System Settings > Privacy & Security >
	// Camera" came to 2373px, and a longer refusal to 4975px -- 2.6 times the
	// width it had to fit in.
	//
	// The zero value keeps the old behaviour exactly: no wrapping, no measuring,
	// one line. Ignored when Lines is set, since a caller supplying its own rows
	// has already decided where they break.
	MaxW int

	// wrapped memoises the last wrap, because lines() is asked three times per
	// frame (sizing, height, drawing) and wrapping is a measure per word.
	//
	// ⚠ THE KEY IS (text, width, glyph height) AND NOT THE FONT ITSELF. A Font
	// is an interface and comparing one can panic on an uncomparable dynamic
	// type. Two different fonts of the SAME height would therefore reuse a wrap
	// computed for the other -- a mis-wrap, never a crash, and it corrects
	// itself the moment the text or the width changes.
	wrapped              []string
	wrapText             string
	wrapWidth, wrapGlyph int

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

	// visible is the reactive show/hide state; life is the auto-dismiss
	// countdown. Both are MVVM-only (no settable field) — see [Toast.Visible]
	// and [Toast.Life].
	visible *mvvm.Observable[bool]
	life    *mvvm.Observable[int]
}

// Visible is the toast's show/hide state as a shared [mvvm.Observable]: a host
// binds it (Set / Subscribe / two-way) — there is no settable Visible field.
// Draw paints the pill exactly while it is true. The zero value is false, so a
// bare &Toast{} starts hidden.
func (t *Toast) Visible() *mvvm.Observable[bool] {
	if t.visible == nil {
		t.visible = mvvm.NewObservable(false)
	}
	return t.visible
}

// Life is the number of Tick() calls remaining before the toast auto-hides, as
// a shared [mvvm.Observable] — there is no settable Life field. The zero value
// is a sentinel meaning "sticky": Tick is a no-op until the host Sets a positive
// Life. When Life is positive, each Tick decrements it; when the countdown
// reaches zero Visible is cleared.
func (t *Toast) Life() *mvvm.Observable[int] {
	if t.life == nil {
		t.life = mvvm.NewObservable(0)
	}
	return t.life
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
// Sets Visible().Set(true) (typically via a Show helper it wraps around
// the widget) + Sets Life to arm the auto-dismiss countdown.
func NewToast(text string, kind ToastKind) *Toast {
	return &Toast{
		Text:    text,
		Kind:    kind,
		visible: mvvm.NewObservable(false),
		life:    mvvm.NewObservable(0),
	}
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

// lines returns the message rows: Lines when non-empty, else Text wrapped to
// MaxW, else a single-element slice holding Text (the backward-compatible
// default, and what a zero MaxW still gives).
func (t *Toast) lines() []string {
	if len(t.Lines) > 0 {
		return t.Lines
	}
	inner := t.wrapWidthFor()
	if inner <= 0 {
		return []string{t.Text}
	}
	f := t.EffectiveFont()
	gh := f.Height()
	if t.wrapped != nil && t.wrapText == t.Text && t.wrapWidth == inner && t.wrapGlyph == gh {
		return t.wrapped
	}
	out := wrapText(f, t.Text, inner)
	if len(out) == 0 {
		// All-whitespace or empty: one empty row, so the pill keeps a height
		// and every caller still gets exactly one line as it did before.
		out = []string{t.Text}
	}
	t.wrapped, t.wrapText, t.wrapWidth, t.wrapGlyph = out, t.Text, inner, gh
	return out
}

// wrapWidthFor is how many pixels the message itself may occupy inside a pill
// capped at MaxW: the cap less the pill's own padding, its icon slot and its
// action zone. Zero or less means "do not wrap" -- either MaxW is unset, or the
// furniture already leaves the text no room, and a pill that overflows is still
// better than one with no words in it.
func (t *Toast) wrapWidthFor() int {
	if t.MaxW <= 0 {
		return 0
	}
	inner := t.MaxW - 2*ToastPadX - t.iconSlotW()
	if aw := t.actionsW(); aw > 0 {
		// AnchorIn adds actionsW()-ToastPadX on top of the two-sided padding,
		// so that is exactly what the text loses.
		inner -= aw - ToastPadX
	}
	return inner
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
	if !t.Visible().Get() {
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

// ButtonRects returns the laid-out rectangle of each action button in the
// toast's local (painted) coordinate space: X measured from the pill's LEFT
// edge and Y from its TOP (independent of the toast's current Bounds() origin),
// each rect spanning the full pill height. The i-th rect is the click target for
// the i-th action -- the Actions slice in order, else the single button
// synthesised from the legacy ActionLabel/Action pair. It returns nil when the
// toast carries no actions.
//
// The rects use the toast's current Bounds() width + height, so call it AFTER
// sizing the pill (AnchorIn, or a direct SetBounds). A host that hit-tests a
// click itself -- rather than routing it through OnEvent -- maps the click into
// the toast's local space (click minus the pill's top-left) and finds the button
// whose rect contains it; OnEvent hit-tests against these very rects, so the two
// paths can never disagree.
func (t *Toast) ButtonRects() []Rect {
	a := t.acts()
	if len(a) == 0 {
		return nil
	}
	r := t.Bounds()
	rects := make([]Rect, len(a))
	bx := r.W - t.actionsW() + ToastPadX // local x of the first button
	for i, act := range a {
		seg := 1 + 2*ToastPadX + t.textWidth(act.Label)
		rects[i] = Rect{X: bx, Y: 0, W: seg, H: r.H}
		bx += seg
	}
	return rects
}

// OnEvent runs the clicked button's Callback + hides the toast when a click
// lands inside an action button; a click anywhere else in the pill (or when
// there are no actions) is a no-op. ev.X/ev.Y are widget-local. The Callback is
// nil-checked, so an action-less button still dismisses the toast on click. The
// hit-test runs against [Toast.ButtonRects], the same geometry a host reads to
// route a click itself.
func (t *Toast) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	a := t.acts()
	for i, br := range t.ButtonRects() {
		if ev.X >= br.X && ev.X < br.X+br.W && ev.Y >= br.Y && ev.Y < br.Y+br.H {
			if a[i].Callback != nil {
				a[i].Callback()
			}
			t.Visible().Set(false)
			return
		}
	}
}

// Tick decrements Life by 1 when Life is positive. When the countdown
// reaches 0 the toast auto-hides. Life == 0 is a sticky sentinel and
// leaves Visible untouched, so a host may post a persistent toast by
// leaving Life at its zero value.
func (t *Toast) Tick() {
	if t.Life().Get() <= 0 {
		return
	}
	t.Life().Set(t.Life().Get() - 1)
	if t.Life().Get() == 0 {
		t.Visible().Set(false)
	}
}
