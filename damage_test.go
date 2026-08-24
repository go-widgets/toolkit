// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

// diffPx flips pixel (x,y) of a w-wide RGBA buffer to a non-zero value, so a diff
// against an all-zero buffer sees exactly that pixel change.
func diffPx(buf []byte, w, x, y int) {
	o := (y*w + x) * 4
	buf[o], buf[o+1], buf[o+2], buf[o+3] = 1, 2, 3, 4
}

func TestDiffRects(t *testing.T) {
	const w, h = 4, 4
	blank := func() []byte { return make([]byte, w*h*4) }

	t.Run("identical is no damage", func(t *testing.T) {
		if got := DiffRects(blank(), blank(), w, h); len(got) != 0 {
			t.Fatalf("damage = %v, want none", got)
		}
	})

	t.Run("one pixel", func(t *testing.T) {
		cur := blank()
		diffPx(cur, w, 1, 1)
		want := []Rect{{X: 1, Y: 1, W: 1, H: 1}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("contiguous block is one rect", func(t *testing.T) {
		cur := blank()
		for y := 1; y <= 2; y++ {
			for x := 1; x <= 2; x++ {
				diffPx(cur, w, x, y)
			}
		}
		want := []Rect{{X: 1, Y: 1, W: 2, H: 2}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("a band takes the widest x-span of its rows", func(t *testing.T) {
		cur := blank()
		diffPx(cur, w, 1, 1) // row 1: opens the band at x=1
		diffPx(cur, w, 0, 2) // row 2: minX shrinks to 0
		diffPx(cur, w, 3, 2) // row 2: maxX grows to 3
		want := []Rect{{X: 0, Y: 1, W: 4, H: 2}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("vertically separated changes are separate rects", func(t *testing.T) {
		cur := blank()
		diffPx(cur, w, 1, 0) // top band
		diffPx(cur, w, 1, 3) // bottom band, rows 1-2 unchanged between
		want := []Rect{{X: 1, Y: 0, W: 1, H: 1}, {X: 1, Y: 3, W: 1, H: 1}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("same-row changes union their x-span", func(t *testing.T) {
		cur := blank()
		diffPx(cur, w, 0, 1)
		diffPx(cur, w, 3, 1) // gap at x=1,2 is over-covered (safe)
		want := []Rect{{X: 0, Y: 1, W: 4, H: 1}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("edges: last row and last column", func(t *testing.T) {
		cur := blank()
		diffPx(cur, w, w-1, h-1)
		want := []Rect{{X: w - 1, Y: h - 1, W: 1, H: 1}}
		if got := DiffRects(cur, blank(), w, h); !reflect.DeepEqual(got, want) {
			t.Fatalf("damage = %v, want %v", got, want)
		}
	})

	t.Run("unusable buffers are whole-surface (nil)", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			cur, prev []byte
			w, h      int
		}{
			{"short cur", make([]byte, 4), blank(), w, h},
			{"short prev", blank(), make([]byte, 4), w, h},
			{"zero width", blank(), blank(), 0, h},
			{"zero height", blank(), blank(), w, 0},
		} {
			if got := DiffRects(tc.cur, tc.prev, tc.w, tc.h); got != nil {
				t.Fatalf("%s: damage = %v, want nil", tc.name, got)
			}
		}
	})
}
