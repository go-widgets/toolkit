// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"unicode/utf8"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// TextView / CodeEditor scrollbars.
//
// With [TextView.ShowScrollbars] set, the view draws the toolkit's shared
// scrollbar (paintScrollTrack / paintScrollThumb, the same house style as
// ScrollView, ListBox, Table …) inside its border: a vertical bar on the right
// when the buffer has more lines than fit, and a horizontal bar along the bottom
// of the TEXT area (not under the line-number gutter) when a line is wider than
// the text area. The bars are always shown while the content overflows — the
// "classic" always-visible model the sibling widgets use — and the area they take
// is subtracted from the text window, so a bar never covers text or the gutter.
//
// Geometry is one sbGeom per axis (the toolkit's single source of truth, see
// scrolldrag.go), shared by drawing and hit-testing, and driven by the same
// scroll observables the wheel and the keyboard move (ScrollLine, ScrollX), so
// the thumb follows every way of scrolling with no extra bookkeeping. Scrolling
// repaints but never re-lays-out text: the widest line is cached (colsMax) and
// invalidated only by an edit.

// textScrollEndPad is the logical space after the longest line, so the caret at
// the end of that line is reachable and not flush against the bar.
const textScrollEndPad = 8

// ScrollX is the horizontal scroll offset, in device pixels, as a bindable
// Observable. It is only meaningful (and only non-zero) while ShowScrollbars is
// true; reads clamp on the fly like ScrollLine's.
func (t *TextView) ScrollX() *mvvm.Observable[int] {
	if t.hscroll == nil {
		t.hscroll = mvvm.NewObservable(0)
	}
	return t.hscroll
}

// maxCols is the rune length of the widest line, cached until the next edit.
func (t *TextView) maxCols() int {
	if !t.colsOK {
		m := 0
		for _, l := range t.lines {
			if n := utf8.RuneCountInString(l); n > m {
				m = n
			}
		}
		t.colsMax, t.colsOK = m, true
	}
	return t.colsMax
}

// contentWidth is the pixel width of the widest line (the caret model is
// columns x glyphAdvance, as in Draw/caretAt) plus a little air.
func (t *TextView) contentWidth() int {
	return t.maxCols()*t.glyphAdvance() + scaled(textScrollEndPad)
}

// bars reports which scrollbars are shown. Showing one shrinks the window the
// other is measured against, so it settles in a few rounds. Always false, v and
// h, unless ShowScrollbars is set.
func (t *TextView) bars() (v, h bool) {
	if !t.ShowScrollbars {
		return false, false
	}
	r := t.Bounds()
	tr := scrollbarTrack()
	cw := t.contentWidth()
	for i := 0; i < 3; i++ {
		tw := r.W - t.textLeftInset() - 1 - t.minimapW()
		if v {
			tw -= tr
		}
		h = cw > tw
		avail := r.H
		if h {
			avail -= tr
		}
		v = len(t.lines) > t.linesFitIn(avail)
	}
	return v, h
}

// textAreaWidth is the width of the window the text scrolls in.
func (t *TextView) textAreaWidth() int {
	v, _ := t.bars()
	w := t.Bounds().W - t.textLeftInset() - 1 - t.minimapW()
	if v {
		w -= scrollbarTrack()
	}
	return w
}

// maxScrollX is the largest ScrollX that still leaves the longest line's end in
// view; 0 when no horizontal bar is shown.
func (t *TextView) maxScrollX() int {
	if _, h := t.bars(); !h {
		return 0
	}
	// A horizontal bar is only shown when the content is wider than the area, so
	// this is positive.
	return t.contentWidth() - t.textAreaWidth()
}

// clampedScrollX is ScrollX clamped to [0, maxScrollX()] without mutating it,
// the horizontal twin of clampedScrollLine; always 0 without scrollbars.
func (t *TextView) clampedScrollX() int {
	if !t.ShowScrollbars {
		return 0
	}
	s := t.ScrollX().Get()
	if s < 0 {
		s = 0
	}
	if m := t.maxScrollX(); s > m {
		s = m
	}
	return s
}

// scrollXBy shifts ScrollX by delta device pixels, clamped at both ends. A no-op
// without scrollbars.
func (t *TextView) scrollXBy(delta int) {
	if !t.ShowScrollbars {
		return
	}
	t.ScrollX().Set(t.clampedScrollX() + delta)
	t.ScrollX().Set(t.clampedScrollX())
}

