// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

// heightFor is the expected height of n lines without a horizontal bar.
func heightFor(c *CodeEditor, n int) int {
	return 4 + n*(c.glyphHeight()+4) + textBottomPad
}

func newFlowEditor(src string, w int) *CodeEditor {
	c := NewCodeEditor(src)
	c.NoVerticalScroll = true
	c.SetBounds(Rect{W: w, H: 10})
	return c
}

func TestPreferredHeightEmptyAndOneLine(t *testing.T) {
	c := newFlowEditor("", 600)
	if got, want := c.PreferredHeight().Get(), heightFor(c, 1); got != want {
		t.Fatalf("empty buffer: %d, want one line %d", got, want)
	}
	c.SetText("hello")
	if got, want := c.PreferredHeight().Get(), heightFor(c, 1); got != want {
		t.Fatalf("one line: %d, want %d", got, want)
	}
	var tv TextView // zero value, lines nil: still one line
	if got, want := tv.PreferredHeight().Get(), 4+(tv.glyphHeight()+4)+textBottomPad; got != want {
		t.Fatalf("zero TextView: %d, want %d", got, want)
	}
}

func TestPreferredHeightNLines(t *testing.T) {
	c := newFlowEditor(strings.Repeat("ab\n", 11)+"ab", 600)
	if got, want := c.PreferredHeight().Get(), heightFor(c, 12); got != want {
		t.Fatalf("12 lines: %d, want %d", got, want)
	}
}

func TestPreferredHeightFollowsEdits(t *testing.T) {
	c := newFlowEditor("a", 600)
	var seen []int
	c.PreferredHeight().Subscribe(func(h int) { seen = append(seen, h) })
	h1 := c.PreferredHeight().Get()

	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // add a line through the keyboard
	if got := c.PreferredHeight().Get(); got != heightFor(c, 2) || got <= h1 {
		t.Fatalf("after Enter: %d, want %d", got, heightFor(c, 2))
	}
	c.Text().Set("a\nb\nc\nd") // host-driven Set (adopt)
	if got := c.PreferredHeight().Get(); got != heightFor(c, 4) {
		t.Fatalf("after host Set: %d, want %d", got, heightFor(c, 4))
	}
	c.Undo() // restores the snapshot taken before Enter: one line
	if got := c.PreferredHeight().Get(); got != heightFor(c, 1) {
		t.Fatalf("after Undo: %d, want %d", got, heightFor(c, 1))
	}
	c.Redo() // restores the 4-line state Undo started from
	if got := c.PreferredHeight().Get(); got != heightFor(c, 4) {
		t.Fatalf("after Redo: %d, want %d", got, heightFor(c, 4))
	}
	c.SetText("x")
	if got := c.PreferredHeight().Get(); got != h1 {
		t.Fatalf("after shrinking to one line: %d, want %d", got, h1)
	}
	if len(seen) == 0 {
		t.Fatal("subscriber never notified")
	}
	// A change that leaves the height alone does not notify.
	n := len(seen)
	c.OnEvent(Event{Kind: EventChar, Code: "y"})
	if len(seen) != n {
		t.Fatalf("typing on one line notified %d extra time(s)", len(seen)-n)
	}
}

type heightTallFont struct{ Font }

func (f heightTallFont) Height() int { return f.Font.Height() + 10 }

func TestPreferredHeightFollowsFont(t *testing.T) {
	c := newFlowEditor("a\nb", 600)
	h := c.PreferredHeight().Get()
	got := -1
	c.PreferredHeight().Subscribe(func(v int) { got = v })
	if c.SetFont(heightTallFont{CurrentFont()}) != &c.TextView.Base {
		t.Fatal("SetFont must return the Base for chaining")
	}
	if want := heightFor(c, 2); got != want || want != h+20 {
		t.Fatalf("after SetFont: notified %d, want %d (was %d)", got, want, h)
	}
	// A font swapped through the field is caught by the next Draw.
	c.Font = nil
	c.Draw(newP(makeSurface(600, 200), 600), DefaultDark())
	if c.PreferredHeight().Get() != h {
		t.Fatalf("Draw did not republish: %d, want %d", c.PreferredHeight().Get(), h)
	}
}

func TestNoVerticalScrollNeverScrolls(t *testing.T) {
	c := newFlowEditor(strings.Repeat("l\n", 49)+"l", 400)
	c.SetBounds(Rect{W: 400, H: c.PreferredHeight().Get()})
	if v, _ := c.bars(); v {
		t.Fatal("vertical bar shown in no-vertical-scroll mode")
	}
	if got := c.visibleLines(); got != 50 {
		t.Fatalf("visibleLines = %d, want 50", got)
	}
	c.OnEvent(Event{Kind: EventScroll, Delta: 5})
	c.ScrollLine().Set(7)
	if c.clampedScrollLine() != 0 {
		t.Fatalf("clampedScrollLine = %d, want 0", c.clampedScrollLine())
	}
	c.ShowMinimap = true
	if c.minimapW() != 0 {
		t.Fatal("minimap must be off in no-vertical-scroll mode")
	}
	c.Draw(newP(makeSurface(400, c.Bounds().H), 400), DefaultDark()) // must not panic
}

