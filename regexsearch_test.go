// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

// An empty pattern matches nothing (rather than every position) and reports no
// error — the "the query field is blank" case a host feeds every keystroke.
func TestFindMatchesEmptyPattern(t *testing.T) {
	got, err := FindMatches([]string{"anything"}, "", SearchOptions{})
	if err != nil {
		t.Fatalf("empty pattern err = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("empty pattern matches = %v, want nil", got)
	}
}

// With Regex off the pattern is a literal: the metacharacters match themselves,
// so "a.b" finds "a.b" and not "axb".
func TestFindMatchesLiteral(t *testing.T) {
	lines := []string{"a.b axb"}
	got, err := FindMatches(lines, "a.b", SearchOptions{Regex: false})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("literal matches = %v, want %v", got, want)
	}
}

// With Regex on the same "a.b" is a pattern, so the dot matches any rune and
// "axb" is found.
func TestFindMatchesRegex(t *testing.T) {
	got, err := FindMatches([]string{"axb"}, "a.b", SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("regex matches = %v, want %v", got, want)
	}
}

// Case-insensitive by default: "abc" finds "ABC".
func TestFindMatchesCaseInsensitiveDefault(t *testing.T) {
	got, err := FindMatches([]string{"ABC"}, "abc", SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("case-insensitive matches = %v, want 1", got)
	}
}

// Case-sensitive on: "abc" does not find "ABC".
func TestFindMatchesCaseSensitive(t *testing.T) {
	got, err := FindMatches([]string{"ABC"}, "abc", SearchOptions{Regex: true, CaseSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("case-sensitive matches = %v, want none", got)
	}
}

// Whole-word on: "cat" matches the standalone word but not the "cat" inside
// "category".
func TestFindMatchesWholeWord(t *testing.T) {
	got, err := FindMatches([]string{"a cat in a category"}, "cat", SearchOptions{Regex: true, CaseSensitive: true, WholeWord: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("whole-word matches = %v, want just the standalone cat %v", got, want)
	}
}

// An invalid pattern returns (nil, err) — the bar's invalid-regex state.
func TestFindMatchesInvalidRegex(t *testing.T) {
	got, err := FindMatches([]string{"x"}, "(", SearchOptions{Regex: true})
	if err == nil {
		t.Fatal("invalid pattern err = nil, want a compile error")
	}
	if got != nil {
		t.Fatalf("invalid pattern matches = %v, want nil", got)
	}
}

// A pattern that can match empty (a* over a run with no 'a') produces only
// zero-width matches, all of which are skipped, so the result is empty.
func TestFindMatchesSkipsZeroWidth(t *testing.T) {
	got, err := FindMatches([]string{"bbb"}, "a*", SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("zero-width matches = %v, want nil (all skipped)", got)
	}
	// The same pattern still reports its non-empty match where one exists.
	got, err = FindMatches([]string{"baab"}, "a+", SearchOptions{Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("a+ matches = %v, want %v", got, want)
	}
}

// Matches are reported across lines in document order, each on its own row.
func TestFindMatchesMultipleLines(t *testing.T) {
	got, err := FindMatches([]string{"foo", "bar foo", "baz"}, "foo", SearchOptions{Regex: true, CaseSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []Selection{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3},
		{StartLine: 1, StartCol: 4, EndLine: 1, EndCol: 7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-line matches = %v, want %v", got, want)
	}
}

// Columns are RUNE indices, not byte offsets: over a line with multi-byte runes
// the match's start/end land on the right characters.
func TestFindMatchesRuneColumns(t *testing.T) {
	got, err := FindMatches([]string{"héllo"}, "llo", SearchOptions{Regex: true, CaseSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	// h(0) é(1) l(2) l(3) o(4): "llo" spans rune columns [2,5).
	want := []Selection{{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rune-column matches = %v, want %v", got, want)
	}
}