// scrollCaretIntoViewX nudges ScrollX so the caret column stays inside the text
// area: left if it sits before the first visible pixel, right if its cell would
// end past the last. Resets to 0 once no horizontal bar is needed.
func (t *TextView) scrollCaretIntoViewX() {
	if !t.ShowScrollbars {
		return
	}
	if _, h := t.bars(); !h {
		t.ScrollX().Set(0)
		return
	}
	adv := t.glyphAdvance()
	cx := t.CursorCol().Get() * adv
	tw := t.textAreaWidth()
	sx := t.ScrollX().Get()
	if cx < sx {
		sx = cx
	} else if cx+adv > sx+tw {
		sx = cx + adv - tw
	}
	t.ScrollX().Set(sx)
	t.ScrollX().Set(t.clampedScrollX())
}

// vbarGeom is the vertical bar's geometry in widget-local pixels, ok false when
// no vertical bar is shown. The track runs the inner height (inside the 1px
// border), minus the horizontal bar's row when both show.
func (t *TextView) vbarGeom() (sbGeom, bool) {
	v, h := t.bars()
	if !v {
		return sbGeom{}, false
	}
	r := t.Bounds()
	tr := scrollbarTrack()
	trackLen := r.H - 2
	if h {
		trackLen -= tr
	}
	return scrollGeom(false, r.W-1-tr, tr, 1, trackLen,
		len(t.lines), t.visibleLines(), t.clampedScrollLine()), true
}

// hbarGeom is the horizontal bar's geometry, ok false when none is shown. Its
// track covers the TEXT area only, so the gutter keeps its whole height.
func (t *TextView) hbarGeom() (sbGeom, bool) {
	v, h := t.bars()
	if !h {
		return sbGeom{}, false
	}
	r := t.Bounds()
	tr := scrollbarTrack()
	trackLen := r.W - t.textLeftInset() - 1 - t.minimapW()
	if v {
		trackLen -= tr
	}
	return scrollGeom(true, r.H-1-tr, tr, t.textLeftInset(), trackLen,
		t.contentWidth(), t.textAreaWidth(), t.clampedScrollX()), true
}

// drawScrollbars paints whichever bars are shown, in the shared house style.
func (t *TextView) drawScrollbars(p painter.Painter, theme *Theme, v, h bool) {
	r := t.Bounds()
	if g, ok := t.vbarGeom(); v && ok {
		paintScrollTrack(p, theme, r.X+g.cross0, r.Y+g.trackStart, g.crossW, g.trackLen)
		paintScrollThumb(p, theme, r.X+g.cross0, r.Y+g.thumbStart, g.crossW, g.thumbLen)
	}
	if g, ok := t.hbarGeom(); h && ok {
		paintScrollTrack(p, theme, r.X+g.trackStart, r.Y+g.cross0, g.trackLen, g.crossW)
		paintScrollThumb(p, theme, r.X+g.thumbStart, r.Y+g.cross0, g.thumbLen, g.crossW)
	}
}

// barPress routes a press to the scrollbars: a thumb press starts a drag, a
// track press pages toward the click. True when it landed on a bar (the caller
// then must not place the caret).
func (t *TextView) barPress(ev Event) bool {
	if g, ok := t.vbarGeom(); t.vdrag.press(g, ok, ev, t.visibleLines(), t.scrollBy) {
		return true
	}
	g, ok := t.hbarGeom()
	return t.hdrag.press(g, ok, ev, t.textAreaWidth(), t.scrollXBy)
}

// barDrag continues a thumb drag, mapping the pointer to a scroll offset. True
// while a bar drag is active (the selection drag is then suppressed).
func (t *TextView) barDrag(ev Event) bool {
	if t.vdrag.active {
		g, ok := t.vbarGeom()
		t.vdrag.drag(g, ok, ev, func(s int) { t.ScrollLine().Set(s) })
		return true
	}
	if t.hdrag.active {
		g, ok := t.hbarGeom()
		t.hdrag.drag(g, ok, ev, func(s int) { t.ScrollX().Set(s) })
		return true
	}
	return false
}
