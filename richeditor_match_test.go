// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"

	"github.com/go-richdoc/richdoc"
)

// The scroll tests reuse tallDoc() from richeditor_test.go (40 identical
// paragraphs, taller than any viewport below).

// --- plain-text projection ------------------------------------------------

func TestBlockTextsProjection(t *testing.T) {
	e := newSample()
	got := e.BlockTexts()
	want := []string{"Title", "bold and it", "one", "x := 1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BlockTexts() = %#v, want %#v", got, want)
	}
}

func TestBlockTextsNonEditableBlockIsEmptyButKeepsIndex(t *testing.T) {
	// A thematic break has no editable content; its slot must still be present so
	// block indices equal slice indices.
	d := richdoc.New().
		P(richdoc.Txt("above")).
		HR().
		P(richdoc.Txt("below")).
		Doc()
	e := NewRichEditor(d)
	got := e.BlockTexts()
	want := []string{"above", "", "below"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BlockTexts() = %#v, want %#v", got, want)
	}
}

func TestBlockTextsEmptyDocument(t *testing.T) {
	e := NewRichEditor(nil)
	if got := e.BlockTexts(); len(got) != 0 {
		t.Fatalf("BlockTexts() on empty doc = %#v, want empty", got)
	}
}

// --- match -> DocSelection mapping ----------------------------------------

func TestDocSelectionFromMatch(t *testing.T) {
	m := Selection{StartLine: 1, StartCol: 5, EndLine: 1, EndCol: 8}
	got := DocSelectionFromMatch(m)
	want := DocSelection{Start: DocPos{Block: 1, Off: 5}, End: DocPos{Block: 1, Off: 8}}
	if got != want {
		t.Fatalf("DocSelectionFromMatch = %+v, want %+v", got, want)
	}
}

