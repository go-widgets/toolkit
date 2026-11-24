// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-widgets/painter"
)

func TestPreeditSanitize(t *testing.T) {
	long := strings.Repeat("あ", MaxPreeditRunes+50)
	cases := []struct {
		name string
		in   Preedit
		text string
		cur  int
		segs []PreeditSegment
	}{
		{"plain", Preedit{Text: "かん", Cursor: 2}, "かん", 2, nil},
		{"cursor high", Preedit{Text: "ab", Cursor: 9}, "ab", 2, nil},
		{"cursor low", Preedit{Text: "ab", Cursor: -7}, "ab", -1, nil},
		{"invalid utf8", Preedit{Text: "a\xffb", Cursor: 3}, "a�b", 3, nil},
		{"controls dropped", Preedit{Text: "a\x00\n\tb\x7f\u0085", Cursor: 2}, "ab", 2, nil},
		{"truncated", Preedit{Text: long, Cursor: 1 << 30}, strings.Repeat("あ", MaxPreeditRunes), MaxPreeditRunes, nil},
		{"segments clamped", Preedit{Text: "abcd", Cursor: 0, Segments: []PreeditSegment{
			{Start: -3, End: 2, Style: PreeditTarget},
			{Start: 3, End: 99, Style: PreeditConverted},
			{Start: 2, End: 2},             // empty
			{Start: 3, End: 1},             // inverted
			{Start: 0, End: 1, Style: 200}, // unknown style
		}}, "abcd", 0, []PreeditSegment{
			{0, 2, PreeditTarget}, {3, 4, PreeditConverted}, {0, 1, PreeditInput},
		}},
	}
	for _, c := range cases {
		got := c.in.Sanitize()
		if got.Text != c.text || got.Cursor != c.cur || len(got.Segments) != len(c.segs) {
			t.Errorf("%s: got %+v", c.name, got)
			continue
		}
		for i, s := range c.segs {
			if got.Segments[i] != s {
				t.Errorf("%s: segment %d = %+v, want %+v", c.name, i, got.Segments[i], s)
			}
		}
	}
	many := make([]PreeditSegment, MaxPreeditSegments*3)
	for i := range many {
		many[i] = PreeditSegment{Start: 0, End: 1}
	}
	if n := len(Preedit{Text: "a", Segments: many}.Sanitize().Segments); n != MaxPreeditSegments {
		t.Errorf("segments kept: %d", n)
	}
}

func TestPreeditStyleAt(t *testing.T) {
	p := Preedit{Text: "abc", Segments: []PreeditSegment{{0, 3, PreeditConverted}, {1, 2, PreeditTarget}}}
	want := []PreeditStyle{PreeditConverted, PreeditTarget, PreeditConverted, PreeditInput}
	for i, w := range want {
		if got := p.styleAt(i); got != w {
			t.Errorf("styleAt(%d) = %d, want %d", i, got, w)
		}
	}
}

func TestBoundedTextMultiline(t *testing.T) {
	if s, n := boundedText("a\nb\tc\r", 10, true); s != "a\nb\tc" || n != 5 {
		t.Errorf("multiline: %q %d", s, n)
	}
	if s, n := boundedText("a\nb", 10, false); s != "ab" || n != 2 {
		t.Errorf("single line: %q %d", s, n)
	}
	// the excess is never scanned: 4 bytes a rune is the longest a rune gets
	if s, _ := boundedText(strings.Repeat("x", 1<<20), 8, false); len(s) != 8 {
		t.Errorf("len %d", len(s))
	}
}

func composeFields() map[string]interface {
	Composer
	Widget
} {
	e := NewEntry("ab")
	e.SetBounds(Rect{X: 5, Y: 6, W: 200, H: 30})
	e.SetFocused(true)
	v := NewTextView("ab")
	v.SetBounds(Rect{X: 5, Y: 6, W: 300, H: 100})
	v.SetFocused(true)
	v.CursorCol().Set(2) // a TextView starts with the caret at the start
	return map[string]interface {
		Composer
		Widget
	}{"entry": e, "textview": v}
}

func textOf(w Widget) string {
	switch f := w.(type) {
	case *Entry:
		return f.Text().Get()
	case *TextView:
		return f.Text().Get()
	}
	return ""
}

