// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"math"
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// PlotStyle is how an [XYPlot] draws one series.
type PlotStyle int

const (
	// PlotLine joins consecutive points with a polyline.
	PlotLine PlotStyle = iota
	// PlotStem draws a vertical stem from y = 0 to each point, the way a
	// spectrum's bins are usually shown (matplotlib's stem).
	PlotStem
	// PlotMarkers draws a small square at each point and nothing between.
	PlotMarkers
)

// PlotSeries is one curve of an [XYPlot]: Y against X. A nil X plots Y
// against its index (0, 1, 2, ...); otherwise X and Y are paired and the
// shorter one decides how many points there are. Ink's zero value picks a
// colour from the theme (the accent for the first series, then a fixed
// palette), so a host names a colour only when it means one.
type PlotSeries struct {
	Label string
	X, Y  []float64
	Style PlotStyle
	Ink   RGBA
}

// len is the number of plotted points: len(Y), or the shorter of X and Y.
func (s PlotSeries) len() int {
	if s.X == nil {
		return len(s.Y)
	}
	return min(len(s.X), len(s.Y))
}

// x is the abscissa of point i: X[i], or i itself when X is nil.
func (s PlotSeries) x(i int) float64 {
	if s.X == nil {
		return float64(i)
	}
	return s.X[i]
}

// plotPalette is the colour of the second, third, ... series whose Ink is
// zero. The first takes theme.Accent. Mid-tones, so each reads on a light and
// on a dark background alike.
var plotPalette = []RGBA{
	RGB(0xE8, 0x71, 0x0A), // orange
	RGB(0x93, 0x34, 0xE6), // purple
	RGB(0xD9, 0x30, 0x25), // red
	RGB(0x12, 0x8C, 0xB5), // teal-blue
}

// XYPlotPad separates an XYPlot's plot area from its tick labels, legend
// and captions.
const XYPlotPad = 4

// XYPlot draws one or more numeric series against LABELLED axes: "nice"
// ticks (1, 2 or 5 times a power of ten) on both axes, a light grid, a
// legend for the series that carry a Label, and optional axis captions. It is
// the general-purpose sibling of LineChart (one series, index-spaced, no
// axis text) and TimeSeriesChart (one series against time): an XYPlot puts
// several curves on a real numeric x axis, which is what a signal against
// time, or a spectrum against frequency, needs.
//
// The series are a shared [mvvm.ObservableList], so a host binds its model
// to the plot instead of assigning a field. A series much longer than the
// plot is wide is drawn as a per-column min/max envelope, so a 65 536-point
// signal costs one stroke per pixel column rather than one per sample and
// still shows every peak.
//
// Hovering reports the point of the FIRST series nearest the pointer: a
// vertical rule, a marker and an "x  y" readout (Hover/HoverIndex, set by
// OnEvent from pointer motion, as the other charts do).
type XYPlot struct {
	Base
	// XLabel and YLabel are optional axis captions: XLabel under the x tick
	// labels, YLabel above the y axis.
	XLabel, YLabel string
	// FormatX and FormatY render one tick label (and the hover readout). Nil
	// uses a compact %g with four significant digits.
	FormatX, FormatY func(float64) string

	series     *mvvm.ObservableList[PlotSeries]
	hover      *mvvm.Observable[bool]
	hoverIndex *mvvm.Observable[int]
}

// NewXYPlot builds a plot over the given series.
func NewXYPlot(series ...PlotSeries) *XYPlot {
	p := &XYPlot{}
	p.Series().Append(series...)
	return p
}

// Series is the plotted curves as a shared [mvvm.ObservableList]. A list
// rather than an Observable because a slice is not comparable (see
// LineChart.Values). Created on first use, empty.
func (c *XYPlot) Series() *mvvm.ObservableList[PlotSeries] {
	if c.series == nil {
		c.series = mvvm.NewObservableList[PlotSeries]()
	}
	return c.series
}

// SetSeries replaces every curve at once — a Clear then an Append, so a
// subscriber sees the plot go empty and come back, never a mixture of old
// and new curves.
func (c *XYPlot) SetSeries(series ...PlotSeries) {
	c.Series().Clear()
	c.Series().Append(series...)
}

// Hover is whether the hover readout is shown, as a shared
// [mvvm.Observable]. Lazily created, off.
func (c *XYPlot) Hover() *mvvm.Observable[bool] {
	if c.hover == nil {
		c.hover = mvvm.NewObservable(false)
	}
	return c.hover
}

