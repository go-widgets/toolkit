// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"unicode/utf8"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit/vt"
)

// Terminal is a terminal emulator widget: the view of a [vt.Screen] (the
// model, which parses the output stream) plus the keyboard encoding,
// scrollback viewing, selection and copy/paste a terminal needs. The host
// writes a program's output with Write and receives what the user types,
// already encoded as a terminal sends it ("\r" for Enter, "\x1b[A" for the
// up arrow, "\x03" for Ctrl-C), through OnInput.
//
// It is safe for untrusted output by construction: the model has no reply
// channel and discards every string sequence (see package vt), so output
// can neither write the clipboard, nor make links, nor type into the
// shell. Pasted text is stripped of control characters (only tabs and
// line ends survive, as "\r") so a paste cannot smuggle an escape sequence
// into the input either.
//
// Keys: Ctrl+Shift+C / Cmd+C copy the selection, Ctrl+Shift+V / Cmd+V
// paste, Shift+PageUp / Shift+PageDown and the wheel scroll the history;
// every other key goes to the program. [Terminal.ClaimsKey] tells a host
// which keys to leave alone while the terminal has focus.
type Terminal struct {
	Base
	focusState

	// Palette overrides the 16 standard colours; an entry left zero (A==0)
	// keeps xterm's default for that index.
	Palette [16]RGBA

	// OnInput receives the bytes the user typed (keys, pastes).
	OnInput func(s string)

	// OnResize is told the grid's new size when SetBounds changes it, so
	// the host can tell the program (a pty's TIOCSWINSZ, or nothing).
	OnResize func(cols, rows int)

	screen *vt.Screen
	offset *mvvm.Observable[int]

	selecting  bool
	hasSel     bool
	selA, selB vt.Pos
	cellW      int
	cellH      int
}

// NewTerminal returns a terminal over a new cols x rows screen keeping
// scrollback lines of history (bounds: see [vt.New]). SetBounds refits the
// grid to the widget's size.
func NewTerminal(cols, rows, scrollback int) *Terminal {
	return &Terminal{screen: vt.New(cols, rows, scrollback)}
}

// Screen is the model the terminal shows.
func (t *Terminal) Screen() *vt.Screen {
	if t.screen == nil {
		t.screen = vt.New(80, 24, 1000)
	}
	return t.screen
}

// ScrollOffset is how many lines the view is scrolled back from the live
// bottom (0: following the output).
func (t *Terminal) ScrollOffset() *mvvm.Observable[int] {
	if t.offset == nil {
		t.offset = mvvm.NewObservable(0)
	}
	return t.offset
}

// Write feeds program output to the screen. A view scrolled back stays on
// the lines it shows; a view at the bottom follows the output.
func (t *Terminal) Write(p []byte) (int, error) {
	s := t.Screen()
	before := s.ScrollbackLen()
	n, err := s.Write(p)
	if off := t.ScrollOffset().Get(); off > 0 {
		t.ScrollOffset().Set(min(off+s.ScrollbackLen()-before, s.ScrollbackLen()))
	}
	return n, err
}

// ScrollBy moves the view n lines back into the history (negative: toward
// the live bottom), clamped to the history.
func (t *Terminal) ScrollBy(n int) {
	t.ScrollOffset().Set(min(max(t.ScrollOffset().Get()+n, 0), t.Screen().ScrollbackLen()))
}

// Selection is the selected text ("" without a selection).
func (t *Terminal) Selection() string {
	if !t.hasSel {
		return ""
	}
	return t.Screen().Text(t.selA, t.selB)
}

// ClearSelection drops the selection.
func (t *Terminal) ClearSelection() { t.hasSel, t.selecting = false, false }

// Copy puts the selection on the clipboard and reports whether there was one.
func (t *Terminal) Copy() bool {
	if !t.hasSel {
		return false
	}
	SetClipboardText(t.Selection())
	return true
}

// Paste sends the clipboard's text as input, without its control
// characters.
func (t *Terminal) Paste() {
	if s := SanitizePaste(ClipboardText()); s != "" {
		t.send(s)
	}
}

