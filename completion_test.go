// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- test instruments -----------------------------------------------------
//
// staticSource is the control probe the trigger/filter tests rely on: a fixed
// candidate list plus a call counter, so a test can both assert what the popup
// filtered AND prove the editor actually consulted the source. Validated
// against a known-good baseline (TestStaticSourceControl) before any behaviour
// test trusts it.
type staticSource struct {
	items []CompletionItem
	calls int
	line  int
	col   int
}

func (s *staticSource) fn(_ []string, line, col int) []CompletionItem {
	s.calls++
	s.line, s.col = line, col
	return s.items
}

func latexItems() []CompletionItem {
	return []CompletionItem{
		{Label: "\\section", Kind: CompletionKeyword, Detail: "sectioning", InsertText: "\\section{$0}"},
		{Label: "\\subsection", Kind: CompletionKeyword, InsertText: "\\subsection{$0}"},
		{Label: "\\select", Kind: CompletionKeyword},
		{Label: "\\textbf", Kind: CompletionSnippet, InsertText: "\\textbf{$0}"},
	}
}

// backslashWord admits '\' into the current word so a LaTeX command filters as
// one token (the go-tex playground sets exactly this).
func backslashWord(r rune) bool { return r == '\\' || defaultWordChar(r) }

// newCompletionEditor builds a CodeEditor wired to a static LaTeX source with a
// backslash-aware word rule, laid out large enough for the popup, ready to type
// into.
func newCompletionEditor() (*CodeEditor, *staticSource) {
	src := &staticSource{items: latexItems()}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.CompletionWordChar = backslashWord
	c.SetBounds(Rect{X: 0, Y: 0, W: 260, H: 200})
	return c, src
}

// typeString feeds each rune of s as an EventChar, exactly as a host relays
// keystrokes.
func typeString(c *CodeEditor, s string) {
	for _, r := range s {
		c.OnEvent(Event{Kind: EventChar, Code: string(r)})
	}
}

func TestStaticSourceControl(t *testing.T) {
	s := &staticSource{items: latexItems()}
	if s.calls != 0 {
		t.Fatalf("fresh probe calls = %d, want 0", s.calls)
	}
	got := s.fn([]string{"x"}, 3, 7)
	if s.calls != 1 {
		t.Fatalf("after 1 call, calls = %d, want 1", s.calls)
	}
	if s.line != 3 || s.col != 7 {
		t.Errorf("recorded caret = (%d,%d), want (3,7)", s.line, s.col)
	}
	if len(got) != 4 || got[0].Label != "\\section" {
		t.Errorf("returned items = %v, want the 4 latex items", got)
	}
}

// --- model: kind glyphs + colours -----------------------------------------

func TestCompletionKindGlyphAndColorTotal(t *testing.T) {
	th := DefaultLight()
	seen := map[string]bool{}
	for k := CompletionText; k < completionKindEnd; k++ {
		g := completionKindGlyph(k)
		if g == "" {
			t.Errorf("kind %d has an empty glyph", k)
		}
		seen[g] = true
		if col := completionKindColor(k, th); col.A == 0 {
			t.Errorf("kind %d glyph colour is transparent (%+v)", k, col)
		}
	}
	// The distinct kinds must not all collapse onto one glyph.
	if len(seen) < 5 {
		t.Errorf("only %d distinct kind glyphs, want a legible legend", len(seen))
	}
}

// --- model: filtering ------------------------------------------------------

func TestFilterCompletionsPrefixAndFilterText(t *testing.T) {
	items := []CompletionItem{
		{Label: "Alpha"},
		{Label: "alphabet"},
		{Label: "Beta"},
		{Label: "shown", FilterText: "zzz"}, // filters on FilterText, not Label
	}
	// Empty word matches everything, in source order.
	if got := filterCompletions(items, ""); len(got) != 4 {
		t.Fatalf("empty word matched %d, want all 4", len(got))
	}
	// Case-insensitive prefix on Label.
	got := filterCompletions(items, "al")
	if len(got) != 2 || got[0].Label != "Alpha" || got[1].Label != "alphabet" {
		t.Errorf("prefix \"al\" = %v, want [Alpha alphabet]", got)
	}
	// FilterText overrides Label: "zz" reaches the "shown" item.
	if got := filterCompletions(items, "zz"); len(got) != 1 || got[0].Label != "shown" {
		t.Errorf("prefix \"zz\" = %v, want [shown]", got)
	}
	// A prefix nothing starts with narrows to empty.
	if got := filterCompletions(items, "xyz"); len(got) != 0 {
		t.Errorf("prefix \"xyz\" = %v, want none", got)
	}
}

