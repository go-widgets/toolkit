// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package textdiff

import "testing"

func TestOpString(t *testing.T) {
	for op, want := range map[Op]string{Equal: "=", Delete: "-", Insert: "+", Op(9): "Op(9)"} {
		if got := op.String(); got != want {
			t.Errorf("%d: %q want %q", op, got, want)
		}
	}
}

func TestRangeOf(t *testing.T) {
	for _, c := range []struct {
		start, n int
		want     string
	}{{4, 0, "4,0"}, {4, 1, "5"}, {4, 3, "5,3"}} {
		if got := rangeOf(c.start, c.n); got != c.want {
			t.Errorf("rangeOf(%d,%d)=%q want %q", c.start, c.n, got, c.want)
		}
	}
}

// middle on an empty or inverted range finds no snake and says so, rather
// than looping: the guard behind the loop.
func TestMiddleOnNothing(t *testing.T) {
	d := &differ{budget: -1}
	if _, _, ok := d.middle(1, 0, 0, 0); ok {
		t.Fatal("found a snake in nothing")
	}
}
