// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// --- recording painter -------------------------------------------------------

type fillCall struct {
	r painter.Rect
	c painter.RGBA
}
type strokeCall struct {
	r painter.Rect
	c painter.RGBA
	w int
}
type textCall struct {
	x, y int
	s    string
	c    painter.RGBA
}

// capPainter records every draw primitive so tests can assert exact rects,
// colours, and positions. It implements painter.Clipper so VirtualList's
// overflow-clip path is exercised too.
type capPainter struct {
	fills   []fillCall
	strokes []strokeCall
	texts   []textCall
	pushes  int
	pops    int
}

func (p *capPainter) FillRect(r painter.Rect, c painter.RGBA) {
	p.fills = append(p.fills, fillCall{r, c})
}
func (p *capPainter) StrokeRect(r painter.Rect, c painter.RGBA, w int) {
	p.strokes = append(p.strokes, strokeCall{r, c, w})
}
func (p *capPainter) FillRoundRect(r painter.Rect, radius int, c painter.RGBA)       {}
func (p *capPainter) StrokeRoundRect(r painter.Rect, rad int, c painter.RGBA, l int) {}
func (p *capPainter) PutPixel(x, y int, c painter.RGBA)                              {}
func (p *capPainter) Text(x, y int, s string, ink painter.RGBA) {
	p.texts = append(p.texts, textCall{x, y, s, ink})
}
func (p *capPainter) Size() (int, int)        { return 4096, 4096 }
func (p *capPainter) PushClip(r painter.Rect) { p.pushes++ }
func (p *capPainter) PopClip()                { p.pops++ }

var _ painter.Clipper = (*capPainter)(nil)

// cardTheme is a theme with distinct, assertion-friendly colours.
func cardTheme() *toolkit.Theme {
	return &toolkit.Theme{
		Background: toolkit.RGB(10, 20, 30),
		Surface:    toolkit.RGB(40, 50, 60),
		SurfaceAlt: toolkit.RGB(70, 80, 90),
		OnSurface:  toolkit.RGB(200, 200, 200),
		Accent:     toolkit.RGB(1, 2, 3),
	}
}

// newList builds a CardList over 0..n-1 with uniform DefaultRowHeight rows,
// bounds (0,0,w,h), and a no-op CardRender, ready for events.
func newList(n, w, h int) *CardList[int] {
	m := mvvm.NewObservableList[int](intItems(n)...)
	c := NewCardList(m, nil, func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int, st CardState) {})
	c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})
	return c
}

// --- construction ------------------------------------------------------------

func TestNewCardListDefaults(t *testing.T) {
	c := newList(100, 100, 100)
	if c.Selected != -1 {
		t.Fatalf("Selected = %d, want -1", c.Selected)
	}
	if c.VirtualList == nil || c.VirtualList.Render == nil {
		t.Fatal("VirtualList not wired with a Render callback")
	}
	if c.Model.Len() != 100 {
		t.Fatalf("model len = %d, want 100", c.Model.Len())
	}
	if c.pullThreshold() != DefaultPullRows {
		t.Fatalf("pullThreshold = %d, want %d", c.pullThreshold(), DefaultPullRows)
	}
	c.PullRows = 7
	if c.pullThreshold() != 7 {
		t.Fatalf("pullThreshold = %d, want 7", c.pullThreshold())
	}
	// Interface satisfaction (compile-time asserted; touch at runtime too).
	var _ toolkit.Widget = c
	var _ toolkit.Animator = c
}

// --- renderCard: ring, veil, state -------------------------------------------

