// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

const wlText = "sent 12 SendMessage call(s) in the observed window; 3 memory entries recorded"

// No line is wider than the widget, at any width, any font scale, with or
// without the scrollbar: a resize re-wraps.
func TestWrapListNoLineWiderThanTheWidget(t *testing.T) {
	items := []string{wlText, "  /Users/someone/Documents/VCS/GIT/localhost/a-very-long-unbreakable-path-name", "", "x"}
	for _, scale := range []int{1, 2, 3} {
		f := NewBitmapFont(scale)
		for _, w := range []int{20, 48, 90, 140, 233, 400, 900} {
			for _, h := range []int{30, 120, 4000} {
				l := NewWrapList(items)
				l.Base.Font = f
				l.SetBounds(Rect{W: w, H: h})
				for _, line := range l.Lines() {
					// A bitmap glyph is wider than a degenerate width: a line holds at least one rune.
					if got := f.Measure(line); got > w-2*l.textPad() && len([]rune(strings.TrimLeft(line, " "))) > 1 {
						t.Errorf("scale %d, %dx%d: line %q is %d px wide, room %d", scale, w, h, line, got, w-2*l.textPad())
					}
				}
			}
		}
	}
}

// Resizing re-wraps: narrower gives more lines, wider fewer, and the same text
// is kept in order.
func TestWrapListRewrapsOnResize(t *testing.T) {
	l := NewWrapList([]string{wlText})
	l.SetBounds(Rect{W: 600, H: 4000})
	wide := append([]string(nil), l.Lines()...)
	l.SetBounds(Rect{W: 160, H: 4000})
	narrow := append([]string(nil), l.Lines()...)
	if len(wide) != 1 || len(narrow) < 3 {
		t.Fatalf("wide %d lines, narrow %d", len(wide), len(narrow))
	}
	if got := strings.Join(strings.Fields(strings.Join(narrow, " ")), " "); got != wlText {
		t.Errorf("text changed by wrapping: %q", got)
	}
	if !strings.HasPrefix(narrow[1], "  ") || strings.HasPrefix(narrow[0], " ") {
		t.Errorf("continuation not indented: %q", narrow[:2])
	}
	l.SetBounds(Rect{W: 600, H: 4000})
	if len(l.Lines()) != 1 {
		t.Errorf("widening did not unwrap: %d lines", len(l.Lines()))
	}
}

// The pitch is never under the glyph box, at any font scale, so lines cannot
// overlap; and Measure is exactly the lines at that pitch.
func TestWrapListPitchAndMeasure(t *testing.T) {
	for _, scale := range []int{1, 2, 3, 5} {
		f := NewBitmapFont(scale)
		l := NewWrapList([]string{wlText, "b"})
		l.Base.Font = f
		if l.LineHeight() < f.Height()+1 {
			t.Errorf("scale %d: pitch %d, glyph %d", scale, l.LineHeight(), f.Height())
		}
		l.SetBounds(Rect{W: 200, H: 4000})
		if got, want := l.Measure(200), len(l.Lines())*l.LineHeight(); got != want {
			t.Errorf("scale %d: Measure %d, want %d", scale, got, want)
		}
	}
}

// Scrolling ends with the last line wholly in view, and a click on the thumb or a
// key keeps the position in range.
func TestWrapListScrollEndsOnTheLastLine(t *testing.T) {
	var items []string
	for i := 0; i < 40; i++ {
		items = append(items, "line number "+strings.Repeat("x", i%7))
	}
	l := NewWrapList(items)
	lh := l.LineHeight()
	l.SetBounds(Rect{W: 300, H: 10*lh + lh/2}) // ten lines and a half
	l.ScrollBy(1 << 20)
	if got, want := l.ScrollLine().Get(), len(l.Lines())-10; got != want {
		t.Fatalf("scrolled to %d, want %d", got, want)
	}
	l.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if l.ScrollLine().Get() != 0 {
		t.Errorf("Home -> %d", l.ScrollLine().Get())
	}
	l.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if l.ScrollLine().Get() != len(l.Lines())-10 {
		t.Errorf("End -> %d", l.ScrollLine().Get())
	}
	l.OnEvent(Event{Kind: EventScroll, Delta: -3})
	if l.ScrollLine().Get() != len(l.Lines())-13 {
		t.Errorf("wheel -> %d", l.ScrollLine().Get())
	}
	// Fewer items than fit: no scrolling at all.
	l.SetItems([]string{"a"})
	if l.ScrollLine().Get() != 0 {
		t.Errorf("short list scrolled to %d", l.ScrollLine().Get())
	}
}

