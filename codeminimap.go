// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/painter"
)

// CodeMinimap is a VS Code-style code overview: a scaled-down thumbnail of a
// source buffer where each line is a row of tiny coloured segments — one short
// segment per run of non-space characters, coloured by the syntax token under
// it (reusing the very spans an editor paints), with leading whitespace left as
// a blank gap so indentation reads. A translucent accent band marks the
// currently-visible line range. It is a scroll thumbnail: a click or drag maps
// back to a buffer line and the host scrolls its editor there.
//
// It is draw-only reactive-state-free chrome: the host refreshes
// lines/spans/top/visible before each paint with [CodeMinimap.Update] and reads
// the clicked line back through [CodeMinimap.OnScrollToLine] (or the
// [CodeMinimap.LineAt] accessor), so the widget owns no persistent editor state
// of its own — the buffer stays the editor's, and the minimap is a passive
// overview. That is why it carries no MVVM Observable: its inputs are per-paint
// snapshots, not settable widget state (see the mvvm_gate allowlist entry).
//
// Rows are a FIXED small height laid CONTIGUOUSLY from the top, exactly like the
// VS Code minimap: a short file sits as a compact block at the top with blank
// space below (it does NOT stretch to fill the column). Only when the buffer is
// taller than the widget (n*rowH > height) does it compress — sampling one line
// per drawn row — so the overview never spills past its bounds.
type CodeMinimap struct {
	Base

	// OnScrollToLine, when set, is invoked with the 0-based buffer line a click
	// or drag maps to, so the host scrolls its editor there. A func hook (not
	// reactive state), so it stays a plain field under the MVVM gate. Left nil,
	// a click/drag is a no-op — a host that reads [CodeMinimap.LineAt] itself
	// need not set it.
	OnScrollToLine func(line int)

	lines   []string     // reference to the editor buffer lines
	spans   [][]TextSpan // per-line syntax spans (may be shorter/nil)
	top     int          // first visible buffer line (editor scroll offset)
	visible int          // number of lines visible in the editor viewport
}

// NewCodeMinimap constructs an empty minimap. The zero value is also usable
// (all state is fed per-paint through Update), so this is a convenience for
// symmetry with the other widget constructors.
func NewCodeMinimap() *CodeMinimap { return &CodeMinimap{} }

// Update refreshes the overview inputs before a paint. lines is the editor
// buffer; spans is the per-line highlighter output (the editor's own colours,
// same shape a TextView.Highlighter produces) and may be nil or shorter than
// lines — uncovered rows fall back to a neutral ink. top is the first visible
// buffer line and visible the viewport's visible-line count, together placing
// the accent band. Passing the editor's live slices by reference is intended:
// the minimap only reads them during Draw.
func (m *CodeMinimap) Update(lines []string, spans [][]TextSpan, top, visible int) {
	m.lines, m.spans, m.top, m.visible = lines, spans, top, visible
}

// metrics returns the fixed row height and the number of rows actually drawn:
// one row per source line when the buffer fits, otherwise as many rows as fit
// the height (the buffer is then sampled, one line per row). displayRows is 0
// only when there is no area or no lines.
func (m *CodeMinimap) metrics() (rowH, displayRows int) {
	r := m.Bounds()
	n := len(m.lines)
	if r.H <= 0 || n == 0 {
		return 0, 0
	}
	rowH = m.rowH()
	maxRows := r.H / rowH
	if maxRows < 1 {
		maxRows = 1
	}
	displayRows = n
	if displayRows > maxRows {
		displayRows = maxRows
	}
	return rowH, displayRows
}

// lineForRow maps a drawn row index to its 0-based source line. It is the
// identity when the buffer fits (displayRows == n) and a proportional sample
// when it does not. Callers pass 0 <= row < displayRows <= n with displayRows >=
// 1, so row*n/displayRows is always in [0, n) — no clamp is needed.
func lineForRow(row, displayRows, n int) int {
	return row * n / displayRows
}

// rowForLine maps a 0-based source line to the drawn row it appears on — the
// inverse of lineForRow, used to place the viewport indicator over the rows.
func rowForLine(line, displayRows, n int) int {
	return line * displayRows / n
}

// LineAt maps a SURFACE-space y (device pixels, the same space as Bounds) to a
// 0-based buffer line, in the fixed-row-height geometry: a y above the drawn
// block clamps to line 0, a y in or below it clamps to the last drawn row's
// line. It is the pointer→line mapping a host uses to turn a click on the
// overview into an editor scroll target; OnEvent calls it for its own
// click/drag handling. Returns 0 when there is nothing drawn.
func (m *CodeMinimap) LineAt(y int) int {
	r := m.Bounds()
	n := len(m.lines)
	rowH, displayRows := m.metrics()
	if displayRows == 0 {
		return 0
	}
	rel := y - r.Y
	if rel < 0 {
		rel = 0
	}
	row := rel / rowH
	if row >= displayRows {
		row = displayRows - 1
	}
	return lineForRow(row, displayRows, n)
}

