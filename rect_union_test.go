// Copyright (c) 2026 the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Rect is an alias of painter.Rect, so painter's Union is toolkit's. This pins
// that a toolkit consumer (a host folding the damage of a frame) gets the
// bounding box, with an empty rectangle as the identity, through toolkit.Rect.
func TestRectHasUnion(t *testing.T) {
	a, b := Rect{X: 0, Y: 0, W: 10, H: 10}, Rect{X: 5, Y: 8, W: 10, H: 10}
	if got, want := a.Union(b), (Rect{X: 0, Y: 0, W: 15, H: 18}); got != want {
		t.Fatalf("Union = %v, want %v", got, want)
	}
	if got := (Rect{}).Union(b); got != b {
		t.Fatalf("empty.Union(b) = %v, want b", got)
	}
	var fold Rect
	for _, r := range []Rect{a, {}, b} {
		fold = fold.Union(r)
	}
	if fold != a.Union(b) {
		t.Fatalf("a fold with an empty rect in it = %v, want %v", fold, a.Union(b))
	}
}
