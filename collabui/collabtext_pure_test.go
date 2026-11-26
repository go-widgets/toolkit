// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import "testing"

// These two tests need no server, so they are built on every platform, js/wasm
// included (collabtext_test.go, which starts an in-process server, is not).

func TestRuneOffsetAndLineCol(t *testing.T) {
	const buf = "ab\ncdé\n"
	// buf lines: "ab" (2), "cdé" (3 runes), "" — total runes 2+1+3+1+0 = 7.
	cases := []struct {
		line, col, off int
	}{
		{0, 0, 0},
		{0, 2, 2}, // end of line 0
		{1, 0, 3}, // start of line 1
		{1, 3, 6}, // end of line 1 (after the é)
		{2, 0, 7}, // start of the trailing empty line
	}
	for _, c := range cases {
		if got := runeOffset(buf, c.line, c.col); got != c.off {
			t.Errorf("runeOffset(%d,%d) = %d, want %d", c.line, c.col, got, c.off)
		}
		gl, gc := lineCol(buf, c.off)
		if gl != c.line || gc != c.col {
			t.Errorf("lineCol(%d) = (%d,%d), want (%d,%d)", c.off, gl, gc, c.line, c.col)
		}
	}

	// Bounds and clamping.
	if got := runeOffset(buf, -1, 5); got != 0 {
		t.Errorf("runeOffset(negative line) = %d, want 0", got)
	}
	if got := runeOffset(buf, 99, 0); got != 7 {
		t.Errorf("runeOffset(line past end) = %d, want 7 (buffer length)", got)
	}
	if got := runeOffset(buf, 0, -1); got != 0 {
		t.Errorf("runeOffset(negative col) = %d, want 0", got)
	}
	if got := runeOffset(buf, 0, 99); got != 2 {
		t.Errorf("runeOffset(col past line end) = %d, want 2", got)
	}
	if l, c := lineCol(buf, -3); l != 0 || c != 0 {
		t.Errorf("lineCol(negative) = (%d,%d), want (0,0)", l, c)
	}
	if l, c := lineCol(buf, 999); l != 2 || c != 0 {
		t.Errorf("lineCol(past end) = (%d,%d), want (2,0)", l, c)
	}
}

// TestPublishCursorSkipsWhileApplyingRemote deterministically covers the
// applyingRemote guard in publishCursor: with applyingRemote set, publishCursor
// must return before touching the (nil here) editor/client — a caret notification
// caused by our own remote application is never echoed back as presence.
func TestPublishCursorSkipsWhileApplyingRemote(t *testing.T) {
	c := &CollabText{applyingRemote: true}
	c.publishCursor() // returns at the guard; a panic would mean it fell through
}