func TestRenderCardStateRingAndVeil(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(10)...)
	var got []CardState
	c := NewCardList(m, nil, func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int, st CardState) {
		got = append(got, st)
	})
	c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 100, H: 200}) // all 10 rows visible (10*20=200)
	c.Selected = 3
	c.Dimmed = func(i int) bool { return i%2 == 0 }

	p := &capPainter{}
	th := cardTheme()
	c.Draw(p, th)

	if len(got) != 10 {
		t.Fatalf("CardRender invoked %d times, want 10", len(got))
	}
	for i, st := range got {
		if st.Selected != (i == 3) {
			t.Errorf("row %d Selected = %v", i, st.Selected)
		}
		if st.Dimmed != (i%2 == 0) {
			t.Errorf("row %d Dimmed = %v", i, st.Dimmed)
		}
	}
	// Selection ring: exactly one StrokeRect, Accent, at row 3's rect.
	if len(p.strokes) != 1 {
		t.Fatalf("stroke count = %d, want 1", len(p.strokes))
	}
	wantRing := painter.Rect{X: 0, Y: 60, W: 100, H: 20}
	if p.strokes[0].r != wantRing || p.strokes[0].c != th.Accent || p.strokes[0].w != cardSelectRingWidth {
		t.Fatalf("ring = %+v, want rect %+v accent %+v w %d", p.strokes[0], wantRing, th.Accent, cardSelectRingWidth)
	}
	// Veil: one FillRect per even row, Background at cardDimAlpha.
	veil := th.Background
	veil.A = cardDimAlpha
	var veils int
	for _, f := range p.fills {
		if f.c == veil {
			veils++
			if f.r.W != 100 || f.r.H != 20 || f.r.Y%40 != 0 {
				t.Errorf("veil at unexpected rect %+v", f.r)
			}
		}
	}
	if veils != 5 {
		t.Fatalf("veil count = %d, want 5", veils)
	}
}

func TestRenderCardNilCardRenderAndNilDimmed(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(3)...)
	// CardRender nil: renderCard must still layer ring + veil without panicking.
	c := &CardList[int]{
		VirtualList: &VirtualList[int]{Model: m},
		Selected:    0,
		Dimmed:      func(i int) bool { return true },
		topSpin:     &toolkit.Spinner{},
		botSpin:     &toolkit.Spinner{},
	}
	c.VirtualList.Render = c.renderCard
	c.VirtualList.ensure()
	c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 50, H: 60})

	p := &capPainter{}
	c.Draw(p, cardTheme())
	if len(p.strokes) != 1 {
		t.Fatalf("nil CardRender: strokes = %d, want 1 (ring)", len(p.strokes))
	}
	if len(p.fills) != 3 {
		t.Fatalf("nil CardRender: fills = %d, want 3 (veils)", len(p.fills))
	}

	// Dimmed nil: no veil at all.
	c2 := newList(3, 50, 60)
	c2.Selected = -1
	p2 := &capPainter{}
	c2.Draw(p2, cardTheme())
	if len(p2.fills) != 0 || len(p2.strokes) != 0 {
		t.Fatalf("nil Dimmed / no selection: fills %d strokes %d, want 0/0", len(p2.fills), len(p2.strokes))
	}
}

// --- pull-to-fetch strips ----------------------------------------------------

func TestDrawStripsBothEdges(t *testing.T) {
	c := newList(100, 120, 300)
	c.FetchingTop = true
	c.FetchingBottom = true
	c.TopLabel = "older"
	c.BottomLabel = "" // no label on the bottom strip

	p := &capPainter{}
	th := cardTheme()
	c.Draw(p, th)

	if !c.topSpin.Active().Get() || !c.botSpin.Active().Get() {
		t.Fatal("strip spinners not activated by Fetching flags")
	}
	// Two Surface bands: top at y=0, bottom at y=300-28=272, both full width.
	var top, bottom bool
	for _, f := range p.fills {
		if f.c != th.Surface {
			continue
		}
		if f.r == (painter.Rect{X: 0, Y: 0, W: 120, H: cardStripHeight}) {
			top = true
		}
		if f.r == (painter.Rect{X: 0, Y: 300 - cardStripHeight, W: 120, H: cardStripHeight}) {
			bottom = true
		}
	}
	if !top || !bottom {
		t.Fatalf("strip bands missing: top=%v bottom=%v", top, bottom)
	}
	// Exactly one label (top); bottom has none.
	if len(p.texts) != 1 || p.texts[0].s != "older" {
		t.Fatalf("labels = %+v, want single \"older\"", p.texts)
	}
}

func TestDrawStripClampsToTinyViewport(t *testing.T) {
	c := newList(100, 100, 10) // viewport shorter than a strip
	c.FetchingTop = true
	p := &capPainter{}
	th := cardTheme()
	c.Draw(p, th)
	var found bool
	for _, f := range p.fills {
		if f.c == th.Surface {
			found = true
			if f.r.H != 10 { // sh clamped to r.H
				t.Fatalf("clamped strip height = %d, want 10", f.r.H)
			}
		}
	}
	if !found {
		t.Fatal("clamped strip band not drawn")
	}
	// spinner square clamped to sh=10.
	if b := c.topSpin.Bounds(); b.W != 10 || b.H != 10 {
		t.Fatalf("clamped spinner bounds = %+v, want 10x10", b)
	}
}

func TestDrawStripZeroBounds(t *testing.T) {
	c := newList(100, 0, 0)
	c.FetchingTop = true
	c.FetchingBottom = true
	p := &capPainter{}
	c.Draw(p, cardTheme())
	if len(p.fills) != 0 || len(p.texts) != 0 {
		t.Fatalf("zero-bounds strip drew fills %d texts %d, want 0/0", len(p.fills), len(p.texts))
	}
}

// --- Animator ----------------------------------------------------------------

func TestAnimatingAndTick(t *testing.T) {
	c := newList(10, 100, 100)
	if c.Animating() {
		t.Fatal("Animating true with no fetch in flight")
	}
	c.FetchingTop = true
	if !c.Animating() {
		t.Fatal("Animating false while FetchingTop")
	}
	c.FetchingTop = false
	c.FetchingBottom = true
	if !c.Animating() {
		t.Fatal("Animating false while FetchingBottom")
	}

	// Tick advances only the active spinner(s) by dt.
	c.FetchingTop = true
	c.FetchingBottom = true
	c.topSpin.Phase = 0
	c.botSpin.Phase = 0
	c.Tick(0.25)
	if c.topSpin.Phase != 0.25 || c.botSpin.Phase != 0.25 {
		t.Fatalf("phases = %v/%v, want 0.25/0.25", c.topSpin.Phase, c.botSpin.Phase)
	}
	// Inactive spinner is not ticked.
	c.FetchingTop = false
	c.topSpin.Phase = 0
	c.botSpin.Phase = 0
	c.Tick(0.5)
	if c.topSpin.Phase != 0 {
		t.Fatalf("inactive top phase = %v, want 0", c.topSpin.Phase)
	}
	if c.botSpin.Phase != 0.5 {
		t.Fatalf("active bottom phase = %v, want 0.5", c.botSpin.Phase)
	}
	// Drive through the tree walk too (CardList is the Animator leaf).
	if !toolkit.TreeAnimating(c) {
		t.Fatal("TreeAnimating false for a fetching CardList")
	}
	toolkit.TickTree(c, 0.1)
	if c.botSpin.Phase != 0.6 {
		t.Fatalf("TickTree bottom phase = %v, want 0.6", c.botSpin.Phase)
	}
}

// --- ScrollToBottom ----------------------------------------------------------

func TestScrollToBottom(t *testing.T) {
	c := newList(100, 100, 100) // total 2000, maxOff 1900
	c.ScrollToBottom()
	if c.ScrollOffset != 1900 {
		t.Fatalf("ScrollOffset = %d, want 1900", c.ScrollOffset)
	}
	short := newList(2, 100, 100) // total 40 < viewport
	short.ScrollToBottom()
	if short.ScrollOffset != 0 {
		t.Fatalf("short ScrollOffset = %d, want 0", short.ScrollOffset)
	}
}

// --- keyboard navigation + activation ----------------------------------------

