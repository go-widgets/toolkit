// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- test instrument: a counting, deterministic Highlighter --------------
//
// countingHL is the probe the cache tests rely on to prove Draw re-lexes only
// when an input actually changed. Before any cache test trusts it (see
// TestCountingHighlighterControl), it is validated against a known-good
// control: a fresh probe reports zero calls, each Highlight bumps the counter
// by exactly one, records the language + a copy of the lines it was handed,
// and returns the exact spans it was seeded with. Only once the instrument is
// proven do the cache assertions ("calls == 1 after two identical Draws")
// carry weight.
type countingHL struct {
	calls     int
	lastLang  string
	lastLines []string
	spans     [][]TextSpan
}

func (h *countingHL) Highlight(language string, lines []string, _ *Theme) [][]TextSpan {
	h.calls++
	h.lastLang = language
	h.lastLines = append([]string(nil), lines...)
	return h.spans
}

func TestCountingHighlighterControl(t *testing.T) {
	kw := RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}
	seed := [][]TextSpan{{{Start: 0, End: 2, Color: kw}}}
	h := &countingHL{spans: seed}

	// Known-good baseline: an untouched probe has never been called, so a
	// later "was it called?" assertion is meaningful.
	if h.calls != 0 {
		t.Fatalf("fresh probe calls = %d, want 0", h.calls)
	}
	got := h.Highlight("go", []string{"ab", "cd"}, nil)
	if h.calls != 1 {
		t.Fatalf("after 1 call, calls = %d, want 1", h.calls)
	}
	if h.lastLang != "go" {
		t.Errorf("recorded language = %q, want %q", h.lastLang, "go")
	}
	if len(h.lastLines) != 2 || h.lastLines[0] != "ab" || h.lastLines[1] != "cd" {
		t.Errorf("recorded lines = %v, want [ab cd]", h.lastLines)
	}
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != (TextSpan{Start: 0, End: 2, Color: kw}) {
		t.Errorf("returned spans = %v, want the seeded span", got)
	}
	// The recorded lines are a copy: mutating the probe's record must not be
	// observable to the caller's slice, and a second call bumps the counter.
	h.Highlight("rb", []string{"x"}, nil)
	if h.calls != 2 {
		t.Fatalf("after 2 calls, calls = %d, want 2", h.calls)
	}
}

// --- construction ---------------------------------------------------------

func TestNewCodeEditorDefaults(t *testing.T) {
	c := NewCodeEditor("a\nb")
	if c.TextView == nil {
		t.Fatal("NewCodeEditor must build the embedded TextView")
	}
	if !c.ShowLineNumbers {
		t.Error("gutter (ShowLineNumbers) should default on")
	}
	if !c.HighlightCurrentLine {
		t.Error("current-line highlight should default on")
	}
	if c.Syntax != nil {
		t.Error("Syntax should default nil (core ships no lexer)")
	}
	if c.TextView.Highlighter == nil || c.TextView.RowBackground == nil {
		t.Error("constructor must wire the TextView Highlighter + RowBackground hooks")
	}
	if got := c.Text(); got != "a\nb" { // promoted method
		t.Errorf("promoted Text() = %q, want %q", got, "a\nb")
	}
}

// --- A11y participation ---------------------------------------------------

func TestCodeEditorA11y(t *testing.T) {
	c := NewCodeEditor("package main")
	c.Language = "go"
	got := c.A11y()
	want := A11yInfo{Role: RoleTextbox, Name: "go", Value: "package main"}
	if got != want {
		t.Errorf("A11y() = %+v, want %+v", got, want)
	}
	// It participates in the collected tree (not filtered as presentational).
	if infos := CollectA11y([]Widget{c}); len(infos) != 1 || infos[0] != want {
		t.Errorf("CollectA11y = %+v, want one %+v", infos, want)
	}
}

// --- highlight cache: control-driven re-lex accounting --------------------

func drawInto(c *CodeEditor, theme *Theme, w, h int) []byte {
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	c.Draw(newP(surf, w), theme)
	return surf
}

func TestCodeEditorCacheReuse(t *testing.T) {
	h := &countingHL{spans: [][]TextSpan{{}, {}}}
	c := NewCodeEditor("one\ntwo")
	c.Language = "go"
	c.Syntax = h
	th := DefaultLight()

	drawInto(c, th, 120, 80)
	if h.calls != 1 {
		t.Fatalf("first Draw: calls = %d, want 1", h.calls)
	}
	// Identical inputs on the next Draw: the cache holds, no re-lex.
	drawInto(c, th, 120, 80)
	if h.calls != 1 {
		t.Fatalf("second identical Draw: calls = %d, want 1 (cache hit)", h.calls)
	}
	// A cursor move changes no text, so still a cache hit.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	drawInto(c, th, 120, 80)
	if h.calls != 1 {
		t.Fatalf("Draw after cursor move: calls = %d, want 1 (text unchanged)", h.calls)
	}
	if h.lastLang != "go" {
		t.Errorf("highlighter saw language %q, want go", h.lastLang)
	}
}

