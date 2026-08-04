// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// fakeFocusable is a minimal Focusable test double that records every
// SetFocused call so tests can assert exactly when focus toggled.
type fakeFocusable struct {
	name     string
	focused  bool
	setCalls []bool
}

func (f *fakeFocusable) SetFocused(focused bool) {
	f.focused = focused
	f.setCalls = append(f.setCalls, focused)
}

func (f *fakeFocusable) Focused() bool { return f.focused }

func newFakes(names ...string) []*fakeFocusable {
	fakes := make([]*fakeFocusable, len(names))
	for i, n := range names {
		fakes[i] = &fakeFocusable{name: n}
	}
	return fakes
}

func asFocusables(fakes []*fakeFocusable) []Focusable {
	items := make([]Focusable, len(fakes))
	for i, f := range fakes {
		items[i] = f
	}
	return items
}

func TestNewFocusRing_EmptyRing(t *testing.T) {
	r := NewFocusRing()
	if got := r.Current(); got != -1 {
		t.Fatalf("Current() = %d, want -1", got)
	}
	if got := r.Focused(); got != nil {
		t.Fatalf("Focused() = %v, want nil", got)
	}
}

func TestNewFocusRing_FocusesFirstItem(t *testing.T) {
	fakes := newFakes("a", "b", "c")
	r := NewFocusRing(asFocusables(fakes)...)

	if got := r.Current(); got != 0 {
		t.Fatalf("Current() = %d, want 0", got)
	}
	if !fakes[0].focused {
		t.Fatalf("item 0 should be focused")
	}
	if fakes[1].focused || fakes[2].focused {
		t.Fatalf("only item 0 should be focused")
	}
	if got := r.Focused(); got != Focusable(fakes[0]) {
		t.Fatalf("Focused() = %v, want fakes[0]", got)
	}
}

func TestFocusRing_NextWraps(t *testing.T) {
	fakes := newFakes("a", "b", "c")
	r := NewFocusRing(asFocusables(fakes)...)

	r.Next()
	if got := r.Current(); got != 1 {
		t.Fatalf("after Next(): Current() = %d, want 1", got)
	}
	if fakes[0].focused {
		t.Fatalf("item 0 should have been defocused")
	}
	if !fakes[1].focused {
		t.Fatalf("item 1 should be focused")
	}

	r.Next()
	if got := r.Current(); got != 2 {
		t.Fatalf("after second Next(): Current() = %d, want 2", got)
	}

	r.Next() // wrap back to 0
	if got := r.Current(); got != 0 {
		t.Fatalf("after wrapping Next(): Current() = %d, want 0", got)
	}
	if !fakes[0].focused {
		t.Fatalf("item 0 should be focused again after wrap")
	}
	if fakes[2].focused {
		t.Fatalf("item 2 should have been defocused after wrap")
	}
}

func TestFocusRing_PrevWraps(t *testing.T) {
	fakes := newFakes("a", "b", "c")
	r := NewFocusRing(asFocusables(fakes)...)

	r.Prev() // wrap back to last item
	if got := r.Current(); got != 2 {
		t.Fatalf("after Prev() from 0: Current() = %d, want 2", got)
	}
	if fakes[0].focused {
		t.Fatalf("item 0 should have been defocused")
	}
	if !fakes[2].focused {
		t.Fatalf("item 2 should be focused")
	}

	r.Prev()
	if got := r.Current(); got != 1 {
		t.Fatalf("after second Prev(): Current() = %d, want 1", got)
	}
}

func TestFocusRing_NextPrevNoOpOnEmptyRing(t *testing.T) {
	r := NewFocusRing()
	r.Next()
	r.Prev()
	if got := r.Current(); got != -1 {
		t.Fatalf("Current() = %d, want -1 (still empty)", got)
	}
}

func TestFocusRing_FocusOutOfRangeIsIgnored(t *testing.T) {
	fakes := newFakes("a", "b")
	r := NewFocusRing(asFocusables(fakes)...)

	r.Focus(-1)
	if got := r.Current(); got != 0 {
		t.Fatalf("Focus(-1): Current() = %d, want unchanged 0", got)
	}
	r.Focus(5)
	if got := r.Current(); got != 0 {
		t.Fatalf("Focus(5): Current() = %d, want unchanged 0", got)
	}
	if !fakes[0].focused {
		t.Fatalf("item 0 should still be focused after out-of-range Focus calls")
	}

	// Out-of-range Focus on an empty ring must also be a no-op (exercises
	// the i >= len(r.items) branch when len is 0).
	empty := NewFocusRing()
	empty.Focus(0)
	if got := empty.Current(); got != -1 {
		t.Fatalf("Focus(0) on empty ring: Current() = %d, want -1", got)
	}
}

