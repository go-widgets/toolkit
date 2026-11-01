// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"testing"
)

// numbered is n lines "line 1".."line n", with the lines named in repl
// replaced.
func numbered(n int, repl map[int]string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if s, ok := repl[i]; ok {
			if s != "" {
				b.WriteString(s + "\n")
			}
			continue
		}
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestDiffViewLayoutFoldsAndCountsChanges(t *testing.T) {
	old := numbered(40, nil)
	new := numbered(40, map[int]string{5: "five", 30: ""}) // one changed, one removed
	v := NewDiffView(old, new)
	if v.Changes() != 2 {
		t.Fatalf("changes = %d, want 2", v.Changes())
	}
	if add, del := v.Stats(); add != 1 || del != 2 {
		t.Fatalf("stats +%d -%d", add, del)
	}
	folds := 0
	for _, r := range v.rows {
		if r.kind == dvFold {
			folds++
		}
	}
	// before line 5: 4 lines < 3+... kept; between 5 and 30 and after 30: folded
	if folds != 2 {
		t.Fatalf("folds = %d, want 2 (rows %d)", folds, v.Rows())
	}
	// never folding shows every line
	v.Context = -1
	v.layout()
	if v.Rows() != 40 { // side by side: the changed line faces its replacement
		t.Fatalf("unfolded rows = %d, want 40", v.Rows())
	}
	// inline: the replacement takes two rows
	v.SetLayout(false)
	if v.Rows() != 41 {
		t.Fatalf("inline rows = %d, want 41", v.Rows())
	}
}

func TestDiffViewNavigation(t *testing.T) {
	old := numbered(200, nil)
	new := numbered(200, map[int]string{20: "x", 100: "y", 180: "z"})
	v := NewDiffView(old, new)
	v.SetBounds(Rect{W: 400, H: 10 * v.rowH()})
	if v.Current() != -1 {
		t.Fatal("a change is current before any move")
	}
	if !v.NextChange(-1) || v.Current() != 2 {
		t.Fatalf("previous from the start goes to the last: %d", v.Current())
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "F7"})
	if v.Current() != 0 {
		t.Fatalf("F7 wraps to the first: %d", v.Current())
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "n"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "F7", Shift: true})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "p"})
	if v.Current() != 2 {
		t.Fatalf("n, Shift+F7, p: %d", v.Current())
	}
	// the current change is in view
	if c := v.changes[v.Current()]; c < v.Top() || c >= v.Top()+v.visible() {
		t.Fatalf("change row %d not in view (top %d)", c, v.Top())
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if v.Top() != 0 {
		t.Fatal("Home")
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	end := v.Top()
	if end != v.Rows()-v.visible() {
		t.Fatalf("End: top %d", end)
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"}) // clamped
	v.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if v.Top() != end {
		t.Fatal("scrolled past the end")
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "PageUp"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if v.Top() != end-v.visible() {
		t.Fatalf("PageUp+ArrowUp: %d", v.Top())
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"}) // not ours: nothing
	v.OnEvent(Event{Kind: EventClick})
	if !v.Focused() {
		t.Fatal("a click focuses")
	}
	// switching layout keeps the current change
	v.SetLayout(false)
	v.SetLayout(false) // the same: nothing
	if v.Current() != 2 || v.SideBySide {
		t.Fatal("SetLayout lost the change")
	}
	// no changes: nowhere to go
	same := NewDiffView("a\nb\n", "a\nb\n")
	if same.NextChange(1) || same.Changes() != 0 {
		t.Fatal("a change in identical texts")
	}
	same.SetLayout(false)
	if same.Current() != -1 {
		t.Fatal("current without changes")
	}
}

// Text against its row at 4.5:1, markers against their row and the ruler at
// 3:1, the current-change outline at 3:1, in both default themes.
func TestDiffViewContrast(t *testing.T) {
	for name, th := range map[string]*Theme{"light": DefaultLight(), "dark": DefaultDark()} {
		c := diffColors(th)
		check := func(what string, fg, bg RGBA, min float64) {
			if r := ContrastRatio(fg, bg); r < min {
				t.Errorf("%s: %s %.2f:1 < %.1f:1", name, what, r, min)
			}
		}
		check("added text", c.addInk, c.addFill, 4.5)
		check("removed text", c.delInk, c.delFill, 4.5)
		check("plain text", th.OnSurface, th.Surface, 4.5)
		check("line numbers on surface", c.numInk, th.Surface, 4.5)
		check("line numbers on added", c.numInk, c.addFill, 3)
		check("line numbers on removed", c.numInk, c.delFill, 3)
		check("fold", c.foldInk, th.Surface, 4.5)
		check("added mark", c.addMark, c.addFill, 3)
		check("removed mark", c.delMark, c.delFill, 3)
		check("added mark on ruler", c.addMark, th.SurfaceAlt, 3)
		check("removed mark on ruler", c.delMark, th.SurfaceAlt, 3)
		check("ruler window", th.OnSurface, th.SurfaceAlt, 3)
		check("outline on added", th.Accent, c.addFill, 3)
		check("outline on removed", th.Accent, c.delFill, 3)
		check("added text on a changed word", c.addInk, c.addWord, 4.5)
		check("removed text on a changed word", c.delInk, c.delWord, 4.5)
		// the changed word's fill must stand out from its row's
		check("changed word on its added row", c.addWord, c.addFill, 1.25)
		check("changed word on its removed row", c.delWord, c.delFill, 1.25)
	}
}

func TestWordChanges(t *testing.T) {
	a, b := WordChanges("# Hello, {{ .name }}!Q", "# Hello, {{ .name }}!Q (draft 2)")
	if len(a) != 0 || len(b) != 1 || "# Hello, {{ .name }}!Q (draft 2)"[b[0][0]:b[0][1]] != " (draft 2)" {
		t.Fatalf("a %v b %v", a, b)
	}
	a, b = WordChanges("the quick brown fox", "the quick red fox")
	if "the quick brown fox"[a[0][0]:a[0][1]] != "brown" || "the quick red fox"[b[0][0]:b[0][1]] != "red" {
		t.Fatalf("a %v b %v", a, b)
	}
	if a, b := WordChanges(strings.Repeat("x", 5000), "y"); a != nil || b != nil {
		t.Fatal("a long line was split")
	}
	if got := words("ab  c,d"); len(got) != 5 {
		t.Fatalf("words %v", got)
	}
}

// The coordinator's case: a narrow side-by-side view, a change past column
// 40. The changed words must be on screen: their fill painted, and ink (non
// background pixels) inside the region it covers.
func TestDiffViewShowsAChangePastColumn40(t *testing.T) {
	th := DefaultLight()
	c := diffColors(th)
	head := strings.Repeat("abcde ", 8) // 48 columns, then the change
	for _, sbs := range []bool{true, false} {
		const w, h = 360, 200
		v := NewDiffView(head+"old ending\n", head+"new ending (draft 2)\n")
		v.SetLayout(sbs)
		v.SetBounds(Rect{W: w, H: h})
		buf := makeSurface(w, h)
		v.Draw(newP(buf, w), th)
		// the right side's changed words: their bounding box
		x0, y0, x1, y1 := w, h, -1, -1
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pixelAt(buf, w, x, y) == c.addWord {
					x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				}
			}
		}
		if x1 < 0 {
			t.Fatalf("side-by-side=%v: the changed words are not shown", sbs)
		}
		ink := 0
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if px := pixelAt(buf, w, x, y); px != c.addWord && px != c.addFill {
					ink++
				}
			}
		}
		if ink < 20 {
			t.Fatalf("side-by-side=%v: only %d ink pixels in the changed region", sbs, ink)
		}
		// and the long line wrapped: its row is taller than one line
		changed := 0
		for i, r := range v.rows {
			if r.kind != dvEqual && v.heights[i] > 1 {
				changed++
			}
		}
		if changed == 0 {
			t.Fatalf("side-by-side=%v: the long changed line did not wrap", sbs)
		}
	}
}