// A box sizes a WrapList to its wrapped height with AddNatural, and the height
// follows the width.
func TestWrapListInAColumnFollowsTheWidth(t *testing.T) {
	l := NewWrapList([]string{wlText})
	col := NewVBox()
	col.AddNatural(l)
	col.AddFlex(NewLabel("rest"), 1)
	col.SetBounds(Rect{W: 600, H: 600})
	one := l.Bounds().H
	col.SetBounds(Rect{W: 160, H: 600})
	if l.Bounds().H <= one || l.Bounds().H != l.Measure(160) {
		t.Errorf("height %d at 160, %d at 600, Measure(160) %d", l.Bounds().H, one, l.Measure(160))
	}
}

// Draw paints inside the widget only, with and without a scrollbar.
func TestWrapListDrawStaysInside(t *testing.T) {
	const bw, bh, w, h = 300, 200, 90, 60
	for _, n := range []int{1, 30} {
		items := make([]string, n)
		for i := range items {
			items[i] = wlText
		}
		l := NewWrapList(items)
		l.SetBounds(Rect{W: w, H: h})
		buf := make([]byte, 4*bw*bh)
		l.Draw(painter.NewPixelPainter(buf, bw, bh), DefaultLight())
		if x, y, ok := paintedIn(buf, bw, bh, w, bw); ok {
			t.Errorf("%d items: pixel (%d,%d) right of the widget", n, x, y)
		}
		if _, _, ok := paintedIn(buf, bw, bh, 0, w); !ok {
			t.Errorf("%d items: nothing painted", n)
		}
	}
}

// A disabled list ignores the wheel and the keys; a list with no height has no
// scrollbar to grab.
func TestWrapListDisabledAndEmptyExtent(t *testing.T) {
	items := make([]string, 50)
	for i := range items {
		items[i] = "line"
	}
	l := NewWrapList(items)
	l.SetBounds(Rect{W: 200, H: 5 * l.LineHeight()})
	l.Disabled().Set(true)
	l.OnEvent(Event{Kind: EventScroll, Delta: 5})
	l.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if l.ScrollLine().Get() != 0 {
		t.Errorf("a disabled list scrolled to %d", l.ScrollLine().Get())
	}
	l.Disabled().Set(false)
	l.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if l.ScrollLine().Get() != 5 {
		t.Errorf("an enabled list scrolled to %d", l.ScrollLine().Get())
	}
	l.SetBounds(Rect{W: 200, H: 0})
	if _, ok := l.scrollbarGeom(); ok {
		t.Error("a list with no height has a scrollbar")
	}
	l.SetBounds(Rect{W: 200, H: 100 * l.LineHeight()})
	if _, ok := l.scrollbarGeom(); ok {
		t.Error("a list that fits has a scrollbar")
	}
}

type countingFont struct {
	Font
	n *int
}

func (c countingFont) Measure(s string) int { *c.n++; return c.Font.Measure(s) }

// Laying out is done once per SetBounds or SetItems, not once per question.
func TestWrapListLaysOutOncePerChange(t *testing.T) {
	n := 0
	l := NewWrapList([]string{wlText})
	l.Base.Font = countingFont{NewBitmapFont(1), &n}
	l.SetBounds(Rect{W: 160, H: 4000})
	l.Lines()
	after := n
	l.Lines()
	l.Lines()
	if n != after {
		t.Errorf("Lines re-wrapped: %d Measure calls, then %d", after, n)
	}
	l.SetItems([]string{"other text"})
	l.Lines()
	if n == after {
		t.Error("SetItems did not re-wrap")
	}
}

func TestWrapItemCases(t *testing.T) {
	f := NewBitmapFont(1)
	cases := []struct {
		item string
		room int
		want []string
	}{
		{"", 100, []string{""}},
		{"   ", 100, []string{""}},
		{"ab cd", 100, []string{"ab cd"}},
		{"ab cd ef", 6 * 5, []string{"ab cd", "  ef"}},
		{"  ab cd", 6 * 6, []string{"  ab", "    cd"}},
		{"abcdefghij", 6 * 4, []string{"abcd", "  ef", "  gh", "  ij"}},
		{"abc", 1, []string{"a", "  b", "  c"}},
	}
	for _, c := range cases {
		if got := wrapItem(f, c.item, c.room); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrapItem(%q, %d) = %q, want %q", c.item, c.room, got, c.want)
		}
	}
}