// charW is the device width of one source character cell in the overview, and
// rowH the device height of one line's row. Both are metric-scaled and floored
// at one device pixel so the overview never collapses to nothing.
func (m *CodeMinimap) charW() int { return atLeast1(scaled(1)) }
func (m *CodeMinimap) rowH() int  { return atLeast1(scaled(3)) }

// isSpace reports whether r is a horizontal-space rune (space or tab), which the
// overview renders as a gap so indentation and word structure survive.
func isSpace(r rune) bool { return r == ' ' || r == '\t' }

// spanColorAt returns the colour of the span covering rune index idx, or the
// fallback ink when no span does (a plain, unlexed run).
func spanColorAt(spans []TextSpan, idx int, fallback RGBA) RGBA {
	for _, s := range spans {
		if idx >= s.Start && idx < s.End {
			return s.Color
		}
	}
	return fallback
}

// forEachSegment walks the whole overview and calls cb with the device rectangle
// and colour of every token segment it would paint, so Draw (which paints) and
// segmentCount (which tallies) share one geometry. It is a no-op when the widget
// has no area or no lines.
func (m *CodeMinimap) forEachSegment(neutral RGBA, cb func(rect Rect, c RGBA)) {
	r := m.Bounds()
	n := len(m.lines)
	rowH, displayRows := m.metrics()
	if r.W <= 0 || displayRows == 0 {
		return
	}
	charW := m.charW()
	pad := scaled(2)
	// Both floored at one so a razor-thin or high-DPI minimap still draws at
	// least one column of segments instead of collapsing to nothing.
	usableW := atLeast1(r.W - 2*pad)
	maxCols := atLeast1(usableW / charW)
	segH := atLeast1(rowH - scaled(1)) // a 1px gap between rows
	for row := 0; row < displayRows; row++ {
		i := lineForRow(row, displayRows, n)
		y := r.Y + row*rowH

		runes := []rune(m.lines[i])
		cols := len(runes)
		if cols > maxCols {
			cols = maxCols
		}
		var spans []TextSpan
		if i < len(m.spans) {
			spans = m.spans[i]
		}
		j := 0
		for j < cols {
			if isSpace(runes[j]) {
				j++ // gap (leading indentation and inter-word spaces alike)
				continue
			}
			start := j
			col := spanColorAt(spans, j, neutral)
			for j < cols && !isSpace(runes[j]) && spanColorAt(spans, j, neutral) == col {
				j++
			}
			seg := Rect{X: r.X + pad + start*charW, Y: y, W: (j - start) * charW, H: segH}
			cb(seg, col)
		}
	}
}

// segmentCount is the number of token segments the overview paints for the
// current buffer — an introspection hook that proves the minimap draws
// multi-token rows (not one solid bar per line) without rendering a surface.
func (m *CodeMinimap) segmentCount(neutral RGBA) int {
	n := 0
	m.forEachSegment(neutral, func(Rect, RGBA) { n++ })
	return n
}

// Draw paints the SurfaceAlt panel, its left divider, every token segment, and
// the translucent viewport indicator band. Zero-area bounds and an empty buffer
// both short-circuit to (at most) the panel, so a minimap with no content never
// panics or paints a stray band.
func (m *CodeMinimap) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y, scaled(1), r.H, theme.Border) // left divider

	n := len(m.lines)
	if n == 0 {
		return
	}

	m.forEachSegment(theme.OnSurface, func(seg Rect, c RGBA) {
		fillRect(p, seg.X, seg.Y, seg.W, seg.H, c)
	})

	// Viewport indicator: a translucent accent band over the visible range,
	// mapped through the SAME fixed-row geometry so it lines up with the rows.
	if m.visible > 0 {
		rowH, displayRows := m.metrics()
		contentH := displayRows * rowH
		startRow := rowForLine(m.top, displayRows, n)
		end := m.top + m.visible
		if end > n {
			end = n
		}
		endRow := rowForLine(end, displayRows, n)
		vy := r.Y + startRow*rowH
		vh := (endRow - startRow) * rowH
		if vh < scaled(6) {
			vh = scaled(6)
		}
		if (vy-r.Y)+vh > contentH {
			vh = contentH - (vy - r.Y)
		}
		fillRect(p, r.X, vy, r.W, vh, RGBA{R: theme.Accent.R, G: theme.Accent.G, B: theme.Accent.B, A: 0x33})
		strokeRect(p, r.X, vy, r.W, vh, theme.Accent)
	}
}

// OnEvent maps a click or a press-drag on the overview to a buffer line and
// hands it to OnScrollToLine, so dragging up/down the minimap scrolls the
// editor. Event X/Y are widget-local, so it re-anchors y to surface space
// (Bounds().Y + ev.Y) for LineAt. Every other event kind is ignored, and a
// disabled minimap consumes nothing.
func (m *CodeMinimap) OnEvent(ev Event) {
	if m.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick, EventMouseDrag:
		if m.OnScrollToLine != nil {
			m.OnScrollToLine(m.LineAt(m.Bounds().Y + ev.Y))
		}
	}
}

// A11y reports the minimap as an img carrying its buffer line count — a
// scaled-down picture of the source, like the charts, with nothing a reader can
// read off it beyond how much code it overviews.
func (m *CodeMinimap) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(m.lines)) + " lines"}
}
