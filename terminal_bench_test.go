// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"testing"
)

// fullTerminal is an 80 × 24 Terminal with every cell printed, in the eight
// standard colours by line, and its bounds.
func fullTerminal(tb testing.TB) (*Terminal, Rect) {
	tb.Helper()
	t := NewTerminal(80, 24, 0)
	r := Rect{W: 80 * GlyphAdvance(), H: 24 * GlyphHeight()}
	t.SetBounds(r)
	var b strings.Builder
	for row := range 24 {
		if row > 0 {
			b.WriteString("\r\n")
		}
		fmt.Fprintf(&b, "\x1b[3%dm", row%8)
		for col := range 80 {
			b.WriteByte(byte('!' + (row*80+col)%94))
		}
	}
	if _, err := t.Write([]byte(b.String())); err != nil {
		tb.Fatal(err)
	}
	return t, r
}

// A full ASCII redraw allocates nothing, and a cell outside ASCII still draws
// its own rune.
func TestTerminalDrawDoesNotAllocatePerCell(t *testing.T) {
	term, r := fullTerminal(t)
	buf := makeSurface(r.W, r.H)
	p := newP(buf, r.W)
	theme := DefaultLight()
	if n := testing.AllocsPerRun(20, func() { term.Draw(p, theme) }); n != 0 {
		t.Errorf("a full 80 × 24 redraw allocates %v times, want 0", n)
	}
	for _, c := range []rune{'A', '~', 'é', '─', -1} {
		want := string(c)
		if got := runeText(c); got != want {
			t.Errorf("runeText(%q) = %q, want %q", c, got, want)
		}
	}
}

// BenchmarkTerminalDraw is one full redraw of an 80 × 24 terminal, the
// frame a terminal paints on output (go-widgets/toolkit#548).
func BenchmarkTerminalDraw(b *testing.B) {
	t, r := fullTerminal(b)
	buf := makeSurface(r.W, r.H)
	p := newP(buf, r.W)
	theme := DefaultLight()
	b.ReportAllocs()
	for b.Loop() {
		t.Draw(p, theme)
	}
}
