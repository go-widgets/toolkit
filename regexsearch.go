// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// SearchOptions tunes how [FindMatches] interprets a pattern. The zero value
// treats the pattern as a plain regular expression, case-sensitively, with no
// word-boundary constraint — the "regex query field" default a [FindReplace]
// panel exposes.
type SearchOptions struct {
	// Regex, when false, treats the pattern as a LITERAL string (every
	// metacharacter is quoted before compiling), so a user typing "a.b" matches
	// the three characters a-dot-b rather than "a<any>b". When true (the default
	// a find/replace bar exposes) the pattern is a Go regular expression.
	Regex bool
	// CaseSensitive, when false (the default), matches without regard to case by
	// prefixing the compiled pattern with the (?i) flag. When true the match is
	// exact.
	CaseSensitive bool
	// WholeWord, when true, brackets the pattern with \b word boundaries so it
	// matches only whole words — "cat" then does not match inside "category".
	WholeWord bool
}

// FindMatches returns the [Selection] range of every non-overlapping match of
// pattern across lines, in document order, honouring opts. lines is a buffer
// split on "\n" (a [TextView]/[CodeEditor] host passes strings.Split(text,
// "\n")); each match is reported in that line's rune coordinates, so the result
// feeds [CodeEditor.SetMatchHighlights] directly.
//
// It is the toolkit's only search primitive: the [FindReplace] widget is UI
// only and never runs a regexp itself, so a host wires the widget's callbacks to
// this helper and pushes the count + ranges back. It stays a free function (no
// widget state) so any editor — not just the ones in this toolkit — can reuse it.
//
// Matching is per line: the regexp never spans a line break (the '.' class does
// not cross "\n"), which is what keeps every match on a single row and its
// columns unambiguous. An empty pattern matches nothing (nil, nil) rather than
// every position, and a zero-width match (e.g. from "x*" or "^") is skipped so
// the highlight set carries only visible spans. An invalid pattern — or a valid
// one made invalid by the word-boundary wrapping — returns (nil, err), the
// signal a host shows as the panel's invalid-regex state.
func FindMatches(lines []string, pattern string, opts SearchOptions) ([]Selection, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := compileSearch(pattern, opts)
	if err != nil {
		return nil, err
	}
	var out []Selection
	for li, line := range lines {
		for _, loc := range re.FindAllStringIndex(line, -1) {
			s, e := loc[0], loc[1]
			if s == e {
				continue // zero-width match: nothing to highlight
			}
			startCol := utf8.RuneCountInString(line[:s])
			endCol := utf8.RuneCountInString(line[:e])
			out = append(out, Selection{StartLine: li, StartCol: startCol, EndLine: li, EndCol: endCol})
		}
	}
	return out, nil
}

// compileSearch builds the regexp FindMatches runs, applying opts: literal
// quoting (when !Regex), the whole-word \b brackets, and the case-insensitive
// flag. It is separated so its branches are unit-testable without a buffer.
func compileSearch(pattern string, opts SearchOptions) (*regexp.Regexp, error) {
	body := pattern
	if !opts.Regex {
		body = regexp.QuoteMeta(pattern)
	}
	if opts.WholeWord {
		// Non-capturing group so the boundaries bracket the whole alternation,
		// not just its first branch.
		body = `\b(?:` + body + `)\b`
	}
	var b strings.Builder
	if !opts.CaseSensitive {
		b.WriteString("(?i)")
	}
	b.WriteString(body)
	return regexp.Compile(b.String())
}
