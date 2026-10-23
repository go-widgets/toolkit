// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Splitter lays N panes side by side (or one above the other) with a draggable,
// focusable divider between each neighbouring pair, after the WAI-ARIA APG
// "window splitter" pattern (https://www.w3.org/WAI/ARIA/apg/patterns/windowsplitter/).
//
// A pane is either FIXED (its Size, in logical pixels, is a reactive
// [mvvm.Observable] the divider writes) or FLEXIBLE (it shares what the fixed
// panes leave by its Weight, also an Observable): the Files column of an IDE is
// fixed, the editor and the preview beside it are flexible. Each pane has a Min
// and a Max; a Collapsible fixed pane drags down to nothing (Size 0) when the
// divider is pulled below half its Min, and comes back with a double-click, Enter
// or a drag out. A pane whose Visible Observable is false takes no room and loses
// its divider, so a View menu can switch panes off without the layout knowing.
//
// Moving a divider by d moves the pane before it by +d and the pane after it by
// -d, each within its bounds, the other panes keeping their size: the divider
// stays under the pointer until a bound stops it. Sizes live in the Observables, so
// a view-model can bind them, persist them and restore them; OnResized tells it
// what the PERSON changed (a drag, a key, a double-click), never a layout that
// followed a window resize.
//
// Keyboard (a divider holds the focus, a ring shows it; Tab moves between the
// dividers and the panes): Left/Right (Up/Down when Vertical) move it by Step
// logical pixels, four times that with Shift; Home and End take the pane before it
// to its smallest and largest; Enter collapses the fixed pane it controls and
// brings it back. The accessibility description of a divider is a WAI-ARIA
// "separator" with aria-valuemin/max/now (RoleSeparator).
type Splitter struct {
	Base
	// Vertical stacks the panes top to bottom (dividers are horizontal bars)
	// instead of left to right.
	Vertical bool
	// Thickness is the divider's thickness and Step the keyboard step, in logical
	// pixels (SplitterHandleW and SplitterStep when 0).
	Thickness, Step int
	// OnResized is called once for each gesture the person finished (a drag's
	// release, a key press, a double-click) with the indexes of the panes whose
	// size or weight it changed.
	OnResized func(changed []int)
	// Now is the clock the double-click detection reads (time.Now when nil).
	Now func() time.Time

	panes   []*SplitPane
	handles []*dividerHandle
	vis     []int // the indexes of the panes that are on, in order
	px      []int // each pane's laid-out size along the axis, device pixels

	batch    bool // sizes are being written: layout once, at the end
	grab     int  // the divider being dragged, or -1
	grabAt   int  // the pointer's place along the axis when it was grabbed
	grabBase []int
	hover    int // the divider under the pointer, or -1
	lastAt   time.Time
	lastDiv  int
	dragging *mvvm.Observable[bool]
}

// SplitPane is one pane of a [Splitter].
type SplitPane struct {
	// Widget is what the pane shows.
	Widget Widget
	// Name is the pane's accessible name: its divider is "Resize <Name>".
	Name string
	// Size, in logical pixels, makes the pane fixed; Weight makes it flexible.
	// Exactly one of the two is set.
	Size, Weight *mvvm.Observable[int]
	// Min and Max bound the pane, in logical pixels (Max 0: no upper bound).
	Min, Max int
	// Collapsible lets a fixed pane go down to Size 0.
	Collapsible bool
	// Visible, when set, switches the pane (and a divider beside it) off.
	Visible *mvvm.Observable[bool]

	restore int // the logical size before the pane was collapsed
}

// Splitter metrics, in logical pixels.
const (
	SplitterHandleW = 10
	SplitterStep    = 16
)

// splitterDoubleClick is how soon a second press on a divider counts as a
// double-click.
const splitterDoubleClick = 450 * time.Millisecond

// RoleSeparator is the role of a divider the person can move: a focusable
// WAI-ARIA "separator" with a value.
const RoleSeparator Role = "separator"

