// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Optional children leave, last first, as the column shrinks; the flexible child
// keeps at least its keep; a child that leaves has an empty Rect and no gap.
func TestVBoxOptionalLeavesAsTheColumnShrinks(t *testing.T) {
	mk := func(h int) (v *VBox, rows [3]*Label, rest *Label) {
		v = NewVBox()
		v.Spacing = 4
		for i := range rows {
			rows[i] = NewLabel("r")
			v.AddOptional(rows[i], 20, 40)
		}
		rest = NewLabel("rest")
		v.AddFlex(rest, 1)
		v.SetBounds(Rect{W: 100, H: h})
		return
	}
	cases := []struct {
		h     int
		shown int
	}{
		// row k needs k*20 + k gaps of 4 + the flexible child's 40 (its gap included)
		{400, 3}, {112, 3}, {111, 2}, {88, 2}, {87, 1}, {64, 1}, {63, 0}, {0, 0},
	}
	for _, c := range cases {
		_, rows, rest := mk(c.h)
		got := 0
		for _, r := range rows {
			if r.Bounds().H > 0 {
				got++
			}
		}
		if got != c.shown {
			t.Errorf("h=%d: %d rows shown, want %d", c.h, got, c.shown)
		}
		if c.h > 0 && rest.Bounds().H < min(40, c.h) {
			t.Errorf("h=%d: the flexible child has %d px", c.h, rest.Bounds().H)
		}
	}
}

// The exact boundary: the flexible child keeps keep pixels, gaps excluded.
func TestVBoxOptionalBoundary(t *testing.T) {
	const sp, size, keep = 4, 20, 40
	row, rest := NewLabel("row"), NewLabel("rest")
	build := func(h int) {
		v := NewVBox()
		v.Spacing = sp
		v.AddOptional(row, size, keep)
		v.AddFlex(rest, 1)
		v.SetBounds(Rect{W: 100, H: h})
	}
	need := size + sp + keep // the row, a gap, and the rest's keep
	build(need)
	if row.Bounds().H != size || rest.Bounds().H != keep || rest.Bounds().Y != size+sp {
		t.Errorf("at %d: row %+v rest %+v", need, row.Bounds(), rest.Bounds())
	}
	build(need - 1)
	if row.Bounds() != (Rect{}) || rest.Bounds().Y != 0 || rest.Bounds().H != need-1 {
		t.Errorf("at %d: row %+v rest %+v", need-1, row.Bounds(), rest.Bounds())
	}
}

// Optional children outrank each other in the order they were added, and one
// that leaves takes the later ones with it, whatever their keep.
func TestVBoxOptionalPrefixAndPerChildKeep(t *testing.T) {
	a, b, c, rest := NewLabel("a"), NewLabel("b"), NewLabel("c"), NewLabel("rest")
	v := NewVBox()
	v.Spacing = 0
	v.AddOptional(a, 10, 10) // stays down to 20 px
	v.AddOptional(b, 50, 10) // needs 70
	v.AddOptional(c, 10, 0)  // would fit anywhere, but follows b
	v.AddFlex(rest, 1)
	for _, tc := range []struct {
		h          int
		a, b, c    bool
		restHeight int
	}{
		{200, true, true, true, 130},
		{70, true, true, true, 0},
		{69, true, false, false, 59},
		{20, true, false, false, 10},
		{19, false, false, false, 19},
	} {
		v.SetBounds(Rect{W: 50, H: tc.h})
		if (a.Bounds().H > 0) != tc.a || (b.Bounds().H > 0) != tc.b || (c.Bounds().H > 0) != tc.c {
			t.Errorf("h=%d: a %v b %v c %v", tc.h, a.Bounds(), b.Bounds(), c.Bounds())
		}
	}
}

// An HBox sheds on its width the same way.
func TestHBoxOptionalLeavesAsTheRowNarrows(t *testing.T) {
	icon, rest := NewLabel("i"), NewLabel("rest")
	h := NewHBox()
	h.Spacing = 2
	h.AddOptional(icon, 16, 30)
	h.AddFlex(rest, 1)
	h.SetBounds(Rect{W: 48, H: 10})
	if icon.Bounds().W != 16 || rest.Bounds().X != 18 || rest.Bounds().W != 30 {
		t.Errorf("room: icon %+v rest %+v", icon.Bounds(), rest.Bounds())
	}
	h.SetBounds(Rect{W: 47, H: 10})
	if icon.Bounds() != (Rect{}) || rest.Bounds().W != 47 {
		t.Errorf("short: icon %+v rest %+v", icon.Bounds(), rest.Bounds())
	}
}

// A box with no optional child lays out exactly as before.
func TestBoxWithoutOptionalIsUnchanged(t *testing.T) {
	kids := []boxChild{{w: NewLabel("a"), size: 10}, {w: NewLabel("b"), flex: 1}}
	if got := boxShed(kids, 100, 4); len(got) != 2 || &got[0] != &kids[0] {
		t.Errorf("boxShed changed a box without optional children: %v", got)
	}
}
