// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"

	"github.com/go-widgets/painter"
)

// GanttTask is one horizontal bar in a Gantt chart. Label names the task and
// is drawn in the left gutter; Start and End are integer time-unit columns on
// the shared axis (End must be greater than Start) so the bar spans the
// half-open range [Start, End). Fill is the bar colour — its zero value falls
// back to the theme's Accent so a task added without an explicit colour still
// paints in the app's palette. Progress in [0, 1] draws a darker overlay across
// that leading fraction of the bar, the usual "% complete" cue.
type GanttTask struct {
	Label      string
	Start, End int
	Fill       RGBA
	Progress   float64
}

// Gantt is a horizontal project-schedule chart: a left gutter of task Labels, a
// tick header naming the time-unit columns, and one row per task carrying a bar
// that spans its [Start, End) columns across the shared axis. Units is the total
// number of columns on that axis; when it is <= 0 it is derived from the largest
// task End so a caller can leave it unset. Progress paints a darker overlay on
// each bar, and Selected (when it indexes a task) tints that row and fires
// OnSelect on a click.
//
// Gantt renders through painter.Painter, so the same schedule draws as pixels
// (WUI/GUI) or promoted cells (TUI). An empty task slice draws just the gutter
// separator, header band and axis ticks.
type Gantt struct {
	Base
	Tasks    []GanttTask
	Units    int
	OnSelect func(i int)
	Selected int
	// OnTaskChange fires when a drag edits a task's span, with the task index
	// and its new [start, end) columns. Nil is safe -- Tasks is still mutated
	// in place, so the chart reflects the edit whether or not a host listens.
	OnTaskChange func(i, start, end int)

	// Edit/drag state, following the toolkit's grab-on-EventClick / track-on-
	// EventMouseDrag / release-on-EventMouseUp convention. editMode records
	// whether the grab moves the whole bar or resizes one edge; editGrab is
	// the pointer-unit-minus-Start offset captured at grab time (move only) so
	// the bar tracks the pointer without jumping.
	editing  bool
	editIdx  int
	editMode ganttDragMode
	editGrab int
}

// ganttDragMode is how an in-flight bar drag edits its task: shift the whole
// bar, or drag its left/right edge to change Start/End.
type ganttDragMode int

const (
	ganttNone ganttDragMode = iota
	ganttMove
	ganttResizeStart
	ganttResizeEnd
)

// ganttEdgeGrab is the pixel hit-slop around a bar's left/right edge within
// which a press starts an edge-resize rather than a whole-bar move.
const ganttEdgeGrab = 5

// Gantt sizing constants, exported like TableRowHeight / TableHeaderHeight so a
// host can measure a chart before it has a surface (rows*GanttRowH +
// GanttHeaderH gives the natural height; GanttLabelW is the fixed gutter width).
const (
	// GanttRowH is the pixel height of one task row.
	GanttRowH = 24
	// GanttHeaderH is the pixel height of the tick-header band.
	GanttHeaderH = 20
	// GanttLabelW is the pixel width of the left label gutter.
	GanttLabelW = 96
)

// ganttBarPadY is the vertical inset between a row's top/bottom edge and its
// bar, so successive bars have a thin gap and never touch the row separator.
const ganttBarPadY = 4

// NewGantt builds a Gantt over the given tasks with no selection (Selected =
// -1) and an auto-derived axis (Units = 0). A nil slice is normalised to a
// non-nil empty slice so range loops and len() checks never special-case nil.
func NewGantt(tasks []GanttTask) *Gantt {
	if tasks == nil {
		tasks = []GanttTask{}
	}
	return &Gantt{Tasks: tasks, Selected: -1}
}

// axisUnits returns the effective column count of the time axis: the explicit
// Units when positive, else the largest task End (min 1 so an empty or all-zero
// schedule still has a non-degenerate scale and never divides by zero).
func (g *Gantt) axisUnits() int {
	if g.Units > 0 {
		return g.Units
	}
	mx := 0
	for _, tk := range g.Tasks {
		if tk.End > mx {
			mx = tk.End
		}
	}
	if mx <= 0 {
		mx = 1
	}
	return mx
}

// ganttProgressInk is the overlay colour for a bar's completed fraction: the
// bar's own Fill darkened toward black (60% fill), so the progress portion
// reads as the same hue a shade deeper regardless of the task's colour.
func ganttProgressInk(fill RGBA) RGBA {
	return blendRGBA(fill, RGB(0, 0, 0), 0.6)
}

// ganttSelectInk is the row-tint colour for the Selected task: the theme's
// Accent blended lightly into the Surface so the whole row reads as highlighted
// without overpowering the bar drawn on top of it.
func ganttSelectInk(theme *Theme) RGBA {
	return blendRGBA(theme.Accent, theme.Surface, 0.25)
}

