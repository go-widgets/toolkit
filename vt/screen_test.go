// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package vt

import (
	"strings"
	"testing"
)

// row renders visible row y as text, blanks as '.'.
func row(s *Screen, y int) string {
	var b strings.Builder
	for _, c := range s.Row(y) {
		if c.Rune == 0 {
			b.WriteByte('.')
		} else {
			b.WriteRune(c.Rune)
		}
	}
	return b.String()
}

func screen(t *testing.T, cols, rows int, in string) *Screen {
	t.Helper()
	s := New(cols, rows, 10)
	n, err := s.WriteString(in)
	if err != nil || n != len(in) {
		t.Fatalf("Write = %d, %v", n, err)
	}
	return s
}

func wantRows(t *testing.T, s *Screen, want ...string) {
	t.Helper()
	for y, w := range want {
		if got := row(s, y); got != w {
			t.Errorf("row %d = %q, want %q", y, got, w)
		}
	}
}

func wantCursor(t *testing.T, s *Screen, x, y int) {
	t.Helper()
	if cx, cy, _ := s.Cursor(); cx != x || cy != y {
		t.Errorf("cursor = %d,%d, want %d,%d", cx, cy, x, y)
	}
}

func TestColor(t *testing.T) {
	if _, ok := Default.Index(); ok {
		t.Error("Default is indexed")
	}
	if n, ok := Indexed(200).Index(); !ok || n != 200 {
		t.Errorf("Indexed(200).Index() = %d, %v", n, ok)
	}
	if r, g, b, ok := RGB(1, 2, 3).RGB(); !ok || r != 1 || g != 2 || b != 3 {
		t.Errorf("RGB = %d %d %d %v", r, g, b, ok)
	}
	if _, _, _, ok := Indexed(1).RGB(); ok {
		t.Error("Indexed reads as RGB")
	}
	if !(Pos{1, 5}).Before(Pos{2, 0}) || !(Pos{1, 1}).Before(Pos{1, 2}) || (Pos{1, 2}).Before(Pos{1, 2}) {
		t.Error("Pos.Before")
	}
}

func TestNewClamps(t *testing.T) {
	s := New(0, 99999, -1)
	if c, r := s.Size(); c != 1 || r != MaxRows {
		t.Errorf("Size = %d,%d", c, r)
	}
	s.WriteString("a\nb\nc")
	if s.ScrollbackLen() != 0 {
		t.Error("scrollback kept with a zero ring")
	}
	if s.Row(-1) != nil || s.Row(MaxRows) != nil || s.Line(-1) != nil {
		t.Error("out-of-range rows")
	}
}

func TestPrintWrapAndControls(t *testing.T) {
	s := screen(t, 4, 3, "abcdef\r\nxy\bZ\tq")
	wantRows(t, s, "abcd", "ef..", "xZ.q")
	wantCursor(t, s, 3, 2)
	// pending wrap: the cursor sits on the last column until the next rune.
	s = screen(t, 3, 2, "abc")
	wantCursor(t, s, 2, 0)
	s.WriteString("\b")
	wantCursor(t, s, 1, 0)
	s.WriteString("\x7f\x01X") // DEL and other C0 do nothing
	wantRows(t, s, "aXc")
	s.WriteString("\b\b\b\b")
	wantCursor(t, s, 0, 0)
}

func TestNewlineMode(t *testing.T) {
	s := screen(t, 4, 2, "ab\ncd")
	wantRows(t, s, "ab..", "..cd")
	s = New(4, 2, 0)
	s.SetNewlineMode(true)
	s.WriteString("ab\ncd\v\fe")
	wantRows(t, s, "....", "e...") // VT and FF are line feeds too
}