// The preedit is shown, not typed; the commit types it once, at the caret.
func TestComposerLifecycle(t *testing.T) {
	for name, f := range composeFields() {
		pre := Preedit{Text: "かん", Cursor: 1, Segments: []PreeditSegment{{0, 1, PreeditTarget}}}
		f.SetPreedit(pre)
		if textOf(f) != "ab" {
			t.Errorf("%s: preedit entered the text: %q", name, textOf(f))
		}
		got := f.Preedit()
		if got.Text != "かん" || got.Cursor != 1 || len(got.Segments) != 1 {
			t.Errorf("%s: Preedit() = %+v", name, got)
		}
		f.SetPreedit(Preedit{Text: "かんじ", Cursor: 3})
		if f.Preedit().Text != "かんじ" || len(f.Preedit().Segments) != 0 {
			t.Errorf("%s: an update must replace the preedit: %+v", name, f.Preedit())
		}
		f.CommitText("漢字")
		if textOf(f) != "ab漢字" || f.Preedit().Text != "" {
			t.Errorf("%s: after commit: %q, preedit %+v", name, textOf(f), f.Preedit())
		}
		f.SetPreedit(Preedit{Text: "x"})
		f.CancelPreedit()
		if textOf(f) != "ab漢字" || f.Preedit().Text != "" {
			t.Errorf("%s: cancel typed something or kept the preedit: %q %+v", name, textOf(f), f.Preedit())
		}
		f.SetPreedit(Preedit{Text: "x"})
		f.SetPreedit(Preedit{})
		if f.Preedit().Text != "" {
			t.Errorf("%s: an empty preedit must end the composition", name)
		}
		f.CommitText("")
		f.CommitText("\xff\x00")
		if !utf8.ValidString(textOf(f)) {
			t.Errorf("%s: invalid UTF-8 got in: %q", name, textOf(f))
		}
	}
}

func TestComposerRefusals(t *testing.T) {
	// A disabled field keeps no preedit and types no commit.
	for name, f := range composeFields() {
		switch x := f.(type) {
		case *Entry:
			x.Disabled().Set(true)
		case *TextView:
			x.Disabled().Set(true)
		}
		f.SetPreedit(Preedit{Text: "x"})
		f.CommitText("y")
		if f.Preedit().Text != "" || textOf(f) != "ab" {
			t.Errorf("%s: disabled field took input: %q %+v", name, textOf(f), f.Preedit())
		}
	}
	// A masked entry keeps no preedit (it would be the secret in clear); the
	// commit still goes in, masked.
	m := NewEntry("")
	m.Mask = '•'
	m.SetPreedit(Preedit{Text: "secret"})
	if m.Preedit().Text != "" {
		t.Error("masked entry kept a preedit")
	}
	m.CommitText("s")
	if m.Text().Get() != "s" {
		t.Error("masked commit lost")
	}
	// A single-line entry composes on one line.
	e := NewEntry("")
	e.SetPreedit(Preedit{Text: "a\nb"})
	if e.Preedit().Text != "ab" {
		t.Errorf("entry preedit %q", e.Preedit().Text)
	}
	// A multi-line commit stays multi-line in a TextView and is flat in an Entry.
	v := NewTextView("")
	v.CommitText("a\nb")
	if v.Text().Get() != "a\nb" {
		t.Errorf("textview commit %q", v.Text().Get())
	}
	e.CommitText("a\nb")
	if e.Text().Get() != "ab" {
		t.Errorf("entry commit %q", e.Text().Get())
	}
}

// The event path (a host that only has the preview text) and SetPreedit agree.
func TestComposerEventPathShowsDefaultPreedit(t *testing.T) {
	e := NewEntry("")
	e.OnEvent(Event{Kind: EventCompositionUpdate, Code: "ê"})
	if p := e.Preedit(); p.Text != "ê" || p.Cursor != 1 || len(p.Segments) != 0 {
		t.Errorf("%+v", p)
	}
	// stale metadata is never applied to a text it does not describe
	e.SetPreedit(Preedit{Text: "ê", Cursor: 0, Segments: []PreeditSegment{{0, 1, PreeditTarget}}})
	e.OnEvent(Event{Kind: EventCompositionUpdate, Code: "î"})
	if p := e.Preedit(); p.Cursor != 1 || len(p.Segments) != 0 {
		t.Errorf("stale metadata applied: %+v", p)
	}
	if p := (preeditMeta{}).view(""); p.Text != "" || p.Cursor != -1 {
		t.Errorf("empty view %+v", p)
	}
}

func drawEntry(e *Entry) []byte {
	const w, h = 200, 30
	e.SetBounds(Rect{W: w, H: h})
	e.focused = true
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), DefaultLight())
	return buf
}

