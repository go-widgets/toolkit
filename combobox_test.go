// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// newTestComboBox returns a combo bounded at the top-left so popover coordinate
// math is exercised, seeded with a handful of fruit options.
func newTestComboBox() *ComboBox {
	c := NewComboBox([]string{"Apple", "Apricot", "Banana", "Cherry"})
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	return c
}

func TestComboBoxFilteredEmptyReturnsAll(t *testing.T) {
	c := newTestComboBox()
	got := c.Filtered()
	if len(got) != 4 {
		t.Fatalf("Filtered() with empty text = %d options, want 4", len(got))
	}
}

func TestComboBoxFilteredSubset(t *testing.T) {
	c := newTestComboBox()
	// "ap" (case-insensitive) matches "Apple" + "Apricot" only.
	c.Text().Set("AP")
	got := c.Filtered()
	if len(got) != 2 || got[0] != "Apple" || got[1] != "Apricot" {
		t.Fatalf("Filtered(%q) = %v, want [Apple Apricot]", c.Text().Get(), got)
	}
}

func TestComboBoxFilteredNoMatch(t *testing.T) {
	c := newTestComboBox()
	c.Text().Set("zzz")
	if got := c.Filtered(); len(got) != 0 {
		t.Fatalf("Filtered(%q) = %v, want none", c.Text().Get(), got)
	}
}

func TestComboBoxVisibleClampsToMaxRows(t *testing.T) {
	opts := make([]string, PopoverMaxRows+3)
	for i := range opts {
		opts[i] = "row" + itoa(i)
	}
	c := NewComboBox(opts)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	// PopoverBounds height is proportional to the clamped visible count.
	if got := c.PopoverBounds().H; got != PopoverMaxRows*comboRowH {
		t.Fatalf("clamped popover height = %d, want %d", got, PopoverMaxRows*comboRowH)
	}
}

func TestComboBoxPopoverBounds(t *testing.T) {
	c := newTestComboBox()
	pb := c.PopoverBounds()
	want := Rect{X: 0, Y: 24, W: 120, H: 4 * comboRowH}
	if pb != want {
		t.Fatalf("PopoverBounds() = %+v, want %+v", pb, want)
	}
}

func TestComboBoxDrawClosedWithText(t *testing.T) {
	c := newTestComboBox()
	c.Text().Set("Banana")
	surf := makeSurface(200, 300)
	c.Draw(newP(surf, 200), DefaultLight())
	// The chevron's widest row (a plain fillRect) lands on the right edge.
	if got := pixelAt(surf, 200, 107, 11); got != DefaultLight().OnSurface {
		t.Errorf("chevron pixel = %+v, want OnSurface", got)
	}
}

func TestComboBoxDrawClosedWithPlaceholder(t *testing.T) {
	c := newTestComboBox()
	c.Placeholder = "search fruit…"
	surf := makeSurface(200, 300)
	// Empty text + placeholder exercises the muted-hint branch.
	c.Draw(newP(surf, 200), DefaultLight())
}

func TestComboBoxDrawOpenPaintsPopover(t *testing.T) {
	c := newTestComboBox()
	c.Open().Set(true)
	surf := makeSurface(200, 300)
	c.Draw(newP(surf, 200), DefaultLight())
	pb := c.PopoverBounds()
	if got := pixelAt(surf, 200, pb.X, pb.Y); got != DefaultLight().Border {
		t.Errorf("open popover corner = %+v, want Border", got)
	}
}

func TestComboBoxTypingFiltersAndNotifiesText(t *testing.T) {
	c := newTestComboBox()
	var changed string
	c.Text().Subscribe(func(s string) { changed = s })
	c.OnEvent(Event{Kind: EventChar, Code: "B"})
	if c.Text().Get() != "B" || changed != "B" {
		t.Fatalf("after typing: Text=%q changed=%q, want both B", c.Text().Get(), changed)
	}
	if !c.Open().Get() {
		t.Error("typing should open the popover")
	}
	if got := c.Filtered(); len(got) != 1 || got[0] != "Banana" {
		t.Fatalf("Filtered after 'B' = %v, want [Banana]", got)
	}
}

func TestComboBoxCharEmptyIgnored(t *testing.T) {
	c := newTestComboBox()
	c.OnEvent(Event{Kind: EventChar, Code: ""})
	if c.Text().Get() != "" || c.Open().Get() {
		t.Errorf("empty char should be a no-op, got Text=%q Open=%v", c.Text().Get(), c.Open().Get())
	}
}