// NewSplitter builds a splitter over panes. Weights below 1 count as 1.
func NewSplitter(vertical bool, panes ...*SplitPane) *Splitter {
	s := &Splitter{Vertical: vertical, panes: panes, grab: -1, hover: -1, lastDiv: -1,
		dragging: mvvm.NewObservable(false), px: make([]int, len(panes))}
	for j := 0; j+1 < len(panes); j++ {
		s.handles = append(s.handles, &dividerHandle{s: s, j: j, a: -1, b: -1})
	}
	relayout := func() {
		if !s.batch {
			s.layout()
		}
	}
	for _, p := range panes {
		if p.Size != nil {
			p.Size.Subscribe(func(int) { relayout() })
		}
		if p.Weight != nil {
			p.Weight.Subscribe(func(int) { relayout() })
		}
		if p.Visible != nil {
			p.Visible.Subscribe(func(bool) { relayout() })
		}
	}
	return s
}

// Dragging is true while a divider is held.
func (s *Splitter) Dragging() *mvvm.Observable[bool] { return s.dragging }

func (s *Splitter) handleW() int {
	if s.Thickness > 0 {
		return scaled(s.Thickness)
	}
	return scaled(SplitterHandleW)
}

func (s *Splitter) step() int {
	if s.Step > 0 {
		return scaled(s.Step)
	}
	return scaled(SplitterStep)
}

func (s *Splitter) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// SetBounds places the splitter and lays its panes out.
func (s *Splitter) SetBounds(r Rect) {
	s.Base.SetBounds(r)
	s.layout()
}

// along is a rectangle's start and extent along the splitter's axis.
func (s *Splitter) along(r Rect) (pos, size int) {
	if s.Vertical {
		return r.Y, r.H
	}
	return r.X, r.W
}

// rect builds a rectangle from a place and a size along the axis, the cross
// extent being the splitter's.
func (s *Splitter) rect(pos, size int) Rect {
	r := s.Bounds()
	if s.Vertical {
		return Rect{X: r.X, Y: pos, W: r.W, H: size}
	}
	return Rect{X: pos, Y: r.Y, W: size, H: r.H}
}

func (s *Splitter) on(i int) bool {
	p := s.panes[i]
	return p.Visible == nil || p.Visible.Get()
}

const unbounded = 1 << 30

// bounds are a pane's lowest and highest size, device pixels. Both are kept
// around cur so that a pane squeezed out of its range by a small window is not
// pushed back into it by the first drag.
func (s *Splitter) bounds(i, cur int) (lo, hi int) {
	p := s.panes[i]
	lo, hi = scaled(p.Min), unbounded
	if p.Max > 0 {
		hi = scaled(p.Max)
	}
	return min(lo, cur), max(hi, cur)
}

// collapsed reports whether pane i is a collapsible fixed pane at size 0.
func (s *Splitter) collapsed(i int) bool {
	p := s.panes[i]
	return p.Collapsible && p.Size != nil && p.Size.Get() <= 0
}

// Collapsed reports whether pane i has been pulled down to nothing.
func (s *Splitter) Collapsed(i int) bool { return s.collapsed(i) }

// fixedPx is a fixed pane's size, device pixels, within its bounds.
func (s *Splitter) fixedPx(i int) int {
	p := s.panes[i]
	if s.collapsed(i) {
		return 0
	}
	lo, hi := scaled(p.Min), unbounded
	if p.Max > 0 {
		hi = scaled(p.Max)
	}
	return min(max(scaled(p.Size.Get()), lo), hi)
}

// layout assigns the panes and the dividers their rectangles.
func (s *Splitter) layout() {
	s.vis = s.vis[:0]
	for i := range s.panes {
		if s.on(i) {
			s.vis = append(s.vis, i)
		} else {
			s.px[i] = 0
			s.panes[i].Widget.SetBounds(Rect{})
		}
	}
	for _, h := range s.handles {
		h.SetBounds(Rect{})
		h.a, h.b = -1, -1
	}
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 || len(s.vis) == 0 {
		for _, i := range s.vis {
			s.px[i] = 0
			s.panes[i].Widget.SetBounds(Rect{})
		}
		return
	}
	start, total := s.along(r)
	hw := s.handleW()
	avail := max(total-hw*(len(s.vis)-1), 0)
	sizes := s.solve(avail)
	pos := start
	for j, i := range s.vis {
		s.px[i] = sizes[j]
		s.panes[i].Widget.SetBounds(s.rect(pos, sizes[j]))
		pos += sizes[j]
		if j+1 < len(s.vis) {
			h := s.handles[j]
			h.a, h.b = i, s.vis[j+1]
			h.SetBounds(s.rect(pos, hw))
			pos += hw
		}
	}
}

