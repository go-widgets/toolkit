// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// sixPxFont is a deterministic monospace test Font: every rune is 6px wide, so
// caret math and highlight rects are exactly predictable.
type sixPxFont struct{}

func (sixPxFont) Height() int                                        { return 10 }
func (sixPxFont) Advance() int                                       { return 6 }
func (sixPxFont) Measure(s string) int                               { return len([]rune(s)) * 6 }
func (sixPxFont) Draw(_ painter.Painter, _, _ int, _ string, _ RGBA) {}

// ttOrFixed returns the 6px monospace font for deterministic measuring.
func ttOrFixed() Font { return sixPxFont{} }

// makeSel builds a TextSelection over two lines: "Hello world" at y=0 and
// "second line" at y=10, each a single run, 6px/char.
func makeSel() *TextSelection {
	f := ttOrFixed()
	s := &TextSelection{}
	s.SetRuns([]TextRun{
		{Text: "Hello world", Bounds: Rect{X: 0, Y: 0, W: 11 * 6, H: 10}, Font: f},
		{Text: "second line", Bounds: Rect{X: 0, Y: 10, W: 11 * 6, H: 10}, Font: f},
	})
	return s
}

func TestTextSelectionEmptyAndClear(t *testing.T) {
	s := &TextSelection{}
	// No runs: Begin is a no-op, selection stays empty.
	s.Begin(5, 5)
	if !s.IsEmpty() || s.SelectedText() != "" {
		t.Fatal("selection over no runs must be empty")
	}
	s = makeSel()
	// A press without a drag is an empty (collapsed) selection.
	s.Begin(0, 0)
	if !s.IsEmpty() || s.SelectedText() != "" {
		t.Fatalf("collapsed selection not empty: %q", s.SelectedText())
	}
	s.Drag(6, 0) // extend one char
	if s.IsEmpty() {
		t.Fatal("drag should make the selection non-empty")
	}
	s.End()
	s.Clear()
	if !s.IsEmpty() {
		t.Fatal("Clear should empty the selection")
	}
	// Drag without an active drag is ignored.
	s.Drag(30, 0)
	if !s.IsEmpty() {
		t.Fatal("Drag after Clear must not select")
	}
}

func TestTextSelectionWithinRun(t *testing.T) {
	s := makeSel()
	// Select "Hello" (chars 0..5) on the first line: 6px/char → x 0..30.
	s.Begin(0, 3)
	s.Drag(30, 3)
	s.End()
	if got := s.SelectedText(); got != "Hello" {
		t.Fatalf("within-run selection = %q, want %q", got, "Hello")
	}
	// Reverse drag (cursor before anchor) yields the same canonical text.
	s.Begin(30, 3)
	s.Drag(0, 3)
	if got := s.SelectedText(); got != "Hello" {
		t.Fatalf("reverse selection = %q, want %q", got, "Hello")
	}
}

func TestTextSelectionAcrossLines(t *testing.T) {
	s := makeSel()
	// From mid-first-line (char 6, "world") across to mid-second-line (char 6).
	s.Begin(6*6, 3) // start at char 6 of line 1 ("world")
	s.Drag(6*6, 13) // to char 6 of line 2 ("second")
	s.End()
	if got := s.SelectedText(); got != "world\nsecond" {
		t.Fatalf("cross-line selection = %q, want %q", got, "world\nsecond")
	}
}

func TestTextSelectionSameLineSpaceJoin(t *testing.T) {
	f := ttOrFixed()
	s := &TextSelection{}
	// Two runs on the SAME line (equal Y), no trailing/leading space: joined by
	// a single inserted space.
	s.SetRuns([]TextRun{
		{Text: "foo", Bounds: Rect{X: 0, Y: 0, W: 18, H: 10}, Font: f},
		{Text: "bar", Bounds: Rect{X: 30, Y: 0, W: 18, H: 10}, Font: f},
	})
	s.Begin(0, 3)
	s.Drag(48, 3)
	s.End()
	if got := s.SelectedText(); got != "foo bar" {
		t.Fatalf("same-line join = %q, want %q", got, "foo bar")
	}
	// When the seam already has whitespace, no extra space is inserted.
	s.SetRuns([]TextRun{
		{Text: "foo ", Bounds: Rect{X: 0, Y: 0, W: 24, H: 10}, Font: f},
		{Text: "bar", Bounds: Rect{X: 30, Y: 0, W: 18, H: 10}, Font: f},
	})
	s.Begin(0, 3)
	s.Drag(48, 3)
	if got := s.SelectedText(); got != "foo bar" {
		t.Fatalf("seam-space join = %q, want %q", got, "foo bar")
	}
	// When the SECOND run starts with a space, likewise no extra space.
	s.SetRuns([]TextRun{
		{Text: "foo", Bounds: Rect{X: 0, Y: 0, W: 18, H: 10}, Font: f},
		{Text: " bar", Bounds: Rect{X: 30, Y: 0, W: 24, H: 10}, Font: f},
	})
	s.Begin(0, 3)
	s.Drag(54, 3)
	if got := s.SelectedText(); got != "foo bar" {
		t.Fatalf("leading-space join = %q, want %q", got, "foo bar")
	}
}