// SanitizePaste keeps the printable text of s, tabs, and line ends (as
// "\r", what Enter sends): an escape sequence, a C1 control or invalid
// UTF-8 in pasted text never reaches the program.
func SanitizePaste(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\r")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			b.WriteByte('\r')
		case r == '\t':
			b.WriteByte('\t')
		case r < 0x20 || r >= 0x7f && r <= 0x9f || r == utf8.RuneError:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (t *Terminal) send(s string) {
	t.ClearSelection()
	t.ScrollOffset().Set(0)
	if t.OnInput != nil {
		t.OnInput(s)
	}
}

// SetBounds places the terminal and refits the grid to as many cells as
// the bounds hold (at least 1 x 1), telling OnResize when that changes it.
func (t *Terminal) SetBounds(r Rect) {
	t.Base.SetBounds(r)
	t.cellW, t.cellH = t.glyphAdvance(), t.glyphHeight()
	if t.cellW <= 0 || t.cellH <= 0 {
		return
	}
	cols, rows := max(r.W/t.cellW, 1), max(r.H/t.cellH, 1)
	s := t.Screen()
	if c, rw := s.Size(); c == cols && rw == rows {
		return
	}
	s.Resize(cols, rows)
	cols, rows = s.Size()
	t.ScrollBy(0)
	if t.OnResize != nil {
		t.OnResize(cols, rows)
	}
}

// xterm's 16 standard colours.
var termPalette = [16]RGBA{
	RGB(0x00, 0x00, 0x00), RGB(0xcd, 0x00, 0x00), RGB(0x00, 0xcd, 0x00), RGB(0xcd, 0xcd, 0x00),
	RGB(0x00, 0x00, 0xee), RGB(0xcd, 0x00, 0xcd), RGB(0x00, 0xcd, 0xcd), RGB(0xe5, 0xe5, 0xe5),
	RGB(0x7f, 0x7f, 0x7f), RGB(0xff, 0x00, 0x00), RGB(0x00, 0xff, 0x00), RGB(0xff, 0xff, 0x00),
	RGB(0x5c, 0x5c, 0xff), RGB(0xff, 0x00, 0xff), RGB(0x00, 0xff, 0xff), RGB(0xff, 0xff, 0xff),
}

// TerminalColor resolves a cell colour against palette (16 entries; zero
// entries fall back to xterm's) and def, the default colour.
func TerminalColor(c vt.Color, palette *[16]RGBA, def RGBA) RGBA {
	if r, g, b, ok := c.RGB(); ok {
		return RGB(r, g, b)
	}
	n, ok := c.Index()
	switch {
	case !ok:
		return def
	case n < 16:
		if palette[n].A != 0 {
			return palette[n]
		}
		return termPalette[n]
	case n < 232:
		n -= 16
		level := func(v uint8) uint8 {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return RGB(level(n/36), level(n/6%6), level(n%6))
	}
	g := 8 + (n-232)*10
	return RGB(g, g, g)
}

// cellColors is the ink and ground a cell paints with.
func (t *Terminal) cellColors(c vt.Cell, fg0, bg0 RGBA) (fg, bg RGBA) {
	fgc := c.FG
	if n, ok := fgc.Index(); ok && n < 8 && c.Attr&vt.Bold != 0 {
		fgc = vt.Indexed(n + 8) // bold brightens, as xterm does
	}
	fg = TerminalColor(fgc, &t.Palette, fg0)
	bg = TerminalColor(c.BG, &t.Palette, bg0)
	if c.Attr&vt.Faint != 0 {
		fg = blendRGBA(fg, bg, 0.5)
	}
	if c.Attr&vt.Reverse != 0 {
		fg, bg = bg, fg
	}
	return fg, bg
}

func (t *Terminal) selected(p vt.Pos) bool {
	if !t.hasSel {
		return false
	}
	a, b := t.selA, t.selB
	if b.Before(a) {
		a, b = b, a
	}
	return !p.Before(a) && !b.Before(p)
}

// Draw paints the visible lines (scrolled back by ScrollOffset), the
// selection, the cursor (when shown and following the output) and the
// focus ring.
func (t *Terminal) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	fg0, bg0 := theme.OnSurface, theme.Surface
	fillRect(p, r.X, r.Y, r.W, r.H, bg0)
	if t.cellW <= 0 || t.cellH <= 0 {
		return
	}
	s := t.Screen()
	cols, rows := s.Size()
	first := s.ScrollbackLen() - t.ScrollOffset().Get()
	cx, cy, vis := s.Cursor()
	vis = vis && t.ScrollOffset().Get() == 0
	for row := 0; row < rows; row++ {
		line := s.Line(first + row)
		for col := 0; col < len(line) && col < cols; col++ {
			c := line[col]
			fg, bg := t.cellColors(c, fg0, bg0)
			if t.selected(vt.Pos{Line: first + row, Col: col}) {
				fg, bg = bg0, theme.Accent
			}
			if vis && col == cx && row == cy {
				fg, bg = bg, fg
			}
			x, y := r.X+col*t.cellW, r.Y+row*t.cellH
			if bg != bg0 {
				fillRect(p, x, y, t.cellW, t.cellH, bg)
			}
			if c.Rune != 0 && c.Rune != ' ' && c.Attr&vt.Conceal == 0 {
				t.drawText(p, x, y, string(c.Rune), fg)
			}
			if c.Attr&vt.Underline != 0 {
				fillRect(p, x, y+t.cellH-1, t.cellW, 1, fg)
			}
			if c.Attr&vt.Strike != 0 {
				fillRect(p, x, y+t.cellH/2, t.cellW, 1, fg)
			}
		}
	}
	t.drawFocusRing(p, theme, r)
}

// termKeys are the named keys and the bytes a terminal sends for them.
var termKeys = map[string]string{
	"Enter": "\r", "Backspace": "\x7f", "Tab": "\t", "Escape": "\x1b",
	"ArrowUp": "\x1b[A", "ArrowDown": "\x1b[B", "ArrowRight": "\x1b[C", "ArrowLeft": "\x1b[D",
	"Home": "\x1b[H", "End": "\x1b[F", "Insert": "\x1b[2~", "Delete": "\x1b[3~",
	"PageUp": "\x1b[5~", "PageDown": "\x1b[6~",
	"F1": "\x1bOP", "F2": "\x1bOQ", "F3": "\x1bOR", "F4": "\x1bOS",
}

// termAction is what a key press does in a terminal.
type termAction int

const (
	termNone termAction = iota
	termSend
	termCopy
	termPaste
	termScrollUp
	termScrollDown
)

// keyAction decides what ev does, and the bytes to send for termSend.
func keyAction(ev Event) (termAction, string) {
	code := ev.Code
	clip := ev.Meta || ev.Ctrl && ev.Shift
	switch {
	case clip && (code == "c" || code == "C"):
		return termCopy, ""
	case clip && (code == "v" || code == "V"):
		return termPaste, ""
	case ev.Shift && code == "PageUp":
		return termScrollUp, ""
	case ev.Shift && code == "PageDown":
		return termScrollDown, ""
	case ev.Shift && code == "Tab":
		return termSend, "\x1b[Z"
	case ev.Meta:
		return termNone, "" // the platform's own shortcuts
	}
	if seq, ok := termKeys[code]; ok {
		if ev.Alt {
			seq = "\x1b" + seq
		}
		return termSend, seq
	}
	if len(code) != 1 {
		return termNone, ""
	}
	c := code[0]
	switch {
	case ev.Ctrl && (c >= 'a' && c <= 'z' || c >= '@' && c <= '_'):
		return termSend, string(rune(c & 0x1f))
	case ev.Ctrl && c == ' ':
		return termSend, "\x00"
	case ev.Alt && c >= ' ' && c < 0x7f:
		return termSend, "\x1b" + code
	}
	return termNone, ""
}

// ClaimsKey reports whether the terminal acts on ev, a key press: a host
// with global shortcuts gives the terminal the keys it claims while it has
// focus (Ctrl-C, Ctrl-D, Ctrl-L, Ctrl-R, Ctrl-W, Ctrl-U, arrows, ...).
func (t *Terminal) ClaimsKey(ev Event) bool {
	a, _ := keyAction(ev)
	return a != termNone
}

func (t *Terminal) posAt(x, y int) vt.Pos {
	s := t.Screen()
	cols, rows := s.Size()
	col := min(max(x/max(t.cellW, 1), 0), cols-1)
	row := min(max(y/max(t.cellH, 1), 0), rows-1)
	return vt.Pos{Line: s.ScrollbackLen() - t.ScrollOffset().Get() + row, Col: col}
}

// OnEvent handles typing, the clipboard keys, scrolling and selection.
func (t *Terminal) OnEvent(ev Event) {
	if t.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventChar:
		if ev.Code == "" {
			return
		}
		if ev.Alt {
			t.send("\x1b" + ev.Code)
			return
		}
		t.send(ev.Code)
	case EventKeyDown:
		a, seq := keyAction(ev)
		_, rows := t.Screen().Size()
		switch a {
		case termSend:
			t.send(seq)
		case termCopy:
			t.Copy()
		case termPaste:
			t.Paste()
		case termScrollUp:
			t.ScrollBy(rows)
		case termScrollDown:
			t.ScrollBy(-rows)
		}
	case EventScroll:
		t.ScrollBy(-ev.Delta)
	case EventClick:
		t.selA = t.posAt(ev.X, ev.Y)
		t.selB = t.selA
		t.selecting, t.hasSel = true, false
	case EventMouseDrag:
		if t.selecting {
			t.selB = t.posAt(ev.X, ev.Y)
			t.hasSel = t.selA != t.selB
		}
	case EventMouseUp:
		t.selecting = false
	}
}

// A11y reports the terminal as a textbox whose value is the visible text.
func (t *Terminal) A11y() A11yInfo {
	s := t.Screen()
	cols, rows := s.Size()
	first := s.ScrollbackLen() - t.ScrollOffset().Get()
	return A11yInfo{Role: RoleTextbox, Name: "Terminal",
		Value: s.Text(vt.Pos{Line: first}, vt.Pos{Line: first + rows - 1, Col: cols - 1})}
}

var _ Accessible = (*Terminal)(nil)
