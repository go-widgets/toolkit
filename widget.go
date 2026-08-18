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
	// EventMouseDrag fires when the mouse moves while a button is
	// still pressed. X/Y carry the current widget-local position.
	// The initial button press was already dispatched as EventClick,
	// so a widget that wants drag semantics remembers "am I being
	// dragged" from the EventClick and consults it on drag ticks.
	EventMouseDrag
	// EventMouseUp fires when the button is released. Widgets that
	// track drag state clear it here. X/Y carry the release
	// position (widget-local).
	EventMouseUp
	// EventDragStart fires on a DropTarget when a drag first enters it
	// (drag-enter). The target typically raises a hover cue. Code
	// carries the drag payload the host is offering, so the target can
	// decide via AcceptsDrop whether to signal acceptance.
	EventDragStart
	// EventDragMove fires as the drag pointer moves while still over the
	// same DropTarget. X/Y carry the widget-local position (for an
	// insertion indicator); Code still carries the payload.
	EventDragMove
	// EventDragLeave fires when the drag pointer exits a DropTarget
	// without dropping. The target clears its hover cue. It completes
	// the enter/move/leave/drop lifecycle so a target never stays stuck
	// in the hover state.
	EventDragLeave
	// EventDrop fires when the drag is released over a DropTarget. Code
	// carries the payload (multiple items newline-separated — see
	// SplitDropPayload); the target consumes it and clears its hover cue.
	EventDrop

	// EventTouchStart fires when a touch point first lands inside the
	// widget. X/Y carry the widget-local touch position. Code carries
	// the touch/pointer id (a host-assigned string, stable for the
	// lifetime of that contact) so a widget — or a GestureRecognizer —
	// can distinguish concurrent contacts in a multi-touch stream even
	// though the toolkit's own GestureRecognizer only tracks one active
	// pointer at a time.
	EventTouchStart
	// EventTouchMove fires as an already-started touch point moves.
	// X/Y carry the current widget-local position; Code carries the
	// same touch/pointer id as the EventTouchStart that began it.
	EventTouchMove
	// EventTouchEnd fires when a touch point is lifted. X/Y carry the
	// widget-local release position; Code carries the same touch/pointer
	// id as the EventTouchStart that began it, so a listener can match
	// the end to its start even if other contacts are interleaved.
	EventTouchEnd

	// EventScroll fires when the user scrolls the wheel (or a trackpad
	// two-finger swipe) over the widget — the toolkit's native scroll
	// intent. Delta carries the scroll amount in ROWS: positive scrolls
	// down / forward (toward the end of the content), negative scrolls
	// up / back. X/Y carry the widget-local pointer position at the time
	// of the scroll, so a container can hit-test which child the wheel is
	// over. Scrollable widgets (ListBox, Table, TreeTable, TreeView,
	// ScrollView) handle it by calling their own ScrollBy(Delta), which
	// clamps at both ends; every other widget ignores it. Hosts translate
	// the browser's wheel event (or a native scroll gesture) into this
	// kind so no app has to hand-roll wheel routing.
	EventScroll

	// EventMouseMove fires when the pointer moves over the widget with NO
	// button pressed — the plain hover-tracking move (its pressed-button
	// counterpart is EventMouseDrag). X/Y carry the widget-local pointer
	// position. Containers forward it to their children (translating
	// coordinates like every other kind) so a leaf sets its hover face when
	// the pointer is over it and clears it when the pointer moves off; a
	// widget that draws no hover state simply ignores it. Purely a
	// visual-feedback signal — it never activates anything. Appended last
	// (rather than beside EventMouseDrag) so the pre-existing kinds keep
	// their integer values for any host that persists them.
	EventMouseMove

	// EventSecondaryClick fires on a secondary (right / two-finger / long-press)
	// button press inside the widget — the gesture a desktop user expects to open
	// a context menu. It carries the same widget-local X/Y and modifier fields as
	// EventClick; it is a press, with no paired release event, because opening a
	// menu needs only the down. A host that has no secondary button (or chooses
	// not to map one) simply never sends it, and a widget that does not handle it
	// is unaffected.
	EventSecondaryClick

	// eventKindEnd is the exclusive upper bound of valid EventKind
	// values — not itself a valid kind. Its sole purpose is letting
	// TestEventKindValuesAreDistinct assert exhaustive coverage: any
	// new EventKind appended above bumps this by one, and a stale
	// test list will fail loudly instead of silently.
	eventKindEnd
)

