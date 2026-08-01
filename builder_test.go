// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestBuildLeaf: a leaf node builds to its own widget.
func TestBuildLeaf(t *testing.T) {
	w := &dockProbe{}
	if got := Leaf(w).Build(); got != Widget(w) {
		t.Fatalf("leaf Build returned %v, want the widget itself", got)
	}
}

// TestBuildEmptyNode: a node with neither Widget nor Layout builds an empty
// container (no panic, no children).
func TestBuildEmptyNode(t *testing.T) {
	got := Node{}.Build()
	c, ok := got.(*Container)
	if !ok || len(c.Items()) != 0 {
		t.Fatalf("empty node Build = %T with %d items, want empty *Container", got, len(c.Items()))
	}
}

// TestBuildBorderTree builds a border shell declaratively and checks the leaf
// widgets land in the right regions after one SetBounds — proving the tree both
// instantiates and lays out.
func TestBuildBorderTree(t *testing.T) {
	top, side, body := &dockProbe{}, &dockProbe{}, &dockProbe{}
	root := BorderNode(
		Leaf(top).At(RegionNorth).Sized(10),
		Leaf(side).At(RegionWest).Sized(20),
		Leaf(body).At(RegionCenter),
	).Build()
	root.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	if b := top.Bounds(); b != (Rect{X: 0, Y: 0, W: 100, H: 10}) {
		t.Fatalf("north = %+v", b)
	}
	if b := side.Bounds(); b != (Rect{X: 0, Y: 10, W: 20, H: 40}) {
		t.Fatalf("west = %+v", b)
	}
	if b := body.Bounds(); b != (Rect{X: 20, Y: 10, W: 80, H: 40}) {
		t.Fatalf("center = %+v", b)
	}
}

// TestBuildNestedBox builds nested V/H boxes with flex and fixed sizing and checks
// the deep leaf lands correctly — exercising recursive Build + VBoxNode/HBoxNode
// + Flexed/Sized.
func TestBuildNestedBox(t *testing.T) {
	header, nav, main := &dockProbe{}, &dockProbe{}, &dockProbe{}
	root := VBoxNode(
		Leaf(header).Sized(40),
		HBoxNode(
			Leaf(nav).Sized(200),
			Leaf(main).Flexed(1),
		).Flexed(1),
	).Build()
	root.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})

	// header: full width, top 40.
	if b := header.Bounds(); b != (Rect{X: 0, Y: 0, W: 400, H: 40}) {
		t.Fatalf("header = %+v", b)
	}
	// The HBox flexes to fill Y 40..300 (spacing default 4 after header): the box
	// row starts at Y=44, height 256. nav fixed 200, main flexes the rest.
	if b := nav.Bounds(); b.X != 0 || b.Y != 44 || b.W != 200 || b.H != 256 {
		t.Fatalf("nav = %+v, want X0 Y44 W200 H256", b)
	}
	if b := main.Bounds(); b.X != 204 || b.W != 196 { // 400-200-4 gap
		t.Fatalf("main = %+v, want X204 W196", b)
	}
}

// TestBuildFitAndCard covers FitNode and CardNode constructors and that the built
// card shows only its active child.
func TestBuildFitAndCard(t *testing.T) {
	f := &dockProbe{}
	fit := FitNode(Leaf(f)).Build()
	fit.SetBounds(Rect{X: 1, Y: 2, W: 8, H: 6})
	if f.Bounds() != (Rect{X: 1, Y: 2, W: 8, H: 6}) {
		t.Fatalf("fit child = %+v", f.Bounds())
	}

	a, b := &dockProbe{}, &dockProbe{}
	card := CardNode(1, Leaf(a), Leaf(b)).Build()
	card.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	if a.Bounds() != (Rect{}) || b.Bounds() != (Rect{X: 0, Y: 0, W: 10, H: 10}) {
		t.Fatalf("card active 1: a=%+v b=%+v", a.Bounds(), b.Bounds())
	}
}
