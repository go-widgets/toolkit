// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"
)

const chW, chH = 160, 120

func drawInBounds(t *testing.T, name string, w Widget) {
	t.Helper()
	buf := makeSurface(chW, chH)
	w.Draw(newP(buf, chW), DefaultLight())
	minX, minY, maxX, maxY := nbPaintedBBox(buf, chW, chH)
	r := w.Bounds()
	if minX >= 0 && (minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H) {
		t.Fatalf("%s hover painted out of bounds: X[%d..%d] Y[%d..%d] r=%+v", name, minX, maxX, minY, maxY, r)
	}
}

func TestSparklineValueAtAndHover(t *testing.T) {
	sl := NewSparkline([]float64{3, 7, 2, 8, 5})
	sl.SetBounds(Rect{X: 10, Y: 10, W: 130, H: 40})
	n := len(sl.Values)
	span := sl.plot().W - 1
	for i := range sl.Values {
		lx := SparkPad + i*span/(n-1)
		gi, gv, ok := sl.ValueAt(lx)
		if !ok || gi != i || gv != sl.Values[i] {
			t.Fatalf("Sparkline.ValueAt %d = (%d,%v,%v)", i, gi, gv, ok)
		}
	}
	if _, _, ok := NewSparkline(nil).ValueAt(5); ok {
		t.Fatal("empty Sparkline.ValueAt should be ok=false")
	}
	if i, v, ok := NewSparkline([]float64{9}).ValueAt(999); !ok || i != 0 || v != 9 {
		t.Fatalf("single Sparkline.ValueAt = (%d,%v,%v)", i, v, ok)
	}
	tiny := NewSparkline([]float64{1, 2, 3})
	tiny.SetBounds(Rect{X: 0, Y: 0, W: 2*SparkPad + 1, H: 10})
	if _, _, ok := tiny.ValueAt(0); !ok {
		t.Fatal("narrow Sparkline.ValueAt should resolve")
	}
	// Line crosshair.
	sl.Hover().Set(true)
	sl.HoverIndex().Set(2)
	drawInBounds(t, "sparkline-line", sl)
	// Bar highlight.
	sb := NewSparkline([]float64{3, 7, 2, 8, 5})
	sb.Kind = SparkBar
	sb.SetBounds(Rect{X: 10, Y: 10, W: 130, H: 40})
	sb.Hover().Set(true)
	sb.HoverIndex().Set(3)
	drawInBounds(t, "sparkline-bar", sb)
	// A bar spark with more values than pixels exercises the slot<1 clamp in
	// the hover highlight. (drawBars itself overflows a sub-pixel plot — a
	// separate Sparkline limitation — so this only asserts no panic.)
	nb := NewSparkline(make([]float64, 300))
	nb.Kind = SparkBar
	nb.SetBounds(Rect{X: 0, Y: 0, W: 2*SparkPad + 10, H: 20})
	nb.Hover().Set(true)
	nb.HoverIndex().Set(5)
	nb.Draw(newP(makeSurface(chW, chH), chW), DefaultLight())
}

func TestBarChartHover(t *testing.T) {
	bc := NewBarChart([]float64{4, 7, 2, 8, 5})
	bc.SetBounds(Rect{X: 10, Y: 10, W: 130, H: 90})
	bc.Hover().Set(true)
	bc.HoverIndex().Set(4) // last bar → exercises the clamp
	drawInBounds(t, "barchart", bc)
}

func TestScatterNearestPointAndHover(t *testing.T) {
	sc := NewScatterChart([][]ScatterPoint{
		{{X: 1, Y: 2}, {X: 3, Y: 5}, {X: 6, Y: 4}},
		{{X: 2, Y: 6}, {X: 5, Y: 2}},
	})
	sc.SetBounds(Rect{X: 10, Y: 10, W: 130, H: 100})
	// Aim at series 0, point 1's projected pixel.
	xr, yr, _ := sc.ranges()
	px, py := sc.project(sc.Series[0][1], xr, yr)
	r := sc.Bounds()
	si, pi, pt, ok := sc.NearestPoint(px-r.X, py-r.Y)
	if !ok || si != 0 || pi != 1 || pt != sc.Series[0][1] {
		t.Fatalf("NearestPoint = (%d,%d,%v,%v)", si, pi, pt, ok)
	}
	if _, _, _, ok := NewScatterChart(nil).NearestPoint(5, 5); ok {
		t.Fatal("empty ScatterChart.NearestPoint should be ok=false")
	}
	sc.Hover().Set(true)
	sc.HoverSeries().Set(0)
	sc.HoverPoint().Set(1)
	drawInBounds(t, "scatter", sc)
}

func TestPieSliceAtAndHover(t *testing.T) {
	pc := NewPieChart([]float64{3, 5, 2, 4})
	pc.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 100})
	// A point just clockwise of 12 o'clock lands in slice 0.
	r := pc.Bounds()
	idx, val, ok := pc.SliceAt(r.W/2+2, r.H/2-20)
	if !ok || idx != 0 || val != pc.Values[0] {
		t.Fatalf("SliceAt(top) = (%d,%v,%v), want slice 0", idx, val, ok)
	}
	if _, _, ok := pc.SliceAt(0, 0); ok { // corner is outside the disc
		t.Fatal("SliceAt(corner) should be ok=false")
	}
	if _, _, ok := pc.SliceAt(r.W/2-20, r.H/2); !ok { // left of centre → theta<0 wrap
		t.Fatal("SliceAt(left) should resolve a slice")
	}
	if _, _, ok := NewPieChart(nil).SliceAt(5, 5); ok {
		t.Fatal("empty PieChart.SliceAt should be ok=false")
	}
	tinyPie := NewPieChart([]float64{1, 1})
	tinyPie.SetBounds(Rect{X: 0, Y: 0, W: 1, H: 1}) // radius < 1
	if _, _, ok := tinyPie.SliceAt(0, 0); ok {
		t.Fatal("sub-pixel PieChart.SliceAt should be ok=false")
	}
	pc.Hover().Set(true)
	pc.HoverIndex().Set(2)
	drawInBounds(t, "pie", pc)
	pc.HoverIndex().Set(0) // first slice → a0 == 0 branch
	drawInBounds(t, "pie0", pc)
}

func TestRadarAxisAtAndHover(t *testing.T) {
	rc := NewRadarChart([]string{"A", "B", "C", "D", "E"}, [][]float64{{8, 6, 7, 4, 9}})
	rc.SetBounds(Rect{X: 10, Y: 10, W: 130, H: 100})
	r := rc.Bounds()
	n := len(rc.Axes)
	for k := 0; k < n; k++ {
		a := axisAngle(k, n)
		lx := r.W/2 + int(20*math.Cos(a))
		ly := r.H/2 + int(20*math.Sin(a))
		if got, ok := rc.AxisAt(lx, ly); !ok || got != k {
			t.Fatalf("AxisAt axis %d = (%d,%v)", k, got, ok)
		}
	}
	if got, ok := rc.AxisAt(r.W/2, r.H/2); !ok || got != 0 { // centre → axis 0
		t.Fatalf("AxisAt(centre) = (%d,%v)", got, ok)
	}
	if _, ok := NewRadarChart(nil, nil).AxisAt(5, 5); ok {
		t.Fatal("no-axes AxisAt should be ok=false")
	}
	if angleNorm(3*math.Pi) > math.Pi || angleNorm(-3*math.Pi) <= -math.Pi {
		t.Fatal("angleNorm did not wrap into (-π, π]")
	}
	rc.Hover().Set(true)
	rc.HoverAxis().Set(2)
	drawInBounds(t, "radar", rc)
}
