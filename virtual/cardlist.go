// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// ---------------------------------------------------------------------------
// CardList — a card feed on top of VirtualList
// ---------------------------------------------------------------------------

// Tunables shared by every CardList. They are package constants rather than
// per-instance fields because a card feed reads best when its chrome (the
// selection ring weight, the read-item veil, the pull-to-fetch strip) is
// uniform across the whole application; the one knob a caller is expected to
// touch — how hard a pull has to be before it triggers a fetch — is the
// PullRows field.
const (
	// DefaultPullRows is the pull distance, in rows scrolled toward an edge
	// while the viewport is already within one screen of it, that a CardList
	// requires before it fires OnReachTop / OnReachBottom. A single wheel notch
	// (one row) is a micro-nudge and must not trigger a fetch; a deliberate pull
	// of several rows does. Overridable per instance via PullRows.
	DefaultPullRows = 3
	// cardDimAlpha is the opacity of the "already read" veil painted over a
	// dimmed card — theme.Background composited src-over at just under half, so
	// the card reads as muted without vanishing, in both light and dark themes.
	cardDimAlpha = 128
	// cardSelectRingWidth is the stroke weight of the selection ring drawn
	// around the selected card in theme.Accent.
	cardSelectRingWidth = 2
	// cardStripHeight is the height of the pull-to-fetch loading strip pinned to
	// the top / bottom viewport edge while a fetch is in flight.
	cardStripHeight = 28
	// cardStripSpinnerSize is the side of the square the strip's spinner spins
	// in, inset and vertically centred within the strip.
	cardStripSpinnerSize = 20
)

// CardState carries the per-card display flags a CardList computes for each
// visible card and hands to the caller's CardRender: whether the card is the
// selected one (so the caller can lift it, or simply let CardList draw the ring
// on top) and whether it is dimmed (an "already read" item — the caller may
// mute its own content, and CardList additionally veils it).
type CardState struct {
	// Selected is true for the one card at CardList.Selected.
	Selected bool
	// Dimmed is true when CardList.Dimmed reports this card as read/muted.
	Dimmed bool
}

// CardList binds a scrollable feed of cards to a live mvvm.ObservableList by
// composing a virtual.VirtualList[T]: the VirtualList owns the model
// subscription, the O(1)/O(log n) offset↔row index, the stable scroll anchor
// across mutations, and the recycled per-row drawing — CardList adds only what a
// card feed needs on top of a plain virtual list:
//
//   - a card-shaped Render wrapper that calls the caller's CardRender and then
//     paints the selection ring and the read-item veil over it;
//   - keyboard + click selection (Selected / OnSelect / OnActivate) with
//     scroll-into-view, the card-feed analogue of ListBox's cursor;
//   - infinite scroll: OnReachTop / OnReachBottom fire when the viewport is
//     pulled within one screen of an edge, gated by a per-edge accumulator so a
//     micro-nudge does not spam the loader;
//   - pull-to-fetch strips: while FetchingTop / FetchingBottom a spinning strip
//     is drawn over the corresponding edge, and CardList is itself an
//     [toolkit.Animator] so a host present loop driving [toolkit.TreeAnimating]
//     / [toolkit.TickTree] spins those strips with no per-app bookkeeping;
//   - ScrollToBottom for the "newest at the bottom, open at the bottom" feed.
//
// CardList embeds *VirtualList[T], so ScrollTo / ScrollBy / ScrollByRows /
// VisibleRange / Close / Bounds / SetBounds and the Model / RowHeight /
// ScrollOffset fields are all reachable directly; it overrides only Draw and
// OnEvent.
type CardList[T any] struct {
	*VirtualList[T]

	// CardRender draws one card into rectangle r (its exact on-screen span) for
	// item i, given its display state. It is invoked only for cards in the
	// viewport. CardList paints the selection ring and the dim veil ON TOP of
	// whatever CardRender draws, so the caller only draws the card's own content.
	CardRender func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T, state CardState)

	// Selected is the index of the selected card, or -1 for none. It doubles as
	// the keyboard cursor.
	Selected int
	// OnSelect fires with the new index whenever selection changes (arrow / page
	// key or click). nil is safe.
	OnSelect func(i int)
	// OnActivate fires with Selected when the selected card is activated (Enter,
	// the card-feed analogue of opening it). nil is safe.
	OnActivate func(i int)

	// Dimmed reports whether card i is "already read" / muted. CardList never
	// stores read state itself — the application owns it — it only asks. nil
	// means no card is dimmed.
	Dimmed func(i int) bool

	// OnReachTop fires when the viewport is deliberately pulled to within one
	// screen of the top (load older items when newest is at the bottom).
	OnReachTop func()
	// OnReachBottom fires when the viewport is deliberately pulled to within one
	// screen of the bottom.
	OnReachBottom func()
	// PullRows overrides DefaultPullRows: how many rows of edge-ward pull, while
	// already within one screen of that edge, are needed before OnReach* fires.
	// Zero uses DefaultPullRows.
	PullRows int

	// FetchingTop / FetchingBottom, when set by the app, draw a spinning
	// pull-to-fetch strip over the top / bottom viewport edge and make Animating
	// report true so a host keeps ticking. The app sets one true when it starts
	// a fetch (typically from OnReach*) and clears it when the fetch lands.
	FetchingTop    bool
	FetchingBottom bool
	// TopLabel / BottomLabel are optional captions drawn beside the strip
	// spinner (e.g. "Loading older…"). Empty draws just the spinner.
	TopLabel    string
	BottomLabel string

	topSpin, botSpin      *toolkit.Spinner
	pullTop, pullBottom   int
	armedTop, armedBottom bool
}