// solve shares avail pixels among the visible panes: the fixed ones as asked,
// squeezed from the last toward their Min when the flexible ones would not get
// theirs, then the flexible ones by weight, none under its Min.
func (s *Splitter) solve(avail int) []int {
	n := len(s.vis)
	sizes := make([]int, n)
	var flex []int
	fixedSum, flexMin, weights := 0, 0, 0
	for j, i := range s.vis {
		p := s.panes[i]
		if p.Size != nil {
			sizes[j] = s.fixedPx(i)
			fixedSum += sizes[j]
			continue
		}
		flex = append(flex, j)
		flexMin += scaled(p.Min)
		weights += max(p.Weight.Get(), 1)
	}
	room := avail - fixedSum
	if room < flexMin { // squeeze the fixed panes, the last first
		need := flexMin - room
		for j := n - 1; j >= 0 && need > 0; j-- {
			i := s.vis[j]
			if s.panes[i].Size == nil || sizes[j] == 0 {
				continue
			}
			take := min(max(sizes[j]-scaled(s.panes[i].Min), 0), need)
			sizes[j] -= take
			need -= take
			fixedSum -= take
		}
		room = avail - fixedSum
	}
	// a window too small even for the fixed panes at their Min: they give way
	// further, the last first, rather than overrun it
	for j := n - 1; j >= 0 && fixedSum > avail; j-- {
		if s.panes[s.vis[j]].Size != nil {
			take := min(sizes[j], fixedSum-avail)
			sizes[j] -= take
			fixedSum -= take
		}
	}
	room = max(avail-fixedSum, 0)
	if len(flex) == 0 {
		return sizes
	}
	pinned := make([]bool, n)
	if room >= flexMin {
		for pass := 0; pass <= len(flex); pass++ {
			changed := false
			rest, w := room, weights
			for _, j := range flex {
				if pinned[j] {
					rest -= sizes[j]
					w -= max(s.panes[s.vis[j]].Weight.Get(), 1)
				}
			}
			for _, j := range flex {
				if pinned[j] {
					continue
				}
				p := s.panes[s.vis[j]]
				if rest*max(p.Weight.Get(), 1)/max(w, 1) < scaled(p.Min) {
					pinned[j], sizes[j], changed = true, scaled(p.Min), true
				}
			}
			if !changed {
				break
			}
		}
	}
	rest, w := room, 0
	for _, j := range flex {
		if pinned[j] {
			rest -= sizes[j]
		} else {
			w += max(s.panes[s.vis[j]].Weight.Get(), 1)
		}
	}
	rest = max(rest, 0)
	given, acc := 0, 0
	last := -1
	for _, j := range flex {
		if !pinned[j] {
			last = j
		}
	}
	for _, j := range flex {
		if pinned[j] {
			continue
		}
		acc += max(s.panes[s.vis[j]].Weight.Get(), 1)
		share := (rest*acc*2+w)/(2*max(w, 1)) - given
		if j == last {
			share = rest - given
		}
		sizes[j] = share
		given += share
	}
	return sizes
}

// clamp is the size pane i takes when asked for want pixels: within its
// bounds, or nothing when it is collapsible and asked for under half its Min.
func (s *Splitter) clamp(i, want, cur int) int {
	lo, hi := s.bounds(i, cur)
	if s.panes[i].Collapsible && want < lo/2 {
		return 0
	}
	return min(max(want, lo), hi)
}

// move shifts the divider after visible pane a (before pane b) by d device
// pixels from the sizes in base, and writes the result into the Observables.
func (s *Splitter) move(a, b int, base []int, d int) {
	na := s.clamp(a, base[a]+d, base[a])
	d = na - base[a]
	nb := s.clamp(b, base[b]-d, base[b])
	d = base[b] - nb
	na = s.clamp(a, base[a]+d, base[a])
	nb = base[b] - (na - base[a])
	px := append([]int(nil), base...)
	px[a], px[b] = na, nb
	s.write(px, base)
}