func TestCompletionFilterTextFallback(t *testing.T) {
	if got := completionFilterText(CompletionItem{Label: "L"}); got != "L" {
		t.Errorf("no FilterText: got %q, want the Label", got)
	}
	if got := completionFilterText(CompletionItem{Label: "L", FilterText: "F"}); got != "F" {
		t.Errorf("with FilterText: got %q, want %q", got, "F")
	}
}

// --- model: snippet ($0) expansion ----------------------------------------

func TestParseSnippet(t *testing.T) {
	cases := []struct {
		in    string
		body  string
		caret int
	}{
		{"plain", "plain", 5},        // no marker: caret after body
		{"f($0)", "f()", 2},          // bare $0: caret at the stop
		{"f(${0:x})", "f(x)", 2},     // braced ${0:default}: caret before the kept default
		{"a$1b", "ab", 2},            // bare $1 placeholder: stripped, no caret move
		{"a${1:name}b", "anameb", 6}, // ${1:default}: default kept, wrapper stripped
		{"a${2}b", "ab", 2},          // ${2} no default: stripped
		{"cost $5", "cost ", 5},      // '$5' is a bare numbered stop -> stripped
		{"a$b", "a$b", 3},            // '$' not followed by a digit or brace: literal
		{"a${b}", "a${b}", 5},        // '${' not followed by a digit: literal
		{"a${1", "a${1", 4},          // unterminated brace (no default): literal
		{"a${1:x", "a${1:x", 6},      // unterminated brace after ':' : literal
		{"$0$0", "", 0},              // second $0 ignored (caret already placed)
		{"${0:a}${0:b}", "ab", 0},    // second braced $0 ignored, both defaults kept
		{"end$", "end$", 4},          // trailing lone '$'
	}
	for _, tc := range cases {
		body, caret := parseSnippet(tc.in)
		if body != tc.body || caret != tc.caret {
			t.Errorf("parseSnippet(%q) = (%q,%d), want (%q,%d)", tc.in, body, caret, tc.body, tc.caret)
		}
	}
}

// --- accessors + lazy init -------------------------------------------------

func TestCompletionAccessorsLazyInit(t *testing.T) {
	// A bare struct literal (no constructor) must lazy-init the observables.
	c := &CodeEditor{TextView: NewTextView("")}
	if c.CompletionActive() {
		t.Error("fresh editor should not have completion active")
	}
	if c.CompletionSelected() != 0 {
		t.Errorf("fresh selected = %d, want 0", c.CompletionSelected())
	}
	if c.CompletionOpen() == nil || c.CompletionSelection() == nil {
		t.Error("Observable accessors must never return nil")
	}
	if got := c.CompletionItems(); got != nil {
		t.Errorf("fresh items = %v, want nil", got)
	}
	if got := c.CompletionBounds(); got != (Rect{}) {
		t.Errorf("closed popup bounds = %+v, want zero", got)
	}
}

// --- triggering ------------------------------------------------------------

func TestCompletionOpensOnIdentifierChar(t *testing.T) {
	c, src := newCompletionEditor()
	typeString(c, "\\se")
	if !c.CompletionActive() {
		t.Fatal("typing an identifier prefix should open the popup")
	}
	if src.calls == 0 {
		t.Fatal("the editor never consulted the CompletionSource")
	}
	// Filtered to the two commands starting "\se": \section, \select.
	got := c.CompletionItems()
	if len(got) != 2 || got[0].Label != "\\section" || got[1].Label != "\\select" {
		t.Fatalf("filtered = %v, want [\\section \\select]", got)
	}
	if c.Text().Get() != "\\se" {
		t.Errorf("buffer = %q, want the typed \\se", c.Text().Get())
	}
}

func TestCompletionClosesOnNonIdentifierChar(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	if !c.CompletionActive() {
		t.Fatal("precondition: popup open")
	}
	typeString(c, " ") // a space is not a word char
	if c.CompletionActive() {
		t.Error("a non-identifier char should close the popup")
	}
}