var (
	_ toolkit.Widget   = (*CardList[int])(nil)
	_ toolkit.Animator = (*CardList[int])(nil)
)

// NewCardList builds a CardList over model with the given per-row height and
// card-draw callbacks, wiring the underlying VirtualList (and its model
// subscription) immediately. Selection starts empty (Selected == -1).
func NewCardList[T any](
	model *mvvm.ObservableList[T],
	rowHeight func(i int) int,
	cardRender func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T, state CardState),
) *CardList[T] {
	c := &CardList[T]{
		VirtualList: &VirtualList[T]{Model: model, RowHeight: rowHeight},
		CardRender:  cardRender,
		Selected:    -1,
		topSpin:     &toolkit.Spinner{Style: toolkit.SpinnerDots},
		botSpin:     &toolkit.Spinner{Style: toolkit.SpinnerDots},
	}
	// Route VirtualList's per-row draw through the card wrapper, which layers the
	// ring + veil over the caller's CardRender.
	c.VirtualList.Render = c.renderCard
	c.VirtualList.ensure()
	return c
}

// renderCard is the VirtualList.Render callback: it draws the caller's card,
// then composites the read-item veil (Dimmed) and finally the selection ring
// (Selected) on top, so the ring stays crisp over a dimmed card.
func (c *CardList[T]) renderCard(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T) {
	st := CardState{Selected: i == c.Selected}
	if c.Dimmed != nil {
		st.Dimmed = c.Dimmed(i)
	}
	if c.CardRender != nil {
		c.CardRender(p, th, r, i, item, st)
	}
	if st.Dimmed {
		veil := th.Background
		veil.A = cardDimAlpha
		p.FillRect(r, veil)
	}
	if st.Selected {
		p.StrokeRect(r, th.Accent, cardSelectRingWidth)
	}
}

// Draw paints the cards (via the embedded VirtualList) and then, over the top,
// any active pull-to-fetch strip. The strip spinners' Active state is synced to
// the Fetching flags here so an app toggling a flag needs no other wiring.
func (c *CardList[T]) Draw(p painter.Painter, th *toolkit.Theme) {
	c.topSpin.Active = c.FetchingTop
	c.botSpin.Active = c.FetchingBottom
	c.VirtualList.Draw(p, th)
	if c.FetchingTop {
		c.drawStrip(p, th, true, c.topSpin, c.TopLabel)
	}
	if c.FetchingBottom {
		c.drawStrip(p, th, false, c.botSpin, c.BottomLabel)
	}
}