// write sets the Observables so that the panes come out at px (device pixels,
// one per pane); the fixed ones get their size, the flexible ones the weights
// that give them their share of the room the fixed ones leave. base is what the
// gesture started from: a pane that goes down to nothing remembers that size.
func (s *Splitter) write(px, base []int) {
	s.batch = true
	defer func() {
		s.batch = false
		s.layout()
	}()
	scale := MetricScale()
	flexSum, weights := 0, 0
	for _, i := range s.vis {
		p := s.panes[i]
		if p.Size != nil {
			if px[i] == s.px[i] {
				continue
			}
			if px[i] == 0 && base[i] > 0 {
				p.restore = int(float64(base[i])/scale + 0.5)
			}
			if px[i] == 0 {
				p.Size.Set(0)
			} else {
				p.Size.Set(int(float64(px[i])/scale + 0.5))
			}
			continue
		}
		flexSum += px[i]
		weights += max(p.Weight.Get(), 1)
	}
	if flexSum <= 0 {
		return
	}
	// weights fine enough to hold a hundredth of a pixel; every flexible pane
	// takes its share of the new total, those that did not move included
	k := max(weights, 100000)
	for _, i := range s.vis {
		if p := s.panes[i]; p.Weight != nil {
			p.Weight.Set(max((px[i]*k+flexSum/2)/flexSum, 1))
		}
	}
}

// changedSince lists the panes whose laid-out size is not what it was in before.
func (s *Splitter) changedSince(before []int) []int {
	var out []int
	for i, v := range s.px {
		if v != before[i] {
			out = append(out, i)
		}
	}
	return out
}

func (s *Splitter) fire(before []int) {
	if ch := s.changedSince(before); len(ch) > 0 && s.OnResized != nil {
		s.OnResized(ch)
	}
}

// --- the dividers ---------------------------------------------------------

// dividerHandle is a divider: a focusable widget the splitter places between two
// panes. It is not a child of the splitter (a drag that starts on it belongs to the
// splitter, which keeps its place), but the focus system reaches it.
type dividerHandle struct {
	Base
	focusState
	s    *Splitter
	j    int
	a, b int // the panes before and after it; -1 while it is off
}

// OnEvent takes the keys of a focused divider.
func (h *dividerHandle) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown && h.a >= 0 {
		h.s.key(h, ev)
	}
}