// Wrapping keeps every character: the pieces cover the line end to end, each
// within the width, and a break falls after a space when there is one.
func TestDiffViewWrapCoversTheLine(t *testing.T) {
	v := NewDiffView("", "")
	for _, s := range []string{"", "short", strings.Repeat("word ", 40), strings.Repeat("x", 300), "été à l’école, ça va"} {
		w := v.glyphAdvance() * 12
		ps := v.wrap(s, w)
		at := 0
		for _, p := range ps {
			if p[0] != at {
				t.Fatalf("%q: gap at %d", s, at)
			}
			if p[1] > p[0]+1 && v.width(s[p[0]:p[1]]) > w {
				t.Fatalf("%q: piece %q too wide", s, s[p[0]:p[1]])
			}
			at = p[1]
		}
		if at != len(s) {
			t.Fatalf("%q: ends at %d", s, at)
		}
	}
	ps := v.wrap(strings.Repeat("word ", 10), v.width("word word "))
	if s := strings.Repeat("word ", 10); s[ps[0][0]:ps[0][1]] != "word word " {
		t.Fatalf("not broken at a space: %q", s[ps[0][0]:ps[0][1]])
	}
}

// Draw paints the added and removed fills, the header, the blanks facing a
// change, the fold, the ruler and the focus ring, in both layouts and both
// themes, without painting outside its bounds.
func TestDiffViewDraw(t *testing.T) {
	old := numbered(30, map[int]string{12: ""})
	new := numbered(30, map[int]string{3: "three", 25: "twenty-five\nextra"})
	for _, th := range []*Theme{DefaultLight(), DefaultDark()} {
		for _, sbs := range []bool{true, false} {
			c := diffColors(th)
			const w, h = 420, 400
			buf := makeSurface(w+20, h+20)
			v := NewDiffView(old, new)
			v.OldTitle, v.NewTitle = "Version 3", "Now"
			v.SetLayout(sbs)
			v.SetBounds(Rect{X: 10, Y: 10, W: w, H: h})
			v.SetFocused(true)
			v.NextChange(1)
			v.Draw(newP(buf, w+20), th)
			seen := map[RGBA]bool{}
			for y := 0; y < h+20; y++ {
				for x := 0; x < w+20; x++ {
					px := pixelAt(buf, w+20, x, y)
					if (x < 10 || y < 10 || x >= w+10 || y >= h+10) && px != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
						t.Fatalf("painted outside at (%d,%d)", x, y)
					}
					seen[px] = true
				}
			}
			for what, col := range map[string]RGBA{"added fill": c.addFill, "removed fill": c.delFill, "header": th.SurfaceAlt, "accent": th.Accent, "added mark": c.addMark, "removed mark": c.delMark} {
				if !seen[col] {
					t.Errorf("side-by-side=%v dark=%v: no %s", sbs, isDarkTheme(th), what)
				}
			}
		}
	}
}

