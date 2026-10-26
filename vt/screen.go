// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package vt is a terminal screen model: a byte stream in the xterm control
// sequence grammar goes in, a grid of cells, a cursor and a bounded
// scrollback come out. It is pure Go with no dependency, so the model a
// terminal widget renders is natively testable.
//
// # Safe by construction
//
// A Screen has no output channel: it cannot answer a device-status or
// device-attributes query, report a window size, echo a title, or say
// anything else that would reach a shell's input as if the user had typed
// it. Every string sequence (OSC, DCS, SOS, PM, APC) is consumed and
// discarded without being buffered: no OSC 52 clipboard write, no OSC 8
// hyperlink, no title. What it obeys is layout and colour only: the C0
// controls BS HT LF VT FF CR, the ESC functions IND NEL RI DECSC DECRC RIS,
// and the CSI functions CUU CUD CUF CUB CNL CPL CHA HPA CUP HVP VPA ED EL IL
// DL ICH DCH ECH SU SD DECSTBM SCOSC SCORC SGR, plus the private modes
// DECAWM (7) and DECTCEM (25). C1 controls are never obeyed: written as
// UTF-8 (U+0080-U+009F) they are dropped, and as raw 8-bit bytes they are
// invalid UTF-8, shown as U+FFFD. Bidi and other format characters
// (Unicode Cf, e.g. U+202E) are dropped too. CAN and SUB abort a sequence.
//
// Every quantity is bounded: the grid size, the scrollback ring, a
// sequence's parameter count and values, and the length of a string
// sequence the parser swallows before it gives up and returns to text.
//
// Reference: XTerm Control Sequences (invisible-island.net/xterm/ctlseqs).
//
// Limitations: every rune occupies one cell (no East Asian wide cells),
// combining marks and format characters are dropped, there is no alternate
// screen and no reflow on resize.
package vt

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Bounds on what a stream can make the model hold.
const (
	MaxCols = 1024
	MaxRows = 512
	// MaxScrollback bounds the ring New accepts.
	MaxScrollback = 100000

	maxParams = 32
	maxValue  = 9999
	maxString = 4096
)

type state uint8

const (
	ground state = iota
	escape
	escInter // ESC + intermediate: waiting for the final byte
	csiParam
	csiIgnore
	str
	strEsc
)

type cursor struct {
	x, y     int
	pen      Cell
	wrapNext bool
}

// Screen is the terminal model. The zero value is not usable; call New.
type Screen struct {
	cols, rows int
	grid       [][]Cell
	cur        cursor
	saved      cursor
	top, bot   int // scroll region, inclusive
	autowrap   bool
	hidden     bool
	lfCR       bool

	sb            [][]Cell // ring
	sbHead, sbLen int
	sbMax         int
	version       uint64
	st            state
	params        []int
	colon         []bool
	priv          byte
	inter         bool
	strLen        int
	utf           [utf8.UTFMax]byte
	utfN          int
}

// New returns a cols x rows screen keeping up to scrollback lines of
// history. Sizes are clamped to [1, MaxCols] x [1, MaxRows] and the
// scrollback to [0, MaxScrollback].
func New(cols, rows, scrollback int) *Screen {
	s := &Screen{sbMax: clamp(scrollback, 0, MaxScrollback)}
	s.params = make([]int, 0, maxParams)
	s.colon = make([]bool, 0, maxParams)
	s.cols, s.rows = clamp(cols, 1, MaxCols), clamp(rows, 1, MaxRows)
	s.grid = make([][]Cell, s.rows)
	for i := range s.grid {
		s.grid[i] = make([]Cell, s.cols)
	}
	s.reset()
	return s
}

// SetNewlineMode makes LF also return the carriage (what a terminal with
// LNM set, or xterm.js's convertEol, does), for output written with bare
// "\n" line ends.
func (s *Screen) SetNewlineMode(on bool) { s.lfCR = on }

