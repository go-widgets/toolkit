// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Button is a clickable rectangle with a centred label. Paints a
// 1-pixel border in Theme.Border on a Theme.Surface body; hovered /
// pressed states cycle through SurfaceAlt + Accent so the user sees
// click feedback before the callback fires.
//
// Wire a handler via OnClick; the button calls it from OnEvent when
// it receives an EventClick. Callers re-paint via Draw after any
// state mutation (the toolkit doesn't drive its own frame loop --
// the wasmbox compositor's tick is the redraw trigger).
type Button struct {
	Base
	focusState
	// label is the button caption, reactive via the Label() accessor.
	label   *mvvm.Observable[string]
	OnClick func()
	Style   ButtonStyle // resting appearance; default is ButtonDefault

	// Icon, when set, lets the host paint a real vector glyph in the button's
	// face instead of the text Label — the seam other widgets use for
	// host-supplied icons (mirrors Banner.Icon / SearchEntry.Icon). Draw invokes
	// Icon with the button's full bounds and the current face ink (which already
	// carries the pressed / disabled tint), so the glyph tracks every button
	// state; the callback is responsible for centring + sizing itself within the
	// rect. When nil the button falls back to drawing Label, so existing callers
	// are unaffected and headless renders still show text.
	Icon func(p painter.Painter, r Rect, ink RGBA)

	// selected is a sticky, app-managed "active" state (a pill in a selector, the
	// current tab/provider): when true the button fills with Accent regardless of
	// Style. The app drives it from its own model via the Selected() Observable;
	// the button never flips it itself.
	selected *mvvm.Observable[bool]

	// leadingIcon and shortcut are OPTIONAL, reactive adornments drawn ALONGSIDE the
	// caption (unlike the Icon field above, which replaces it): a small vector glyph
	// before the label, and a right-aligned keyboard-shortcut hint (e.g. "⌘F") in a
	// muted tone. Both are lazily-created Observables via LeadingIcon()/Shortcut();
	// a button that never sets them draws exactly as before.
	leadingIcon *mvvm.Observable[IconFunc]
	shortcut    *mvvm.Observable[string]

	// PressFeedback shows the pressed face on EventClick (until EventMouseUp).
	// NewButton enables it; set it false to opt a button out (e.g. one whose
	// action already navigates away so the flash would just flicker).
	PressFeedback bool

	// Flat suppresses the button's own rounded border + fill rounding, painting a
	// square-cornered face only — so it can sit inside a container that owns the
	// shared chrome (see ButtonGroup, which sets it on its members). The zero
	// value (false) keeps the standalone rounded look, so existing callers are
	// unaffected.
	Flat bool

	hovered bool
	pressed bool
}

// ButtonStyle selects a button's resting fill, giving a layout visual hierarchy
// (macOS "prominent"/default/secondary buttons). Hover + press still override
// the fill on top of the style.
type ButtonStyle int

const (
	// ButtonDefault is a Surface-faced button (the plain look).
	ButtonDefault ButtonStyle = iota
	// ButtonProminent is filled with Accent + accent-foreground text -- the
	// primary/default action (e.g. a calculator's operator keys, "OK").
	ButtonProminent
	// ButtonSecondary is filled with SurfaceAlt -- a muted grey key that sits
	// between Default and Prominent (e.g. a calculator's C / +/- / % keys).
	ButtonSecondary
	// ButtonDanger is a Surface-faced button with a red border + red label -- a
	// destructive action (Delete, Remove).
	ButtonDanger
)

// dangerInk is the red used for a ButtonDanger's border + label.
var dangerInk = RGB(0xD0, 0x30, 0x30)

// buttonRadius is the corner radius for the button + text-entry body, giving
// the WhiteSur / macOS soft-rectangle look.
const buttonRadius = 6

// accentFg returns the ink to draw on an Accent fill: the theme's
// accent_fg_color when present (GTK/WhiteSur themes set it to white), else a
// safe white fallback.
func accentFg(theme *Theme) RGBA {
	if c, ok := theme.Extra["accent_fg_color"]; ok {
		return c
	}
	return RGB(0xFF, 0xFF, 0xFF)
}

// NewButton constructs a Button with the given label + click handler.
// Handler may be nil (a no-op button is still rendered).
func NewButton(label string, onClick func()) *Button {
	b := &Button{OnClick: onClick, PressFeedback: true}
	b.label = mvvm.NewObservable(label)
	return b
}

