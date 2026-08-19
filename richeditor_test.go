// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-richdoc/richdoc"
	"github.com/go-widgets/painter"
)

// --- test helpers ---------------------------------------------------------

// reRender draws e into a fresh w*h buffer positioned at the origin and returns
// the RGBA bytes. e's bounds are set to the same rect.
func reRender(e *RichEditor, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	e.Draw(p, theme)
	return buf
}

// inkAt reports whether the pixel at (x,y) has been painted (non-zero alpha).
func inkAt(buf []byte, w, x, y int) bool { return buf[(y*w+x)*4+3] != 0 }

// pixAt returns the RGBA at (x,y).
func pixAt(buf []byte, w, x, y int) RGBA {
	i := (y*w + x) * 4
	return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

// inkCount counts painted pixels in the whole buffer.
func inkCount(buf []byte, w, h int) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inkAt(buf, w, x, y) {
				n++
			}
		}
	}
	return n
}

// glyphInk counts pixels that differ from the surface fill — the actual glyph /
// chrome ink, ignoring the opaque background.
func glyphInk(buf []byte, w, h int, theme *Theme) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixAt(buf, w, x, y) != theme.Surface {
				n++
			}
		}
	}
	return n
}

// diffCount counts pixels that differ between two equal-sized buffers.
func diffCount(a, b []byte) int {
	n := 0
	for i := 0; i < len(a); i += 4 {
		if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] || a[i+3] != b[i+3] {
			n++
		}
	}
	return n
}

// walkHas reports whether any node of d satisfies match.
func walkHas(d *richdoc.Document, match func(any) bool) bool {
	v := &finder{match: match}
	richdoc.Walk(d, v)
	return v.found
}

type finder struct {
	match func(any) bool
	found bool
}

func (f *finder) Enter(n any) bool {
	if f.match(n) {
		f.found = true
	}
	return true
}
func (f *finder) Leave(n any) {}

// sampleDoc is the heading + styled paragraph + 2-item list + code block used
// across the layout/interaction tests.
func sampleDoc() *richdoc.Document {
	return richdoc.New().
		H(1, richdoc.Txt("Title")).
		P(richdoc.Bold(richdoc.Txt("bold")), richdoc.Txt(" and "), richdoc.Italic(richdoc.Txt("it"))).
		UList(true,
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("one")}}),
			richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("two")}})).
		CodeBlock("go", "x := 1").
		Doc()
}

func newSample() *RichEditor {
	e := NewRichEditor(sampleDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 400})
	return e
}

// --- layout / rendering ---------------------------------------------------

func TestHeadingMetricsDifferFromBody(t *testing.T) {
	e := newSample()
	lay := e.buildLayout(DefaultLight())
	var head, body reLine
	for _, ln := range lay.lines {
		switch ln.blockIdx {
		case 0:
			head = ln
		case 1:
			body = ln
		}
	}
	if head.h <= body.h {
		t.Errorf("heading height %d not greater than body %d", head.h, body.h)
	}
	if head.textY == body.textY {
		t.Errorf("heading baseline %d equals body %d, want a distinct row", head.textY, body.textY)
	}
	// Heading glyphs are painted through a synthetic-bold face; body is not.
	if _, ok := head.runs[0].font.(*syntheticBoldFont); !ok {
		t.Errorf("heading run font = %T, want *syntheticBoldFont", head.runs[0].font)
	}
}

func TestBoldRunAdvancesAndFonts(t *testing.T) {
	e := newSample()
	lay := e.buildLayout(DefaultLight())
	var para reLine
	for _, ln := range lay.lines {
		if ln.blockIdx == 1 {
			para = ln
			break
		}
	}
	// The paragraph is "bold and it": three runs with distinct faces.
	if len(para.runs) != 3 {
		t.Fatalf("paragraph runs = %d, want 3", len(para.runs))
	}
	if _, ok := para.runs[0].font.(*syntheticBoldFont); !ok {
		t.Errorf("run 0 font = %T, want bold", para.runs[0].font)
	}
	if _, ok := para.runs[2].font.(*syntheticItalicFont); !ok {
		t.Errorf("run 2 font = %T, want italic", para.runs[2].font)
	}
	// Cells advance by exactly one base glyph advance (monospace layout metrics),
	// so the bold run's glyph origins land where the caller expects.
	adv := e.baseFont().Advance()
	left := rePadX()
	for k := 0; k <= 4; k++ {
		if para.cellX[k] != left+k*adv {
			t.Errorf("cellX[%d] = %d, want %d", k, para.cellX[k], left+k*adv)
		}
	}
}

