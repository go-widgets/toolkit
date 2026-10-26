// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package vt

import "testing"

// FuzzWrite feeds arbitrary streams, split at an arbitrary point, and checks
// the model's invariants: the cursor stays on the grid, the grid keeps its
// shape, the scrollback stays bounded, and Text never panics.
func FuzzWrite(f *testing.F) {
	for _, s := range []string{
		"hello\r\nworld", "\x1b[31;1mred\x1b[0m", "\x1b[2;3r\x1b[L\x1b[M\x1b[S\x1b[T",
		"\x1b]52;c;aGk=\x07x", "\x1b[38:2::1:2:3m\x1b[48;5;9m", "\x1b[999;999H\x1b[99@\x1b[99P",
		"\xe2\x82\xac\xff\xc2\x9b", "\x1b[?7l\x1b[?25l", "\x1b7\x1b8\x1b[s\x1b[u\x1bM\x1bD\x1bE\x1bc",
	} {
		f.Add([]byte(s), uint8(3), uint8(7), uint8(4))
	}
	f.Fuzz(func(t *testing.T, in []byte, split, cols, rows uint8) {
		s := New(int(cols%40)+1, int(rows%20)+1, 16)
		cut := int(split) % (len(in) + 1)
		s.Write(in[:cut])
		s.Resize(int(rows%30)+1, int(cols%15)+1)
		s.Write(in[cut:])
		c, r := s.Size()
		x, y, _ := s.Cursor()
		if x < 0 || x >= c || y < 0 || y >= r {
			t.Fatalf("cursor %d,%d off a %dx%d grid", x, y, c, r)
		}
		for i := 0; i < r; i++ {
			if len(s.Row(i)) != c {
				t.Fatalf("row %d has %d cells, want %d", i, len(s.Row(i)), c)
			}
		}
		if s.ScrollbackLen() > 16 {
			t.Fatalf("scrollback %d over its bound", s.ScrollbackLen())
		}
		_ = s.Text(Pos{0, 0}, Pos{s.Lines(), c})
	})
}
