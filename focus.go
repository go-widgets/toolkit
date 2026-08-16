// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Focusable is a widget that can hold keyboard focus. A widget satisfies it by
// embedding focusState (SetFocused/Focused come for free) — that embedding is
// what makes a widget focusable: display-only widgets omit it and are therefore
// never focused, drawn with a focus ring, nor visited by container Tab
// traversal.
//
// The surface is deliberately minimal — narrower than the terminal sibling's
// tui.Focusable, which embeds the whole Widget interface because tui's
// FocusRing also lays out, draws, and routes events to its members. A toolkit
// widget instead draws its own focus ring (see focusState.drawFocusRing) and a
// focus-managing Container/HBox/VBox/Grid/Frame routes keys and clicks to it,
// so Focusable only has to expose and toggle the focused flag.
type Focusable interface {
	// SetFocused is called when focus enters (true) or leaves (false) this
	// item.
	SetFocused(focused bool)
	// Focused reports whether this item currently holds keyboard focus.
	Focused() bool
}

// focusState is the shared keyboard-focus state every interactive widget embeds
// to become Focusable. It stores the focused flag, supplies the SetFocused /
// Focused methods, and draws the focus ring — so a widget opts into the focus
// system with a single embedded field and one drawFocusRing call at the end of
// its Draw, with no per-widget focus bookkeeping. Non-interactive/display
// widgets simply do not embed it and stay non-focusable.
type focusState struct {
	focused bool
	// FocusRingRadius, when > 0, draws the focus ring as a rounded rectangle of
	// that corner radius (in device units) instead of the default 1-pixel square
	// inset — for a widget whose body is a rounded field (a pill-shaped search
	// box). Zero keeps the classic square ring, so every widget that does not set
	// it renders byte-for-byte as before.
	FocusRingRadius int
	// FocusRingWidth is the rounded ring's stroke thickness; it applies only when
	// FocusRingRadius > 0, and a value < 1 is treated as 1. Ignored for the square
	// ring.
	FocusRingWidth int
}

// SetFocused records whether this widget holds keyboard focus.
func (f *focusState) SetFocused(focused bool) { f.focused = focused }

// Focused reports whether this widget currently holds keyboard focus.
func (f *focusState) Focused() bool { return f.focused }

// drawFocusRing paints a 1-pixel focus ring, inset one pixel from r, in the
// theme's Accent colour when this widget holds focus — and paints nothing when
// it does not, so an unfocused widget renders byte-for-byte as it did before
// the focus system existed. Interactive widgets call it at the very end of
// their Draw with their own Bounds so the ring sits on top of their content.
func (f *focusState) drawFocusRing(p painter.Painter, theme *Theme, r Rect) {
	if !f.focused {
		return
	}
	if f.FocusRingRadius > 0 {
		w := f.FocusRingWidth
		if w < 1 {
			w = 1
		}
		p.StrokeRoundRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, f.FocusRingRadius, theme.Accent, w)
		return
	}
	strokeRect(p, r.X+1, r.Y+1, r.W-2, r.H-2, theme.Accent)
}

// focusEnumerator is implemented by every focus-managing container (Container,
// HBox, VBox, Grid, Frame): it yields the container's direct child widgets in
// visual order so the focus walker can descend the widget tree.
type focusEnumerator interface {
	focusableChildren() []Widget
}

// disabledReporter exposes a widget's Wave-1 Disabled flag through an interface
// (a struct field cannot be type-asserted). Every widget satisfies it via Base,
// letting the focus walker skip Disabled widgets.
type disabledReporter interface {
	isDisabled() bool
}

// isDisabled reports Base.Disabled so the focus walker can skip disabled
// widgets when enumerating focus candidates.
func (b *Base) isDisabled() bool { return b.Disabled }

// gatherFocusables appends every enabled, visible Focusable descendant of ce to
// out, in visual (child) order, descending into nested containers so a focusable
// nested any number of containers deep is reached. Hidden cells (empty Bounds,
// e.g. an inactive CardLayout page or a collapsed box) and Disabled widgets are
// skipped.
func gatherFocusables(ce focusEnumerator, out *[]Widget) {
	for _, child := range ce.focusableChildren() {
		if child == nil {
			continue
		}
		b := child.Bounds()
		if b.W <= 0 || b.H <= 0 {
			continue
		}
		if _, ok := child.(Focusable); ok {
			if d, ok := child.(disabledReporter); ok && d.isDisabled() {
				continue
			}
			*out = append(*out, child)
			continue
		}
		if sub, ok := child.(focusEnumerator); ok {
			gatherFocusables(sub, out)
		}
	}
}

// focusListOf returns ce's flat, ordered focusable descendants.
func focusListOf(ce focusEnumerator) []Widget {
	var out []Widget
	gatherFocusables(ce, &out)
	return out
}

// setSingleFocus focuses target and defocuses every other focusable in list,
// enforcing the single-focus-per-tree invariant.
func setSingleFocus(list []Widget, target Widget) {
	for _, w := range list {
		if f, ok := w.(Focusable); ok {
			f.SetFocused(w == target)
		}
	}
}

// focusedInList returns the currently-focused widget in list, or nil when none
// is focused.
func focusedInList(list []Widget) Widget {
	for _, w := range list {
		if f, ok := w.(Focusable); ok && f.Focused() {
			return w
		}
	}
	return nil
}

