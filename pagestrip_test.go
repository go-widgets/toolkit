// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func newStrip(n int) *PageStrip {
	s := NewPageStrip(n)
	s.SetBounds(Rect{X: 3, Y: 4, W: 160, H: 400})
	return s
}

func solid(w, h int, c RGBA) []byte {
	b := make([]byte, w*h*4)
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = c.R, c.G, c.B, c.A
	}
	return b
}

func TestPageStripVirtualisesTheWindow(t *testing.T) {
	s := newStrip(500)
	first, last := s.Visible()
	if first != 0 || last < 1 || last > 12 {
		t.Fatalf("window = [%d,%d] on 500 pages", first, last)
	}
	asked := map[int]int{}
	s.NeedImage = func(i int) { asked[i]++ }
	th := DefaultLight()
	buf := makeSurface(170, 420)
	s.Draw(newP(buf, 170), th)
	s.Draw(newP(buf, 170), th)
	if len(asked) > last+1+stripPrefetch+1 {
		t.Fatalf("asked for %d pages with a window of %d: not virtualised", len(asked), last-first+1)
	}
	for i := 0; i <= last+stripPrefetch; i++ {
		if asked[i] != 1 {
			t.Fatalf("page %d asked %d times, want exactly once", i, asked[i])
		}
	}
	if asked[last+stripPrefetch+1] != 0 || asked[499] != 0 {
		t.Fatal("pages far outside the window were requested")
	}
	// Scroll to the end: the last pages are the window, the first are not asked again.
	s.scrollBy(1 << 30)
	f2, l2 := s.Visible()
	if l2 != 499 || f2 < 480 {
		t.Fatalf("end window = [%d,%d]", f2, l2)
	}
	s.Draw(newP(buf, 170), th)
	if asked[499] != 1 {
		t.Fatal("the last page was not requested")
	}
}

func TestPageStripImageDrawPageAndPlaceholder(t *testing.T) {
	th := DefaultLight()
	s := newStrip(3)
	s.Current().Set(-1)
	card := s.cardRect(0)
	probe := func(buf []byte) RGBA {
		return pixelAt(buf, 170, s.Bounds().X+card.X+card.W/2, s.Bounds().Y+card.Y+3)
	}
	red := RGB(200, 20, 20)
	// placeholder first
	buf := makeSurface(170, 420)
	s.Draw(newP(buf, 170), th)
	if got := probe(buf); got != th.SurfaceAlt {
		t.Fatalf("placeholder = %v, want SurfaceAlt", got)
	}
	// DrawPage hook
	s.DrawPage = func(p painter.Painter, _ *Theme, r Rect, i int) { fillRect(p, r.X, r.Y, r.W, r.H, RGB(1, 2, 3)) }
	buf = makeSurface(170, 420)
	s.Draw(newP(buf, 170), th)
	if got := probe(buf); got != RGB(1, 2, 3) {
		t.Fatalf("DrawPage = %v", got)
	}
	// bitmap wins
	s.SetImage(0, solid(16, 9, red), 16, 9)
	buf = makeSurface(170, 420)
	s.Draw(newP(buf, 170), th)
	if got := probe(buf); got != red {
		t.Fatalf("bitmap = %v, want red", got)
	}
	// bad inputs are ignored
	s.SetImage(-1, solid(2, 2, red), 2, 2)
	s.SetImage(3, solid(2, 2, red), 2, 2)
	s.SetImage(1, solid(2, 2, red)[:4], 2, 2)
	s.SetImage(1, nil, 0, 0)
	if s.Resident() != 1 {
		t.Fatalf("Resident = %d", s.Resident())
	}
}

func TestPageStripInvalidateKeepsStaleUntilReplaced(t *testing.T) {
	s := newStrip(4)
	asked := 0
	s.NeedImage = func(i int) {
		if i == 1 {
			asked++
		}
	}
	buf := makeSurface(170, 420)
	th := DefaultDark()
	s.Draw(newP(buf, 170), th)
	s.SetImage(1, solid(16, 9, RGB(9, 9, 9)), 16, 9)
	s.Invalidate(1)
	s.Invalidate(-5)
	s.Invalidate(99)
	s.Draw(newP(buf, 170), th)
	if asked != 2 {
		t.Fatalf("Invalidate must ask again: asked %d times", asked)
	}
	if s.imgs[1] == nil {
		t.Fatal("the stale bitmap must stay until its replacement arrives")
	}
	s.InvalidateAll()
	s.Draw(newP(buf, 170), th)
	if asked != 3 {
		t.Fatalf("InvalidateAll must ask again: %d", asked)
	}
}

