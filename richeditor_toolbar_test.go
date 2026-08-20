// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-iconoir/iconoir"
	"github.com/go-richdoc/richdoc"
	"github.com/go-widgets/painter"
)

// button indices into RichEditorToolbar.buttons (grouped order).
const (
	tbBold = iota
	tbItalic
	tbStrike
	tbCode
	tbParagraph
	tbH1
	tbH2
	tbH3
	tbQuote
	tbCodeBlock
	tbBullet
	tbNumbered
)

// newToolbarOn builds an editor over doc (caret parked at start) plus a bound
// toolbar laid out at r.
func newToolbarOn(doc *richdoc.Document, r Rect) (*RichEditor, *RichEditorToolbar) {
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 400})
	t := NewRichEditorToolbar(e)
	t.SetBounds(r)
	return e, t
}

func paraDoc(text string) *richdoc.Document {
	return richdoc.New().P(richdoc.Txt(text)).Doc()
}

// --- construction & layout ------------------------------------------------

func TestToolbarButtonCountAndDefaults(t *testing.T) {
	_, tb := newToolbarOn(paraDoc("hi"), Rect{X: 0, Y: 0, W: 600, H: 28})
	if got := len(tb.buttons); got != 12 {
		t.Fatalf("button count = %d, want 12", got)
	}
	if tb.IconSize != RichEditorToolbarIconSize || tb.Spacing != RichEditorToolbarSpacing {
		t.Fatalf("defaults not applied: IconSize=%d Spacing=%d", tb.IconSize, tb.Spacing)
	}
	if tb.Editor() == nil {
		t.Fatal("Editor() nil for a bound toolbar")
	}
}

func TestToolbarButtonPositionsGrouped(t *testing.T) {
	r := Rect{X: 10, Y: 5, W: 600, H: 28}
	_, tb := newToolbarOn(paraDoc("hi"), r)

	// Replay the same width sequence the builder feeds the HBox: a divider cell
	// before every group after the first, then one icon-button cell per spec.
	icon, sp, sepW := tb.iconSize(), tb.spacing(), tb.sepW()
	var widths []int
	var isButton []bool
	for gi, group := range reToolbarGroups {
		if gi > 0 {
			widths = append(widths, sepW)
			isButton = append(isButton, false)
		}
		for range group {
			widths = append(widths, icon)
			isButton = append(isButton, true)
		}
	}
	// Cumulative x with inter-child gaps, collecting the button x's in order.
	x := r.X
	var wantX []int
	for j := range widths {
		if isButton[j] {
			wantX = append(wantX, x)
		}
		x += widths[j] + sp
	}
	if len(wantX) != len(tb.buttons) {
		t.Fatalf("computed %d button slots, have %d buttons", len(wantX), len(tb.buttons))
	}
	for i, b := range tb.buttons {
		if got := b.Bounds().X; got != wantX[i] {
			t.Fatalf("button %d x = %d, want %d", i, got, wantX[i])
		}
		if b.Bounds().W != icon {
			t.Fatalf("button %d width = %d, want %d", i, b.Bounds().W, icon)
		}
	}
	// The block group must start strictly after the inline group + one divider:
	// a real gap proves the separator cell was inserted between groups.
	inlineEnd := tb.buttons[tbCode].Bounds().X + icon
	if tb.buttons[tbParagraph].Bounds().X <= inlineEnd+sepW-1 {
		t.Fatalf("no divider gap between inline and block groups: code ends %d, paragraph at %d",
			inlineEnd, tb.buttons[tbParagraph].Bounds().X)
	}
}

// --- icon resolution ------------------------------------------------------

func TestReToolbarIconResolution(t *testing.T) {
	if reToolbarIcon("", 4) != nil {
		t.Fatal("empty name should resolve to nil (text-glyph fallback)")
	}
	if reToolbarIcon("definitely-not-an-iconoir-name", 4) != nil {
		t.Fatal("absent icon should resolve to nil (text-glyph fallback)")
	}
	if reToolbarIcon("bold", 4) == nil {
		t.Fatal("present icon 'bold' should resolve to a painter")
	}
}

func TestEveryDeclaredIconExists(t *testing.T) {
	for _, group := range reToolbarGroups {
		for _, spec := range group {
			if spec.icon == "" {
				continue // heading buttons intentionally use a text glyph
			}
			if _, ok := iconoir.Get(spec.icon); !ok {
				t.Errorf("declared iconoir icon %q does not exist", spec.icon)
			}
		}
	}
}

// --- click → verb → tree --------------------------------------------------