func TestCompletionNarrowsToZeroCloses(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	if !c.CompletionActive() {
		t.Fatal("precondition: popup open")
	}
	typeString(c, "q") // "\seq" matches nothing
	if c.CompletionActive() {
		t.Error("a filter that narrows to zero should close the popup")
	}
}

func TestCompletionNilSourceNoPopup(t *testing.T) {
	c := NewCodeEditor("")
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 120})
	typeString(c, "abc")
	if c.CompletionActive() {
		t.Error("with no CompletionSource, typing must not open a popup")
	}
	if c.Text().Get() != "abc" {
		t.Errorf("buffer = %q, want abc (typing still edits)", c.Text().Get())
	}
	// Explicit trigger with no source is a safe no-op too.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	if c.CompletionActive() {
		t.Error("Ctrl+Space with no source must not open a popup")
	}
}

func TestCompletionExplicitTriggerEncodings(t *testing.T) {
	// Every accepted encoding of the trigger chord opens the popup with the
	// full (empty-prefix) candidate list.
	encodings := []Event{
		{Kind: EventKeyDown, Code: "Ctrl+Space"},
		{Kind: EventKeyDown, Code: "Meta+Space"},
		{Kind: EventKeyDown, Code: "Ctrl+ "},
		{Kind: EventKeyDown, Code: "Meta+ "},
		{Kind: EventKeyDown, Code: " ", Ctrl: true},
		{Kind: EventKeyDown, Code: "Space", Meta: true},
	}
	for _, ev := range encodings {
		c, _ := newCompletionEditor()
		c.OnEvent(ev)
		if !c.CompletionActive() {
			t.Errorf("trigger %+v did not open the popup", ev)
		}
		if len(c.CompletionItems()) != 4 {
			t.Errorf("trigger %+v: items = %d, want all 4", ev, len(c.CompletionItems()))
		}
	}
	// A plain space with no modifier is NOT a trigger.
	if isCompletionTrigger(Event{Kind: EventKeyDown, Code: " "}) {
		t.Error("plain space must not be a trigger")
	}
	if isCompletionTrigger(Event{Kind: EventKeyDown, Code: "x"}) {
		t.Error("an ordinary key must not be a trigger")
	}
}

// --- keyboard navigation while open ---------------------------------------

func TestCompletionKeyNavigationClamps(t *testing.T) {
	c, _ := newCompletionEditor()
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"}) // all 4 items
	before := c.Text().Get()

	down := func() { c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) }
	up := func() { c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"}) }

	if c.CompletionSelected() != 0 {
		t.Fatalf("initial selection = %d, want 0", c.CompletionSelected())
	}
	down()
	if c.CompletionSelected() != 1 {
		t.Fatalf("after Down: %d, want 1", c.CompletionSelected())
	}
	// Clamp at the bottom (no wraparound).
	for i := 0; i < 10; i++ {
		down()
	}
	if got := c.CompletionSelected(); got != 3 {
		t.Fatalf("selection clamped at bottom = %d, want 3", got)
	}
	// Clamp at the top.
	for i := 0; i < 10; i++ {
		up()
	}
	if got := c.CompletionSelected(); got != 0 {
		t.Fatalf("selection clamped at top = %d, want 0", got)
	}
	// PageDown/PageUp move by a page, clamped.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if got := c.CompletionSelected(); got != 3 {
		t.Fatalf("PageDown = %d, want clamp to 3", got)
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "PageUp"})
	if got := c.CompletionSelected(); got != 0 {
		t.Fatalf("PageUp = %d, want clamp to 0", got)
	}
	// Navigation must NOT touch the buffer, and the popup stays open.
	if c.Text().Get() != before {
		t.Errorf("buffer changed during navigation: %q, want %q", c.Text().Get(), before)
	}
	if !c.CompletionActive() {
		t.Error("navigation should not close the popup")
	}
}

func TestCompletionEscapeCloses(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	before := c.Text().Get()
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if c.CompletionActive() {
		t.Error("Escape should close the popup")
	}
	if c.Text().Get() != before {
		t.Errorf("Escape changed the buffer: %q, want %q", c.Text().Get(), before)
	}
}

