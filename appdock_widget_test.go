// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

var _ Widget = (*AppDock)(nil) // the point of #578: AppDock satisfies Widget

func widgetDock() *AppDock {
	d := NewAppDock(AppDockItem{Id: "a", Label: "Alpha"}, AppDockItem{Id: "b", Label: "Beta"}, AppDockItem{Id: "c"})
	d.SetBounds(Rect{X: 10, Y: 20, W: 3*scaled(AppDockItemW) + 4*scaled(AppDockGap), H: 40})
	return d
}

// ItemAt is the former index HitTest: an item's index, -1 in the padding.
func TestAppDockItemAt(t *testing.T) {
	d := widgetDock()
	for i, r := range d.ItemRects() {
		if got := d.ItemAt(r.X+1, r.Y+1); got != i {
			t.Errorf("ItemAt over item %d = %d", i, got)
		}
	}
	b := d.Bounds()
	if got := d.ItemAt(b.X, b.Y); got != -1 {
		t.Errorf("ItemAt in the corner padding = %d, want -1", got)
	}
}

// HitTest is the Widget contract: absolute, true over the bounds (padding
// included) and over an item that overflows them while swollen, false elsewhere.
func TestAppDockHitTestIsTheWidgetContract(t *testing.T) {
	d := widgetDock()
	b := d.Bounds()
	if !d.HitTest(b.X, b.Y) {
		t.Error("the padding inside the bounds must hit (bounds clause)")
	}
	if d.ItemAt(b.X, b.Y) != -1 {
		t.Fatal("fixture: the corner must be padding, or the bounds clause is not what answered")
	}
	if d.HitTest(b.X-1, b.Y) || d.HitTest(b.X+b.W, b.Y+b.H) {
		t.Error("a point just outside the bounds and over no item must miss")
	}

	// Swollen, the row overflows the bounds: the overflowing item still hits.
	d.SetCursor(b.X+b.W/2, true)
	first := d.ItemRects()[0]
	if first.X >= b.X {
		t.Fatalf("fixture: swollen item 0 (%+v) does not overflow the bounds %+v", first, b)
	}
	if !d.HitTest(first.X, first.Y+1) {
		t.Error("the part of a swollen item outside the bounds must hit (item clause)")
	}
}

// Items are the toolbar's child nodes: a button named by the label, at the
// item's live rectangle, and the walk reaches them.
func TestAppDockItemsAreA11yNodes(t *testing.T) {
	d := widgetDock()
	nodes := WalkA11y(d)
	if len(nodes) != 4 || nodes[0].Role != RoleToolbar {
		t.Fatalf("nodes = %+v, want the toolbar and 3 buttons", nodes)
	}
	for i, want := range []string{"Alpha", "Beta", ""} {
		n := nodes[i+1]
		if n.Role != RoleButton || n.Name != want {
			t.Errorf("node %d = %+v, want button %q", i+1, n.A11yInfo, want)
		}
	}
	// the rectangle follows the swell, as ItemRects does
	d.SetCursor(d.Bounds().X+d.Bounds().W/2, true)
	rects := d.ItemRects()
	for i, k := range d.Children() {
		if k.Bounds() != rects[i] {
			t.Errorf("node %d rect = %+v, want the item rect %+v", i, k.Bounds(), rects[i])
		}
		x, y := centre(k.Bounds())
		if got := d.ItemAt(x, y); got != i {
			t.Errorf("node %d centre sits over item %d", i, got)
		}
	}
}

// Disabled reaches the dock's node and, through the walk, every item node.
func TestAppDockDisabledReachesItsNodes(t *testing.T) {
	d := widgetDock()
	if d.A11y().Disabled {
		t.Error("a fresh dock reads disabled")
	}
	d.Disabled().Set(true)
	if !d.A11y().Disabled {
		t.Error("A11y().Disabled is false on a disabled dock")
	}
	for _, n := range WalkA11y(d) {
		if !n.Disabled {
			t.Errorf("%s %q reads enabled in a disabled dock", n.Role, n.Name)
		}
	}
}

// A disabled dock does not swell, whichever way the pointer got there: the move
// event, or the host-driven SetCursor that bypasses OnEvent.
func TestAppDockDisabledDoesNotMagnify(t *testing.T) {
	d := widgetDock()
	resting := d.ItemRects()
	x := resting[1].X + resting[1].W/2

	d.Disabled().Set(true)
	d.OnEvent(Event{Kind: EventMouseMove, X: x - d.Bounds().X, Y: 5})
	if d.cursorInside {
		t.Error("a move on a disabled dock was tracked")
	}
	d.SetCursor(x, true) // host-driven: OnEvent's gate is not on this path
	if got := d.ItemRects(); got[1] != resting[1] {
		t.Errorf("disabled dock swelled through SetCursor: %+v, resting %+v", got[1], resting[1])
	}

	// hover already held when the dock is disabled: the next event drops it, so
	// re-enabling does not swell from a stale position
	d.Disabled().Set(false)
	d.SetCursor(x, true)
	if got := d.ItemRects(); got[1] == resting[1] {
		t.Fatal("fixture: the enabled dock should swell")
	}
	d.Disabled().Set(true)
	d.OnEvent(Event{Kind: EventMouseMove, X: x - d.Bounds().X, Y: 5})
	d.Disabled().Set(false)
	if got := d.ItemRects(); got[1] != resting[1] {
		t.Errorf("re-enabled dock swells from a stale pointer: %+v", got[1])
	}
}