func TestScrollback(t *testing.T) {
	s := New(3, 2, 2)
	s.WriteString("1\r\n2\r\n3\r\n4\r\n5")
	if s.ScrollbackLen() != 2 || s.Lines() != 4 {
		t.Fatalf("ScrollbackLen = %d Lines = %d", s.ScrollbackLen(), s.Lines())
	}
	if got := s.Text(Pos{0, 0}, Pos{3, 2}); got != "2\n3\n4\n5" {
		t.Errorf("Text = %q", got)
	}
	if got := s.Text(Pos{3, 2}, Pos{-5, 0}); got != "2\n3\n4\n5" {
		t.Errorf("reversed Text = %q", got)
	}
	if got := s.Text(Pos{1, 1}, Pos{1, 9}); got != "" {
		t.Errorf("Text past the end = %q", got)
	}
	s.ClearScrollback()
	if s.ScrollbackLen() != 0 || s.Line(0)[0].Rune != '4' {
		t.Error("ClearScrollback")
	}
	s.WriteString("\x1b[3J")
	if s.ScrollbackLen() != 0 {
		t.Error("ED 3")
	}
}

func TestResize(t *testing.T) {
	s := New(4, 3, 5)
	s.WriteString("aaaa\r\nbb\r\ncc\x1b[2;3r")
	v := s.Version()
	s.Resize(4, 3) // same size: nothing
	if s.Version() != v {
		t.Error("same-size Resize changed the model")
	}
	s.WriteString("\x1b[3;4H\x1b7") // saved at the bottom-right
	s.Resize(2, 2)
	wantRows(t, s, "bb", "cc")
	if s.ScrollbackLen() != 1 || s.Line(0)[3].Rune != 'a' {
		t.Error("the top row did not move to the scrollback")
	}
	wantCursor(t, s, 1, 1)
	s.WriteString("\x1b8")
	wantCursor(t, s, 1, 1)
	s.Resize(3, 3)
	wantRows(t, s, "bb.", "cc.", "...")
	s.WriteString("\x1b[3;1Hz\r\nw") // the region was reset: the whole grid scrolls
	wantRows(t, s, "cc.", "z..", "w..")
}

func TestCursorMoves(t *testing.T) {
	s := screen(t, 10, 6, "\x1b[3;4H")
	wantCursor(t, s, 3, 2)
	for _, c := range []struct {
		seq  string
		x, y int
	}{
		{"\x1b[A", 3, 1}, {"\x1b[9A", 3, 0}, {"\x1b[2B", 3, 2}, {"\x1b[99B", 3, 5},
		{"\x1b[2C", 5, 5}, {"\x1b[99C", 9, 5}, {"\x1b[3D", 6, 5}, {"\x1b[99D", 0, 5},
		{"\x1b[5G", 4, 5}, {"\x1b[99`", 9, 5}, {"\x1b[2F", 0, 3}, {"\x1b[E", 0, 4},
		{"\x1b[99F", 0, 0}, {"\x1b[99E", 0, 5}, {"\x1b[2d", 0, 1}, {"\x1b[;7f", 6, 0},
		{"\x1b[H", 0, 0}, {"\x1b[99;99H", 9, 5}, {"\x1b[0;0H", 0, 0},
		{"\x1b[4;5H\x1b[s\x1b[H\x1b[u", 4, 3},
		{"\x1bD", 4, 4}, {"\x1bM\x1bM", 4, 2}, {"\x1bE", 0, 3},
	} {
		s.WriteString(c.seq)
		wantCursor(t, s, c.x, c.y)
	}
}