func TestCompletionCaretMoveCloses(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	if !c.CompletionActive() {
		t.Fatal("precondition: open")
	}
	// ArrowLeft is a caret move: it closes the popup AND still moves the caret.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if c.CompletionActive() {
		t.Error("a caret move should close the popup")
	}
	if c.CursorCol().Get() != 2 {
		t.Errorf("caret col = %d, want 2 (ArrowLeft still reached the buffer)", c.CursorCol().Get())
	}
}

func TestCompletionBackspaceRefilters(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\sec") // "\sec" -> only \section
	if got := c.CompletionItems(); len(got) != 1 || got[0].Label != "\\section" {
		t.Fatalf("\\sec filtered = %v, want [\\section]", got)
	}
	// Backspace to "\se": the list widens back to \section + \select and the
	// popup stays open (the deletion reached the buffer).
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c.Text().Get() != "\\se" {
		t.Fatalf("after Backspace buffer = %q, want \\se", c.Text().Get())
	}
	if got := c.CompletionItems(); len(got) != 2 {
		t.Errorf("after Backspace filtered = %v, want 2 items", got)
	}
	if !c.CompletionActive() {
		t.Error("Backspace within a word should keep the popup open")
	}
	// Backspace away the whole word until it empties: an empty word still
	// matches all, so the popup shows the full list rather than closing.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // "\s"
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // "\"
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // ""
	if !c.CompletionActive() || len(c.CompletionItems()) != 4 {
		t.Errorf("empty word: active=%v items=%d, want open with all 4", c.CompletionActive(), len(c.CompletionItems()))
	}
}

// --- accept ----------------------------------------------------------------

func TestCompletionAcceptInsertsSnippetAndPlacesCaret(t *testing.T) {
	for _, key := range []string{"Enter", "Tab"} {
		c, _ := newCompletionEditor()
		typeString(c, "\\se") // -> [\section, \select], \section selected
		c.OnEvent(Event{Kind: EventKeyDown, Code: key})
		if c.CompletionActive() {
			t.Errorf("%s should close the popup after accepting", key)
		}
		// \section's InsertText "\section{$0}" replaces "\se": the caret lands
		// inside the braces (at the $0).
		if c.Text().Get() != "\\section{}" {
			t.Errorf("%s inserted %q, want \\section{}", key, c.Text().Get())
		}
		if c.CursorCol().Get() != 9 {
			t.Errorf("%s caret col = %d, want 9 (inside the braces)", key, c.CursorCol().Get())
		}
	}
}

func TestCompletionAcceptFallsBackToLabel(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\sel") // -> [\select] (no InsertText)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	// \select has no InsertText, so the Label is inserted and the caret lands
	// at its end.
	if c.Text().Get() != "\\select" {
		t.Errorf("insert = %q, want \\select", c.Text().Get())
	}
	if c.CursorCol().Get() != 7 {
		t.Errorf("caret col = %d, want 7 (end of the label)", c.CursorCol().Get())
	}
}

func TestCompletionAcceptUndoable(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\sel")
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.Text().Get() != "\\select" {
		t.Fatalf("precondition insert = %q", c.Text().Get())
	}
	c.Undo()
	if c.Text().Get() != "\\sel" {
		t.Errorf("after Undo = %q, want the pre-accept \\sel", c.Text().Get())
	}
}

func TestAcceptCompletionEmptyGuard(t *testing.T) {
	// Defensive: accept with no filtered items just closes, touching nothing.
	c, _ := newCompletionEditor()
	c.CompletionOpen().Set(true)
	c.compItems = nil
	c.acceptCompletion()
	if c.CompletionActive() {
		t.Error("accept on an empty list should close")
	}
	if c.Text().Get() != "" {
		t.Errorf("buffer = %q, want unchanged empty", c.Text().Get())
	}
}

func TestClampCompSelEmptyList(t *testing.T) {
	// Defensive: clamping an empty list pins the selection to 0.
	c, _ := newCompletionEditor()
	c.CompletionSelection().Set(5)
	c.compItems = nil
	c.clampCompSel()
	if c.CompletionSelected() != 0 {
		t.Errorf("empty-list clamp = %d, want 0", c.CompletionSelected())
	}
}

// --- mouse -----------------------------------------------------------------