func TestPageStripEvictsTheFarthestResident(t *testing.T) {
	s := newStrip(300)
	s.MaxResident = 10
	for i := 0; i < 30; i++ {
		s.SetImage(i, solid(4, 3, RGB(1, 1, 1)), 4, 3)
	}
	if s.Resident() != 10 {
		t.Fatalf("Resident = %d, want the cap 10", s.Resident())
	}
	if s.imgs[29] == nil || s.imgs[0] != nil {
		t.Fatal("the farthest pages (from the newest) must go first")
	}
	if s.asked[0] {
		t.Fatal("an evicted page must be asked for again")
	}
	// default cap
	d := newStrip(300)
	if d.maxResident() != PageStripMaxResident {
		t.Fatal("default MaxResident")
	}
	// every other page is the page being kept: nothing to evict, no loop
	one := newStrip(3)
	one.MaxResident = 0
	one.nres = 200
	one.evict(0)
}

func TestPageStripSelectionClickKeysAndFollow(t *testing.T) {
	s := newStrip(50)
	var got []int
	s.Current().Subscribe(func(i int) { got = append(got, i) })
	c2 := s.cardRect(2)
	s.OnEvent(Event{Kind: EventClick, X: c2.X + 5, Y: c2.Y + 5})
	if s.Current().Get() != 2 {
		t.Fatalf("click selected %d", s.Current().Get())
	}
	// keys need focus
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if s.Current().Get() != 2 {
		t.Fatal("an unfocused strip must ignore keys")
	}
	s.SetFocused(true)
	for _, k := range []struct {
		code string
		want int
	}{{"ArrowDown", 3}, {"ArrowRight", 4}, {"ArrowUp", 3}, {"ArrowLeft", 2}, {"End", 49}, {"ArrowDown", 49}, {"Home", 0}, {"ArrowUp", 0}} {
		s.OnEvent(Event{Kind: EventKeyDown, Code: k.code})
		if s.Current().Get() != k.want {
			t.Fatalf("%s -> %d, want %d", k.code, s.Current().Get(), k.want)
		}
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	pd := s.Current().Get()
	if pd < 2 {
		t.Fatalf("PageDown -> %d", pd)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "PageUp"})
	if s.Current().Get() != 0 {
		t.Fatalf("PageUp -> %d", s.Current().Get())
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "q"})
	// The window follows the selection on the next Draw, both ways.
	buf := makeSurface(170, 420)
	s.Current().Set(40)
	s.Draw(newP(buf, 170), DefaultLight())
	if f, l := s.Visible(); 40 < f || 40 > l {
		t.Fatalf("selected page 40 not scrolled into view: [%d,%d]", f, l)
	}
	s.Current().Set(1)
	s.Draw(newP(buf, 170), DefaultLight())
	if f, l := s.Visible(); 1 < f || 1 > l {
		t.Fatalf("selected page 1 not scrolled into view: [%d,%d]", f, l)
	}
	// a click on nothing keeps the selection; clicks past the gap map to no page
	s.OnEvent(Event{Kind: EventClick, X: 2, Y: 100000})
	if s.Current().Get() != 1 {
		t.Fatal("a click outside every card changed the selection")
	}
	s.SetCount(0)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if s.PageAt(5, 5) != -1 {
		t.Fatal("PageAt on an empty strip")
	}
	if len(got) == 0 {
		t.Fatal("subscribers must see selection changes")
	}
	dis := newStrip(5)
	dis.Disabled().Set(true)
	dis.OnEvent(Event{Kind: EventClick, X: 40, Y: 20})
	if dis.Current().Get() != -1 {
		t.Fatal("a disabled strip must not select")
	}
}