func TestToolbarBoldClickTogglesStrong(t *testing.T) {
	e, tb := newToolbarOn(paraDoc("hello"), Rect{X: 0, Y: 0, W: 600, H: 28})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})

	isStrong := func() bool {
		return walkHas(e.Document(), func(n any) bool { _, ok := n.(richdoc.Strong); return ok })
	}
	if isStrong() {
		t.Fatal("precondition: doc should have no Strong")
	}
	// Drive the click through the widget's own event path (HBox routing) at the
	// Bold button's cell centre.
	bx := tb.buttons[tbBold]
	local := bx.Bounds().X - tb.Bounds().X + tb.iconSize()/2
	tb.OnEvent(Event{Kind: EventClick, X: local, Y: tb.iconSize() / 2})
	if !isStrong() {
		t.Fatal("Bold click did not make the selection Strong")
	}
	if !tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold button not lit while the selection is Strong")
	}
	// Toggling again removes it.
	tb.OnEvent(Event{Kind: EventClick, X: local, Y: tb.iconSize() / 2})
	if isStrong() {
		t.Fatal("second Bold click did not remove Strong")
	}
	if tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold button still lit after Strong removed")
	}
}

func TestToolbarBlockButtonsChangeKind(t *testing.T) {
	e, tb := newToolbarOn(paraDoc("hello"), Rect{X: 0, Y: 0, W: 600, H: 28})

	cases := []struct {
		idx  int
		want BlockKind
	}{
		{tbH2, BlockH2},
		{tbH1, BlockH1},
		{tbH3, BlockH3},
		{tbQuote, BlockQuoteKind},
		{tbCodeBlock, BlockCodeKind},
		{tbParagraph, BlockParagraph},
	}
	for _, c := range cases {
		tb.buttons[c.idx].OnClick()
		if got := e.CurrentBlockKind(); got != c.want {
			t.Fatalf("after clicking button %d, CurrentBlockKind = %d, want %d", c.idx, got, c.want)
		}
		// Exactly the matching block button is lit within the block group.
		for _, bi := range []int{tbParagraph, tbH1, tbH2, tbH3, tbQuote, tbCodeBlock} {
			lit := tb.buttons[bi].Selected().Get()
			if want := bi == c.idx; lit != want {
				t.Fatalf("block button %d lit=%v, want %v (active kind %d)", bi, lit, want, c.want)
			}
		}
	}
}

func TestToolbarListButtonsFlip(t *testing.T) {
	e, tb := newToolbarOn(paraDoc("item"), Rect{X: 0, Y: 0, W: 600, H: 28})

	// Not a list yet: neither list button lit.
	if tb.buttons[tbBullet].Selected().Get() || tb.buttons[tbNumbered].Selected().Get() {
		t.Fatal("list buttons lit before any list exists")
	}
	// Bullet list.
	tb.buttons[tbBullet].OnClick()
	if ord, isList := e.CurrentListOrdered(); !isList || ord {
		t.Fatalf("after Bullet: ordered=%v isList=%v, want unordered list", ord, isList)
	}
	if !tb.buttons[tbBullet].Selected().Get() || tb.buttons[tbNumbered].Selected().Get() {
		t.Fatal("Bullet should be lit, Numbered not, inside a bullet list")
	}
	// Convert to numbered.
	tb.buttons[tbNumbered].OnClick()
	if ord, isList := e.CurrentListOrdered(); !isList || !ord {
		t.Fatalf("after Numbered: ordered=%v isList=%v, want ordered list", ord, isList)
	}
	if tb.buttons[tbBullet].Selected().Get() || !tb.buttons[tbNumbered].Selected().Get() {
		t.Fatal("Numbered should be lit, Bullet not, inside an ordered list")
	}
	// Clicking Numbered again unwraps back to a plain paragraph.
	tb.buttons[tbNumbered].OnClick()
	if _, isList := e.CurrentListOrdered(); isList {
		t.Fatal("second Numbered click did not unwrap the list")
	}
}

// --- active-state reflection follows the caret ----------------------------

