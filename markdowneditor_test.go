// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- construction ----------------------------------------------------------

func TestNewMarkdownEditorSeedsBothPanes(t *testing.T) {
	me := NewMarkdownEditor("# Hi\n\nBody text.")
	if me.Source == nil || me.Preview == nil {
		t.Fatal("NewMarkdownEditor must build both panes")
	}
	if got := me.Source.Text(); got != "# Hi\n\nBody text." {
		t.Errorf("Source.Text() = %q", got)
	}
	if me.Preview.Source != "# Hi\n\nBody text." {
		t.Errorf("Preview.Source = %q", me.Preview.Source)
	}
	if me.Split != 0.5 {
		t.Errorf("Split = %v, want 0.5", me.Split)
	}
	if !me.SideBySide {
		t.Error("SideBySide should default true")
	}
}

func TestNewMarkdownEditorEmptyInitial(t *testing.T) {
	// Empty initial text must still produce a usable single-line buffer on
	// both panes (mirrors NewTextView / NewMarkdownView's own empty-input
	// handling) and must not panic when drawn.
	me := NewMarkdownEditor("")
	if got := me.Source.Text(); got != "" {
		t.Errorf("Source.Text() = %q, want empty", got)
	}
	if len(me.Source.Lines) != 1 || me.Source.Lines[0] != "" {
		t.Errorf("Source.Lines = %v, want one empty line", me.Source.Lines)
	}
	if me.Preview.Source != "" {
		t.Errorf("Preview.Source = %q, want empty", me.Preview.Source)
	}
	me.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	surf := makeSurface(40, 20)
	me.Draw(newP(surf, 40), DefaultLight()) // must not panic
}

// --- Text / SetText round-trip ----------------------------------------------

func TestMarkdownEditorTextRoundTrip(t *testing.T) {
	me := NewMarkdownEditor("one")
	if me.Text() != "one" {
		t.Fatalf("Text() = %q, want %q", me.Text(), "one")
	}
	me.SetText("two\nthree")
	if got := me.Text(); got != "two\nthree" {
		t.Errorf("Text() after SetText = %q", got)
	}
	if me.Preview.Source != "two\nthree" {
		t.Errorf("Preview.Source after SetText = %q", me.Preview.Source)
	}
}

func TestMarkdownEditorTextNilSource(t *testing.T) {
	me := &MarkdownEditor{}
	if got := me.Text(); got != "" {
		t.Errorf("Text() with nil Source = %q, want empty", got)
	}
	// SetText with a nil Source must not panic, and syncPreview's nil guard
	// (Source == nil) must be exercised without a Preview either.
	me.SetText("ignored")
}

func TestMarkdownEditorSetTextNilPreview(t *testing.T) {
	// Source present, Preview nil: syncPreview's OR-guard short-circuits on
	// the Preview side this time.
	me := &MarkdownEditor{Source: NewTextView("x")}
	me.SetText("y")
	if me.Source.Text() != "y" {
		t.Errorf("Source.Text() = %q, want %q", me.Source.Text(), "y")
	}
}

// --- split() fallback --------------------------------------------------------

func TestMarkdownEditorSplitFallback(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"zero value defaults", 0, 0.5},
		{"negative defaults", -0.2, 0.5},
		{"one or above defaults", 1, 0.5},
		{"in range kept", 0.25, 0.25},
	}
	for _, c := range cases {
		me := &MarkdownEditor{Split: c.in}
		if got := me.split(); got != c.want {
			t.Errorf("%s: split() = %v, want %v", c.name, got, c.want)
		}
	}
}

// --- layout: sourceRect / previewRect + Draw sub-rects ----------------------

func TestMarkdownEditorLayoutSideBySide(t *testing.T) {
	me := &MarkdownEditor{
		Source:     NewTextView(""),
		Preview:    NewMarkdownView(""),
		Split:      0.5,
		SideBySide: true,
	}
	me.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	wantSrc := Rect{X: 0, Y: 0, W: 50, H: 60}
	wantPrev := Rect{X: 51, Y: 0, W: 49, H: 60}
	if got := me.sourceRect(); got != wantSrc {
		t.Errorf("sourceRect() = %+v, want %+v", got, wantSrc)
	}
	if got := me.previewRect(); got != wantPrev {
		t.Errorf("previewRect() = %+v, want %+v", got, wantPrev)
	}

	surf := makeSurface(100, 60)
	me.Draw(newP(surf, 100), DefaultLight())
	if me.Source.Bounds() != wantSrc {
		t.Errorf("Source.Bounds() = %+v, want %+v", me.Source.Bounds(), wantSrc)
	}
	if me.Preview.Bounds() != wantPrev {
		t.Errorf("Preview.Bounds() = %+v, want %+v", me.Preview.Bounds(), wantPrev)
	}
	// The divider column between the panes carries Border ink.
	th := DefaultLight()
	if got := countInk(surf, 100, 60, th.Border); got == 0 {
		t.Error("no divider pixels drawn")
	}
}