func TestScrollRegion(t *testing.T) {
	s := screen(t, 2, 4, "a\r\nb\r\nc\r\nd\x1b[2;3r")
	wantCursor(t, s, 0, 0)
	s.WriteString("\x1b[3;1H\nX") // LF at the region's bottom scrolls the region
	wantRows(t, s, "a.", "c.", "X.", "d.")
	if s.ScrollbackLen() != 0 {
		t.Error("a region below the top fed the scrollback")
	}
	s.WriteString("\x1b[2;1H\x1bMY") // RI at the region's top scrolls down
	wantRows(t, s, "a.", "Y.", "c.", "d.")
	s.WriteString("\x1b[2;1H\x1b[9A") // CUU stops at the region's top
	wantCursor(t, s, 0, 1)
	s.WriteString("\x1b[9B")
	wantCursor(t, s, 0, 2)
	s.WriteString("\x1b[4;1H\x1b[9A\x1b[1;1H\x1b[9B") // outside the region
	wantCursor(t, s, 0, 2)
	s.WriteString("\x1b[1;1H\x1b[A\x1b[4;1H\x1b[B\n")
	wantCursor(t, s, 0, 3)
	s.WriteString("\x1b[3;2r\x1b[0;9r") // an inverted region is refused; 9 clamps
	wantCursor(t, s, 0, 0)
	s.WriteString("\x1b[1;1H\x1b[S")
	wantRows(t, s, "Y.", "c.", "d.", "..")
	s.WriteString("\x1b[2T")
	wantRows(t, s, "..", "..", "Y.", "c.")
}

func TestInsertDeleteLines(t *testing.T) {
	s := screen(t, 2, 4, "a\r\nb\r\nc\r\nd\x1b[2;1H\x1b[L")
	wantRows(t, s, "a.", "..", "b.", "c.")
	s.WriteString("\x1b[M\x1b[M")
	wantRows(t, s, "a.", "c.", "..", "..")
	if s.ScrollbackLen() != 0 {
		t.Error("DL fed the scrollback")
	}
	s.WriteString("\x1b[2;3r\x1b[4;1H\x1b[L") // outside the region: nothing
	wantRows(t, s, "a.", "c.", "..", "..")
}

func TestInsertDeleteEraseChars(t *testing.T) {
	s := screen(t, 6, 1, "abcdef\x1b[1;3H\x1b[2@")
	wantRows(t, s, "ab..cd")
	s.WriteString("\x1b[99@")
	wantRows(t, s, "ab....")
	s = screen(t, 6, 1, "abcdef\x1b[1;2H\x1b[2P")
	wantRows(t, s, "adef..")
	s.WriteString("\x1b[99P")
	wantRows(t, s, "a.....")
	s = screen(t, 6, 1, "abcdef\x1b[1;2H\x1b[2X")
	wantRows(t, s, "a..def")
	s.WriteString("\x1b[99X")
	wantRows(t, s, "a.....")
}

func TestErase(t *testing.T) {
	fill := "abc\r\ndef\r\nghi\x1b[2;2H"
	for _, c := range []struct {
		seq  string
		want []string
	}{
		{"\x1b[J", []string{"abc", "d..", "..."}},
		{"\x1b[1J", []string{"...", "..f", "ghi"}},
		{"\x1b[2J", []string{"...", "...", "..."}},
		{"\x1b[K", []string{"abc", "d..", "ghi"}},
		{"\x1b[1K", []string{"abc", "..f", "ghi"}},
		{"\x1b[2K", []string{"abc", "...", "ghi"}},
		{"\x1b[9J\x1b[9K", []string{"abc", "def", "ghi"}},
	} {
		s := screen(t, 3, 3, fill+c.seq)
		wantRows(t, s, c.want...)
	}
	// Erase uses the current background (BCE).
	s := screen(t, 2, 1, "\x1b[44m\x1b[2K")
	if s.Row(0)[1].BG != Indexed(4) {
		t.Error("erase ignored the background")
	}
}

func TestModes(t *testing.T) {
	s := screen(t, 3, 2, "\x1b[?25l")
	if _, _, vis := s.Cursor(); vis {
		t.Error("DECTCEM l")
	}
	s.WriteString("\x1b[?7;25h")
	if _, _, vis := s.Cursor(); !vis {
		t.Error("DECTCEM h")
	}
	s.WriteString("\x1b[?7labcdef")
	wantRows(t, s, "abf", "...")
	s.WriteString("\x1b[>1h\x1b[?1049h\x1b[4h") // other modes: ignored
	s.WriteString("\x1b[?7h\r\nxyzw")           // wraps again, scrolling at the bottom
	wantRows(t, s, "xyz", "w..")
	s.WriteString("\x1bc") // RIS
	wantRows(t, s, "...", "...")
	wantCursor(t, s, 0, 0)
}