func TestCompletionClickAcceptsRow(t *testing.T) {
	c, _ := newCompletionEditor()
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"}) // all 4
	pb := c.CompletionBounds()
	// Click the third row (\select).
	rowH := scaled(CompletionRowH)
	c.OnEvent(Event{Kind: EventClick, X: pb.X + 4, Y: pb.Y + 2*rowH + 2})
	if c.CompletionActive() {
		t.Error("clicking a row should accept + close")
	}
	if c.Text().Get() != "\\select" {
		t.Errorf("clicked insert = %q, want \\select", c.Text().Get())
	}
}

func TestCompletionClickOutsideCloses(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	pb := c.CompletionBounds()
	// A click well outside the popup closes it and reaches the buffer.
	c.OnEvent(Event{Kind: EventClick, X: pb.X, Y: pb.Y + pb.H + 40})
	if c.CompletionActive() {
		t.Error("an off-popup click should close the popup")
	}
}

func TestCompletionWheelScrollsPopup(t *testing.T) {
	// A source with more than CompletionMaxRows candidates so the popup scrolls.
	many := make([]CompletionItem, CompletionMaxRows+5)
	for i := range many {
		many[i] = CompletionItem{Label: string(rune('a'+i)) + "word"}
	}
	src := &staticSource{items: many}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 260})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	if c.clampedCompScroll() != 0 {
		t.Fatalf("initial scroll = %d, want 0", c.clampedCompScroll())
	}
	c.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if got := c.clampedCompScroll(); got != 3 {
		t.Errorf("after wheel = %d, want 3", got)
	}
	// Over-scroll clamps to the last full window.
	c.OnEvent(Event{Kind: EventScroll, Delta: 100})
	if got := c.clampedCompScroll(); got != c.maxCompScroll() {
		t.Errorf("over-scroll = %d, want max %d", got, c.maxCompScroll())
	}
}

func TestCompletionSelectionScrollsIntoView(t *testing.T) {
	many := make([]CompletionItem, CompletionMaxRows+5)
	for i := range many {
		many[i] = CompletionItem{Label: string(rune('a'+i)) + "word"}
	}
	src := &staticSource{items: many}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 260})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	// Drive the selection to the last row: scroll must follow it past the fold.
	for i := 0; i < len(many); i++ {
		c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if c.CompletionSelected() != len(many)-1 {
		t.Fatalf("selection = %d, want last %d", c.CompletionSelected(), len(many)-1)
	}
	if c.clampedCompScroll() != c.maxCompScroll() {
		t.Errorf("scroll = %d, want max %d so the last row is visible", c.clampedCompScroll(), c.maxCompScroll())
	}
	// And back to the top scrolls back up.
	for i := 0; i < len(many); i++ {
		c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if c.clampedCompScroll() != 0 {
		t.Errorf("scroll after returning to top = %d, want 0", c.clampedCompScroll())
	}
}

func TestCompletionIgnoredMouseEventWhileOpen(t *testing.T) {
	// An event the popup does not handle (a bare MouseUp) while open is passed
	// straight through and leaves the popup open.
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	c.OnEvent(Event{Kind: EventMouseUp, X: 1, Y: 1})
	if !c.CompletionActive() {
		t.Error("an unrelated mouse event should not close the popup")
	}
}

// --- bounds geometry -------------------------------------------------------

func TestCompletionBoundsBelowCaret(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\se")
	_, cy := c.CaretPixel(c.CursorLine().Get(), c.compWordStart)
	pb := c.CompletionBounds()
	if pb.Y <= cy {
		t.Errorf("popup top %d should sit below the caret line at %d", pb.Y, cy)
	}
	if pb.W < CompletionMinW {
		t.Errorf("popup width %d below the floor %d", pb.W, CompletionMinW)
	}
	// It stays within the editor frame horizontally.
	r := c.Bounds()
	if pb.X < r.X || pb.X+pb.W > r.X+r.W {
		t.Errorf("popup x-range [%d,%d] escapes the editor [%d,%d]", pb.X, pb.X+pb.W, r.X, r.X+r.W)
	}
}

func TestCompletionBoundsFlipsAboveNearBottom(t *testing.T) {
	c, src := newCompletionEditor()
	// A short editor whose only line sits near the bottom forces an above-flip.
	c.SetBounds(Rect{X: 0, Y: 0, W: 260, H: 40})
	_ = src
	typeString(c, "\\se")
	_, cy := c.CaretPixel(c.CursorLine().Get(), c.compWordStart)
	pb := c.CompletionBounds()
	if pb.Y >= cy {
		t.Errorf("near the bottom the popup should flip above: top %d, caret %d", pb.Y, cy)
	}
}

func TestCompletionBoundsWidthCapAndClamp(t *testing.T) {
	// A narrow editor caps the popup width to the frame; a wide detail proves
	// the widest-row measurement path.
	c, _ := newCompletionEditor()
	c.compItems = []CompletionItem{{Label: "x", Detail: "a-very-long-detail-hint-string"}}
	c.CompletionOpen().Set(true)
	c.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 200})
	pb := c.CompletionBounds()
	if pb.W != 90 {
		t.Errorf("capped width = %d, want the editor width 90", pb.W)
	}
	// A zero-width frame drives the left-edge clamp defensively.
	c.SetBounds(Rect{X: 5, Y: 0, W: 0, H: 200})
	if got := c.CompletionBounds(); got.X != 5 {
		t.Errorf("zero-width clamp X = %d, want the editor left edge 5", got.X)
	}
}