func TestDocSelectionsFromMatches(t *testing.T) {
	ms := []Selection{
		{StartLine: 0, StartCol: 1, EndLine: 0, EndCol: 3},
		{StartLine: 2, StartCol: 0, EndLine: 2, EndCol: 2},
	}
	got := DocSelectionsFromMatches(ms)
	want := []DocSelection{
		{Start: DocPos{0, 1}, End: DocPos{0, 3}},
		{Start: DocPos{2, 0}, End: DocPos{2, 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DocSelectionsFromMatches = %#v, want %#v", got, want)
	}
	// Empty input yields an empty (non-panicking) result.
	if got := DocSelectionsFromMatches(nil); len(got) != 0 {
		t.Fatalf("DocSelectionsFromMatches(nil) = %#v, want empty", got)
	}
}

// TestFindMatchesRoundTrip proves the projection + mapping wire straight into a
// real search, exactly as the playground will call them.
func TestFindMatchesRoundTrip(t *testing.T) {
	e := newSample()
	matches, err := FindMatches(e.BlockTexts(), "and", SearchOptions{Regex: true, CaseSensitive: true})
	if err != nil {
		t.Fatalf("FindMatches: %v", err)
	}
	ranges := DocSelectionsFromMatches(matches)
	want := []DocSelection{{Start: DocPos{1, 5}, End: DocPos{1, 8}}}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("round-trip ranges = %#v, want %#v", ranges, want)
	}
}

// --- highlight set accessors ----------------------------------------------

func TestSetAndReadMatchHighlights(t *testing.T) {
	e := newSample()
	if got := e.MatchHighlights(); got != nil {
		t.Fatalf("fresh MatchHighlights = %#v, want nil", got)
	}
	ranges := []DocSelection{{Start: DocPos{1, 5}, End: DocPos{1, 8}}}
	e.SetMatchHighlights(ranges)
	got := e.MatchHighlights()
	if !reflect.DeepEqual(got, ranges) {
		t.Fatalf("MatchHighlights = %#v, want %#v", got, ranges)
	}
	// The read is a copy: mutating it must not disturb the editor's set.
	got[0] = DocSelection{}
	if e.MatchHighlights()[0] != ranges[0] {
		t.Error("MatchHighlights returned an aliasing slice, not a copy")
	}
}

func TestSetAndReadCurrentMatch(t *testing.T) {
	e := newSample()
	if !e.CurrentMatch().IsEmpty() {
		t.Fatal("fresh CurrentMatch is not empty")
	}
	sel := DocSelection{Start: DocPos{1, 5}, End: DocPos{1, 8}}
	e.SetCurrentMatch(sel)
	if e.CurrentMatch() != sel {
		t.Fatalf("CurrentMatch = %+v, want %+v", e.CurrentMatch(), sel)
	}
}

func TestClearMatchHighlights(t *testing.T) {
	e := newSample()
	e.SetMatchHighlights([]DocSelection{{Start: DocPos{1, 5}, End: DocPos{1, 8}}})
	e.SetCurrentMatch(DocSelection{Start: DocPos{1, 5}, End: DocPos{1, 8}})
	e.ClearMatchHighlights()
	if len(e.MatchHighlights()) != 0 || !e.CurrentMatch().IsEmpty() {
		t.Fatal("ClearMatchHighlights left residual state")
	}
}

// --- painting -------------------------------------------------------------

func TestMatchHighlightPaintsAndClears(t *testing.T) {
	e := newSample()
	none := reRender(e, 240, 400, DefaultLight())
	e.SetMatchHighlights([]DocSelection{{Start: DocPos{1, 5}, End: DocPos{1, 8}}})
	withBand := reRender(e, 240, 400, DefaultLight())
	if diffCount(none, withBand) == 0 {
		t.Error("a soft match produced no visible band")
	}
	e.ClearMatchHighlights()
	cleared := reRender(e, 240, 400, DefaultLight())
	if diffCount(none, cleared) != 0 {
		t.Error("clearing the match highlights left a residual band")
	}
}

func TestEmptyMatchInSetIsSkipped(t *testing.T) {
	e := newSample()
	real := DocSelection{Start: DocPos{1, 5}, End: DocPos{1, 8}}
	onlyReal := func() []byte {
		e.SetMatchHighlights([]DocSelection{real})
		return reRender(e, 240, 400, DefaultLight())
	}()
	withEmpty := func() []byte {
		e.SetMatchHighlights([]DocSelection{{}, real})
		return reRender(e, 240, 400, DefaultLight())
	}()
	if diffCount(onlyReal, withEmpty) != 0 {
		t.Error("an empty range in the set changed the painted output")
	}
}

func TestCurrentMatchAddsEmphasisOverSoft(t *testing.T) {
	e := newSample()
	sel := DocSelection{Start: DocPos{1, 5}, End: DocPos{1, 8}}
	e.SetMatchHighlights([]DocSelection{sel})
	soft := reRender(e, 240, 400, DefaultLight())
	e.SetCurrentMatch(sel)
	current := reRender(e, 240, 400, DefaultLight())
	if diffCount(soft, current) == 0 {
		t.Error("the current-match emphasis (stronger fill + outline) added no pixels")
	}
}

func TestCurrentMatchAloneWithoutSoftSet(t *testing.T) {
	// currentMatch drives the band even with an empty soft set: hasMatchHighlights
	// is true and drawMatchBands paints only the current band.
	e := newSample()
	none := reRender(e, 240, 400, DefaultLight())
	e.SetCurrentMatch(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})
	band := reRender(e, 240, 400, DefaultLight())
	if diffCount(none, band) == 0 {
		t.Error("a lone current match produced no band")
	}
}

func TestMatchColorOverride(t *testing.T) {
	e := newSample()
	e.SetMatchHighlights([]DocSelection{{Start: DocPos{1, 0}, End: DocPos{1, 4}}})
	def := reRender(e, 240, 400, DefaultLight())
	e.MatchColor = RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0x60}
	over := reRender(e, 240, 400, DefaultLight())
	if diffCount(def, over) == 0 {
		t.Error("MatchColor override did not change the band tint")
	}
}

func TestCurrentMatchColorOverride(t *testing.T) {
	e := newSample()
	sel := DocSelection{Start: DocPos{1, 0}, End: DocPos{1, 4}}
	e.SetCurrentMatch(sel)
	def := reRender(e, 240, 400, DefaultLight())
	e.CurrentMatchColor = RGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0x90}
	over := reRender(e, 240, 400, DefaultLight())
	if diffCount(def, over) == 0 {
		t.Error("CurrentMatchColor override did not change the band tint")
	}
}