// HoverIndex is the hovered point of the first series, as a shared
// [mvvm.Observable]. Lazily created, 0.
func (c *XYPlot) HoverIndex() *mvvm.Observable[int] {
	if c.hoverIndex == nil {
		c.hoverIndex = mvvm.NewObservable(0)
	}
	return c.hoverIndex
}

func (c *XYPlot) formatX(v float64) string {
	if c.FormatX != nil {
		return c.FormatX(v)
	}
	return FormatPlotNumber(v)
}

func (c *XYPlot) formatY(v float64) string {
	if c.FormatY != nil {
		return c.FormatY(v)
	}
	return FormatPlotNumber(v)
}

// FormatPlotNumber is the default tick format: four significant digits in
// %g form, with a negative zero printed as "0".
func FormatPlotNumber(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'g', 4, 64)
}

// NiceStep is the tick spacing for a span cut into about n intervals: the
// smallest of 1, 2, 5 or 10 times a power of ten that is at least span/n.
// A span or n that is not positive gives 1.
func NiceStep(span float64, n int) float64 {
	if !(span > 0) || n < 1 || math.IsInf(span, 0) {
		return 1
	}
	raw := span / float64(n)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5} {
		if m*mag >= raw*(1-1e-9) {
			return m * mag
		}
	}
	return 10 * mag
}

// maxPlotTicks caps the ticks on one axis, so a degenerate step can never
// turn the loop that lists them into a long one.
const maxPlotTicks = 64

// plotTicks lists the multiples of step within [lo, hi]. A multiple that
// rounds to within a millionth of a step of zero is exactly zero, so the
// origin is labelled "0", not "-1.1e-16".
func plotTicks(lo, hi, step float64) []float64 {
	first := math.Ceil(lo/step - 1e-9)
	var ticks []float64
	for i := 0; i < maxPlotTicks; i++ {
		v := (first + float64(i)) * step
		if v > hi+step*1e-9 {
			break
		}
		if math.Abs(v) < step*1e-6 {
			v = 0
		}
		ticks = append(ticks, v)
	}
	return ticks
}

// span widens an empty or degenerate range so a scale can be drawn at all.
func span(lo, hi float64) (float64, float64) {
	if hi > lo {
		return lo, hi
	}
	if lo == 0 {
		return -1, 1
	}
	d := math.Abs(lo) / 2
	return lo - d, hi + d
}