// Label is the button caption as a shared [mvvm.Observable]: a host binds it (or
// subscribes) instead of touching a field. Lazily created so a bare &Button{}
// works. When Icon is set it is drawn instead of the caption.
func (b *Button) Label() *mvvm.Observable[string] {
	if b.label == nil {
		b.label = mvvm.NewObservable("")
	}
	return b.label
}

// Selected is the sticky app-managed "active" state as a shared [mvvm.Observable]:
// the host drives it from its own model (true fills the button with Accent
// regardless of Style); the button never flips it itself. Lazily created.
func (b *Button) Selected() *mvvm.Observable[bool] {
	if b.selected == nil {
		b.selected = mvvm.NewObservable(false)
	}
	return b.selected
}

// LeadingIcon is an OPTIONAL vector glyph drawn just before the caption, as a
// shared [mvvm.Observable]: set a non-nil IconFunc to show it, nil to hide it.
// Lazily created; a button that never sets it draws exactly as before.
func (b *Button) LeadingIcon() *mvvm.Observable[IconFunc] {
	if b.leadingIcon == nil {
		// IconFunc is not comparable, so an equality function is required; funcs
		// cannot be compared beyond nil, so every Set is treated as a change.
		b.leadingIcon = mvvm.NewObservableEq[IconFunc](nil, func(a, b IconFunc) bool { return false })
	}
	return b.leadingIcon
}

// Shortcut is an OPTIONAL keyboard-shortcut hint (e.g. "⌘F") drawn right-aligned
// in a muted tone, as a shared [mvvm.Observable]. Empty hides it. Lazily created.
func (b *Button) Shortcut() *mvvm.Observable[string] {
	if b.shortcut == nil {
		b.shortcut = mvvm.NewObservable("")
	}
	return b.shortcut
}

// leadingIconFn / shortcutText read the optional adornments without forcing the
// Observable to exist (an unset button keeps the byte-identical legacy draw).
func (b *Button) leadingIconFn() IconFunc {
	if b.leadingIcon == nil {
		return nil
	}
	return b.leadingIcon.Get()
}

func (b *Button) shortcutText() string {
	if b.shortcut == nil {
		return ""
	}
	return b.shortcut.Get()
}

// SetHovered/SetPressed are wired by the parent container's mouse
// dispatcher so the button can render its hover/press visual states.
// Direct setters (vs deducing from OnEvent kinds) keep the parent
// in control of state propagation -- enter/leave events would
// duplicate the same logic in every leaf widget.
func (b *Button) SetHovered(v bool) { b.hovered = v }
func (b *Button) SetPressed(v bool) { b.pressed = v }

// Draw paints the button through p using theme's palette. Face
// cycles through Surface / SurfaceAlt (hovered) / Accent (pressed);
// the Label is centred in the body using the toolkit's 5x7 bitmap
// font. When the button is pressed the ink swaps to the theme's
// Background so the label stays legible against the Accent face.
func (b *Button) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	// Resting face/ink/border from the style.
	face := theme.Surface
	ink := theme.OnSurface
	border := theme.Border
	switch b.Style {
	case ButtonProminent:
		face, ink = theme.Accent, accentFg(theme)
	case ButtonSecondary:
		face = theme.SurfaceAlt
	case ButtonDanger:
		ink, border = dangerInk, dangerInk
	}
	// A sticky selection fills with Accent regardless of style.
	if b.Selected().Get() {
		face, ink = theme.Accent, accentFg(theme)
	}
	// Interaction states override the resting fill.
	switch {
	case b.pressed:
		face = theme.Accent
		ink = theme.Background
	case b.hovered:
		face = theme.SurfaceAlt
	}
	// A disabled button ignores style + interaction and paints a muted face,
	// so it reads as inert. Only taken when Disabled, so the enabled draw is
	// byte-identical.
	if b.Disabled().Get() {
		face, ink, border = mutedFace(theme), mutedInk(theme), mutedInk(theme)
	}
	if b.Flat {
		// Square face only; the enclosing container (e.g. ButtonGroup) owns the
		// shared border/rounding.
		fillRect(p, r.X, r.Y, r.W, r.H, face)
	} else {
		// buttonRadius routes through scaled so the corner grows with HiDPI and
		// touch density; at compact/1x scaled(buttonRadius) == buttonRadius, so the
		// drawn corner is byte-identical to before.
		rad := scaled(buttonRadius)
		fillRoundRect(p, r.X, r.Y, r.W, r.H, rad, face)
		strokeRoundRect(p, r.X, r.Y, r.W, r.H, rad, border)
	}
	// A host-supplied Icon replaces the text Label (like SearchEntry.Icon): it is
	// handed the full button rect and the current ink so the glyph tracks the
	// button's pressed / disabled tint. Otherwise the caption is drawn, with its
	// optional leading icon and trailing shortcut.
	if b.Icon != nil {
		b.Icon(p, r, ink)
	} else {
		b.drawContent(p, theme, r, ink)
	}
	b.drawFocusRing(p, theme, r)
}