func TestTextSelectionCaretClampAndNearestLine(t *testing.T) {
	s := makeSel()
	// A click far right + below the last line clamps to the very end.
	s.Begin(0, 0)
	s.Drag(1000, 1000)
	s.End()
	if got := s.SelectedText(); got != "Hello world\nsecond line" {
		t.Fatalf("select-all-ish = %q", got)
	}
	// A click far left + above clamps to the very start (empty when anchored there).
	s.Begin(-50, -50)
	if s.anchor.run != 0 || s.anchor.char != 0 {
		t.Fatalf("above/left anchor = %+v, want {0,0}", s.anchor)
	}
}

func TestTextSelectionSetRunsFiltersAndClamps(t *testing.T) {
	f := ttOrFixed()
	s := &TextSelection{}
	s.SetRuns([]TextRun{
		{Text: "keep", Bounds: Rect{X: 0, Y: 0, W: 24, H: 10}, Font: f},
		{Text: "", Bounds: Rect{X: 0, Y: 10, W: 10, H: 10}, Font: f},    // empty → dropped
		{Text: "zero", Bounds: Rect{X: 0, Y: 20, W: 0, H: 10}, Font: f}, // zero-area → dropped
	})
	if len(s.runs) != 1 {
		t.Fatalf("filtered runs = %d, want 1", len(s.runs))
	}
	// Select everything, then shrink the run set: endpoints clamp, no panic.
	s.Begin(0, 3)
	s.Drag(24, 3)
	s.End()
	s.SetRuns([]TextRun{{Text: "hi", Bounds: Rect{X: 0, Y: 0, W: 12, H: 10}, Font: f}})
	_ = s.SelectedText() // must not panic with the stale (now out-of-range) cursor
	if s.cursor.run >= len(s.runs) || s.cursor.char > 2 {
		t.Fatalf("cursor not clamped: %+v", s.cursor)
	}
}

func TestTextSelectionClampEdges(t *testing.T) {
	s := makeSel() // two lines
	// Select from line 1 into line 2 so the cursor sits on run index 1.
	s.Begin(0, 3)
	s.Drag(6*6, 13)
	s.End()
	if s.cursor.run != 1 {
		t.Fatalf("cursor.run = %d, want 1", s.cursor.run)
	}
	// Shrink to a single run: the run>=len clamp pulls the cursor back to run 0.
	f := ttOrFixed()
	s.SetRuns([]TextRun{{Text: "one", Bounds: Rect{X: 0, Y: 0, W: 18, H: 10}, Font: f}})
	if s.cursor.run != 0 || s.cursor.char > 3 {
		t.Fatalf("cursor not clamped into range: %+v", s.cursor)
	}
	// Drop all runs while active: both endpoints collapse to the zero position.
	s.SetRuns(nil)
	if s.anchor != (textPos{}) || s.cursor != (textPos{}) {
		t.Fatalf("endpoints not reset on empty run set: a=%+v c=%+v", s.anchor, s.cursor)
	}
}

func TestCaretLeftOfLineRuns(t *testing.T) {
	f := ttOrFixed()
	s := &TextSelection{}
	// Two runs on one line; a click to the LEFT of both exercises the left-edge
	// horizontal-distance branch and lands at the start of the first run.
	s.SetRuns([]TextRun{
		{Text: "left", Bounds: Rect{X: 20, Y: 0, W: 24, H: 10}, Font: f},
		{Text: "right", Bounds: Rect{X: 60, Y: 0, W: 30, H: 10}, Font: f},
	})
	s.Begin(-10, 3)
	if s.anchor.run != 0 || s.anchor.char != 0 {
		t.Fatalf("left-of-line click = %+v, want {0,0}", s.anchor)
	}
}

func TestCharAtNilFontAndEdges(t *testing.T) {
	// nil font: left of the run → 0, right of it → full length.
	run := TextRun{Text: "abc", Bounds: Rect{X: 10, Y: 0, W: 30, H: 10}}
	if got := charAt(run, 5); got != 0 {
		t.Fatalf("nil-font left = %d, want 0", got)
	}
	if got := charAt(run, 100); got != 3 {
		t.Fatalf("nil-font right = %d, want 3", got)
	}
	// With a font, a click exactly at the left edge is offset 0.
	run.Font = ttOrFixed()
	if got := charAt(run, 10); got != 0 {
		t.Fatalf("at left edge = %d, want 0", got)
	}
}