// Draw paints the divider: a line at 3:1 against the ground, a grip, and a ring
// when it holds the focus; the accent while it is pointed at or held.
func (h *dividerHandle) Draw(p painter.Painter, theme *Theme) {
	r := h.Bounds()
	if h.a < 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	ink, active := SplitterColors(theme)
	held := h.s.grab == h.j
	hot := held || h.s.hover == h.j
	vertical := !h.s.Vertical // a bar standing between panes side by side
	col, width := ink, scaled(1)
	if hot {
		col, width = active, scaled(3)
		fillRect(p, r.X, r.Y, r.W, r.H, blendRGBA(theme.Background, active, 0.14))
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	long, short := min(scaled(32), max(r.H, r.W)/2), scaled(4)
	if vertical {
		fillRect(p, cx-width/2, r.Y, width, r.H, col)
		fillRect(p, cx-short/2, cy-long/2, short, long, col)
	} else {
		fillRect(p, r.X, cy-width/2, r.W, width, col)
		fillRect(p, cx-long/2, cy-short/2, long, short, col)
	}
	if h.focused {
		strokeRect(p, r.X, r.Y, r.W, r.H, active)
		strokeRect(p, r.X+1, r.Y+1, r.W-2, r.H-2, active)
	}
}

// A11y describes the divider as a WAI-ARIA separator: aria-valuenow is the size
// of the pane before it, aria-valuemin and aria-valuemax the bounds of that
// pane, all in logical pixels.
func (h *dividerHandle) A11y() A11yInfo {
	if h.a < 0 {
		return A11yInfo{Role: RoleSeparator}
	}
	p := h.s.panes[h.a]
	lo, hi := p.Min, p.Max
	if p.Collapsible {
		lo = 0
	}
	if hi <= 0 {
		_, total := h.s.along(h.s.Bounds())
		hi = int(float64(total) / MetricScale())
	}
	return A11yInfo{Role: RoleSeparator, Name: "Resize " + p.Name, HasRange: true,
		Min: float64(lo), Max: float64(hi), Now: float64(h.s.px[h.a]) / MetricScale()}
}

// handleAt is the divider at surface point (x, y), or -1.
func (s *Splitter) handleAt(x, y int) int {
	for j, h := range s.handles {
		if h.a >= 0 && h.HitTest(x, y) {
			return j
		}
	}
	return -1
}

// HandleAt is the divider at surface point (x, y), or -1: a host that keeps the
// keyboard itself clears the dividers' focus (Blur) on a press anywhere else.
func (s *Splitter) HandleAt(x, y int) int { return s.handleAt(x, y) }

// HandleRect is divider j's rectangle (surface coordinates); ok is false while
// it is off (fewer than j+2 panes are on).
func (s *Splitter) HandleRect(j int) (Rect, bool) {
	if j < 0 || j >= len(s.handles) || s.handles[j].a < 0 {
		return Rect{}, false
	}
	return s.handles[j].Bounds(), true
}

// Handles is the number of dividers there are while every pane is on.
func (s *Splitter) Handles() int { return len(s.handles) }

// Handle returns divider j as a widget (focusable, with an accessibility
// description), for a host that tracks the focus itself.
func (s *Splitter) Handle(j int) Widget { return s.handles[j] }

// PaneSize is pane i's laid-out size along the axis, device pixels.
func (s *Splitter) PaneSize(i int) int { return s.px[i] }

// Cursor is the CSS cursor the pointer at (x, y), local to the splitter, should
// show: "col-resize" or "row-resize" over a divider or while one is held, else
// "" (the host's default).
func (s *Splitter) Cursor(x, y int) string {
	b := s.Bounds()
	if s.grab < 0 && s.handleAt(x+b.X, y+b.Y) < 0 {
		return ""
	}
	if s.Vertical {
		return "row-resize"
	}
	return "col-resize"
}

// FocusHandle gives divider j the keyboard (false while it is off).
func (s *Splitter) FocusHandle(j int) bool {
	if j < 0 || j >= len(s.handles) || s.handles[j].a < 0 {
		return false
	}
	for k, h := range s.handles {
		h.focused = k == j
	}
	return true
}

// FocusedHandle is the divider that holds the keyboard, or -1.
func (s *Splitter) FocusedHandle() int {
	for j, h := range s.handles {
		if h.focused && h.a >= 0 {
			return j
		}
	}
	return -1
}

// Blur takes the keyboard off every divider.
func (s *Splitter) Blur() {
	for _, h := range s.handles {
		h.focused = false
	}
}

// --- events ----------------------------------------------------------------

func (s *Splitter) focusableChildren() []Widget {
	var out []Widget
	for j, i := range s.vis {
		out = append(out, s.panes[i].Widget)
		if j+1 < len(s.vis) {
			out = append(out, s.handles[j])
		}
	}
	return out
}

// Children are the panes that are on, for the walks over the tree (drag
// capture finds the splitter itself on a divider).
func (s *Splitter) Children() []Widget {
	var out []Widget
	for _, i := range s.vis {
		out = append(out, s.panes[i].Widget)
	}
	return out
}

// Draw paints the panes that are on, then the dividers.
func (s *Splitter) Draw(p painter.Painter, theme *Theme) {
	for _, i := range s.vis {
		if b := s.panes[i].Widget.Bounds(); b.W > 0 && b.H > 0 {
			s.panes[i].Widget.Draw(p, theme)
		}
	}
	for _, h := range s.handles {
		h.Draw(p, theme)
	}
}

// OnEvent routes a press on a divider to the splitter's own drag, a key to the
// divider that holds the focus, and the rest to the pane under the pointer.
func (s *Splitter) OnEvent(ev Event) {
	if routeFocusKey(s, ev) {
		return
	}
	pr := s.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	main := ev.X
	if s.Vertical {
		main = ev.Y
	}
	switch ev.Kind {
	case EventMouseMove:
		s.hover = s.handleAt(sx, sy)
		for _, i := range s.vis {
			w := s.panes[i].Widget
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
		}
		return
	case EventMouseDrag:
		if s.grab >= 0 {
			h := s.handles[s.grab]
			if h.a >= 0 {
				s.move(h.a, h.b, s.grabBase, main-s.grabAt)
			}
			return
		}
	case EventMouseUp:
		if s.grab >= 0 {
			before := s.grabBase
			s.grab = -1
			s.dragging.Set(false)
			s.fire(before)
			return
		}
	case EventClick:
		focusClick(s, sx, sy)
		if j := s.handleAt(sx, sy); j >= 0 {
			s.press(j, main, ev.Code == "double")
			return
		}
	}
	for _, i := range s.vis {
		w := s.panes[i].Widget
		if w.HitTest(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
}

// press is a press on divider j at place main along the axis: a second one
// soon after toggles the collapse of the pane it controls, else a drag begins.
func (s *Splitter) press(j, main int, double bool) {
	now := s.now()
	if double || (s.lastDiv == j && now.Sub(s.lastAt) < splitterDoubleClick) {
		s.lastDiv = -1
		before := append([]int(nil), s.px...)
		s.toggle(j)
		s.fire(before)
		return
	}
	s.lastDiv, s.lastAt = j, now
	s.grab, s.grabAt = j, main
	s.grabBase = append([]int(nil), s.px...)
	s.dragging.Set(true)
}

// control is the pane divider j's collapse acts on: the fixed collapsible one
// before it, else after it; -1 if neither is.
func (s *Splitter) control(h *dividerHandle) int {
	for _, i := range []int{h.a, h.b} {
		if p := s.panes[i]; p.Collapsible && p.Size != nil {
			return i
		}
	}
	return -1
}

// toggle collapses the pane divider j controls, or brings it back to the size
// it had (its Min if it never had one).
func (s *Splitter) toggle(j int) {
	h := s.handles[j]
	c := s.control(h)
	if c < 0 {
		return
	}
	p := s.panes[c]
	target := 0
	if s.collapsed(c) {
		back := p.restore
		if back <= 0 {
			back = max(p.Min, 1)
		}
		hi := unbounded
		if p.Max > 0 {
			hi = scaled(p.Max)
		}
		target = min(max(scaled(back), scaled(p.Min)), hi)
	}
	d := target - s.px[c]
	if c == h.b {
		d = -d
	}
	s.move(h.a, h.b, append([]int(nil), s.px...), d)
}

// key moves the divider that holds the focus.
func (s *Splitter) key(h *dividerHandle, ev Event) {
	before := append([]int(nil), s.px...)
	step := s.step()
	if ev.Shift {
		step *= 4
	}
	less, more := "ArrowLeft", "ArrowRight"
	if s.Vertical {
		less, more = "ArrowUp", "ArrowDown"
	}
	switch ev.Code {
	case less:
		s.move(h.a, h.b, before, -step)
	case more:
		s.move(h.a, h.b, before, step)
	case "Home":
		s.move(h.a, h.b, before, -unbounded)
	case "End":
		s.move(h.a, h.b, before, unbounded)
	case "Enter":
		s.toggle(h.j)
	default:
		return
	}
	s.fire(before)
}

// --- colours ---------------------------------------------------------------

func linear(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.03928 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

func luminance(c RGBA) float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// ContrastRatio is the WCAG 2 contrast ratio of two colours, 1 to 21.
func ContrastRatio(a, b RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// legible moves c toward the ink until it has at least 3:1 (the WCAG 2.1
// non-text minimum) against every ground.
func legible(c, ink RGBA, grounds ...RGBA) RGBA {
	for t := 0.0; t <= 1.0; t += 0.05 {
		x := blendRGBA(c, ink, t)
		ok := true
		for _, g := range grounds {
			ok = ok && ContrastRatio(x, g) >= 3
		}
		if ok {
			return x
		}
	}
	return ink
}

// SplitterColors are the colours a divider paints with in a theme: its line at
// rest, and the accent for the pointer, the drag and the focus ring, each at
// least 3:1 against the theme's Background and Surface.
func SplitterColors(theme *Theme) (ink, active RGBA) {
	return legible(theme.Border, theme.OnBackground, theme.Background, theme.Surface),
		legible(theme.Accent, theme.OnBackground, theme.Background, theme.Surface)
}

// A11y describes the splitter itself as a plain container; its dividers are
// reached through Handle.
func (s *Splitter) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }
