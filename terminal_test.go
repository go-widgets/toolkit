// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit/vt"
)

// zeroFont has no metrics: a terminal cannot lay out cells with it.
type zeroFont struct{}

func (zeroFont) Advance() int                                 { return 0 }
func (zeroFont) Height() int                                  { return 0 }
func (zeroFont) Measure(string) int                           { return 0 }
func (zeroFont) Draw(painter.Painter, int, int, string, RGBA) {}

func newTestTerminal(t *testing.T, cols, rows int) (*Terminal, *[]string) {
	t.Helper()
	term := NewTerminal(1, 1, 50)
	var got []string
	term.OnInput = func(s string) { got = append(got, s) }
	term.SetBounds(Rect{W: cols * GlyphAdvance(), H: rows * GlyphHeight()})
	return term, &got
}

func TestTerminalFitsBoundsAndResizes(t *testing.T) {
	var sizes [][2]int
	term := NewTerminal(80, 24, 10)
	term.OnResize = func(c, r int) { sizes = append(sizes, [2]int{c, r}) }
	term.SetBounds(Rect{W: 10*GlyphAdvance() + 3, H: 4 * GlyphHeight()})
	if c, r := term.Screen().Size(); c != 10 || r != 4 {
		t.Fatalf("Size = %d,%d", c, r)
	}
	term.SetBounds(Rect{X: 5, W: 10 * GlyphAdvance(), H: 4 * GlyphHeight()}) // same grid
	term.SetBounds(Rect{W: 1, H: 1})                                         // at least 1x1
	if len(sizes) != 2 || sizes[0] != [2]int{10, 4} || sizes[1] != [2]int{1, 1} {
		t.Errorf("OnResize calls = %v", sizes)
	}
	term.OnResize = nil
	term.SetBounds(Rect{W: 100, H: 100})

	z := NewTerminal(5, 5, 0)
	z.Font = zeroFont{}
	z.SetBounds(Rect{W: 100, H: 100})
	if c, _ := z.Screen().Size(); c != 5 {
		t.Error("a font without metrics resized the grid")
	}
	buf := makeSurface(100, 100)
	z.Draw(newP(buf, 100), DefaultLight()) // fills, draws no cell
}

func TestTerminalZeroValue(t *testing.T) {
	var term Terminal
	if c, r := term.Screen().Size(); c != 80 || r != 24 {
		t.Errorf("zero-value Size = %d,%d", c, r)
	}
	if term.A11y().Role != RoleTextbox {
		t.Error("A11y role")
	}
}

func TestTerminalKeys(t *testing.T) {
	term, got := newTestTerminal(t, 10, 3)
	for _, ev := range []Event{
		{Kind: EventChar, Code: "a"},
		{Kind: EventChar, Code: "x", Alt: true},
		{Kind: EventChar, Code: ""},
		{Kind: EventKeyDown, Code: "Enter"},
		{Kind: EventKeyDown, Code: "ArrowUp"},
		{Kind: EventKeyDown, Code: "ArrowLeft", Alt: true},
		{Kind: EventKeyDown, Code: "Tab", Shift: true},
		{Kind: EventKeyDown, Code: "c", Ctrl: true},
		{Kind: EventKeyDown, Code: "D", Ctrl: true},
		{Kind: EventKeyDown, Code: "[", Ctrl: true},
		{Kind: EventKeyDown, Code: " ", Ctrl: true},
		{Kind: EventKeyDown, Code: "b", Alt: true},
		{Kind: EventKeyDown, Code: "q", Meta: true}, // the platform's
		{Kind: EventKeyDown, Code: "Shift"},         // a bare modifier
		{Kind: EventKeyDown, Code: "1", Ctrl: true}, // nothing to send
		{Kind: EventKeyDown, Code: "F1"},
		{Kind: EventKeyUp, Code: "Enter"}, // ignored
	} {
		term.OnEvent(ev)
	}
	want := []string{"a", "\x1bx", "\r", "\x1b[A", "\x1b\x1b[D", "\x1b[Z", "\x03", "\x04", "\x1b", "\x00", "\x1bb", "\x1bOP"}
	if strings.Join(*got, "|") != strings.Join(want, "|") {
		t.Errorf("input = %q, want %q", *got, want)
	}
	for _, c := range []struct {
		ev   Event
		want bool
	}{
		{Event{Code: "c", Ctrl: true}, true}, {Event{Code: "l", Ctrl: true}, true},
		{Event{Code: "C", Ctrl: true, Shift: true}, true}, {Event{Code: "k", Meta: true}, false},
		{Event{Code: "Shift"}, false},
	} {
		if term.ClaimsKey(c.ev) != c.want {
			t.Errorf("ClaimsKey(%+v) = %v", c.ev, !c.want)
		}
	}
	term.Disabled().Set(true)
	term.OnEvent(Event{Kind: EventChar, Code: "z"})
	if len(*got) != len(want) {
		t.Error("a disabled terminal sent input")
	}
	term.Disabled().Set(false)
	term.OnInput = nil
	term.OnEvent(Event{Kind: EventChar, Code: "z"}) // no listener: nothing breaks
}