// --- rendering (pixel proof) ----------------------------------------------

func TestCompletionRendersSelectedRow(t *testing.T) {
	c, _ := newCompletionEditor()
	th := DefaultLight()
	typeString(c, "\\se") // 2 items, row 0 selected
	surf := drawInto(c, th, 260, 200)
	pb := c.CompletionBounds()
	// The selected row paints the accent highlight somewhere inside the popup.
	var accent int
	for y := pb.Y; y < pb.Y+scaled(CompletionRowH); y++ {
		for x := pb.X; x < pb.X+pb.W; x++ {
			if pixelAt(surf, 260, x, y) == th.Accent {
				accent++
			}
		}
	}
	if accent == 0 {
		t.Error("selected row's accent highlight was never painted")
	}
}

func TestCompletionRenderClosedPaintsNothingExtra(t *testing.T) {
	// With the popup closed, Draw paints exactly the editor: the accent-heavy
	// popup band must be absent.
	c, _ := newCompletionEditor()
	th := DefaultLight()
	surf := drawInto(c, th, 260, 200)
	// No completion popup => no wide accent band. (The plain editor draws only a
	// thin focus/border accent, far less than a filled row.)
	band := countInk(surf, 260, 200, th.Accent)
	if band > 200 {
		t.Errorf("closed editor painted %d accent px, unexpectedly popup-like", band)
	}
}

func TestCompletionDocPanelRenders(t *testing.T) {
	src := &staticSource{items: []CompletionItem{
		{Label: "alpha", Documentation: "the first letter"},
	}}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	th := DefaultLight()
	typeString(c, "alp")
	if !c.CompletionActive() {
		t.Fatal("precondition: popup open")
	}
	surf := drawInto(c, th, 240, 200)
	pb := c.CompletionBounds()
	// The doc panel sits one row below the list, filled in SurfaceAlt.
	docY := pb.Y + pb.H + scaled(CompletionRowH)/2
	if got := pixelAt(surf, 240, pb.X+scaled(completionPadX)/2, docY); got != th.SurfaceAlt {
		t.Errorf("doc-panel pixel = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
}

func TestCompletionDocPanelAbsentWithoutDoc(t *testing.T) {
	// The selected item has no Documentation: drawCompletionDoc paints nothing,
	// so the region just below the list stays on the plain editor Surface.
	c, _ := newCompletionEditor() // latex items carry no Documentation
	th := DefaultLight()
	typeString(c, "\\se")
	surf := drawInto(c, th, 260, 200)
	pb := c.CompletionBounds()
	docY := pb.Y + pb.H + scaled(CompletionRowH)/2
	if got := pixelAt(surf, 260, pb.X+2, docY); got == th.SurfaceAlt {
		t.Errorf("no-doc item should not paint a SurfaceAlt doc panel (pixel %+v)", got)
	}
}

func TestCompletionDocPanelFlipsAboveNearBottom(t *testing.T) {
	// When the list already fills to the editor bottom, the doc panel flips
	// above it rather than painting off-frame.
	src := &staticSource{items: []CompletionItem{{Label: "alpha", Documentation: "doc"}}}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 44})
	th := DefaultLight()
	typeString(c, "alp")
	surf := drawInto(c, th, 240, 44)
	pb := c.CompletionBounds()
	// The doc row is above the list here; assert a SurfaceAlt band exists above
	// the list top.
	found := false
	for y := 0; y < pb.Y; y++ {
		if pixelAt(surf, 240, pb.X+2, y) == th.SurfaceAlt {
			found = true
			break
		}
	}
	if !found {
		t.Error("doc panel should flip above the list near the editor bottom")
	}
}