func TestToolbarActiveStateFollowsCaret(t *testing.T) {
	// "hello world": bold "hello" only, then probe the caret in a bold run and
	// in a plain run.
	e, tb := newToolbarOn(paraDoc("hello world"), Rect{X: 0, Y: 0, W: 600, H: 28})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})
	tb.buttons[tbBold].OnClick() // bold "hello"

	// Collapsed caret inside the bold run → Bold lit.
	e.Selection().Set(DocSelection{Start: DocPos{0, 2}, End: DocPos{0, 2}})
	e.Caret().Set(DocPos{0, 2})
	if !tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold not lit with the caret inside a bold run")
	}
	// Caret in the plain run → Bold not lit.
	e.Caret().Set(DocPos{0, 8})
	if tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold lit with the caret in a plain run")
	}

	// Caret in an H2 heading → H2 lit, H1 not.
	e2, tb2 := newToolbarOn(richdoc.New().H(2, richdoc.Txt("Head")).Doc(), Rect{X: 0, Y: 0, W: 600, H: 28})
	e2.Caret().Set(DocPos{0, 1})
	if !tb2.buttons[tbH2].Selected().Get() {
		t.Fatal("H2 not lit with the caret in an H2 heading")
	}
	if tb2.buttons[tbH1].Selected().Get() {
		t.Fatal("H1 lit with the caret in an H2 heading")
	}
}

func TestToolbarPendingInlineStyleLights(t *testing.T) {
	// A collapsed caret with no selection: clicking Bold arms the pending style,
	// which is not carried on any Observable — the button must still light
	// because OnClick refreshes.
	_, tb := newToolbarOn(paraDoc("hello"), Rect{X: 0, Y: 0, W: 600, H: 28})
	if tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold lit before any interaction")
	}
	tb.buttons[tbBold].OnClick()
	if !tb.buttons[tbBold].Selected().Get() {
		t.Fatal("Bold not lit after arming the pending style on a collapsed caret")
	}
}

// --- painting -------------------------------------------------------------

func TestIconoirDrawPaintsInk(t *testing.T) {
	// Control-style: iconoir.Draw reports the name exists AND paints a non-empty
	// inked region for every real icon the toolbar declares.
	const sz = 24
	ink := RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}
	for _, group := range reToolbarGroups {
		for _, spec := range group {
			if spec.icon == "" {
				continue
			}
			buf := make([]byte, 4*sz*sz)
			p := painter.NewPixelPainter(buf, sz, sz)
			if !iconoir.Draw(p, Rect{X: 0, Y: 0, W: sz, H: sz}, spec.icon, ink) {
				t.Errorf("iconoir.Draw(%q) returned false", spec.icon)
				continue
			}
			painted := 0
			for i := 3; i < len(buf); i += 4 {
				if buf[i] != 0 {
					painted++
				}
			}
			if painted == 0 {
				t.Errorf("iconoir.Draw(%q) painted no ink", spec.icon)
			}
		}
	}
}

func TestToolbarDrawPaintsButtons(t *testing.T) {
	const w, h = 600, 28
	theme := DefaultLight()
	_, tb := newToolbarOn(paraDoc("hi"), Rect{X: 0, Y: 0, W: w, H: h})
	buf := make([]byte, 4*w*h)
	tb.Draw(painter.NewPixelPainter(buf, w, h), theme)

	// The Bold button (an iconoir glyph) paints ink differing from the surface.
	if inkInRect(buf, w, tb.buttons[tbBold].Bounds(), theme.Surface) == 0 {
		t.Fatal("Bold button painted no icon ink")
	}
	// The H1 button (a text-glyph fallback, Icon nil) paints its "H1" caption.
	if tb.buttons[tbH1].Icon != nil {
		t.Fatal("H1 button should have no iconoir Icon (text-glyph fallback)")
	}
	if inkInRect(buf, w, tb.buttons[tbH1].Bounds(), theme.Surface) == 0 {
		t.Fatal("H1 button painted no glyph ink")
	}
}

func TestToolbarSeparatorDrawsDivider(t *testing.T) {
	const w, h = 20, 28
	theme := DefaultLight()
	s := &reToolbarSeparator{}
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := make([]byte, 4*w*h)
	s.Draw(painter.NewPixelPainter(buf, w, h), theme)
	found := false
	cx := w / 2
	for y := 0; y < h; y++ {
		if pixAt(buf, w, cx, y) == theme.Border {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("separator drew no divider line at its centre")
	}
}

// inkInRect counts pixels inside r that differ from the surface fill.
func inkInRect(buf []byte, w int, r Rect, surface RGBA) int {
	n := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if pixAt(buf, w, x, y) != surface {
				n++
			}
		}
	}
	return n
}

// --- measure, dispose, nil editor -----------------------------------------

