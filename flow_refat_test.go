// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestFlowLayout checks items flow left-to-right and wrap on overflow, with the
// per-item width taken from Item.Size or, when unset, the widget's Bounds.
func TestFlowLayout(t *testing.T) {
	a, b, c := &dockProbe{}, &dockProbe{}, &dockProbe{}
	// c has no Item.Size, so its width comes from its current Bounds.
	c.SetBounds(Rect{W: 30, H: 0})

	cont := NewContainer(&FlowLayout{RowHeight: 10, HGap: 5, VGap: 4})
	cont.Add(Item{Widget: a, Size: 40})
	cont.Add(Item{Widget: b, Size: 40})
	cont.Add(Item{Widget: c}) // natural width 30
	cont.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 100})

	// a at x0; b at 45 (40+5). b right edge = 85 <= 90, fits on row 0.
	if got := a.Bounds(); got != (Rect{X: 0, Y: 0, W: 40, H: 10}) {
		t.Fatalf("a = %+v", got)
	}
	if got := b.Bounds(); got != (Rect{X: 45, Y: 0, W: 40, H: 10}) {
		t.Fatalf("b = %+v", got)
	}
	// c: x would be 90 (85+5); 90+30 > 90 → wrap to row 1 (y = 10+4 = 14), x=0.
	if got := c.Bounds(); got != (Rect{X: 0, Y: 14, W: 30, H: 10}) {
		t.Fatalf("c (wrapped, natural width) = %+v", got)
	}
}

// TestFlowLayoutNoWrapAtRowStart checks an over-wide item at the start of a row is
// NOT wrapped (there is nowhere to wrap to).
func TestFlowLayoutNoWrapAtRowStart(t *testing.T) {
	a := &dockProbe{}
	cont := NewContainer(&FlowLayout{RowHeight: 8})
	cont.Add(Item{Widget: a, Size: 200}) // wider than the container
	cont.SetBounds(Rect{X: 1, Y: 2, W: 50, H: 20})
	if got := a.Bounds(); got != (Rect{X: 1, Y: 2, W: 200, H: 8}) {
		t.Fatalf("oversize-at-start should not wrap: %+v", got)
	}
}

// TestViewControllerRefAt checks reference hit-testing, including reverse order
// (a later overlapping ref shadows an earlier one) and the no-hit case.
func TestViewControllerRefAt(t *testing.T) {
	back, list := &dockProbe{}, &dockProbe{}
	vc := NewViewController(VBoxNode(
		Leaf(back).Ref("back").Sized(20),
		Leaf(list).Ref("list").Flexed(1),
	))
	vc.Root().SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	if n, ok := vc.RefAt(10, 5); !ok || n != "back" { // in the top 20px band
		t.Fatalf("RefAt in back = %q,%v", n, ok)
	}
	if n, ok := vc.RefAt(10, 60); !ok || n != "list" {
		t.Fatalf("RefAt in list = %q,%v", n, ok)
	}
	if _, ok := vc.RefAt(500, 500); ok {
		t.Fatal("RefAt outside every ref should report ok=false")
	}

	// Overlap: a second ref covering the whole area, added later, must shadow the
	// earlier ones (reverse-order scan).
	over := &dockProbe{}
	vc2 := NewViewController(FitNode(
		Leaf(back).Ref("under"),
		Leaf(over).Ref("over"),
	))
	vc2.Root().SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	if n, ok := vc2.RefAt(20, 20); !ok || n != "over" {
		t.Fatalf("overlapping RefAt should pick the later ref, got %q,%v", n, ok)
	}
}