// dataRange is the extent of every series: x over all points, y over all
// points plus 0 for a stem series (its stems start there). ok is false when
// there is no finite point at all.
func (c *XYPlot) dataRange() (xlo, xhi, ylo, yhi float64, ok bool) {
	xlo, ylo = math.Inf(1), math.Inf(1)
	xhi, yhi = math.Inf(-1), math.Inf(-1)
	for _, s := range c.Series().Slice() {
		for i := 0; i < s.len(); i++ {
			x, y := s.x(i), s.Y[i]
			if !finite(x) || !finite(y) {
				continue
			}
			xlo, xhi = math.Min(xlo, x), math.Max(xhi, x)
			ylo, yhi = math.Min(ylo, y), math.Max(yhi, y)
			ok = true
			if s.Style == PlotStem {
				ylo, yhi = math.Min(ylo, 0), math.Max(yhi, 0)
			}
		}
	}
	return xlo, xhi, ylo, yhi, ok
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// plotScale is the geometry of one Draw: the axis ranges, their ticks and
// the plot rectangle they map onto.
type plotScale struct {
	xlo, xhi, ylo, yhi float64
	xTicks, yTicks     []float64
	pl                 Rect
}

// px and py map data coordinates onto the plot rectangle, clamped to it.
func (s *plotScale) px(x float64) int {
	f := (x - s.xlo) / (s.xhi - s.xlo)
	return s.pl.X + int(math.Round(math.Min(1, math.Max(0, f))*float64(s.pl.W-1)))
}

func (s *plotScale) py(y float64) int {
	f := (y - s.ylo) / (s.yhi - s.ylo)
	return s.pl.Y + int(math.Round((1-math.Min(1, math.Max(0, f)))*float64(s.pl.H-1)))
}

// scale lays the plot out inside Bounds: the y range is widened to whole
// ticks (so the curve never touches the frame without a label there), the x
// range is the data's own (a spectrum's first and last bin sit on the frame).
func (c *XYPlot) scale() plotScale {
	r := c.Bounds()
	gh := c.glyphHeight()
	pad := scaled(XYPlotPad)
	xlo, xhi, ylo, yhi, ok := c.dataRange()
	if !ok {
		xlo, xhi, ylo, yhi = 0, 1, 0, 1
	}
	xlo, xhi = span(xlo, xhi)
	ylo, yhi = span(ylo, yhi)

	top := r.Y + gh/2 + pad
	if c.YLabel != "" {
		top = r.Y + gh + 2*pad
	}
	bottom := r.Y + r.H - (gh + pad)
	if c.XLabel != "" {
		bottom -= gh + pad
	}
	// A tick every three or so text heights.
	yStep := NiceStep(yhi-ylo, max(2, (bottom-top)/(3*gh+1)))
	ylo = math.Floor(ylo/yStep+1e-9) * yStep
	yhi = math.Ceil(yhi/yStep-1e-9) * yStep
	yTicks := plotTicks(ylo, yhi, yStep)
	left := 0
	for _, v := range yTicks {
		left = max(left, c.textWidth(c.formatY(v)))
	}
	left += r.X + pad
	right := r.X + r.W - 1 - (c.textWidth(c.formatX(xhi))/2 + pad)
	pl := Rect{X: left, Y: top, W: right - left, H: bottom - top}
	// A label is about eight glyphs wide; leave twice that between ticks.
	xStep := NiceStep(xhi-xlo, max(2, pl.W/(8*c.glyphAdvance()+1)))
	return plotScale{xlo: xlo, xhi: xhi, ylo: ylo, yhi: yhi,
		xTicks: plotTicks(xlo, xhi, xStep), yTicks: yTicks, pl: pl}
}

// inkFor is the colour of series i: its own Ink, else the theme's accent for
// the first, else the palette.
func inkFor(s PlotSeries, i int, theme *Theme) RGBA {
	if s.Ink != (RGBA{}) {
		return s.Ink
	}
	if i == 0 {
		return theme.Accent
	}
	return plotPalette[(i-1)%len(plotPalette)]
}

// Draw paints the grid and tick labels, every series, the legend and the
// hover readout.
func (c *XYPlot) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	s := c.scale()
	if s.pl.W < 2 || s.pl.H < 2 {
		return
	}
	pl := s.pl
	label := dimInk(theme)
	gh := c.glyphHeight()
	pad := scaled(XYPlotPad)

	for _, v := range s.yTicks {
		y := s.py(v)
		drawLine(p, pl.X, y, pl.X+pl.W-1, y, theme.Border)
		t := c.formatY(v)
		c.drawText(p, pl.X-pad-c.textWidth(t), y-gh/2, t, label)
	}
	for _, v := range s.xTicks {
		x := s.px(v)
		drawLine(p, x, pl.Y, x, pl.Y+pl.H-1, theme.Border)
		t := c.formatX(v)
		tx := min(max(x-c.textWidth(t)/2, r.X), r.X+r.W-c.textWidth(t))
		c.drawText(p, tx, pl.Y+pl.H+pad/2, t, label)
	}
	// The frame: left and bottom rules, in the label ink so the axes read
	// above the grid.
	drawLine(p, pl.X, pl.Y, pl.X, pl.Y+pl.H-1, label)
	drawLine(p, pl.X, pl.Y+pl.H-1, pl.X+pl.W-1, pl.Y+pl.H-1, label)
	if c.YLabel != "" {
		c.drawText(p, r.X, r.Y, c.YLabel, label)
	}
	if c.XLabel != "" {
		c.drawText(p, pl.X+pl.W-c.textWidth(c.XLabel), r.Y+r.H-gh, c.XLabel, label)
	}

	series := c.Series().Slice()
	for i, ser := range series {
		ink := inkFor(ser, i, theme)
		switch ser.Style {
		case PlotStem:
			c.drawStems(p, &s, ser, ink)
		case PlotMarkers:
			c.drawMarkers(p, &s, ser, ink)
		default:
			c.drawLineSeries(p, &s, ser, ink)
		}
	}
	c.drawLegend(p, theme, &s, series)
	c.drawHover(p, theme, &s, series)
}