func TestToolbarMeasure(t *testing.T) {
	_, tb := newToolbarOn(paraDoc("hi"), Rect{X: 0, Y: 0, W: 600, H: 28})
	wgot, hgot := tb.Measure(1000, 1000)
	icon, sp, sepW := tb.iconSize(), tb.spacing(), tb.sepW()
	children := len(tb.buttons) + (len(reToolbarGroups) - 1)
	wantW := len(tb.buttons)*icon + (len(reToolbarGroups)-1)*sepW + (children-1)*sp
	if wgot != wantW || hgot != icon {
		t.Fatalf("Measure = (%d,%d), want (%d,%d)", wgot, hgot, wantW, icon)
	}
}

func TestToolbarDisposeStopsUpdates(t *testing.T) {
	e, tb := newToolbarOn(paraDoc("hello"), Rect{X: 0, Y: 0, W: 600, H: 28})
	if len(tb.subs) == 0 {
		t.Fatal("a bound toolbar should hold subscriptions")
	}
	tb.Dispose()
	if len(tb.subs) != 0 {
		t.Fatal("Dispose should drop all subscriptions")
	}
	// After Dispose, moving the caret no longer relights buttons: bold "hello",
	// snapshot Bold's lit state, dispose, then move the caret off the bold run —
	// the (now-detached) toolbar must not update.
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 5}})
	tb.buttons[tbBold].OnClick() // bold + refresh (still lit)
	tb.Dispose()
	lit := tb.buttons[tbBold].Selected().Get()
	e.Caret().Set(DocPos{0, 3})
	e.Selection().Set(DocSelection{}) // caret-only in the bold run
	if tb.buttons[tbBold].Selected().Get() != lit {
		t.Fatal("disposed toolbar still reacted to a caret change")
	}
	// Dispose is idempotent.
	tb.Dispose()
}

func TestToolbarNilEditorIsInert(t *testing.T) {
	tb := NewRichEditorToolbar(nil)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 28})
	if tb.Editor() != nil {
		t.Fatal("Editor() should be nil")
	}
	if len(tb.subs) != 0 {
		t.Fatal("a nil-editor toolbar should hold no subscriptions")
	}
	// Clicking a button is a safe no-op (no verb, no panic) and lights nothing.
	tb.buttons[tbBold].OnClick()
	if tb.buttons[tbBold].Selected().Get() {
		t.Fatal("a nil-editor toolbar should never light a button")
	}
	// Draw + Dispose are safe too.
	buf := make([]byte, 4*600*28)
	tb.Draw(painter.NewPixelPainter(buf, 600, 28), DefaultLight())
	tb.Dispose()
}

// --- query helpers --------------------------------------------------------

func TestActiveInlineStylesSelectionIntersection(t *testing.T) {
	// "ab": bold both, italic only "a". Over the whole selection Strong is
	// active (all cells) but Emph is not (only one cell).
	e := NewRichEditor(paraDoc("ab"))
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 2}})
	e.ToggleStrong()
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 1}})
	e.ToggleEmph()
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 2}})
	got := e.ActiveInlineStyles()
	if !got.Strong || got.Emph || got.Strikethrough || got.Code {
		t.Fatalf("ActiveInlineStyles over mixed selection = %+v, want Strong only", got)
	}
}

func TestActiveInlineStylesEmptyAndCode(t *testing.T) {
	e := NewRichEditor(&richdoc.Document{}) // no editable blocks
	// A selection that covers no editable cell → no active styles.
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{1, 0}})
	if got := e.ActiveInlineStyles(); got != (InlineStyles{}) {
		t.Fatalf("empty-doc selection styles = %+v, want none", got)
	}

	e2 := NewRichEditor(paraDoc("x"))
	e2.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 1}})
	e2.ToggleCode()
	e2.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{0, 1}})
	if got := e2.ActiveInlineStyles(); !got.Code {
		t.Fatalf("Code not reported active over a code run: %+v", got)
	}
}

func TestCurrentBlockKindAllTypes(t *testing.T) {
	d := richdoc.New().
		P(richdoc.Txt("p")).
		H(3, richdoc.Txt("h")).
		CodeBlock("go", "x").
		UList(false, richdoc.Item(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("i")}})).
		Doc()
	// Append a block quote via the model verb so we cover BlockQuoteKind too.
	e := NewRichEditor(d)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})

	want := []BlockKind{BlockParagraph, BlockH3, BlockCodeKind, BlockParagraph /* list */}
	for i, w := range want {
		e.Caret().Set(DocPos{i, 0})
		if got := e.CurrentBlockKind(); got != w {
			t.Fatalf("block %d kind = %d, want %d", i, got, w)
		}
	}
	// Quote via SetBlockType, then re-read.
	e.Caret().Set(DocPos{0, 0})
	e.SetBlockType(BlockQuoteKind)
	if got := e.CurrentBlockKind(); got != BlockQuoteKind {
		t.Fatalf("after SetBlockType(Quote), kind = %d, want %d", got, BlockQuoteKind)
	}
	// Out-of-range caret defaults to paragraph.
	e.Caret().Set(DocPos{99, 0})
	if got := e.CurrentBlockKind(); got != BlockParagraph {
		t.Fatalf("out-of-range caret kind = %d, want BlockParagraph", got)
	}
}