func TestBoldRendersMoreInkThanPlain(t *testing.T) {
	bold := NewRichEditor(richdoc.New().P(richdoc.Bold(richdoc.Txt("mmmm"))).Doc())
	plain := NewRichEditor(richdoc.New().P(richdoc.Txt("mmmm")).Doc())
	th := DefaultLight()
	nb := glyphInk(reRender(bold, 120, 40, th), 120, 40, th)
	np := glyphInk(reRender(plain, 120, 40, th), 120, 40, th)
	if nb <= np {
		t.Errorf("bold ink %d not greater than plain ink %d (over-strike missing)", nb, np)
	}
}

func TestListBulletsAndIndent(t *testing.T) {
	e := newSample()
	lay := e.buildLayout(DefaultLight())
	// The first list item's paragraph is indented one list step.
	var item0 reLine
	found := false
	for _, ln := range lay.lines {
		if ln.blockIdx == 2 && ln.hasStops {
			item0 = ln
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no caret line for the list")
	}
	wantX := rePadX() + reListIndent()
	if item0.cellX[0] != wantX {
		t.Errorf("list text indent = %d, want %d", item0.cellX[0], wantX)
	}
	// A bullet marker is painted at the list's own left edge for each item.
	bullets := 0
	for _, c := range lay.chrome {
		if c.text == "•" && c.r.X == rePadX() {
			bullets++
		}
	}
	if bullets != 2 {
		t.Errorf("bullet markers = %d, want 2", bullets)
	}
}

func TestOrderedListMarkers(t *testing.T) {
	e := NewRichEditor(richdoc.New().OList(1, true,
		richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("a")}}),
		richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("b")}})).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	lay := e.buildLayout(DefaultLight())
	got := map[string]bool{}
	for _, c := range lay.chrome {
		if c.text != "" {
			got[c.text] = true
		}
	}
	if !got["1."] || !got["2."] {
		t.Errorf("ordered markers = %v, want 1. and 2.", got)
	}
}

// --- caret geometry -------------------------------------------------------

func TestCaretPixelMatchesRun(t *testing.T) {
	e := newSample()
	x, y := e.CaretPixel(DocPos{1, 2})
	lay := e.buildLayout(DefaultLight())
	var para reLine
	for _, ln := range lay.lines {
		if ln.blockIdx == 1 {
			para = ln
			break
		}
	}
	if x != para.cellX[2] {
		t.Errorf("caret x = %d, want cellX[2] %d", x, para.cellX[2])
	}
	if y != para.textY {
		t.Errorf("caret y = %d, want textY %d", y, para.textY)
	}
}

func TestClickRoundTrip(t *testing.T) {
	e := newSample()
	for _, pos := range []DocPos{{0, 3}, {1, 5}, {2, 2}, {3, 4}} {
		x, y := e.CaretPixel(pos)
		lay := e.buildLayout(DefaultLight())
		got := e.posAtLocal(lay, x, y)
		if got != pos {
			t.Errorf("click round-trip at %+v: got %+v (pixel %d,%d)", pos, got, x, y)
		}
	}
}

func TestClickPlacesCaret(t *testing.T) {
	e := newSample()
	x, y := e.CaretPixel(DocPos{1, 3})
	e.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if !e.Focused().Get() {
		t.Error("click did not focus")
	}
	if e.Caret().Get() != (DocPos{1, 3}) {
		t.Errorf("click caret = %+v, want {1,3}", e.Caret().Get())
	}
}

func TestCaretPixelOutOfRange(t *testing.T) {
	e := newSample()
	// A block that does not exist has no caret line; CaretPixel falls back to the
	// content origin rather than panicking.
	x, y := e.CaretPixel(DocPos{99, 0})
	if x != rePadX() || y != rePadTop() {
		t.Errorf("out-of-range caret = %d,%d, want %d,%d", x, y, rePadX(), rePadTop())
	}
	// An out-of-range offset within a real block clamps to the block's last cell.
	x2, _ := e.CaretPixel(DocPos{1, 999})
	lay := e.buildLayout(DefaultLight())
	var para reLine
	for _, ln := range lay.lines {
		if ln.blockIdx == 1 {
			para = ln
		}
	}
	if x2 != para.cellX[para.nCells()] {
		t.Errorf("clamped caret x = %d, want %d", x2, para.cellX[para.nCells()])
	}
}

func TestPosAtLayoutEmpty(t *testing.T) {
	e := newSample()
	if got := e.posAtLayout(reLayout{}, 0, 0); got != (DocPos{}) {
		t.Errorf("posAtLayout(empty) = %+v, want zero", got)
	}
}

// --- editing --------------------------------------------------------------

func TestTypeInsertsText(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 2})
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	if got := blockPlain(e, 1); got != "boXld and it" {
		t.Errorf("after typing: %q, want %q", got, "boXld and it")
	}
	if e.Caret().Get() != (DocPos{1, 3}) {
		t.Errorf("caret = %+v, want {1,3}", e.Caret().Get())
	}
}