// column accumulates the points that land in one pixel column.
type column struct {
	used                      bool
	minY, maxY, firstY, lastY int
}

// columns buckets a series by pixel column: the envelope of a series denser
// than the plot is wide.
func columns(s *plotScale, ser PlotSeries) []column {
	cols := make([]column, s.pl.W)
	for i := 0; i < ser.len(); i++ {
		x, y := ser.x(i), ser.Y[i]
		if !finite(x) || !finite(y) {
			continue
		}
		cx, cy := s.px(x)-s.pl.X, s.py(y)
		col := &cols[cx]
		if !col.used {
			*col = column{used: true, minY: cy, maxY: cy, firstY: cy}
		}
		col.minY, col.maxY, col.lastY = min(col.minY, cy), max(col.maxY, cy), cy
	}
	return cols
}

// dense reports whether a series has more points than the plot has columns.
func dense(s *plotScale, ser PlotSeries) bool { return ser.len() > s.pl.W }

func (c *XYPlot) drawLineSeries(p painter.Painter, s *plotScale, ser PlotSeries, ink RGBA) {
	if dense(s, ser) {
		prev := -1
		cols := columns(s, ser)
		for x, col := range cols {
			if !col.used {
				continue
			}
			if prev >= 0 {
				drawLine(p, s.pl.X+prev, cols[prev].lastY, s.pl.X+x, col.firstY, ink)
			}
			drawLine(p, s.pl.X+x, col.minY, s.pl.X+x, col.maxY, ink)
			prev = x
		}
		return
	}
	havePrev := false
	var px, py int
	for i := 0; i < ser.len(); i++ {
		x, y := ser.x(i), ser.Y[i]
		if !finite(x) || !finite(y) {
			havePrev = false // a gap, not a jump across it
			continue
		}
		cx, cy := s.px(x), s.py(y)
		if havePrev {
			plotStroke(p, px, py, cx, cy, ink)
		} else {
			fillRect(p, cx, cy, 1, 1, ink)
		}
		px, py, havePrev = cx, cy, true
	}
}

func (c *XYPlot) drawStems(p painter.Painter, s *plotScale, ser PlotSeries, ink RGBA) {
	base := s.py(0)
	if dense(s, ser) {
		for x, col := range columns(s, ser) {
			if col.used {
				drawLine(p, s.pl.X+x, min(col.minY, base), s.pl.X+x, max(col.maxY, base), ink)
			}
		}
		return
	}
	// Head markers only while the stems stand apart.
	heads := ser.len()*3 <= s.pl.W
	for i := 0; i < ser.len(); i++ {
		x, y := ser.x(i), ser.Y[i]
		if !finite(x) || !finite(y) {
			continue
		}
		cx, cy := s.px(x), s.py(y)
		drawLine(p, cx, base, cx, cy, ink)
		if heads {
			c.marker(p, s, cx, cy, ink)
		}
	}
}

func (c *XYPlot) drawMarkers(p painter.Painter, s *plotScale, ser PlotSeries, ink RGBA) {
	for i := 0; i < ser.len(); i++ {
		x, y := ser.x(i), ser.Y[i]
		if finite(x) && finite(y) {
			c.marker(p, s, s.px(x), s.py(y), ink)
		}
	}
}

// plotStroke draws one curve segment one LOGICAL pixel wide, anti-aliased
// on a pixel back-end and Bresenham on a cell one (see drawCurveLine): a
// one-device-pixel curve is a hairline on a HiDPI screen.
func plotStroke(p painter.Painter, x0, y0, x1, y1 int, c RGBA) {
	if pp, ok := p.(painter.PathPainter); ok {
		path := painter.NewPath().MoveTo(float64(x0), float64(y0)).LineTo(float64(x1), float64(y1))
		pp.StrokePath(path, c, float64(strokeWidth()))
		return
	}
	drawLine(p, x0, y0, x1, y1, c)
}

// marker paints a small square centred on (x, y), kept inside the plot.
func (c *XYPlot) marker(p painter.Painter, s *plotScale, x, y int, ink RGBA) {
	m := max(3, scaled(3))
	pl := s.pl
	fillRect(p, min(max(x-m/2, pl.X), pl.X+pl.W-m), min(max(y-m/2, pl.Y), pl.Y+pl.H-m), m, m, ink)
}

