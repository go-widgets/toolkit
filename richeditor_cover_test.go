// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-richdoc/richdoc"
	"github.com/go-widgets/painter"
)

func TestA11yInfo(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("hi")).Doc())
	info := e.A11y()
	if info.Role != RoleTextbox || info.Value != "hi" {
		t.Errorf("A11y = %+v, want textbox/hi", info)
	}
}

func TestDocValueNil(t *testing.T) {
	e := newSample()
	e.Doc().Set(nil)
	if e.docValue() == nil || len(e.docValue().Blocks) != 0 {
		t.Error("docValue did not substitute an empty document for nil")
	}
}

func TestResizeFontClampsToOne(t *testing.T) {
	// A shrink that would round below 1 clamps to a scale-1 bitmap.
	if got := resizeFont(NewBitmapFont(1), 1, 4); got.Height() != NewBitmapFont(1).Height() {
		t.Errorf("bitmap shrink height = %d, want %d", got.Height(), NewBitmapFont(1).Height())
	}
	// Same for a TrueType face shrunk past 1px.
	base, _ := NewTrueTypeFont(testFontTTF, 10)
	if got := resizeFont(base, 1, 100); got.Height() <= 0 {
		t.Errorf("ttf shrink height = %d, want > 0", got.Height())
	}
}

func TestHeadingFactorAllLevels(t *testing.T) {
	e := NewRichEditor(richdoc.New().
		H(2, richdoc.Txt("h2")).H(3, richdoc.Txt("h3")).H(4, richdoc.Txt("h4")).H(5, richdoc.Txt("h5")).
		Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})
	lay := e.buildLayout(DefaultLight())
	heights := map[int]int{}
	for _, ln := range lay.lines {
		heights[ln.blockIdx] = ln.h
	}
	// Each larger heading level is at least as tall as the next.
	if !(heights[0] >= heights[1] && heights[1] >= heights[2] && heights[2] >= heights[3]) {
		t.Errorf("heading heights not monotone: %v", heights)
	}
}

func TestClickBelowContentPicksNearest(t *testing.T) {
	e := newSample()
	lay := e.buildLayout(DefaultLight())
	// A click well below all content maps to the nearest (last) caret line.
	pos := e.posAtLocal(lay, 20, 399)
	last := len(e.docValue().Blocks) - 1
	if pos.Block != last {
		t.Errorf("click below content = block %d, want %d", pos.Block, last)
	}
}

func TestInvalidCaretOperationsAreSafe(t *testing.T) {
	// Driving every command from an out-of-range caret must not panic and must
	// leave the document intact (exercises the defensive guards).
	e := newSample()
	before := richdoc.PlainText(e.docValue())
	e.Caret().Set(DocPos{99, 0})
	for _, code := range []string{"ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "Backspace", "Delete", "Enter"} {
		e.OnEvent(Event{Kind: EventKeyDown, Code: code})
		e.Caret().Set(DocPos{99, 0})
	}
	e.ToggleStrong() // no selection + invalid caret -> styleAtCaret guard
	e.SetBlockType(BlockH1)
	e.ToggleList(false)
	e.InsertText("z")
	if richdoc.PlainText(e.docValue()) != before {
		t.Errorf("invalid-caret ops mutated the document: %q -> %q", before, richdoc.PlainText(e.docValue()))
	}
}

func TestScrollCaretInvalidNoop(t *testing.T) {
	e := newSample()
	_ = reRender(e, 240, 400, DefaultLight())
	e.Caret().Set(DocPos{99, 0})
	e.scrollCaretIntoView() // !ok branch
}

func TestBigScrollClamps(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	_ = reRender(e, 200, 80, DefaultLight())
	e.OnEvent(Event{Kind: EventScroll, Delta: 100000}) // clamps (reClamp v>hi)
	lay := e.buildLayout(DefaultLight())
	if e.ScrollOffset().Get() != e.maxScroll(lay) {
		t.Errorf("over-scroll = %d, want max %d", e.ScrollOffset().Get(), e.maxScroll(lay))
	}
}

