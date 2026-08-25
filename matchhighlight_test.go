// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

// SetMatchHighlights / MatchHighlights / SetCurrentMatch / CurrentMatch /
// ClearMatchHighlights round-trip, and the getter returns a COPY the caller can
// mutate without disturbing the editor's set.
func TestCodeEditorMatchHighlightSetGet(t *testing.T) {
	c := NewCodeEditor("alpha\nbeta")
	ranges := []Selection{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3},
		{StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 4},
	}
	c.SetMatchHighlights(ranges)
	got := c.MatchHighlights()
	if len(got) != 2 || got[0] != ranges[0] || got[1] != ranges[1] {
		t.Fatalf("MatchHighlights = %v, want %v", got, ranges)
	}
	// Mutating the returned slice must not reach into the editor.
	got[0] = Selection{}
	if again := c.MatchHighlights(); again[0] != ranges[0] {
		t.Fatalf("returned slice is not a copy: editor now %v", again[0])
	}

	cur := Selection{StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 4}
	c.SetCurrentMatch(cur)
	if c.CurrentMatch() != cur {
		t.Fatalf("CurrentMatch = %v, want %v", c.CurrentMatch(), cur)
	}

	c.ClearMatchHighlights()
	if len(c.MatchHighlights()) != 0 {
		t.Errorf("after clear, MatchHighlights = %v, want empty", c.MatchHighlights())
	}
	if !c.CurrentMatch().IsEmpty() {
		t.Errorf("after clear, CurrentMatch = %v, want empty", c.CurrentMatch())
	}
}

// A soft match highlight paints a band behind the matched runes: with an opaque
// override colour the band's pixels land on the match's row within its column
// span, and an editor with no highlights renders bare.
func TestCodeEditorMatchHighlightPaintsBand(t *testing.T) {
	band := RGBA{R: 0xE0, G: 0x30, B: 0x90, A: 0xFF}
	c := NewCodeEditor("abcd\nefgh")
	c.MatchColor = band
	th := DefaultLight()
	w, hgt := 160, 80

	// Bare (no highlights): remember the pixels.
	bare := drawInto(c, th, w, hgt)
	if n := countInk(bare, w, hgt, band); n != 0 {
		t.Fatalf("band colour present with no highlights: %d px", n)
	}

	// Highlight runes [1,3) on line 0 ("bc").
	c.SetMatchHighlights([]Selection{{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 3}})
	surf := drawInto(c, th, w, hgt)
	minX, minY, maxX, maxY, n := colorBBox(surf, w, hgt, band)
	if n == 0 {
		t.Fatal("soft match band never painted")
	}
	lineH := c.glyphHeight() + 4
	adv := c.glyphAdvance()
	textX := c.textLeftInset()
	if minY < 2 || maxY >= 4+lineH {
		t.Errorf("band y-span [%d,%d] not within row 0 [2,%d)", minY, maxY, 4+lineH)
	}
	if minX < textX || maxX >= textX+3*adv {
		t.Errorf("band x-span [%d,%d] not within match cols [%d,%d)", minX, maxX, textX+3*adv, textX)
	}

	// Clearing returns to the bare pixels.
	c.ClearMatchHighlights()
	back := drawInto(c, th, w, hgt)
	if !bytes.Equal(bare, back) {
		t.Error("clearing the highlights did not return to the bare render")
	}
}

// The current match gets a distinct fill AND an accent outline box on top, so it
// reads apart from the soft highlights.
func TestCodeEditorCurrentMatchOutline(t *testing.T) {
	fill := RGBA{R: 0x22, G: 0xEE, B: 0x22, A: 0xFF}
	c := NewCodeEditor("abcd")
	c.CurrentMatchColor = fill
	th := DefaultLight()
	// Not focused: the only accent-coloured pixels are the outline box (a focused
	// editor would also paint an accent border).
	c.Focused().Set(false)
	c.SetCurrentMatch(Selection{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 4})
	surf := drawInto(c, th, 160, 60)

	if n := countInk(surf, 160, 60, fill); n == 0 {
		t.Error("current-match fill never painted")
	}
	if n := countInk(surf, 160, 60, th.Accent); n == 0 {
		t.Error("current-match accent outline never painted")
	}
}