func TestNoVerticalScrollCaretSelectionIME(t *testing.T) {
	c := newFlowEditor(strings.Repeat("abc\n", 29)+"abc", 400)
	c.SetBounds(Rect{W: 400, H: c.PreferredHeight().Get()})
	c.SetFocused(true)
	// Caret on the last line, via a click on it and via the keyboard.
	x, y := c.CaretPixel(29, 2)
	c.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if c.CursorLine().Get() != 29 || c.CursorCol().Get() != 2 {
		t.Fatalf("click -> (%d,%d), want (29,2)", c.CursorLine().Get(), c.CursorCol().Get())
	}
	if c.ScrollLine().Get() != 0 {
		t.Fatalf("click scrolled vertically: %d", c.ScrollLine().Get())
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if c.CursorLine().Get() != 28 || c.ScrollLine().Get() != 0 {
		t.Fatalf("ArrowUp: line %d scroll %d", c.CursorLine().Get(), c.ScrollLine().Get())
	}
	// Selection across the whole buffer.
	c.SelectAll()
	if !c.HasSelection() || !strings.HasPrefix(c.SelectionText(), "abc\nabc") {
		t.Fatalf("SelectAll lost: %q", c.SelectionText())
	}
	// IME: the candidate rect lands inside the (unscrolled) widget.
	c.SetSelection(Selection{})
	c.SetInputSelection(len(c.InputText()), len(c.InputText()))
	c.SetMarkedInput("かん", 2, 2, -1, -1)
	ix, iy, _, ih := c.InputRect(len(c.InputText())-2, len(c.InputText()))
	b := c.Bounds()
	if ix < b.X || iy < b.Y || iy+ih > b.Y+b.H {
		t.Fatalf("IME rect (%d,%d,h=%d) outside bounds %+v", ix, iy, ih, b)
	}
	c.UnmarkInput()
	if c.ScrollLine().Get() != 0 {
		t.Fatalf("IME scrolled vertically: %d", c.ScrollLine().Get())
	}
}

func TestNoVerticalScrollKeepsHorizontalScroll(t *testing.T) {
	c := newFlowEditor("short\n"+strings.Repeat("x", 400), 300)
	c.SetBounds(Rect{W: 300, H: c.PreferredHeight().Get()})
	v, h := c.bars()
	if v || !h {
		t.Fatalf("bars v=%v h=%v, want h only", v, h)
	}
	if c.maxScrollX() <= 0 {
		t.Fatal("a long line must scroll horizontally")
	}
	c.scrollXBy(40)
	if c.clampedScrollX() != 40 {
		t.Fatalf("ScrollX = %d, want 40", c.clampedScrollX())
	}
	c.OnEvent(Event{Kind: EventScroll, DeltaX: 1})
	if c.clampedScrollX() <= 40 {
		t.Fatal("wheel DeltaX no longer scrolls horizontally")
	}
	// The track is part of the preferred height.
	if got, want := c.PreferredHeight().Get(), heightFor(c, 2)+1+scrollbarTrack(); got != want {
		t.Fatalf("height with bar = %d, want %d", got, want)
	}
}

// TestPreferredHeight360 is the 360 px fit: at a phone width the gutter plus the
// text area stay inside 360, the bar appears only for a line that overflows, and
// the height tracks that.
func TestPreferredHeight360(t *testing.T) {
	c := newFlowEditor("fmt.Println(1)\nx := 2", 360)
	bare := c.PreferredHeight().Get()
	if bare != heightFor(c, 2) {
		t.Fatalf("short lines at 360: %d, want %d", bare, heightFor(c, 2))
	}
	c.SetText("fmt.Println(1)\n" + strings.Repeat("y", 200))
	withBar := c.PreferredHeight().Get()
	if withBar != heightFor(c, 2)+1+scrollbarTrack() {
		t.Fatalf("long line at 360: %d, want bar added", withBar)
	}
	if got := c.textLeftInset() + c.textAreaWidth(); got > 360 {
		t.Fatalf("gutter + text area = %d px, exceeds 360", got)
	}
	// Widening past the content removes the bar and republishes.
	c.SetBounds(Rect{W: 4000, H: withBar})
	if c.PreferredHeight().Get() != bare {
		t.Fatalf("after widening: %d, want %d", c.PreferredHeight().Get(), bare)
	}
}