func TestTerminalScrollback(t *testing.T) {
	term, _ := newTestTerminal(t, 4, 2)
	term.Write([]byte("1\r\n2\r\n3\r\n4\r\n5"))
	if term.Screen().ScrollbackLen() != 3 {
		t.Fatalf("scrollback = %d", term.Screen().ScrollbackLen())
	}
	term.OnEvent(Event{Kind: EventScroll, Delta: -1})
	if term.ScrollOffset().Get() != 1 {
		t.Fatalf("offset = %d", term.ScrollOffset().Get())
	}
	term.Write([]byte("\r\n6")) // scrolled back: stays on its lines
	if term.ScrollOffset().Get() != 2 {
		t.Errorf("anchored offset = %d", term.ScrollOffset().Get())
	}
	term.OnEvent(Event{Kind: EventKeyDown, Code: "PageUp", Shift: true})
	if term.ScrollOffset().Get() != 4 {
		t.Errorf("Shift+PageUp offset = %d (clamped to the history)", term.ScrollOffset().Get())
	}
	if v := term.A11y().Value; v != "1\n2" {
		t.Errorf("A11y value scrolled back = %q", v)
	}
	term.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown", Shift: true})
	if term.ScrollOffset().Get() != 2 {
		t.Errorf("Shift+PageDown offset = %d", term.ScrollOffset().Get())
	}
	term.OnEvent(Event{Kind: EventChar, Code: "k"}) // typing returns to the bottom
	if term.ScrollOffset().Get() != 0 {
		t.Errorf("offset after typing = %d", term.ScrollOffset().Get())
	}
	term.Write([]byte("\r\n7"))
	if term.ScrollOffset().Get() != 0 {
		t.Error("a view at the bottom stopped following")
	}
}

func TestTerminalSelectionAndClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	term, got := newTestTerminal(t, 8, 2)
	term.Write([]byte("hello\r\nworld"))
	cw, ch := GlyphAdvance(), GlyphHeight()
	if term.Copy() || term.Selection() != "" {
		t.Fatal("a selection out of nothing")
	}
	term.OnEvent(Event{Kind: EventClick, X: 1 * cw, Y: 0})
	term.OnEvent(Event{Kind: EventMouseDrag, X: 2 * cw, Y: ch}) // to "wor"
	term.OnEvent(Event{Kind: EventMouseUp})
	term.OnEvent(Event{Kind: EventMouseDrag, X: 0, Y: 0}) // not selecting: ignored
	if s := term.Selection(); s != "ello\nwor" {
		t.Fatalf("Selection = %q", s)
	}
	term.OnEvent(Event{Kind: EventKeyDown, Code: "C", Ctrl: true, Shift: true})
	if ClipboardText() != "ello\nwor" {
		t.Errorf("clipboard = %q", ClipboardText())
	}
	// Drawing a selection paints the accent; a reversed drag selects too.
	buf := makeSurface(8*cw, 2*ch)
	term.Draw(newP(buf, 8*cw), DefaultLight())
	if pixelAt(buf, 8*cw, 1*cw, ch-1) != DefaultLight().Accent {
		t.Error("selection not painted")
	}
	term.OnEvent(Event{Kind: EventClick, X: 3 * cw, Y: ch})
	term.OnEvent(Event{Kind: EventMouseDrag, X: -50, Y: -50})
	if s := term.Selection(); s != "hello\nworl" {
		t.Errorf("reversed Selection = %q", s)
	}
	term.Draw(newP(buf, 8*cw), DefaultLight())
	if pixelAt(buf, 8*cw, cw-1, 0) != DefaultLight().Accent { // the spacing column
		t.Error("reversed selection not painted")
	}
	SetClipboardText("ls -l\r\n\x1b[31mrm\x9b\t\x07é\n")
	term.OnEvent(Event{Kind: EventKeyDown, Code: "v", Meta: true})
	if last := (*got)[len(*got)-1]; last != "ls -l\r[31mrm\té\r" {
		t.Errorf("paste sent %q", last)
	}
	if term.Selection() != "" {
		t.Error("input kept the selection")
	}
	n := len(*got)
	SetClipboardText("\x1b\x07")
	term.Paste()
	if len(*got) != n {
		t.Error("an empty paste was sent")
	}
	if SanitizePaste("a\xffb") != "ab" {
		t.Error("invalid UTF-8 kept")
	}
}

