// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package rougelex is the batteries-included syntax highlighter for
// github.com/go-widgets/toolkit's CodeEditor, backed by the pure-Go
// github.com/go-rouge/rouge lexers (Go, Ruby, Python, JavaScript, CSS, HTML,
// JSON, YAML, SQL, shell, Markdown, LaTeX, diff, …).
//
// It lives in its own module so importing the core toolkit never pulls a
// lexing engine — and its transitive regexp dependency — into a consumer that
// only wants buttons and boxes. A code surface opts in with a single import:
//
//	ed := toolkit.NewCodeEditor(src)
//	ed.Language = "go"
//	ed.Syntax = rougelex.New()
//
// The highlighter maps rouge's token stream onto a small Palette of ink
// colours. The zero-value Palette derives its colours from the theme handed to
// Highlight, so an editor colours itself consistently with the surrounding UI;
// set an explicit Palette to pin a fixed scheme. A set of shared named schemes
// (Monokai, Solarized, GitHub, Dracula, plus the theme-derived "Default") is
// available via ThemeNames and PaletteByName for a scheme picker:
//
//	pal, _ := rougelex.PaletteByName("Monokai")
//	ed.Syntax = &rougelex.Highlighter{Palette: pal}
package rougelex

import (
	"strings"
	"unicode/utf8"

	"github.com/go-rouge/rouge"
	"github.com/go-widgets/toolkit"
)

// Palette maps coarse syntactic categories to ink colours. A rouge token is
// resolved to one of these via its category ancestry (a Name.Function token
// takes Function, a Literal.String.Double takes String, and so on); anything
// unmatched takes Default.
//
// Tag, Variable and Attribute cover the Name subtypes that markup, templating
// and LaTeX lean on: Name.Tag is an HTML/XML element and a LaTeX
// \begin{env}/\end{env}; Name.Variable is a template/shell variable and a
// LaTeX math control sequence (\alpha); Name.Attribute is an HTML attribute and
// a LaTeX command's optional argument ([width=1cm]).
//
// The zero value (every field's alpha 0) is the sentinel for "derive from the
// theme": Highlight then fills the palette with DefaultPalette(theme).
type Palette struct {
	Default     toolkit.RGBA
	Keyword     toolkit.RGBA
	Type        toolkit.RGBA
	Function    toolkit.RGBA
	Class       toolkit.RGBA
	Builtin     toolkit.RGBA
	Tag         toolkit.RGBA
	Variable    toolkit.RGBA
	Attribute   toolkit.RGBA
	String      toolkit.RGBA
	Number      toolkit.RGBA
	Comment     toolkit.RGBA
	Operator    toolkit.RGBA
	Punctuation toolkit.RGBA
}

// DefaultPalette derives a theme-consistent palette from a toolkit theme:
// keywords take the accent, comments a dimmed ink, strings and numbers an
// ink/accent blend, and plain names the ordinary ink. Tags and variables read
// as structural, so they lean toward the accent; attributes lean toward the
// ink, like strings. It is what Highlight uses when the Highlighter's Palette
// is the zero value.
func DefaultPalette(theme *toolkit.Theme) Palette {
	ink := theme.OnSurface
	accent := theme.Accent
	return Palette{
		Default:     ink,
		Keyword:     accent,
		Type:        blend(accent, ink, 0.25),
		Function:    blend(accent, ink, 0.4),
		Class:       blend(accent, ink, 0.25),
		Builtin:     blend(accent, ink, 0.4),
		Tag:         blend(accent, ink, 0.25),
		Variable:    blend(accent, ink, 0.4),
		Attribute:   blend(ink, accent, 0.5),
		String:      blend(ink, accent, 0.5),
		Number:      blend(ink, accent, 0.5),
		Comment:     blend(ink, theme.Surface, 0.45),
		Operator:    ink,
		Punctuation: ink,
	}
}

// colorFor resolves a rouge token to a palette colour, most-specific category
// first (KeywordType before Keyword, Name.Function/Class/Builtin/Tag/Variable/
// Attribute before the plain-name Default).
func (p Palette) colorFor(t *rouge.Token) toolkit.RGBA {
	switch {
	case t.Matches(rouge.Comment):
		return p.Comment
	case t.Matches(rouge.LiteralString):
		return p.String
	case t.Matches(rouge.LiteralNumber):
		return p.Number
	case t.Matches(rouge.NameFunction):
		return p.Function
	case t.Matches(rouge.NameClass):
		return p.Class
	case t.Matches(rouge.NameBuiltin):
		return p.Builtin
	case t.Matches(rouge.NameTag):
		return p.Tag
	case t.Matches(rouge.NameVariable):
		return p.Variable
	case t.Matches(rouge.NameAttribute):
		return p.Attribute
	case t.Matches(rouge.KeywordType):
		return p.Type
	case t.Matches(rouge.Keyword):
		return p.Keyword
	case t.Matches(rouge.Operator):
		return p.Operator
	case t.Matches(rouge.Punctuation):
		return p.Punctuation
	default:
		return p.Default
	}
}

// Highlighter is a rouge-backed toolkit.Highlighter. The zero value is usable
// (equivalent to New()) and colours against the theme; set Palette to pin a
// fixed scheme.
type Highlighter struct {
	// Palette overrides the token colours. When it is the zero value the
	// colours are derived from the theme passed to Highlight.
	Palette Palette
}

// New returns a theme-derived rouge highlighter.
func New() *Highlighter { return &Highlighter{} }

// Highlight tokenises the buffer with the rouge lexer named by language (an
// unknown or empty name falls back to plain text, i.e. no colouring) and maps
// the token stream to per-line TextSpans. Multi-line tokens are split at line
// breaks so each line receives the spans that fall within it, in that line's
// rune coordinates.
func (h *Highlighter) Highlight(language string, lines []string, theme *toolkit.Theme) [][]toolkit.TextSpan {
	pal := h.Palette
	if pal == (Palette{}) {
		pal = DefaultPalette(theme)
	}
	out := make([][]toolkit.TextSpan, len(lines))
	lex := rouge.FindLexer(language)
	if lex == nil {
		lex = rouge.FindLexer("plaintext")
	}
	line, col := 0, 0
	for _, tv := range lex.Lex(strings.Join(lines, "\n")) {
		c := pal.colorFor(tv.Token)
		for i, seg := range strings.Split(tv.Value, "\n") {
			if i > 0 {
				// Crossed a line break inside this token's value.
				line++
				col = 0
			}
			n := utf8.RuneCountInString(seg)
			if n == 0 {
				continue
			}
			if line < len(out) {
				out[line] = append(out[line], toolkit.TextSpan{Start: col, End: col + n, Color: c})
			}
			col += n
		}
	}
	return out
}

// blend linearly interpolates a toward b by 1-t (t == 1 is all a), returning an
// opaque colour. A local copy of the toolkit's own blend so rougelex needs no
// exported blend helper from the core.
func blend(a, b toolkit.RGBA, t float64) toolkit.RGBA {
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*t + float64(y)*(1-t) + 0.5) }
	return toolkit.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}