func TestKeyboardNavigation(t *testing.T) {
	c := newList(100, 100, 100) // 5 rows visible, page = 5
	var sel []int
	c.OnSelect = func(i int) { sel = append(sel, i) }

	key := func(code string) { c.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: code}) }

	key("ArrowDown") // from -1 -> 0
	if c.Selected != 0 {
		t.Fatalf("after ArrowDown from -1: Selected = %d, want 0", c.Selected)
	}
	key("ArrowDown") // -> 1
	key("ArrowUp")   // -> 0
	if c.Selected != 0 {
		t.Fatalf("after down,down,up: Selected = %d, want 0", c.Selected)
	}
	key("PageDown") // 0 -> 5
	if c.Selected != 5 {
		t.Fatalf("PageDown: Selected = %d, want 5", c.Selected)
	}
	key("PageUp") // 5 -> 0
	if c.Selected != 0 {
		t.Fatalf("PageUp: Selected = %d, want 0", c.Selected)
	}
	key("End") // -> 99, scroll so its bottom meets the viewport bottom
	if c.Selected != 99 {
		t.Fatalf("End: Selected = %d, want 99", c.Selected)
	}
	if c.ScrollOffset != 1900 {
		t.Fatalf("End scroll-into-view offset = %d, want 1900", c.ScrollOffset)
	}
	key("Home") // -> 0, scroll to top
	if c.Selected != 0 || c.ScrollOffset != 0 {
		t.Fatalf("Home: Selected=%d off=%d, want 0/0", c.Selected, c.ScrollOffset)
	}
	// Non-navigation key: no change, no OnSelect.
	before := len(sel)
	key("x")
	if c.Selected != 0 || len(sel) != before {
		t.Fatalf("unknown key mutated state (sel now %d, was %d)", len(sel), before)
	}
}

func TestScrollSelectedIntoView(t *testing.T) {
	c := newList(100, 100, 100) // vh 100, rows 20px

	// Select a row below the viewport: scroll so its bottom == off+vh.
	c.Selected = 10 // top 200, bot 220
	c.scrollSelectedIntoView()
	if c.ScrollOffset != 120 { // 220 - 100
		t.Fatalf("below-view offset = %d, want 120", c.ScrollOffset)
	}
	// Select a row above the viewport: scroll to its top.
	c.Selected = 2 // top 40
	c.scrollSelectedIntoView()
	if c.ScrollOffset != 40 {
		t.Fatalf("above-view offset = %d, want 40", c.ScrollOffset)
	}
	// Select a row already visible: no change.
	c.Selected = 4 // top 80, bot 100, within [40,140)
	c.scrollSelectedIntoView()
	if c.ScrollOffset != 40 {
		t.Fatalf("in-view offset = %d, want 40 (unchanged)", c.ScrollOffset)
	}
	// Guards: no selection, out of range, zero height.
	c.Selected = -1
	c.scrollSelectedIntoView()
	if c.ScrollOffset != 40 {
		t.Fatalf("no-selection changed offset to %d", c.ScrollOffset)
	}
	c.Selected = 1000
	c.scrollSelectedIntoView()
	if c.ScrollOffset != 40 {
		t.Fatalf("out-of-range changed offset to %d", c.ScrollOffset)
	}
	z := newList(100, 100, 0)
	z.Selected = 5
	z.scrollSelectedIntoView() // vh<=0 -> no-op, must not panic
}

func TestActivate(t *testing.T) {
	c := newList(10, 100, 100)
	var activated []int
	c.OnActivate = func(i int) { activated = append(activated, i) }
	enter := func() { c.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: "Enter"}) }

	// Nothing selected: no activation.
	enter()
	if len(activated) != 0 {
		t.Fatalf("activated with no selection: %v", activated)
	}
	c.Selected = 4
	enter()
	if len(activated) != 1 || activated[0] != 4 {
		t.Fatalf("activated = %v, want [4]", activated)
	}
	// Out-of-range selection: no activation.
	c.Selected = 999
	enter()
	if len(activated) != 1 {
		t.Fatalf("out-of-range activated: %v", activated)
	}
	// nil OnActivate is safe.
	c.OnActivate = nil
	c.Selected = 2
	enter()
}

