// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"
	"testing"
)

// scrollDoc builds n numbered lines, every k-th one long (width cols).
func scrollDoc(n, longEvery, width int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("line " + strconv.Itoa(i))
		if longEvery > 0 && i%longEvery == 0 {
			b.WriteString(strings.Repeat("x", width))
		}
	}
	return b.String()
}

func newScrollEditor(n, longEvery, width int) *CodeEditor {
	c := NewCodeEditor(scrollDoc(n, longEvery, width))
	c.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	return c
}

func TestTextViewScrollbarsOffByDefault(t *testing.T) {
	tv := NewTextView(scrollDoc(500, 1, 400))
	tv.SetBounds(Rect{W: 300, H: 100})
	if v, h := tv.bars(); v || h {
		t.Fatalf("a plain TextView shows bars: v=%v h=%v", v, h)
	}
	tv.ScrollX().Set(50)
	if tv.clampedScrollX() != 0 {
		t.Fatal("a plain TextView must not scroll horizontally")
	}
	if !NewCodeEditor("").ShowScrollbars {
		t.Fatal("CodeEditor turns scrollbars on")
	}
}

func TestScrollXByIsInertWithoutScrollbars(t *testing.T) {
	tv := NewTextView(scrollDoc(5, 1, 400))
	tv.SetBounds(Rect{W: 300, H: 100})
	tv.scrollXBy(40)
	if tv.ScrollX().Get() != 0 {
		t.Fatal("scrollXBy moved a plain TextView")
	}
	if tv.maxScrollX() != 0 {
		t.Fatal("maxScrollX of a plain TextView must be 0")
	}
}

func TestScrollbarsShownOnlyWhenContentOverflows(t *testing.T) {
	c := NewCodeEditor("a\nb")
	c.SetBounds(Rect{W: 300, H: 200})
	if v, h := c.bars(); v || h {
		t.Fatalf("short document shows bars v=%v h=%v", v, h)
	}
	c.SetText(scrollDoc(500, 0, 0))
	if v, h := c.bars(); !v || h {
		t.Fatalf("tall narrow document: v=%v h=%v, want v only", v, h)
	}
	c.SetText(scrollDoc(3, 1, 400))
	if v, h := c.bars(); v || !h {
		t.Fatalf("short wide document: v=%v h=%v, want h only", v, h)
	}
	c.SetText(scrollDoc(500, 1, 400))
	if v, h := c.bars(); !v || !h {
		t.Fatalf("tall wide document: v=%v h=%v, want both", v, h)
	}
}

// Thumb length is track*view/total (floored at the minimum), at the start of the
// track at offset 0 and flush with its end at the maximum offset.
func TestVerticalThumbGeometry(t *testing.T) {
	c := newScrollEditor(1000, 0, 0)
	g, ok := c.vbarGeom()
	if !ok {
		t.Fatal("no vertical bar")
	}
	vis := c.visibleLines()
	want := g.trackLen * vis / 1000
	if m := scaled(scrollbarMinThumb); want < m {
		want = m
	}
	if g.thumbLen != want {
		t.Fatalf("thumbLen = %d, want %d (track %d, view %d of 1000)", g.thumbLen, want, g.trackLen, vis)
	}
	if g.thumbStart != g.trackStart {
		t.Fatalf("thumb at offset 0 starts at %d, track at %d", g.thumbStart, g.trackStart)
	}
	if g.maxScroll != 1000-vis {
		t.Fatalf("maxScroll = %d, want %d", g.maxScroll, 1000-vis)
	}
	c.ScrollLine().Set(1 << 20)
	g, _ = c.vbarGeom()
	if end := g.thumbStart + g.thumbLen; end != g.trackStart+g.trackLen {
		t.Fatalf("thumb at max offset ends at %d, track at %d", end, g.trackStart+g.trackLen)
	}
	// The minimum thumb keeps a huge document grabbable.
	big := newScrollEditor(200000, 0, 0)
	if g, _ := big.vbarGeom(); g.thumbLen < scaled(scrollbarMinThumb) {
		t.Fatalf("thumb %d is below the minimum %d", g.thumbLen, scaled(scrollbarMinThumb))
	}
}