func TestMarkdownEditorLayoutStackedOffsetOrigin(t *testing.T) {
	// Rect math must respect a non-zero origin, matching how every other
	// container in the toolkit (Paned, Notebook, ...) computes child rects
	// relative to its own Bounds().
	me := &MarkdownEditor{Split: 0.25, SideBySide: false}
	me.SetBounds(Rect{X: 5, Y: 5, W: 80, H: 40})

	wantSrc := Rect{X: 5, Y: 5, W: 80, H: 10}
	wantPrev := Rect{X: 5, Y: 16, W: 80, H: 29}
	if got := me.sourceRect(); got != wantSrc {
		t.Errorf("sourceRect() = %+v, want %+v", got, wantSrc)
	}
	if got := me.previewRect(); got != wantPrev {
		t.Errorf("previewRect() = %+v, want %+v", got, wantPrev)
	}
}

func TestMarkdownEditorLayoutStackedDraw(t *testing.T) {
	me := &MarkdownEditor{
		Source:     NewTextView(""),
		Preview:    NewMarkdownView(""),
		Split:      0.25,
		SideBySide: false,
	}
	me.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})

	wantSrc := Rect{X: 0, Y: 0, W: 80, H: 10}
	wantPrev := Rect{X: 0, Y: 11, W: 80, H: 29}
	surf := makeSurface(80, 40)
	me.Draw(newP(surf, 80), DefaultLight())
	if me.Source.Bounds() != wantSrc {
		t.Errorf("Source.Bounds() = %+v, want %+v", me.Source.Bounds(), wantSrc)
	}
	if me.Preview.Bounds() != wantPrev {
		t.Errorf("Preview.Bounds() = %+v, want %+v", me.Preview.Bounds(), wantPrev)
	}
	th := DefaultLight()
	if got := countInk(surf, 80, 40, th.Border); got == 0 {
		t.Error("no divider pixels drawn (stacked)")
	}
}

// --- Draw nil-child guards ---------------------------------------------------

func TestMarkdownEditorDrawNilSource(t *testing.T) {
	me := &MarkdownEditor{Preview: NewMarkdownView("hi"), Split: 0.5, SideBySide: true}
	me.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	surf := makeSurface(40, 20)
	me.Draw(newP(surf, 40), DefaultLight()) // must not panic
}

func TestMarkdownEditorDrawNilPreview(t *testing.T) {
	me := &MarkdownEditor{Source: NewTextView("hi"), Split: 0.5, SideBySide: false}
	me.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	surf := makeSurface(40, 20)
	me.Draw(newP(surf, 40), DefaultLight()) // must not panic
}

// --- event routing + live preview sync --------------------------------------

func TestMarkdownEditorTypingUpdatesPreview(t *testing.T) {
	me := NewMarkdownEditor("")
	me.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	// Non-click events (keyboard/char) always route to Source regardless of
	// X/Y -- there is no other interactive pane to compete for them.
	me.OnEvent(Event{Kind: EventChar, Code: "x", X: 999, Y: 999})
	if me.Source.Text() != "x" {
		t.Fatalf("Source.Text() = %q, want %q", me.Source.Text(), "x")
	}
	if me.Preview.Source != "x" {
		t.Errorf("Preview.Source = %q, want synced to %q", me.Preview.Source, "x")
	}

	me.OnEvent(Event{Kind: EventChar, Code: "y"})
	if me.Source.Text() != "xy" {
		t.Fatalf("Source.Text() = %q, want %q", me.Source.Text(), "xy")
	}
	if me.Preview.Source != "xy" {
		t.Errorf("Preview.Source = %q, want synced to %q", me.Preview.Source, "xy")
	}
}

func TestMarkdownEditorClickInSourceFocuses(t *testing.T) {
	me := NewMarkdownEditor("hello")
	me.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60}) // source pane = x in [0,50)
	me.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if !me.Source.Focused {
		t.Error("click inside the source pane should focus it")
	}
}

func TestMarkdownEditorClickInPreviewIsNoOp(t *testing.T) {
	me := NewMarkdownEditor("hello")
	me.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60}) // preview pane = x in [51,100)
	me.OnEvent(Event{Kind: EventClick, X: 70, Y: 30})
	if me.Source.Focused {
		t.Error("click inside the preview pane must not focus Source")
	}
	if me.Source.Text() != "hello" || me.Preview.Source != "hello" {
		t.Error("click inside the preview pane must not mutate either pane")
	}
}

func TestMarkdownEditorClickOnDividerIsNoOp(t *testing.T) {
	me := NewMarkdownEditor("hello")
	me.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60}) // divider at x == 50
	me.OnEvent(Event{Kind: EventClick, X: 50, Y: 30})
	if me.Source.Focused {
		t.Error("click on the divider must not focus Source")
	}
}

func TestMarkdownEditorOnEventNilSource(t *testing.T) {
	me := &MarkdownEditor{Preview: NewMarkdownView("hi")}
	me.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	me.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // must not panic; returns early
	if me.Preview.Source != "hi" {
		t.Error("no-Source OnEvent must not touch Preview")
	}
}

// --- offset origin: OnEvent coordinates are widget-local --------------------

func TestMarkdownEditorOnEventNonZeroOrigin(t *testing.T) {
	me := NewMarkdownEditor("z")
	me.SetBounds(Rect{X: 20, Y: 20, W: 100, H: 60}) // source pane local x in [0,50)
	// Widget-local click (per the Widget.OnEvent contract) at (10,10) must
	// land in the source pane even though Bounds() is offset.
	me.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if !me.Source.Focused {
		t.Error("widget-local click inside source pane should focus it despite non-zero origin")
	}
}