// Event is one input event delivered to a widget. The parent container
// translated mouse coordinates into widget-local pixels; Code is the
// key/char text for keyboard events.
//
// Ctrl and Shift report whether those modifier keys were held when the event
// was produced. They are meaningful on any event kind but most useful on
// EventClick, where they drive multi-selection (Ctrl-click toggles a row,
// Shift-click extends a range from the anchor). Hosts that don't track
// modifiers simply leave them false, which preserves the original
// single-selection behaviour everywhere.
//
// Alt and Meta report the two remaining desktop modifiers, so a host can
// deliver a platform-native accelerator a widget could not otherwise tell
// apart from a plain Ctrl chord. Alt is the ⌥ Option key on macOS and the Alt
// key on X11/Wayland/Windows; Meta is the ⌘ Command key on macOS and the
// Super/Windows/logo key elsewhere. They let a file manager distinguish, for
// example, ⌘V (paste) from ⌘⌥V (paste-as-move) — a distinction Ctrl/Shift
// alone cannot express. Like Ctrl/Shift they default false, so a host that
// does not track them (or a widget that ignores them) behaves exactly as
// before; only code that opts in by reading them sees any change.
type Event struct {
	Kind        EventKind
	X, Y        int
	Code        string
	Ctrl, Shift bool
	// Alt is the Option (⌥) / Alt modifier; Meta is the Command (⌘) / Super
	// (Windows/logo) modifier. Both default false. See the type doc above.
	Alt, Meta bool
	// Delta is the VERTICAL scroll amount, in ROWS, for an EventScroll:
	// positive scrolls down / forward (toward the end of the content),
	// negative scrolls up / back. It is zero and ignored on every other event
	// kind. Scrollable widgets pass it straight to ScrollBy, which clamps at
	// both ends, so an over-large Delta simply pins to the last (or first)
	// row instead of running off.
	Delta int
	// DeltaX is the HORIZONTAL scroll amount, in ROWS, for an EventScroll:
	// positive scrolls right (toward the end of the content), negative
	// scrolls left. It is zero and ignored on every other event kind, and is
	// zero for a purely vertical wheel/notch — a two-finger horizontal swipe
	// (the browser's wheel deltaX) is what fills it, so a ScrollView moves its
	// horizontal scrollbar in response instead of ignoring the gesture.
	DeltaX int
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
//
// Font is an optional per-widget font override. When nil (the zero value,
// the default for every widget) the widget lays out + renders against the
// package-level active font (see SetFont / CurrentFont), so it behaves
// exactly as if per-widget fonts did not exist. When set — e.g.
//
//	badge.Font, _ = NewTrueTypeFont(myFontTTF, 10)   // small tag
//	title.Font, _ = NewTrueTypeFont(myFontTTF, 22)   // large heading
//
// that single widget measures + paints its text with that font while every
// other widget keeps using the global one. Widgets consult it through the
// font-aware helpers (EffectiveFont / textWidth / drawText / glyphHeight /
// glyphAdvance) instead of the package-level TextWidth / DrawText /
// GlyphHeight, so a font swap is scoped to the widget that sets it.
type Base struct {
	rect Rect
	// Font, when non-nil, overrides the global active font for this widget
	// only. nil means "inherit the active font" (the default).
	Font Font
	// Disabled, when true, makes an interactive widget inert: its OnEvent
	// early-returns (no click / drag / scroll / hover / key effect) and its
	// Draw paints a muted, greyed face. The zero value (false) is the normal
	// interactive state, so every widget is enabled by default and existing
	// renders are unchanged. Inherited by every widget that embeds Base, so a
	// caller disables any control with `w.Disabled = true`.
	Disabled bool
}

func (b *Base) Bounds() Rect            { return b.rect }
func (b *Base) SetBounds(r Rect)        { b.rect = r }
func (b *Base) HitTest(px, py int) bool { return b.rect.Contains(px, py) }
func (b *Base) OnEvent(ev Event)        { _ = ev /* no-op default; widgets override */ }
func (b *Base) Draw(p painter.Painter, theme *Theme) {
	// no-op default; concrete widgets override Draw.
	_, _ = p, theme
}

// SetFont sets this widget's per-widget font override and returns the Base so
// the call can be chained fluently (b := (&Badge{}).SetFont(f) style). A nil f
// clears the override, restoring inheritance of the global active font.
func (b *Base) SetFont(f Font) *Base {
	b.Font = f
	return b
}

// EffectiveFont is the font this widget renders with: its own Font override if
// one is set, otherwise the package-level active font (CurrentFont). It never
// returns nil, so callers can measure/draw through it unconditionally.
func (b *Base) EffectiveFont() Font {
	if b.Font != nil {
		return b.Font
	}
	return CurrentFont()
}

// textWidth is the pixel width this widget's text occupies, measured in the
// widget's effective font — the per-widget counterpart of the package-level
// TextWidth.
func (b *Base) textWidth(s string) int { return b.EffectiveFont().Measure(s) }

// drawText paints s at (x, y) in this widget's effective font — the per-widget
// counterpart of the package-level DrawText.
func (b *Base) drawText(p painter.Painter, x, y int, s string, ink RGBA) {
	b.EffectiveFont().Draw(p, x, y, s, ink)
}

// glyphHeight is this widget's effective font's glyph box height — the
// per-widget counterpart of the package-level GlyphHeight.
func (b *Base) glyphHeight() int { return b.EffectiveFont().Height() }

// glyphAdvance is this widget's effective font's horizontal step per glyph —
// the per-widget counterpart of the package-level GlyphAdvance.
func (b *Base) glyphAdvance() int { return b.EffectiveFont().Advance() }

// localInBounds reports whether a widget-local point (as delivered in an
// OnEvent, whose X/Y are already offset into this widget's frame) falls within
// this widget's own rectangle — [0,W) × [0,H). Hover-tracking widgets call it on
// EventMouseMove to raise their hover face when the pointer is over them (true)
// and clear it when a container forwards a move whose point has left them
// (false), so hover-enter and hover-leave both work with no host wiring.
func (b *Base) localInBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < b.rect.W && y < b.rect.H
}

// mutedInk is the ink a Disabled widget paints its text / glyphs in: the theme's
// OnSurface blended halfway to its Background, so a greyed control reads as
// inactive under both light and dark themes. Shared so disabled widgets tint
// consistently.
func mutedInk(theme *Theme) RGBA { return blendRGBA(theme.OnSurface, theme.Background, 0.5) }

// mutedFace is the fill a Disabled widget paints its body in: the theme's
// SurfaceAlt blended halfway to its Background — a flat, greyed panel shared
// across widgets so every disabled control has the same muted face.
func mutedFace(theme *Theme) RGBA { return blendRGBA(theme.SurfaceAlt, theme.Background, 0.5) }
