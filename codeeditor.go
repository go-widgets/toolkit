// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Highlighter turns a source buffer into per-line coloured spans. It is the
// pluggable seam a CodeEditor uses for syntax highlighting, so the toolkit
// core carries no lexer of its own: a consumer supplies a rouge-backed
// implementation (github.com/go-widgets/toolkit/rougelex) — or any other —
// and importing the core toolkit never pulls a highlighting engine in.
//
// Highlight receives the WHOLE buffer (as lines) rather than one line at a
// time, so a multi-line construct — a block comment, a heredoc, a triple
// quoted string — is coloured correctly across the lines it spans. The
// returned slice is indexed by line: element i holds the spans covering
// lines[i] in that line's rune coordinates ([Start, End), the same half-open
// convention as TextSpan). An implementation returns one entry per input line
// (len(result) == len(lines)); CodeEditor tolerates a short or long result by
// treating a missing row as "no spans".
type Highlighter interface {
	Highlight(language string, lines []string, theme *Theme) [][]TextSpan
}

// CodeEditor is a multi-language source editor: a TextView (the editing model
// — lines, cursor, insert / split / backspace, undo/redo, selection, IME,
// scrolling) enriched with a line-number gutter, pluggable syntax highlighting
// and a current-line highlight. It is the one shared widget every wasmdesk
// code surface builds on (the wasmbox "code" client, go-loom, the reader
// source-preview) so they converge on a single implementation instead of each
// re-wiring a TextView by hand.
//
// It embeds *TextView, so the whole editing API is available directly on a
// CodeEditor (Text, SetText, OnEvent, Undo, Lines, CursorLine, …); Draw is
// overridden to refresh the highlight cache, wire the gutter + current-line
// band, and paint through the embedded view.
type CodeEditor struct {
	*TextView

	// Language is the lexer hint handed to Syntax.Highlight (e.g. "go",
	// "ruby", "python"). An empty string lets the Highlighter decide
	// (guess / leave plain). Changing it re-lexes on the next Draw.
	Language string

	// Syntax is the pluggable highlighter. When nil (the zero value) the
	// buffer is painted in the theme's default ink exactly like a bare
	// TextView — the core toolkit ships no lexer, so a CodeEditor is
	// uncoloured until a consumer sets this to e.g. rougelex.New().
	Syntax Highlighter

	// HighlightCurrentLine paints a full-width tint behind the caret's
	// line. NewCodeEditor enables it; the zero value (a struct literal
	// built without the constructor) leaves it off.
	HighlightCurrentLine bool

	// CurrentLineColor overrides the current-line band colour. Its zero
	// value (A == 0, "unset") derives a subtle, theme-safe tint from the
	// theme passed to Draw.
	CurrentLineColor RGBA

	// MatchColor overrides the soft search-match highlight colour (the band
	// behind every occurrence — see SetMatchHighlights). Its zero value (A == 0)
	// derives a faint accent wash from the theme passed to Draw.
	MatchColor RGBA

	// CurrentMatchColor overrides the current-match highlight fill (behind the
	// range set via SetCurrentMatch, under an accent outline box). Its zero value
	// (A == 0) derives a stronger accent wash from the theme.
	CurrentMatchColor RGBA

	// matchRanges are the soft-highlight occurrences and currentMatch the
	// emphasised one, pushed by a search host (see matchhighlight.go); Draw
	// resolves them to the TextView's paint-time bands against the live theme.
	matchRanges  []Selection
	currentMatch Selection

	// highlight cache: the inputs the last Highlight() ran against and the
	// per-line spans it produced, so Draw re-lexes only when the buffer,
	// language, highlighter or theme actually changes — a cursor move
	// (which never alters Text) reuses the cached spans.
	cacheKey   string
	cacheHL    Highlighter
	cacheTheme *Theme
	cacheSpans [][]TextSpan
	cached     bool

	// lastTheme is the theme of the most recent Draw, consulted by the
	// current-line band's tint when CurrentLineColor is unset.
	lastTheme *Theme

	// --- IntelliSense-style autocompletion (see completion.go) -------------

	// CompletionSource, when non-nil, supplies the candidate list for the
	// caret's context: the editor hands it the whole buffer + the caret
	// (line, col) and it returns LSP-shaped CompletionItems. The WIDGET then
	// prefix-filters those against the word before the caret and drives the
	// popup — so a host feeds it from a language server (loom) or from a
	// static list (the go-tex playground) without re-implementing the UI. A
	// host that filters server-side returns already-filtered items (and sets
	// FilterText to match). Nil (the zero value) disables completion
	// entirely: the editor behaves exactly like a plain highlighted view.
	CompletionSource func(doc []string, line, col int) []CompletionItem

	// CompletionWordChar decides which runes form the "current word" the
	// popup filters against (and which typed rune opens/refreshes it). Nil
	// uses defaultWordChar (letters, digits, underscore) — the identifier
	// rule for code. A LaTeX host sets it to also admit '\\' so a command
	// like "\section" filters as one word.
	CompletionWordChar func(r rune) bool

	// compOpen (popup shown?) and compSel (highlighted row within the
	// filtered list) are the reactive popup state, MVVM-only: they live in
	// unexported Observables exposed via [CodeEditor.CompletionOpen] and
	// [CodeEditor.CompletionSelection], so there is no settable field to
	// drift out of the reactive layer (the DropDown/ListBox convention).
	compOpen *mvvm.Observable[bool]
	compSel  *mvvm.Observable[int]

	// compItems is the current filtered candidate list (what the popup
	// renders and CompletionItems returns); compWordStart is the column the
	// word being completed begins at, so accept replaces [compWordStart,
	// CursorCol); compScroll is the top visible row of the popup list (a
	// plain offset, like DropDown.popScroll), clamped on read.
	compItems     []CompletionItem
	compWordStart int
	compScroll    int
}

