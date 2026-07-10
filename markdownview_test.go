// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

func TestParseHeading(t *testing.T) {
	cases := []struct {
		in    string
		level int
		text  string
		ok    bool
	}{
		{"# Title", 1, "Title", true},
		{"### Deep", 3, "Deep", true},
		{"###### Six", 6, "Six", true},
		{"####### Seven", 0, "", false}, // > 6 hashes
		{"#NoSpace", 0, "", false},      // no space after #
		{"plain", 0, "", false},         // no leading #
		{"#", 0, "", false},             // hash with nothing after
	}
	for _, c := range cases {
		lvl, txt, ok := parseHeading(c.in)
		if lvl != c.level || txt != c.text || ok != c.ok {
			t.Errorf("parseHeading(%q) = (%d,%q,%v), want (%d,%q,%v)",
				c.in, lvl, txt, ok, c.level, c.text, c.ok)
		}
	}
}

func TestParseBullet(t *testing.T) {
	for _, marker := range []string{"- item", "* item", "+ item"} {
		if txt, ok := parseBullet(marker); !ok || txt != "item" {
			t.Errorf("parseBullet(%q) = (%q,%v), want (item,true)", marker, txt, ok)
		}
	}
	for _, bad := range []string{"-nospace", "x item", "-", ""} {
		if _, ok := parseBullet(bad); ok {
			t.Errorf("parseBullet(%q) should not match", bad)
		}
	}
}

func TestWordWrap(t *testing.T) {
	// Greedy wrap at a column budget.
	got := wordWrap("the quick brown fox", 9)
	want := []string{"the quick", "brown fox"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("wordWrap = %q, want %q", got, want)
	}
	// maxChars <= 0 returns the whole string as one line.
	if got := wordWrap("abc def", 0); len(got) != 1 || got[0] != "abc def" {
		t.Errorf("wordWrap(_, 0) = %q, want single line", got)
	}
	// Empty / whitespace-only text yields one (empty) line.
	if got := wordWrap("   ", 10); len(got) != 1 {
		t.Errorf("wordWrap(blank) = %q, want one line", got)
	}
	// A single over-long word is not split.
	if got := wordWrap("supercalifragilistic", 5); len(got) != 1 {
		t.Errorf("wordWrap(longword) = %q, want one line", got)
	}
}

func TestParseMarkdownBlocks(t *testing.T) {
	src := "# Heading\n\nA paragraph line.\n\n- one\n- two\n\n```\ncode a\ncode b\n```\n"
	blocks := parseMarkdown(src)
	kinds := []mdKind{mdHeading, mdParagraph, mdBullet, mdBullet, mdCode}
	if len(blocks) != len(kinds) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(kinds), blocks)
	}
	for i, k := range kinds {
		if blocks[i].kind != k {
			t.Errorf("block %d kind = %d, want %d", i, blocks[i].kind, k)
		}
	}
	if blocks[0].level != 1 || blocks[0].text != "Heading" {
		t.Errorf("heading block = %+v", blocks[0])
	}
	if blocks[4].text != "code a\ncode b" {
		t.Errorf("code block text = %q", blocks[4].text)
	}
}

func TestParseMarkdownUnclosedFence(t *testing.T) {
	// An unclosed fence captures to end-of-input and still flushes a code block.
	blocks := parseMarkdown("```\nx = 1\ny = 2")
	if len(blocks) != 1 || blocks[0].kind != mdCode || blocks[0].text != "x = 1\ny = 2" {
		t.Errorf("unclosed fence = %+v", blocks)
	}
}

func TestMarkdownViewDrawAllBlocks(t *testing.T) {
	// Covers every drawBlock arm: H1 (underlined), H3 (no rule), bullet, code,
	// paragraph (wrapped).
	src := "# Big\n\n### Small\n\n- a bullet item that wraps across the width nicely\n\n" +
		"```\ncode\n```\n\nA plain paragraph that is long enough to wrap onto two lines here."
	m := NewMarkdownView(src)
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	surf := makeSurface(120, 200)
	m.Draw(newP(surf, 120), DefaultLight())

	th := DefaultLight()
	// Headings + their underline paint Accent ink.
	if got := countInk(surf, 120, 200, th.Accent); got == 0 {
		t.Error("no heading/accent pixels drawn")
	}
	// Code band paints a SurfaceAlt region.
	if got := countInk(surf, 120, 200, th.SurfaceAlt); got == 0 {
		t.Error("no code-band pixels drawn")
	}
	// Body text paints OnSurface ink.
	if got := countInk(surf, 120, 200, th.OnSurface); got == 0 {
		t.Error("no body-text pixels drawn")
	}
}

func TestMarkdownViewNarrowWidthNoWrap(t *testing.T) {
	// A width narrower than the padding makes maxChars <= 0; Draw must not
	// panic and paragraphs fall back to a single (clipped) line.
	m := NewMarkdownView("some text\n\n- bullet")
	m.SetBounds(Rect{X: 0, Y: 0, W: 6, H: 60})
	surf := makeSurface(6, 60)
	m.Draw(newP(surf, 6), DefaultLight())
}