// Segments, the cursor and the thick clause change what is drawn.
func TestPreeditDrawing(t *testing.T) {
	mk := func(p Preedit) []byte {
		e := NewEntry("")
		e.SetPreedit(p)
		return drawEntry(e)
	}
	plain := mk(Preedit{Text: "かんじ", Cursor: 3})
	target := mk(Preedit{Text: "かんじ", Cursor: 3, Segments: []PreeditSegment{{0, 2, PreeditTarget}}})
	if bytes.Equal(plain, target) {
		t.Error("a target clause draws like plain input")
	}
	mid := mk(Preedit{Text: "かんじ", Cursor: 1})
	if bytes.Equal(plain, mid) {
		t.Error("the input method cursor does not move the caret")
	}
	hidden := mk(Preedit{Text: "かんじ", Cursor: -1})
	if bytes.Equal(plain, hidden) {
		t.Error("cursor -1 must hide the caret")
	}
	none := drawEntry(NewEntry(""))
	if bytes.Equal(plain, none) {
		t.Error("the preedit is not drawn")
	}
	v := NewTextView("x")
	v.SetBounds(Rect{W: 200, H: 60})
	v.SetFocused(true)
	buf := makeSurface(200, 60)
	v.Draw(newP(buf, 200), DefaultLight())
	v.SetPreedit(Preedit{Text: "かん", Cursor: 1, Segments: []PreeditSegment{{0, 1, PreeditTarget}}})
	buf2 := makeSurface(200, 60)
	v.Draw(newP(buf2, 200), DefaultLight())
	if bytes.Equal(buf, buf2) {
		t.Error("a TextView does not draw its preedit")
	}
}

// The candidate window anchors at the composition: after the text, widening
// with the preedit.
func TestCompositionRect(t *testing.T) {
	for name, f := range composeFields() {
		bare := f.CompositionRect()
		if bare.W != 1 || bare.H <= 0 {
			t.Errorf("%s: bare caret rect %+v", name, bare)
		}
		f.SetPreedit(Preedit{Text: "かんじ"})
		with := f.CompositionRect()
		if with.W <= bare.W || with.X != bare.X || with.Y != bare.Y {
			t.Errorf("%s: preedit rect %+v vs caret %+v", name, with, bare)
		}
		b := f.Bounds()
		if with.X < b.X || with.Y < b.Y || with.Y+with.H > b.Y+b.H {
			t.Errorf("%s: rect %+v outside bounds %+v", name, with, b)
		}
	}
	// the entry's rect follows the caret, not the end of the text
	e := NewEntry("hello")
	e.SetBounds(Rect{W: 200, H: 30})
	e.cursor = 1
	r1 := e.CompositionRect()
	e.cursor = 4
	if r4 := e.CompositionRect(); r4.X <= r1.X {
		t.Errorf("caret did not move the rect: %+v %+v", r1, r4)
	}
	e.cursor = 99 // out of range is clamped, never a panic
	_ = e.CompositionRect()
}

// ghost is a container that is not focusable, to hold fields.
type ghost struct {
	Base
	kids []Widget
	off  [2]int
}

func (g *ghost) Draw(painter.Painter, *Theme) {}
func (g *ghost) OnEvent(Event)                {}
func (g *ghost) Children() []Widget           { return g.kids }
func (g *ghost) ChildOffset() (int, int)      { return g.off[0], g.off[1] }

// While an annotation panel is open an input method types into the panel: the
// text view shows no preedit of its own.
func TestTextViewPreeditRefusedWhileAnnotationOpen(t *testing.T) {
	v := NewTextView("abc")
	v.SetBounds(Rect{W: 300, H: 100})
	v.SetFocused(true)
	v.EnableAnnotations(true)
	v.SetPreedit(Preedit{Text: "x"})
	if v.Preedit().Text != "x" {
		t.Fatal("control: a closed panel should show the preedit")
	}
	v.OpenAnnotation(0)
	if !v.ann.open {
		t.Fatal("the annotation panel did not open")
	}
	v.SetPreedit(Preedit{Text: "y"})
	if v.Preedit().Text != "" {
		t.Errorf("preedit shown under an open annotation panel: %+v", v.Preedit())
	}
}

