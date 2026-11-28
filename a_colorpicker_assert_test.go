// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// These two tests sort first on purpose (a_ prefix): a mutant that breaks the
// guard they hold crashes the later ColorPicker tests with a nil dereference, and
// a crash is not a test. Here the failure is an assertion, reached before any
// crash, and a panic is recovered into one.

// Color() lazily creates the Observable once: a bare picker returns a non-nil
// one, and every later call returns the same one. (colorpicker.go:58)
func TestAColorPickerColorIsCreatedOnceAndNeverNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Color() panicked: %v", r)
		}
	}()
	cp := &ColorPicker{h: 120, s: 1, v: 1, alpha: 255}
	first := cp.Color()
	if first == nil {
		t.Fatal("Color() of a bare picker is nil: the lazy creation did not run")
	}
	if got, want := first.Get(), (RGBA{R: 0, G: 255, B: 0, A: 255}); got != want {
		t.Errorf("Color() seeded %+v, want %+v", got, want)
	}
	if cp.Color() != first {
		t.Error("a second Color() returned another Observable, want the same one")
	}
	// The creation also wired the host-Set path: a Set reseeds the HSV state.
	first.Set(RGBA{R: 255, G: 0, B: 0, A: 128})
	if cp.h != 0 || cp.s != 1 || cp.v != 1 || cp.alpha != 128 {
		t.Errorf("a host Set left h=%v s=%v v=%v alpha=%d, want 0 1 1 128", cp.h, cp.s, cp.v, cp.alpha)
	}
}

// clampInt confines v to [lo, hi] on both sides. (colorpicker.go:474)
func TestAClampIntBothSides(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("clampInt panicked: %v", r)
		}
	}()
	for _, c := range []struct{ v, lo, hi, want int }{
		{-5, 0, 10, 0}, {0, 0, 10, 0}, {5, 0, 10, 5}, {10, 0, 10, 10}, {15, 0, 10, 10},
	} {
		if got := clampInt(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clampInt(%d,%d,%d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