// drawStrip paints a loading strip pinned to the top (top == true) or bottom
// viewport edge: a filled Surface band spanning the width, a spinner inset at
// the left, and an optional label beside it. The band and spinner clamp to a
// viewport shorter than the strip so a tiny CardList still draws sanely.
func (c *CardList[T]) drawStrip(p painter.Painter, th *toolkit.Theme, top bool, sp *toolkit.Spinner, label string) {
	r := c.VirtualList.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	sh := cardStripHeight
	if sh > r.H {
		sh = r.H
	}
	y := r.Y
	if !top {
		y = r.Y + r.H - sh
	}
	p.FillRect(toolkit.Rect{X: r.X, Y: y, W: r.W, H: sh}, th.Surface)
	d := cardStripSpinnerSize
	if d > sh {
		d = sh
	}
	pad := (sh - d) / 2
	sp.SetBounds(toolkit.Rect{X: r.X + pad, Y: y + pad, W: d, H: d})
	sp.Draw(p, th)
	if label != "" {
		p.Text(r.X+pad+d+pad, y+sh/2, label, th.OnSurface)
	}
}

// Animating reports whether a pull-to-fetch strip is spinning — true exactly
// when a fetch is in flight — making CardList an [toolkit.Animator] so a host
// stops repainting once both strips are idle.
func (c *CardList[T]) Animating() bool { return c.FetchingTop || c.FetchingBottom }

// Tick advances whichever strip spinners are active by dt seconds. The host
// calls it once per frame (directly or through [toolkit.TickTree]); an inactive
// strip is left untouched, so a stopped feed costs nothing.
func (c *CardList[T]) Tick(dt float64) {
	if c.FetchingTop {
		c.topSpin.Tick(dt)
	}
	if c.FetchingBottom {
		c.botSpin.Tick(dt)
	}
}

// ScrollToBottom scrolls to the very end of the content — the "newest at the
// bottom, open at the bottom" gesture. The offset clamps to the maximum, so it
// is safe on a feed shorter than the viewport.
func (c *CardList[T]) ScrollToBottom() {
	c.VirtualList.ensure()
	c.VirtualList.ScrollTo(c.VirtualList.idx.total)
}

// OnEvent handles selection + activation keys, wheel scrolling, and clicks; it
// replaces VirtualList.OnEvent (which only scrolls). Scroll and keyboard
// navigation both feed the infinite-scroll accumulator via noteScroll.
func (c *CardList[T]) OnEvent(ev toolkit.Event) {
	switch ev.Kind {
	case toolkit.EventScroll:
		c.VirtualList.ScrollByRows(ev.Delta)
		c.noteScroll(ev.Delta)
	case toolkit.EventKeyDown:
		c.onKey(ev)
	case toolkit.EventClick:
		if row := c.rowAtLocal(ev.Y); row >= 0 {
			c.Selected = row
			if c.OnSelect != nil {
				c.OnSelect(row)
			}
		}
	}
}

// onKey moves the selection cursor (arrows / page / home / end, keeping it in
// view) or activates it (Enter). The signed row delta of a selection move is
// fed to the infinite-scroll accumulator, so navigating up into the top screen
// pulls older items exactly as a wheel would.
func (c *CardList[T]) onKey(ev toolkit.Event) {
	if ev.Code == "Enter" {
		c.activate()
		return
	}
	n := c.VirtualList.modelLen()
	if ns, ok := selectMove(c.Selected, n, c.pageRows(), ev.Code); ok {
		old := c.Selected
		c.Selected = ns
		if c.OnSelect != nil {
			c.OnSelect(ns)
		}
		c.scrollSelectedIntoView()
		c.noteScroll(ns - old)
	}
}

// activate fires OnActivate for the selected card, a no-op when nothing (valid)
// is selected or OnActivate is nil.
func (c *CardList[T]) activate() {
	if c.Selected < 0 || c.Selected >= c.VirtualList.modelLen() {
		return
	}
	if c.OnActivate != nil {
		c.OnActivate(c.Selected)
	}
}

// pageRows is the number of whole cards a PageUp / PageDown moves by — the count
// currently in the viewport, floored at 1 so a page always advances.
func (c *CardList[T]) pageRows() int {
	if _, count := c.VirtualList.VisibleRange(); count >= 1 {
		return count
	}
	return 1
}