func TestEmptyCharIgnored(t *testing.T) {
	e := newSample()
	before := richdoc.PlainText(e.docValue())
	e.OnEvent(Event{Kind: EventChar, Code: ""})
	if richdoc.PlainText(e.docValue()) != before {
		t.Error("empty EventChar mutated the document")
	}
}

func TestEnterSplitsBlock(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 4}) // after "bold"
	n0 := len(e.docValue().Blocks)
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(e.docValue().Blocks) != n0+1 {
		t.Fatalf("blocks = %d, want %d", len(e.docValue().Blocks), n0+1)
	}
	if blockPlain(e, 1) != "bold" {
		t.Errorf("left block = %q, want %q", blockPlain(e, 1), "bold")
	}
	if _, ok := e.docValue().Blocks[2].(richdoc.Paragraph); !ok {
		t.Errorf("new block = %T, want Paragraph", e.docValue().Blocks[2])
	}
	if blockPlain(e, 2) != " and it" {
		t.Errorf("right block = %q, want %q", blockPlain(e, 2), " and it")
	}
	if e.Caret().Get() != (DocPos{2, 0}) {
		t.Errorf("caret = %+v, want {2,0}", e.Caret().Get())
	}
}

func TestEnterInCodeInsertsNewline(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{3, 6}) // end of "x := 1"
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	cb := e.docValue().Blocks[3].(richdoc.CodeBlock)
	if cb.Text != "x := 1\n" {
		t.Errorf("code text = %q, want %q", cb.Text, "x := 1\n")
	}
}

func TestBackspaceDeletesCell(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 4})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if blockPlain(e, 1) != "bol and it" {
		t.Errorf("after backspace: %q, want %q", blockPlain(e, 1), "bol and it")
	}
}

func TestBackspaceMergesBlocks(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 0}) // start of the paragraph
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	// The paragraph merges into the heading above it.
	if len(e.docValue().Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(e.docValue().Blocks))
	}
	if blockPlain(e, 0) != "Titlebold and it" {
		t.Errorf("merged block = %q, want %q", blockPlain(e, 0), "Titlebold and it")
	}
	if e.Caret().Get() != (DocPos{0, 5}) {
		t.Errorf("caret = %+v, want {0,5}", e.Caret().Get())
	}
}

func TestBackspaceAtStartNoop(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{0, 0})
	before := richdoc.PlainText(e.docValue())
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if richdoc.PlainText(e.docValue()) != before {
		t.Error("backspace at document start mutated the document")
	}
}

func TestDeleteForwardCell(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if blockPlain(e, 1) != "old and it" {
		t.Errorf("after delete: %q, want %q", blockPlain(e, 1), "old and it")
	}
}

func TestDeleteForwardMerges(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{0, 5}) // end of "Title"
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if len(e.docValue().Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(e.docValue().Blocks))
	}
	if blockPlain(e, 0) != "Titlebold and it" {
		t.Errorf("merged = %q, want %q", blockPlain(e, 0), "Titlebold and it")
	}
}

// --- selection ------------------------------------------------------------

func TestSelectionPaintPresentAndAbsent(t *testing.T) {
	e := newSample()
	noSel := reRender(e, 240, 400, DefaultLight())
	e.Selection().Set(DocSelection{Start: DocPos{1, 0}, End: DocPos{1, 4}})
	withSel := reRender(e, 240, 400, DefaultLight())
	if diffCount(noSel, withSel) == 0 {
		t.Error("selection produced no visible band")
	}
	// Clearing it returns to the unselected pixels.
	e.ClearSelection()
	cleared := reRender(e, 240, 400, DefaultLight())
	if diffCount(noSel, cleared) != 0 {
		t.Error("clearing the selection left a residual band")
	}
}

func TestDragExtendsSelection(t *testing.T) {
	e := newSample()
	x0, y0 := e.CaretPixel(DocPos{1, 1})
	x1, y1 := e.CaretPixel(DocPos{1, 5})
	e.OnEvent(Event{Kind: EventClick, X: x0, Y: y0})
	e.OnEvent(Event{Kind: EventMouseDrag, X: x1, Y: y1})
	sel := e.Selection().Get()
	if sel.IsEmpty() {
		t.Fatal("drag produced no selection")
	}
	if sel != (DocSelection{Start: DocPos{1, 1}, End: DocPos{1, 5}}) {
		t.Errorf("selection = %+v, want {1,1}-{1,5}", sel)
	}
}

func TestTypeReplacesSelection(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 5})
	e.Selection().Set(DocSelection{Start: DocPos{1, 0}, End: DocPos{1, 4}})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	if blockPlain(e, 1) != "Z and it" {
		t.Errorf("after replace: %q, want %q", blockPlain(e, 1), "Z and it")
	}
}

