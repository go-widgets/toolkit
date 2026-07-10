// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit_test

import (
	"fmt"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// newSurface returns a PixelPainter over a fresh w×h RGBA buffer — the render
// target the examples draw into. A CellPainter would render the same widgets to
// a terminal grid instead.
func newSurface(w, h int) *painter.PixelPainter {
	return painter.NewPixelPainter(make([]byte, 4*w*h), w, h)
}

// ExampleLineChart plots a series as a polyline over an axis frame.
func ExampleLineChart() {
	chart := toolkit.NewLineChart([]float64{3, 7, 2, 8, 5, 9, 4})
	chart.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 220, H: 80})
	chart.Draw(newSurface(220, 80), toolkit.DefaultLight())
}

// ExampleBarChart plots non-negative values as vertical bars.
func ExampleBarChart() {
	chart := toolkit.NewBarChart([]float64{4, 7, 2, 8, 5})
	chart.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 80})
	chart.Draw(newSurface(200, 80), toolkit.DefaultLight())
}

// ExamplePieChart fills a disc with one proportional wedge per value.
func ExamplePieChart() {
	chart := toolkit.NewPieChart([]float64{3, 5, 2, 4})
	chart.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 120, H: 120})
	chart.Draw(newSurface(120, 120), toolkit.DefaultLight())
}

// ExampleMarkdownView renders a Markdown subset (headings, lists, code,
// paragraphs) laid out for the bitmap font.
func ExampleMarkdownView() {
	md := toolkit.NewMarkdownView("# Title\n\nA paragraph.\n\n- one\n- two")
	md.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 260, H: 120})
	md.Draw(newSurface(260, 120), toolkit.DefaultLight())
}

// ExampleDatePicker shows a date field with a drop-down calendar.
func ExampleDatePicker() {
	dp := toolkit.NewDatePicker(2026, 7, 10)
	dp.OnChange = func(y, m, d int) { fmt.Printf("picked %04d-%02d-%02d\n", y, m, d) }
	dp.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 170, H: toolkit.DatePickerFieldH()})
	fmt.Println(dp.Text())
	// Output: 2026-07-10
}

// ExampleRangeSlider selects a sub-interval; SetRange orders + clamps its ends.
func ExampleRangeSlider() {
	rs := toolkit.NewRangeSlider(0, 100, 20, 80)
	rs.SetRange(90, 10) // passed out of order → normalised to Low <= High
	fmt.Printf("%.0f-%.0f\n", rs.Low, rs.High)
	// Output: 10-90
}

// ExampleSetFont swaps the active font; every widget re-lays-out against the
// new metrics. NewBitmapFont(2) doubles the built-in bitmap ("retina" text).
func ExampleSetFont() {
	fmt.Println(toolkit.GlyphHeight()) // default 5×7 bitmap
	toolkit.SetFont(toolkit.NewBitmapFont(2))
	fmt.Println(toolkit.GlyphHeight()) // doubled
	toolkit.SetFont(nil)               // restore the default
	fmt.Println(toolkit.GlyphHeight())
	// Output:
	// 7
	// 14
	// 7
}

// ExampleContextMenu shows a right-click popup that auto-sizes, clamps inside
// the surface, and dismisses on an outside click.
func ExampleContextMenu() {
	menu := toolkit.NewMenu([]toolkit.MenuItem{
		{Label: "Cut", Action: func() {}},
		{Label: "Copy", Action: func() {}, Shortcut: "Ctrl+C"},
	})
	cm := toolkit.NewContextMenu(menu)
	cm.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 160})
	cm.Popup(8, 8) // open at the cursor
	cm.Draw(newSurface(200, 160), toolkit.DefaultLight())
}

// ExampleOverlay stacks z-ordered layers above a primary child. Events route
// top-down; a Modal overlay swallows clicks that miss every layer.
func ExampleOverlay() {
	base := toolkit.NewLabel("main content")
	ov := toolkit.NewOverlay(base)
	ov.Push(toolkit.NewTooltip("floating layer"))
	ov.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 200, H: 120})
	ov.Draw(newSurface(200, 120), toolkit.DefaultLight())
}

// ExampleCollectA11y walks a host-composed widget list and returns each
// Accessible widget's role, name, and value for a screen-reader bridge.
func ExampleCollectA11y() {
	widgets := []toolkit.Widget{
		toolkit.NewButton("Save", nil),
		toolkit.NewCheckButton("Wrap", true),
	}
	for _, info := range toolkit.CollectA11y(widgets) {
		fmt.Printf("%s %q %q\n", info.Role, info.Name, info.Value)
	}
	// Output:
	// button "Save" ""
	// checkbox "Wrap" "checked"
}