func TestSGR(t *testing.T) {
	cell := func(seq string) Cell {
		t.Helper()
		s := screen(t, 2, 1, seq+"x")
		return s.Row(0)[0]
	}
	for _, c := range []struct {
		seq  string
		want Cell
	}{
		{"\x1b[1;2;3;4;7;8;9m", Cell{Rune: 'x', Attr: Bold | Faint | Italic | Underline | Reverse | Conceal | Strike}},
		{"\x1b[1;2;3;4;7;8;9;22;23;24;27;28;29m", Cell{Rune: 'x'}},
		{"\x1b[21m", Cell{Rune: 'x', Attr: Underline}},
		{"\x1b[4:3m", Cell{Rune: 'x', Attr: Underline}},
		{"\x1b[4m\x1b[4:0m", Cell{Rune: 'x'}},
		{"\x1b[5;6;25m", Cell{Rune: 'x'}},
		{"\x1b[31;42m", Cell{Rune: 'x', FG: Indexed(1), BG: Indexed(2)}},
		{"\x1b[91;102m", Cell{Rune: 'x', FG: Indexed(9), BG: Indexed(10)}},
		{"\x1b[31;42;39;49m", Cell{Rune: 'x'}},
		{"\x1b[1;31m\x1b[m", Cell{Rune: 'x'}},
		{"\x1b[1;31m\x1b[0m", Cell{Rune: 'x'}},
		{"\x1b[38;5;200;48;5;999m", Cell{Rune: 'x', FG: Indexed(200), BG: Indexed(255)}},
		{"\x1b[38;2;1;2;3;48;2;4;5;300m", Cell{Rune: 'x', FG: RGB(1, 2, 3), BG: RGB(4, 5, 255)}},
		{"\x1b[38:5:7m", Cell{Rune: 'x', FG: Indexed(7)}},
		{"\x1b[38:2:1:2:3;1m", Cell{Rune: 'x', FG: RGB(1, 2, 3), Attr: Bold}},
		{"\x1b[48:2::1:2:3;1m", Cell{Rune: 'x', BG: RGB(1, 2, 3), Attr: Bold}},
		{"\x1b[38;5m", Cell{Rune: 'x'}},                  // malformed: dropped
		{"\x1b[1;38;9;31m", Cell{Rune: 'x', Attr: Bold}}, // the rest is unreadable
		{"\x1b[38:5;1m", Cell{Rune: 'x'}},
	} {
		if got := cell(c.seq); got != c.want {
			t.Errorf("%q: %+v, want %+v", c.seq, got, c.want)
		}
	}
}

func TestStringsAreSwallowed(t *testing.T) {
	for _, seq := range []string{
		"\x1b]52;c;aGVsbG8=\x07",                  // clipboard write
		"\x1b]8;;javascript:alert(1)\x1b\\",       // hyperlink
		"\x1b]0;title\x07",                        // title
		"\x1bPq#0;2;0;0;0\x1b\\",                  // DCS (sixel)
		"\x1b_apc\x1b\\\x1b^pm\x07\x1bXsos\x1b\\", // APC PM SOS
	} {
		s := screen(t, 8, 1, "a"+seq+"b")
		wantRows(t, s, "ab......")
	}
	// ESC inside a string aborts it and starts a new sequence.
	s := screen(t, 8, 1, "\x1b]0;t\x1b[31mx")
	if s.Row(0)[0] != (Cell{Rune: 'x', FG: Indexed(1)}) {
		t.Errorf("ESC in a string: %+v", s.Row(0)[0])
	}
	// An unterminated string gives up after maxString bytes.
	s = screen(t, 4, 1, "\x1b]"+strings.Repeat("z", maxString)+"ab")
	wantRows(t, s, "b...")
}