func TestCodeEditorCacheInvalidation(t *testing.T) {
	h := &countingHL{spans: [][]TextSpan{{}, {}}}
	c := NewCodeEditor("one\ntwo")
	c.Syntax = h
	th := DefaultLight()

	drawInto(c, th, 120, 80) // calls -> 1
	// (1) buffer text changed -> re-lex.
	c.SetText("three\nfour")
	drawInto(c, th, 120, 80)
	if h.calls != 2 {
		t.Fatalf("after text change: calls = %d, want 2", h.calls)
	}
	// (2) language changed -> re-lex.
	c.Language = "ruby"
	drawInto(c, th, 120, 80)
	if h.calls != 3 {
		t.Fatalf("after language change: calls = %d, want 3", h.calls)
	}
	// (3) theme instance changed -> re-lex (colours may depend on it).
	drawInto(c, DefaultDark(), 120, 80)
	if h.calls != 4 {
		t.Fatalf("after theme change: calls = %d, want 4", h.calls)
	}
	// (4) highlighter instance swapped -> re-lex against the new one.
	h2 := &countingHL{spans: [][]TextSpan{{}, {}}}
	c.Syntax = h2
	drawInto(c, DefaultDark(), 120, 80)
	if h2.calls != 1 {
		t.Fatalf("after highlighter swap: new probe calls = %d, want 1", h2.calls)
	}
	// (5) Syntax cleared -> the nil branch, no probe touched, no panic.
	c.Syntax = nil
	drawInto(c, DefaultDark(), 120, 80)
	if h2.calls != 1 {
		t.Errorf("after clearing Syntax: probe calls = %d, want unchanged 1", h2.calls)
	}
}

// --- lineSpans indexing ---------------------------------------------------

func TestCodeEditorLineSpansBounds(t *testing.T) {
	kw := RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}
	// One row of spans for a three-line buffer: line 0 has spans, lines 1-2
	// index past the cached result and must yield nil (not panic).
	h := &countingHL{spans: [][]TextSpan{{{Start: 0, End: 1, Color: kw}}}}
	c := NewCodeEditor("a\nb\nc")
	c.Syntax = h
	drawInto(c, DefaultLight(), 120, 80) // populate the cache via Draw

	if got := c.lineSpans(0, "a"); len(got) != 1 || got[0].Color != kw {
		t.Errorf("lineSpans(0) = %v, want the seeded kw span", got)
	}
	if got := c.lineSpans(1, "b"); got != nil {
		t.Errorf("lineSpans(1) past cache = %v, want nil", got)
	}
	if got := c.lineSpans(-1, ""); got != nil {
		t.Errorf("lineSpans(-1) = %v, want nil", got)
	}
	// With Syntax nil the cache is empty and every row yields nil.
	c.Syntax = nil
	drawInto(c, DefaultLight(), 120, 80)
	if got := c.lineSpans(0, "a"); got != nil {
		t.Errorf("lineSpans(0) with nil Syntax = %v, want nil", got)
	}
}

// --- syntax colours reach exact pixels ------------------------------------