func TestCompletionRowDetailPaints(t *testing.T) {
	// \section carries a Detail ("sectioning"); typing to select it must paint
	// the dim detail ink somewhere in its row.
	c, _ := newCompletionEditor()
	th := DefaultLight()
	typeString(c, "\\sec") // -> [\section], selected
	surf := drawInto(c, th, 260, 200)
	pb := c.CompletionBounds()
	dim := blendRGBA(accentInk(th), th.Surface, 0.45) // ink is accentInk on the selected row
	var n int
	for y := pb.Y; y < pb.Y+scaled(CompletionRowH); y++ {
		for x := pb.X; x < pb.X+pb.W; x++ {
			if pixelAt(surf, 260, x, y) == dim {
				n++
			}
		}
	}
	if n == 0 {
		t.Error("selected row's dim Detail hint was never painted")
	}
}

func TestCompletionWheelScrollUpClampsAtZero(t *testing.T) {
	many := make([]CompletionItem, CompletionMaxRows+5)
	for i := range many {
		many[i] = CompletionItem{Label: string(rune('a'+i)) + "word"}
	}
	src := &staticSource{items: many}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 260})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	// A negative wheel drives compScroll below zero before the clamp pulls it
	// back — exercising the lower bound.
	c.OnEvent(Event{Kind: EventScroll, Delta: -50})
	if got := c.clampedCompScroll(); got != 0 {
		t.Errorf("scroll-up past the top = %d, want clamp to 0", got)
	}
}

func TestCompletionAcceptPublishesTextEdit(t *testing.T) {
	c, _ := newCompletionEditor()
	typeString(c, "\\sel")
	// Accept must publish the edit onto the Text() Observable (sync's job, the
	// OnChange successor): subscribe and require a notification.
	var published string
	var fired bool
	unsub := c.Text().Subscribe(func(s string) { published, fired = s, true })
	defer unsub()
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !fired || published != "\\select" {
		t.Errorf("accept published (%q, fired=%v), want \\select on the Text() Observable", published, fired)
	}
}

func TestCompletionBoundsCapsRowCount(t *testing.T) {
	// More candidates than CompletionMaxRows: the popup height is capped to the
	// window, not the full list.
	many := make([]CompletionItem, CompletionMaxRows+5)
	for i := range many {
		many[i] = CompletionItem{Label: string(rune('a'+i)) + "word"}
	}
	src := &staticSource{items: many}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	pb := c.CompletionBounds()
	if pb.H != CompletionMaxRows*scaled(CompletionRowH) {
		t.Errorf("popup height = %d, want the capped %d rows", pb.H, CompletionMaxRows)
	}
}

func TestCompletionDocPanelToleratesOutOfRangeSelection(t *testing.T) {
	// CompletionSelection is a host-settable Observable; an out-of-range value
	// must not panic the doc panel — it simply paints nothing.
	src := &staticSource{items: []CompletionItem{{Label: "alpha", Documentation: "d"}}}
	c := NewCodeEditor("")
	c.CompletionSource = src.fn
	c.SetBounds(Rect{X: 0, Y: 0, W: 240, H: 200})
	typeString(c, "alp")
	c.CompletionSelection().Set(99) // out of range for the doc read
	drawInto(c, DefaultLight(), 240, 200)
	// No panic reaching here is the assertion; the popup stays open.
	if !c.CompletionActive() {
		t.Error("drawing with an out-of-range selection should not close the popup")
	}
}

// --- integration: no completion => byte-identical delegation ---------------

func TestOnEventDelegatesWithoutCompletion(t *testing.T) {
	// An editor with no source: OnEvent forwards every kind to TextView. Type,
	// arrow, and the buffer + caret behave exactly like a bare TextView.
	c := NewCodeEditor("hi")
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if c.CursorCol().Get() != 2 {
		t.Fatalf("End caret = %d, want 2", c.CursorCol().Get())
	}
	c.OnEvent(Event{Kind: EventChar, Code: "!"})
	if c.Text().Get() != "hi!" {
		t.Errorf("typed buffer = %q, want hi!", c.Text().Get())
	}
}
