// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestCycleButton(t *testing.T) {
	c := NewCycleButton("List", "Grid", "Compact")
	if c.Value() != "List" {
		t.Fatalf("initial = %q, want List", c.Value())
	}

	var gotIdx int
	calls := 0
	c.Index().Subscribe(func(i int) { gotIdx, calls = i, calls+1 })
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	c.Draw(newP(makeSurface(80, 24), 80), DefaultLight()) // paints the active label

	c.OnEvent(Event{Kind: EventClick}) // List → Grid
	if c.Value() != "Grid" || c.Index().Get() != 1 || gotIdx != 1 || calls != 1 {
		t.Fatalf("after 1 click: value=%q idx=%d sub=%d calls=%d", c.Value(), c.Index().Get(), gotIdx, calls)
	}
	c.OnEvent(Event{Kind: EventClick}) // Grid → Compact
	c.OnEvent(Event{Kind: EventClick}) // Compact → List (wrap)
	if c.Value() != "List" {
		t.Fatalf("wrap: value=%q, want List", c.Value())
	}
	// An unmatched key event is ignored.
	c.OnEvent(Event{Kind: EventKeyDown})
	if c.Value() != "List" {
		t.Fatal("unmatched key event should not cycle")
	}

	// Empty options: Value is "", Draw paints only chrome, click is a no-op.
	empty := NewCycleButton()
	if empty.Value() != "" {
		t.Fatal("no options should yield empty Value")
	}
	empty.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	empty.Draw(newP(makeSurface(40, 20), 40), DefaultLight())
	empty.OnEvent(Event{Kind: EventClick}) // no panic, no change

	// Out-of-range index → empty Value.
	c.Index().Set(99)
	if c.Value() != "" {
		t.Fatal("out-of-range index should yield empty Value")
	}

	// A click with no subscriber attached still advances (no panic).
	n := NewCycleButton("a", "b")
	n.OnEvent(Event{Kind: EventClick})
	if n.Value() != "b" {
		t.Fatalf("no-subscriber click: %q", n.Value())
	}
}

// TestCycleButtonIndexObservable covers the zero-value lazy-init of the Index
// accessor and the host binding path: a CycleButton built as a bare struct (no
// NewCycleButton) still yields a usable Observable, and Setting it from outside
// is reflected by the widget (there is no imperative Index field).
func TestCycleButtonIndexObservable(t *testing.T) {
	c := &CycleButton{Options: []string{"one", "two", "three"}} // index Observable nil until accessed
	if c.Index().Get() != 0 {
		t.Fatalf("zero-value CycleButton Index = %d, want 0", c.Index().Get())
	}
	if c.Value() != "one" {
		t.Fatalf("bare CycleButton Value = %q, want one", c.Value())
	}
	seen := -1
	c.Index().Subscribe(func(i int) { seen = i })
	c.Index().Set(2) // a host drives the cycle button through the Observable
	if c.Index().Get() != 2 || seen != 2 || c.Value() != "three" {
		t.Fatalf("host Set: idx=%d subscriber=%d value=%q, want 2/2/three", c.Index().Get(), seen, c.Value())
	}
}