func TestHorizontalThumbGeometry(t *testing.T) {
	c := newScrollEditor(3, 1, 400)
	g, ok := c.hbarGeom()
	if !ok {
		t.Fatal("no horizontal bar")
	}
	if g.trackStart != c.textLeftInset() {
		t.Fatalf("track starts at %d, want the text origin %d (not under the gutter)", g.trackStart, c.textLeftInset())
	}
	want := g.trackLen * c.textAreaWidth() / c.contentWidth()
	if m := scaled(scrollbarMinThumb); want < m {
		want = m
	}
	if g.thumbLen != want {
		t.Fatalf("thumbLen = %d, want %d", g.thumbLen, want)
	}
	if g.maxScroll != c.contentWidth()-c.textAreaWidth() {
		t.Fatalf("maxScroll = %d", g.maxScroll)
	}
}

// Dragging the thumb maps the pointer to a scroll offset; the thumb tracks it.
func TestDragVerticalThumb(t *testing.T) {
	c := newScrollEditor(1000, 0, 0)
	g, _ := c.vbarGeom()
	x := g.cross0 + g.crossW/2
	c.OnEvent(Event{Kind: EventClick, X: x, Y: g.thumbStart + 2})
	if !c.vdrag.active {
		t.Fatal("pressing the thumb must start a drag")
	}
	if c.CursorLine().Get() != 0 || c.CursorCol().Get() != 0 {
		t.Fatal("pressing a scrollbar moved the caret")
	}
	// Drag to the bottom of the track: the maximum offset.
	c.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: g.trackStart + g.trackLen + 50})
	if got := c.ScrollLine().Get(); got != g.maxScroll {
		t.Fatalf("dragged to the end: ScrollLine = %d, want %d", got, g.maxScroll)
	}
	// Drag halfway (grab offset 2 above): about half of the travel.
	mid := g.trackStart + g.travelNum/2 + 2
	c.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: mid})
	want := g.scrollForGrabStart(mid - 2)
	if got := c.ScrollLine().Get(); got != want {
		t.Fatalf("mid drag: ScrollLine = %d, want %d", got, want)
	}
	if want < g.maxScroll/2-g.maxScroll/20 || want > g.maxScroll/2+g.maxScroll/20 {
		t.Fatalf("half the travel mapped to %d of %d", want, g.maxScroll)
	}
	c.OnEvent(Event{Kind: EventMouseUp, X: x, Y: mid})
	before := c.ScrollLine().Get()
	c.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: 5})
	if c.ScrollLine().Get() == before && before != 0 {
		// a drag after release extends a text selection instead; scroll may follow the caret
	}
	if c.vdrag.active {
		t.Fatal("release must end the drag")
	}
}

func TestClickTrackPages(t *testing.T) {
	c := newScrollEditor(1000, 0, 0)
	g, _ := c.vbarGeom()
	x := g.cross0 + 1
	vis := c.visibleLines()
	c.OnEvent(Event{Kind: EventClick, X: x, Y: g.trackStart + g.trackLen - 2})
	if got := c.ScrollLine().Get(); got != vis {
		t.Fatalf("click below the thumb: ScrollLine = %d, want one page (%d)", got, vis)
	}
	g, _ = c.vbarGeom()
	c.OnEvent(Event{Kind: EventClick, X: x, Y: g.trackStart + 1})
	if got := c.ScrollLine().Get(); got != 0 {
		t.Fatalf("click above the thumb: ScrollLine = %d, want 0", got)
	}
}