// drawLegend lists the labelled series in the plot's top-right corner, on a
// surface panel so the curves under it do not cross the text.
func (c *XYPlot) drawLegend(p painter.Painter, theme *Theme, s *plotScale, series []PlotSeries) {
	pad := scaled(XYPlotPad)
	gh := c.glyphHeight()
	swatch := scaled(14)
	w, rows := 0, 0
	for _, ser := range series {
		if ser.Label != "" {
			w = max(w, c.textWidth(ser.Label))
			rows++
		}
	}
	if rows == 0 {
		return
	}
	bw := swatch + pad + w + 2*pad
	bh := rows*(gh+pad) + pad
	bx := s.pl.X + s.pl.W - bw - pad
	by := s.pl.Y + pad
	if bx < s.pl.X || by+bh > s.pl.Y+s.pl.H {
		return // no room: a legend over the axis labels would hide them
	}
	fillRect(p, bx, by, bw, bh, theme.Surface)
	strokeRect(p, bx, by, bw, bh, theme.Border)
	y := by + pad
	for i, ser := range series {
		if ser.Label == "" {
			continue
		}
		fillRect(p, bx+pad, y+gh/2-max(1, scaled(1)), swatch, max(2, scaled(2)), inkFor(ser, i, theme))
		c.drawText(p, bx+pad+swatch+pad, y, ser.Label, theme.OnSurface)
		y += gh + pad
	}
}

// drawHover marks the hovered point of the first series and prints its
// coordinates in the plot's top-left corner.
func (c *XYPlot) drawHover(p painter.Painter, theme *Theme, s *plotScale, series []PlotSeries) {
	if !c.Hover().Get() || len(series) == 0 {
		return
	}
	ser, i := series[0], c.HoverIndex().Get()
	if i < 0 || i >= ser.len() || !finite(ser.x(i)) || !finite(ser.Y[i]) {
		return
	}
	pl := s.pl
	hx, hy := s.px(ser.x(i)), s.py(ser.Y[i])
	drawLine(p, hx, pl.Y, hx, pl.Y+pl.H-1, dimInk(theme))
	c.marker(p, s, hx, hy, inkFor(ser, 0, theme))
	text := fmt.Sprintf("%s  %s", c.formatX(ser.x(i)), c.formatY(ser.Y[i]))
	pad := scaled(XYPlotPad)
	tw, gh := c.textWidth(text), c.glyphHeight()
	bw, bh := min(tw+2*pad, pl.W-pad), gh+pad
	fillRect(p, pl.X+pad, pl.Y+pad, bw, bh, theme.Surface)
	strokeRect(p, pl.X+pad, pl.Y+pad, bw, bh, theme.Border)
	c.drawText(p, pl.X+2*pad, pl.Y+pad+pad/2, text, theme.OnSurface)
}

// ValueAt maps a widget-local x to the point of the first series whose x is
// nearest, returning its index and coordinates. ok is false when the plot
// has no first series or it has no finite point.
func (c *XYPlot) ValueAt(localX int) (index int, x, y float64, ok bool) {
	series := c.Series().Slice()
	if len(series) == 0 {
		return 0, 0, 0, false
	}
	s := c.scale()
	ser := series[0]
	target := c.Bounds().X + localX
	best := math.MaxInt
	for i := 0; i < ser.len(); i++ {
		if !finite(ser.x(i)) || !finite(ser.Y[i]) {
			continue
		}
		d := s.px(ser.x(i)) - target
		if d < 0 {
			d = -d
		}
		if d < best {
			best, index, x, y, ok = d, i, ser.x(i), ser.Y[i], true
		}
	}
	return index, x, y, ok
}

// OnEvent tracks the hover readout from the pointer: a move over the plot
// selects the nearest point of the first series, a move off it (a container
// forwards moves to every child) clears the readout.
func (c *XYPlot) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	i, _, _, ok := c.ValueAt(ev.X)
	if !ok || !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	c.HoverIndex().Set(i)
	c.Hover().Set(true)
}

// A11y reports the plot as an img carrying its series and point counts, the
// convention of the other charts.
func (c *XYPlot) A11y() A11yInfo {
	series := c.Series().Slice()
	n := 0
	for _, s := range series {
		n += s.len()
	}
	return A11yInfo{Role: RoleImg, Name: c.YLabel, Value: fmt.Sprintf("%d series, %d points", len(series), n)}
}

var _ Accessible = (*XYPlot)(nil)