func TestDeleteSelectionEmptyNoop(t *testing.T) {
	e := newSample()
	before := richdoc.PlainText(e.docValue())
	e.DeleteSelection()
	if richdoc.PlainText(e.docValue()) != before {
		t.Error("DeleteSelection with empty selection mutated the document")
	}
}

func TestMultiBlockSelectionDelete(t *testing.T) {
	e := newSample()
	// Select from mid-heading into mid-paragraph.
	e.Selection().Set(DocSelection{Start: DocPos{0, 2}, End: DocPos{1, 4}})
	e.Caret().Set(DocPos{1, 4})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(e.docValue().Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(e.docValue().Blocks))
	}
	if blockPlain(e, 0) != "Ti and it" {
		t.Errorf("merged = %q, want %q", blockPlain(e, 0), "Ti and it")
	}
}

// --- inline formatting ----------------------------------------------------

func TestToggleStrongWrapsAndUnwraps(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("hello world")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Selection().Set(DocSelection{Start: DocPos{0, 6}, End: DocPos{0, 11}})
	e.ToggleStrong()
	if !walkHas(e.docValue(), func(n any) bool { _, ok := n.(richdoc.Strong); return ok }) {
		t.Fatal("ToggleStrong did not create a Strong node")
	}
	if richdoc.PlainText(e.docValue()) != "hello world" {
		t.Errorf("text changed: %q", richdoc.PlainText(e.docValue()))
	}
	// The selection survives so the verb composes; toggling again unwraps.
	e.ToggleStrong()
	if walkHas(e.docValue(), func(n any) bool { _, ok := n.(richdoc.Strong); return ok }) {
		t.Error("second ToggleStrong did not unwrap the Strong node")
	}
}

func TestToggleEmphStrikeCode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		toggle func(*RichEditor)
		match  func(any) bool
	}{
		{"emph", (*RichEditor).ToggleEmph, func(n any) bool { _, ok := n.(richdoc.Emph); return ok }},
		{"strike", (*RichEditor).ToggleStrikethrough, func(n any) bool { _, ok := n.(richdoc.Strikethrough); return ok }},
		{"code", (*RichEditor).ToggleCode, func(n any) bool { _, ok := n.(richdoc.Code); return ok }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewRichEditor(richdoc.New().P(richdoc.Txt("abcdef")).Doc())
			e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
			e.Selection().Set(DocSelection{Start: DocPos{0, 1}, End: DocPos{0, 4}})
			tc.toggle(e)
			if !walkHas(e.docValue(), tc.match) {
				t.Fatalf("%s: node not created", tc.name)
			}
			tc.toggle(e)
			if walkHas(e.docValue(), tc.match) {
				t.Errorf("%s: not unwrapped on second toggle", tc.name)
			}
		})
	}
}

func TestToggleNoSelectionArmsPending(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("ab")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 2})
	e.ToggleStrong() // no selection: arm bold for the next rune
	if !e.pendingActive {
		t.Fatal("pending style not armed")
	}
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	// The typed X is bold.
	if !walkHas(e.docValue(), func(n any) bool {
		s, ok := n.(richdoc.Strong)
		return ok && richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: s.Inlines}}}) == "X"
	}) {
		t.Error("typed rune after arming bold is not Strong")
	}
}

func TestTogglePartialSelectionAddsThenRemoves(t *testing.T) {
	// A selection that is only partly bold gets fully bolded first, then cleared.
	e := NewRichEditor(richdoc.New().P(richdoc.Bold(richdoc.Txt("ab")), richdoc.Txt("cd")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 4}})
	e.ToggleStrong() // not all bold -> make all bold
	rs, _ := blockContent(e.docValue().Blocks[0])
	for i, r := range rs {
		if r.style&styBold == 0 {
			t.Fatalf("cell %d not bold after first toggle", i)
		}
	}
	e.ToggleStrong() // all bold -> clear
	rs, _ = blockContent(e.docValue().Blocks[0])
	for i, r := range rs {
		if r.style&styBold != 0 {
			t.Fatalf("cell %d still bold after second toggle", i)
		}
	}
}

// --- block verbs ----------------------------------------------------------

func TestSetBlockType(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("text")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 2})

	e.SetBlockType(BlockH2)
	if h, ok := e.docValue().Blocks[0].(richdoc.Heading); !ok || h.Level != 2 {
		t.Fatalf("SetBlockType(H2) = %#v", e.docValue().Blocks[0])
	}
	e.SetBlockType(BlockCodeKind)
	if cb, ok := e.docValue().Blocks[0].(richdoc.CodeBlock); !ok || cb.Text != "text" {
		t.Fatalf("SetBlockType(Code) = %#v", e.docValue().Blocks[0])
	}
	e.SetBlockType(BlockQuoteKind)
	if _, ok := e.docValue().Blocks[0].(richdoc.BlockQuote); !ok {
		t.Fatalf("SetBlockType(Quote) = %#v", e.docValue().Blocks[0])
	}
	e.SetBlockType(BlockParagraph)
	if _, ok := e.docValue().Blocks[0].(richdoc.Paragraph); !ok {
		t.Fatalf("SetBlockType(Paragraph) = %#v", e.docValue().Blocks[0])
	}
	if blockPlain(e, 0) != "text" {
		t.Errorf("round-trip text = %q, want %q", blockPlain(e, 0), "text")
	}
}