func TestFocusRing_FocusDirectClick(t *testing.T) {
	fakes := newFakes("a", "b", "c")
	r := NewFocusRing(asFocusables(fakes)...)

	// Simulate "click-to-focus": the caller hit-tests its own widgets and
	// calls Focus(i) directly for whichever one was hit.
	r.Focus(2)
	if got := r.Current(); got != 2 {
		t.Fatalf("Focus(2): Current() = %d, want 2", got)
	}
	if fakes[0].focused {
		t.Fatalf("item 0 should have been defocused")
	}
	if !fakes[2].focused {
		t.Fatalf("item 2 should be focused")
	}

	// Re-focusing the already-focused item still toggles SetFocused(false)
	// then SetFocused(true), mirroring tui.FocusRing.Focus.
	r.Focus(2)
	calls := fakes[2].setCalls
	if len(calls) < 2 || calls[len(calls)-2] != false || calls[len(calls)-1] != true {
		t.Fatalf("Focus on already-focused item: setCalls tail = %v, want [..., false, true]", calls)
	}
}

func TestFocusRing_AddFocusesFirstItemAddedToEmptyRing(t *testing.T) {
	r := NewFocusRing()
	fakes := newFakes("a")
	r.Add(fakes[0])

	if got := r.Current(); got != 0 {
		t.Fatalf("Current() = %d, want 0", got)
	}
	if !fakes[0].focused {
		t.Fatalf("first item added to an empty ring should be focused")
	}
}

func TestFocusRing_AddToNonEmptyRingDoesNotStealFocus(t *testing.T) {
	fakes := newFakes("a", "b")
	r := NewFocusRing(asFocusables(fakes)[:1]...)

	r.Add(fakes[1])
	if got := r.Current(); got != 0 {
		t.Fatalf("Current() = %d, want unchanged 0", got)
	}
	if fakes[1].focused {
		t.Fatalf("newly added item should not steal focus from the current one")
	}

	// The newly added item is now reachable via Next.
	r.Next()
	if got := r.Current(); got != 1 {
		t.Fatalf("Current() = %d, want 1", got)
	}
	if !fakes[1].focused {
		t.Fatalf("item 1 should be focused after Next()")
	}
}

func TestFocusRing_Clear(t *testing.T) {
	fakes := newFakes("a", "b")
	r := NewFocusRing(asFocusables(fakes)...)

	r.Clear()
	if got := r.Current(); got != -1 {
		t.Fatalf("Current() = %d, want -1 after Clear", got)
	}
	if got := r.Focused(); got != nil {
		t.Fatalf("Focused() = %v, want nil after Clear", got)
	}
	if fakes[0].focused {
		t.Fatalf("item 0 should have been defocused by Clear")
	}

	// Clearing an already-empty ring is a no-op, not a panic (exercises the
	// Focused() == nil branch inside Clear).
	r.Clear()
	if got := r.Current(); got != -1 {
		t.Fatalf("Current() = %d, want -1 after clearing an empty ring", got)
	}

	// The ring is reusable after Clear.
	r.Add(fakes[0])
	if got := r.Current(); got != 0 {
		t.Fatalf("Current() = %d, want 0 after Add following Clear", got)
	}
}

func TestFocusRing_HandleKey(t *testing.T) {
	fakes := newFakes("a", "b")
	r := NewFocusRing(asFocusables(fakes)...)

	if !r.HandleKey("Tab") {
		t.Fatalf("HandleKey(Tab) should report consumed")
	}
	if got := r.Current(); got != 1 {
		t.Fatalf("after HandleKey(Tab): Current() = %d, want 1", got)
	}

	if !r.HandleKey("Shift+Tab") {
		t.Fatalf("HandleKey(Shift+Tab) should report consumed")
	}
	if got := r.Current(); got != 0 {
		t.Fatalf("after HandleKey(Shift+Tab): Current() = %d, want 0", got)
	}

	if r.HandleKey("Enter") {
		t.Fatalf("HandleKey(Enter) should report unconsumed")
	}
	if got := r.Current(); got != 0 {
		t.Fatalf("unconsumed key must not move focus: Current() = %d, want 0", got)
	}
}