func TestBackwardDragNormalizes(t *testing.T) {
	e := newSample()
	x0, y0 := e.CaretPixel(DocPos{1, 5})
	x1, y1 := e.CaretPixel(DocPos{1, 1})
	e.OnEvent(Event{Kind: EventClick, X: x0, Y: y0})
	e.OnEvent(Event{Kind: EventMouseDrag, X: x1, Y: y1})
	sel := e.Selection().Get()
	if sel.Start != (DocPos{1, 1}) || sel.End != (DocPos{1, 5}) {
		t.Errorf("backward drag selection = %+v, want normalised {1,1}-{1,5}", sel)
	}
}

func TestInsertTextWithNewlineSplits(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 0})
	e.InsertText("x\ny")
	if len(e.docValue().Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(e.docValue().Blocks))
	}
	if blockPlain(e, 0) != "x" || blockPlain(e, 1) != "y" {
		t.Errorf("split blocks = %q,%q want x,y", blockPlain(e, 0), blockPlain(e, 1))
	}
}

func TestDeleteAndEnterWithSelection(t *testing.T) {
	e := newSample()
	// Delete key with a selection removes the range.
	e.Selection().Set(DocSelection{Start: DocPos{1, 0}, End: DocPos{1, 4}})
	e.Caret().Set(DocPos{1, 4})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if blockPlain(e, 1) != " and it" {
		t.Errorf("Delete-with-selection = %q, want %q", blockPlain(e, 1), " and it")
	}
	// Enter with a selection deletes it, then splits.
	e2 := newSample()
	e2.Selection().Set(DocSelection{Start: DocPos{1, 0}, End: DocPos{1, 4}})
	e2.Caret().Set(DocPos{1, 4})
	n0 := len(e2.docValue().Blocks)
	e2.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(e2.docValue().Blocks) != n0+1 {
		t.Errorf("Enter-with-selection blocks = %d, want %d", len(e2.docValue().Blocks), n0+1)
	}
}

func TestSelectionIntoAtomicBlock(t *testing.T) {
	// A selection whose end lands on an atomic block: the end has no editable
	// content, so DeleteSelection keeps the start head and drops the spanned blocks.
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("abc")).HR().P(richdoc.Txt("def")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Selection().Set(DocSelection{Start: DocPos{0, 1}, End: DocPos{1, 0}})
	e.Caret().Set(DocPos{1, 0})
	e.DeleteSelection()
	if blockPlain(e, 0) != "a" {
		t.Errorf("start head = %q, want %q", blockPlain(e, 0), "a")
	}
	if _, ok := e.docValue().Blocks[1].(richdoc.Paragraph); !ok {
		t.Errorf("block after delete = %#v, want the def paragraph", e.docValue().Blocks[1])
	}
}

func TestToggleAcrossAtomicBlock(t *testing.T) {
	// Toggling a style over a selection that spans an atomic block skips it.
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("ab")).HR().P(richdoc.Txt("cd")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{2, 2}})
	e.ToggleStrong()
	if !walkHas(e.docValue(), func(n any) bool { _, ok := n.(richdoc.Strong); return ok }) {
		t.Error("toggle across atomic block did not bold the editable ends")
	}
}

// --- rendering coverage: multi-block selection, nested list, non-para quote,
//     wide table row, offscreen chrome ------------------------------------

func TestMultiBlockSelectionPaints(t *testing.T) {
	e := newSample()
	e.Selection().Set(DocSelection{Start: DocPos{0, 2}, End: DocPos{2, 2}})
	buf := reRender(e, 240, 400, DefaultLight())
	plain := newSample()
	base := reRender(plain, 240, 400, DefaultLight())
	if diffCount(base, buf) == 0 {
		t.Error("multi-block selection painted no band")
	}
}