// selectMove computes the new selection index for a navigation key, mirroring
// the core ListBox roving-cursor rules for the card feed: a fresh cursor
// (cur < 0) lands on the first row for an arrow, arrows step one card, pages
// jump a viewport, Home/End go to the ends. The bool is false for a
// non-navigation key (or an empty list), leaving the cursor untouched.
func selectMove(cur, n, page int, code string) (int, bool) {
	if n <= 0 {
		return cur, false
	}
	switch code {
	case "ArrowDown":
		if cur < 0 {
			return 0, true
		}
		return min(cur+1, n-1), true
	case "ArrowUp":
		if cur < 0 {
			return 0, true
		}
		return max(cur-1, 0), true
	case "PageDown":
		base := cur
		if base < 0 {
			base = 0
		}
		return min(base+page, n-1), true
	case "PageUp":
		base := cur
		if base < 0 {
			base = 0
		}
		return max(base-page, 0), true
	case "Home":
		return 0, true
	case "End":
		return n - 1, true
	default:
		return cur, false
	}
}

// scrollSelectedIntoView nudges the scroll offset the minimum amount that brings
// the selected card fully into the viewport: up to its top when it sits above,
// down so its bottom edge meets the viewport bottom when it sits below,
// otherwise nothing. A no-op when nothing is selected or the viewport has no
// height yet.
func (c *CardList[T]) scrollSelectedIntoView() {
	vl := c.VirtualList
	vl.ensure()
	if c.Selected < 0 || c.Selected >= vl.idx.n {
		return
	}
	vh := vl.Bounds().H
	if vh <= 0 {
		return
	}
	top := vl.idx.prefix(c.Selected)
	bot := top + vl.idx.heightAt(c.Selected)
	off := vl.clampOffset(vl.ScrollOffset)
	if top < off {
		vl.ScrollTo(top)
		return
	}
	if bot > off+vh {
		vl.ScrollTo(bot - vh)
	}
}

// rowAtLocal maps a widget-local Y (as delivered in a click) to the card index
// under it, or -1 when the click falls on no card (empty list, unsized
// viewport, or a Y outside the content).
func (c *CardList[T]) rowAtLocal(y int) int {
	vl := c.VirtualList
	vl.ensure()
	if vl.idx.n == 0 {
		return -1
	}
	if vl.Bounds().H <= 0 {
		return -1
	}
	cy := vl.clampOffset(vl.ScrollOffset) + y
	if cy < 0 || cy >= vl.idx.total {
		return -1
	}
	return vl.idx.locate(cy)
}

// pullThreshold is the effective pull distance (in rows) before OnReach* fires.
func (c *CardList[T]) pullThreshold() int {
	if c.PullRows > 0 {
		return c.PullRows
	}
	return DefaultPullRows
}

// noteScroll feeds a just-performed scroll of intent rows (negative toward the
// top, positive toward the bottom) into the per-edge pull accumulator. While the
// viewport is within one screen of an edge and the pull is edge-ward, the
// intent accumulates; once it crosses the threshold OnReach* fires ONCE (the
// armed flag suppresses repeats until the pull relaxes). Any pull away from the
// edge — or leaving its one-screen zone — resets that edge, so a fresh
// deliberate pull is required each time, not a drifting micro-nudge.
func (c *CardList[T]) noteScroll(intent int) {
	vl := c.VirtualList
	vl.ensure()
	vh := vl.Bounds().H
	if vh <= 0 {
		return
	}
	off := vl.clampOffset(vl.ScrollOffset)
	maxOff := vl.idx.total - vh
	if maxOff < 0 {
		maxOff = 0
	}
	thr := c.pullThreshold()

	if off < vh && intent < 0 {
		c.pullTop += -intent
		if c.pullTop >= thr && !c.armedTop {
			c.armedTop = true
			if c.OnReachTop != nil {
				c.OnReachTop()
			}
		}
	} else {
		c.pullTop = 0
		c.armedTop = false
	}

	if maxOff-off < vh && intent > 0 {
		c.pullBottom += intent
		if c.pullBottom >= thr && !c.armedBottom {
			c.armedBottom = true
			if c.OnReachBottom != nil {
				c.OnReachBottom()
			}
		}
	} else {
		c.pullBottom = 0
		c.armedBottom = false
	}
}
