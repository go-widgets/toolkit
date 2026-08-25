// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// MatchHighlighter is the search-match highlight surface an editor exposes so a
// find/replace host drives every editor the same way, whatever coordinate model
// the editor uses for a range R. The two toolkit editors satisfy it at their own
// range type: a [CodeEditor] at [Selection] (line, col over a flat line buffer)
// and a [RichEditor] at [DocSelection] (block, off over a rich document). A host
// that wants to stay editor-agnostic — the same find bar wired to either — codes
// against this interface and its FindMatches results, converting once at the
// coordinate boundary (see [DocSelectionsFromMatches] for the rich side).
//
// It is deliberately the four verbs a find/replace UI needs and no more: push
// the occurrence set, mark the active one, clear both, and reveal a match. The
// editor never searches — a host runs [FindMatches] (or its own regexp) over the
// editor's plain-text projection and pushes the ranges back — so this interface
// carries no query surface of its own.
type MatchHighlighter[R any] interface {
	// SetMatchHighlights replaces the soft-highlight occurrence set.
	SetMatchHighlights(ranges []R)
	// SetCurrentMatch marks one range as the emphasised active match; the zero
	// range clears the emphasis without touching the soft set.
	SetCurrentMatch(r R)
	// ClearMatchHighlights drops both the soft set and the current-match emphasis.
	ClearMatchHighlights()
	// ScrollToMatch reveals r, centring it only when it is out of view.
	ScrollToMatch(r R)
}

// Compile-time proof that both editors satisfy the surface at their range type,
// so the playground's find/replace wiring is symmetric across the two.
var (
	_ MatchHighlighter[Selection]    = (*CodeEditor)(nil)
	_ MatchHighlighter[DocSelection] = (*RichEditor)(nil)
)