func TestSetBlockTypeInvalidNoop(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("t")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	e.SetBlockType(BlockKind(999))
	if _, ok := e.docValue().Blocks[0].(richdoc.Paragraph); !ok {
		t.Error("invalid BlockKind changed the block")
	}
}

func TestToggleListWrapUnwrapAndSwitch(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("item")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 2})

	e.ToggleList(false) // paragraph -> bullet list
	l, ok := e.docValue().Blocks[0].(richdoc.List)
	if !ok || l.Ordered {
		t.Fatalf("ToggleList(false) = %#v", e.docValue().Blocks[0])
	}
	e.ToggleList(true) // bullet -> numbered (switch, not unwrap)
	if l, _ := e.docValue().Blocks[0].(richdoc.List); !l.Ordered {
		t.Fatalf("ToggleList(true) did not switch to ordered: %#v", e.docValue().Blocks[0])
	}
	e.ToggleList(true) // numbered -> paragraph (unwrap)
	if _, ok := e.docValue().Blocks[0].(richdoc.Paragraph); !ok {
		t.Fatalf("ToggleList unwrap = %#v", e.docValue().Blocks[0])
	}
	if blockPlain(e, 0) != "item" {
		t.Errorf("unwrapped text = %q, want %q", blockPlain(e, 0), "item")
	}
}

func TestToggleListUnwrapMultiItem(t *testing.T) {
	e := NewRichEditor(richdoc.New().UList(true,
		richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("a")}}),
		richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("b")}}),
		richdoc.Item()).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 0})
	e.ToggleList(false) // unwrap: two paragraphs + one seeded empty paragraph
	if len(e.docValue().Blocks) != 3 {
		t.Fatalf("unwrapped blocks = %d, want 3", len(e.docValue().Blocks))
	}
}

// --- navigation -----------------------------------------------------------

func TestArrowNavigation(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 1})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if e.Caret().Get() != (DocPos{1, 0}) {
		t.Errorf("left = %+v, want {1,0}", e.Caret().Get())
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // wraps to prev block end
	if e.Caret().Get() != (DocPos{0, 5}) {
		t.Errorf("left wrap = %+v, want {0,5}", e.Caret().Get())
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // wraps back
	if e.Caret().Get() != (DocPos{1, 0}) {
		t.Errorf("right wrap = %+v, want {1,0}", e.Caret().Get())
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if e.Caret().Get() != (DocPos{1, 11}) {
		t.Errorf("end = %+v, want {1,11}", e.Caret().Get())
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if e.Caret().Get() != (DocPos{1, 0}) {
		t.Errorf("home = %+v, want {1,0}", e.Caret().Get())
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if e.Caret().Get().Block != 2 {
		t.Errorf("down block = %d, want 2", e.Caret().Get().Block)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if e.Caret().Get().Block != 1 {
		t.Errorf("up block = %d, want 1", e.Caret().Get().Block)
	}
}

func TestArrowBoundsClamp(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if e.Caret().Get() != (DocPos{0, 0}) {
		t.Errorf("left at start = %+v, want {0,0}", e.Caret().Get())
	}
	last := len(e.docValue().Blocks) - 1
	e.Caret().Set(DocPos{last, blockLen(e.docValue().Blocks[last])})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if e.Caret().Get().Block != last {
		t.Errorf("right at end moved off the last block: %+v", e.Caret().Get())
	}
}

func TestShiftArrowExtendsSelection(t *testing.T) {
	e := newSample()
	e.Caret().Set(DocPos{1, 2})
	e.anchor = DocPos{1, 2}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight", Shift: true})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight", Shift: true})
	sel := e.Selection().Get()
	if sel != (DocSelection{Start: DocPos{1, 2}, End: DocPos{1, 4}}) {
		t.Errorf("shift-selection = %+v, want {1,2}-{1,4}", sel)
	}
}

func TestCtrlBoldItalicShortcuts(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("xy")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 2}})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "b", Ctrl: true})
	if !walkHas(e.docValue(), func(n any) bool { _, ok := n.(richdoc.Strong); return ok }) {
		t.Error("Ctrl+B did not bold")
	}
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 2}})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "i", Ctrl: true})
	if !walkHas(e.docValue(), func(n any) bool { _, ok := n.(richdoc.Emph); return ok }) {
		t.Error("Ctrl+I did not italicise")
	}
}