func TestComboBoxBackspaceEdits(t *testing.T) {
	c := newTestComboBox()
	c.Text().Set("Ba")
	var changed string
	c.Text().Subscribe(func(s string) { changed = s })
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c.Text().Get() != "B" || changed != "B" {
		t.Fatalf("after backspace: Text=%q changed=%q, want both B", c.Text().Get(), changed)
	}
	if !c.Open().Get() {
		t.Error("backspace should open the popover")
	}
}

func TestComboBoxBackspaceEmptyNoop(t *testing.T) {
	c := newTestComboBox()
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c.Text().Get() != "" {
		t.Errorf("backspace on empty text should be a no-op, got %q", c.Text().Get())
	}
}

func TestComboBoxClickTogglesOpen(t *testing.T) {
	c := newTestComboBox()
	c.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !c.Open().Get() {
		t.Fatal("field click did not open popover")
	}
	// A second click on the field (above the popover) closes it.
	c.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if c.Open().Get() {
		t.Fatal("second field click did not close popover")
	}
}

func TestComboBoxClickOptionSelects(t *testing.T) {
	c := newTestComboBox()
	c.Open().Set(true)
	var changed string
	c.Text().Subscribe(func(s string) { changed = s })
	pb := c.PopoverBounds()
	// Row index 1 ("Apricot"): field-local y = (pb.Y - r.Y) + row*comboRowH + half.
	ly := 1*comboRowH + comboRowH/2
	c.OnEvent(Event{Kind: EventClick, X: 10, Y: (pb.Y - 0) + ly})
	if c.Text().Get() != "Apricot" || changed != "Apricot" {
		t.Fatalf("select: Text=%q notified=%q, want all Apricot", c.Text().Get(), changed)
	}
	if c.Open().Get() {
		t.Error("selecting an option should close the popover")
	}
}

func TestComboBoxEnterSelectsFirstFiltered(t *testing.T) {
	c := newTestComboBox()
	c.Text().Set("ap")
	var selected string
	c.Text().Subscribe(func(s string) { selected = s })
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.Text().Get() != "Apple" || selected != "Apple" {
		t.Fatalf("Enter: Text=%q notified=%q, want Apple", c.Text().Get(), selected)
	}
}

func TestComboBoxEnterNoMatchNoop(t *testing.T) {
	c := newTestComboBox()
	c.Text().Set("zzz")
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.Text().Get() != "zzz" {
		t.Errorf("Enter with no match should not change Text, got %q", c.Text().Get())
	}
}

// TestComboBoxZeroValueBindsThroughAccessors builds a bare &ComboBox{} (no
// constructor) and drives it purely through the accessors: the first Text() /
// Open() call lazily initialises each nil Observable to its zero value, and a
// host binding via Subscribe sees every mutation. This proves the widget is
// usable as a plain struct literal and that hosts bind to the reactive state
// exactly as they would after NewComboBox.
func TestComboBoxZeroValueBindsThroughAccessors(t *testing.T) {
	c := &ComboBox{Options: []string{"Apple", "Apricot"}}
	// Nil -> lazy init to the zero value.
	if c.Text().Get() != "" {
		t.Fatalf("zero-value Text().Get() = %q, want empty", c.Text().Get())
	}
	if c.Open().Get() {
		t.Fatal("zero-value Open().Get() = true, want false")
	}
	// Host binds both observables.
	var gotText string
	var gotOpen bool
	c.Text().Subscribe(func(s string) { gotText = s })
	c.Open().Subscribe(func(b bool) { gotOpen = b })
	// A keystroke edit Sets Text and opens the popover; the host sees both.
	c.OnEvent(Event{Kind: EventChar, Code: "A"})
	if c.Text().Get() != "A" || gotText != "A" {
		t.Fatalf("bound Text after typing: Get=%q host=%q, want A", c.Text().Get(), gotText)
	}
	if !c.Open().Get() || !gotOpen {
		t.Fatalf("bound Open after typing: Get=%v host=%v, want true", c.Open().Get(), gotOpen)
	}
	// Picking an option Sets Text and closes; the host sees both.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.Text().Get() != "Apple" || gotText != "Apple" {
		t.Fatalf("bound Text after Enter: Get=%q host=%q, want Apple", c.Text().Get(), gotText)
	}
	if c.Open().Get() || gotOpen {
		t.Fatalf("bound Open after Enter: Get=%v host=%v, want false", c.Open().Get(), gotOpen)
	}
}

func TestComboBoxUnhandledEventsSafe(t *testing.T) {
	c := newTestComboBox()
	// An unhandled key code and event kind are quietly ignored.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	c.OnEvent(Event{Kind: EventMouseUp, X: 1, Y: 1})
	if c.Text().Get() != "" || c.Open().Get() {
		t.Errorf("unhandled events mutated state: Text=%q Open=%v", c.Text().Get(), c.Open().Get())
	}
}