func TestPageStripScrollbarDragPageAndWheel(t *testing.T) {
	s := newStrip(500)
	g, ok := s.barGeom()
	if !ok {
		t.Fatal("no bar over 500 pages")
	}
	if g.thumbLen < scaled(scrollbarMinThumb) {
		t.Fatalf("thumb %d below the minimum", g.thumbLen)
	}
	x := g.cross0 + 1
	s.OnEvent(Event{Kind: EventClick, X: x, Y: g.thumbStart + 2})
	s.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: g.trackStart + g.trackLen + 40})
	if s.clampedScroll() != s.maxScroll() {
		t.Fatalf("dragged to the end: scroll %d of %d", s.clampedScroll(), s.maxScroll())
	}
	s.OnEvent(Event{Kind: EventMouseUp})
	// Track click pages by one window.
	s.Scroll().Set(0)
	g, _ = s.barGeom()
	s.OnEvent(Event{Kind: EventClick, X: x, Y: g.trackStart + g.trackLen - 3})
	if s.clampedScroll() != s.viewLen() {
		t.Fatalf("track click scrolled %d, want a window (%d)", s.clampedScroll(), s.viewLen())
	}
	// Wheel
	s.Scroll().Set(0)
	s.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if s.clampedScroll() != 3*s.step() {
		t.Fatalf("wheel scrolled %d, want %d", s.clampedScroll(), 3*s.step())
	}
	s.OnEvent(Event{Kind: EventScroll, Delta: -100})
	if s.clampedScroll() != 0 {
		t.Fatal("wheel must clamp at 0")
	}
	// A click on the bar column selects nothing.
	s.Current().Set(-1)
	s.OnEvent(Event{Kind: EventClick, X: x, Y: g.trackStart + g.trackLen - 3})
	if s.Current().Get() != -1 {
		t.Fatal("a scrollbar click selected a page")
	}
	// No bar when everything fits.
	if _, ok := newStrip(2).barGeom(); ok {
		t.Fatal("a bar over two pages")
	}
}

func TestPageStripHorizontal(t *testing.T) {
	s := NewPageStrip(200)
	s.Horizontal = true
	s.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 110})
	c0, c1 := s.cardRect(0), s.cardRect(1)
	if c1.X <= c0.X+c0.W || c1.Y != c0.Y {
		t.Fatalf("cards not laid left to right: %v %v", c0, c1)
	}
	if c0.W <= c0.H {
		t.Fatalf("16:9 card should be wider than tall: %v", c0)
	}
	g, ok := s.barGeom()
	if !ok || !g.horizontal {
		t.Fatal("expected a horizontal bar")
	}
	buf := makeSurface(400, 110)
	s.Draw(newP(buf, 400), DefaultDark())
	s.OnEvent(Event{Kind: EventScroll, DeltaX: 2})
	if s.clampedScroll() == 0 {
		t.Fatal("DeltaX must scroll a horizontal strip")
	}
	s.OnEvent(Event{Kind: EventScroll, Delta: 1})
	f, _ := s.Visible()
	cr := s.cardRect(f + 1)
	s.OnEvent(Event{Kind: EventClick, X: cr.X + 3, Y: 20})
	if s.Current().Get() != f+1 {
		t.Fatalf("clicked card %d, selected %d", f+1, s.Current().Get())
	}
}

func TestPageStripAspectsAndCount(t *testing.T) {
	s := newStrip(3)
	s.SetAspects([][2]int{{1, 1}}) // wrong length: ignored
	s.SetAspects([][2]int{{1, 1}, {1, 2}, {0, 0}})
	a, b, c := s.cardRect(0), s.cardRect(1), s.cardRect(2)
	if a.H != a.W {
		t.Fatalf("square page: %v", a)
	}
	if b.H != 2*b.W {
		t.Fatalf("portrait page: %v", b)
	}
	if c.H*16 != c.W*9 && c.H != c.W*9/16 {
		t.Fatalf("an invalid aspect falls back to 16:9: %v", c)
	}
	s.PageW, s.PageH = 4, 3
	s.aspects = nil
	s.offs = nil
	if r := s.cardRect(0); r.H != r.W*3/4 {
		t.Fatalf("PageW:PageH = 4:3, card %v", r)
	}
	s.Current().Set(2)
	s.SetCount(2)
	if s.Current().Get() != 1 || s.Count() != 2 {
		t.Fatalf("SetCount must clamp Current: %d", s.Current().Get())
	}
	s.SetImage(1, solid(2, 2, RGB(1, 1, 1)), 2, 2)
	s.SetCount(1) // drops the bitmap of the removed page
	if s.Resident() != 0 {
		t.Fatalf("Resident = %d after dropping its page", s.Resident())
	}
	s.SetCount(-4)
	if s.Count() != 0 {
		t.Fatal("negative count")
	}
	buf := makeSurface(170, 420)
	s.Draw(newP(buf, 170), DefaultLight())
	s.SetBounds(Rect{})
	s.Draw(newP(buf, 170), DefaultLight())
}

