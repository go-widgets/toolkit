// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestStackedBarChartStacksLayersBottomUp(t *testing.T) {
	red, blue := RGBA{R: 255, A: 255}, RGBA{B: 255, A: 255}
	c := NewStackedBarChart([][]float64{{10, 0}, {10, 20}, {}}, []string{"a", "", "c"})
	c.Colors = []RGBA{red, blue}
	c.Max = 20
	c.TimeMin, c.TimeMax = 1000, 2000
	c.SetBounds(Rect{W: 200, H: 120})
	surf := makeSurface(200, 120)
	c.Draw(newP(surf, 200), DefaultLight())
	pl := c.plot()
	bottom := pl.Y + pl.H - 2
	topY := pl.Y + 1
	if got := pixelAt(surf, 200, pl.X+1, bottom); got != red {
		t.Fatalf("lowest layer of bucket 0 = %v, want red", got)
	}
	if got := pixelAt(surf, 200, pl.X+1, topY); got != blue {
		t.Fatalf("top of the full stack = %v, want blue", got)
	}
	// Bucket 1 has only the blue layer, starting at the baseline.
	if got := pixelAt(surf, 200, pl.X+pl.W-3, bottom); got != blue {
		t.Fatalf("bucket 1 base = %v, want blue", got)
	}
	if countInk(surf, 200, 120, red) == 0 || countPainted(surf, 200, 120) == 0 {
		t.Fatal("nothing drawn")
	}
}

func TestStackedBarChartAutoMaxFormatsAndEmpty(t *testing.T) {
	c := NewStackedBarChart([][]float64{{3, -4}, {1}}, nil)
	if c.top() != NiceCeiling(4) {
		t.Fatalf("top %v", c.top())
	}
	c.FormatValue = func(v float64) string { return "v" }
	c.FormatTime = func(int64) string { return "t" }
	if c.formatValue(1) != "v" || c.formatTime(1) != "t" {
		t.Fatal("custom formatters ignored")
	}
	c.FormatValue, c.FormatTime = nil, nil
	if c.formatValue(2) != "2" || c.formatTime(0) == "" {
		t.Fatal("default formatters")
	}
	c.SetBounds(Rect{W: 0, H: 0})
	c.Draw(newP(makeSurface(10, 10), 10), DefaultLight()) // no panic
	e := NewStackedBarChart(nil, nil)
	if e.buckets() != 0 || e.top() != 1 {
		t.Fatal("empty chart scale")
	}
	e.SetBounds(Rect{W: 100, H: 80})
	e.Draw(newP(makeSurface(100, 80), 100), DefaultDark()) // axes only, no bars
	if e.layerColor(0) == (RGBA{}) {
		t.Fatal("default palette")
	}
}

func TestStackedBarChartA11y(t *testing.T) {
	if v := NewStackedBarChart([][]float64{{1, 2}}, nil).A11y().Value; v != "1 layers, 2 buckets" {
		t.Fatal(v)
	}
}
