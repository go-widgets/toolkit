// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package rougelex

import (
	"testing"

	"github.com/go-rouge/rouge"
	"github.com/go-widgets/toolkit"
)

// distinctPalette gives every category a unique, alpha-opaque colour so a
// resolved span's colour identifies exactly which category matched.
func distinctPalette() Palette {
	c := func(n uint8) toolkit.RGBA { return toolkit.RGBA{R: n, G: n, B: n, A: 0xFF} }
	return Palette{
		Default:     c(1),
		Keyword:     c(2),
		Type:        c(3),
		Function:    c(4),
		Class:       c(5),
		Builtin:     c(6),
		Tag:         c(12),
		Variable:    c(13),
		Attribute:   c(14),
		String:      c(7),
		Number:      c(8),
		Comment:     c(9),
		Operator:    c(10),
		Punctuation: c(11),
	}
}

// --- control: colorFor resolves each category to its designated colour -----
//
// This is the known-good/known-bad control for the span colouring: with a
// palette whose colours are all distinct, feeding colorFor each canonical
// rouge token must return exactly that category's colour (known-good), and a
// token in no mapped category must fall through to Default (known-bad for
// every specific case). Only once this holds do the whole-buffer Highlight
// assertions — which read colours back off spans — mean anything.
func TestColorForCategories(t *testing.T) {
	p := distinctPalette()
	cases := []struct {
		tok  *rouge.Token
		want toolkit.RGBA
		name string
	}{
		{rouge.Comment, p.Comment, "comment"},
		{rouge.CommentSingle, p.Comment, "comment.single (descendant)"},
		{rouge.LiteralString, p.String, "string"},
		{rouge.LiteralStringDouble, p.String, "string.double (descendant)"},
		{rouge.LiteralNumber, p.Number, "number"},
		{rouge.NameFunction, p.Function, "name.function"},
		{rouge.NameClass, p.Class, "name.class"},
		{rouge.NameBuiltin, p.Builtin, "name.builtin"},
		{rouge.NameTag, p.Tag, "name.tag"},
		{rouge.NameVariable, p.Variable, "name.variable"},
		{rouge.NameVariableGlobal, p.Variable, "name.variable.global (descendant)"},
		{rouge.NameAttribute, p.Attribute, "name.attribute"},
		{rouge.KeywordType, p.Type, "keyword.type"},
		{rouge.Keyword, p.Keyword, "keyword"},
		{rouge.KeywordNamespace, p.Keyword, "keyword.namespace (descendant)"},
		{rouge.Operator, p.Operator, "operator"},
		{rouge.Punctuation, p.Punctuation, "punctuation"},
		{rouge.Text, p.Default, "text -> default"},
		{rouge.Name, p.Default, "plain name -> default"},
	}
	for _, c := range cases {
		if got := p.colorFor(c.tok); got != c.want {
			t.Errorf("%s: colorFor = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestNewIsZeroPalette(t *testing.T) {
	if got := New(); got.Palette != (Palette{}) {
		t.Errorf("New().Palette = %+v, want zero", got.Palette)
	}
}

func TestDefaultPaletteDerivesFromTheme(t *testing.T) {
	th := toolkit.DefaultDark()
	p := DefaultPalette(th)
	if p == (Palette{}) {
		t.Fatal("DefaultPalette must not be the zero (derive) sentinel")
	}
	if p.Default != th.OnSurface {
		t.Errorf("Default = %+v, want OnSurface %+v", p.Default, th.OnSurface)
	}
	if p.Keyword != th.Accent {
		t.Errorf("Keyword = %+v, want Accent %+v", p.Keyword, th.Accent)
	}
	if want := blend(th.OnSurface, th.Surface, 0.45); p.Comment != want {
		t.Errorf("Comment = %+v, want %+v", p.Comment, want)
	}
	if want := blend(th.Accent, th.OnSurface, 0.4); p.Function != want {
		t.Errorf("Function = %+v, want %+v", p.Function, want)
	}
	if want := blend(th.Accent, th.OnSurface, 0.25); p.Tag != want {
		t.Errorf("Tag = %+v, want %+v", p.Tag, want)
	}
	if want := blend(th.Accent, th.OnSurface, 0.4); p.Variable != want {
		t.Errorf("Variable = %+v, want %+v", p.Variable, want)
	}
	if want := blend(th.OnSurface, th.Accent, 0.5); p.Attribute != want {
		t.Errorf("Attribute = %+v, want %+v", p.Attribute, want)
	}
}

// spanColor returns the colour of the span covering rune position pos on the
// given line, or a zero RGBA if none covers it.
func spanColor(rows [][]toolkit.TextSpan, line, pos int) toolkit.RGBA {
	if line < 0 || line >= len(rows) {
		return toolkit.RGBA{}
	}
	for _, s := range rows[line] {
		if pos >= s.Start && pos < s.End {
			return s.Color
		}
	}
	return toolkit.RGBA{}
}

// --- control: plaintext maps to full-line default spans --------------------

func TestHighlightPlainTextControl(t *testing.T) {
	h := &Highlighter{Palette: distinctPalette()}
	rows := h.Highlight("text", []string{"ab", "cde"}, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// The no-op lexer yields one Default-coloured span per line, spanning the
	// whole line in rune coordinates — the baseline that proves the
	// line/column mapping before any real lexer is trusted.
	want := distinctPalette().Default
	if len(rows[0]) != 1 || rows[0][0] != (toolkit.TextSpan{Start: 0, End: 2, Color: want}) {
		t.Errorf("line 0 spans = %v, want one [0,2) default", rows[0])
	}
	if len(rows[1]) != 1 || rows[1][0] != (toolkit.TextSpan{Start: 0, End: 3, Color: want}) {
		t.Errorf("line 1 spans = %v, want one [0,3) default", rows[1])
	}
}

func TestHighlightGoKeywordPosition(t *testing.T) {
	h := &Highlighter{Palette: distinctPalette()}
	rows := h.Highlight("go", []string{"package main"}, nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	kw := distinctPalette().Keyword
	// "package" (runes 0..6) must colour as a keyword; the space + "main" that
	// follow must not.
	for pos := 0; pos < 7; pos++ {
		if got := spanColor(rows, 0, pos); got != kw {
			t.Fatalf("rune %d of %q: colour %+v, want keyword %+v", pos, "package main", got, kw)
		}
	}
	if got := spanColor(rows, 0, 8); got == kw {
		t.Errorf("rune 8 ('m' of main) coloured as keyword, want not")
	}
	// The spans reconstruct the full 12-rune line without a gap.
	if got := spanColor(rows, 0, 11); got == (toolkit.RGBA{}) {
		t.Error("last rune of the line has no span (gap in coverage)")
	}
}

func TestHighlightMultiLineAndBlankLine(t *testing.T) {
	// A blank middle line exercises both the line-advance (i>0) path and the
	// empty-segment (n==0 continue) path of the span mapper.
	h := &Highlighter{Palette: distinctPalette()}
	rows := h.Highlight("text", []string{"a", "", "b"}, nil)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	def := distinctPalette().Default
	if len(rows[0]) != 1 || rows[0][0].Color != def || rows[0][0].End != 1 {
		t.Errorf("line 0 = %v, want one [0,1) default", rows[0])
	}
	if len(rows[1]) != 0 {
		t.Errorf("blank line 1 = %v, want no spans", rows[1])
	}
	if len(rows[2]) != 1 || rows[2][0].Color != def || rows[2][0].End != 1 {
		t.Errorf("line 2 = %v, want one [0,1) default", rows[2])
	}
}

func TestHighlightUnknownLanguageFallsBackToPlain(t *testing.T) {
	// An unknown language name resolves no lexer and must fall back to plain
	// text (no colouring, no panic) rather than returning nothing.
	h := New() // zero palette -> theme-derived
	th := toolkit.DefaultLight()
	rows := h.Highlight("no-such-language", []string{"hello"}, th)
	if len(rows) != 1 || len(rows[0]) != 1 {
		t.Fatalf("rows = %v, want one line with one span", rows)
	}
	if rows[0][0] != (toolkit.TextSpan{Start: 0, End: 5, Color: th.OnSurface}) {
		t.Errorf("fallback span = %v, want [0,5) OnSurface", rows[0][0])
	}
}

func TestHighlightZeroPaletteUsesTheme(t *testing.T) {
	// With a zero-value palette the colours come from the theme; a custom
	// palette bypasses the theme entirely.
	th := toolkit.DefaultLight()
	zeroRows := New().Highlight("go", []string{"package x"}, th)
	if spanColor(zeroRows, 0, 0) != th.Accent {
		t.Errorf("zero palette keyword = %+v, want theme Accent %+v", spanColor(zeroRows, 0, 0), th.Accent)
	}
	custom := &Highlighter{Palette: distinctPalette()}
	customRows := custom.Highlight("go", []string{"package x"}, th)
	if spanColor(customRows, 0, 0) != distinctPalette().Keyword {
		t.Errorf("custom palette keyword = %+v, want custom Keyword", spanColor(customRows, 0, 0))
	}
}

// --- LaTeX: the Name subtypes the TeX lexer leans on -----------------------
//
// The rouge TeX/LaTeX lexer emits Name.Tag for \begin{env}/\end{env}, Keyword
// for a command, Name.Variable for a math control sequence, Name.Attribute for
// a command's optional [..] argument, Comment for a % comment and Punctuation
// for math delimiters. Each must resolve to its own palette field (not the
// plain-name Default), so a LaTeX buffer is fully coloured.
func TestHighlightLatexNameSubtypes(t *testing.T) {
	p := distinctPalette()
	h := &Highlighter{Palette: p}
	// \begin{doc} -> Name.Tag, whole match at rune 0.
	if got := spanColor(h.Highlight("latex", []string{`\begin{doc}`}, nil), 0, 0); got != p.Tag {
		t.Errorf(`\begin{doc}: colour %+v, want Tag %+v`, got, p.Tag)
	}
	// \textbf -> Keyword (a command).
	if got := spanColor(h.Highlight("latex", []string{`\textbf`}, nil), 0, 0); got != p.Keyword {
		t.Errorf(`\textbf: colour %+v, want Keyword %+v`, got, p.Keyword)
	}
	// $\a$ -> Punctuation delimiters around a Name.Variable control sequence.
	mv := h.Highlight("latex", []string{`$\a$`}, nil)
	if got := spanColor(mv, 0, 0); got != p.Punctuation {
		t.Errorf(`$ delimiter: colour %+v, want Punctuation %+v`, got, p.Punctuation)
	}
	if got := spanColor(mv, 0, 2); got != p.Variable {
		t.Errorf(`\a math: colour %+v, want Variable %+v`, got, p.Variable)
	}
	// \includegraphics[opt] -> Keyword then Name.Attribute for the [..] arg.
	attr := h.Highlight("latex", []string{`\includegraphics[opt]`}, nil)
	if got := spanColor(attr, 0, 16); got != p.Attribute { // '[' starts at rune 16
		t.Errorf(`[opt] attribute: colour %+v, want Attribute %+v`, got, p.Attribute)
	}
	// % comment -> Comment.
	if got := spanColor(h.Highlight("latex", []string{`% note`}, nil), 0, 0); got != p.Comment {
		t.Errorf(`%% comment: colour %+v, want Comment %+v`, got, p.Comment)
	}
}

// --- named palettes: registry, ThemeNames, PaletteByName -------------------

func TestThemeNamesOrder(t *testing.T) {
	// "Default" (theme-derived) must be first, then the fixed schemes, in the
	// order a picker presents them.
	want := []string{"Default", "Monokai", "Solarized", "GitHub", "Dracula"}
	got := ThemeNames()
	if len(got) != len(want) {
		t.Fatalf("ThemeNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ThemeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPaletteByNameDefaultIsZero(t *testing.T) {
	// "Default" resolves (ok==true) to the zero Palette, so a Highlighter set
	// to it stays theme-derived exactly like the zero value.
	p, ok := PaletteByName("Default")
	if !ok {
		t.Fatal("PaletteByName(\"Default\") not found")
	}
	if p != (Palette{}) {
		t.Errorf("Default palette = %+v, want zero (theme-derived)", p)
	}
}

func TestPaletteByNameUnknown(t *testing.T) {
	// An unknown name returns the zero Palette and ok==false.
	if p, ok := PaletteByName("no-such-scheme"); ok || p != (Palette{}) {
		t.Errorf("unknown scheme: got (%+v, %v), want (zero, false)", p, ok)
	}
}

// TestNamedSchemesColourLatexTokens Highlights a LaTeX snippet under every
// shared scheme and asserts a Tag (\begin{env}), a Keyword (command) and a
// Variable (math \a) each get that scheme's own colour. The theme-derived
// "Default" scheme instead follows the theme (DefaultPalette).
func TestNamedSchemesColourLatexTokens(t *testing.T) {
	th := toolkit.DefaultDark()
	for _, name := range ThemeNames() {
		pal, ok := PaletteByName(name)
		if !ok {
			t.Fatalf("PaletteByName(%q) not found", name)
		}
		want := pal
		if pal == (Palette{}) { // "Default": follow the theme.
			want = DefaultPalette(th)
		}
		h := &Highlighter{Palette: pal}
		if got := spanColor(h.Highlight("latex", []string{`\begin{doc}`}, th), 0, 0); got != want.Tag {
			t.Errorf("%s: \\begin tag = %+v, want Tag %+v", name, got, want.Tag)
		}
		if got := spanColor(h.Highlight("latex", []string{`\textbf`}, th), 0, 0); got != want.Keyword {
			t.Errorf("%s: \\textbf command = %+v, want Keyword %+v", name, got, want.Keyword)
		}
		if got := spanColor(h.Highlight("latex", []string{`$\a$`}, th), 0, 2); got != want.Variable {
			t.Errorf("%s: math var = %+v, want Variable %+v", name, got, want.Variable)
		}
	}
}
