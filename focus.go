// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Focusable is a widget that can hold keyboard focus within a FocusRing.
//
// This is intentionally a minimal surface — narrower than the terminal
// sibling's tui.Focusable, which embeds toolkit.Widget (Bounds/SetBounds/
// Draw/HitTest/OnEvent) because tui's FocusRing also lays out, draws, and
// routes events to its members on their behalf. This FocusRing does none of
// that: it only tracks which member holds focus and toggles SetFocused as
// focus moves. A concrete toolkit widget (Button, Entry, ...) can satisfy
// Focusable with nothing more than a SetFocused method; its own Draw/OnEvent
// stay untouched, and wiring a widget's rendering/dispatch into a ring is a
// later task.
type Focusable interface {
	// SetFocused is called by the ring when focus enters (true) or leaves
	// (false) this item.
	SetFocused(focused bool)
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
