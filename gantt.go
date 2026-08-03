// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
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
}

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

// OnEvent selects the task row under an EventClick and fires OnSelect (nil-safe)
// with its index. Clicks above the first row (in the header band) and clicks
// past the last task are no-ops, as is any non-click event.
func (g *Gantt) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
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
}