// NewCodeEditor builds a CodeEditor pre-loaded with initial source (split on
// "\n", empty yields a single empty line, per NewTextView). The line-number
// gutter and current-line highlight are on by default; Syntax is nil until a
// caller plugs a highlighter in.
func NewCodeEditor(initial string) *CodeEditor {
	c := &CodeEditor{TextView: NewTextView(initial)}
	c.ShowLineNumbers = true
	c.HighlightCurrentLine = true
	c.TextView.Highlighter = c.lineSpans
	c.TextView.RowBackground = c.rowBackground
	return c
}

// Draw refreshes the highlight cache and paints the editor through the
// embedded TextView (which draws the gutter, the current-line band via the
// wired RowBackground hook, and the coloured text via the wired Highlighter
// hook).
func (c *CodeEditor) Draw(p painter.Painter, theme *Theme) {
	c.lastTheme = theme
	c.refresh(theme)
	c.buildMatchBands(theme)
	c.TextView.Draw(p, theme)
	// The completion popup floats over the editor at the caret; drawn last so
	// it sits on top of the text it overlays. A no-op while the popup is closed.
	c.drawCompletion(p, theme)
}

// A11y reports the editor as a textbox whose accessible name is the language
// (a hint to assistive tech about what is being edited) and whose value is the
// current buffer text. It shadows the promoted TextView.A11y so a screen
// reader hears the code editor, not a bare textbox.
func (c *CodeEditor) A11y() A11yInfo {
	return A11yInfo{Role: RoleTextbox, Name: c.Language, Value: c.TextView.Text().Get()}
}

// refresh re-runs the highlighter when (and only when) an input it depends on
// changed since the last run: the buffer text, the language, the highlighter
// instance or the theme. With Syntax nil there is nothing to cache and the
// buffer paints in the default ink.
func (c *CodeEditor) refresh(theme *Theme) {
	if c.Syntax == nil {
		c.cacheSpans = nil
		c.cached = false
		return
	}
	key := c.Language + "\x00" + c.TextView.Text().Get()
	if c.cached && c.Syntax == c.cacheHL && theme == c.cacheTheme && key == c.cacheKey {
		return
	}
	c.cacheSpans = c.Syntax.Highlight(c.Language, c.TextView.lines, theme)
	c.cacheKey = key
	c.cacheHL = c.Syntax
	c.cacheTheme = theme
	c.cached = true
}

// lineSpans is wired into TextView.Highlighter: it returns the cached spans
// for lineIndex, or nil when no spans exist for that row (Syntax unset, or an
// index outside the cached result).
func (c *CodeEditor) lineSpans(lineIndex int, _ string) []TextSpan {
	if lineIndex < 0 || lineIndex >= len(c.cacheSpans) {
		return nil
	}
	return c.cacheSpans[lineIndex]
}

// rowBackground is wired into TextView.RowBackground: it paints the band only
// behind the caret's line, and only when HighlightCurrentLine is on. The band
// colour is CurrentLineColor, or a theme-derived tint when that is unset.
func (c *CodeEditor) rowBackground(lineIndex int) (RGBA, bool) {
	if !c.HighlightCurrentLine || lineIndex != c.TextView.CursorLine().Get() {
		return RGBA{}, false
	}
	col := c.CurrentLineColor
	if col.A == 0 {
		col = c.currentLineTint()
	}
	return col, true
}

// currentLineTint is the default current-line band colour: the theme's
// SurfaceAlt blended halfway to its Surface — a faint, opaque wash that reads
// as "this row" under both light and dark themes without fighting the ink. It
// falls back to the light theme when Draw has not run yet (a direct caller of
// rowBackground before the first paint).
func (c *CodeEditor) currentLineTint() RGBA {
	th := c.lastTheme
	if th == nil {
		th = DefaultLight()
	}
	return blendRGBA(th.SurfaceAlt, th.Surface, 0.5)
}