// Draw paints the surface, the label gutter + its separator, the tick header
// band with one rule per axis column, and one row per task: a selection tint
// (Selected only), the task Label in the gutter, and a bar spanning [Start, End)
// with its Progress overlay. The plotting area (everything right of the gutter)
// and the gutter itself are clipped so a long label or an over-long bar never
// bleeds across the boundary.
func (g *Gantt) Draw(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	axisX := r.X + GanttLabelW
	axisW := r.W - GanttLabelW
	units := g.axisUnits()
	colX := func(c int) int {
		return axisX + int(float64(c)/float64(units)*float64(axisW))
	}

	// Header band + gutter separator.
	fillRect(p, r.X, r.Y, r.W, GanttHeaderH, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y+GanttHeaderH-1, r.W, 1, theme.Border)
	fillRect(p, axisX, r.Y, 1, r.H, theme.Border)

	// Tick rules + column index labels, clipped to the axis area.
	axisRect := Rect{X: axisX, Y: r.Y, W: r.X + r.W - axisX, H: r.H}
	tickTop := r.Y + (GanttHeaderH-g.glyphHeight())/2
	withClip(p, axisRect, func() {
		for c := 0; c <= units; c++ {
			x := colX(c)
			fillRect(p, x, r.Y+GanttHeaderH, 1, r.H-GanttHeaderH, theme.Border)
			g.drawText(p, x+2, tickTop, strconv.Itoa(c), dimInk(theme))
		}
	})

	for i, tk := range g.Tasks {
		rowY := r.Y + GanttHeaderH + i*GanttRowH
		if g.Selected == i {
			fillRect(p, r.X, rowY, r.W, GanttRowH, ganttSelectInk(theme))
		}
		labelY := rowY + (GanttRowH-g.glyphHeight())/2
		withClip(p, Rect{X: r.X, Y: rowY, W: GanttLabelW, H: GanttRowH}, func() {
			g.drawText(p, r.X+TableCellPadX, labelY, tk.Label, theme.OnSurface)
		})

		fill := tk.Fill
		if fill == (RGBA{}) {
			fill = theme.Accent
		}
		barX := colX(tk.Start)
		barW := colX(tk.End) - barX
		if barW < 1 {
			barW = 1
		}
		barY := rowY + ganttBarPadY
		barH := GanttRowH - 2*ganttBarPadY
		withClip(p, axisRect, func() {
			fillRect(p, barX, barY, barW, barH, fill)
			if tk.Progress > 0 {
				frac := tk.Progress
				if frac > 1 {
					frac = 1
				}
				pw := int(float64(barW) * frac)
				fillRect(p, barX, barY, pw, barH, ganttProgressInk(fill))
			}
		})
	}
}

// barXLocal is column c's pixel x in widget-local coordinates (0 at the
// widget's left edge), the local-space twin of Draw's colX (which adds r.X).
// Used to place bars for hit-testing an edge grab.
func (g *Gantt) barXLocal(c int) int {
	axisW := g.Bounds().W - GanttLabelW
	return GanttLabelW + int(float64(c)/float64(g.axisUnits())*float64(axisW))
}

// unitAtLocal maps a widget-local x to the nearest axis column, clamped to
// [0, units]. A degenerate (<=0) axis width collapses to column 0.
func (g *Gantt) unitAtLocal(localX int) int {
	axisW := g.Bounds().W - GanttLabelW
	if axisW <= 0 {
		return 0
	}
	units := g.axisUnits()
	u := int(math.Round(float64(localX-GanttLabelW) / float64(axisW) * float64(units)))
	return clampInt(u, 0, units)
}

// OnEvent drives selection and bar editing. On EventClick it selects the task
// row (firing OnSelect) and, from where in the bar the press landed, arms a
// drag: near the left/right edge resizes Start/End, inside the bar moves the
// whole span, and elsewhere in the row is a plain select. EventMouseDrag
// applies the edit live; EventMouseUp commits it and fires OnTaskChange. All
// callbacks are nil-safe.
func (g *Gantt) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		if ev.Y < GanttHeaderH {
			return
		}
		row := (ev.Y - GanttHeaderH) / GanttRowH
		if row >= len(g.Tasks) {
			return
		}
		g.Selected = row
		if g.OnSelect != nil {
			g.OnSelect(row)
		}
		tk := g.Tasks[row]
		startX, endX := g.barXLocal(tk.Start), g.barXLocal(tk.End)
		switch {
		case ev.X >= startX-ganttEdgeGrab && ev.X <= startX+ganttEdgeGrab:
			g.editing, g.editIdx, g.editMode = true, row, ganttResizeStart
		case ev.X >= endX-ganttEdgeGrab && ev.X <= endX+ganttEdgeGrab:
			g.editing, g.editIdx, g.editMode = true, row, ganttResizeEnd
		case ev.X > startX && ev.X < endX:
			g.editing, g.editIdx, g.editMode = true, row, ganttMove
			g.editGrab = g.unitAtLocal(ev.X) - tk.Start
		default:
			g.editing = false
		}
	case EventMouseDrag:
		if g.editing {
			g.applyDrag(g.unitAtLocal(ev.X))
		}
	case EventMouseUp:
		if !g.editing {
			return
		}
		i := g.editIdx
		g.editing = false
		if g.OnTaskChange != nil {
			g.OnTaskChange(i, g.Tasks[i].Start, g.Tasks[i].End)
		}
	}
}

// applyDrag edits the task being dragged so the grabbed feature tracks pointer
// column u: a start/end edge is moved (keeping a minimum one-column span and
// staying within [0, units]), or the whole bar is shifted (preserving its span
// and clamping so neither edge leaves the axis).
func (g *Gantt) applyDrag(u int) {
	tk := &g.Tasks[g.editIdx]
	units := g.axisUnits()
	switch g.editMode {
	case ganttResizeStart:
		tk.Start = clampInt(u, 0, tk.End-1)
	case ganttResizeEnd:
		tk.End = clampInt(u, tk.Start+1, units)
	case ganttMove:
		span := tk.End - tk.Start
		ns := clampInt(u-g.editGrab, 0, units-span)
		tk.Start, tk.End = ns, ns+span
	}
}