// drawContent paints the caption plus its optional leading icon and trailing
// shortcut hint. With neither adornment set the caption is centred exactly as
// before (byte-identical); with either set the row is laid out left-to-right —
// [icon] label … [shortcut] — inside the button's padding.
func (b *Button) drawContent(p painter.Painter, theme *Theme, r Rect, ink RGBA) {
	label := b.Label().Get()
	icon := b.leadingIconFn()
	sc := b.shortcutText()
	gh := b.glyphHeight()
	ty := r.Y + (r.H-gh)/2

	if icon == nil && sc == "" {
		if label != "" {
			tw := b.textWidth(label)
			b.drawText(p, r.X+(r.W-tw)/2, ty, label, ink)
		}
		return
	}

	pad := scaled(8)
	x := r.X + pad
	if icon != nil {
		icon(p, Rect{X: x, Y: r.Y + (r.H-gh)/2, W: gh, H: gh}, ink)
		x += gh + scaled(6)
	}
	if label != "" {
		b.drawText(p, x, ty, label, ink)
	}
	if sc != "" {
		sw := b.textWidth(sc)
		b.drawText(p, r.X+r.W-pad-sw, ty, sc, mutedInk(theme))
	}
}

// PreferredWidth is the intrinsic width the button needs to show its CURRENT
// content without clipping — the caption plus its optional leading icon and
// trailing shortcut, measured with the same paddings Draw uses. A host that sizes
// its buttons to this (instead of a hard-coded width) never clips a longer label,
// a wider toggled caption, an icon or a shortcut. Re-measure after a label change.
func (b *Button) PreferredWidth() int {
	pad := scaled(8)
	// An Icon that REPLACES the caption is square (a glyph in the button's height).
	if b.Icon != nil {
		return b.glyphHeight() + 2*pad
	}
	w := 2 * pad
	if b.leadingIconFn() != nil {
		w += b.glyphHeight() + scaled(6)
	}
	if label := b.Label().Get(); label != "" {
		w += b.textWidth(label)
	}
	if sc := b.shortcutText(); sc != "" {
		w += scaled(12) + b.textWidth(sc) // the gap before the right-aligned hint
	}
	return w
}

// OnEvent drives the button from pointer events: EventClick presses it (shows
// the pressed face + fires OnClick) and EventMouseUp releases it. Self-managing
// the pressed state means any host that routes the press/release pair gets the
// click feedback for free, without also wiring SetPressed. Other event kinds
// are ignored. (SetPressed remains for hosts that drive press state their own
// way, e.g. enter/leave dispatch.)
func (b *Button) OnEvent(ev Event) {
	if b.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		if b.PressFeedback {
			b.pressed = true
		}
		b.activate()
	case EventKeyDown:
		// Enter or Space fires the primary action while focused, reusing the
		// exact click path (OnClick) so keyboard + mouse stay consistent. No
		// pressed-face flash: there is no routed key-up to clear it.
		switch ev.Code {
		case "Enter", " ", "Space":
			b.activate()
		}
	case EventMouseUp:
		b.pressed = false
	case EventMouseMove:
		// Raise the hover face when the pointer is over the button, clear it
		// when a container forwards a move whose point has left it.
		b.hovered = b.localInBounds(ev.X, ev.Y)
	}
}

// activate fires OnClick (nil-safe) -- the single mutate+callback path shared
// by an EventClick and an Enter/Space key press so both routes behave
// identically.
func (b *Button) activate() {
	if b.OnClick != nil {
		b.OnClick()
	}
}

// HitRect is the button's interactive rectangle: its drawn Bounds clamped up to
// the density hit-target and centred over them. Byte-identical to Bounds under
// DensityCompact; a small button exposes a >=44px finger target under
// DensityTouch without changing what it draws.
func (b *Button) HitRect() Rect { return touchHitRect(b.Bounds()) }

// HitTest reports whether a surface point falls on the button's (touch-clamped)
// hit rect. At compact the clamp is a pass-through so this equals the default
// Bounds().Contains; at touch the reachable area grows to the finger floor.
func (b *Button) HitTest(px, py int) bool { return b.HitRect().Contains(px, py) }