func TestTerminalColors(t *testing.T) {
	var term Terminal
	def := RGB(1, 2, 3)
	for _, c := range []struct {
		in   vt.Color
		want RGBA
	}{
		{vt.Default, def}, {vt.RGB(9, 8, 7), RGB(9, 8, 7)}, {vt.Indexed(1), RGB(0xcd, 0, 0)},
		{vt.Indexed(16), RGB(0, 0, 0)}, {vt.Indexed(231), RGB(255, 255, 255)},
		{vt.Indexed(17), RGB(0, 0, 95)}, {vt.Indexed(232), RGB(8, 8, 8)}, {vt.Indexed(255), RGB(238, 238, 238)},
	} {
		if got := TerminalColor(c.in, &term.Palette, def); got != c.want {
			t.Errorf("TerminalColor(%x) = %v, want %v", c.in, got, c.want)
		}
	}
	term.Palette[1] = RGB(1, 1, 1)
	if TerminalColor(vt.Indexed(1), &term.Palette, def) != RGB(1, 1, 1) {
		t.Error("palette override")
	}
	fg, bg := term.cellColors(vt.Cell{FG: vt.Indexed(2), Attr: vt.Bold | vt.Reverse}, def, RGB(0, 0, 0))
	if bg != RGB(0, 0xff, 0) || fg != RGB(0, 0, 0) {
		t.Errorf("bold+reverse = %v on %v", fg, bg)
	}
	fg, _ = term.cellColors(vt.Cell{FG: vt.RGB(200, 200, 200), Attr: vt.Faint | vt.Bold}, def, RGB(0, 0, 0))
	if fg == RGB(200, 200, 200) {
		t.Error("faint did not dim")
	}
}

func TestTerminalDraw(t *testing.T) {
	term, _ := newTestTerminal(t, 6, 2)
	cw, ch := GlyphAdvance(), GlyphHeight()
	w, h := 6*cw, 2*ch
	term.Write([]byte("\x1b[41m \x1b[0;4mB\x1b[0;9mC\x1b[8mD\x1b[0m E"))
	term.SetFocused(true)
	theme := DefaultLight()
	buf := makeSurface(w, h)
	term.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 1) != RGB(0xcd, 0, 0) {
		t.Errorf("red background = %v", pixelAt(buf, w, 0, 1))
	}
	if pixelAt(buf, w, cw+1, ch-1) != theme.OnSurface {
		t.Error("underline not drawn")
	}
	if pixelAt(buf, w, 2*cw, ch/2) != theme.OnSurface {
		t.Error("strike not drawn")
	}
	if scanHasColor(buf, w, 3*cw+1, 1, cw-2, ch-2, theme.OnSurface) {
		t.Error("concealed glyph drawn")
	}
	// The cursor (after "E", at column 6 -> pending wrap on 5) is inverted.
	x, y, _ := term.Screen().Cursor()
	if pixelAt(buf, w, x*cw+cw-1, y*ch+1) != theme.OnSurface {
		t.Error("cursor not inverted")
	}
	// Scrolled back, the cursor is not drawn.
	term.Write([]byte("\r\n\r\n"))
	term.ScrollBy(1)
	buf = makeSurface(w, h)
	term.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 1) != RGB(0xcd, 0, 0) {
		t.Error("the history line is not shown scrolled back")
	}
}