// A host overlaying content on the cards places it from CardRect and follows
// Scroll: the rectangle is in surface coordinates and moves with the scroll.
func TestPageStripCardRectAndScrollObservable(t *testing.T) {
	s := newStrip(100) // X 3, Y 4
	r0, r1 := s.CardRect(0), s.CardRect(1)
	if r0.X < s.Bounds().X || r0.Y < s.Bounds().Y || r1.Y <= r0.Y+r0.H {
		t.Fatalf("CardRect not in surface coordinates / stacked: %v %v", r0, r1)
	}
	if s.CardRect(-1) != (Rect{}) || s.CardRect(100) != (Rect{}) {
		t.Fatal("out-of-range CardRect must be the zero Rect")
	}
	var seen []int
	s.Scroll().Subscribe(func(v int) { seen = append(seen, v) })
	s.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if len(seen) == 0 || seen[len(seen)-1] != s.clampedScroll() {
		t.Fatalf("Scroll observers saw %v, scroll is %d", seen, s.clampedScroll())
	}
	moved := s.CardRect(0)
	if moved.Y != r0.Y-s.clampedScroll() {
		t.Fatalf("card did not follow the scroll: %v vs %v at scroll %d", moved, r0, s.clampedScroll())
	}
}

func TestPageStripEdgeCases(t *testing.T) {
	e := NewPageStrip(0)
	e.SetBounds(Rect{W: 100, H: 100})
	if f, l := e.Visible(); f != 0 || l != -1 {
		t.Fatalf("Visible on an empty strip = %d,%d", f, l)
	}
	e.ScrollTo(3)
	e.ScrollTo(-1)
	e.OnEvent(Event{Kind: EventScroll, Delta: 2}) // step() on an empty strip
	if e.step() != 1 {
		t.Fatal("step of an empty strip")
	}
	// A hairline aspect still gets a one-pixel card.
	h := newStrip(2)
	h.SetAspects([][2]int{{5000, 1}, {1, 1}})
	if r := h.cardRect(0); r.H != 1 {
		t.Fatalf("hairline card %v", r)
	}
	// One page per window: PageDown still moves by one.
	one := NewPageStrip(5)
	one.SetBounds(Rect{W: 160, H: 60})
	one.SetFocused(true)
	one.Current().Set(0)
	one.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if one.Current().Get() != 1 {
		t.Fatalf("PageDown on a one-page window: %d", one.Current().Get())
	}
}

func TestPageStripNarrowShrinksTheCard(t *testing.T) {
	s := NewPageStrip(10)
	s.SetBounds(Rect{W: 60, H: 300})
	if s.cross() >= s.thumb() {
		t.Fatalf("card %d did not shrink below %d in a 60px strip", s.cross(), s.thumb())
	}
	s.SetBounds(Rect{W: 4, H: 300})
	if s.cross() < 1 {
		t.Fatal("card collapsed")
	}
	s.ThumbSize = 40
	if s.thumb() != scaled(40) {
		t.Fatal("ThumbSize ignored")
	}
}

func TestPageStripSelectedOutlineAndNumbersContrast(t *testing.T) {
	for name, th := range popupThemes() {
		s := newStrip(5)
		s.Current().Set(1)
		s.SetFocused(false)
		buf := makeSurface(170, 420)
		for i := 0; i+3 < len(buf); i += 4 {
			buf[i], buf[i+1], buf[i+2], buf[i+3] = th.Surface.R, th.Surface.G, th.Surface.B, 255
		}
		s.Draw(newP(buf, 170), th)
		cr := s.cardRect(1)
		out := pixelAt(buf, 170, s.Bounds().X+cr.X+cr.W/2, s.Bounds().Y+cr.Y-1)
		if r := contrastRatio(out, th.Surface); r < 3 {
			t.Errorf("%s: selected outline %v vs strip = %.2f:1, want >= 3:1", name, out, r)
		}
	}
}

func TestPageStripA11y(t *testing.T) {
	s := newStrip(7)
	if s.A11y().Role != RoleListbox || s.A11y().Value != "" {
		t.Fatalf("A11y = %+v", s.A11y())
	}
	s.Current().Set(2)
	if s.A11y().Value != "page 3 of 7" {
		t.Fatalf("A11y value = %q", s.A11y().Value)
	}
}

// The cost of a Draw does not depend on the number of pages.
func BenchmarkPageStripDraw(b *testing.B) {
	for _, n := range []int{10, 500, 100000} {
		b.Run(itoa(n), func(b *testing.B) {
			s := NewPageStrip(n)
			s.SetBounds(Rect{W: 160, H: 600})
			s.NeedImage = func(i int) { s.SetImage(i, solid(120, 68, RGB(9, 9, 9)), 120, 68) }
			buf := makeSurface(160, 600)
			p := newP(buf, 160)
			th := DefaultDark()
			s.Draw(p, th)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Scroll().Set((i * 37) % 5000)
				s.Draw(p, th)
			}
		})
	}
}