func TestWrappedSelectionPaints(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("one two three four five six seven")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 200})
	e.Selection().Set(DocSelection{Start: DocPos{0, 2}, End: DocPos{0, 25}})
	buf := reRender(e, 90, 200, DefaultLight())
	plain := NewRichEditor(richdoc.New().P(richdoc.Txt("one two three four five six seven")).Doc())
	base := reRender(plain, 90, 200, DefaultLight())
	if diffCount(base, buf) == 0 {
		t.Error("wrapped multi-line selection painted no band")
	}
}

func TestNestedListAndNonParaQuoteRender(t *testing.T) {
	doc := richdoc.New().
		UList(false, richdoc.Item(
			richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("outer")}},
			richdoc.List{Items: []richdoc.ListItem{{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("inner")}}}}}},
		)).
		Quote(richdoc.CodeBlock{Text: "code in quote"}).
		Doc()
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	if inkCount(reRender(e, 200, 300, DefaultLight()), 200, 300) == 0 {
		t.Fatal("nested list / non-para quote rendered nothing")
	}
}

func TestWideTableRowRenders(t *testing.T) {
	// A row wider than the header drives tableCols to count the row.
	doc := richdoc.New().Table(nil,
		[]richdoc.Cell{richdoc.Td(richdoc.Txt("H"))},
		[][]richdoc.Cell{{richdoc.Td(richdoc.Txt("a")), richdoc.Td(richdoc.Txt("b")), richdoc.Td(richdoc.Txt("c"))}}).Doc()
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	if inkCount(reRender(e, 200, 120, DefaultLight()), 200, 120) == 0 {
		t.Fatal("wide table rendered nothing")
	}
}

func TestOffscreenChromeCulled(t *testing.T) {
	// A list whose markers scroll above the viewport exercises the chrome cull
	// (both the text-marker and the fill-band branches).
	b := richdoc.New()
	for i := 0; i < 30; i++ {
		b.UList(false, richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("item")}}))
		b.CodeBlock("", "code")
	}
	e := NewRichEditor(b.Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 80})
	_ = reRender(e, 160, 80, DefaultLight())
	e.OnEvent(Event{Kind: EventScroll, Delta: 400}) // scroll far so early chrome is offscreen
	_ = reRender(e, 160, 80, DefaultLight())
}

// --- model helper direct unit tests (branches unreachable through the widget
//     flow but part of the model's contract) --------------------------------

func TestModelDefensiveBranches(t *testing.T) {
	// setBlockContent on an atomic block returns it unchanged.
	if got := setBlockContent(richdoc.ThematicBreak{}, nil); got != (richdoc.Block)(richdoc.ThematicBreak{}) {
		t.Errorf("setBlockContent(atomic) = %#v, want unchanged", got)
	}
	// primaryParagraph / setPrimaryParagraph on an item-less list.
	if _, ok := primaryParagraph(richdoc.List{}); ok {
		t.Error("primaryParagraph of an empty list reported ok")
	}
	l := setPrimaryParagraph(richdoc.List{}, []richdoc.Inline{richdoc.Txt("x")})
	if len(l.Items) != 1 {
		t.Errorf("setPrimaryParagraph seeded %d items, want 1", len(l.Items))
	}
	// setFirstParagraphSlice prepends when there is no paragraph.
	out := setFirstParagraphSlice([]richdoc.Block{richdoc.ThematicBreak{}}, []richdoc.Inline{richdoc.Txt("p")})
	if _, ok := out[0].(richdoc.Paragraph); !ok || len(out) != 2 {
		t.Errorf("setFirstParagraphSlice prepend = %#v", out)
	}
}

// noopPainter is a non-pixel painter used to drive the italic fallback path.
type noopPainter struct{}

func (noopPainter) FillRect(Rect, RGBA)                  {}
func (noopPainter) StrokeRect(Rect, RGBA, int)           {}
func (noopPainter) FillRoundRect(Rect, int, RGBA)        {}
func (noopPainter) StrokeRoundRect(Rect, int, RGBA, int) {}
func (noopPainter) PutPixel(int, int, RGBA)              {}
func (noopPainter) Text(int, int, string, RGBA)          {}
func (noopPainter) Size() (int, int)                     { return 0, 0 }