func (s *Screen) reset() {
	s.cur = cursor{}
	s.saved = cursor{}
	s.top, s.bot = 0, s.rows-1
	s.autowrap = true
	s.hidden = false
	for _, l := range s.grid {
		clear(l)
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Size returns the grid's columns and rows.
func (s *Screen) Size() (cols, rows int) { return s.cols, s.rows }

// Cursor returns the cursor's position on the visible grid and whether it
// is shown.
func (s *Screen) Cursor() (x, y int, visible bool) { return s.cur.x, s.cur.y, !s.hidden }

// Version changes every time Write or Resize changes the model, so a view
// can skip a frame when nothing moved.
func (s *Screen) Version() uint64 { return s.version }

// Row returns visible row y (nil when out of range). The slice is the
// model's own: read it, do not keep or modify it.
func (s *Screen) Row(y int) []Cell {
	if y < 0 || y >= s.rows {
		return nil
	}
	return s.grid[y]
}

// ScrollbackLen is the number of history lines above the visible grid.
func (s *Screen) ScrollbackLen() int { return s.sbLen }

// Lines is the number of addressable lines: scrollback plus visible rows.
func (s *Screen) Lines() int { return s.sbLen + s.rows }

// Line returns line i of the whole history (see Pos), nil when out of
// range. Scrollback lines keep the width they had when they scrolled off.
func (s *Screen) Line(i int) []Cell {
	if i < 0 {
		return nil
	}
	if i < s.sbLen {
		return s.sb[(s.sbHead+i)%s.sbMax]
	}
	return s.Row(i - s.sbLen)
}

// Text returns the text between from and to (inclusive, in either order):
// blanks become spaces, trailing blanks of each line are dropped, lines are
// joined with "\n". It is what a selection copies.
func (s *Screen) Text(from, to Pos) string {
	if to.Before(from) {
		from, to = to, from
	}
	var b strings.Builder
	for i := max(from.Line, 0); i <= to.Line && i < s.Lines(); i++ {
		l := s.Line(i)
		lo, hi := 0, len(l)-1
		if i == from.Line {
			lo = from.Col
		}
		if i == to.Line {
			hi = min(to.Col, hi)
		}
		var line []rune
		for c := max(lo, 0); c <= hi; c++ {
			r := l[c].Rune
			if r == 0 {
				r = ' '
			}
			line = append(line, r)
		}
		if i > max(from.Line, 0) {
			b.WriteByte('\n')
		}
		b.WriteString(strings.TrimRight(string(line), " "))
	}
	return b.String()
}

// ClearScrollback drops the history.
func (s *Screen) ClearScrollback() {
	s.sbHead, s.sbLen = 0, 0
	s.sb = nil
	s.version++
}

// Resize reshapes the grid, keeping the top-left cells that still fit. When
// the grid loses rows below the cursor's, the top rows move into the
// scrollback so the cursor's line stays visible. The scroll region resets
// to the whole grid.
func (s *Screen) Resize(cols, rows int) {
	cols, rows = clamp(cols, 1, MaxCols), clamp(rows, 1, MaxRows)
	if cols == s.cols && rows == s.rows {
		return
	}
	shift := max(s.cur.y-rows+1, 0)
	for i := 0; i < shift; i++ {
		s.pushScrollback(s.grid[i])
	}
	grid := make([][]Cell, rows)
	for i := range grid {
		grid[i] = make([]Cell, cols)
		if j := i + shift; j < s.rows {
			copy(grid[i], s.grid[j])
		}
	}
	s.grid, s.cols, s.rows = grid, cols, rows
	s.cur.y -= shift
	s.cur.x = min(s.cur.x, cols-1)
	s.cur.wrapNext = false
	s.saved.x, s.saved.y = min(s.saved.x, cols-1), min(s.saved.y, rows-1)
	s.top, s.bot = 0, rows-1
	s.version++
}

func (s *Screen) pushScrollback(l []Cell) {
	if s.sbMax == 0 {
		return
	}
	line := append([]Cell(nil), l...)
	if s.sbLen < s.sbMax {
		s.sb = append(s.sb, line)
		s.sbLen++
		return
	}
	s.sb[s.sbHead] = line
	s.sbHead = (s.sbHead + 1) % s.sbMax
}

// Write feeds the stream. It never fails and consumes all of p: a partial
// UTF-8 sequence or control sequence carries over to the next call.
func (s *Screen) Write(p []byte) (int, error) {
	for _, c := range p {
		s.feed(c)
	}
	s.version++
	return len(p), nil
}

// WriteString is Write for a string.
func (s *Screen) WriteString(str string) (int, error) { return s.Write([]byte(str)) }

func (s *Screen) feed(c byte) {
	if s.utfN > 0 {
		if c&0xc0 == 0x80 {
			s.utf[s.utfN] = c
			s.utfN++
			if utf8.FullRune(s.utf[:s.utfN]) {
				r, _ := utf8.DecodeRune(s.utf[:s.utfN])
				s.utfN = 0
				s.rune(r)
			}
			return
		}
		s.utfN = 0
		s.rune(utf8.RuneError) // a truncated sequence
	}
	switch {
	case c == 0x18 || c == 0x1a: // CAN, SUB: abort any sequence
		s.st = ground
		return
	case c == 0x1b:
		if s.st == str {
			s.st = strEsc
			return
		}
		s.st = escape
		s.inter = false
		return
	}
	switch s.st {
	case ground:
		s.groundByte(c)
	case escape:
		s.escapeByte(c)
	case escInter:
		if c >= 0x30 && c <= 0x7e {
			s.st = ground // a charset designation or the like: dropped
		} else if c < 0x20 {
			s.control(c)
		}
	case csiParam, csiIgnore:
		s.csiByte(c)
	case str:
		s.strLen++
		if c == 0x07 || s.strLen > maxString {
			s.st = ground
		}
	case strEsc:
		if c == '\\' {
			s.st = ground
			return
		}
		s.st = escape // ESC inside a string aborts it and starts anew
		s.escapeByte(c)
	}
}

func (s *Screen) groundByte(c byte) {
	switch {
	case c < 0x20:
		s.control(c)
	case c < 0x7f:
		s.rune(rune(c))
	case c == 0x7f:
	case c >= 0xc2 && c <= 0xf4:
		s.utf[0] = c
		s.utfN = 1
	default:
		s.rune(utf8.RuneError) // a stray continuation or invalid lead byte
	}
}

// rune prints r, unless it is a control or a mark the model cannot place.
func (s *Screen) rune(r rune) {
	if r >= 0x80 && (r <= 0x9f || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf)) {
		return
	}
	if s.cur.wrapNext {
		s.cur.x = 0
		s.index()
	}
	s.grid[s.cur.y][s.cur.x] = Cell{Rune: r, FG: s.cur.pen.FG, BG: s.cur.pen.BG, Attr: s.cur.pen.Attr}
	if s.cur.x == s.cols-1 {
		s.cur.wrapNext = s.autowrap
		return
	}
	s.cur.x++
}

func (s *Screen) control(c byte) {
	switch c {
	case '\b':
		s.cur.wrapNext = false
		s.cur.x = max(s.cur.x-1, 0)
	case '\t':
		s.cur.x = min((s.cur.x/8+1)*8, s.cols-1)
	case '\n', '\v', '\f':
		if s.lfCR {
			s.cur.x = 0
		}
		s.index()
	case '\r':
		s.cur.x = 0
		s.cur.wrapNext = false
	}
}

// index moves the cursor down one line, scrolling the region at its bottom.
func (s *Screen) index() {
	s.cur.wrapNext = false
	if s.cur.y == s.bot {
		s.scrollUp(1)
		return
	}
	s.cur.y = min(s.cur.y+1, s.rows-1)
}

func (s *Screen) reverseIndex() {
	s.cur.wrapNext = false
	if s.cur.y == s.top {
		s.scrollDown(1)
		return
	}
	s.cur.y = max(s.cur.y-1, 0)
}

func (s *Screen) blank() Cell { return Cell{BG: s.cur.pen.BG} }

func (s *Screen) fill(l []Cell) {
	b := s.blank()
	for i := range l {
		l[i] = b
	}
}

// scrollUp moves the region's lines up by n; lines leaving the top of a
// region that starts at the top of the screen go to the scrollback.
func (s *Screen) scrollUp(n int) {
	n = min(n, s.bot-s.top+1)
	for i := 0; i < n; i++ {
		l := s.grid[s.top]
		if s.top == 0 {
			s.pushScrollback(l)
		}
		copy(s.grid[s.top:s.bot], s.grid[s.top+1:s.bot+1])
		s.fill(l)
		s.grid[s.bot] = l
	}
}

func (s *Screen) scrollDown(n int) {
	n = min(n, s.bot-s.top+1)
	for i := 0; i < n; i++ {
		l := s.grid[s.bot]
		copy(s.grid[s.top+1:s.bot+1], s.grid[s.top:s.bot])
		s.fill(l)
		s.grid[s.top] = l
	}
}

func (s *Screen) escapeByte(c byte) {
	s.st = ground
	switch {
	case c < 0x20:
		s.st = escape
		s.control(c)
	case c >= 0x20 && c <= 0x2f:
		s.st = escInter
	case c == '[':
		s.st = csiParam
		s.params = s.params[:0]
		s.colon = s.colon[:0]
		s.priv = 0
		s.inter = false
	case c == ']' || c == 'P' || c == 'X' || c == '^' || c == '_':
		s.st = str
		s.strLen = 0
	case c == '7':
		s.saved = s.cur
	case c == '8':
		s.cur = s.saved
	case c == 'D':
		s.index()
	case c == 'E':
		s.cur.x = 0
		s.index()
	case c == 'M':
		s.reverseIndex()
	case c == 'c':
		s.reset()
	}
}

func (s *Screen) csiByte(c byte) {
	switch {
	case c < 0x20:
		s.control(c)
	case c >= '0' && c <= '9':
		if s.st == csiIgnore {
			return
		}
		if len(s.params) == 0 {
			s.params = append(s.params, 0)
			s.colon = append(s.colon, false)
		}
		v := &s.params[len(s.params)-1]
		*v = min(*v*10+int(c-'0'), maxValue)
	case c == ';' || c == ':':
		if s.st == csiIgnore {
			return
		}
		if len(s.params) == 0 {
			s.params = append(s.params, 0)
			s.colon = append(s.colon, false)
		}
		if len(s.params) == maxParams {
			s.st = csiIgnore
			return
		}
		s.params = append(s.params, 0)
		s.colon = append(s.colon, c == ':')
	case c >= 0x3c && c <= 0x3f: // private marker
		if len(s.params) > 0 || s.priv != 0 {
			s.st = csiIgnore
			return
		}
		s.priv = c
	case c >= 0x20 && c <= 0x2f:
		s.inter = true
	case c >= 0x40 && c <= 0x7e:
		ignore := s.st == csiIgnore || s.inter
		s.st = ground
		if !ignore {
			s.dispatch(c)
		}
	default:
		s.st = csiIgnore
	}
}

// arg is parameter i, or def when it is absent or zero.
func (s *Screen) arg(i, def int) int {
	if i >= len(s.params) || s.params[i] == 0 {
		return def
	}
	return s.params[i]
}

func (s *Screen) dispatch(final byte) {
	if s.priv != 0 {
		if s.priv == '?' && (final == 'h' || final == 'l') {
			for _, m := range s.params {
				switch m {
				case 7:
					s.autowrap = final == 'h'
				case 25:
					s.hidden = final == 'l'
				}
			}
		}
		return
	}
	n := s.arg(0, 1)
	cur := &s.cur
	if final != 'm' {
		cur.wrapNext = false
	}
	switch final {
	case '@':
		l := s.grid[cur.y]
		n = min(n, s.cols-cur.x)
		copy(l[cur.x+n:], l[cur.x:])
		s.fill(l[cur.x : cur.x+n])
	case 'A':
		cur.y = max(cur.y-n, s.upperBound())
	case 'B':
		cur.y = min(cur.y+n, s.lowerBound())
	case 'C':
		cur.x = min(cur.x+n, s.cols-1)
	case 'D':
		cur.x = max(cur.x-n, 0)
	case 'E':
		cur.x = 0
		cur.y = min(cur.y+n, s.lowerBound())
	case 'F':
		cur.x = 0
		cur.y = max(cur.y-n, s.upperBound())
	case 'G', '`':
		cur.x = min(n, s.cols) - 1
	case 'H', 'f':
		cur.y = min(n, s.rows) - 1
		cur.x = min(s.arg(1, 1), s.cols) - 1
	case 'd':
		cur.y = min(n, s.rows) - 1
	case 'J':
		s.eraseDisplay(s.arg(0, 0))
	case 'K':
		s.eraseLine(s.arg(0, 0))
	case 'L', 'M':
		if cur.y < s.top || cur.y > s.bot {
			return
		}
		top := s.top
		s.top = cur.y
		if final == 'L' {
			s.scrollDown(n)
		} else {
			s.scrollUpNoHistory(n)
		}
		s.top = top
		cur.x = 0
	case 'P':
		l := s.grid[cur.y]
		n = min(n, s.cols-cur.x)
		copy(l[cur.x:], l[cur.x+n:])
		s.fill(l[s.cols-n:])
	case 'X':
		s.fill(s.grid[cur.y][cur.x:min(cur.x+n, s.cols)])
	case 'S':
		s.scrollUp(n)
	case 'T':
		s.scrollDown(n)
	case 'r':
		top, bot := s.arg(0, 1)-1, min(s.arg(1, s.rows), s.rows)-1
		if top < bot {
			s.top, s.bot = top, bot
			cur.x, cur.y = 0, 0
		}
	case 's':
		s.saved = *cur
	case 'u':
		*cur = s.saved
	case 'm':
		s.sgr()
	}
}

// upperBound / lowerBound keep vertical moves inside the scroll region
// when the cursor is in it.
func (s *Screen) upperBound() int {
	if s.cur.y >= s.top {
		return s.top
	}
	return 0
}

func (s *Screen) lowerBound() int {
	if s.cur.y <= s.bot {
		return s.bot
	}
	return s.rows - 1
}

// scrollUpNoHistory is DL: lines deleted inside the screen are not history.
func (s *Screen) scrollUpNoHistory(n int) {
	sb := s.sbMax
	s.sbMax = 0
	s.scrollUp(n)
	s.sbMax = sb
}

func (s *Screen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		s.fill(s.grid[s.cur.y][s.cur.x:])
		for _, l := range s.grid[s.cur.y+1:] {
			s.fill(l)
		}
	case 1:
		for _, l := range s.grid[:s.cur.y] {
			s.fill(l)
		}
		s.fill(s.grid[s.cur.y][:s.cur.x+1])
	case 2:
		for _, l := range s.grid {
			s.fill(l)
		}
	case 3:
		s.ClearScrollback()
	}
}

