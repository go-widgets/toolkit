// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// MarkdownView renders a subset of Markdown as laid-out text -- the read-only
// document widget the toolkit lacked. It handles the block structure that
// matters on a fixed-width bitmap font: ATX headings (`#`..`######`), bullet
// lists (`-`/`*`/`+`), fenced code blocks (```), and blank-line-separated
// paragraphs, word-wrapped to the widget width.
//
// The 5x7 font has a single weight and size, so hierarchy is shown through
// layout rather than type scale: headings get an Accent underline (levels 1-2)
// and the Accent ink, bullets get a "• " marker with a hanging indent, and code
// blocks sit on a SurfaceAlt band. Inline emphasis (*bold*, _italic_) is not
// styled -- the bitmap font has no variants -- but its text still renders.
// Display-only; wrap in a ScrollView for long documents.
type MarkdownView struct {
	Base
	Source string
}

// NewMarkdownView builds a MarkdownView over the given Markdown source.
func NewMarkdownView(source string) *MarkdownView { return &MarkdownView{Source: source} }

// mdKind is a parsed block's type.
type mdKind int

const (
	mdParagraph mdKind = iota
	mdHeading
	mdBullet
	mdCode
)

// mdBlock is one parsed block. For mdHeading, level is 1..6; for mdCode, text
// holds the fenced lines joined by "\n".
type mdBlock struct {
	kind  mdKind
	level int
	text  string
}

// parseMarkdown splits source into block-level elements. Fenced code spans
// (delimited by a line whose trimmed form is "```") capture their lines
// verbatim; an unclosed fence runs to the end of input.
func parseMarkdown(source string) []mdBlock {
	var blocks []mdBlock
	lines := strings.Split(source, "\n")
	inCode := false
	var code []string
	flushCode := func() {
		blocks = append(blocks, mdBlock{kind: mdCode, text: strings.Join(code, "\n")})
		code = nil
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "```" {
			if inCode {
				flushCode()
			}
			inCode = !inCode
			continue
		}
		if inCode {
			code = append(code, ln)
			continue
		}
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue // blank line: block separator
		}
		if lvl, htext, ok := parseHeading(trimmed); ok {
			blocks = append(blocks, mdBlock{kind: mdHeading, level: lvl, text: htext})
			continue
		}
		if btext, ok := parseBullet(trimmed); ok {
			blocks = append(blocks, mdBlock{kind: mdBullet, text: btext})
			continue
		}
		blocks = append(blocks, mdBlock{kind: mdParagraph, text: trimmed})
	}
	if inCode {
		flushCode()
	}
	return blocks
}

// parseHeading returns the level (1..6) and text of an ATX heading line, or
// ok=false. Requires a space after the run of '#' (per CommonMark).
func parseHeading(s string) (level int, text string, ok bool) {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[n+1:]), true
}

// parseBullet returns the item text of a "- "/"* "/"+ " list line, or ok=false.
func parseBullet(s string) (text string, ok bool) {
	if len(s) < 2 || (s[0] != '-' && s[0] != '*' && s[0] != '+') || s[1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(s[2:]), true
}

// wordWrap greedily breaks text into lines of at most maxChars columns, never
// splitting a word. maxChars <= 0 (or a single over-long word) yields one line.
func wordWrap(text string, maxChars int) []string {
	words := strings.Fields(text)
	if maxChars <= 0 || len(words) == 0 {
		return []string{text}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) <= maxChars {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	return append(lines, cur)
}

// mdLineH is the baseline-to-baseline advance for a rendered text line.
func mdLineH() int { return GlyphHeight() + 4 }

// Draw lays the parsed blocks top-to-bottom within Bounds.
func (m *MarkdownView) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	maxChars := (r.W - 8) / GlyphAdvance()
	y := r.Y + 4
	for _, b := range parseMarkdown(m.Source) {
		y = m.drawBlock(p, theme, b, r, maxChars, y)
	}
}

// drawBlock renders one block starting at y and returns the next y cursor.
func (m *MarkdownView) drawBlock(p painter.Painter, theme *Theme, b mdBlock, r Rect, maxChars, y int) int {
	switch b.kind {
	case mdHeading:
		DrawText(p, r.X+4, y, b.text, theme.Accent)
		if b.level <= 2 { // rule under H1/H2
			fillRect(p, r.X+4, y+GlyphHeight()+1, TextWidth(b.text), 1, theme.Accent)
			y += 2
		}
		return y + mdLineH() + 2
	case mdBullet:
		DrawText(p, r.X+4, y, "•", theme.OnSurface) // "•"
		for i, ln := range wordWrap(b.text, maxChars-2) {
			DrawText(p, r.X+4+2*GlyphAdvance(), y+i*mdLineH(), ln, theme.OnSurface)
		}
		return y + len(wordWrap(b.text, maxChars-2))*mdLineH()
	case mdCode:
		codeLines := strings.Split(b.text, "\n")
		bandH := len(codeLines) * mdLineH()
		fillRect(p, r.X+4, y-2, r.W-8, bandH+4, theme.SurfaceAlt)
		for i, ln := range codeLines {
			DrawText(p, r.X+8, y+i*mdLineH(), ln, theme.OnSurface)
		}
		return y + bandH + 4
	default: // mdParagraph
		wrapped := wordWrap(b.text, maxChars)
		for i, ln := range wrapped {
			DrawText(p, r.X+4, y+i*mdLineH(), ln, theme.OnSurface)
		}
		return y + len(wrapped)*mdLineH() + 2
	}
}