func TestPageRowsEmpty(t *testing.T) {
	c := newList(0, 100, 100) // empty model -> VisibleRange count 0
	if got := c.pageRows(); got != 1 {
		t.Fatalf("pageRows on empty list = %d, want 1", got)
	}
}

// --- click selection ---------------------------------------------------------

func TestClickSelection(t *testing.T) {
	c := newList(100, 100, 100) // off 0
	var sel []int
	c.OnSelect = func(i int) { sel = append(sel, i) }

	c.OnEvent(toolkit.Event{Kind: toolkit.EventClick, Y: 25}) // -> row 1
	if c.Selected != 1 || len(sel) != 1 || sel[0] != 1 {
		t.Fatalf("click y=25: Selected=%d sel=%v", c.Selected, sel)
	}

	// Click above the content (negative local Y): no selection change.
	c.OnEvent(toolkit.Event{Kind: toolkit.EventClick, Y: -5})
	if c.Selected != 1 || len(sel) != 1 {
		t.Fatalf("negative-Y click changed selection: Selected=%d sel=%v", c.Selected, sel)
	}
	// Click beyond the content bottom.
	c.OnEvent(toolkit.Event{Kind: toolkit.EventClick, Y: 5000})
	if len(sel) != 1 {
		t.Fatalf("beyond-content click selected: %v", sel)
	}

	// Empty model.
	e := newList(0, 100, 100)
	if e.rowAtLocal(10) != -1 {
		t.Fatal("rowAtLocal on empty list != -1")
	}
	// Zero-height viewport.
	z := newList(100, 100, 0)
	if z.rowAtLocal(10) != -1 {
		t.Fatal("rowAtLocal on zero-height viewport != -1")
	}
}

// --- wheel scroll wiring -----------------------------------------------------

func TestScrollEventScrollsAndNotes(t *testing.T) {
	c := newList(100, 100, 100)
	c.OnEvent(toolkit.Event{Kind: toolkit.EventScroll, Delta: 2})
	if c.ScrollOffset != 40 { // 2 rows * 20
		t.Fatalf("wheel offset = %d, want 40", c.ScrollOffset)
	}
	// An unhandled event kind is a no-op.
	c.OnEvent(toolkit.Event{Kind: toolkit.EventMouseMove, X: 1, Y: 1})
	if c.ScrollOffset != 40 {
		t.Fatalf("mouse-move changed offset to %d", c.ScrollOffset)
	}
}

// --- infinite-scroll accumulator (noteScroll) --------------------------------

func TestNoteScrollTopThreshold(t *testing.T) {
	c := newList(100, 100, 100) // total 2000, vh 100, near-top zone off<100
	var tops int
	c.OnReachTop = func() { tops++ }
	c.ScrollOffset = 0 // at the top

	c.noteScroll(-1) // pullTop 1
	c.noteScroll(-1) // pullTop 2
	if tops != 0 {
		t.Fatalf("fired before threshold: tops=%d", tops)
	}
	c.noteScroll(-1) // pullTop 3 == threshold -> fire once
	if tops != 1 {
		t.Fatalf("did not fire at threshold: tops=%d", tops)
	}
	c.noteScroll(-1) // armed -> no repeat
	if tops != 1 {
		t.Fatalf("re-fired while armed: tops=%d", tops)
	}
	// A pull the other way resets the top edge.
	c.noteScroll(+1)
	if c.armedTop || c.pullTop != 0 {
		t.Fatalf("top edge not reset: armed=%v pull=%d", c.armedTop, c.pullTop)
	}
	// After reset, a fresh deliberate pull fires again.
	c.noteScroll(-1)
	c.noteScroll(-1)
	c.noteScroll(-1)
	if tops != 2 {
		t.Fatalf("did not re-fire after reset: tops=%d", tops)
	}
}