func TestFocusedComposition(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("a nil widget panicked the walk: %v", r)
		}
	}()
	if _, _, ok := FocusedComposition(nil); ok {
		t.Error("nil root")
	}
	withNil := &ghost{kids: []Widget{nil}}
	withNil.SetBounds(Rect{W: 10, H: 10})
	if _, _, ok := FocusedComposition(withNil); ok {
		t.Error("nil child")
	}
	a := NewEntry("a")
	a.SetBounds(Rect{X: 1, Y: 2, W: 100, H: 30})
	b := NewEntry("b")
	b.SetBounds(Rect{X: 1, Y: 40, W: 100, H: 30})
	hidden := NewEntry("h")
	hidden.SetFocused(true) // empty bounds: never returned
	inner := &ghost{kids: []Widget{a, b, hidden}}
	inner.SetBounds(Rect{X: 10, Y: 20, W: 300, H: 300})
	inner.off = [2]int{3, 4}
	root := &ghost{kids: []Widget{inner}}
	root.SetBounds(Rect{W: 400, H: 400})
	root.off = [2]int{100, 200}

	if _, _, ok := FocusedComposition(root); ok {
		t.Fatal("nothing focused yet")
	}
	b.SetFocused(true)
	c, rect, ok := FocusedComposition(root)
	if !ok || c != Composer(b) {
		t.Fatalf("focused field not found: %v %v", c, ok)
	}
	want := b.CompositionRect()
	want.X, want.Y = want.X+100+3, want.Y+200+4
	if rect != want {
		t.Errorf("rect %+v, want %+v", rect, want)
	}
	// A focused widget that cannot compose is not a target.
	cb := NewButton("x", nil)
	cb.SetBounds(Rect{W: 10, H: 10})
	cb.SetFocused(true)
	b.SetFocused(false)
	if _, _, ok := FocusedComposition(&ghost{Base: Base{}, kids: []Widget{cb}}); ok {
		t.Error("a button is not a composer")
	}
	root.SetBounds(Rect{})
	b.SetFocused(true)
	if _, _, ok := FocusedComposition(root); ok {
		t.Error("a hidden root hides its fields")
	}
}

// An arbitrary sequence of preedit, commit and cancel keeps the text valid
// UTF-8 and the caret in range, and the preedit within its bounds.
func FuzzComposition(f *testing.F) {
	f.Add([]byte{0, 'a', 'b', 1, 'x', 2, 0, 0xe3, 0x81, 0x82, 1, 0xff, 0xfe})
	f.Add([]byte{0, 0xf0, 0x9f, 0x98, 0x80, 3, 1, 0, 1})
	f.Add([]byte("\x00\x00\x00\x00\x01\x01\x01\x02\x02\x02"))
	f.Fuzz(func(t *testing.T, data []byte) {
		e := NewEntry("é")
		e.SetBounds(Rect{W: 200, H: 30})
		e.SetFocused(true)
		v := NewTextView("é\nz")
		v.SetBounds(Rect{W: 200, H: 80})
		v.SetFocused(true)
		fields := []Composer{e, v}
		for len(data) > 0 {
			op, rest := data[0]%4, data[1:]
			n := 0
			if len(rest) > 0 {
				n = int(rest[0]) % 16
				rest = rest[1:]
			}
			if n > len(rest) {
				n = len(rest)
			}
			payload := string(rest[:n])
			data = rest[n:]
			for _, c := range fields {
				switch op {
				case 0:
					c.SetPreedit(Preedit{Text: payload, Cursor: n - 3, Segments: []PreeditSegment{{n - 5, n, PreeditStyle(n)}}})
				case 1:
					c.CommitText(payload)
				case 2:
					c.CancelPreedit()
				case 3:
					c.SetPreedit(Preedit{Text: strings.Repeat(payload+"é", 400), Cursor: 1 << 20})
				}
				check(t, e, v)
			}
		}
		// drawing a composition never panics either
		drawEntry(e)
		v.Draw(newP(makeSurface(200, 80), 200), DefaultLight())
	})
}

func check(t *testing.T, e *Entry, v *TextView) {
	t.Helper()
	if s := e.Text().Get(); !utf8.ValidString(s) {
		t.Fatalf("entry text invalid UTF-8: %q", s)
	}
	if e.cursor < 0 || e.cursor > utf8.RuneCountInString(e.Text().Get()) {
		t.Fatalf("entry caret %d out of range for %q", e.cursor, e.Text().Get())
	}
	if s := v.Text().Get(); !utf8.ValidString(s) {
		t.Fatalf("textview text invalid UTF-8: %q", s)
	}
	l, c := v.CursorLine().Get(), v.CursorCol().Get()
	if l < 0 || l >= len(v.lines) || c < 0 || c > utf8.RuneCountInString(v.lines[l]) {
		t.Fatalf("textview caret (%d,%d) out of range", l, c)
	}
	for _, p := range []Preedit{e.Preedit(), v.Preedit()} {
		n := utf8.RuneCountInString(p.Text)
		if !utf8.ValidString(p.Text) || n > MaxPreeditRunes || p.Cursor < -1 || p.Cursor > n || len(p.Segments) > MaxPreeditSegments {
			t.Fatalf("preedit out of bounds: %+v", p)
		}
	}
}