// A multi-line highlight paints on every row it spans, exercising the through-
// line band geometry (start row, spanned row).
func TestCodeEditorMatchHighlightMultiLine(t *testing.T) {
	band := RGBA{R: 0x10, G: 0xC0, B: 0xF0, A: 0xFF}
	c := NewCodeEditor("aaaa\nbbbb\ncccc")
	c.MatchColor = band
	th := DefaultLight()
	w, hgt := 160, 90
	c.SetMatchHighlights([]Selection{{StartLine: 0, StartCol: 2, EndLine: 1, EndCol: 2}})
	surf := drawInto(c, th, w, hgt)

	lineH := c.glyphHeight() + 4
	_, minY, _, maxY, n := colorBBox(surf, w, hgt, band)
	if n == 0 {
		t.Fatal("multi-line band never painted")
	}
	// Band pixels appear on both row 0 and row 1.
	if minY >= 4+lineH {
		t.Errorf("band starts at y=%d, want some pixels on row 0 (< %d)", minY, 4+lineH)
	}
	if maxY < 4+lineH {
		t.Errorf("band ends at y=%d, want some pixels on row 1 (>= %d)", maxY, 4+lineH)
	}
}

// An empty Selection inside the highlight set is skipped rather than painted (a
// zero-width entry contributes no band), and overlapping ranges are safe.
func TestCodeEditorMatchHighlightEmptyAndOverlap(t *testing.T) {
	c := NewCodeEditor("abcdef")
	c.MatchColor = RGBA{R: 1, G: 2, B: 3, A: 0xFF}
	th := DefaultLight()
	// A live range, an empty (skipped) range, and a range overlapping the first.
	c.SetMatchHighlights([]Selection{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 3},
		{StartLine: 0, StartCol: 2, EndLine: 0, EndCol: 2}, // empty -> skipped
		{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 4}, // overlaps the first
	})
	// Must not panic and must paint something.
	surf := drawInto(c, th, 160, 60)
	if n := countInk(surf, 160, 60, c.MatchColor); n == 0 {
		t.Fatal("overlapping highlight set painted no band")
	}
}

// matchTint / currentMatchTint honour an explicit override and otherwise derive
// from the theme accent.
func TestCodeEditorMatchTintDefaults(t *testing.T) {
	c := NewCodeEditor("x")
	th := DefaultLight()

	override := RGBA{R: 9, G: 8, B: 7, A: 0xFF}
	c.MatchColor = override
	if got := c.matchTint(th); got != override {
		t.Errorf("matchTint override = %v, want %v", got, override)
	}
	c.MatchColor = RGBA{}
	wantSoft := RGBA{R: th.Accent.R, G: th.Accent.G, B: th.Accent.B, A: 0x3C}
	if got := c.matchTint(th); got != wantSoft {
		t.Errorf("matchTint default = %v, want %v", got, wantSoft)
	}

	c.CurrentMatchColor = override
	if got := c.currentMatchTint(th); got != override {
		t.Errorf("currentMatchTint override = %v, want %v", got, override)
	}
	c.CurrentMatchColor = RGBA{}
	wantCur := RGBA{R: th.Accent.R, G: th.Accent.G, B: th.Accent.B, A: 0x80}
	if got := c.currentMatchTint(th); got != wantCur {
		t.Errorf("currentMatchTint default = %v, want %v", got, wantCur)
	}
}

// ScrollToMatch reveals an out-of-view match by centring it, leaves an already
// visible match's scroll alone, no-ops on a zero-height editor, and clamps a
// negative start line.
func TestCodeEditorScrollToMatch(t *testing.T) {
	// 40-line buffer in a viewport a handful of lines tall.
	var lines string
	for i := 0; i < 40; i++ {
		if i > 0 {
			lines += "\n"
		}
		lines += "line"
	}
	c := NewCodeEditor(lines)
	c.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 80})
	vis := c.visibleLines()
	if vis <= 0 {
		t.Fatalf("precondition: visibleLines = %d, want > 0", vis)
	}

	// Out of view: scroll changes and the target ends up visible, centred.
	c.ScrollToMatch(Selection{StartLine: 30})
	top := c.clampedScrollLine()
	if 30 < top || 30 >= top+vis {
		t.Fatalf("after ScrollToMatch(30), top=%d vis=%d — line 30 not visible", top, vis)
	}
	// A match already on screen leaves the scroll untouched.
	before := c.ScrollLine().Get()
	inView := top + vis/2
	c.ScrollToMatch(Selection{StartLine: inView})
	if c.ScrollLine().Get() != before {
		t.Errorf("ScrollToMatch of an in-view line moved scroll %d -> %d", before, c.ScrollLine().Get())
	}
	// Negative start line clamps to 0 and scrolls to the top without panic.
	c.ScrollToMatch(Selection{StartLine: -5})
	if got := c.clampedScrollLine(); got != 0 {
		t.Errorf("ScrollToMatch(-5) top = %d, want 0", got)
	}

	// Zero-height editor: no room for a line, so it is a no-op (no panic).
	z := NewCodeEditor(lines)
	z.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 0})
	z.ScrollToMatch(Selection{StartLine: 20})
	if z.ScrollLine().Get() != 0 {
		t.Errorf("zero-height ScrollToMatch moved scroll to %d, want 0", z.ScrollLine().Get())
	}
}