func (s *Screen) eraseLine(mode int) {
	l := s.grid[s.cur.y]
	switch mode {
	case 0:
		s.fill(l[s.cur.x:])
	case 1:
		s.fill(l[:s.cur.x+1])
	case 2:
		s.fill(l)
	}
}

func (s *Screen) sgr() {
	p := s.params
	if len(p) == 0 {
		p = []int{0}
	}
	pen := &s.cur.pen
	for i := 0; i < len(p); i++ {
		// sub counts the colon subparameters that follow p[i].
		sub := 0
		for i+sub+1 < len(p) && s.colon[i+sub+1] {
			sub++
		}
		switch v := p[i]; {
		case v == 0:
			*pen = Cell{}
		case v == 1:
			pen.Attr |= Bold
		case v == 2:
			pen.Attr |= Faint
		case v == 3:
			pen.Attr |= Italic
		case v == 4:
			if sub > 0 && p[i+1] == 0 {
				pen.Attr &^= Underline
			} else {
				pen.Attr |= Underline
			}
		case v == 7:
			pen.Attr |= Reverse
		case v == 8:
			pen.Attr |= Conceal
		case v == 9:
			pen.Attr |= Strike
		case v == 21:
			pen.Attr |= Underline
		case v == 22:
			pen.Attr &^= Bold | Faint
		case v == 23:
			pen.Attr &^= Italic
		case v == 24:
			pen.Attr &^= Underline
		case v == 27:
			pen.Attr &^= Reverse
		case v == 28:
			pen.Attr &^= Conceal
		case v == 29:
			pen.Attr &^= Strike
		case v >= 30 && v <= 37:
			pen.FG = Indexed(uint8(v - 30))
		case v == 39:
			pen.FG = Default
		case v >= 40 && v <= 47:
			pen.BG = Indexed(uint8(v - 40))
		case v == 49:
			pen.BG = Default
		case v >= 90 && v <= 97:
			pen.FG = Indexed(uint8(v - 90 + 8))
		case v >= 100 && v <= 107:
			pen.BG = Indexed(uint8(v - 100 + 8))
		case v == 38 || v == 48:
			c, used, ok := s.extColor(p, i, sub)
			if !ok {
				return // a malformed colour: the rest is unreadable
			}
			if v == 38 {
				pen.FG = c
			} else {
				pen.BG = c
			}
			i += used
			continue
		}
		i += sub
	}
}

// extColor reads the colour after p[i] (38 or 48), in the colon form
// (38:5:n, 38:2:r:g:b, 38:2:id:r:g:b) when sub > 0, else the semicolon
// form (38;5;n, 38;2;r;g;b). It returns how many parameters it consumed.
func (s *Screen) extColor(p []int, i, sub int) (Color, int, bool) {
	args := p[i+1:]
	if sub > 0 {
		args = p[i+1 : i+1+sub]
	}
	b := func(v int) uint8 { return uint8(min(v, 255)) }
	switch {
	case len(args) >= 2 && args[0] == 5:
		return Indexed(b(args[1])), max(sub, 2), true
	case sub >= 5 && args[0] == 2:
		return RGB(b(args[2]), b(args[3]), b(args[4])), sub, true
	case len(args) >= 4 && args[0] == 2:
		return RGB(b(args[1]), b(args[2]), b(args[3])), max(sub, 4), true
	}
	return Default, 0, false
}