// colorBBox returns the pixel bounding box of every pixel equal to c, and how
// many there were. Used to assert coloured spans land on the correct row, not
// merely "somewhere".
func colorBBox(surf []byte, w, h int, c RGBA) (minX, minY, maxX, maxY, n int) {
	minX, minY = w, h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(surf, w, x, y) == c {
				n++
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return
}

func TestCodeEditorSyntaxPaintsSpanColor(t *testing.T) {
	kw := RGBA{R: 0x10, G: 0x90, B: 0xE0, A: 0xFF}
	// Colour the first three runes of line 0; leave line 1 uncoloured.
	h := &countingHL{spans: [][]TextSpan{
		{{Start: 0, End: 3, Color: kw}},
		nil,
	}}
	c := NewCodeEditor("abc\nxyz")
	c.Syntax = h
	th := DefaultLight()
	w, hgt := 140, 80
	surf := drawInto(c, th, w, hgt)

	minX, minY, maxX, maxY, n := colorBBox(surf, w, hgt, kw)
	if n == 0 {
		t.Fatal("keyword colour never painted")
	}
	lineH := c.glyphHeight() + 4
	adv := c.glyphAdvance()
	gutterW := c.textWidth("2") + 8 // two-line buffer -> widest number "2"
	textX := 4 + gutterW
	// Vertical bound: the coloured glyphs sit on row 0 only — every kw pixel
	// is above where row 1 begins (r.Y + 4 + lineH).
	if minY < 4 || maxY >= 4+lineH {
		t.Errorf("kw pixels span y [%d,%d], want within row 0 [4,%d)", minY, maxY, 4+lineH)
	}
	// Horizontal bound: the 3 coloured runes start at the text origin and end
	// within the first three glyph cells.
	if minX < textX || maxX >= textX+3*adv {
		t.Errorf("kw pixels span x [%d,%d], want within [%d,%d)", minX, maxX, textX, textX+3*adv)
	}
}

// --- current-line band ----------------------------------------------------

func TestCodeEditorCurrentLineBandExactPixels(t *testing.T) {
	band := RGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xFF}
	c := NewCodeEditor("first\nsecond\nthird")
	c.CurrentLineColor = band // pin an explicit, distinctive band colour
	th := DefaultLight()
	w, hgt := 160, 90
	surf := drawInto(c, th, w, hgt)

	lineH := c.glyphHeight() + 4
	probeX := w - 6 // blank band area, right of any glyph, left of the border
	// Cursor is on line 0: the band paints there.
	if got := pixelAt(surf, w, probeX, 4); got != band {
		t.Errorf("row-0 band pixel = %+v, want %+v", got, band)
	}
	// Line 1 is not the caret line: it stays on the plain Surface.
	if got := pixelAt(surf, w, probeX, 4+lineH+3); got == band {
		t.Errorf("row-1 pixel = %+v, must not be the band colour", got)
	}
	if got := pixelAt(surf, w, probeX, 4+lineH+3); got != th.Surface {
		t.Errorf("row-1 blank pixel = %+v, want Surface %+v", got, th.Surface)
	}

	// Move the caret down one line: the band follows it.
	c.CursorLine = 1
	surf = drawInto(c, th, w, hgt)
	if got := pixelAt(surf, w, probeX, 4); got == band {
		t.Errorf("row-0 pixel after caret moved to row 1 = %+v, must not be band", got)
	}
	if got := pixelAt(surf, w, probeX, 4+lineH); got != band {
		t.Errorf("row-1 band pixel after caret move = %+v, want %+v", got, band)
	}
}

func TestCodeEditorCurrentLineDisabled(t *testing.T) {
	band := RGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xFF}
	c := NewCodeEditor("only\nlines")
	c.CurrentLineColor = band
	c.HighlightCurrentLine = false // off -> no band anywhere
	th := DefaultLight()
	w, hgt := 120, 60
	surf := drawInto(c, th, w, hgt)
	if n := countInk(surf, w, hgt, band); n != 0 {
		t.Errorf("band painted %d px with HighlightCurrentLine off, want 0", n)
	}
}

func TestCodeEditorRowBackgroundHook(t *testing.T) {
	c := NewCodeEditor("x\ny")
	c.lastTheme = DefaultLight()

	// Not the caret line -> no band.
	if _, ok := c.rowBackground(1); ok {
		t.Error("rowBackground(non-caret line) should report no band")
	}
	// Caret line with an explicit colour -> that colour.
	custom := RGBA{R: 9, G: 8, B: 7, A: 0xFF}
	c.CurrentLineColor = custom
	if col, ok := c.rowBackground(0); !ok || col != custom {
		t.Errorf("rowBackground(caret) = (%+v,%v), want (%+v,true)", col, ok, custom)
	}
	// Caret line with an unset colour -> the theme-derived tint.
	c.CurrentLineColor = RGBA{}
	wantTint := blendRGBA(DefaultLight().SurfaceAlt, DefaultLight().Surface, 0.5)
	if col, ok := c.rowBackground(0); !ok || col != wantTint {
		t.Errorf("rowBackground default tint = (%+v,%v), want (%+v,true)", col, ok, wantTint)
	}
}

func TestCodeEditorCurrentLineTintNilTheme(t *testing.T) {
	// currentLineTint before the first Draw (lastTheme still nil) must fall
	// back to the light theme rather than dereference nil.
	c := NewCodeEditor("z")
	if c.lastTheme != nil {
		t.Fatal("precondition: lastTheme should be nil before Draw")
	}
	want := blendRGBA(DefaultLight().SurfaceAlt, DefaultLight().Surface, 0.5)
	if got := c.currentLineTint(); got != want {
		t.Errorf("currentLineTint(nil theme) = %+v, want %+v", got, want)
	}
}