// TestMatchBandSpansBlocks covers the multi-block band geometry (a start line
// that continues to the right edge, and an end line clipped to its offset).
func TestMatchBandSpansBlocks(t *testing.T) {
	e := newSample()
	none := reRender(e, 240, 400, DefaultLight())
	e.SetMatchHighlights([]DocSelection{{Start: DocPos{0, 2}, End: DocPos{1, 3}}})
	span := reRender(e, 240, 400, DefaultLight())
	if diffCount(none, span) == 0 {
		t.Error("a multi-block match produced no band")
	}
}

// TestMatchBandOnWrappedBlockFirstLine forces a block to wrap onto two visual
// lines with the match on the first one, so a later visual line of the same
// block exercises the "covers no cell on this line" path of bandXRange.
func TestMatchBandOnWrappedBlockFirstLine(t *testing.T) {
	d := richdoc.New().
		P(richdoc.Txt("alpha beta gamma delta epsilon zeta eta theta iota kappa")).
		Doc()
	e := NewRichEditor(d)
	none := reRender(e, 90, 200, DefaultLight()) // narrow: the paragraph wraps
	// "alpha" sits at the very start, on the first visual line only.
	e.SetMatchHighlights([]DocSelection{{Start: DocPos{0, 0}, End: DocPos{0, 5}}})
	band := reRender(e, 90, 200, DefaultLight())
	if diffCount(none, band) == 0 {
		t.Error("a match on the first wrapped line produced no band")
	}
}

// --- scrolling ------------------------------------------------------------

func TestScrollToMatchOutOfRangeBlockIsNoOp(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	e.ScrollOffset().Set(0)
	e.ScrollToMatch(DocSelection{Start: DocPos{99, 0}, End: DocPos{99, 0}})
	if got := e.ScrollOffset().Get(); got != 0 {
		t.Fatalf("out-of-range ScrollToMatch moved scroll to %d, want 0", got)
	}
}

func TestScrollToMatchZeroHeightIsNoOp(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 0})
	e.ScrollToMatch(DocSelection{Start: DocPos{5, 0}, End: DocPos{5, 3}})
	if got := e.ScrollOffset().Get(); got != 0 {
		t.Fatalf("zero-height ScrollToMatch moved scroll to %d, want 0", got)
	}
}

func TestScrollToMatchVisibleIsNoOp(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	e.ScrollOffset().Set(0)
	e.ScrollToMatch(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 4}})
	if got := e.ScrollOffset().Get(); got != 0 {
		t.Fatalf("visible ScrollToMatch moved scroll to %d, want 0", got)
	}
}

func TestScrollToMatchBelowViewScrollsDown(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	e.ScrollOffset().Set(0)
	e.ScrollToMatch(DocSelection{Start: DocPos{20, 0}, End: DocPos{20, 4}})
	if got := e.ScrollOffset().Get(); got <= 0 {
		t.Fatalf("ScrollToMatch below the view did not scroll down (offset %d)", got)
	}
}

func TestScrollToMatchClampsToMax(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	e.ScrollToMatch(DocSelection{Start: DocPos{39, 0}, End: DocPos{39, 4}})
	lay := e.buildLayout(e.theme())
	if got, max := e.ScrollOffset().Get(), e.maxScroll(lay); got != max {
		t.Fatalf("ScrollToMatch on the last block = %d, want clamped max %d", got, max)
	}
}

func TestScrollToMatchAboveViewClampsToZero(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	lay := e.buildLayout(e.theme())
	e.ScrollOffset().Set(e.maxScroll(lay)) // scrolled to the very bottom
	e.ScrollToMatch(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 4}})
	if got := e.ScrollOffset().Get(); got != 0 {
		t.Fatalf("ScrollToMatch on the top block from the bottom = %d, want 0", got)
	}
}

// --- shared interface -----------------------------------------------------

// TestMatchHighlighterInterface exercises both editors through the generic
// MatchHighlighter surface the playground can code against.
func TestMatchHighlighterInterface(t *testing.T) {
	var rich MatchHighlighter[DocSelection] = NewRichEditor(sampleDoc())
	rich.SetMatchHighlights([]DocSelection{{Start: DocPos{0, 0}, End: DocPos{0, 5}}})
	rich.SetCurrentMatch(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})
	rich.ScrollToMatch(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})
	rich.ClearMatchHighlights()

	var code MatchHighlighter[Selection] = NewCodeEditor("hello\nworld")
	code.SetMatchHighlights([]Selection{{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5}})
	code.SetCurrentMatch(Selection{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5})
	code.ScrollToMatch(Selection{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5})
	code.ClearMatchHighlights()
}