// moveFocus advances focus to the next (forward) or previous focusable in list,
// wrapping at both ends. With nothing focused yet it takes the first (forward)
// or last (backward) member.
func moveFocus(list []Widget, forward bool) {
	n := len(list)
	if n == 0 {
		return
	}
	cur := -1
	for i, w := range list {
		if f, ok := w.(Focusable); ok && f.Focused() {
			cur = i
			break
		}
	}
	var next int
	switch {
	case cur < 0:
		if forward {
			next = 0
		} else {
			next = n - 1
		}
	case forward:
		next = (cur + 1) % n
	default:
		next = (cur - 1 + n) % n
	}
	setSingleFocus(list, list[next])
}

// routeFocusKey gives a focus-managing container its keyboard behaviour: Tab
// (or Shift+Tab / a shifted Tab) moves focus through the container's focusable
// descendants with wrapping, and any other EventKeyDown or EventChar is
// delivered straight to the currently-focused descendant so its own key handler
// runs. It reports whether ev was a keyboard event it took charge of; keyboard
// events never fall through to the container's positional routing.
func routeFocusKey(ce focusEnumerator, ev Event) bool {
	switch ev.Kind {
	case EventKeyDown:
		list := focusListOf(ce)
		switch {
		case ev.Code == "Tab":
			moveFocus(list, !ev.Shift)
		case ev.Code == "Shift+Tab":
			moveFocus(list, false)
		default:
			if w := focusedInList(list); w != nil {
				w.OnEvent(ev)
			}
		}
		return true
	case EventChar:
		if w := focusedInList(focusListOf(ce)); w != nil {
			w.OnEvent(ev)
		}
		return true
	}
	return false
}

// focusClick moves focus to the focusable descendant containing surface point
// (sx, sy) when a click lands on it, defocusing the previously-focused widget. A
// click that hits no focusable leaves focus unchanged. Idempotent, so a nested
// container re-running it after its parent is harmless.
func focusClick(ce focusEnumerator, sx, sy int) {
	list := focusListOf(ce)
	for _, w := range list {
		if w.Bounds().Contains(sx, sy) {
			setSingleFocus(list, w)
			return
		}
	}
}

// FocusRing gives a set of Focusables a single, shared keyboard focus: Next
// and Prev move it forward/backward through the members, wrapping at both
// ends; Focus jumps straight to a member (e.g. in response to a click hit-
// test performed by the caller); HandleKey maps the ring's Tab/Shift+Tab key
// convention onto Next/Prev for callers that dispatch toolkit.Event directly.
//
// A FocusRing does not implement toolkit.Widget: it neither draws nor
// receives events on its own, it only supervises which member is focused.
// The zero value is not usable; construct one with NewFocusRing.
type FocusRing struct {
	items   []Focusable
	current int // index of the focused item; -1 when the ring is empty.
}

// NewFocusRing builds a ring over items, focusing the first one (if any).
func NewFocusRing(items ...Focusable) *FocusRing {
	r := &FocusRing{current: -1}
	for _, it := range items {
		r.Add(it)
	}
	return r
}

// Current returns the index of the focused item, or -1 when the ring is
// empty.
func (r *FocusRing) Current() int {
	return r.current
}

// Focused returns the focused item, or nil when the ring is empty.
func (r *FocusRing) Focused() Focusable {
	if r.current < 0 || r.current >= len(r.items) {
		return nil
	}
	return r.items[r.current]
}

// Focus moves focus to item i. Out-of-range i (including any index on an
// empty ring) is ignored. SetFocused(false) is called on the previously
// focused item and SetFocused(true) on item i, matching tui.FocusRing.Focus.
func (r *FocusRing) Focus(i int) {
	if i < 0 || i >= len(r.items) {
		return
	}
	if old := r.Focused(); old != nil {
		old.SetFocused(false)
	}
	r.current = i
	r.items[i].SetFocused(true)
}

// Next moves focus to the following item, wrapping from the last item to the
// first. No-op on an empty ring.
func (r *FocusRing) Next() {
	if n := len(r.items); n > 0 {
		r.Focus((r.current + 1) % n)
	}
}

// Prev moves focus to the preceding item, wrapping from the first item to
// the last. No-op on an empty ring.
func (r *FocusRing) Prev() {
	if n := len(r.items); n > 0 {
		r.Focus((r.current - 1 + n) % n)
	}
}

// Add appends f to the ring. If the ring was empty, f is focused
// immediately (mirroring NewFocusRing's treatment of the first item).
func (r *FocusRing) Add(f Focusable) {
	r.items = append(r.items, f)
	if r.current < 0 {
		r.current = len(r.items) - 1
		f.SetFocused(true)
	}
}

// Clear removes every item from the ring, first defocusing whichever item
// currently holds focus, and resets the ring to its empty state.
func (r *FocusRing) Clear() {
	if cur := r.Focused(); cur != nil {
		cur.SetFocused(false)
	}
	r.items = nil
	r.current = -1
}

// HandleKey maps a toolkit.EventKeyDown Code — "Tab" or "Shift+Tab" — onto
// Next/Prev, matching the key convention tui.FocusRing.OnEvent dispatches
// on. It reports whether the key was consumed so a caller forwards
// everything else (e.g. Enter, arrow keys) to the focused member itself.
func (r *FocusRing) HandleKey(code string) bool {
	switch code {
	case "Tab":
		r.Next()
		return true
	case "Shift+Tab":
		r.Prev()
		return true
	default:
		return false
	}
}
