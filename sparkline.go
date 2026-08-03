// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SparkKind selects a Sparkline's visual form.
type SparkKind int

const (
	// SparkLine (the default) draws the series as a polyline.
	SparkLine SparkKind = iota
	// SparkBar draws the series as a row of thin vertical bars.
	SparkBar
)

// Sparkline is a tiny, axis-less inline trend chart -- the kind embedded in a
// KPI card, a table cell or a status row to show a series' shape at a glance.
// Unlike LineChart / BarChart it draws no axes, labels or gridlines: the whole
// bounds is plot area, and the Values are auto-scaled to fit it. Display-only.
//
// It renders through painter.Painter, so the same spark draws as anti-aliased
// pixels (WUI/GUI) or promoted cells (TUI). An empty series draws nothing; a
// single value renders as a dot.
type Sparkline struct {
	Base
	// Values is the data series, plotted left-to-right and auto-scaled between
	// its own min and max across the bounds height.
	Values []float64
	// Kind selects the form: SparkLine (polyline, default) or SparkBar.
	Kind SparkKind
	// Fill is the ink for the line/bars. The zero value (A == 0) inherits the
	// theme's Accent colour.
	Fill RGBA
	// ShowLast emphasises the final data point: a small dot on a SparkLine, a
	// brighter final bar on a SparkBar.
	ShowLast bool
}

// SparkPad is the inset (painter units) reserved on every edge so the line's
// endpoints and the tallest bar never bleed against the widget's border.
const SparkPad = 1

// NewSparkline builds a SparkLine over the given values.
func NewSparkline(values []float64) *Sparkline { return &Sparkline{Values: values} }

// HitTest returns false unconditionally: a Sparkline is decorative, like Label.
func (s *Sparkline) HitTest(_, _ int) bool { return false }

// ink is the effective line/bar colour: Fill when set, else theme.Accent.
func (s *Sparkline) ink(theme *Theme) RGBA {
	if s.Fill.A != 0 {
		return s.Fill
	}
	return theme.Accent
}

// plot is the drawable rectangle: the bounds inset by SparkPad on each edge.
func (s *Sparkline) plot() Rect {
	r := s.Bounds()
	return Rect{X: r.X + SparkPad, Y: r.Y + SparkPad, W: r.W - 2*SparkPad, H: r.H - 2*SparkPad}
}

// valueRange returns the (min, max) of Values; only called for a non-empty
// series.
func (s *Sparkline) valueRange() (float64, float64) {
	mn, mx := s.Values[0], s.Values[0]
	for _, v := range s.Values[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

// sparkFrac maps v into [0,1] across the range [mn,mx]. A flat series (mx <= mn)
// yields 0.5 so it sits centred rather than dividing by zero.
func sparkFrac(v, mn, mx float64) float64 {
	span := mx - mn
	if span <= 0 {
		return 0.5
	}
	return (v - mn) / span
}

// brighter blends c 40% toward white, used for the emphasised final bar.
func brighter(c RGBA) RGBA { return blendRGBA(RGB(0xFF, 0xFF, 0xFF), c, 0.4) }

// pointAt maps series index i to a pixel in the plot area pl.
func (s *Sparkline) pointAt(i int, mn, mx float64, pl Rect) (int, int) {
	x := pl.X
	if n := len(s.Values); n > 1 {
		x = pl.X + i*(pl.W-1)/(n-1)
	}
	f := sparkFrac(s.Values[i], mn, mx)
	y := pl.Y + int((1-f)*float64(pl.H-1))
	return x, y
}

// Draw paints the spark: nothing for an empty series or a sub-pixel bounds, a
// dot for a lone value, otherwise a polyline (SparkLine) or bar row (SparkBar).
func (s *Sparkline) Draw(p painter.Painter, theme *Theme) {
	pl := s.plot()
	if pl.W <= 0 || pl.H <= 0 {
		return
	}
	n := len(s.Values)
	if n == 0 {
		return
	}
	col := s.ink(theme)
	mn, mx := s.valueRange()
	if s.Kind == SparkBar {
		s.drawBars(p, pl, mn, mx, col)
		return
	}
	if n == 1 {
		x, y := s.pointAt(0, mn, mx, pl)
		fillRect(p, x, y, 2, 2, col)
		return
	}
	px, py := s.pointAt(0, mn, mx, pl)
	for i := 1; i < n; i++ {
		x, y := s.pointAt(i, mn, mx, pl)
		drawLine(p, px, py, x, y, col)
		px, py = x, y
	}
	if s.ShowLast {
		fillRect(p, px-1, py-1, 2, 2, col)
	}
}

// drawBars renders the series as thin vertical bars filling the plot width, the
// final bar brightened when ShowLast.
func (s *Sparkline) drawBars(p painter.Painter, pl Rect, mn, mx float64, col RGBA) {
	n := len(s.Values)
	slot := pl.W / n
	if slot < 1 {
		slot = 1
	}
	bw := slot - 1
	if bw < 1 {
		bw = 1
	}
	for i, v := range s.Values {
		bh := int(sparkFrac(v, mn, mx) * float64(pl.H-1))
		if bh < 1 {
			bh = 1
		}
		c := col
		if s.ShowLast && i == n-1 {
			c = brighter(col)
		}
		bx := pl.X + i*slot
		fillRect(p, bx, pl.Y+pl.H-bh, bw, bh, c)
	}
}
