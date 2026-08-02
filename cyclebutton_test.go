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
	var gotVal string
	calls := 0
	c.OnChange = func(i int, v string) { gotIdx, gotVal, calls = i, v, calls+1 }
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	c.Draw(newP(makeSurface(80, 24), 80), DefaultLight()) // paints the active label

	c.OnEvent(Event{Kind: EventClick}) // List → Grid
	if c.Value() != "Grid" || gotIdx != 1 || gotVal != "Grid" || calls != 1 {
		t.Fatalf("after 1 click: value=%q cb=%d/%q calls=%d", c.Value(), gotIdx, gotVal, calls)
	}
	c.OnEvent(Event{Kind: EventClick}) // Grid → Compact
	c.OnEvent(Event{Kind: EventClick}) // Compact → List (wrap)
	if c.Value() != "List" {
		t.Fatalf("wrap: value=%q, want List", c.Value())
	}
	// A non-click event is ignored.
	c.OnEvent(Event{Kind: EventKeyDown})
	if c.Value() != "List" {
		t.Fatal("key event should not cycle")
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
	c.Index = 99
	if c.Value() != "" {
		t.Fatal("out-of-range index should yield empty Value")
	}

	// Nil OnChange is safe.
	n := NewCycleButton("a", "b")
	n.OnEvent(Event{Kind: EventClick})
	if n.Value() != "b" {
		t.Fatalf("nil-callback click: %q", n.Value())
	}
}