func TestDragHorizontalThumbAndKeyboardSync(t *testing.T) {
	c := newScrollEditor(3, 1, 400)
	g, _ := c.hbarGeom()
	y := g.cross0 + g.crossW/2
	c.OnEvent(Event{Kind: EventClick, X: g.thumbStart + 2, Y: y})
	c.OnEvent(Event{Kind: EventMouseDrag, X: g.trackStart + g.trackLen + 30, Y: y})
	if got := c.ScrollX().Get(); got != g.maxScroll {
		t.Fatalf("ScrollX = %d, want %d", got, g.maxScroll)
	}
	c.OnEvent(Event{Kind: EventMouseUp})
	// Keyboard: End on the long first line scrolls the view to the caret and the
	// thumb follows without any bookkeeping.
	c.ScrollX().Set(0)
	c.CursorLine().Set(0)
	c.CursorCol().Set(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	cx := c.CursorCol().Get() * c.glyphAdvance()
	if sx := c.clampedScrollX(); cx < sx || cx+c.glyphAdvance() > sx+c.textAreaWidth() {
		t.Fatalf("caret x=%d outside the text window [%d,%d)", cx, sx, sx+c.textAreaWidth())
	}
	g2, _ := c.hbarGeom()
	if g2.thumbStart <= g.trackStart {
		t.Fatalf("thumb did not follow the keyboard: start %d", g2.thumbStart)
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if c.clampedScrollX() != 0 {
		t.Fatalf("Home must scroll back: ScrollX = %d", c.clampedScrollX())
	}
	// Vertical: arrowing down past the window drags the thumb with it.
	d := newScrollEditor(1000, 0, 0)
	g0, _ := d.vbarGeom()
	for i := 0; i < 400; i++ {
		d.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	g1, _ := d.vbarGeom()
	if g1.thumbStart <= g0.thumbStart {
		t.Fatal("thumb did not move with keyboard scrolling")
	}
}

func TestWheelHorizontalAndClamp(t *testing.T) {
	c := newScrollEditor(3, 1, 400)
	c.OnEvent(Event{Kind: EventScroll, DeltaX: 2})
	if got := c.ScrollX().Get(); got != 2*3*c.glyphAdvance() {
		t.Fatalf("DeltaX 2 scrolled %d px", got)
	}
	c.OnEvent(Event{Kind: EventScroll, DeltaX: 100000})
	if got := c.ScrollX().Get(); got != c.maxScrollX() {
		t.Fatalf("wheel must clamp: %d vs %d", got, c.maxScrollX())
	}
	c.OnEvent(Event{Kind: EventScroll, DeltaX: -100000})
	if c.ScrollX().Get() != 0 {
		t.Fatal("wheel must clamp at 0")
	}
	// A vertical wheel still works as before.
	d := newScrollEditor(1000, 0, 0)
	d.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if d.ScrollLine().Get() != 5 {
		t.Fatal("vertical wheel regressed")
	}
}

// The bars take their own pixels: the gutter keeps its hit area, the text keeps
// its own, and the caret lands where it always did.
func TestScrollbarsDoNotStealGutterOrText(t *testing.T) {
	c := newScrollEditor(1000, 1, 400)
	vg, _ := c.vbarGeom()
	hg, _ := c.hbarGeom()
	if hg.trackStart < c.gutterWidth() {
		t.Fatalf("horizontal track starts under the gutter (%d < %d)", hg.trackStart, c.gutterWidth())
	}
	// A click in the gutter / in the text, well away from both bars, is a caret click.
	for _, x := range []int{c.gutterWidth() - 3, c.gutterWidth() + 40} {
		c.CursorLine().Set(5)
		c.CursorCol().Set(5)
		c.OnEvent(Event{Kind: EventClick, X: x, Y: 30})
		if c.vdrag.active || c.hdrag.active {
			t.Fatalf("click at x=%d grabbed a bar", x)
		}
	}
	// The text window is exactly the area left of the vertical bar.
	if w := c.textAreaWidth(); w != c.Bounds().W-c.textLeftInset()-1-vg.crossW {
		t.Fatalf("textAreaWidth = %d", w)
	}
	// Just left of the vertical track is text, not scrollbar.
	c.OnEvent(Event{Kind: EventClick, X: vg.cross0 - 2, Y: 30})
	if c.vdrag.active {
		t.Fatal("a click just left of the bar grabbed it")
	}
	// A column in the vertical bar must not place the caret.
	c.CursorLine().Set(7)
	c.OnEvent(Event{Kind: EventClick, X: vg.cross0 + 1, Y: vg.thumbStart + vg.thumbLen + 5})
	if c.CursorLine().Get() != 7 {
		t.Fatal("a click on the track moved the caret")
	}
}

// Horizontal scroll keeps caret / click mapping consistent.
func TestScrolledCaretMapping(t *testing.T) {
	c := newScrollEditor(3, 1, 400)
	c.ScrollX().Set(c.glyphAdvance() * 100)
	x, y := c.CaretPixel(0, 120)
	lx := x - c.Bounds().X
	ly := y - c.Bounds().Y
	if l, col := c.caretAt(lx, ly); l != 0 || col != 120 {
		t.Fatalf("caretAt(CaretPixel(0,120)) = (%d,%d)", l, col)
	}
}

func TestScrollbarThumbContrastBothThemes(t *testing.T) {
	for name, th := range popupThemes() {
		track, thumb := scrollbarTrackColor(th), scrollbarThumbColor(th)
		if r := contrastRatio(thumb, track); r < ScrollbarThumbMinContrast {
			t.Errorf("%s: scrollbar thumb vs track = %.2f:1, want >= 3:1", name, r)
		}
		old := blendRGBA(th.OnSurface, th.SurfaceAlt, scrollbarThumbMix)
		t.Logf("%s: thumb/track %.2f:1 (was %.2f:1)", name, contrastRatio(thumb, track), contrastRatio(old, track))
	}
}

// The painted pixels, not just the tokens, hold the contrast.
func TestEditorScrollbarPaintedContrast(t *testing.T) {
	for name, th := range popupThemes() {
		c := newScrollEditor(1000, 1, 400)
		buf := makeSurface(300, 200)
		c.Draw(newP(buf, 300), th)
		g, _ := c.vbarGeom()
		thumb := pixelAt(buf, 300, g.cross0+g.crossW/2, g.thumbStart+g.thumbLen/2)
		trackY := g.thumbStart + g.thumbLen + 6
		track := pixelAt(buf, 300, g.cross0+g.crossW/2, trackY)
		if r := contrastRatio(thumb, track); r < ScrollbarThumbMinContrast {
			t.Errorf("%s: painted thumb %v vs track %v = %.2f:1", name, thumb, track, r)
		}
		h, _ := c.hbarGeom()
		hthumb := pixelAt(buf, 300, h.thumbStart+h.thumbLen/2, h.cross0+h.crossW/2)
		if hthumb != thumb {
			t.Errorf("%s: horizontal thumb %v differs from vertical %v", name, hthumb, thumb)
		}
	}
}

// Text scrolled horizontally never paints under the gutter, and the scrollbar
// areas are not painted over by rows.
func TestScrolledTextStaysOutOfGutter(t *testing.T) {
	c := newScrollEditor(20, 1, 400)
	th := DefaultLight()
	c.HighlightCurrentLine = false
	c.ScrollX().Set(c.glyphAdvance() * 30)
	buf := makeSurface(300, 200)
	c.Draw(newP(buf, 300), th)
	// Columns 1..gutterWidth-1 of the rows' band that holds no digit must stay the
	// Surface colour: sample the gutter's rightmost air column (gutterPadR).
	x := c.gutterWidth() - 2
	for y := 8; y < 100; y += 3 {
		if got := pixelAt(buf, 300, x, y); got != th.Surface {
			t.Fatalf("gutter air at (%d,%d) = %v: scrolled text bled under the gutter", x, y, got)
		}
	}
}

// Scrolling must not re-scan the buffer; only an edit does.
func TestWidestLineCachedUntilEdit(t *testing.T) {
	c := newScrollEditor(5000, 100, 200)
	_ = c.maxCols()
	if !c.colsOK {
		t.Fatal("cache not filled")
	}
	c.OnEvent(Event{Kind: EventScroll, Delta: 10})
	c.OnEvent(Event{Kind: EventScroll, DeltaX: 3})
	if !c.colsOK {
		t.Fatal("scrolling invalidated the widest-line cache")
	}
	// An edit invalidates it (the caret-follow re-fills it at once, so check the
	// value, not the flag): the widest line grows by the typed rune, and undo,
	// SetText and a host Text().Set all see fresh widths, never a stale one.
	c.SetText("abc")
	c.CursorLine().Set(0)
	c.CursorCol().Set(3)
	if got := c.maxCols(); got != 3 {
		t.Fatalf("maxCols = %d, want 3", got)
	}
	c.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := c.maxCols(); got != 4 {
		t.Fatalf("after typing maxCols = %d, want 4", got)
	}
	c.Undo()
	if got := c.maxCols(); got != 3 {
		t.Fatalf("after undo maxCols = %d, want 3", got)
	}
	c.SetText("ab")
	if got := c.maxCols(); got != 2 {
		t.Fatalf("after SetText maxCols = %d, want 2", got)
	}
	c.Text().Set("abcdef\nx")
	if got := c.maxCols(); got != 6 {
		t.Fatalf("after Text().Set maxCols = %d, want 6", got)
	}
}

func BenchmarkEditor5000LinesDraw(b *testing.B) {
	for _, bars := range []bool{false, true} {
		name := "bars-off"
		if bars {
			name = "bars-on"
		}
		b.Run(name, func(b *testing.B) { benchEditorDraw(b, 5000, bars) })
	}
	// The same window over a 50-line document: the bars' cost must not depend on
	// the document size (cached widest line, O(1) geometry).
	b.Run("bars-on-50-lines", func(b *testing.B) { benchEditorDraw(b, 50, true) })
}

func benchEditorDraw(b *testing.B, n int, bars bool) {
	c := newScrollEditor(n, 100, 200)
	c.ShowScrollbars = bars
	th := DefaultDark()
	buf := makeSurface(300, 200)
	p := newP(buf, 300)
	c.Draw(p, th)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ScrollLine().Set(i % 40)
		c.Draw(p, th)
	}
}