func TestNoReplies(t *testing.T) {
	// Queries a terminal would answer; the model has nowhere to answer to,
	// and none of them may print or move anything.
	s := screen(t, 4, 1, "a\x1b[6n\x1b[c\x1b[>c\x1b[18t\x1b[5n\x1b[?6n\x1b[>0q\x1b[1$pb")
	wantRows(t, s, "ab..")
	wantCursor(t, s, 2, 0)
}

func TestParserEdges(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a\x1b[3\x18b", "ab...."},           // CAN aborts a CSI
		{"a\x1b]0;x\x1ab", "ab...."},         // SUB aborts a string
		{"a\x1b(Bb\x1b)0c\x1b#8d", "abcd.."}, // designations dropped
		{"a\x1b( \x01Bb", "ab...."},          // intermediates continue; C0 runs
		{"a\x1b[1\rmb", "b....."},            // C0 inside a CSI executes
		{"ab\x1b\r[mc", "cb...."},            // C0 inside an escape executes
		{"a\x1b[?1;?2hb", "ab...."},          // a second private marker
		{"a\x1b[1?hb", "ab...."},             // a marker after a parameter
		{"a\x1b[1\x80mb", "ab...."},          // an invalid byte in a CSI
		{"a\x1b[1;\x80;;3mb", "ab...."},      // ignored to its final byte
		{"a\x1b[ qb", "ab...."},              // an intermediate: ignored
		{"a\x1bZb\x1b\\c", "abc..."},         // unknown escapes
		{"a\x1b[;5Hb", "a...b."},             // an empty first parameter
		{"\x1b[31;1:3;4mx", "x....."},        // subparameters
		{"\x1b[" + strings.Repeat("1;", maxParams+3) + "mz", "z....."},
		{"\x1b[" + strings.Repeat("9", 20) + "Cy", ".....y"},
		{"\x1b[" + strings.Repeat(";", maxParams+3) + "1Ck", "k....."},
	} {
		s := screen(t, 6, 1, c.in)
		wantRows(t, s, c.want)
	}
}

func TestUTF8(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"é€😀", "é€😀..."},
		{"a\xffb", "a�b..."},
		{"a\x80b", "a�b..."},
		{"a\xe2\x82b", "a�b..."},         // truncated
		{"a\xe2\x82\x1b[31mb", "a�b..."}, // truncated by an escape
		{"a\xc2\x9bb", "ab...."},         // C1 CSI as UTF-8: dropped
		{"é‮‍x", "ex...."},              // marks and format characters
		{"\xe0\x80z", "�z...."},          // overlong: invalid
	} {
		s := screen(t, 6, 1, c.in)
		wantRows(t, s, c.want)
	}
	// A rune split across two writes.
	s := New(4, 1, 0)
	s.Write([]byte{0xe2, 0x82})
	s.Write([]byte{0xac})
	wantRows(t, s, "€...")
}

func TestVersion(t *testing.T) {
	s := New(2, 2, 0)
	v := s.Version()
	s.WriteString("x")
	if s.Version() == v {
		t.Error("Write left Version")
	}
	v = s.Version()
	s.Resize(3, 3)
	if s.Version() == v {
		t.Error("Resize left Version")
	}
}

func TestScrollbackRing(t *testing.T) {
	s := New(1, 1, 3)
	for _, c := range "abcdefg" {
		s.WriteString(string(c) + "\n")
	}
	if s.ScrollbackLen() != 3 {
		t.Fatalf("ScrollbackLen = %d", s.ScrollbackLen())
	}
	if got := s.Text(Pos{0, 0}, Pos{2, 0}); got != "e\nf\ng" {
		t.Errorf("ring = %q", got)
	}
}