// --- scrolling ------------------------------------------------------------

func tallDoc() *richdoc.Document {
	b := richdoc.New()
	for i := 0; i < 40; i++ {
		b.P(richdoc.Txt("line of text number here"))
	}
	return b.Doc()
}

func TestScrollWheelAndScrollbar(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	buf := reRender(e, 200, 80, DefaultLight())
	// The content overflows, so a scrollbar thumb (accent) appears in the right
	// gutter column.
	track := e.scrollbarReserve()
	tx := 200 - track
	thumb := false
	for y := 0; y < 80; y++ {
		if pixAt(buf, 200, tx, y) == DefaultLight().Accent {
			thumb = true
		}
	}
	if !thumb {
		t.Error("no scrollbar thumb painted for overflowing content")
	}
	e.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if e.ScrollOffset().Get() == 0 {
		t.Error("wheel did not scroll")
	}
}

func TestScrollCaretIntoView(t *testing.T) {
	e := NewRichEditor(tallDoc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	_ = reRender(e, 200, 80, DefaultLight()) // establish lastTheme
	last := len(e.docValue().Blocks) - 1
	e.moveCaret(DocPos{last, 0}, false)
	if e.ScrollOffset().Get() == 0 {
		t.Error("caret at the bottom did not scroll the view")
	}
	// Moving back to the top scrolls up again (to within the top padding).
	e.moveCaret(DocPos{0, 0}, false)
	if e.ScrollOffset().Get() > rePadTop() {
		t.Errorf("caret at the top left scroll = %d, want <= %d", e.ScrollOffset().Get(), rePadTop())
	}
}

// --- empty document + snapshots ------------------------------------------

func TestEmptyDocument(t *testing.T) {
	e := NewRichEditor(nil)
	e.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	_ = reRender(e, 120, 60, DefaultLight()) // must not panic on an empty doc
	// Typing into an empty document seeds a paragraph.
	e.OnEvent(Event{Kind: EventChar, Code: "H"})
	e.OnEvent(Event{Kind: EventChar, Code: "i"})
	if richdoc.PlainText(e.docValue()) != "Hi" {
		t.Errorf("typed into empty doc = %q, want %q", richdoc.PlainText(e.docValue()), "Hi")
	}
}

func TestEnterOnEmptyDocument(t *testing.T) {
	e := NewRichEditor(nil)
	e.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(e.docValue().Blocks) != 1 {
		t.Errorf("blocks after Enter on empty = %d, want 1", len(e.docValue().Blocks))
	}
}

func TestDocumentSnapshotIndependent(t *testing.T) {
	e := newSample()
	snap := e.Document()
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	if richdoc.PlainText(snap) == richdoc.PlainText(e.docValue()) {
		t.Error("snapshot tracked a later edit — not an independent copy")
	}
}

func TestDocObservableNotifies(t *testing.T) {
	e := newSample()
	fired := 0
	e.Doc().Subscribe(func(*richdoc.Document) { fired++ })
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	if fired == 0 {
		t.Error("editing did not notify Doc() subscribers")
	}
}

func TestSetDocumentNilResets(t *testing.T) {
	e := newSample()
	e.SetDocument(nil)
	if len(e.docValue().Blocks) != 0 {
		t.Errorf("SetDocument(nil) blocks = %d, want 0", len(e.docValue().Blocks))
	}
	if e.Caret().Get() != (DocPos{}) {
		t.Errorf("caret = %+v, want zero", e.Caret().Get())
	}
}

func TestBareEditorWorks(t *testing.T) {
	e := &RichEditor{}
	e.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	_ = reRender(e, 100, 100, DefaultLight())
	if e.HasSelection() {
		t.Error("bare editor reports a selection")
	}
}

// --- focus + kitchen-sink rendering --------------------------------------

func TestFocusBorder(t *testing.T) {
	e := newSample()
	unfocused := reRender(e, 240, 400, DefaultLight())
	e.Focused().Set(true)
	focused := reRender(e, 240, 400, DefaultLight())
	if pixAt(unfocused, 240, 0, 0) == pixAt(focused, 240, 0, 0) {
		t.Error("focus did not change the border colour")
	}
	if pixAt(focused, 240, 0, 0) != DefaultLight().Accent {
		t.Errorf("focused border = %v, want accent", pixAt(focused, 240, 0, 0))
	}
}

// kitchenSink exercises every block + inline kind the layout renders.
func kitchenSink() *richdoc.Document {
	return richdoc.New().
		H(9, richdoc.Txt("clamped-high")). // level clamps to 6
		H(0, richdoc.Txt("clamped-low")).  // level clamps to 1
		P(
			richdoc.Href("http://x", "t", richdoc.Txt("link")),
			richdoc.Strike(richdoc.Txt("gone")),
			richdoc.Mono("code"),
			richdoc.Img("u", "alt", ""),
			richdoc.Img("u", "", ""), // empty alt -> generic label
			richdoc.InlineMath("e^x"),
			richdoc.Br(),
			richdoc.Txt("after break"),
			richdoc.RawI("html", "<b>"),
		).
		Quote(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("quoted")}}).
		Add(richdoc.BlockQuote{}).                                                                                      // empty quote
		Add(richdoc.List{Items: []richdoc.ListItem{{Blocks: []richdoc.Block{richdoc.CodeBlock{Text: "nested"}}}, {}}}). // non-paragraph + empty item
		Table([]richdoc.Alignment{richdoc.AlignLeft}, []richdoc.Cell{richdoc.Td(richdoc.Txt("H"))}, [][]richdoc.Cell{{richdoc.Td(richdoc.Txt("c"))}}).
		Add(richdoc.Table{}). // empty table
		HR().
		MathBlock("\\int x").
		RawBlock("latex", "\\foo").
		Doc()
}