func TestArrowVertAtEdgesStays(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{0, 0}) // first stop line
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if e.Caret().Get().Block != 0 {
		t.Errorf("ArrowUp at top = block %d, want 0", e.Caret().Get().Block)
	}
	last := len(e.docValue().Blocks) - 1
	e.Caret().Set(DocPos{last, 0}) // last stop line
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if e.Caret().Get().Block != last {
		t.Errorf("ArrowDown at bottom = block %d, want %d", e.Caret().Get().Block, last)
	}
}

func TestInsertEmptyStringNoop(t *testing.T) {
	e := newSample()
	before := richdoc.PlainText(e.docValue())
	e.InsertText("")
	if richdoc.PlainText(e.docValue()) != before {
		t.Error("InsertText(\"\") mutated the document")
	}
}

func TestTypeIntoCodeBlock(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{3, 2})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	cb := e.docValue().Blocks[3].(richdoc.CodeBlock)
	if cb.Text != "x Z:= 1" {
		t.Errorf("code after type = %q, want %q", cb.Text, "x Z:= 1")
	}
}

func TestToggleListUnwrapEmptyList(t *testing.T) {
	e := NewRichEditor(richdoc.New().Add(richdoc.List{}).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 120})
	e.Caret().Set(DocPos{0, 0})
	e.ToggleList(false) // unwrap an item-less list -> a seeded empty paragraph
	if _, ok := e.docValue().Blocks[0].(richdoc.Paragraph); !ok {
		t.Errorf("unwrapped empty list = %#v, want Paragraph", e.docValue().Blocks[0])
	}
}

func TestEditOnContainerWithoutParagraph(t *testing.T) {
	// A list / quote whose primary block is not a paragraph has no editable
	// content; navigating off it exercises blockContent's not-editable arms.
	doc := richdoc.New().
		Add(richdoc.List{Items: []richdoc.ListItem{{Blocks: []richdoc.Block{richdoc.CodeBlock{Text: "c"}}}}}).
		Add(richdoc.BlockQuote{Blocks: []richdoc.Block{richdoc.CodeBlock{Text: "c"}}}).
		P(richdoc.Txt("end")).
		Doc()
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 200})
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // list -> blockLen -> blockContent
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // quote -> blockContent
	// Typing on them is a no-op (not editable).
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "x"})
	if _, ok := e.docValue().Blocks[0].(richdoc.List); !ok {
		t.Error("typing mutated a paragraph-less list")
	}
}

func TestSliceHelperGuards(t *testing.T) {
	// Out-of-range cell/block removals and an empty-doc clamp are no-ops.
	rs := []styledRune{{r: 'a'}}
	if got := removeCell(rs, 99); len(got) != 1 {
		t.Errorf("removeCell out of range mutated: %v", got)
	}
	blocks := []richdoc.Block{richdoc.Paragraph{}}
	if got := removeBlock(blocks, 99); len(got) != 1 {
		t.Errorf("removeBlock out of range mutated: %v", got)
	}
	if got := clampPosIn(&richdoc.Document{}, DocPos{5, 5}); got != (DocPos{}) {
		t.Errorf("clampPosIn(empty) = %+v, want zero", got)
	}
}

func TestSyntheticItalicDrawEdgeCases(t *testing.T) {
	f, _ := NewSyntheticItalicFont(NewBitmapFont(1))
	// Non-pixel back-end: falls back to the base draw (must not panic).
	f.Draw(noopPainter{}, 0, 0, "x", RGBA{A: 0xFF})
	// Empty text: nothing to render.
	buf := make([]byte, 4*10*10)
	f.Draw(painter.NewPixelPainter(buf, 10, 10), 0, 0, "", RGBA{A: 0xFF})
}