func TestTextSelectionDrawHighlight(t *testing.T) {
	col := RGBA{R: 0x33, G: 0x99, B: 0xFF, A: 0xFF}
	// Empty selection paints nothing.
	empty := &TextSelection{}
	buf0 := makeSurface(60, 20)
	empty.Draw(newP(buf0, 60), col)
	if pixelAt(buf0, 60, 5, 5) == col {
		t.Fatal("empty selection should paint nothing")
	}

	// Within-run: select "Hello" (x 0..30) on line 1 → highlight there, not past it.
	s := makeSel()
	s.Begin(0, 3)
	s.Drag(30, 3)
	s.End()
	buf := makeSurface(80, 20)
	s.Draw(newP(buf, 80), col)
	if pixelAt(buf, 80, 5, 3) != col {
		t.Fatal("expected highlight inside the selected span")
	}
	if pixelAt(buf, 80, 40, 3) == col {
		t.Fatal("highlight should stop at the selection end")
	}

	// A three-line selection whose MIDDLE run has a nil font exercises the
	// full-width fallback (zero-measured span on an intermediate line).
	f := ttOrFixed()
	s = &TextSelection{}
	s.SetRuns([]TextRun{
		{Text: "top", Bounds: Rect{X: 0, Y: 0, W: 18, H: 10}, Font: f},
		{Text: "midnofont", Bounds: Rect{X: 0, Y: 10, W: 54, H: 10}}, // nil font
		{Text: "bot", Bounds: Rect{X: 0, Y: 20, W: 18, H: 10}, Font: f},
	})
	s.Begin(0, 3)  // start of "top"
	s.Drag(18, 23) // end of "bot"
	s.End()
	buf2 := makeSurface(60, 40)
	s.Draw(newP(buf2, 60), col)
	if pixelAt(buf2, 60, 40, 13) != col { // deep into the middle line's full width
		t.Fatal("middle nil-font line should be fully highlighted")
	}

	// Starting the selection at the very END of the first run makes that run a
	// zero-width span on the start line → it is skipped (continue), the next line
	// is highlighted.
	s = makeSel()
	s.Begin(11*6, 3) // end of "Hello world"
	s.Drag(6*6, 13)  // char 6 of line 2
	s.End()
	buf3 := makeSurface(80, 20)
	s.Draw(newP(buf3, 80), col)
	if pixelAt(buf3, 80, 3, 3) == col {
		t.Fatal("first line should not be highlighted (zero-width start span)")
	}
	if pixelAt(buf3, 80, 3, 13) != col {
		t.Fatal("second line should be highlighted")
	}
}

func TestCollectRuns(t *testing.T) {
	// A container of two labels (one empty → contributes nothing) plus a
	// non-selectable widget, and a nested container with another label.
	inner := NewContainer(&BoxLayout{Vertical: true})
	inner.AddWidget(NewLabel("nested"))
	root := NewContainer(&BoxLayout{Vertical: true})
	root.AddWidget(NewLabel("first"))
	root.AddWidget(NewLabel(""))            // empty label → no run
	root.AddWidget(NewButton("Click", nil)) // not SelectableText
	root.AddWidget(inner)
	root.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})

	runs := CollectRuns(root)
	if len(runs) != 2 {
		t.Fatalf("collected %d runs, want 2 (first + nested)", len(runs))
	}
	texts := runs[0].Text + "|" + runs[1].Text
	if texts != "first|nested" && texts != "nested|first" {
		t.Fatalf("collected texts = %q", texts)
	}

	// A bare Label (no container) is collected directly.
	if r := CollectRuns(NewLabel("solo")); len(r) != 1 || r[0].Text != "solo" {
		t.Fatalf("bare label runs = %+v", r)
	}
	// An empty label yields no runs.
	if r := CollectRuns(NewLabel("")); r != nil {
		t.Fatalf("empty label should yield no runs, got %+v", r)
	}

	// The box containers (VBox/HBox) are walked too, via their Children()
	// accessor — the reader's reading views are built from these.
	vb := NewVBox()
	vb.AddFixed(NewLabel("v1"), 10)
	hb := NewHBox()
	hb.AddFixed(NewLabel("h1"), 10)
	vb.AddFixed(hb, 10)
	vb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
	if r := CollectRuns(vb); len(r) != 2 {
		t.Fatalf("VBox/HBox walk collected %d runs, want 2", len(r))
	}
	if len(vb.Children()) != 2 || len(hb.Children()) != 1 {
		t.Fatalf("Children() accessors wrong: vb=%d hb=%d", len(vb.Children()), len(hb.Children()))
	}
}

func TestTextSelectionCopy(t *testing.T) {
	SetClipboard(nil) // fresh in-process clipboard
	s := makeSel()
	// Empty selection: CopySelection leaves the clipboard untouched.
	SetClipboardText("keep")
	if got := s.CopySelection(); got != "" {
		t.Fatalf("empty copy = %q", got)
	}
	if ClipboardText() != "keep" {
		t.Fatalf("empty copy clobbered clipboard: %q", ClipboardText())
	}
	// Real selection: CopySelection writes + returns it.
	s.Begin(0, 3)
	s.Drag(30, 3)
	s.End()
	if got := s.CopySelection(); got != "Hello" || ClipboardText() != "Hello" {
		t.Fatalf("copy = %q, clipboard = %q", got, ClipboardText())
	}
}