func TestKitchenSinkRenders(t *testing.T) {
	e := NewRichEditor(kitchenSink())
	e.SetBounds(Rect{X: 0, Y: 0, W: 260, H: 240}) // shorter than content -> scrolls
	buf := reRender(e, 260, 240, DefaultLight())
	if inkCount(buf, 260, 240) == 0 {
		t.Fatal("kitchen-sink rendered nothing")
	}
	// Scroll to the bottom and redraw so the trailing blocks + culling paths run.
	e.OnEvent(Event{Kind: EventScroll, Delta: 200})
	_ = reRender(e, 260, 240, DefaultLight())
	// The caret can traverse every block without panicking.
	for i := range e.docValue().Blocks {
		e.CaretPixel(DocPos{i, 0})
	}
}

func TestQuoteAndListEditing(t *testing.T) {
	// A blockquote's first paragraph and a list's first item are caret-editable.
	e := NewRichEditor(richdoc.New().
		Quote(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("qq")}}).
		UList(false, richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("li")}})).
		Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 2})
	e.OnEvent(Event{Kind: EventChar, Code: "!"})
	if blockPlain(e, 0) != "qq!" {
		t.Errorf("quote edit = %q, want %q", blockPlain(e, 0), "qq!")
	}
	e.Caret().Set(DocPos{1, 2})
	e.OnEvent(Event{Kind: EventChar, Code: "!"})
	if blockPlain(e, 1) != "li!" {
		t.Errorf("list edit = %q, want %q", blockPlain(e, 1), "li!")
	}
}

func TestAtomicBlockEditing(t *testing.T) {
	// Typing on a thematic break is a no-op; Enter inserts a paragraph after it;
	// Backspace at the start of the following block removes the break.
	e := NewRichEditor(richdoc.New().HR().P(richdoc.Txt("p")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "x"}) // no-op on the rule
	if _, ok := e.docValue().Blocks[0].(richdoc.ThematicBreak); !ok {
		t.Error("typing mutated the thematic break")
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // paragraph after the rule
	if _, ok := e.docValue().Blocks[1].(richdoc.Paragraph); !ok {
		t.Errorf("Enter on atomic did not insert a paragraph: %#v", e.docValue().Blocks[1])
	}
}

func TestBackspaceRemovesAtomicPrev(t *testing.T) {
	e := NewRichEditor(richdoc.New().HR().P(richdoc.Txt("p")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // removes the rule
	if len(e.docValue().Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(e.docValue().Blocks))
	}
	if _, ok := e.docValue().Blocks[0].(richdoc.Paragraph); !ok {
		t.Error("wrong block survived removing the atomic previous block")
	}
}

func TestBackspaceRemovesAtomicCur(t *testing.T) {
	// Backspace at the start of an atomic block (after a text block) removes it.
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("p")).HR().Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{1, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(e.docValue().Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(e.docValue().Blocks))
	}
}

func TestDeleteForwardAtomicNext(t *testing.T) {
	// Delete at the end of a text block whose next block is atomic removes the rule.
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("p")).HR().Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 1})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if len(e.docValue().Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(e.docValue().Blocks))
	}
}

func TestDeleteForwardFromAtomic(t *testing.T) {
	// Delete while resting on an atomic block removes that block.
	e := NewRichEditor(richdoc.New().HR().P(richdoc.Txt("p")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if len(e.docValue().Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(e.docValue().Blocks))
	}
}

func TestDeleteForwardAtEndNoop(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("p")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	e.Caret().Set(DocPos{0, 1})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if richdoc.PlainText(e.docValue()) != "p" {
		t.Error("Delete at document end mutated the document")
	}
}

// --- wrapping -------------------------------------------------------------

func TestParagraphWraps(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("one two three four five six")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 200}) // narrow -> must wrap
	lay := e.buildLayout(DefaultLight())
	lines := 0
	for _, ln := range lay.lines {
		if ln.blockIdx == 0 {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("paragraph produced %d lines, want it to wrap", lines)
	}
	// A caret in a later visual line maps back to the same offset it came from.
	x, y := e.CaretPixel(DocPos{0, 20})
	if got := e.posAtLocal(lay, x, y); got != (DocPos{0, 20}) {
		t.Errorf("wrapped caret round-trip = %+v, want {0,20}", got)
	}
}