func TestBlockKindOfHeadingLevelClamp(t *testing.T) {
	if got := blockKindOf(richdoc.Heading{Level: 0}); got != BlockH1 {
		t.Fatalf("Heading level 0 → %d, want BlockH1 (clamped)", got)
	}
	if got := blockKindOf(richdoc.Heading{Level: 9}); got != BlockH6 {
		t.Fatalf("Heading level 9 → %d, want BlockH6 (clamped)", got)
	}
}

func TestCurrentListOrderedOutOfRange(t *testing.T) {
	e := NewRichEditor(paraDoc("x"))
	e.Caret().Set(DocPos{5, 0})
	if ord, isList := e.CurrentListOrdered(); ord || isList {
		t.Fatalf("out-of-range caret list = (%v,%v), want (false,false)", ord, isList)
	}
	// A non-list block reports isList false.
	e.Caret().Set(DocPos{0, 0})
	if _, isList := e.CurrentListOrdered(); isList {
		t.Fatal("a paragraph should report isList=false")
	}
}

func TestToolbarSizeResolvers(t *testing.T) {
	tb := NewRichEditorToolbar(nil)
	// IconSize: an explicit value is honoured; zero falls back to the default.
	tb.IconSize = 40
	if tb.iconSize() != 40 {
		t.Fatalf("iconSize() = %d, want 40", tb.iconSize())
	}
	tb.IconSize = 0
	if tb.iconSize() != RichEditorToolbarIconSize {
		t.Fatalf("iconSize() = %d, want default %d", tb.iconSize(), RichEditorToolbarIconSize)
	}
	// Spacing: a non-negative value is honoured; a negative one clamps to 0.
	tb.Spacing = 5
	if tb.spacing() != 5 {
		t.Fatalf("spacing() = %d, want 5", tb.spacing())
	}
	tb.Spacing = -3
	if tb.spacing() != 0 {
		t.Fatalf("spacing() = %d, want 0 (clamped)", tb.spacing())
	}
}

func TestActiveInlineStylesNegativeStartBlock(t *testing.T) {
	e := NewRichEditor(paraDoc("x"))
	// A selection whose start block is negative: the read-only cell walk must
	// skip the out-of-range block and still process block 0 without panicking.
	e.Selection().Set(DocSelection{Start: DocPos{-1, 0}, End: DocPos{0, 1}})
	if got := e.ActiveInlineStyles(); got != (InlineStyles{}) {
		t.Fatalf("styles over a plain run = %+v, want none", got)
	}
}

func TestActiveInlineStylesSkipsAtomicBlock(t *testing.T) {
	// A selection spanning a non-editable (atomic) block: the read-only cell walk
	// must skip it and still visit the editable blocks on either side.
	d := &richdoc.Document{Blocks: []richdoc.Block{
		richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Strong{Inlines: []richdoc.Inline{richdoc.Txt("a")}}}},
		richdoc.ThematicBreak{},
		richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Strong{Inlines: []richdoc.Inline{richdoc.Txt("b")}}}},
	}}
	e := NewRichEditor(d)
	e.Selection().Set(DocSelection{Start: DocPos{0, 0}, End: DocPos{2, 1}})
	if got := e.ActiveInlineStyles(); !got.Strong {
		t.Fatalf("Strong should be active across both bold runs (atomic block skipped): %+v", got)
	}
}

func TestToolbarA11yAndChildren(t *testing.T) {
	_, tb := newToolbarOn(paraDoc("hi"), Rect{X: 0, Y: 0, W: 600, H: 28})
	if a := tb.A11y(); a.Role != RoleToolbar {
		t.Fatalf("A11y Role = %q, want %q", a.Role, RoleToolbar)
	}
	// The walk descends into the strip and announces the individual buttons.
	nodes := WalkA11y(tb)
	buttons := 0
	for _, n := range nodes {
		if n.Role == RoleButton {
			buttons++
		}
	}
	if buttons != len(tb.buttons) {
		t.Fatalf("WalkA11y found %d button nodes, want %d", buttons, len(tb.buttons))
	}
}