// A long title and long lines stay in their half: nothing of the left side is
// painted over the right one.
func TestDiffViewClipsEachHalf(t *testing.T) {
	th := DefaultLight()
	const w, h = 200, 60
	long := strings.Repeat("W", 80)
	v := NewDiffView(long+"\n", long+"x\n")
	v.OldTitle, v.NewTitle = long, ""
	v.SetBounds(Rect{W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), th)
	half := (w - scaled(6)) / 2
	for y := 1; y < v.rowH()-1; y++ {
		for x := half + 2; x < w-scaled(6)-2; x++ {
			if px := pixelAt(buf, w, x, y); px != th.SurfaceAlt {
				t.Fatalf("the old title ran into the right half at (%d,%d): %+v", x, y, px)
			}
		}
	}
}

func TestDiffViewEdgeCases(t *testing.T) {
	th := DefaultLight()
	// empty against empty: no rows, the ruler draws nothing
	v := NewDiffView("", "")
	v.SetBounds(Rect{W: 100, H: 40})
	buf := makeSurface(100, 40)
	v.Draw(newP(buf, 100), th)
	if v.Rows() != 0 || v.headerH() != 0 {
		t.Fatal("rows in nothing")
	}
	// a tiny view still shows one row and clamps
	v.SetTexts("a\n", "b\n")
	v.SetBounds(Rect{W: 100, H: 1})
	if v.visible() != 1 {
		t.Fatal("visible")
	}
	v.ScrollBy(10)
	if v.Top() != v.Rows()-1 {
		t.Fatalf("top %d rows %d", v.Top(), v.Rows())
	}
	// everything changed, at the start and the end: no fold at the edges
	v.SetTexts(numbered(10, nil), "other\n")
	for _, r := range v.rows {
		if r.kind == dvFold {
			t.Fatal("folded with nothing unchanged")
		}
	}
	// unchanged lines only at the very start and end fold on one side
	v.SetTexts(numbered(30, nil), numbered(30, map[int]string{15: "x"}))
	if v.rows[0].kind != dvFold || v.rows[len(v.rows)-1].kind != dvFold {
		t.Fatalf("edge folds: first %v last %v", v.rows[0].kind, v.rows[len(v.rows)-1].kind)
	}
	a := v.A11y()
	if a.Role != RoleDocument || !strings.Contains(a.Value, "1 changes") {
		t.Fatalf("a11y %+v", a)
	}
}
