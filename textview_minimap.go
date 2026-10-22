// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TextView / CodeEditor minimap.
//
// With [TextView.ShowScrollbars] and [TextView.ShowMinimap] both set, a
// [CodeMinimap] — the scaled-down picture of the whole buffer the toolkit already
// had as a standalone widget — sits between the text and the vertical scrollbar,
// the arrangement of VS Code's and Sublime's. The visible range is the slider;
// pressing on the slider and dragging moves it with the pointer kept at the same
// spot on it, pressing anywhere else centres the viewport on that line, and the
// wheel over it scrolls as everywhere else. The overview is the "fit" kind of VS
// Code's minimap: one 1-px-per-character, 3-px-per-line picture that samples one
// line per row once the buffer is taller than the window (the standalone widget's
// documented behaviour), so its cost is bounded by the window height, never by the
// buffer length. Errors, warnings and search matches are ticks on its right edge.
//
// The minimap only exists when the text area stays wide enough to be worth
// reading (a window narrower than four minimap widths hides it), and it takes its
// own columns from the text window like the scrollbars do, so it never covers
// text, the gutter or a bar.

// minimapWidth is the minimap's logical width: 1 px per character, so about 80
// columns of code.
const minimapWidth = 90

// SetMinimapMarks replaces the line marks (errors, warnings …) the minimap shows,
// next to the search matches it shows by itself.
func (t *TextView) SetMinimapMarks(marks []MinimapMark) { t.mmMarks = marks }

// MinimapMarks returns the marks last set by SetMinimapMarks.
func (t *TextView) MinimapMarks() []MinimapMark { return t.mmMarks }

// minimapW is the minimap's width, 0 when it is off or the window is too narrow.
func (t *TextView) minimapW() int {
	if !t.ShowScrollbars || !t.ShowMinimap {
		return 0
	}
	w := scaled(minimapWidth)
	if t.Bounds().W < 4*w {
		return 0
	}
	return w
}

// minimapRect is the minimap's rectangle in widget-local pixels: the inner
// height, left of the vertical bar when one shows.
func (t *TextView) minimapRect() Rect {
	r := t.Bounds()
	v, _ := t.bars()
	w := t.minimapW()
	x := r.W - 1 - w
	if v {
		x -= scrollbarTrack()
	}
	return Rect{X: x, Y: 1, W: w, H: r.H - 2}
}

// syncMinimap points the embedded CodeMinimap at the current window and buffer
// (the minimap reads the editor's slices by reference; nothing is copied).
func (t *TextView) syncMinimap() *CodeMinimap {
	if t.minimap == nil {
		t.minimap = NewCodeMinimap()
	}
	r, rc := t.Bounds(), t.minimapRect()
	m := t.minimap
	m.SetBounds(Rect{X: r.X + rc.X, Y: r.Y + rc.Y, W: rc.W, H: rc.H})
	m.Update(t.lines, t.mmSpans, t.clampedScrollLine(), t.visibleLines())
	return m
}

// drawMinimap paints the overview with the marks: the caller's, plus one per
// search match.
func (t *TextView) drawMinimap(p painter.Painter, theme *Theme) {
	if t.minimapW() == 0 {
		return
	}
	m := t.syncMinimap()
	t.mmBuf = append(t.mmBuf[:0], t.mmMarks...)
	for _, b := range t.matchBands {
		t.mmBuf = append(t.mmBuf, MinimapMark{Line: b.sel.StartLine, Kind: MarkMatch})
	}
	m.Marks = t.mmBuf
	m.Draw(p, theme)
}

// minimapPress handles a press on the minimap. True when it landed on it.
func (t *TextView) minimapPress(ev Event) bool {
	if t.minimapW() == 0 {
		return false
	}
	rc := t.minimapRect()
	if ev.X < rc.X || ev.X >= rc.X+rc.W || ev.Y < rc.Y || ev.Y >= rc.Y+rc.H {
		return false
	}
	m := t.syncMinimap()
	line := m.LineAt(t.Bounds().Y + ev.Y)
	top, vis := t.clampedScrollLine(), t.visibleLines()
	if line >= top && line < top+vis {
		t.mmGrab = line - top // on the slider: keep the pointer where it grabbed it
	} else {
		t.mmGrab = vis / 2 // elsewhere: centre the window on the line
	}
	t.mmDrag = true
	t.minimapScrollTo(line)
	return true
}

// minimapDrag moves the slider with the pointer. True while a drag is active.
func (t *TextView) minimapDrag(ev Event) bool {
	if !t.mmDrag {
		return false
	}
	t.minimapScrollTo(t.syncMinimap().LineAt(t.Bounds().Y + ev.Y))
	return true
}

// minimapScrollTo scrolls so that line sits mmGrab rows below the window top.
func (t *TextView) minimapScrollTo(line int) {
	t.ScrollLine().Set(line - t.mmGrab)
	t.ScrollLine().Set(t.clampedScrollLine())
}
