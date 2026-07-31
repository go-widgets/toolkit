// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestBorderLayout sets all five regions and checks the classic border geometry:
// north/south span the full width, west/east span the band between them, center
// fills the rest. Regions are assigned out of order to prove the precedence is
// structural, not insertion-based.
func TestBorderLayout(t *testing.T) {
	n, s, e, w, c := &dockProbe{}, &dockProbe{}, &dockProbe{}, &dockProbe{}, &dockProbe{}
	b := NewBorder()
	// Assign in a deliberately scrambled order.
	b.Center = c
	b.East = e
	b.NorthSize, b.North = 10, n
	b.WestSize, b.West = 12, w
	b.SouthSize, b.South = 8, s
	b.EastSize = 6
	b.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	if got := n.Bounds(); got != (Rect{X: 0, Y: 0, W: 100, H: 10}) {
		t.Fatalf("north = %+v", got)
	}
	if got := s.Bounds(); got != (Rect{X: 0, Y: 42, W: 100, H: 8}) {
		t.Fatalf("south = %+v", got)
	}
	if got := w.Bounds(); got != (Rect{X: 0, Y: 10, W: 12, H: 32}) {
		t.Fatalf("west = %+v", got)
	}
	if got := e.Bounds(); got != (Rect{X: 94, Y: 10, W: 6, H: 32}) {
		t.Fatalf("east = %+v", got)
	}
	if got := c.Bounds(); got != (Rect{X: 12, Y: 10, W: 82, H: 32}) {
		t.Fatalf("center = %+v", got)
	}
}

// TestBorderPartial checks nil regions are skipped and a lone center fills the
// whole rect, and that a negative size clamps to zero.
func TestBorderPartial(t *testing.T) {
	// Center only: fills everything.
	c := &dockProbe{}
	b := NewBorder()
	b.Center = c
	b.SetBounds(Rect{X: 3, Y: 4, W: 30, H: 20})
	if got := c.Bounds(); got != (Rect{X: 3, Y: 4, W: 30, H: 20}) {
		t.Fatalf("lone center = %+v", got)
	}

	// North with a negative size clamps to 0; no center (that branch stays false).
	n := &dockProbe{}
	b2 := NewBorder()
	b2.North, b2.NorthSize = n, -5
	b2.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	if n.Bounds().H != 0 {
		t.Fatalf("negative north size not clamped: H=%d", n.Bounds().H)
	}
}

// TestBorderDrawEvents checks Draw paints without panic and events route to a
// region, and miss cleanly when outside every region.
func TestBorderDrawEvents(t *testing.T) {
	n, c := &dockProbe{}, &dockProbe{}
	b := NewBorder()
	b.North, b.NorthSize = n, 10
	b.Center = c
	b.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	b.Draw(newP(makeSurface(100, 50), 100), DefaultLight()) // no panic

	b.OnEvent(Event{Kind: EventClick, X: 20, Y: 5}) // in the north band
	if !n.got || c.got {
		t.Fatalf("north routing: n=%v c=%v", n.got, c.got)
	}
	n.got = false
	b.OnEvent(Event{Kind: EventClick, X: 20, Y: 30}) // in the center
	if !c.got || n.got {
		t.Fatalf("center routing: n=%v c=%v", n.got, c.got)
	}
	// A Border with no regions at all: OnEvent is a safe no-op.
	NewBorder().OnEvent(Event{Kind: EventClick, X: 1, Y: 1})
}