func TestOverlongWordDoesNotLoop(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("supercalifragilisticexpialidocious")).Doc())
	e.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200})
	lay := e.buildLayout(DefaultLight()) // must terminate
	if len(lay.lines) == 0 {
		t.Fatal("no lines")
	}
}

// --- font resizing --------------------------------------------------------

func TestResizeFontBitmapAndIdentity(t *testing.T) {
	base := NewBitmapFont(2)
	if got := resizeFont(base, 1, 1); got != base {
		t.Error("num==den should return the base face unchanged")
	}
	bigger := resizeFont(base, 2, 1)
	if bigger.Height() <= base.Height() {
		t.Errorf("scaled bitmap height %d not larger than %d", bigger.Height(), base.Height())
	}
}

func TestResizeFontTTF(t *testing.T) {
	base, err := NewTrueTypeFont(testFontTTF, 10)
	if err != nil {
		t.Fatal(err)
	}
	bigger := resizeFont(base, 2, 1)
	if bigger.Height() <= base.Height() {
		t.Errorf("scaled TTF height %d not larger than %d", bigger.Height(), base.Height())
	}
}

// unresizableFont is a minimal Font that exposes neither a bitmap scale nor sfnt
// data, so resizeFont cannot scale it and must return it unchanged.
type unresizableFont struct{}

func (unresizableFont) Advance() int                                         { return 6 }
func (unresizableFont) Height() int                                          { return 7 }
func (unresizableFont) Measure(s string) int                                 { return len(s) * 6 }
func (unresizableFont) Draw(p painter.Painter, x, y int, s string, ink RGBA) {}

func TestResizeFontFallback(t *testing.T) {
	base := unresizableFont{}
	if got := resizeFont(base, 2, 1); got != Font(base) {
		t.Error("an unresizable font should be returned unchanged")
	}
}

func TestHeadingFontFollowsTTFBase(t *testing.T) {
	// A per-widget TTF font drives the heading resize path + cache reset.
	e := NewRichEditor(richdoc.New().H(1, richdoc.Txt("H")).P(richdoc.Txt("b")).Doc())
	ttf, _ := NewTrueTypeFont(testFontTTF, 12)
	e.Font = ttf
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	lay := e.buildLayout(DefaultLight())
	var head, body reLine
	for _, ln := range lay.lines {
		if ln.blockIdx == 0 {
			head = ln
		}
		if ln.blockIdx == 1 {
			body = ln
		}
	}
	if head.h <= body.h {
		t.Errorf("TTF heading height %d not larger than body %d", head.h, body.h)
	}
}

// --- model round-trip -----------------------------------------------------

func TestFlattenRebuildRoundTrip(t *testing.T) {
	inlines := []richdoc.Inline{
		richdoc.Bold(richdoc.Txt("b")),
		richdoc.Italic(richdoc.Txt("i")),
		richdoc.Strike(richdoc.Txt("s")),
		richdoc.Mono("c"),
		richdoc.Href("u", "t", richdoc.Txt("link")),
		richdoc.Img("u", "a", ""),
		richdoc.InlineMath("x"),
		richdoc.Br(),
		richdoc.RawI("html", "<b>"),
		richdoc.Txt("plain"),
	}
	rs := flattenInlines(inlines)
	got := rebuildInlines(rs)
	a := richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: inlines}}})
	b := richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: got}}})
	if a != b {
		t.Errorf("round-trip plain text: %q != %q", a, b)
	}
	// The link, image, math, break and raw inlines survive structurally.
	d := &richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: got}}}
	for _, want := range []func(any) bool{
		func(n any) bool { _, ok := n.(richdoc.Link); return ok },
		func(n any) bool { _, ok := n.(richdoc.Image); return ok },
		func(n any) bool { _, ok := n.(richdoc.Math); return ok },
		func(n any) bool { _, ok := n.(richdoc.LineBreak); return ok },
	} {
		if !walkHas(d, want) {
			t.Error("an inline kind was lost in the round-trip")
		}
	}
}

// blockPlain is the plain text of block i of e's document.
func blockPlain(e *RichEditor, i int) string {
	b := e.docValue().Blocks[i]
	return richdoc.PlainText(&richdoc.Document{Blocks: []richdoc.Block{b}})
}