func TestNoteScrollBottomThreshold(t *testing.T) {
	c := newList(100, 100, 100) // maxOff 1900
	var bottoms int
	c.OnReachBottom = func() { bottoms++ }
	c.ScrollOffset = 1900 // at the bottom (near-bottom zone maxOff-off<100)

	c.noteScroll(+1)
	c.noteScroll(+1)
	c.noteScroll(+1) // threshold
	if bottoms != 1 {
		t.Fatalf("bottom did not fire at threshold: bottoms=%d", bottoms)
	}
	c.noteScroll(+1) // armed
	if bottoms != 1 {
		t.Fatalf("bottom re-fired while armed: bottoms=%d", bottoms)
	}
	// Pull away resets the bottom edge.
	c.noteScroll(-1)
	if c.armedBottom || c.pullBottom != 0 {
		t.Fatalf("bottom edge not reset: armed=%v pull=%d", c.armedBottom, c.pullBottom)
	}
}

func TestNoteScrollGuardsAndNilCallbacks(t *testing.T) {
	// Not near either edge: nothing accumulates, both edges reset.
	c := newList(100, 100, 100)
	c.ScrollOffset = 900 // off 900: not <100, and 1900-900=1000 not <100
	c.OnReachTop = func() { t.Fatal("OnReachTop fired away from top") }
	c.OnReachBottom = func() { t.Fatal("OnReachBottom fired away from bottom") }
	c.noteScroll(-1)
	c.noteScroll(+1)

	// Zero-height viewport: early return, no accumulation.
	z := newList(100, 100, 0)
	z.OnReachTop = func() { t.Fatal("OnReachTop fired with zero viewport") }
	z.noteScroll(-1)
	if z.pullTop != 0 {
		t.Fatalf("zero-viewport accumulated: %d", z.pullTop)
	}

	// nil callbacks: crossing the threshold arms the edge without panicking.
	n := newList(100, 100, 100)
	n.ScrollOffset = 0
	n.noteScroll(-1)
	n.noteScroll(-1)
	n.noteScroll(-1)
	if !n.armedTop {
		t.Fatal("nil OnReachTop: top edge not armed at threshold")
	}
	n.ScrollOffset = 1900
	n.noteScroll(+1)
	n.noteScroll(+1)
	n.noteScroll(+1)
	if !n.armedBottom {
		t.Fatal("nil OnReachBottom: bottom edge not armed at threshold")
	}

	// Content shorter than the viewport (maxOff clamped to 0): still fires top.
	short := newList(2, 100, 100)
	var fired bool
	short.OnReachTop = func() { fired = true }
	short.noteScroll(-1)
	short.noteScroll(-1)
	short.noteScroll(-1)
	if !fired {
		t.Fatal("short content did not fire OnReachTop")
	}
}

// --- selectMove exotic branches ----------------------------------------------

func TestSelectMoveBranches(t *testing.T) {
	if _, ok := selectMove(5, 0, 3, "ArrowDown"); ok {
		t.Fatal("selectMove on empty list returned ok")
	}
	if _, ok := selectMove(5, 10, 3, "Backspace"); ok {
		t.Fatal("selectMove on non-nav key returned ok")
	}
	if v, ok := selectMove(-1, 10, 3, "PageDown"); !ok || v != 3 {
		t.Fatalf("PageDown from -1 = %d,%v, want 3,true", v, ok)
	}
	if v, ok := selectMove(-1, 10, 3, "PageUp"); !ok || v != 0 {
		t.Fatalf("PageUp from -1 = %d,%v, want 0,true", v, ok)
	}
	if v, ok := selectMove(-1, 10, 3, "ArrowUp"); !ok || v != 0 {
		t.Fatalf("ArrowUp from -1 = %d,%v, want 0,true", v, ok)
	}
}

// --- MVVM integration: insert above keeps selection anchored -----------------

func TestInsertAboveKeepsView(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	c := NewCardList(m, nil, func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int, st CardState) {})
	c.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 100, H: 100})
	c.ScrollTo(400) // row 20 at the top
	before := c.ScrollOffset
	m.Insert(0, -1) // prepend one row above the viewport
	if c.ScrollOffset != before+DefaultRowHeight {
		t.Fatalf("insert above shifted offset to %d, want %d (anchor held)", c.ScrollOffset, before+DefaultRowHeight)
	}
}
