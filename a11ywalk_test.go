// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestWalkA11yNil(t *testing.T) {
	if got := WalkA11y(nil); got != nil {
		t.Fatalf("WalkA11y(nil) = %+v, want nil", got)
	}
}

// A container is presentational and has children, so one tree exercises both
// halves of the walk: the container itself must NOT be announced, and its
// children must still be reached through it.
func TestWalkA11yDescendsThroughPresentation(t *testing.T) {
	box := NewContainer(nil)
	ok := NewButton("OK", nil)
	label := NewLabel("Hello")
	box.AddWidget(ok).AddWidget(label)

	got := WalkA11y(box)
	if len(got) != 2 {
		t.Fatalf("WalkA11y = %d nodes, want 2 (the container is presentational): %+v", len(got), got)
	}
	if got[0].Role != RoleButton || got[0].Name != "OK" {
		t.Errorf("node 0 = %+v, want the button", got[0].A11yInfo)
	}
	if got[1].Role != RoleText || got[1].Name != "Hello" {
		t.Errorf("node 1 = %+v, want the label", got[1].A11yInfo)
	}
}

// The bounds a node carries are the widget's own, NOT its bounds offset by its
// ancestors': this toolkit lays out in surface coordinates throughout. A walk
// that accumulated offsets would place every nested element at roughly twice
// its true distance from the origin, which reads as plausible and points a
// screen reader at the wrong part of the window.
func TestWalkA11yBoundsAreNotAccumulated(t *testing.T) {
	box := NewContainer(nil)
	box.SetBounds(Rect{X: 10, Y: 20, W: 200, H: 100})
	btn := NewButton("Go", nil)
	btn.SetBounds(Rect{X: 30, Y: 40, W: 50, H: 24})
	box.AddWidget(btn)

	got := WalkA11y(box)
	if len(got) != 1 {
		t.Fatalf("WalkA11y = %d nodes, want 1: %+v", len(got), got)
	}
	want := Rect{X: 30, Y: 40, W: 50, H: 24}
	if got[0].Rect != want {
		t.Fatalf("button rect = %+v, want %+v (the container's origin must not be added)", got[0].Rect, want)
	}
}

// A leaf that is accessible and has no children is the simplest case, and the
// one every bridge sees most of.
func TestWalkA11yLeaf(t *testing.T) {
	btn := NewButton("Only", nil)
	btn.SetBounds(Rect{X: 1, Y: 2, W: 3, H: 4})
	got := WalkA11y(btn)
	if len(got) != 1 {
		t.Fatalf("WalkA11y = %d nodes, want 1", len(got))
	}
	if got[0].Name != "Only" || got[0].Rect != (Rect{X: 1, Y: 2, W: 3, H: 4}) {
		t.Fatalf("node = %+v rect %+v, want the button at 1,2 3x4", got[0].A11yInfo, got[0].Rect)
	}
}

// A nil child must be stepped over rather than panicking: containers are built
// incrementally by application code, and one empty slot should not take the
// whole accessibility tree down with it.
func TestWalkA11ySkipsNilChild(t *testing.T) {
	box := NewContainer(nil)
	box.AddWidget(nil)
	box.AddWidget(NewButton("After", nil))

	got := WalkA11y(box)
	if len(got) != 1 || got[0].Name != "After" {
		t.Fatalf("WalkA11y = %+v, want just the button after the nil slot", got)
	}
}
